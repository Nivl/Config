#!/bin/bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

# =============================================================================
# Helpers
# =============================================================================

assert_contains() {
  local label="$1"
  local needle="$2"
  local haystack="$3"
  if [[ "$haystack" != *"$needle"* ]]; then
    printf '[%s] Expected to find "%s" in:\n%s\n' "$label" "$needle" "$haystack" >&2
    exit 1
  fi
}

assert_not_contains() {
  local label="$1"
  local needle="$2"
  local haystack="$3"
  if [[ "$haystack" == *"$needle"* ]]; then
    printf '[%s] Did not expect to find "%s" in:\n%s\n' "$label" "$needle" "$haystack" >&2
    exit 1
  fi
}

# =============================================================================
# Shared harness template — written once, reused across scenarios
# =============================================================================
# The harness sources all install.sh functions (minus the trailing `main` call)
# and stubs out anything that would touch real system state.
#
# Scenario-specific behaviour is injected via env vars:
#   MOCK_CONFIG_EXISTS   — non-empty → pre-create CONFIG_DIR before main()
#   MOCK_UPDATE_RETURN   — exit code returned by update_config_repo (default 0)
#   MOCK_REEXEC_SCRIPT   — path written as "$CONFIG_DIR/install.sh" for re-exec
#
# Outputs appended to $TEST_TMP_DIR/brew.log for assertion.

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

clone_config_repo() {
  mkdir -p "$CONFIG_DIR"
  printf 'clone_config_repo\n' >> "$BREW_LOG"
}

update_config_repo() {
  printf 'update_config_repo\n' >> "$BREW_LOG"
  return "${MOCK_UPDATE_RETURN:-0}"
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

reload_zshrc() {
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

# Pre-create CONFIG_DIR when testing update paths.
if [ -n "${MOCK_CONFIG_EXISTS:-}" ]; then
  mkdir -p "$CONFIG_DIR"
  if [ -n "${MOCK_REEXEC_SCRIPT:-}" ]; then
    cp "$MOCK_REEXEC_SCRIPT" "$CONFIG_DIR/install.sh"
  fi
fi

main
EOF

# =============================================================================
# Scenario 1: Fresh install — CONFIG_DIR does not exist
# Expects clone_config_repo to run, full brew install to proceed.
# =============================================================================

SCENARIO="fresh-install"
SCENARIO_DIR="$TMP_DIR/$SCENARIO"
mkdir -p "$SCENARIO_DIR"

output="$(TEST_TMP_DIR="$SCENARIO_DIR" REPO_ROOT="$REPO_ROOT" bash "$TMP_DIR/harness.sh")"

brew_log="$(cat "$SCENARIO_DIR/brew.log")"
first_brew_call="$(printf '%s\n' "$brew_log" | sed -n '1p')"

assert_contains     "$SCENARIO" 'upgrade:upgrade'                    "$brew_log"
assert_contains     "$SCENARIO" 'install-formula:install gnupg diff-so-fancy emacs pinentry-mac jq less grep zsh-syntax-highlighting shellcheck lsd gh' "$brew_log"
assert_contains     "$SCENARIO" 'install-cask:zoom'                  "$brew_log"
assert_contains     "$SCENARIO" 'install-cask:warp'                  "$brew_log"
assert_contains     "$SCENARIO" 'install-cask:keka'                  "$brew_log"
assert_not_contains "$SCENARIO" 'install-cask:docker'                "$brew_log"
assert_not_contains "$SCENARIO" 'install-cask:raycast'               "$brew_log"
assert_contains     "$SCENARIO" 'clone_config_repo'                  "$brew_log"
assert_not_contains "$SCENARIO" 'update_config_repo'                 "$brew_log"
assert_contains     "$SCENARIO" 'upgrade:upgrade'                    "$first_brew_call"
assert_contains     "$SCENARIO" 'Skipped cask updates'               "$output"
assert_contains     "$SCENARIO" 'docker'                             "$output"
assert_contains     "$SCENARIO" 'Failed cask installs or upgrades'   "$output"
assert_contains     "$SCENARIO" 'raycast'                            "$output"
assert_contains     "$SCENARIO" 'Error: raycast download failed'     "$output"

# =============================================================================
# Scenario 2: Update — CONFIG_DIR exists, no new commits (update returns 0)
# Expects update_config_repo to run, full brew install to proceed.
# clone_config_repo must NOT run (CONFIG_DIR already present).
# =============================================================================

SCENARIO="update-no-new-commits"
SCENARIO_DIR="$TMP_DIR/$SCENARIO"
mkdir -p "$SCENARIO_DIR"

output="$(TEST_TMP_DIR="$SCENARIO_DIR" REPO_ROOT="$REPO_ROOT" \
  MOCK_CONFIG_EXISTS=1 MOCK_UPDATE_RETURN=0 \
  bash "$TMP_DIR/harness.sh")"

brew_log="$(cat "$SCENARIO_DIR/brew.log")"

assert_contains     "$SCENARIO" 'upgrade:upgrade'                    "$brew_log"
assert_contains     "$SCENARIO" 'install-formula:install gnupg diff-so-fancy emacs pinentry-mac jq less grep zsh-syntax-highlighting shellcheck lsd gh' "$brew_log"
assert_contains     "$SCENARIO" 'install-cask:zoom'                  "$brew_log"
assert_contains     "$SCENARIO" 'install-cask:warp'                  "$brew_log"
assert_contains     "$SCENARIO" 'install-cask:keka'                  "$brew_log"
assert_not_contains "$SCENARIO" 'install-cask:docker'                "$brew_log"
assert_not_contains "$SCENARIO" 'install-cask:raycast'               "$brew_log"
assert_contains     "$SCENARIO" 'update_config_repo'                 "$brew_log"
assert_not_contains "$SCENARIO" 'clone_config_repo'                  "$brew_log"
assert_contains     "$SCENARIO" 'Skipped cask updates'               "$output"
assert_contains     "$SCENARIO" 'Failed cask installs or upgrades'   "$output"

# =============================================================================
# Scenario 3: Re-exec — CONFIG_DIR exists and new commits were fetched
# Expects the process to exec into the updated install.sh without completing
# the original brew install.  The fake install.sh writes a marker file.
# =============================================================================

SCENARIO="reexec-after-pull"
SCENARIO_DIR="$TMP_DIR/$SCENARIO"
mkdir -p "$SCENARIO_DIR"

# Fake updated install.sh that signals it was invoked.
FAKE_INSTALL="$TMP_DIR/fake_install.sh"
cat > "$FAKE_INSTALL" <<'FAKE'
#!/bin/bash
printf 'reexec-ran\n'
printf 'reexec\n' > "$TEST_TMP_DIR/reexec.marker"
FAKE

output="$(TEST_TMP_DIR="$SCENARIO_DIR" REPO_ROOT="$REPO_ROOT" \
  MOCK_CONFIG_EXISTS=1 MOCK_UPDATE_RETURN=1 \
  MOCK_REEXEC_SCRIPT="$FAKE_INSTALL" \
  bash "$TMP_DIR/harness.sh")"

brew_log="$(cat "$SCENARIO_DIR/brew.log")"

# update_config_repo was called
assert_contains     "$SCENARIO" 'update_config_repo'                 "$brew_log"
# Brew install did NOT proceed (exec replaced the process)
assert_not_contains "$SCENARIO" 'upgrade:upgrade'                    "$brew_log"
assert_not_contains "$SCENARIO" 'install-formula'                    "$brew_log"
assert_not_contains "$SCENARIO" 'install-cask'                       "$brew_log"
# The re-execed script ran
assert_contains     "$SCENARIO" 'reexec-ran'                         "$output"
if [ ! -f "$SCENARIO_DIR/reexec.marker" ]; then
  printf '[%s] Expected reexec.marker to exist\n' "$SCENARIO" >&2
  exit 1
fi

# =============================================================================
# Scenario 4: SKIP_CONFIG_FILE_SETUP pre-set — setup_skip_existing_flag
# respects it and does not reset the flag.
# =============================================================================

SCENARIO="skip-flag-preset"
SCENARIO_DIR="$TMP_DIR/$SCENARIO"
mkdir -p "$SCENARIO_DIR"

output="$(TEST_TMP_DIR="$SCENARIO_DIR" REPO_ROOT="$REPO_ROOT" \
  SKIP_CONFIG_FILE_SETUP=true \
  bash "$TMP_DIR/harness.sh")"

brew_log="$(cat "$SCENARIO_DIR/brew.log")"

# Full install should still complete
assert_contains "$SCENARIO" 'upgrade:upgrade'   "$brew_log"
assert_contains "$SCENARIO" 'install-cask:zoom' "$brew_log"
