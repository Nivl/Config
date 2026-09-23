#!/usr/bin/env python3
# PostToolUse hook (Workflow, Bash): when a Workflow call that carried `args.tag`
# returns, run the review-and-fix usage filter over this session's sub-agent
# transcripts and produce the `usage kind=...` lines for that tag. A Workflow
# that runs in the background returns before its agents write anything, so
# _after_bash also sweeps every stamped transcript on each Bash call while a run
# marker exists.
#
# Where the lines go depends on one marker file:
#
#   ~/.melvin/config/logs/review-and-fix/.active-<session_id>
#
# review-and-fix writes it at Step 0 with the run log path as its content and
# deletes it at the Final Report. When it exists, the lines are appended to that
# run log directly, skipping any `id=` the log already has at the same turn
# count or higher, so a second firing for the same tag is idempotent. A logged
# line with fewer turns than the fresh one was written while that agent was
# still running, and it is replaced rather than skipped. One line per id is what
# the Spend total sums over, so appending the complete line beside the partial
# one would count that agent twice. When it does not exist (pr-review, a
# standalone in-depth run, any other tagged workflow), the lines come back as
# additionalContext so the orchestrator has them in front of it with nothing to
# run.
#
# Why a hook and not the skill's own instruction: two of three logged
# review-and-fix runs never ran the filter and wrote an unpriced token total
# instead. The instruction still exists in Step 3. This makes the lines exist
# whether or not it is followed.
#
# What this cannot capture: an agent still running when the workflow returns.
# The scorer launches after the fan-out, so its line lands on the next tagged
# workflow call in the same session (the dedupe makes that safe), and the last
# iteration's scorer is the skill's own jq run at the Final Report. Any failure
# here is silent. A missing line is the skill's problem to notice, and a hook
# that errors would only add noise to a tool result.

import glob
import json
import os
import re
import subprocess
import sys

HOOK_DIR = os.path.dirname(os.path.realpath(__file__))
USAGE_JQ = os.path.join(HOOK_DIR, "..", "skills", "review-and-fix", "usage.jq")
MARKER_DIR = os.path.expanduser("~/.melvin/config/logs/review-and-fix")
ID_RE = re.compile(r"\bid=([0-9a-f]+)\b")
TURNS_RE = re.compile(r"\bturns=(\d+)\b")


def _turns(line):
    m = TURNS_RE.search(line)
    return int(m.group(1)) if m else 0


def _transcripts(session_dir):
    return sorted(
        glob.glob(os.path.join(session_dir, "subagents", "agent-*.jsonl"))
        + glob.glob(os.path.join(session_dir, "subagents", "workflows", "*", "agent-*.jsonl"))
    )


def _usage_lines(files):
    if not files or not os.path.exists(USAGE_JQ):
        return []
    try:
        out = subprocess.run(
            ["jq", "-r", "-n", "-f", USAGE_JQ, *files],
            capture_output=True, text=True, timeout=60,
        )
    except Exception:
        return []
    if out.returncode != 0:
        return []
    return [ln for ln in out.stdout.splitlines() if ln.startswith("usage ")]


def _emit(text):
    json.dump(
        {"hookSpecificOutput": {"hookEventName": "PostToolUse", "additionalContext": text}},
        sys.stdout,
    )


def _read_marker(session_id):
    marker = os.path.join(MARKER_DIR, f".active-{session_id}")
    try:
        with open(marker) as fh:
            run_log = fh.read().strip() or None
    except OSError:
        return marker, None
    return marker, run_log if run_log and os.path.isfile(run_log) else None


def _merge_into_log(run_log, lines):
    # Returns (appended, replaced, already_complete). Raises OSError.
    with open(run_log) as fh:
        log = fh.read().splitlines()
    # Only ids on usage lines count, so a Jira id or a sha elsewhere
    # in the log cannot make a fresh line look already present.
    have = {}
    for i, ln in enumerate(log):
        if not ln.startswith("usage "):
            continue
        m = ID_RE.search(ln)
        if m:
            have[m.group(1)] = (_turns(ln), i)

    fresh, grown = [], {}
    for ln in lines:
        m = ID_RE.search(ln)
        prior = have.get(m.group(1)) if m else None
        if prior is None:
            fresh.append(ln)
        elif _turns(ln) > prior[0]:
            # The logged line was written while this agent was still
            # running, so its counts stopped short of the agent's own
            # total. Replacing it keeps one line per id, which is what
            # the Spend total sums over.
            grown[prior[1]] = ln

    if grown:
        for i, ln in grown.items():
            log[i] = ln
        tmp = run_log + ".usage-lines.tmp"
        with open(tmp, "w") as fh:
            fh.write("\n".join(log + fresh) + "\n")
        os.replace(tmp, run_log)
    elif fresh:
        with open(run_log, "a") as fh:
            fh.write("\n".join(fresh) + "\n")
    return len(fresh), len(grown), len(lines) - len(fresh) - len(grown)


def _after_bash(data):
    # The Workflow tool returns as soon as it launches, so at its PostToolUse no
    # role has a transcript yet and the tagged pass above finds nothing. The
    # orchestrator's next Bash calls come after the workflow's notification, so
    # this pass sweeps every stamped transcript into the run log then. It runs
    # only while a run marker exists, and only when a transcript changed since
    # the last sweep, which the .seen file beside the marker records.
    marker, run_log = _read_marker(data.get("session_id", ""))
    if not run_log:
        return
    transcript = data.get("transcript_path") or ""
    if not transcript.endswith(".jsonl"):
        return
    files = _transcripts(transcript[: -len(".jsonl")])
    if not files:
        return
    try:
        newest = max(os.path.getmtime(f) for f in files)
    except OSError:
        return
    seen_path = marker + ".seen"
    try:
        with open(seen_path) as fh:
            if float(fh.read().strip() or 0) >= newest:
                return
    except (OSError, ValueError):
        pass
    lines = [ln for ln in _usage_lines(files) if not ln.startswith("usage kind=unstamped ")]
    try:
        added, replaced, _ = _merge_into_log(run_log, lines)
        with open(seen_path, "w") as fh:
            fh.write(repr(newest))
    except OSError:
        return
    if added or replaced:
        _emit(
            f"usage-lines: appended {added} usage line(s) to {run_log} and replaced {replaced} "
            "written mid-run. Do not append them again."
        )


def main() -> None:
    try:
        data = json.load(sys.stdin)
    except Exception:
        return
    if data.get("tool_name") == "Bash":
        _after_bash(data)
        return
    if data.get("tool_name") != "Workflow":
        return
    args = (data.get("tool_input") or {}).get("args") or {}
    if isinstance(args, str):
        # One run passed args as a JSON string and the workflow accepted it.
        # The hook has to read the tag out of that shape too, or it goes
        # silent for the whole run, which is what happened.
        try:
            args = json.loads(args)
        except ValueError:
            args = {}
    tag = args.get("tag") if isinstance(args, dict) else None
    if not tag:
        return
    transcript = data.get("transcript_path") or ""
    if not transcript.endswith(".jsonl"):
        return
    session_dir = transcript[: -len(".jsonl")]

    lines = [ln for ln in _usage_lines(_transcripts(session_dir)) if f" tag={tag} " in ln + " "]
    if not lines:
        return

    _, run_log = _read_marker(data.get("session_id", ""))
    if run_log:
        try:
            added, replaced, complete = _merge_into_log(run_log, lines)
            _emit(
                f"usage-lines: appended {added} usage line(s) for tag={tag} to {run_log}, "
                f"replaced {replaced} that had been written mid-run "
                f"({complete} already complete). "
                "Do not append them again."
            )
            return
        except OSError:
            pass

    _emit(
        f"usage-lines: {len(lines)} priced usage line(s) for tag={tag}, from usage.jq over this "
        "session's transcripts. Append them to the run log as they are:\n" + "\n".join(lines)
    )


if __name__ == "__main__":
    main()
