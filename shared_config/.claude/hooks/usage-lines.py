#!/usr/bin/env python3
# PostToolUse hook (Workflow): when a Workflow call that carried `args.tag`
# returns, run the review-and-fix usage filter over this session's sub-agent
# transcripts and produce the `usage kind=...` lines for that tag.
#
# Where the lines go depends on one marker file:
#
#   ~/.melvin/config/logs/review-and-fix/.active-<session_id>
#
# review-and-fix writes it at Step 0 with the run log path as its content and
# deletes it at the Final Report. When it exists, the lines are appended to that
# run log directly, skipping any `id=` the log already has, so a second firing
# for the same tag is idempotent. When it does not exist (pr-review, a
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


def main() -> None:
    try:
        data = json.load(sys.stdin)
    except Exception:
        return
    if data.get("tool_name") != "Workflow":
        return
    args = (data.get("tool_input") or {}).get("args") or {}
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

    marker = os.path.join(MARKER_DIR, f".active-{data.get('session_id', '')}")
    run_log = None
    try:
        with open(marker) as fh:
            run_log = fh.read().strip() or None
    except OSError:
        run_log = None

    if run_log and os.path.isfile(run_log):
        try:
            with open(run_log) as fh:
                have = set(ID_RE.findall(fh.read()))
            fresh = [ln for ln in lines if not (ID_RE.search(ln) and ID_RE.search(ln).group(1) in have)]
            if fresh:
                with open(run_log, "a") as fh:
                    fh.write("\n".join(fresh) + "\n")
            _emit(
                f"usage-lines: appended {len(fresh)} usage line(s) for tag={tag} to {run_log} "
                f"({len(lines) - len(fresh)} already present). Do not append them again."
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
