#!/bin/bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

cat > "$TMP_DIR/harness.sh" <<'EOF'
set -euo pipefail

HOME="$TEST_TMP_DIR/home"
mkdir -p "$HOME"

sed '$d' "$REPO_ROOT/install.sh" > "$TEST_TMP_DIR/install_functions.sh"
source "$TEST_TMP_DIR/install_functions.sh"

BREW_LOG="$TEST_TMP_DIR/brew.log"
touch "$BREW_LOG"

setup_personal_computer_flag() {
  PERSONAL_COMPUTER=false
}

setup_skip_existing_flag() {
  SKIP_CONFIG_FILE_SETUP=true
}

install_homebrew() {
  :
}

setup_ssh() {
  :
}

setup_config_repo() {
  mkdir -p "$CONFIG_DIR"
}

copy_config_files() {
  :
}

setup_zshrc() {
  :
}

setup_gitconfig() {
  :
}

setup_gpg() {
  :
}

setup_github() {
  SETUP_GITHUB=false
}

print_remaining_tasks() {
  :
}

is_installed_cask() {
  case "$1" in
    docker|warp) return 0 ;;
    *) return 1 ;;
  esac
}

brew() {
  if [ "${1:-}" = "list" ] && [ "${2:-}" = "--cask" ]; then
    local cask="${3:-}"
    [ -n "$cask" ] || return 1
    if is_installed_cask "$cask"; then
      return 0
    fi
    return 1
  fi

  if [ "${1:-}" = "info" ] && [ "${2:-}" = "--cask" ] && [ "${3:-}" = "--json=v2" ]; then
    local cask="${4:-}"
    [ -n "$cask" ] || return 1
    case "$cask" in
      docker)
        printf '%s\n' '{"casks":[{"artifacts":[{"app":["Docker.app"]}]}]}'
        ;;
      warp)
        printf '%s\n' '{"casks":[{"artifacts":[{"app":["Warp.app"]}]}]}'
        ;;
      *)
        printf '%s\n' '{"casks":[{"artifacts":[{"app":["Other.app"]}]}]}'
        ;;
    esac
    return 0
  fi

  if [ "${1:-}" = "upgrade" ]; then
    printf 'upgrade:%s\n' "$*" >> "$BREW_LOG"
    return 0
  fi

  if [ "${1:-}" = "install" ] && [ "${2:-}" = "--cask" ]; then
    shift 2
    local cask
    for cask in "$@"; do
      if is_installed_cask "$cask" && [ "$cask" = "docker" ]; then
        printf 'docker is running\n' >&2
        return 1
      fi
      if [ "$cask" = "raycast" ]; then
        printf 'Error: raycast download failed\n' >&2
        return 1
      fi
      printf 'install-cask:%s\n' "$cask" >> "$BREW_LOG"
    done
    return 0
  fi

  if [ "${1:-}" = "install" ]; then
    printf 'install-formula:%s\n' "$*" >> "$BREW_LOG"
    return 0
  fi

  printf 'unexpected brew call: %s\n' "$*" >&2
  return 1
}

gh() {
  case "$*" in
    *'.state'*)
      printf 'success\n'
      ;;
    *'.login'*)
      printf 'Nivl\n'
      ;;
    *)
      printf 'unexpected gh call: %s\n' "$*" >&2
      return 1
      ;;
  esac
}

osascript() {
  case "$*" in
    *'Docker'*)
      printf 'true\n'
      ;;
    *)
      printf 'false\n'
      ;;
  esac
}

pgrep() {
  case "$*" in
    *'Docker'*)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

main
EOF

output="$(TEST_TMP_DIR="$TMP_DIR" REPO_ROOT="$REPO_ROOT" bash "$TMP_DIR/harness.sh")"

assert_contains() {
  local needle="$1"
  local haystack="$2"
  if [[ "$haystack" != *"$needle"* ]]; then
    printf 'Expected to find "%s" in output:\n%s\n' "$needle" "$haystack" >&2
    exit 1
  fi
}

assert_not_contains() {
  local needle="$1"
  local haystack="$2"
  if [[ "$haystack" == *"$needle"* ]]; then
    printf 'Did not expect to find "%s" in output:\n%s\n' "$needle" "$haystack" >&2
    exit 1
  fi
}

brew_log="$(cat "$TMP_DIR/brew.log")"
first_brew_call="$(printf '%s\n' "$brew_log" | sed -n '1p')"

assert_contains 'upgrade:upgrade' "$brew_log"
assert_contains 'install-formula:install gnupg diff-so-fancy emacs pinentry-mac jq less grep zsh-syntax-highlighting shellcheck lsd gh' "$brew_log"
assert_contains 'install-cask:zoom' "$brew_log"
assert_contains 'install-cask:warp' "$brew_log"
assert_contains 'install-cask:keka' "$brew_log"
assert_not_contains 'install-cask:docker' "$brew_log"
assert_not_contains 'install-cask:raycast' "$brew_log"
assert_contains 'upgrade:upgrade' "$first_brew_call"
assert_contains 'Skipped cask updates' "$output"
assert_contains 'docker' "$output"
assert_contains 'Failed cask installs or upgrades' "$output"
assert_contains 'raycast' "$output"
assert_contains 'Error: raycast download failed' "$output"
