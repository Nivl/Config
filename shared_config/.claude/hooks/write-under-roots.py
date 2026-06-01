#!/usr/bin/env python3
# PreToolUse hook: auto-allow Write/Edit to paths under any configured root.
# Each root entry is (env_var, optional_subpath); the env var must be set to a
# non-empty value for the root to be honored. Paths are canonicalized via
# realpath before the boundary check, so symlinks and `..` segments can't be
# used to slip past the root.
#
# Relative file_path handling: the Write tool is supposed to send an absolute
# path, but a subagent can send a relative one. A relative path is relative to
# the session's working directory, which the hook input provides as `cwd` —
# resolving against the hook process's own getcwd() would be wrong whenever the
# two differ (e.g. a subagent running from another directory), which silently
# dropped the rule and forced a prompt. Prefer the input cwd; fall back to
# getcwd() only when it's absent.
#
# .git carve-out: even when a path sits under an allowed root, writes that
# touch a `.git` segment are downgraded from `allow` to `ask`. Both the input
# path and its realpath are checked, so neither a direct `.git/...` write nor
# a symlink that resolves into a real `.git` dir slips through silently.

import json
import os
import sys

ROOT_SPECS = (
    ("WORKTREES_ROOT", None),
    ("REPOS_ROOT", None),
    ("HOME", ".melvin/config"),
)
FIXED_ROOTS = ("/tmp", "/private/tmp")


def _has_git_segment(path: str) -> bool:
    # Lowercase before comparing — macOS APFS is case-insensitive by default,
    # so `.GIT/HEAD` and `.Git/HEAD` resolve to the same directory as
    # `.git/HEAD`. A case-sensitive check would miss writes through those
    # spellings.
    return ".git" in [p.lower() for p in path.split(os.sep)]


def main() -> None:
    try:
        data = json.load(sys.stdin)
    except Exception:
        return

    file_path = (data.get("tool_input") or {}).get("file_path") or ""
    if not file_path:
        return

    if not os.path.isabs(file_path):
        base = (data.get("cwd") or "").strip() or os.getcwd()
        file_path = os.path.join(base, file_path)

    roots = [os.path.realpath(r) for r in FIXED_ROOTS]
    for var, sub in ROOT_SPECS:
        value = os.environ.get(var, "").strip()
        if not value:
            continue
        path = os.path.join(value, sub) if sub else value
        roots.append(os.path.realpath(path))
    if not roots:
        return

    target = os.path.realpath(file_path)

    for root in roots:
        try:
            if os.path.commonpath([target, root]) != root:
                continue
        except ValueError:
            # commonpath raises on mixed absolute/relative or different drives
            continue

        if _has_git_segment(file_path) or _has_git_segment(target):
            decision = "ask"
            reason = "path traverses a .git directory; refusing to auto-allow"
        else:
            decision = "allow"
            reason = f"path is inside {root}"

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
        return


if __name__ == "__main__":
    main()
