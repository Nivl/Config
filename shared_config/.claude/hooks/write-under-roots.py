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

import json
import os
import sys

ROOT_SPECS = (
    ("WORKTREES_ROOT", None),
    ("REPOS_ROOT", None),
    ("HOME", ".melvin/config"),
)
FIXED_ROOTS = ("/tmp", "/private/tmp")


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
            if os.path.commonpath([target, root]) == root:
                json.dump(
                    {
                        "hookSpecificOutput": {
                            "hookEventName": "PreToolUse",
                            "permissionDecision": "allow",
                            "permissionDecisionReason": f"path is inside {root}",
                        }
                    },
                    sys.stdout,
                )
                return
        except ValueError:
            # commonpath raises on mixed absolute/relative or different drives
            continue


if __name__ == "__main__":
    main()
