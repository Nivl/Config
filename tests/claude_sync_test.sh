#!/bin/bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

# install.sh sources its functions; we strip the trailing `main` call so that
# sourcing only defines functions instead of running the full installer.
sed '$d' "$REPO_ROOT/install.sh" > "$TMP_DIR/install_funcs.sh"

# =============================================================================
# Assertions
# =============================================================================

fail() {
  printf '\n[FAIL] %s\n' "$1" >&2
  if [ -n "${2:-}" ]; then printf '%s\n' "$2" >&2; fi
  exit 1
}

assert_eq() {
  local label="$1" expected="$2" actual="$3"
  if [ "$expected" != "$actual" ]; then
    fail "$label: expected '$expected', got '$actual'"
  fi
}

assert_file_exists() {
  local label="$1" path="$2"
  if [ ! -e "$path" ]; then fail "$label: file not found: $path"; fi
}

assert_file_absent() {
  local label="$1" path="$2"
  if [ -e "$path" ]; then fail "$label: file unexpectedly present: $path"; fi
}

assert_symlink_to() {
  local label="$1" path="$2" target="$3"
  if [ ! -L "$path" ]; then fail "$label: not a symlink: $path"; fi
  local actual
  actual=$(readlink "$path")
  if [ "$actual" != "$target" ]; then
    fail "$label: symlink points to '$actual', expected '$target'"
  fi
}

# Compares a JSON value at a given jq filter against the expected value (also JSON).
# Usage: assert_json_eq <label> <file> <jq-filter> <expected-json>
assert_json_eq() {
  local label="$1" file="$2" filter="$3" expected="$4"
  local actual
  actual=$(jq -c "$filter" "$file")
  if [ "$actual" != "$expected" ]; then
    fail "$label: jq '$filter' on $file = $actual, expected $expected"
  fi
}

assert_match_regex() {
  local label="$1" regex="$2" haystack="$3"
  if ! [[ "$haystack" =~ $regex ]]; then
    fail "$label: '$haystack' does not match /$regex/"
  fi
}

# =============================================================================
# Per-scenario environment
# =============================================================================
# Each scenario gets its own ephemeral $TEST_HOME/.claude/ and $TEST_REPO/.claude/.
# $TEST_REPO is a real git repo so claude_show_base / claude_base_has work.

scenario_setup() {
  local name="$1"
  TEST_DIR="$TMP_DIR/$name"
  TEST_HOME="$TEST_DIR/home"
  TEST_REPO="$TEST_DIR/repo"
  rm -rf "$TEST_DIR"
  mkdir -p "$TEST_HOME"
  mkdir -p "$TEST_REPO/.claude/skills" "$TEST_REPO/.claude/agents" "$TEST_REPO/.claude/commands"
  ( cd "$TEST_REPO" && \
      git init -q && \
      git config user.email t@t.com && \
      git config user.name t && \
      git config commit.gpgsign false )
}

# Source install.sh helpers in the current shell with the right CONFIG_DIR/HOME.
# Must be called inside a scenario subshell.
load_helpers() {
  source "$TMP_DIR/install_funcs.sh"
  CONFIG_DIR="$TEST_REPO"
  HOME="$TEST_HOME"
  # Defaults so `set -u` in the test runner doesn't trip on unset env vars
  # that install.sh tolerates as empty in normal use.
  SKIP_CLAUDE_MERGE_PROMPTS=""
  SKIP_CONFIG_FILE_SETUP=""
  PERSONAL_COMPUTER=""
  claude_init_paths
  mkdir -p "$CLAUDE_HOME_DIR"
  claude_ensure_state_dir
}

git_commit_in_repo() {
  ( cd "$TEST_REPO" && git add -A . && git commit -q --allow-empty -m "$1" )
}

# Returns the current HEAD SHA of the test repo
test_repo_head() {
  git -C "$TEST_REPO" rev-parse HEAD
}

# Driver
run_scenario() {
  local name="$1"
  shift
  printf '  • %-50s ' "$name"
  local logfile="$TMP_DIR/$name.log"
  if (
    set +o pipefail
    set -e
    scenario_setup "$name"
    "$@"
  ) > "$logfile" 2>&1; then
    printf 'PASS\n'
  else
    printf 'FAIL\n'
    cat "$logfile" >&2
    return 1
  fi
}

# =============================================================================
# Scenarios — symlink mode
# =============================================================================

scenario_personal_symlink_fresh() {
  load_helpers
  PERSONAL_COMPUTER=true
  SKIP_CONFIG_FILE_SETUP=true
  printf '{"model":"opus"}' > "$CLAUDE_REPO_DIR/settings.json"
  claude_setup
  assert_symlink_to "settings.json symlinked" "$CLAUDE_HOME_DIR/settings.json" "$CLAUDE_REPO_DIR/settings.json"
  assert_symlink_to "skills symlinked"        "$CLAUDE_HOME_DIR/skills"        "$CLAUDE_REPO_DIR/skills"
  assert_symlink_to "agents symlinked"        "$CLAUDE_HOME_DIR/agents"        "$CLAUDE_REPO_DIR/agents"
  assert_symlink_to "commands symlinked"      "$CLAUDE_HOME_DIR/commands"      "$CLAUDE_REPO_DIR/commands"
}

scenario_personal_symlink_existing_skipped() {
  load_helpers
  PERSONAL_COMPUTER=true
  SKIP_CONFIG_FILE_SETUP=true
  printf '{"model":"opus"}'   > "$CLAUDE_REPO_DIR/settings.json"
  printf '{"model":"sonnet"}' > "$CLAUDE_HOME_DIR/settings.json"
  claude_setup
  # SKIP_CONFIG_FILE_SETUP=true should leave the existing real file alone
  if [ -L "$CLAUDE_HOME_DIR/settings.json" ]; then
    fail "settings.json was symlinked despite SKIP_CONFIG_FILE_SETUP=true"
  fi
  assert_json_eq "existing local settings preserved" "$CLAUDE_HOME_DIR/settings.json" '.model' '"sonnet"'
}

# =============================================================================
# Scenarios — copy mode, settings merge
# =============================================================================

# Helper: commit the repo's settings.json + dirs and snapshot the SHA as
# "last-sync-commit" so subsequent edits to the repo are visible as "remote".
seed_repo_and_record_base() {
  local content="$1"
  printf '%s' "$content" > "$CLAUDE_REPO_DIR/settings.json"
  git_commit_in_repo "v1"
  test_repo_head > "$CLAUDE_LAST_SYNC_FILE"
}

scenario_copy_fresh_no_base() {
  load_helpers
  PERSONAL_COMPUTER=false
  printf '{"model":"opus","enabledPlugins":{"a":true}}' > "$CLAUDE_REPO_DIR/settings.json"
  git_commit_in_repo "v1"
  # No local file yet, no last-sync-commit
  rm -f "$CLAUDE_LAST_SYNC_FILE"
  claude_setup
  assert_file_exists "settings.json copied" "$CLAUDE_HOME_DIR/settings.json"
  assert_json_eq "model copied"   "$CLAUDE_HOME_DIR/settings.json" '.model' '"opus"'
  assert_json_eq "plugin a copied" "$CLAUDE_HOME_DIR/settings.json" '.enabledPlugins.a' 'true'
  assert_file_exists "last-sync-commit written" "$CLAUDE_LAST_SYNC_FILE"
  assert_eq "last-sync-commit matches HEAD" "$(test_repo_head)" "$(cat "$CLAUDE_LAST_SYNC_FILE")"
}

scenario_update_remote_add_plugin_no_prompt() {
  load_helpers
  PERSONAL_COMPUTER=false
  seed_repo_and_record_base '{"model":"opus","enabledPlugins":{"a":true}}'
  cp "$CLAUDE_REPO_DIR/settings.json" "$CLAUDE_HOME_DIR/settings.json"
  # Repo adds plugin b
  printf '{"model":"opus","enabledPlugins":{"a":true,"b":true}}' > "$CLAUDE_REPO_DIR/settings.json"
  git_commit_in_repo "add b"
  # Run merge with no env override — must not prompt because no real conflict
  claude_setup < /dev/null
  assert_json_eq "plugin a kept"  "$CLAUDE_HOME_DIR/settings.json" '.enabledPlugins.a' 'true'
  assert_json_eq "plugin b added" "$CLAUDE_HOME_DIR/settings.json" '.enabledPlugins.b' 'true'
}

scenario_update_local_add_plugin_no_prompt() {
  load_helpers
  PERSONAL_COMPUTER=false
  seed_repo_and_record_base '{"enabledPlugins":{"a":true}}'
  # Disk has the user-added plugin x; repo unchanged
  printf '{"enabledPlugins":{"a":true,"x":true}}' > "$CLAUDE_HOME_DIR/settings.json"
  claude_setup < /dev/null
  assert_json_eq "user-added plugin x kept" "$CLAUDE_HOME_DIR/settings.json" '.enabledPlugins.x' 'true'
  assert_json_eq "plugin a still there"     "$CLAUDE_HOME_DIR/settings.json" '.enabledPlugins.a' 'true'
}

scenario_update_smart_remote_modify_no_prompt() {
  load_helpers
  PERSONAL_COMPUTER=false
  seed_repo_and_record_base '{"model":"opus"}'
  cp "$CLAUDE_REPO_DIR/settings.json" "$CLAUDE_HOME_DIR/settings.json"
  # Repo bumps model to sonnet; disk still matches base — smart auto-update.
  printf '{"model":"sonnet"}' > "$CLAUDE_REPO_DIR/settings.json"
  git_commit_in_repo "bump model"
  claude_setup < /dev/null
  assert_json_eq "model auto-updated by smart rule" "$CLAUDE_HOME_DIR/settings.json" '.model' '"sonnet"'
}

scenario_update_true_conflict_keep_local() {
  load_helpers
  PERSONAL_COMPUTER=false
  SKIP_CLAUDE_MERGE_PROMPTS=keep-local
  seed_repo_and_record_base '{"model":"opus"}'
  printf '{"model":"sonnet"}' > "$CLAUDE_HOME_DIR/settings.json"
  printf '{"model":"haiku"}'  > "$CLAUDE_REPO_DIR/settings.json"
  git_commit_in_repo "bump model differently"
  claude_setup < /dev/null
  assert_json_eq "local model preserved" "$CLAUDE_HOME_DIR/settings.json" '.model' '"sonnet"'
}

scenario_update_true_conflict_take_remote() {
  load_helpers
  PERSONAL_COMPUTER=false
  SKIP_CLAUDE_MERGE_PROMPTS=take-remote
  seed_repo_and_record_base '{"model":"opus"}'
  printf '{"model":"sonnet"}' > "$CLAUDE_HOME_DIR/settings.json"
  printf '{"model":"haiku"}'  > "$CLAUDE_REPO_DIR/settings.json"
  git_commit_in_repo "bump model differently"
  claude_setup < /dev/null
  assert_json_eq "remote model adopted" "$CLAUDE_HOME_DIR/settings.json" '.model' '"haiku"'
}

scenario_update_cached_decision() {
  load_helpers
  PERSONAL_COMPUTER=false
  seed_repo_and_record_base '{"model":"opus"}'
  printf '{"model":"sonnet"}' > "$CLAUDE_HOME_DIR/settings.json"
  printf '{"model":"haiku"}'  > "$CLAUDE_REPO_DIR/settings.json"
  git_commit_in_repo "bump model differently"
  # Pre-seed decision cache: always keep local for ["model"].
  # NOTE: pass the JSON via %s to avoid printf eating the \" escapes.
  printf '%s' '{"version":1,"settings":{"[\"model\"]":"local"},"files":{}}' > "$CLAUDE_DECISIONS_FILE"
  # No env override; should auto-resolve via cache.
  SKIP_CLAUDE_MERGE_PROMPTS=""
  claude_setup < /dev/null
  assert_json_eq "cached decision honored" "$CLAUDE_HOME_DIR/settings.json" '.model' '"sonnet"'
}

scenario_skipped_conflict_does_not_advance() {
  load_helpers
  PERSONAL_COMPUTER=false
  SKIP_CLAUDE_MERGE_PROMPTS=skip
  seed_repo_and_record_base '{"model":"opus"}'
  local original_sha
  original_sha=$(test_repo_head)
  printf '{"model":"sonnet"}' > "$CLAUDE_HOME_DIR/settings.json"
  printf '{"model":"haiku"}'  > "$CLAUDE_REPO_DIR/settings.json"
  git_commit_in_repo "bump model differently"
  claude_setup < /dev/null
  # Local file untouched; last-sync-commit must NOT advance
  assert_json_eq "local untouched" "$CLAUDE_HOME_DIR/settings.json" '.model' '"sonnet"'
  assert_eq "last-sync-commit did not advance" "$original_sha" "$(cat "$CLAUDE_LAST_SYNC_FILE")"
}

# =============================================================================
# Scenarios — skill / dir merges
# =============================================================================

scenario_skill_remote_add_no_prompt() {
  load_helpers
  PERSONAL_COMPUTER=false
  printf '{}' > "$CLAUDE_REPO_DIR/settings.json"
  git_commit_in_repo "v1"
  test_repo_head > "$CLAUDE_LAST_SYNC_FILE"
  # Repo adds a new skill file
  echo "new skill content" > "$CLAUDE_REPO_DIR/skills/new-skill.md"
  git_commit_in_repo "add new-skill"
  claude_setup < /dev/null
  assert_file_exists "remote-added skill copied" "$CLAUDE_HOME_DIR/skills/new-skill.md"
  assert_eq "skill content matches" "new skill content" "$(cat "$CLAUDE_HOME_DIR/skills/new-skill.md")"
}

scenario_skill_remote_delete_clean_no_prompt() {
  load_helpers
  PERSONAL_COMPUTER=false
  printf '{}' > "$CLAUDE_REPO_DIR/settings.json"
  echo "v1 content" > "$CLAUDE_REPO_DIR/skills/foo.md"
  git_commit_in_repo "v1"
  test_repo_head > "$CLAUDE_LAST_SYNC_FILE"
  # Disk has the v1 content unchanged
  echo "v1 content" > "$CLAUDE_HOME_DIR/skills/foo.md"
  # Repo removes the skill
  rm "$CLAUDE_REPO_DIR/skills/foo.md"
  git_commit_in_repo "remove foo"
  claude_setup < /dev/null
  assert_file_absent "skill deleted from disk" "$CLAUDE_HOME_DIR/skills/foo.md"
}

scenario_skill_modify_modify_conflict_with_backup() {
  load_helpers
  PERSONAL_COMPUTER=false
  SKIP_CLAUDE_MERGE_PROMPTS=keep-local
  printf '{}' > "$CLAUDE_REPO_DIR/settings.json"
  echo "base content" > "$CLAUDE_REPO_DIR/skills/foo.md"
  git_commit_in_repo "v1"
  test_repo_head > "$CLAUDE_LAST_SYNC_FILE"
  mkdir -p "$CLAUDE_HOME_DIR/skills"
  echo "local edit" > "$CLAUDE_HOME_DIR/skills/foo.md"
  echo "remote edit" > "$CLAUDE_REPO_DIR/skills/foo.md"
  git_commit_in_repo "remote edit"
  claude_setup < /dev/null
  assert_eq "local kept" "local edit" "$(cat "$CLAUDE_HOME_DIR/skills/foo.md")"
  # keep-local does NOT touch the file → no backup expected for keep-local.
  # Verify with take-remote variant separately.
}

scenario_skill_take_remote_creates_backup() {
  load_helpers
  PERSONAL_COMPUTER=false
  SKIP_CLAUDE_MERGE_PROMPTS=take-remote
  printf '{}' > "$CLAUDE_REPO_DIR/settings.json"
  echo "base content"   > "$CLAUDE_REPO_DIR/skills/foo.md"
  git_commit_in_repo "v1"
  test_repo_head > "$CLAUDE_LAST_SYNC_FILE"
  mkdir -p "$CLAUDE_HOME_DIR/skills"
  echo "local edit"  > "$CLAUDE_HOME_DIR/skills/foo.md"
  echo "remote edit" > "$CLAUDE_REPO_DIR/skills/foo.md"
  git_commit_in_repo "remote edit"
  claude_setup < /dev/null
  assert_eq "remote adopted" "remote edit" "$(cat "$CLAUDE_HOME_DIR/skills/foo.md")"
  local backup
  backup=$(ls "$CLAUDE_HOME_DIR/skills/" | grep '^foo\.md\.[0-9]\{14\}\.bkp$' || true)
  if [ -z "$backup" ]; then
    fail "no timestamped backup created"
  fi
  assert_match_regex "backup name format" "^foo\.md\.[0-9]{14}\.bkp\$" "$backup"
}

# =============================================================================
# Scenarios — edge cases
# =============================================================================

scenario_stale_last_sync_recovery() {
  load_helpers
  PERSONAL_COMPUTER=false
  printf '{"model":"opus"}' > "$CLAUDE_REPO_DIR/settings.json"
  git_commit_in_repo "v1"
  printf '{"model":"opus"}' > "$CLAUDE_HOME_DIR/settings.json"
  # Pretend last-sync-commit references an unreachable SHA
  echo "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" > "$CLAUDE_LAST_SYNC_FILE"
  # Should not crash; falls back to base=empty
  claude_setup < /dev/null
  assert_json_eq "settings still valid after stale-sha recovery" \
    "$CLAUDE_HOME_DIR/settings.json" '.model' '"opus"'
}

scenario_invalid_json_skips_settings() {
  load_helpers
  PERSONAL_COMPUTER=false
  printf '{"model":"opus"}' > "$CLAUDE_REPO_DIR/settings.json"
  echo "not valid json {{{" > "$CLAUDE_HOME_DIR/settings.json"
  git_commit_in_repo "v1"
  test_repo_head > "$CLAUDE_LAST_SYNC_FILE"
  # Should refuse to merge settings, NOT crash, and not modify the broken file.
  claude_setup < /dev/null
  assert_eq "broken settings.json untouched" \
    "not valid json {{{" "$(cat "$CLAUDE_HOME_DIR/settings.json")"
}

# =============================================================================
# Driver
# =============================================================================

main() {
  printf 'Running claude_sync_test.sh...\n'

  run_scenario personal_symlink_fresh                  scenario_personal_symlink_fresh
  run_scenario personal_symlink_existing_skipped       scenario_personal_symlink_existing_skipped
  run_scenario copy_fresh_no_base                      scenario_copy_fresh_no_base
  run_scenario update_remote_add_plugin_no_prompt      scenario_update_remote_add_plugin_no_prompt
  run_scenario update_local_add_plugin_no_prompt       scenario_update_local_add_plugin_no_prompt
  run_scenario update_smart_remote_modify_no_prompt    scenario_update_smart_remote_modify_no_prompt
  run_scenario update_true_conflict_keep_local         scenario_update_true_conflict_keep_local
  run_scenario update_true_conflict_take_remote        scenario_update_true_conflict_take_remote
  run_scenario update_cached_decision                  scenario_update_cached_decision
  run_scenario skipped_conflict_does_not_advance       scenario_skipped_conflict_does_not_advance
  run_scenario skill_remote_add_no_prompt              scenario_skill_remote_add_no_prompt
  run_scenario skill_remote_delete_clean_no_prompt     scenario_skill_remote_delete_clean_no_prompt
  run_scenario skill_modify_modify_conflict_with_bkp   scenario_skill_modify_modify_conflict_with_backup
  run_scenario skill_take_remote_creates_backup        scenario_skill_take_remote_creates_backup
  run_scenario stale_last_sync_recovery                scenario_stale_last_sync_recovery
  run_scenario invalid_json_skips_settings             scenario_invalid_json_skips_settings

  printf '\nAll scenarios passed.\n'
}

main "$@"
