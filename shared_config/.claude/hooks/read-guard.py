#!/usr/bin/env python3
# PreToolUse hook: gate Read by sensitivity and location.
#
# Decisions, first match wins:
#   1. Sensitive file (env files, private keys, credential stores) → ASK,
#      regardless of location. A `.env` inside a repo root still prompts —
#      the sensitivity check runs before the root check on purpose.
#   2. Path under a configured root (/tmp, $WORKTREES_ROOT, $REPOS_ROOT,
#      $HOME/.melvin/config) → ALLOW.
#   3. Otherwise → silent (falls through to Claude Code's default prompt).
#
# A relative file_path is resolved against the session cwd from the hook
# input (Claude Code provides `cwd`), falling back to the hook process's own
# getcwd() only when absent. realpath is applied before the root boundary
# check so symlinks / `..` can't escape a root.

import json
import os
import sys

ROOT_SPECS = (
    ("WORKTREES_ROOT", None),
    ("REPOS_ROOT", None),
    ("HOME", ".melvin/config"),
)
FIXED_ROOTS = ("/tmp",)

# Look like .env files but carry no real secrets — safe to read, so NOT asked.
ENV_TEMPLATE_SUFFIXES = (".example", ".sample", ".template", ".dist", ".defaults")

# Exact basenames that are always sensitive.
SENSITIVE_NAMES = {
    "id_rsa",
    "id_dsa",
    "id_ecdsa",
    "id_ed25519",
    ".netrc",
    ".pgpass",
    ".git-credentials",
    "credentials.json",
}

# Basename extensions indicating private keys / keystores.
SENSITIVE_EXTS = (".pem", ".key", ".p12", ".pfx", ".p8")


def is_sensitive(name: str) -> bool:
    if name in SENSITIVE_NAMES:
        return True
    if name.endswith(SENSITIVE_EXTS):
        return True
    if name.endswith(".json") and "service-account" in name:
        return True
    # env files: `.env`, `*.env`, and `.env.*` — but not template variants.
    if name == ".env" or name.endswith(".env"):
        return True
    if name.startswith(".env."):
        return not name.endswith(ENV_TEMPLATE_SUFFIXES)
    return False


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

    file_path = (data.get("tool_input") or {}).get("file_path") or ""
    if not file_path:
        return

    if not os.path.isabs(file_path):
        base = (data.get("cwd") or "").strip() or os.getcwd()
        file_path = os.path.join(base, file_path)
    target = os.path.realpath(file_path)

    # 1. Sensitive files ASK regardless of location. Check both the requested
    #    name and the symlink target's name so `read cfg` (cfg -> /x/.env)
    #    can't slip a secret read past the filter.
    if is_sensitive(os.path.basename(file_path)) or is_sensitive(os.path.basename(target)):
        emit("ask", "sensitive file (may contain secrets) — confirm before reading")
        return

    # 2. Reads under a configured root auto-allow.
    for root in allowed_roots():
        try:
            if os.path.commonpath([target, root]) == root:
                emit("allow", f"path is inside {root}")
                return
        except ValueError:
            continue

    # 3. Otherwise stay silent → default prompt.


if __name__ == "__main__":
    main()
