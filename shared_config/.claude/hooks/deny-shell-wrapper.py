#!/usr/bin/env python3
# PreToolUse hook: deny wrapping a command in `bash -c` / `sh -c` / `zsh -c`.
#
# The Bash tool already runs each command in a shell, so re-invoking a shell
# with -c is redundant. Worse, it hides the real command inside an opaque
# string: the permission allowlist and the other PreToolUse hooks (cd, git,
# gh) all key off the FIRST token of the command, which for `bash -c '...'`
# is just `bash`. So `bash -c 'cd /repo && git push'` slips past the cd and
# git hooks and the allowlist entirely. Deny it and tell the agent to run the
# command directly (and to `cd` as its own Bash call, since the tool's working
# directory persists across calls).
#
# Only a LEADING shell -c is denied (the shell is the first token). Embedded
# uses such as `timeout bash -c ...` or `xargs ... bash -c ...` are left alone
# — they are not the evasion pattern and have legitimate uses. Running a script
# file (`bash deploy.sh`) is also fine: no -c, no hidden command string.

import json
import os
import shlex
import sys

SHELLS = {"bash", "sh", "zsh", "dash", "ksh"}

DENY_REASON = (
    "Don't wrap commands in `bash -c '...'` (or `sh -c` / `zsh -c`). The Bash tool "
    "already runs in a shell, so it is redundant — and it hides the real command "
    "inside an opaque string, so the permission allowlist and the cd / git / gh "
    "hooks (which all key off the first token) never see it. Run the command "
    "directly. To work in another directory, run `cd <path>` as its own Bash call "
    "first — the tool's working directory persists across calls."
)


def emit_deny(reason: str) -> None:
    json.dump(
        {
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "deny",
                "permissionDecisionReason": reason,
            }
        },
        sys.stdout,
    )


def uses_dash_c(args) -> bool:
    # A leading `-c` — or a short-flag cluster containing c, e.g. `-lc`, `-ec` —
    # puts the shell in command-string mode: the next argument is a command to
    # run. The first non-option token means the shell is running a script/file,
    # not a -c command string, so it is not the wrapper pattern.
    for a in args:
        if a == "--" or not a.startswith("-"):
            return False
        if a.startswith("--"):
            continue
        if "c" in a[1:]:
            return True
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
    if os.path.basename(tokens[0]) not in SHELLS:
        return
    if uses_dash_c(tokens[1:]):
        emit_deny(DENY_REASON)


if __name__ == "__main__":
    main()
