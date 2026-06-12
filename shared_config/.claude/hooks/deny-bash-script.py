#!/usr/bin/env python3
# PreToolUse hook: deny executing shell SCRIPT FILES (`bash foo.sh`,
# `sh build.sh`, `zsh run.sh`, any of bash/sh/zsh/dash/ksh/mksh/ash)
# unless the script lives under an allow-listed root. This is the same
# leading-token bypass the sibling hooks close: the allowlist and every
# other gate see only `bash`, while the file can contain anything.
# Inline `-c` strings and heredocs-into-interpreters are already
# deny-shell-wrapper.py's job; this hook covers the script-file shape
# that one deliberately leaves alone.
#
# The escape route: a script under a root listed in the sibling
# deny-bash-script.json (committed) or deny-bash-script.local.json
# (gitignored, per-machine) is ALLOWED outright — meant for repos whose
# test suites are bash files. The ALLOW skips a redirect-target scan on
# purpose: plain `bash` is NOT in sandbox.excludedCommands, so the
# script runs inside the sandbox and writes outside the write roots
# already fail at the kernel.
#
# Decisions (first match wins):
#   1. No shell-family token at command-position          -> silent
#   2. `-c` option cluster on the shell                   -> silent (deny-shell-wrapper owns it)
#   3. Compound command (| && || ; & newline, subshell)   -> DENY (must run alone)
#   4. Leading env assignment (FOO=1 bash x.sh)           -> silent (falls to the prompt;
#      same documented carve-out as deny-awk.py)
#   5. Lone `shell <script>` with script under a root     -> ALLOW
#   6. Any other `shell ...` (bare, stdin, redirect-as-
#      script, script outside the roots)                  -> DENY
#
# Missing or malformed config files contribute no roots (fail closed:
# every script-file invocation denies).

import json
import os
import re
import shlex
import sys

SHELL_RE = re.compile(r"^(?:bash|sh|zsh|dash|ksh|mksh|ash)$")
# Superset of the sibling hooks' separator conventions so a shell after
# any chaining/grouping token counts as command-position.
CMD_SEPARATORS = frozenset({";", ";;", "|", "|&", "&", "&&", "||", "(", "{"})

DENY_OUTSIDE = (
    "Don't execute shell script files (`bash <file>`, `sh`, `zsh`, ...) — "
    "the allowlist and the sibling gates only see the interpreter token, "
    "not what the file does. Run the script's steps directly as separate "
    "Bash calls instead. If this directory legitimately holds runnable "
    "scripts (e.g. a bash-file test suite), add its absolute path to "
    "`allowedRoots` in deny-bash-script.json (committed) or "
    "deny-bash-script.local.json (this machine only), next to the hooks."
)

DENY_COMPOUND = (
    "Shell script files must run ALONE — no pipe (|), chain (&&, ||, ;, "
    "newline), or subshell. Run `bash <script> > /tmp/out.txt` first, "
    "then process the file in a SEPARATE command."
)


def _config_dir():
    return os.environ.get("DENY_BASH_SCRIPT_DIR") or os.path.dirname(os.path.realpath(__file__))


def _allowed_roots():
    d = _config_dir()
    roots = []
    for name in ("deny-bash-script.json", "deny-bash-script.local.json"):
        try:
            with open(os.path.join(d, name)) as f:
                data = json.load(f)
        except (OSError, ValueError):
            continue
        for r in data.get("allowedRoots", []):
            if isinstance(r, str) and r.strip():
                real = os.path.realpath(os.path.expanduser(r.strip()))
                if real not in roots:
                    roots.append(real)
    return roots


def under_root(target, cwd, roots):
    path = os.path.expanduser(target)
    if not os.path.isabs(path):
        path = os.path.join(cwd, path)
    try:
        real = os.path.realpath(path)
    except OSError:
        return False
    for root in roots:
        try:
            if os.path.commonpath([real, root]) == root:
                return True
        except ValueError:
            continue
    return False


def has_unquoted_newline(cmd):
    in_single = in_double = False
    i, n = 0, len(cmd)
    while i < n:
        ch = cmd[i]
        if ch == "\\" and not in_single and i + 1 < n:
            i += 2
            continue
        if ch == "'" and not in_double:
            in_single = not in_single
        elif ch == '"' and not in_single:
            in_double = not in_double
        elif ch == "\n" and not in_single and not in_double:
            return True
        i += 1
    return False


def emit(decision, reason):
    json.dump(
        {"hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": decision,
            "permissionDecisionReason": reason,
        }},
        sys.stdout,
    )


def main():
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
        return

    # First shell-family token at command-position. Plain occurrences as
    # arguments (`echo bash`, `which bash`) and assignment-prefixed forms
    # (`FOO=1 bash x.sh`) don't sit at command-position and fall through
    # to the normal permission flow.
    pos = None
    for i, t in enumerate(tokens):
        if SHELL_RE.match(os.path.basename(t)) and (i == 0 or tokens[i - 1] in CMD_SEPARATORS):
            pos = i
            break
    if pos is None:
        return

    # Defer `-c` clusters (-c, -lc, -ec, ...) to deny-shell-wrapper.py so
    # the agent gets its richer inline-code reason instead of this one.
    for t in tokens[pos + 1:]:
        if t in CMD_SEPARATORS:
            break
        if t == "--":
            break
        if t.startswith("-") and not t.startswith("--") and "c" in t[1:]:
            return

    if any(t in CMD_SEPARATORS for t in tokens) or has_unquoted_newline(cmd.strip()):
        emit("deny", DENY_COMPOUND)
        return

    # Single simple command with the shell leading. Find the script: the
    # first non-option argument. A redirect/heredoc token in that slot
    # means the "script" comes from stdin — no file to vet, so deny.
    script = None
    args = tokens[1:]
    i = 0
    while i < len(args):
        a = args[i]
        if a == "--":
            script = args[i + 1] if i + 1 < len(args) else None
            break
        if a.startswith("-") and len(a) > 1:
            i += 1
            continue
        script = a
        break
    if not script or script[0] in "<>&":
        emit("deny", DENY_OUTSIDE)
        return

    cwd = (data.get("cwd") or "").strip() or os.getcwd()
    if under_root(script, cwd, _allowed_roots()):
        emit("allow", "shell script under an allowed root")
        return
    emit("deny", DENY_OUTSIDE)


if __name__ == "__main__":
    main()
