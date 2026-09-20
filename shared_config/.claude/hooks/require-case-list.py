#!/usr/bin/env python3
# PreToolUse hook (Bash): inside a review-and-fix run, deny a `git commit` whose
# staged diff changes code when the run log has no case list for the current
# iteration.
#
# review-and-fix's IMPLEMENTER.md section 0 says a logic group writes three
# `cases iter=<N> ...` lines before any edit, the last carrying `reaches=`, the
# finding's case and each neighbour the change touches. That list exists
# because the self-inflicted logic fixes on the logged runs were the neighbour
# the finding did not name. On the run that made the rule's case, the list was
# written in iteration 1 and skipped in iterations 2 to 4, and each of those
# iterations fixed a bug the previous one's fix had introduced in the same two
# files. A reading instruction decays with the iteration count. A deny does
# not.
#
# When the iteration's `reaches=` names paired state (release, lock, unlock,
# retry, transaction, rollback, acquire), an `invariant=` line is required as
# well, since the measured fix chains were one exit of such a region per
# iteration with the property never written down.
#
# Scope. The hook does nothing unless
#   ~/.melvin/config/logs/review-and-fix/.active-<session_id>
# exists, which review-and-fix writes at Step 0 and deletes at the Final
# Report, so a commit outside a run is untouched. The current iteration comes
# from the log's own markers, `t0 iter=N`, `## Iteration N` and `stamps: iter=N`,
# and a bare `t0=` after the last stamps line opens the next one, since the
# logs write any of those. Code means an added or removed
# line in a source file that is not blank and not a comment. A diff confined to
# prose files, test files, or comment lines passes, since those groups write no
# list. The check is per iteration and not per commit, so the pre-commit
# follow-up commit passes on the list the iteration already wrote.
#
# The override is a CASE_LIST_OK=1 prefix, which passes here and, like
# PROSE_CLAIMS_OK, falls to the normal permission prompt because git is
# sandbox-excluded and the variable is not in safe_assignments. Any failure is
# silent. A broken check must not block every commit.

import json
import os
import re
import shlex
import subprocess
import sys

MARKER_DIR = os.path.expanduser("~/.melvin/config/logs/review-and-fix")
KNOWN_GIT_PATHS = {"git", "/usr/bin/git", "/opt/homebrew/bin/git", "/usr/local/bin/git", "/opt/local/bin/git"}
VALUE_OPTS = {"-C", "-c", "--git-dir", "--work-tree", "--namespace", "--super-prefix"}
ASSIGN = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*=")

CODE_EXTS = {".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".py", ".go", ".rb", ".java", ".kt", ".kts",
             ".swift", ".rs", ".c", ".h", ".cc", ".cpp", ".hpp", ".cs", ".scala", ".php", ".dart", ".sql", ".sh"}
COMMENT_STARTS = ("//", "#", "/*", "*", "--")
# A test file is named as one. A helper or fixture under a test directory is
# logic, per review-and-fix's CLASSIFIER.md, so the directory alone is not enough.
TEST_PATH = re.compile(r"\.(test|spec)\.[a-z]+$|_test\.[a-z]+$|(^|/)test_[^/]+\.py$|(^|/)[^/]*\.(test|spec)\.[a-z]+\.snap$")


def _tokens(s):
    try:
        lex = shlex.shlex(s, posix=True, punctuation_chars=True)
        lex.whitespace_split = True
        return list(lex)
    except ValueError:
        return []


def _is_git_commit(cmd):
    for line in cmd.splitlines():
        tokens = _tokens(line)
        starts = [0] + [i + 1 for i, t in enumerate(tokens) if t in {";", "&&", "||", "|"}]
        for s in starts:
            while s < len(tokens) and ASSIGN.match(tokens[s]) and tokens[s] not in KNOWN_GIT_PATHS:
                s += 1
            if s >= len(tokens) or tokens[s] not in KNOWN_GIT_PATHS:
                continue
            i = s + 1
            while i < len(tokens) and tokens[i].startswith("-"):
                i += 2 if tokens[i] in VALUE_OPTS else 1
            if i < len(tokens) and tokens[i] == "commit":
                return True
    return False


def _staged_diff(cwd):
    try:
        out = subprocess.run(["git", "diff", "--cached", "-U0", "--no-color", "--no-ext-diff"],
                             cwd=cwd, capture_output=True, text=True, timeout=20)
    except Exception:
        return ""
    return out.stdout if out.returncode == 0 else ""


def _changes_code(diff):
    path = None
    prev = ""
    for raw in diff.splitlines():
        if raw.startswith("+++ ") and prev.startswith("--- "):
            p = raw[4:].strip()
            if p == "/dev/null":
                # A deleted file. Its removed lines are code too, so the path
                # comes from the `--- a/` side.
                q = prev[4:].strip()
                path = None if q == "/dev/null" else re.sub(r"^a/", "", q)
            else:
                path = re.sub(r"^b/", "", p)
            prev = raw
            continue
        prev = raw
        if raw.startswith(("--- ", "diff ", "index ", "@@")):
            continue
        if path is None or not (raw.startswith("+") or raw.startswith("-")):
            continue
        if os.path.splitext(path)[1].lower() not in CODE_EXTS or TEST_PATH.search(path):
            continue
        body = raw[1:].strip()
        if not body or body.startswith(COMMENT_STARTS):
            continue
        return path
    return None


def _current_iter(log_text):
    # The logs mark an iteration three ways, and a run may use any of them:
    # `t0 iter=N ...` at the start, `## Iteration N` as a heading, and
    # `stamps: iter=N ...` at the end. Some write a bare `t0=<time>` with no
    # iteration number, so a bare t0 after the last stamps line means the next
    # iteration has started. Fail open when none of these is present.
    started = [int(m) for m in re.findall(r"^t0 iter=(\d+)", log_text, re.M)]
    started += [int(m) for m in re.findall(r"^#+ Iteration (\d+)\s*$", log_text, re.M)]
    completed = [int(m) for m in re.findall(r"^stamps: iter=(\d+)", log_text, re.M)]
    cur = max(started + completed) if (started or completed) else None
    last_stamp = max((m.end() for m in re.finditer(r"^stamps: iter=\d+", log_text, re.M)), default=-1)
    bare_t0_after = any(m.start() > last_stamp for m in re.finditer(r"^t0=\S", log_text, re.M))
    if completed and bare_t0_after and (not started or max(started) <= max(completed)):
        cur = max(completed) + 1
    return cur


def main() -> None:
    try:
        data = json.load(sys.stdin)
    except Exception:
        return
    cmd = (data.get("tool_input") or {}).get("command") or ""
    if not _is_git_commit(cmd):
        return
    if any(t.startswith("CASE_LIST_OK=") for t in _tokens(cmd)):
        return
    marker = os.path.join(MARKER_DIR, f".active-{data.get('session_id', '')}")
    try:
        with open(marker) as fh:
            run_log = fh.read().strip()
        with open(run_log) as fh:
            log_text = fh.read()
    except OSError:
        return
    it = _current_iter(log_text)
    if it is None:
        return
    # The value is the quoted text alone, so a later field on the same line
    # cannot leak into it. An unquoted value runs to the next whitespace.
    reaches = re.findall(rf"^cases iter={it} .*?(?:^|\s)reaches=(?:\"([^\"]*)\"|(\S+))", log_text, re.M)
    reaches = [a or b for a, b in reaches]
    if reaches:
        # Paired state needs its property written down beside the exits. The
        # words are the ones the measured chains were about. A bare "claim" is
        # ordinary prose too often, so it counts as paired only beside a release.
        def paired(r):
            if re.search(r"\b(release|unlock|rollback|acquire|transaction|retry|lock)(s|ed|ing|es)?\b", r, re.I):
                return True
            return False
        # The field has to sit outside any quoted value, or "invariant=" quoted
        # inside reaches text would pass for the line.
        has_invariant = any(
            re.search(r"(^|\s)invariant=", re.sub(r"\"[^\"]*\"", '""', ln))
            for ln in re.findall(rf"^cases iter={it} .*$", log_text, re.M)
        )
        if not any(paired(r) for r in reaches) or has_invariant:
            return
        missing_invariant = True
    else:
        missing_invariant = False
    cwd = data.get("cwd") or os.getcwd()
    path = _changes_code(_staged_diff(cwd))
    if not path:
        return
    json.dump(
        {
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "deny",
                "permissionDecisionReason": (
                    (f"This commit changes code ({path}) and iteration {it}'s case list mentions paired state "
                     f"(claim, release, lock, retry, transaction) with no `cases iter={it} ... invariant=\"...\"` line. "
                     "State what must hold on every exit of the region, and list each exit in reaches=, per "
                     "IMPLEMENTER.md section 0. ")
                    if missing_invariant else
                    f"This commit changes code ({path}) and the run log has no case list for iteration {it}. "
                    f"IMPLEMENTER.md section 0: before a logic edit, append `cases iter={it} findings=<ids> changes=\"...\"`, "
                    f"`... keeps=\"...\"` and `... reaches=\"<the finding's case>; <each neighbour>\"` to the run log, "
                    "pin each working neighbour with an assertion, then commit. A prose- or test-only diff does not "
                    "need one. CASE_LIST_OK=1 in front of the command overrides, and puts the commit in front of the user. "
                    f"If no review-and-fix run is in progress in this session, the marker {marker} is stale from an "
                    "earlier run that did not reach its Final Report. Delete it and commit again."
                ),
            }
        },
        sys.stdout,
    )


if __name__ == "__main__":
    main()
