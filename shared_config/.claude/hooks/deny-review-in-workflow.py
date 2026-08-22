#!/usr/bin/env python3
# PreToolUse hook: deny a Workflow whose body runs a fan-out review skill.
#
# A workflow agent (one spawned by the Workflow tool's agent() call) has no
# Agent tool. So a skill that fans out into sub-agents has nothing to fan out
# with inside one. in-depth-review cannot launch its eight to twelve reviewer
# roles. review-and-fix cannot launch its reviewers. pr-review cannot launch its
# finders. Each aborts rather than returning a review, and this hook stops the
# call before it gets that far. Without either guard the pass would collapse to a
# single reader and report degraded coverage in the shape a completed review
# uses, and the caller would move on. That is the failure both guards exist to
# prevent.
#
# A subagent keeps its own Agent tool, so a nested sub-agent launch is fine.
# pr-review launching in-depth-review as a sub-agent still gets the full role
# fan-out. Only the workflow boundary takes the tool away. The mirror-image fact
# is that a subagent typically has no Workflow tool. Both are silent capability
# losses across an agent boundary.
#
# gh-style-review is deliberately absent from DENIED_SKILLS. It is a single pass
# and spawns nothing, so a workflow does not degrade it. Do not "helpfully" add
# it. The same goes for any other leaf skill that reads and reports without
# fanning out.
#
# What gets matched is a mention of the name, not a provable call. A workflow
# that only warns "do NOT run in-depth-review from inside this workflow" is
# denied. That over-blocking is deliberate. A text scan cannot tell a call from a
# reference, and a guard that errs loud beats one that errs silent.
#
# The scan reads the workflow's CODE and never its DATA. `script` is the inline
# body. `scriptPath` is a persisted body, and a Workflow can be re-invoked by
# path after an edit, so checking only `script` leaves a hole. `name` is a saved
# workflow, matched both as the bare string and as whatever resolves under
# .claude/workflows/. `args` is deliberately NOT scanned. It carries a ticket
# description, a comment thread, PR titles, and commit subjects, all prose
# written by other people for another purpose. This repo's own history holds a
# commit subject naming work-on, and scanning args made that one subject enough
# to deny work-on's own Step 2 validation call. A name in data is not a call.
# Every file read is best-effort. A missing or unreadable path falls through to
# silence and never becomes a deny.
#
# The escape hatch is a `no-fanout-ok:` marker with a reason after it, in the
# workflow body. It is for a workflow that must name these skills without
# running them, such as one that audits or edits the skills themselves, where no
# wording change can avoid the name. The deny reason does not advertise it, so
# reaching for it takes knowing it is here.

import json
import os
import re
import sys

# Skills whose worth comes from a fan-out. Three of them launch reviewer roles of
# their own. work-on hands its review step to review-and-fix, so it inherits the
# same loss one level down. A workflow agent lets none of them reach the
# sub-agents they assume.
DENIED_SKILLS = ("in-depth-review", "review-and-fix", "pr-review", "work-on")

# Agent types rather than skills. Both wrappers run in-depth-review, so they fan
# out exactly as the skill does and a workflow agent breaks them the same way.
# Under the word-boundary match neither one reads as `pr-review`, so each needs
# its own entry, and the longer name does not cover the shorter one either. The
# other agent types under .claude/agents are absent on purpose. Each of those is
# a leaf that reads and reports, and a workflow script's own agent() calls run at
# the orchestrator level, so launching a leaf from a script is the way to
# hand-roll a fan-out inside a workflow.
DENIED_AGENT_TYPES = ("pr-review-finder-indepth", "pr-review-finder-indepth-deep")

# A name counts only on a word boundary, so an identifier that merely contains
# one is left alone. The two boundaries are deliberately asymmetric. The one
# before excludes letters and digits only, so a leading hyphen still matches and
# `nightly-pr-review` denies. The one after excludes a hyphen as well, so a
# trailing hyphen does not match and `work-on-validate` stays silent. That
# asymmetry is what lets a workflow of its own share a skill's prefix. Do not add
# `-` to BOUNDARY_BEFORE, and do not drop it from BOUNDARY_AFTER, without
# re-reading both of those cases.
BOUNDARY_BEFORE = r"(?<![a-z0-9])"
BOUNDARY_AFTER = r"(?![a-z0-9-])"


def compile_patterns(names):
    # Longest name first, so a deny reports the most specific match.
    return tuple(
        (name, re.compile(BOUNDARY_BEFORE + re.escape(name) + BOUNDARY_AFTER))
        for name in sorted(names, key=len, reverse=True)
    )


DENIED_PATTERNS = compile_patterns(DENIED_SKILLS + DENIED_AGENT_TYPES)

# The marker carries a reason on the same line, so a bare token is not a bypass.
# Matching any whitespace here would let the next line of code serve as the
# reason, which is every bare marker.
HATCH_PATTERN = re.compile(r"no-fanout-ok:[ \t]*\S")

# Reading a body that is not a regular file can block forever (a FIFO) or never
# end (/dev/zero), and a huge one exhausts memory. Both would stall the tool call
# this hook gates, so the read is capped and takes regular files only.
MAX_READ_BYTES = 1_000_000

# Where a saved workflow's body is kept, relative to a base directory.
WORKFLOW_DIR = os.path.join(".claude", "workflows")

# A saved name may or may not carry its extension, so try the plausible ones.
NAME_EXTS = ("", ".js", ".mjs", ".ts", ".md", ".json")

REASON_TEMPLATE = (
    "Don't run `{target}` inside a Workflow. A workflow agent has no Agent tool, "
    "so a skill that fans out into sub-agents has nothing to fan out with. "
    "in-depth-review cannot launch its reviewer roles. review-and-fix cannot "
    "launch its reviewers. pr-review cannot launch its finders. work-on hands "
    "its review step to review-and-fix, so it inherits the same loss. What comes "
    "back is an abort with no coverage, not a review. Invoke these from the main "
    "thread instead. What does belong inside a workflow is a leaf agent that "
    "spawns nothing, such as a single review lens, a telemetry probe, or an "
    "in-depth-review-role reviewer the script launches itself."
)


def read_best_effort(path):
    try:
        if not os.path.isfile(path):
            return ""
        with open(path, "r", encoding="utf-8", errors="replace") as handle:
            return handle.read(MAX_READ_BYTES)
    except Exception:
        return ""


def workflow_bases(data):
    # A saved workflow resolves against the session cwd or the home dir. Both
    # are tried because neither is guaranteed to be the one that holds it.
    bases = []
    cwd = (data.get("cwd") or "").strip()
    if cwd:
        bases.append(cwd)
    home = os.environ.get("HOME", "").strip()
    if home:
        bases.append(home)
    return bases


def saved_workflow_texts(name, data):
    # A saved workflow is a plain filename inside .claude/workflows. Anything
    # with a separator in it, or an absolute path, would join away the base and
    # turn this lookup into a read of any file on disk.
    if os.path.isabs(name) or os.sep in name or name in (".", ".."):
        return []
    texts = []
    for base in workflow_bases(data):
        for ext in NAME_EXTS:
            texts.append(read_best_effort(os.path.join(base, WORKFLOW_DIR, name + ext)))
    return texts


def candidate_texts(tool_input, data):
    texts = []

    script = tool_input.get("script")
    if isinstance(script, str):
        texts.append(script)

    script_path = tool_input.get("scriptPath")
    if isinstance(script_path, str) and script_path.strip():
        texts.append(read_best_effort(script_path))

    name = tool_input.get("name")
    if isinstance(name, str) and name.strip():
        texts.append(name)
        texts.extend(saved_workflow_texts(name, data))

    return texts


def normalise(text):
    # Lowercasing catches `In-Depth-Review`. A slash prefix needs no work,
    # because `/pr-review` already contains the name. Underscores are left as
    # they are. Mapping them to hyphens made `PR_REVIEW` and `def work_on` read
    # as skill names, and nobody spells a skill in snake_case to invoke it.
    return text.lower()


def matched_target(texts):
    for text in texts:
        if not text:
            continue
        haystack = normalise(text)
        for name, pattern in DENIED_PATTERNS:
            if pattern.search(haystack):
                return name
    return None


def emit_deny(target):
    json.dump(
        {
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "deny",
                "permissionDecisionReason": REASON_TEMPLATE.format(target=target),
            }
        },
        sys.stdout,
    )


def main():
    try:
        data = json.load(sys.stdin)
    except Exception:
        return
    tool_input = data.get("tool_input") or {}
    if not isinstance(tool_input, dict):
        return
    texts = candidate_texts(tool_input, data)
    if any(HATCH_PATTERN.search(text) for text in texts if text):
        return
    target = matched_target(texts)
    if target:
        emit_deny(target)


if __name__ == "__main__":
    main()
