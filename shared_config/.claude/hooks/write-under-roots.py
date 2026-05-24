#!/usr/bin/env python3
# PreToolUse hook: auto-allow Write to paths under any of the configured roots.
# Each root entry is (env_var, optional_subpath); the env var must be set to a
# non-empty value for the root to be honored. Paths are canonicalized via
# realpath before the boundary check, so symlinks and `..` segments can't be
# used to slip past the root.

import json
import os
import sys

ROOT_SPECS = (
    ("WORKTREES_ROOT", None),
    ("REPOS_ROOT", None),
    ("HOME", ".melvin/config"),
)


def main() -> None:
    try:
        data = json.load(sys.stdin)
    except Exception:
        return

    file_path = (data.get("tool_input") or {}).get("file_path") or ""
    if not file_path:
        return

    roots = []
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
