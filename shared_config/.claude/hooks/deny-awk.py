#!/usr/bin/env python3
# PreToolUse hook: deny `awk`/`gawk`/`mawk`/`nawk` at any command-position
# and steer the agent at single-purpose tools (`cut`, `sed`, `grep`,
# `head`/`tail`, `jq`, `git symbolic-ref`). awk is a full programming
# language with `system()`, file redirects (`> "file"`, `>> "file"`,
# `| "cmd"`), and external-script loading (`-f` / `-i` / `-l` / `-e`),
# so allow-listing `Bash(awk *)` would re-open a large attack surface.
# The common shapes the agent reaches for (`'{print $N}'`, `'{print
# $NF}'`, `/pattern/ {print}`, `NR<=N`) all have direct replacements
# with narrower tools.
#
# Walks every command-position token so the canonical piped form
# `cat file | awk '...'` (and chained `; awk`, subshell `(awk ...)`)
# trips the rule too. Wrapper-prefixed forms (`env awk`, `sudo awk`,
# `xargs awk`) intentionally fall through — same documented carve-out
# as `deny-json-tool.py`; the agent will be redirected on the simpler
# form and that's enough for the steering goal.

import json
import os
import re
import shlex
import sys

AWK_RE = re.compile(r"^(?:g|m|n)?awk$")
# Tokens that mark a new simple command on their right. Same convention as
# deny-shell-wrapper.py / deny-json-tool.py.
CMD_SEPARATORS = frozenset({";", "|", "&", "&&", "||", "(", "{"})

REASON = (
    "Don't use `awk` (or `gawk`/`mawk`/`nawk`) — it's a full programming "
    "language with `system()`, file redirects, and external-script "
    "loading, which is too much to allow-list safely. Use a single-"
    "purpose tool instead:\n"
    "  - field extraction:        `cut -d'<delim>' -f<N>`\n"
    "  - field extraction (last): `rev | cut -d' ' -f1 | rev`  OR  `sed 's/.* //'`\n"
    "  - line filtering:          `grep <pattern>`  OR  `grep -E <pattern>`\n"
    "  - line range:              `head -n <N>` / `tail -n <N>` / `sed -n '<a>,<b>p'`\n"
    "  - extract & transform:     `sed -n 's/<from>/<to>/p'`\n"
    "  - JSON:                    `jq '<filter>'`\n"
    "For `git remote show origin | grep 'HEAD branch' | awk '{print $NF}'` "
    "specifically, `git symbolic-ref refs/remotes/origin/HEAD --short` is "
    "the direct equivalent."
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

    # Walk every command-position token. If it's an awk-family binary,
    # deny. Plain occurrences as arguments (`echo awk`, `which awk`)
    # don't sit at command-position so they fall through.
    for i, t in enumerate(tokens):
        if not AWK_RE.match(os.path.basename(t)):
            continue
        if i > 0 and tokens[i - 1] not in CMD_SEPARATORS:
            continue
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
