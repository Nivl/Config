#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/deny-git-init-in-repo.py.
#   git init / git config user.* with cwd inside a repo      -> deny
#   the same with cwd under a plain directory (a fixture)    -> silent
#   --global config, other git subcommands, non-git commands -> silent
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/deny-git-init-in-repo.py"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

FIX="/tmp/claude/deny-git-init-fixture-$$"
rm -rf "$FIX"
mkdir -p "$FIX/repo/sub" "$FIX/plain"
mkdir "$FIX/repo/.git"
trap 'rm -rf "$FIX"' EXIT

decision() {
  local payload out
  payload="$(jq -nc --arg c "$1" --arg d "$2" '{cwd: $d, tool_input: {command: $c}}')"
  out="$(printf '%s' "$payload" | python3 "$HOOK")"
  if [[ -z "$out" ]]; then echo "silent"; else jq -r '.hookSpecificOutput.permissionDecision' <<<"$out"; fi
}

R="$FIX/repo"; S="$FIX/repo/sub"; P="$FIX/plain"

assert_eq "deny_init_root" "deny" "$(decision 'git init' "$R")"
assert_eq "deny_init_subdir" "deny" "$(decision 'git init -q .' "$S")"
assert_eq "deny_config_email" "deny" "$(decision 'git config user.email t@example.com' "$R")"
assert_eq "deny_config_name_local" "deny" "$(decision 'git config --local user.name t' "$R")"
assert_eq "deny_chained" "deny" "$(decision 'mkdir x && git init' "$R")"
assert_eq "deny_env_prefix" "deny" "$(decision 'GIT_DIR=.g git init' "$R")"
printf -v MULTI "echo hi\ngit config user.email a@a.com"
assert_eq "deny_multiline" "deny" "$(decision "$MULTI" "$R")"
assert_eq "deny_global_in_next_command" "deny" "$(decision 'git config user.email a@a.com && git config --global core.pager cat' "$R")"
assert_eq "silent_read_then_chain" "silent" "$(decision 'git config user.name && echo x' "$R")"

assert_eq "silent_init_plain" "silent" "$(decision 'git init -q .' "$P")"
assert_eq "silent_config_plain" "silent" "$(decision 'git config user.email t@example.com' "$P")"
assert_eq "silent_global" "silent" "$(decision 'git config --global user.email t@example.com' "$R")"
assert_eq "silent_config_read" "silent" "$(decision 'git config user.email' "$R")"
assert_eq "silent_config_read_get" "silent" "$(decision 'git config --get user.name' "$R")"
assert_eq "silent_other_config" "silent" "$(decision 'git config core.autocrlf false' "$R")"
assert_eq "silent_status" "silent" "$(decision 'git status' "$R")"
assert_eq "silent_commit" "silent" "$(decision 'git commit -m init' "$R")"
assert_eq "silent_not_git" "silent" "$(decision 'echo git init' "$R")"

echo "deny-git-init-in-repo.py: all tests passed"
