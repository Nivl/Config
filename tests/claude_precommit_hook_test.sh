#!/bin/zsh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

# Build a fresh git repo at $1 with .githooks/* copied from REPO_ROOT and
# .git/hooks/pre-commit symlinked to .githooks/pre-commit.
setup_hook_repo() {
  local dir="$1"
  mkdir -p "$dir"
  git -C "$dir" init -q -b main
  git -C "$dir" config user.email "test@example.com"
  git -C "$dir" config user.name "test"
  mkdir -p "$dir/.githooks" "$dir/.claude"
  cp "$REPO_ROOT/.githooks/canonicalize.jq" "$dir/.githooks/canonicalize.jq"
  cp "$REPO_ROOT/.githooks/pre-commit" "$dir/.githooks/pre-commit"
  chmod +x "$dir/.githooks/pre-commit"
  ln -s "../../.githooks/pre-commit" "$dir/.git/hooks/pre-commit"
  # Stage the hook infrastructure once so subsequent commits aren't blocked by it.
  git -C "$dir" add .githooks
  git -C "$dir" -c core.hooksPath=/dev/null commit -q -m "init hooks"
}

# 10. canonicalizes unsorted/duplicated permissions.allow
S="hook_canonicalizes_unsorted_array"
SDIR="$TMP_DIR/$S"
setup_hook_repo "$SDIR"
printf '%s\n' '{"permissions":{"allow":["Bash(git *)","Bash(* --help *)","Bash(git *)"]}}' \
  >"$SDIR/.claude/settings.json"
git -C "$SDIR" add .claude/settings.json
git -C "$SDIR" commit -q -m "add settings"
committed_canonical="$(git -C "$SDIR" show HEAD:.claude/settings.json | jq -c .)"
assert_eq "$S" '{"permissions":{"allow":["Bash(* --help *)","Bash(git *)"]}}' "$committed_canonical"

# 11. noop on already-canonical input — blob unchanged byte-for-byte
S="hook_noop_on_canonical_input"
SDIR="$TMP_DIR/$S"
setup_hook_repo "$SDIR"
canonical_input='{
  "permissions": {
    "allow": [
      "Bash(* --help *)",
      "Bash(git *)"
    ]
  }
}'
printf '%s' "$canonical_input" >"$SDIR/.claude/settings.json"
git -C "$SDIR" add .claude/settings.json
git -C "$SDIR" commit -q -m "add canonical settings"
committed_blob="$(git -C "$SDIR" show HEAD:.claude/settings.json)"
assert_eq "$S" "$canonical_input" "$committed_blob"

# 12. aborts on invalid JSON
S="hook_aborts_on_invalid_json"
SDIR="$TMP_DIR/$S"
setup_hook_repo "$SDIR"
printf '%s' '{not valid json' >"$SDIR/.claude/settings.json"
git -C "$SDIR" add .claude/settings.json
if git -C "$SDIR" commit -q -m "should fail" 2>"$TMP_DIR/$S.stderr"; then
  echo "[$S] expected commit to fail but it succeeded" >&2
  exit 1
fi
assert_contains "$S" "not valid JSON" "$(cat "$TMP_DIR/$S.stderr")"

# 13. ignores JSON outside .claude/
S="hook_ignores_non_claude_json"
SDIR="$TMP_DIR/$S"
setup_hook_repo "$SDIR"
printf '%s' '{"x":[3,2,1]}' >"$SDIR/other.json"
git -C "$SDIR" add other.json
git -C "$SDIR" commit -q -m "non-claude json"
committed="$(git -C "$SDIR" show HEAD:other.json)"
assert_eq "$S" '{"x":[3,2,1]}' "$committed" # untouched

# 18. picks up versioned changes without re-running install.sh
S="hook_picks_up_versioned_changes"
SDIR="$TMP_DIR/$S"
setup_hook_repo "$SDIR"
# Append a marker echo to the versioned hook script
printf '\necho HOOK_RAN_FROM_VERSIONED >&2\n' >>"$SDIR/.githooks/pre-commit"
git -C "$SDIR" add .githooks/pre-commit
git -C "$SDIR" commit -q -m "modify hook"
# Now make a normal commit and check the marker fires
printf '%s' '{"permissions":{"allow":[]}}' >"$SDIR/.claude/settings.json"
git -C "$SDIR" add .claude/settings.json
out="$(git -C "$SDIR" commit -m "trigger hook" 2>&1)"
assert_contains "$S" "HOOK_RAN_FROM_VERSIONED" "$out"

# --- installer scenarios ---------------------------------------------------

# Source install.sh's functions for direct invocation. We strip the trailing
# `main "$@"` call (last line) the same way claude_sync_test.sh does.
TMP_FN_FILE="$TMP_DIR/install_functions.sh"
sed '$d' "$REPO_ROOT/install.sh" >"$TMP_FN_FILE"
# shellcheck disable=SC1090
. "$TMP_FN_FILE"

# Build a bare repo with .githooks/ in place but NO symlink at .git/hooks/pre-commit.
setup_repo_no_symlink() {
  local dir="$1"
  mkdir -p "$dir"
  git -C "$dir" init -q -b main
  mkdir -p "$dir/.githooks"
  cp "$REPO_ROOT/.githooks/canonicalize.jq" "$dir/.githooks/canonicalize.jq"
  cp "$REPO_ROOT/.githooks/pre-commit" "$dir/.githooks/pre-commit"
  chmod +x "$dir/.githooks/pre-commit"
}

# 14. install creates the symlink on a fresh clone
S="hook_install_creates_symlink"
SDIR="$TMP_DIR/$S"
setup_repo_no_symlink "$SDIR"
CONFIG_DIR="$SDIR" install_claude_precommit_hook
hooks_dir="$(git -C "$SDIR" rev-parse --git-path hooks)"
target="$SDIR/$hooks_dir/pre-commit"
[ -L "$target" ] || {
  echo "[$S] expected symlink at $target" >&2
  exit 1
}
resolved="$(readlink "$target")"
assert_eq "$S" "../../.githooks/pre-commit" "$resolved"
[ -x "$SDIR/.githooks/pre-commit" ] || {
  echo "[$S] target not executable" >&2
  exit 1
}

# 15. idempotent: re-running does nothing
S="hook_install_idempotent"
SDIR="$TMP_DIR/$S"
setup_repo_no_symlink "$SDIR"
CONFIG_DIR="$SDIR" install_claude_precommit_hook
out="$(CONFIG_DIR="$SDIR" install_claude_precommit_hook 2>&1)"
assert_eq "$S idempotent stdout/stderr empty" "" "$out"

# 16. refuses to clobber a regular file
S="hook_install_refuses_to_clobber_regular_file"
SDIR="$TMP_DIR/$S"
setup_repo_no_symlink "$SDIR"
hooks_dir_abs="$SDIR/$(git -C "$SDIR" rev-parse --git-path hooks)"
echo '#!/bin/sh' >"$hooks_dir_abs/pre-commit"
chmod +x "$hooks_dir_abs/pre-commit"
out="$(CONFIG_DIR="$SDIR" install_claude_precommit_hook 2>&1 || true)"
assert_contains "$S" "existing pre-commit" "$out"
[ ! -L "$hooks_dir_abs/pre-commit" ] || {
  echo "[$S] symlink should NOT have been created" >&2
  exit 1
}

# 17. refuses to clobber a foreign symlink
S="hook_install_refuses_to_clobber_foreign_symlink"
SDIR="$TMP_DIR/$S"
setup_repo_no_symlink "$SDIR"
hooks_dir_abs="$SDIR/$(git -C "$SDIR" rev-parse --git-path hooks)"
ln -s /tmp/some-other-hook "$hooks_dir_abs/pre-commit"
out="$(CONFIG_DIR="$SDIR" install_claude_precommit_hook 2>&1 || true)"
assert_contains "$S" "existing pre-commit" "$out"
[ "$(readlink "$hooks_dir_abs/pre-commit")" = "/tmp/some-other-hook" ] ||
  {
    echo "[$S] foreign symlink should be untouched" >&2
    exit 1
  }

echo "claude_precommit_hook_test.sh: all hook scenarios passed"
