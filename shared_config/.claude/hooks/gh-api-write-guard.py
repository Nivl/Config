#!/usr/bin/env python3
# PreToolUse hook: auto-allow `gh api` read-only calls.
#
# Reads PreToolUse JSON from stdin. If the Bash command is a clean
# `gh api` invocation with no write semantics, emits a permission
# decision of "allow" to skip the prompt. Otherwise stays silent so
# the existing `Bash(gh api *)` ask rule fires.
#
# A call is treated as a WRITE (and NOT auto-allowed) when any of:
#   - the command contains a shell separator (;, &, &&, ||, |, |&) —
#     we can't reason about chained commands, so fall through
#   - explicit method flag: -X / --method = POST|PUT|PATCH|DELETE
#   - any field flag (-f / -F / --field / --raw-field) UNLESS the
#     method is explicitly -X GET (gh defaults to POST when -f present)
#   - the endpoint is `graphql` (always POST; mutations vs queries
#     are indistinguishable without parsing the body)

import json
import shlex
import sys

WRITE_METHODS = {"POST", "PUT", "PATCH", "DELETE"}
FIELD_FLAGS = {"-f", "-F", "--field", "--raw-field"}
CHAIN_TOKENS = {";", "&", "&&", "||", "|", "|&"}


def main() -> None:
    try:
        data = json.load(sys.stdin)
    except Exception:
        return

    cmd = (data.get("tool_input") or {}).get("command") or ""
    if not cmd.strip():
        return

    stripped = cmd.lstrip()
    if not (
        stripped.startswith("gh api ")
        or stripped.startswith("/opt/homebrew/bin/gh api ")
    ):
        return

    try:
        lex = shlex.shlex(cmd, posix=True, punctuation_chars=True)
        lex.whitespace_split = True
        tokens = list(lex)
    except ValueError:
        return

    if any(t in CHAIN_TOKENS for t in tokens):
        return

    try:
        api_idx = tokens.index("api")
    except ValueError:
        return
    args = tokens[api_idx + 1 :]
    if not args:
        return

    is_write = False
    explicit_get = False

    i = 0
    while i < len(args):
        a = args[i]
        if a in ("-X", "--method") and i + 1 < len(args):
            v = args[i + 1].upper()
            if v in WRITE_METHODS:
                is_write = True
            elif v == "GET":
                explicit_get = True
            i += 2
            continue
        if a.startswith("-X") and len(a) > 2:
            v = a[2:].upper()
            if v in WRITE_METHODS:
                is_write = True
            elif v == "GET":
                explicit_get = True
            i += 1
            continue
        if a.startswith("--method="):
            v = a.split("=", 1)[1].upper()
            if v in WRITE_METHODS:
                is_write = True
            elif v == "GET":
                explicit_get = True
            i += 1
            continue
        i += 1

    if args[0] == "graphql":
        is_write = True

    if not explicit_get:
        for a in args:
            if a in FIELD_FLAGS:
                is_write = True
                break

    if is_write:
        return

    json.dump(
        {
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "allow",
                "permissionDecisionReason": "gh api read-only call",
            }
        },
        sys.stdout,
    )


if __name__ == "__main__":
    main()
