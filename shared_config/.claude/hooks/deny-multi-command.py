#!/usr/bin/env python3
# PreToolUse hook: deny Bash invocations that bundle multiple unrelated
# statements into one tool call via `;` or a bare newline. The point is
# per-command auditability — when the agent batches 5 probes into one
# Bash call, the user gets ONE permission prompt for the whole blob,
# and the other hooks (deny-shell-wrapper, file-ops-under-roots, etc.) can't
# walk the boundary between statements because shlex collapses newlines
# into whitespace and the agent can hide arbitrary chained commands.
#
# Conditional chains (`&&`, `||`) and data pipelines (`|`) stay allowed
# because they express semantics that one-call-at-a-time can't replicate
# (run second only if first succeeded; pipe data from one to the next).
# Plain sequencing (`cmd1; cmd2` or `cmd1\ncmd2`) has no such excuse —
# the agent can issue two separate Bash tool calls instead, and the
# tool's working directory persists across calls so state is shared.
#
# Detection:
#   - tokens contain `;` or `;;` as standalone tokens (shlex+punctuation_
#     chars puts unquoted `;` in its own token; quoted `;` stays inside
#     the quoted token).
#   - raw command contains an unescaped newline outside single/double
#     quotes. Backslash-escaped `\<newline>` (line continuation) is OK.
# Heredoc bodies are not specially excluded — the agent should use the
# Write tool for file content rather than `cat <<EOF`.

import json
import shlex
import sys

REASON_BASE = (
    "Don't bundle multiple statements into one Bash call. Use ONE "
    "command at a time so the permission allowlist and the sibling "
    "hooks can vet each invocation independently. The Bash tool's "
    "working directory persists across calls, so each call shares "
    "state with the previous one.\n"
    "  - conditional chain: `cmd1 && cmd2`  /  `cmd1 || cmd2`\n"
    "  - data pipeline:     `cmd1 | cmd2`\n"
    "  - write a file:      use the Write tool, not `cat <<EOF > file`\n"
    "  - sequence unrelated commands: issue separate Bash tool calls."
)


def has_unquoted_newline(cmd: str) -> bool:
    # Walk raw text, track single/double-quote state, skip backslash
    # escapes (covers `\<newline>` line continuation). Return True on
    # the first newline seen outside any quoting. Heredoc bodies are
    # NOT tracked — false positive on `cat <<EOF\nbody\nEOF` is the
    # intended trade-off (use Write tool instead).
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
        if in_double:
            if ch == "\\" and i + 1 < n:
                i += 2
                continue
            if ch == '"':
                in_double = False
            i += 1
            continue
        if ch == "\\" and i + 1 < n:
            i += 2
            continue
        if ch == "'":
            in_single = True
            i += 1
            continue
        if ch == '"':
            in_double = True
            i += 1
            continue
        if ch == "\n":
            return True
        i += 1
    return False


def emit_deny(detail: str) -> None:
    json.dump(
        {
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "deny",
                "permissionDecisionReason": f"{REASON_BASE}\n\n(Detected: {detail}.)",
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

    try:
        lex = shlex.shlex(cmd, posix=True, punctuation_chars=True)
        lex.whitespace_split = True
        tokens = list(lex)
    except ValueError:
        tokens = []

    # `&` (background) is also a chaining separator: `cmd1 & cmd2` runs
    # cmd1 in the background and cmd2 immediately. `&&` (logical and) and
    # `&>` (redirect) are distinct tokens under punctuation_chars=True so
    # they aren't matched here.
    # `$'...'` ANSI-C strings are not tracked by has_unquoted_newline —
    # multi-line ANSI-C forms are already denied by deny-shell-wrapper.py's
    # R5 rule.
    if ";" in tokens or ";;" in tokens or "&" in tokens:
        emit_deny("`;` or `&` separator chains multiple statements")
        return

    if has_unquoted_newline(cmd):
        emit_deny("unescaped newline outside quotes splits the command into multiple statements")
        return


if __name__ == "__main__":
    main()
