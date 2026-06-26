#!/usr/bin/env python3
# PreToolUse hook: deny `find` rooted at the filesystem root (`find /`).
# A whole-filesystem walk is slow, noisy, and almost never intended — finds
# should be scoped to a directory (a repo path, `.`, or a system subtree like
# `/etc`). Only the SEARCH ROOTS are inspected: the leading path operands after
# `find`, past the `-H`/`-L`/`-P` global options and before the expression
# begins. A `/` that appears later as an expression VALUE (`-path /`, `-newer /`)
# is not a search root and is left alone.
#
# Matches `find`/`gfind` at any command-position (leading, or after | ; && || …)
# so `… | find /` trips too. Wrapper/env-prefixed forms (`env find /`,
# `FOO=1 find /`, `xargs find /`) intentionally fall through — same documented
# carve-out as deny-awk.py; the agent is steered on the bare form, which is what
# it reaches for.

import json
import os
import re
import shlex
import sys

FIND_RE = re.compile(r"^g?find$")
# Tokens that mark a new simple command on their right. Same convention as
# deny-awk.py / deny-shell-wrapper.py.
CMD_SEPARATORS = frozenset({";", "|", "&", "&&", "||", "(", "{"})
# find's only options that may precede the path operands.
GLOBAL_OPTS = frozenset({"-H", "-L", "-P"})

REASON = (
    "Don't run `find /` (a walk of the whole filesystem) — keep finds focused. "
    "Scope the search to a directory: `find <dir> ...` (a repo path or `.`), or "
    "`rg --files <dir>` / `fd . <dir>` within a project. If you genuinely need a "
    "system search, narrow it to the relevant subtree (`/etc`, `/usr/local`, …) "
    "rather than `/`."
)


def is_root(path):
    # Root when the operand is only slashes (`/`, `//`, `///`) or normalizes to
    # root (`/.`, `/usr/..`). normpath preserves a literal leading `//`.
    if path and set(path) <= {"/"}:
        return True
    return os.path.normpath(path) in ("/", "//")


def is_expr_start(tok):
    # find's expression begins at the first option/operator token; the
    # search-root list ends there.
    return tok.startswith("-") or tok in ("(", ")", "!", ",")


def has_root_operand(tokens, start):
    j, n = start, len(tokens)
    while j < n and tokens[j] in GLOBAL_OPTS:
        j += 1
    while j < n and not is_expr_start(tokens[j]):
        if is_root(tokens[j]):
            return True
        j += 1
    return False


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

    for i, t in enumerate(tokens):
        if not FIND_RE.match(os.path.basename(t)):
            continue
        if i > 0 and tokens[i - 1] not in CMD_SEPARATORS:
            continue
        if has_root_operand(tokens, i + 1):
            emit_deny()
            return


def emit_deny():
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
