#!/usr/bin/env python3
# PreToolUse hook (Bash): inside a review-and-fix run, deny the command that
# deletes the run marker until the run log holds the record the skill requires.
#
#   ~/.melvin/config/logs/review-and-fix/.active-<session_id>
#
# Deleting that marker is the Final Report's last act, so it is the one moment
# the whole log can be checked. Per iteration: a `stamps:` line, a `severity`
# line, a scoring record when the iteration kept or dropped anything, a
# `usage kind=review-roles` line, and the `### Iteration <N> summary` block.
# Then the Final Report's heading and its Coverage, Spend, Severity and Outcome
# sections. The deny lists what is missing, so the orchestrator fills it in and
# deletes again. A run that aborted on row 0 is exempt, since it reviewed
# nothing. RUN_LOG_OK=1 in front of the command overrides and falls to the
# permission prompt. Any failure here is silent.
#
# Why: runs orchestrated on Opus 5.5 skipped the scorer, the per-iteration
# summaries and most of the Final Report while following the rest of the loop,
# and a skipped record is invisible until someone reads the log.

import importlib.util
import json
import os
import re
import sys

HOOK_DIR = os.path.dirname(os.path.realpath(__file__))
MARKER_DIR = os.path.expanduser("~/.melvin/config/logs/review-and-fix")
DELETERS = {"rm", "unlink", "mv", "trash"}
REPORT_SECTIONS = ("### Coverage", "### Spend", "### Severity", "### Outcome")
SEVERITY_NUMS = re.compile(r"=(\d+)")


def _current_iter(text):
    spec = importlib.util.spec_from_file_location("rcl", os.path.join(HOOK_DIR, "require-case-list.py"))
    rcl = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(rcl)
    return rcl._current_iter(text)


def _deletes_marker(cmd, sid):
    if f".active-{sid}" not in cmd:
        return False
    words = re.split(r"[\s;&|()]+", cmd)
    return any(os.path.basename(w) in DELETERS for w in words)


def _missing(text):
    if "REVIEW_UNAVAILABLE_NO_FANOUT" in text:
        return []
    cur = _current_iter(text)
    out = []
    if cur is None:
        out.append("no iteration marker at all (t0 iter=N, ## Iteration N or stamps: iter=N)")
        cur = 0
    for i in range(1, cur + 1):
        need = []
        if not re.search(rf"^stamps: iter={i}\b", text, re.M):
            need.append(f"`stamps: iter={i}`")
        sev = re.search(rf"^severity iter={i} kept:.*$", text, re.M)
        if not sev:
            need.append(f"`severity iter={i} kept: ... | dropped: ... | fixed: ...`")
        # Count the buckets only, after "kept:", so the iteration number is not read as one.
        found_any = sev is None or any(int(n) for n in SEVERITY_NUMS.findall(sev.group(0).split("kept:", 1)[1]))
        scored = re.search(rf"^scored iter={i}\b", text, re.M) or re.search(
            rf"^usage kind=review-scorer\b.*\btag=iter{i}\b", text, re.M)
        if found_any and not scored:
            need.append(f"a scoring record (`scored iter={i} by=...` or the scorer's usage line)")
        if not re.search(rf"^usage kind=review-roles\b.*\btag=iter{i}\b", text, re.M):
            need.append(f"`usage kind=review-roles ... tag=iter{i}` lines")
        if not re.search(rf"^#+ Iteration {i} summary", text, re.M):
            need.append(f"the `### Iteration {i} summary` block")
        if need:
            out.append(f"iteration {i}: " + ", ".join(need))
    if not re.search(r"^## Review and Fix Report", text, re.M):
        out.append("the Final Report (`## Review and Fix Report`)")
    else:
        report = text[re.search(r"^## Review and Fix Report", text, re.M).start():]
        gone = [s for s in REPORT_SECTIONS if not re.search(rf"^{re.escape(s)}\b", report, re.M)]
        if gone:
            out.append("Final Report sections " + ", ".join(f"`{s}`" for s in gone))
    return out


def main() -> None:
    data = json.load(sys.stdin)
    cmd = (data.get("tool_input") or {}).get("command") or ""
    sid = data.get("session_id") or ""
    if not sid or not _deletes_marker(cmd, sid):
        return
    if re.search(r"(^|[\s;&|])RUN_LOG_OK=", cmd):
        return
    marker = os.path.join(MARKER_DIR, f".active-{sid}")
    with open(marker) as fh:
        run_log = fh.read().strip()
    with open(run_log, errors="replace") as fh:
        text = fh.read()
    missing = _missing(text)
    if not missing:
        return
    reason = (
        f"The run log {run_log} is missing what review-and-fix requires before the run closes:\n  "
        + "\n  ".join(missing[:12])
        + (f"\n  ... and {len(missing) - 12} more" if len(missing) > 12 else "")
        + "\nWrite each from what the run already did, per SUMMARY.md and FINAL-REPORT.md, then "
        "delete the marker again. A scoring record that cannot exist because the findings were "
        "never scored means they were acted on unscored, which the Final Report must say. If a gap "
        "is genuinely unrecoverable, say which and rerun the delete prefixed with RUN_LOG_OK=1, "
        "which puts it in front of the user."
    )
    json.dump(
        {"hookSpecificOutput": {"hookEventName": "PreToolUse", "permissionDecision": "deny",
                                "permissionDecisionReason": reason}},
        sys.stdout,
    )


if __name__ == "__main__":
    try:
        main()
    except Exception:
        pass
