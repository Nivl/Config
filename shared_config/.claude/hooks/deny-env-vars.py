#!/usr/bin/env python3
# PreToolUse hook: deny Bash commands that EXPAND a banned environment
# variable. Currently only $TMPDIR: in this setup it points at the
# macOS per-user temp dir (/var/folders/...), which the sandbox denies
# writing — so a command that honors $TMPDIR dies at the kernel with a
# confusing "operation not permitted" instead of using the real write
# roots. Spelling the path out (/tmp/claude/..., /tmp/...) keeps the
# target visible to the permission gates and the sandbox alike.
#
# BANNED_VARS is the extension point — add a name there (plus tests in
# tests/deny_env_vars_test.sh) to ban another variable.
#
# The scan tracks quote state the same way deny-command-substitution.py
# does, so it only fires where the shell would actually expand the
# variable. These stay allowed: single-quoted literals (`rg '\$TMPDIR'`),
# escaped forms (`echo \$TMPDIR`, `echo "\$TMPDIR"`), assignments that
# only SET the variable (`TMPDIR=/tmp/claude cmd`), and longer names
# that merely share the prefix (`$TMPDIRS`).

import json
import sys

BANNED_VARS = frozenset({"TMPDIR"})


def _name_at(cmd, j):
    # Extract the variable name starting at cmd[j] (just past `$` or
    # `${`). `#` and `!` are length/indirection operators that still
    # read the variable, so skip them before collecting name chars.
    n = len(cmd)
    while j < n and cmd[j] in "#!":
        j += 1
    k = j
    while k < n and (cmd[k].isalnum() or cmd[k] == "_"):
        k += 1
    return cmd[j:k], k


def banned_expansion(cmd):
    in_single = in_double = False
    i, n = 0, len(cmd)
    while i < n:
        ch = cmd[i]
        if in_single:
            if ch == "'":
                in_single = False
            i += 1
            continue
        if ch == "\\" and i + 1 < n:
            i += 2
            continue
        if ch == "'" and not in_double:
            in_single = True
            i += 1
            continue
        if ch == '"':
            in_double = not in_double
            i += 1
            continue
        if ch == "$":
            j = i + 1
            if j < n and cmd[j] == "{":
                name, k = _name_at(cmd, j + 1)
            else:
                name, k = _name_at(cmd, j)
            if name in BANNED_VARS:
                return name
            i = max(k, i + 1)
            continue
        i += 1
    return None


def main():
    try:
        data = json.load(sys.stdin)
    except Exception:
        return
    cmd = (data.get("tool_input") or {}).get("command") or ""
    if not cmd:
        return
    name = banned_expansion(cmd)
    if name is None:
        return
    json.dump(
        {"hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "deny",
            "permissionDecisionReason": (
                f"`${name}` is banned: use the full path directly instead "
                "of using an env variable."
            ),
        }},
        sys.stdout,
    )


if __name__ == "__main__":
    main()
