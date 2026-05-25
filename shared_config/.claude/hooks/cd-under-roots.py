#!/usr/bin/env python3
# PreToolUse hook: auto-allow `cd` into directories under configured roots.
#
# Reads PreToolUse JSON from stdin. If the Bash command is a simple `cd <path>`
# and the resolved target lies under one of the allowed roots (/tmp,
# $WORKTREES_ROOT, $REPOS_ROOT), emits permissionDecision="allow" to skip the
# prompt. Otherwise stays silent so the `Bash(cd *)` ask rule fires.
#
# Falls through to ask when any of:
#   - the command contains a shell separator (;, &, &&, ||, |, |&) —
#     we can't reason about chained commands, so let the user confirm
#   - the target uses unexpanded shell substitution ($var, $(...), `...`)
#   - the target is a relative path (session cwd isn't reliably available)
#   - the resolved path doesn't lie under any configured root

import json
import os
import shlex
import sys

CHAIN_TOKENS = {";", "&", "&&", "||", "|", "|&"}
ENV_ROOT_VARS = ("WORKTREES_ROOT", "REPOS_ROOT")
FIXED_ROOTS = ("/tmp",)


def allowed_roots():
    roots = [os.path.realpath(r) for r in FIXED_ROOTS]
    for var in ENV_ROOT_VARS:
        value = os.environ.get(var, "").strip()
        if value:
            roots.append(os.path.realpath(value))
    return roots


def main() -> None:
    try:
        data = json.load(sys.stdin)
    except Exception:
        return

    cmd = (data.get("tool_input") or {}).get("command") or ""
    if "$" in cmd or "`" in cmd:
        return

    try:
        lex = shlex.shlex(cmd, posix=True, punctuation_chars=True)
        lex.whitespace_split = True
        tokens = list(lex)
    except ValueError:
        return

    if any(t in CHAIN_TOKENS for t in tokens):
        return
    if not tokens or tokens[0] != "cd" or len(tokens) < 2:
        return

    # Take the last token as the target so flags like `cd -L /tmp/x` still work.
    target = tokens[-1]
    if target.startswith("~"):
        target = os.path.expanduser(target)
    if not os.path.isabs(target):
        return
    resolved = os.path.realpath(target)

    for root in allowed_roots():
        try:
            if os.path.commonpath([resolved, root]) == root:
                json.dump(
                    {
                        "hookSpecificOutput": {
                            "hookEventName": "PreToolUse",
                            "permissionDecision": "allow",
                            "permissionDecisionReason": f"cd target is inside {root}",
                        }
                    },
                    sys.stdout,
                )
                return
        except ValueError:
            continue


if __name__ == "__main__":
    main()
