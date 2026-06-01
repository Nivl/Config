#!/usr/bin/env python3
# PreToolUse hook: deny `python3 -m json.tool` and redirect the agent
# to `jq`. The stdlib's json.tool is a fine pretty-printer, but `jq`
# does the same job with a smaller invocation surface (no module path,
# no Python startup cost) and is already allow-listed in settings.json.
# Allow-listing `python3 -m json.tool *` would also need a guard hook to
# scrub chained commands and shell expansion — more work than steering
# the agent at the simpler tool.
#
# Fires when the leading token (basename, version-suffixed allowed) is
# a python interpreter AND its arg stream contains `-m json.tool`
# (separate or packed `-mjson.tool`). Wrappers like `env python3 -m
# json.tool` are not peeled here — they're rare for this specific
# invocation and the agent will be redirected on the simpler form too.

import json
import os
import re
import shlex
import sys

PYTHON_RE = re.compile(r"^python(?:\d+(?:\.\d+)?)?$")
# Tokens that mark a new simple command on their right. Same convention as
# deny-shell-wrapper.py — needed so `cat file.json | python3 -m json.tool`
# (the canonical piped form) trips the rule too.
CMD_SEPARATORS = frozenset({";", "|", "&", "&&", "||", "(", "{"})
# Python flags whose argument lives in the NEXT positional token (e.g.
# `python -W default -m json.tool x` or `python -X faulthandler -m
# json.tool x`). Without this, the option-walker would bail on `default`
# / `faulthandler` and miss the `-m json.tool` that follows.
ARG_TAKING_FLAGS = frozenset({"-W", "-X"})

REASON = (
    "Don't use `python3 -m json.tool` to inspect or pretty-print JSON — "
    "use `jq` instead. `jq .` is equivalent for pretty-printing, faster, "
    "and is already in the permission allowlist. Examples: "
    "`jq . file.json`, `jq -r '.field' file.json`, or "
    "`cat file.json | jq .`."
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
        return

    # Walk every command-position token. If a python interpreter sits at
    # one (start of stream or after a separator), check its arg run for
    # `-m json.tool` (separate or packed). Catches the direct form and
    # the piped `cat file.json | python3 -m json.tool` form alike.
    for i, t in enumerate(tokens):
        if not PYTHON_RE.match(os.path.basename(t)):
            continue
        if i > 0 and tokens[i - 1] not in CMD_SEPARATORS:
            continue
        rest = tokens[i + 1:]
        skip_next = False
        for j, a in enumerate(rest):
            if skip_next:
                skip_next = False
                if not a.startswith("-"):
                    continue
            if a == "--" or not a.startswith("-"):
                break
            if a in ARG_TAKING_FLAGS:
                skip_next = True
                continue
            if a == "-m" and j + 1 < len(rest) and rest[j + 1] == "json.tool":
                emit_deny()
                return
            if a == "-mjson.tool":
                emit_deny()
                return


def emit_deny() -> None:
    json.dump(
        {
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "deny",
                "permissionDecisionReason": REASON,
            }
        },
        sys.stdout,
    )


if __name__ == "__main__":
    main()
