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
# only SET the variable (`TMPDIR=/tmp/claude cmd`), longer names that
# merely share the prefix (`$TMPDIRS`), bare `$!TMPDIR` / `$$TMPDIR`
# (the shell reads the `$!` / `$$` special parameter there, never
# TMPDIR), and ANSI-C strings (`$'...'` — single-quote semantics with
# backslash escapes, nothing inside expands). Bare `$#TMPDIR` DENIES:
# the Bash tool executes via zsh, where that is csh-style length
# expansion and does read the variable (bash would read `$#` instead).
# zsh's flag and operator forms (`${(P)TMPDIR}`, `${=TMPDIR}`,
# `$~TMPDIR`, ...) read the variable too and also deny.
#
# Known limitation: the scanner has no heredoc awareness. A quoted-
# delimiter heredoc body (`cat <<'EOF' ... $TMPDIR ... EOF`) is literal
# to the shell but still denies, and quote characters inside an
# unquoted heredoc body corrupt the tracking the other way (an
# apostrophe in the body can hide a later live expansion).
# deny-command-substitution.py shares the same blind spot, the OS
# sandbox stays the enforcement floor for writes, and the Write tool
# covers writing a file that needs the literal text.

import json
import re
import sys

BANNED_VARS = frozenset({"TMPDIR"})


def _name_at(cmd, j, braced):
    # Extract the variable name starting at cmd[j] (just past `$` or
    # `${`). The skip sets model zsh, the shell the Bash tool executes:
    # inside braces, a `(...)` flag group (`${(P)NAME}`, `${(U)NAME}`)
    # and the `#`/`!`/`=`/`~`/`^` operators all still read the variable;
    # after a bare `$`, zsh's `#` (csh-style length) and the `=`/`~`/`^`
    # word modifiers read it too, while `!` is the last-background-pid
    # special parameter, so the name after it is literal text the shell
    # never reads (`$!` and `$$` mean the same thing in bash and zsh).
    n = len(cmd)
    if braced and j < n and cmd[j] == "(":
        # Match parens by depth: a zsh flag argument can itself be
        # paren-delimited (`${(j(,))NAME}`, `${(s(,))NAME}`), so jumping
        # to the first `)` would stop inside the group and miss NAME.
        depth = 1
        j += 1
        while j < n and depth:
            if cmd[j] == "(":
                depth += 1
            elif cmd[j] == ")":
                depth -= 1
            j += 1
        if depth:
            return "", j
    ops = "#!=~^" if braced else "#=~^"
    while j < n and cmd[j] in ops:
        j += 1
    k = j
    while k < n and (cmd[k].isalnum() or cmd[k] == "_"):
        k += 1
    return cmd[j:k], k


def banned_expansion(cmd):
    # The shell's lexer removes backslash-newline line continuations
    # before tokenizing, joining `$TMP\<newline>DIR` back into one name.
    # Mirror that first so a split name can't slip past the scan — but
    # only when the backslash is itself unescaped (an even run of
    # backslashes before it means the shell sees a literal backslash
    # plus a real newline, and what follows can still expand). The join
    # also fires inside single quotes, where the joined text still sits
    # in a skipped region, so detection is unchanged there.
    cmd = re.sub(r"(?<!\\)((?:\\\\)*)\\\n", r"\1", cmd)
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
            if j < n and cmd[j] == "$":
                # `$$` is the PID special parameter: the shell consumes
                # both characters, so a name right after is literal text.
                # ($$$TMPDIR still denies — the third `$` re-enters here.)
                i = j + 1
                continue
            if j < n and cmd[j] == "'" and not in_double:
                # ANSI-C string $'...': single-quote semantics plus
                # backslash escapes — nothing inside expands, and \'
                # does not close the string. Inside double quotes the
                # `$` stays literal instead, so this branch must not
                # fire there (the content still expands).
                j += 1
                while j < n and cmd[j] != "'":
                    if cmd[j] == "\\" and j + 1 < n:
                        j += 1
                    j += 1
                i = j + 1
                continue
            if j < n and cmd[j] == "{":
                name, k = _name_at(cmd, j + 1, braced=True)
            else:
                name, k = _name_at(cmd, j, braced=False)
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
                "of using an env variable — temp files go under "
                "/tmp/claude/ (or /tmp/)."
            ),
        }},
        sys.stdout,
    )


if __name__ == "__main__":
    main()
