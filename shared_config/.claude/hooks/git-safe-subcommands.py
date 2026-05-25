#!/usr/bin/env python3
# PreToolUse hook: route `git` subcommands to allow/ask/deny based on a
# per-list prefix table, regardless of `-C <path>` and regardless of the
# `git` invocation form (`git`, `/usr/bin/git`, etc.).
#
# Reads PreToolUse JSON from stdin. After tokenizing, the hook normalizes the
# leading executable token and optionally strips a leading `-C <absolute-path>`
# argument, then checks the remaining tokens against three prefix tables in
# precedence order: DENY_PREFIXES > ASK_PREFIXES > ALLOW_PREFIXES. The first
# match wins and emits the corresponding permissionDecision.
#
# Stays silent (falls through to Claude Code's default Bash permission flow)
# when any of:
#   - the command contains a shell separator (;, &, &&, ||, |, |&)
#   - the command contains $var or `cmd` substitution
#   - the leading token is not a recognized git executable
#   - `-C` is present but its path is not under $WORKTREES_ROOT or $REPOS_ROOT
#     (after realpath resolution, so symlinks and `..` can't slip past)
#   - the post-`git` (and post-`-C`) tokens don't start with any listed prefix
#
# Maintenance: the three prefix tables are edited by `melvin-config claude
# perms <list> add/remove --bash 'git <subcmd> *'`. Trailing args after a
# matched prefix are always allowed, so `("show",)` matches `git show
# <anything>`.

import json
import os
import shlex
import sys

CHAIN_TOKENS = {";", "&", "&&", "||", "|", "|&"}
KNOWN_GIT_PATHS = {
    "/usr/bin/git",
    "/opt/homebrew/bin/git",
    "/usr/local/bin/git",
    "/opt/local/bin/git",
}
DASH_C_ROOT_VARS = ("WORKTREES_ROOT", "REPOS_ROOT")


def dash_c_roots():
    roots = []
    for var in DASH_C_ROOT_VARS:
        value = os.environ.get(var, "").strip()
        if value:
            roots.append(os.path.realpath(value))
    return roots


def is_under_allowed_root(path: str) -> bool:
    target = os.path.realpath(path)
    for root in dash_c_roots():
        try:
            if os.path.commonpath([target, root]) == root:
                return True
        except ValueError:
            continue
    return False


ALLOW_PREFIXES = (
    ("add",),
    ("blame",),
    ("branch", "--show-current"),
    ("check-ignore", "-v"),
    ("cherry-pick",),
    ("diff",),
    ("fetch",),
    ("fsck",),
    ("help",),
    ("log",),
    ("ls-files",),
    ("ls-remote",),
    ("ls-tree",),
    ("merge", "origin/main"),
    ("merge", "origin/master"),
    ("mv",),
    ("pull",),
    ("remote", "show"),
    ("rev-list",),
    ("rev-parse",),
    ("rm",),
    ("show",),
    ("stash", "apply"),
    ("stash", "list"),
    ("status",),
    ("worktree", "list"),
)

ASK_PREFIXES = ()

DENY_PREFIXES = ()


def first_prefix_match(args, prefixes):
    for prefix in prefixes:
        if len(args) >= len(prefix) and tuple(args[: len(prefix)]) == prefix:
            return prefix
    return None


def main() -> None:
    try:
        data = json.load(sys.stdin)
    except Exception:
        return

    cmd = (data.get("tool_input") or {}).get("command") or ""
    if "$" in cmd or "`" in cmd:
        return

    try:
        lex = shlex.shlex(cmd, posix=True, punctuation_chars=True)
        lex.whitespace_split = True
        tokens = list(lex)
    except ValueError:
        return

    if any(t in CHAIN_TOKENS for t in tokens):
        return
    if not tokens:
        return

    head = tokens[0]
    if head != "git" and head not in KNOWN_GIT_PATHS:
        return

    args = tokens[1:]
    if len(args) >= 2 and args[0] == "-C":
        if not args[1].startswith("/") or not is_under_allowed_root(args[1]):
            return
        args = args[2:]
    if not args:
        return

    # Precedence: deny first (security), then ask (escalation), then allow.
    for decision, table in (("deny", DENY_PREFIXES), ("ask", ASK_PREFIXES), ("allow", ALLOW_PREFIXES)):
        match = first_prefix_match(args, table)
        if match is not None:
            json.dump(
                {
                    "hookSpecificOutput": {
                        "hookEventName": "PreToolUse",
                        "permissionDecision": decision,
                        "permissionDecisionReason": f"git {' '.join(match)} is on the {decision} list",
                    }
                },
                sys.stdout,
            )
            return


if __name__ == "__main__":
    main()
