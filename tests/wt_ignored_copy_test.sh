#!/bin/zsh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

WORKTREES_ROOT="$TMP_DIR/worktrees"
TEST_HOME="$TMP_DIR/home"

mkdir -p "$WORKTREES_ROOT" "$TEST_HOME"

init_source_repo() {
  local source_repo="$1"
  local remote_url="$2"

  mkdir -p "$source_repo"

  git -C "$source_repo" init >/dev/null
  git -C "$source_repo" config user.name "Test User"
  git -C "$source_repo" config user.email "test@example.com"
  git -C "$source_repo" remote add origin "$remote_url"

  cat > "$source_repo/.gitignore" <<'EOF'
.env
.env.*
node_modules/
nested/node_modules/
.venv/
.cache/
dist/
.next/
EOF

  mkdir -p \
    "$source_repo/node_modules/package" \
    "$source_repo/nested/node_modules/tool" \
    "$source_repo/nested/service" \
    "$source_repo/.venv/bin" \
    "$source_repo/.cache" \
    "$source_repo/dist" \
    "$source_repo/.next/cache"

  printf 'tracked\n' > "$source_repo/README.md"
  printf 'DATABASE_URL=postgres://local\n' > "$source_repo/.env"
  printf 'ROOT_SECRET=value\n' > "$source_repo/.env.local"
  printf 'module.exports = 1;\n' > "$source_repo/node_modules/package/index.js"
  printf 'console.log("nested");\n' > "$source_repo/nested/node_modules/tool/bin.js"
  printf 'SERVICE_TOKEN=abc123\n' > "$source_repo/nested/service/.env.local"
  printf '#!/bin/sh\n' > "$source_repo/.venv/bin/python"
  printf 'skip-me\n' > "$source_repo/.cache/cache.txt"
  printf 'compiled\n' > "$source_repo/dist/app.js"
  printf '{"cached":true}\n' > "$source_repo/.next/cache/data.json"

  git -C "$source_repo" add .gitignore README.md
  git -C "$source_repo" commit -m "init" >/dev/null
}

run_wt() {
  local source_repo="$1"
  local feature_name="$2"

  REPO_ROOT="$REPO_ROOT" \
  SOURCE_REPO="$source_repo" \
  WORKTREES_ROOT="$WORKTREES_ROOT" \
  TEST_HOME="$TEST_HOME" \
  TEST_TMP_DIR="$TMP_DIR" \
  FEATURE_NAME="$feature_name" \
  zsh <<'EOF'
set -euo pipefail

brew() {
  if [ "${1:-}" = "--prefix" ]; then
    local prefix="$TEST_TMP_DIR/brew-prefix/${2:-default}"
    mkdir -p "$prefix"
    printf '%s\n' "$prefix"
    return 0
  fi

  printf 'unexpected brew call: %s\n' "$*" >&2
  return 1
}

HOME="$TEST_HOME"
source "$REPO_ROOT/config.zshrc"

code() {
  :
}

cd "$SOURCE_REPO"
WORKTREES_ROOT="$WORKTREES_ROOT" wt "$FEATURE_NAME"
pwd
EOF
}

run_wt_done() {
  local worktree_repo="$1"

  REPO_ROOT="$REPO_ROOT" \
  WORKTREE_REPO="$worktree_repo" \
  TEST_HOME="$TEST_HOME" \
  TEST_TMP_DIR="$TMP_DIR" \
  zsh <<'EOF'
set -euo pipefail

brew() {
  if [ "${1:-}" = "--prefix" ]; then
    local prefix="$TEST_TMP_DIR/brew-prefix/${2:-default}"
    mkdir -p "$prefix"
    printf '%s\n' "$prefix"
    return 0
  fi

  printf 'unexpected brew call: %s\n' "$*" >&2
  return 1
}

HOME="$TEST_HOME"
source "$REPO_ROOT/config.zshrc"

cd "$WORKTREE_REPO"
wt_done
EOF
}

EXPECTED_WORKTREE="$WORKTREES_ROOT/github.com/example/project/feature-copy"

assert_exists() {
  local target_path="$1"
  if [ ! -e "$target_path" ]; then
    printf 'Expected path to exist: %s\n' "$target_path" >&2
    exit 1
  fi
}

assert_not_exists() {
  local target_path="$1"
  if [ -e "$target_path" ]; then
    printf 'Expected path to be absent: %s\n' "$target_path" >&2
    exit 1
  fi
}

assert_file_contains() {
  local target_path="$1"
  local needle="$2"
  assert_exists "$target_path"
  if ! grep -Fq "$needle" "$target_path"; then
    printf 'Expected %s to contain %s\n' "$target_path" "$needle" >&2
    exit 1
  fi
}

assert_branch_name() {
  local repo_path="$1"
  local expected_branch="$2"
  local actual_branch

  actual_branch="$(git -C "$repo_path" symbolic-ref --quiet --short HEAD)"
  if [ "$actual_branch" != "$expected_branch" ]; then
    printf 'Expected branch %s, got %s\n' "$expected_branch" "$actual_branch" >&2
    exit 1
  fi
}

assert_output_contains() {
  local haystack="$1"
  local needle="$2"

  case "$haystack" in
    *"$needle"* ) ;;
    *)
      printf 'Expected output to contain %s, got:\n%s\n' "$needle" "$haystack" >&2
      exit 1
      ;;
  esac
}

assert_output_not_contains() {
  local haystack="$1"
  local needle="$2"

  case "$haystack" in
    *"$needle"* )
      printf 'Expected output not to contain %s, got:\n%s\n' "$needle" "$haystack" >&2
      exit 1
      ;;
    *)
      ;;
  esac
}

assert_wt_result() {
  local expected_worktree="$1"
  local test_result="$2"

  assert_file_contains "$expected_worktree/.env" 'DATABASE_URL=postgres://local'
  assert_file_contains "$expected_worktree/.env.local" 'ROOT_SECRET=value'
  assert_file_contains "$expected_worktree/nested/service/.env.local" 'SERVICE_TOKEN=abc123'
  assert_exists "$expected_worktree/node_modules/package/index.js"
  assert_exists "$expected_worktree/nested/node_modules/tool/bin.js"
  assert_exists "$expected_worktree/.venv/bin/python"
  assert_not_exists "$expected_worktree/.cache/cache.txt"
  assert_not_exists "$expected_worktree/dist/app.js"
  assert_not_exists "$expected_worktree/.next/cache/data.json"

  case "$test_result" in
    *"Ready to work"* ) ;;
    *)
      printf 'Expected wt output to mention Ready to work, got:\n%s\n' "$test_result" >&2
      exit 1
      ;;
  esac
}

HTTPS_SOURCE_REPO="$TMP_DIR/source-repo-https"
SSH_SOURCE_REPO="$TMP_DIR/source-repo-ssh"
CLEANUP_SOURCE_REPO="$TMP_DIR/source-repo-cleanup"
CLEANUP_REMOTE_REPO="$TMP_DIR/source-repo-cleanup-remote.git"
REUSE_SOURCE_REPO="$TMP_DIR/source-repo-reuse"

init_source_repo "$HTTPS_SOURCE_REPO" "https://github.com/example/project.git"
init_source_repo "$SSH_SOURCE_REPO" "git@github.com:Nivl/next-themes.git"
init_source_repo "$REUSE_SOURCE_REPO" "https://github.com/Nivl/melvin.la.git"
git init --bare "$CLEANUP_REMOTE_REPO" >/dev/null
init_source_repo "$CLEANUP_SOURCE_REPO" "$CLEANUP_REMOTE_REPO"
git -C "$CLEANUP_SOURCE_REPO" push -u origin HEAD >/dev/null
git -C "$HTTPS_SOURCE_REPO" branch feature-existing
REUSED_BRANCH="renovate/design-system"
REUSED_WORKTREE="$WORKTREES_ROOT/github.com/Nivl/melvin.la/$REUSED_BRANCH"
mkdir -p "$(dirname "$REUSED_WORKTREE")"
git -C "$REUSE_SOURCE_REPO" worktree add -b "$REUSED_BRANCH" "$REUSED_WORKTREE" >/dev/null

HTTPS_TEST_RESULT="$(run_wt "$HTTPS_SOURCE_REPO" feature-copy)"
SSH_TEST_RESULT="$(run_wt "$SSH_SOURCE_REPO" feature-ssh)"
EXISTING_BRANCH_TEST_RESULT="$(run_wt "$HTTPS_SOURCE_REPO" feature-existing)"
REUSED_WORKTREE_RESULT="$(run_wt "$REUSE_SOURCE_REPO" "$REUSED_BRANCH")"

assert_wt_result "$WORKTREES_ROOT/github.com/example/project/feature-copy" "$HTTPS_TEST_RESULT"
assert_wt_result "$WORKTREES_ROOT/github.com/Nivl/next-themes/feature-ssh" "$SSH_TEST_RESULT"
assert_wt_result "$WORKTREES_ROOT/github.com/example/project/feature-existing" "$EXISTING_BRANCH_TEST_RESULT"
assert_branch_name "$WORKTREES_ROOT/github.com/example/project/feature-existing" "feature-existing"
assert_output_contains "$REUSED_WORKTREE_RESULT" "$REUSED_WORKTREE"
assert_output_not_contains "$REUSED_WORKTREE_RESULT" "Copying '"
assert_output_not_contains "$REUSED_WORKTREE_RESULT" "fatal:"

CLEANUP_BRANCH="a/b/c/d/e"
CLEANUP_WORKTREE="$WORKTREES_ROOT/cleanup-repo/$CLEANUP_BRANCH"
mkdir -p "$(dirname "$CLEANUP_WORKTREE")"
git -C "$CLEANUP_SOURCE_REPO" worktree add -b "$CLEANUP_BRANCH" "$CLEANUP_WORKTREE" >/dev/null
git -C "$CLEANUP_WORKTREE" push -u origin "$CLEANUP_BRANCH" >/dev/null

WT_DONE_RESULT="$(run_wt_done "$CLEANUP_WORKTREE")"

assert_not_exists "$CLEANUP_WORKTREE"
assert_not_exists "$WORKTREES_ROOT/cleanup-repo/a"
assert_exists "$WORKTREES_ROOT/cleanup-repo"

case "$WT_DONE_RESULT" in
  *"Deleted worktree"* ) ;;
  *)
    printf 'Expected wt_done output to mention Deleted worktree, got:\n%s\n' "$WT_DONE_RESULT" >&2
    exit 1
    ;;
esac
