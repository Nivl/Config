#!/usr/bin/env python3
# PreToolUse hook: deny `git -C <path>`.
#
# The top-level -C flag runs git as if it had been started in <path> — a way
# to operate on a repo from a different working directory. Project policy is
# to `cd` into the target repo first (as its own Bash call, since the Bash
# tool's CWD persists across calls), then run a plain `git` command. This
# hook denies any git invocation that uses the top-level -C so the agent gets
# a clear, actionable reason instead of silently succeeding.
#
# Only the TOP-LEVEL -C (the option that appears before the subcommand) is
# denied. Subcommand-level -C — `git diff -C`, `git log -C`, `git blame -C`
# (copy/rename detection) — is left alone; it has nothing to do with changing
# directories.

import json
import shlex
import sys

KNOWN_GIT_PATHS = {
    "/usr/bin/git",
    "/opt/homebrew/bin/git",
    "/usr/local/bin/git",
    "/opt/local/bin/git",
}

# Top-level git options that consume a following argument. When the
# pre-subcommand scan skips one of these, it must skip its value too, or the
# value could be mistaken for the subcommand and end the scan early.
VALUE_OPTS = {"-C", "-c", "--git-dir", "--work-tree", "--namespace", "--super-prefix"}

DENY_REASON = (
    "`git -C <path>` is not allowed. `cd` into the target repo first — as its "
    "own Bash call, since the Bash tool's working directory persists across "
    "calls — then run the plain `git` command without -C."
)


def uses_top_level_dash_c(args) -> bool:
    i = 0
    while i < len(args):
        a = args[i]
        if a == "-C" or (a.startswith("-C") and len(a) > 2):
            return True
        if not a.startswith("-"):
            # First non-option token is the subcommand; any -C after this is
            # subcommand-level (e.g. `git diff -C`) and not our concern.
            return False
        # Some other top-level option. If it takes a separate-arg value, skip
        # that value too so it isn't misread as the subcommand.
        i += 2 if a in VALUE_OPTS else 1
    return False


def main() -> None:
    try:
        data = json.load(sys.stdin)
    except Exception:
        return

    cmd = (data.get("tool_input") or {}).get("command") or ""
    try:
        lex = shlex.shlex(cmd, posix=True, punctuation_chars=True)
        lex.whitespace_split = True
        tokens = list(lex)
    except ValueError:
        return

    if not tokens:
        return
    if tokens[0] != "git" and tokens[0] not in KNOWN_GIT_PATHS:
        return

    if uses_top_level_dash_c(tokens[1:]):
        json.dump(
            {
                "hookSpecificOutput": {
                    "hookEventName": "PreToolUse",
                    "permissionDecision": "deny",
                    "permissionDecisionReason": DENY_REASON,
                }
            },
            sys.stdout,
        )


if __name__ == "__main__":
    main()
