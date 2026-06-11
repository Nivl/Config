#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/deny-env-vars.py: feeds it
# PreToolUse Bash payloads and asserts the permission decision.
#   $TMPDIR expanded in any form the shell honors -> deny
#   literals, escapes, assignments, other vars    -> silent
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/deny-env-vars.py"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

decision() {
  local payload out
  payload="$(jq -nc --arg c "$1" '{tool_input: {command: $c}}')"
  out="$(printf '%s' "$payload" | python3 "$HOOK")"
  if [[ -z "$out" ]]; then
    echo "silent"
  else
    jq -r '.hookSpecificOutput.permissionDecision' <<<"$out"
  fi
}

reason() {
  local payload out
  payload="$(jq -nc --arg c "$1" '{tool_input: {command: $c}}')"
  out="$(printf '%s' "$payload" | python3 "$HOOK")"
  if [[ -z "$out" ]]; then
    echo ""
  else
    jq -r '.hookSpecificOutput.permissionDecisionReason' <<<"$out"
  fi
}

# ---- Deny: every shape the shell would expand ----
assert_eq "deny_bare" "deny" "$(decision 'ls $TMPDIR')"
assert_eq "deny_braced" "deny" "$(decision 'ls ${TMPDIR}')"
assert_eq "deny_double_quoted" "deny" "$(decision 'ls "$TMPDIR"')"
assert_eq "deny_braced_quoted" "deny" "$(decision 'ls "${TMPDIR}/probe"')"
assert_eq "deny_path_suffix" "deny" "$(decision 'mktemp -d $TMPDIR/probe.XXXXXX')"
assert_eq "deny_default_expansion" "deny" "$(decision 'echo ${TMPDIR:-/tmp}')"
assert_eq "deny_length_op" "deny" "$(decision 'echo ${#TMPDIR}')"
assert_eq "deny_mid_pipeline" "deny" "$(decision 'ls $TMPDIR | head -3')"

# ---- Silent: literal text the shell never expands ----
assert_eq "silent_single_quoted" "silent" "$(decision "ls '\$TMPDIR'")"
assert_eq "silent_rg_pattern" "silent" "$(decision "rg '\\\$TMPDIR' file.txt")"
assert_eq "silent_escaped_bare" "silent" "$(decision 'echo \$TMPDIR')"
assert_eq "silent_escaped_double_quoted" "silent" "$(decision 'echo "\$TMPDIR"')"

# ---- Silent: assignment only SETS the variable ----
assert_eq "silent_assignment" "silent" "$(decision 'TMPDIR=/tmp/claude ls /tmp')"

# ---- Silent: longer names sharing the prefix are different variables ----
assert_eq "silent_prefix_bare" "silent" "$(decision 'ls $TMPDIRS')"
assert_eq "silent_prefix_braced" "silent" "$(decision 'ls ${TMPDIRX}')"

# ---- Silent: only the banned list is denied ----
assert_eq "silent_other_var" "silent" "$(decision 'echo $HOME')"
assert_eq "silent_plain_path" "silent" "$(decision 'ls /tmp/claude')"
assert_eq "silent_empty" "silent" "$(decision '')"

# ---- Reason names the variable and steers at the full path ----
assert_contains "reason_full_path" "use the full path directly" "$(reason 'ls $TMPDIR')"
assert_contains "reason_names_var" "TMPDIR" "$(reason 'ls $TMPDIR')"

echo "deny-env-vars.py: all tests passed"
