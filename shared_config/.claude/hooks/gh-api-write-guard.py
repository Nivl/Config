#!/usr/bin/env python3
# PreToolUse hook: gate `gh api` calls by read/write semantics.
#
# Reads PreToolUse JSON from stdin. For a clean, single `gh api`
# invocation it emits a permission decision:
#   - read-only call -> "allow" (skip the prompt)
#   - write call     -> "ask"   (force a confirmation prompt)
#
# The hook is self-contained: it does NOT pair with a `Bash(gh api *)`
# ask rule. An ask rule can't be bypassed by a hook "allow", so pairing
# the two would prompt on every read. Emitting "ask" for writes here
# keeps that guard without an ask rule behind it.
#
# Anything that can't be classified cleanly stays silent and falls
# through to the normal permission flow (default-mode prompt):
#   - the command contains a shell separator (;, &, &&, ||, |, |&) —
#     we can't reason about chained commands
#   - the command can't be tokenized
#
# A call is treated as a WRITE when any of:
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


def emit(decision: str, reason: str) -> None:
    json.dump(
        {
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": decision,
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
        emit("ask", "gh api write call — confirm before sending")
        return

    emit("allow", "gh api read-only call")


if __name__ == "__main__":
    main()
