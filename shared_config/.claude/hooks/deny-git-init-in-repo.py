#!/usr/bin/env python3
# PreToolUse hook (Bash): deny `git init` and `git config user.name` /
# `user.email` when the command's cwd is already inside a git repository.
#
# A fixture repo belongs under /tmp/claude, where no .git sits above it, and
# both commands run there untouched. The case this stops is a sub-agent whose
# `cd` into the fixture did not carry to the next Bash call, so its setup ran
# against the real repo. One review agent did exactly that: `git init` was a
# no-op in an existing repo, `git config user.email a@a.com` rewrote the
# repo's identity, and `git commit -qm init` landed a stray commit under that
# name. The commit was reverted by hand. The identity would have signed every
# later commit until someone noticed.
#
# `--global` and `--system` config writes are not this hook's business, and a
# cwd with no .git above it is silent. Any failure is silent too.

import json
import os
import shlex
import sys

KNOWN_GIT_PATHS = {"git", "/usr/bin/git", "/opt/homebrew/bin/git", "/usr/local/bin/git", "/opt/local/bin/git"}
VALUE_OPTS = {"-C", "-c", "--git-dir", "--work-tree", "--namespace", "--super-prefix"}


def _tokens(cmd):
    try:
        lex = shlex.shlex(cmd, posix=True, punctuation_chars=True)
        lex.whitespace_split = True
        return list(lex)
    except ValueError:
        return []


def _repo_root(cwd):
    d = os.path.abspath(cwd)
    while True:
        if os.path.exists(os.path.join(d, ".git")):
            return d
        parent = os.path.dirname(d)
        if parent == d:
            return None
        d = parent


def _offending(tokens):
    # Returns a short description of the offending git command, or None.
    starts = [0] + [i + 1 for i, t in enumerate(tokens) if t in {";", "&&", "||", "|"}]
    for s in starts:
        while s < len(tokens) and "=" in tokens[s] and not tokens[s].startswith("-") and tokens[s] not in KNOWN_GIT_PATHS:
            s += 1
        if s >= len(tokens) or tokens[s] not in KNOWN_GIT_PATHS:
            continue
        i = s + 1
        while i < len(tokens) and tokens[i].startswith("-"):
            i += 2 if tokens[i] in VALUE_OPTS else 1
        if i >= len(tokens):
            continue
        sub = tokens[i]
        rest = tokens[i + 1:]
        if sub == "init":
            return "git init"
        if sub == "config":
            if any(r in {"--global", "--system"} for r in rest):
                continue
            # A key with a value after it is a write. A bare key is a read and is fine.
            for j, r in enumerate(rest):
                if r.lower() in {"user.name", "user.email"} and j + 1 < len(rest) and not rest[j + 1].startswith("-"):
                    return f"git config {r}"
    return None


def main() -> None:
    try:
        data = json.load(sys.stdin)
    except Exception:
        return
    cmd = (data.get("tool_input") or {}).get("command") or ""
    cwd = data.get("cwd") or os.getcwd()
    hit = None
    for line in cmd.splitlines():
        hit = _offending(_tokens(line))
        if hit:
            break
    if not hit:
        return
    root = _repo_root(cwd)
    if not root:
        return
    json.dump(
        {
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "deny",
                "permissionDecisionReason": (
                    f"`{hit}` inside an existing repository ({root}). A fixture repo belongs under "
                    "/tmp/claude, in a directory with no .git above it. `cd` there as its own Bash "
                    "call first, then run this. If you meant to change this repo's identity, ask the user."
                ),
            }
        },
        sys.stdout,
    )


if __name__ == "__main__":
    main()
