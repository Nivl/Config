#!/bin/bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

SOURCE_REPO="$TMP_DIR/source-repo"
WORKTREES_ROOT="$TMP_DIR/worktrees"
TEST_HOME="$TMP_DIR/home"

mkdir -p "$SOURCE_REPO" "$WORKTREES_ROOT" "$TEST_HOME"

git -C "$SOURCE_REPO" init >/dev/null
git -C "$SOURCE_REPO" config user.name "Test User"
git -C "$SOURCE_REPO" config user.email "test@example.com"
git -C "$SOURCE_REPO" remote add origin "https://github.com/example/project.git"

cat > "$SOURCE_REPO/.gitignore" <<'EOF'
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
  "$SOURCE_REPO/node_modules/package" \
  "$SOURCE_REPO/nested/node_modules/tool" \
  "$SOURCE_REPO/nested/service" \
  "$SOURCE_REPO/.venv/bin" \
  "$SOURCE_REPO/.cache" \
  "$SOURCE_REPO/dist" \
  "$SOURCE_REPO/.next/cache"

printf 'tracked\n' > "$SOURCE_REPO/README.md"
printf 'DATABASE_URL=postgres://local\n' > "$SOURCE_REPO/.env"
printf 'ROOT_SECRET=value\n' > "$SOURCE_REPO/.env.local"
printf 'module.exports = 1;\n' > "$SOURCE_REPO/node_modules/package/index.js"
printf 'console.log("nested");\n' > "$SOURCE_REPO/nested/node_modules/tool/bin.js"
printf 'SERVICE_TOKEN=abc123\n' > "$SOURCE_REPO/nested/service/.env.local"
printf '#!/bin/sh\n' > "$SOURCE_REPO/.venv/bin/python"
printf 'skip-me\n' > "$SOURCE_REPO/.cache/cache.txt"
printf 'compiled\n' > "$SOURCE_REPO/dist/app.js"
printf '{"cached":true}\n' > "$SOURCE_REPO/.next/cache/data.json"

git -C "$SOURCE_REPO" add .gitignore README.md
git -C "$SOURCE_REPO" commit -m "init" >/dev/null

TEST_RESULT="$(
  REPO_ROOT="$REPO_ROOT" \
  SOURCE_REPO="$SOURCE_REPO" \
  WORKTREES_ROOT="$WORKTREES_ROOT" \
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

code() {
  :
}

cd "$SOURCE_REPO"
WORKTREES_ROOT="$WORKTREES_ROOT" wt feature-copy
EOF
)"

EXPECTED_WORKTREE="$WORKTREES_ROOT/github.com/example/project/feature-copy"

assert_exists() {
  local path="$1"
  if [ ! -e "$path" ]; then
    printf 'Expected path to exist: %s\n' "$path" >&2
    exit 1
  fi
}

assert_not_exists() {
  local path="$1"
  if [ -e "$path" ]; then
    printf 'Expected path to be absent: %s\n' "$path" >&2
    exit 1
  fi
}

assert_file_contains() {
  local path="$1"
  local needle="$2"
  assert_exists "$path"
  if ! grep -Fq "$needle" "$path"; then
    printf 'Expected %s to contain %s\n' "$path" "$needle" >&2
    exit 1
  fi
}

assert_file_contains "$EXPECTED_WORKTREE/.env" 'DATABASE_URL=postgres://local'
assert_file_contains "$EXPECTED_WORKTREE/.env.local" 'ROOT_SECRET=value'
assert_file_contains "$EXPECTED_WORKTREE/nested/service/.env.local" 'SERVICE_TOKEN=abc123'
assert_exists "$EXPECTED_WORKTREE/node_modules/package/index.js"
assert_exists "$EXPECTED_WORKTREE/nested/node_modules/tool/bin.js"
assert_exists "$EXPECTED_WORKTREE/.venv/bin/python"
assert_not_exists "$EXPECTED_WORKTREE/.cache/cache.txt"
assert_not_exists "$EXPECTED_WORKTREE/dist/app.js"
assert_not_exists "$EXPECTED_WORKTREE/.next/cache/data.json"

case "$TEST_RESULT" in
  *"Ready to work"* ) ;;
  *)
    printf 'Expected wt output to mention Ready to work, got:\n%s\n' "$TEST_RESULT" >&2
    exit 1
    ;;
esac
