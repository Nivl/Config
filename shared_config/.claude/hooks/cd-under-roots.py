#!/usr/bin/env python3
# PreToolUse hook: enforce `cd` discipline.
#
# Two behaviours, in priority order:
#
# 1. DENY any `cd` command that is chained with another command via a shell
#    separator (;, &, &&, ||, |, |&). Wrapping a subsequent command in a `cd`
#    prefix hides it from prefix-based permission rules and hook matchers
#    (e.g. `cd /repo && git status` won't match `Bash(git *)`, so a git hook
#    keyed on that matcher never fires). The Bash tool's CWD persists across
#    calls — run `cd` as its own Bash call, then issue the next command
#    separately.
#
# 2. ALLOW `cd` into directories under one of the configured roots (/tmp,
#    $WORKTREES_ROOT, $REPOS_ROOT, $HOME/.melvin/config) when the command is
#    a simple, standalone `cd <path>`. Otherwise stays silent so the
#    `Bash(cd *)` ask rule fires.
#
# Relative targets are resolved against the session cwd from the hook input
# (Claude Code provides `cwd`), falling back to the hook process's own getcwd()
# only when absent — matches read-guard.py and write-under-roots.py.
#
# Stays silent (falls through to ask) when:
#   - the target uses unexpanded shell substitution ($var, $(...), `...`)
#   - the target is dash-prefixed (e.g. `cd -` for OLDPWD — can't resolve)
#   - the resolved path doesn't lie under any configured root

import json
import os
import shlex
import sys

CHAIN_TOKENS = {";", "&", "&&", "||", "|", "|&"}
ROOT_SPECS = (
    ("WORKTREES_ROOT", None),
    ("REPOS_ROOT", None),
    ("HOME", ".melvin/config"),
)
FIXED_ROOTS = ("/tmp", "/private/tmp")

CHAIN_DENY_REASON = (
    "`cd` should not be chained with other commands. The Bash tool's CWD "
    "persists across calls — run `cd <path>` as its own Bash call first, "
    "then issue the next command separately. Chaining also hides downstream "
    "commands from permission rules and hooks that match by prefix "
    "(e.g. `cd /repo && git status` won't match `Bash(git *)`)."
)


def allowed_roots():
    roots = [os.path.realpath(r) for r in FIXED_ROOTS]
    for var, sub in ROOT_SPECS:
        value = os.environ.get(var, "").strip()
        if not value:
            continue
        path = os.path.join(value, sub) if sub else value
        roots.append(os.path.realpath(path))
    return roots


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
    try:
        lex = shlex.shlex(cmd, posix=True, punctuation_chars=True)
        lex.whitespace_split = True
        tokens = list(lex)
    except ValueError:
        return

    if not tokens or tokens[0] != "cd":
        return

    # Deny chained cd commands regardless of target. This rule fires before
    # the allow-list check on purpose — `cd /tmp/x && rm -rf /` should not
    # auto-allow just because /tmp/x is an allowed root.
    if any(t in CHAIN_TOKENS for t in tokens):
        emit("deny", CHAIN_DENY_REASON)
        return

    # From here on we're dealing with a standalone `cd <args>` command.
    if "$" in cmd or "`" in cmd:
        return
    if len(tokens) < 2:
        return

    # Take the last token as the target so flags like `cd -L /tmp/x` still work.
    target = tokens[-1]
    if target.startswith("~"):
        target = os.path.expanduser(target)
    if target.startswith("-"):
        # `cd -` (OLDPWD) and other dash-prefixed targets can't be resolved
        # statically — fall through to the ask prompt.
        return
    if not os.path.isabs(target):
        # Resolve against the session cwd from the hook input. Fall back to
        # the hook process's own getcwd() only when absent.
        base = (data.get("cwd") or "").strip() or os.getcwd()
        target = os.path.join(base, target)
    resolved = os.path.realpath(target)

    for root in allowed_roots():
        try:
            if os.path.commonpath([resolved, root]) == root:
                emit("allow", f"cd target is inside {root}")
                return
        except ValueError:
            continue


if __name__ == "__main__":
    main()
