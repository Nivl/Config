#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/write-under-roots.py: feeds it
# PreToolUse Write payloads and asserts the permission decision.
#   path under an allowed root              -> allow
#   path traversing a .git segment           -> ask
#   path outside every root                  -> silent (falls through)
# /tmp is one of the FIXED_ROOTS, so the test uses it as the allowed root
# without having to mutate environment variables.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/write-under-roots.py"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

# decision <file_path> [cwd] -> permissionDecision string, or "silent"
# when the hook emits nothing.
decision() {
  local file_path="$1"
  local cwd="${2:-/tmp}"
  local payload out
  payload="$(jq -nc --arg fp "$file_path" --arg cwd "$cwd" \
    '{cwd: $cwd, tool_input: {file_path: $fp}}')"
  out="$(printf '%s' "$payload" | python3 "$HOOK")"
  if [[ -z "$out" ]]; then
    echo "silent"
  else
    jq -r '.hookSpecificOutput.permissionDecision' <<<"$out"
  fi
}

# ---- Plain writes under the allowed root ----
assert_eq "plain_under_tmp" "allow" "$(decision "/tmp/foo.txt")"
assert_eq "nested_under_tmp" "allow" "$(decision "/tmp/sub/dir/foo.txt")"

# ---- .git segment must downgrade to ask ----
assert_eq "git_head" "ask" "$(decision "/tmp/repo/.git/HEAD")"
assert_eq "git_hooks_pre" "ask" "$(decision "/tmp/repo/.git/hooks/pre-commit")"
assert_eq "git_itself" "ask" "$(decision "/tmp/repo/.git")"
assert_eq "git_submodule_file" "ask" "$(decision "/tmp/repo/sub/.git")"
assert_eq "git_at_root" "ask" "$(decision "/tmp/.git/config")"

# ---- Substring matches that aren't path segments must not trigger ----
assert_eq "gitignore" "allow" "$(decision "/tmp/repo/.gitignore")"
assert_eq "github_dir" "allow" "$(decision "/tmp/repo/.github/workflows/ci.yml")"
assert_eq "gitkeep" "allow" "$(decision "/tmp/repo/.gitkeep")"
assert_eq "bare_git_dir" "allow" "$(decision "/tmp/repo/foo.git/HEAD")"
assert_eq "dotfile_with_git" "allow" "$(decision "/tmp/repo/gitconfig")"

# ---- Case-insensitive match (macOS APFS) ----
assert_eq "git_upper" "ask" "$(decision "/tmp/repo/.GIT/HEAD")"
assert_eq "git_mixed" "ask" "$(decision "/tmp/repo/.Git/config")"

# ---- Relative file_path is resolved against the payload's cwd ----
assert_eq "relative_plain" "allow" "$(decision "foo.txt" "/tmp/repo")"
assert_eq "relative_git" "ask" "$(decision ".git/HEAD" "/tmp/repo")"

# ---- Symlink whose name has no .git but whose realpath enters a real .git ----
# Force the temp dir under /tmp because /var/folders/... (the macOS default)
# is not one of the hook's configured roots.
tmpdir="$(mktemp -d /tmp/write_under_roots_test.XXXXXX)"
mkdir -p "$tmpdir/real/.git"
ln -s "$tmpdir/real/.git" "$tmpdir/link"
assert_eq "symlink_into_git" "ask" "$(decision "$tmpdir/link/config")"
# Symlink NAMED .git pointing OUT to a non-git target — input path still
# carries `.git`, so the carve-out fires via the input-path check.
mkdir -p "$tmpdir/notgit_target" "$tmpdir/repo2"
ln -s "$tmpdir/notgit_target" "$tmpdir/repo2/.git"
assert_eq "symlink_named_git" "ask" "$(decision "$tmpdir/repo2/.git/x")"
rm -rf "$tmpdir"

# ---- Paths outside every root fall through ----
assert_eq "outside_roots" "silent" "$(decision "/etc/hosts")"
assert_eq "outside_with_git" "silent" "$(decision "/etc/.git/x")"

# ---- Missing file_path means nothing to check ----
empty_payload="$(jq -nc '{cwd:"/tmp", tool_input:{}}')"
empty_out="$(printf '%s' "$empty_payload" | python3 "$HOOK")"
assert_eq "missing_file_path" "" "$empty_out"

echo "write-under-roots.py: all tests passed"
