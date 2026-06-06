#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/deny-command-substitution.py: feeds it
# PreToolUse Bash payloads and asserts the permission decision.
#   active $(...) / backtick / <(...) / >(...)   -> deny
#   single-quoted literal, arithmetic, ${VAR}    -> silent
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/deny-command-substitution.py"

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

# ---- Deny: $(...) command substitution ----
assert_eq "deny_dollar_paren" "deny" "$(decision "grep x \$(find . -name y)")"
assert_eq "deny_dollar_paren_in_path" "deny" "$(decision "ls \$(pwd)/src")"
assert_eq "deny_dollar_paren_in_double" "deny" "$(decision "echo \"\$(date)\"")"
# Nested in a parameter-expansion default — it still runs the command.
assert_eq "deny_dollar_paren_in_default" "deny" "$(decision "echo \${VAR:-\$(id)}")"
# `$( (subshell) )` has a space after `$(`, so it is command sub, not arithmetic.
assert_eq "deny_dollar_paren_subshell" "deny" "$(decision "echo \$( (id) )")"
# Arithmetic first, then a real substitution later in the same command.
assert_eq "deny_arith_then_sub" "deny" "$(decision "echo \$((1 + 2)) \$(date)")"
# A substitution nested inside arithmetic still runs the inner command.
assert_eq "deny_sub_in_arithmetic" "deny" "$(decision "echo \$(( \$(id) + 1 ))")"

# ---- Deny: backtick command substitution ----
assert_eq "deny_backtick" "deny" "$(decision "echo \`date\`")"
assert_eq "deny_backtick_in_double" "deny" "$(decision "echo \"\`date\`\"")"

# ---- Deny: process substitution ----
assert_eq "deny_proc_sub_read" "deny" "$(decision "diff <(sort a) <(sort b)")"
assert_eq "deny_proc_sub_write" "deny" "$(decision "tee >(wc -l) < in")"

# ---- Deny: the real-world command that motivated this hook ----
assert_eq "deny_motivating_case" "deny" \
  "$(decision "grep -rn \"X\" \$(find . -type d -name errors | head -1)/src | grep -i y | head")"

# ---- Reason must steer the agent at the run-it-separately alternative ----
assert_contains "reason_mentions_substitution" "substitution" "$(reason "echo \$(date)")"
assert_contains "reason_own_call" "own Bash call" "$(reason "echo \$(date)")"

# ---- Silent: plain commands, no substitution ----
assert_eq "silent_plain" "silent" "$(decision "grep x file")"
assert_eq "silent_redirect" "silent" "$(decision "make build > /tmp/log")"
assert_eq "silent_stderr_redirect" "silent" "$(decision "cmd 2>&1")"
assert_eq "silent_append" "silent" "$(decision "cmd >> /tmp/log")"
assert_eq "silent_amp_redirect" "silent" "$(decision "cmd &> /tmp/log")"
assert_eq "silent_empty" "silent" "$(decision "")"

# ---- Silent: single-quoted text is literal, the shell never runs it ----
assert_eq "silent_dollar_paren_single" "silent" "$(decision "grep '\$(x)' file")"
assert_eq "silent_backtick_single" "silent" "$(decision "echo '\`date\`'")"
assert_eq "silent_proc_sub_single" "silent" "$(decision "echo '<(x)'")"

# ---- Silent: backslash-escaped, so not a substitution ----
assert_eq "silent_escaped_dollar_paren" "silent" "$(decision 'rg \$\( src')"
assert_eq "silent_escaped_in_single" "silent" "$(decision "rg '\$\\(' src")"
assert_eq "silent_escaped_backtick" "silent" "$(decision 'echo \`date\`')"

# ---- Deny: a backslash that consumes itself (\\) or precedes a non-special
# char inside double quotes leaves the following substitution live ----
assert_eq "deny_double_backslash_then_sub" "deny" "$(decision 'echo \\$(id)')"
assert_eq "deny_dq_backslash_nonspecial_then_sub" "deny" "$(decision 'echo "\x$(id)"')"

# ---- Silent: arithmetic expansion runs no command ----
assert_eq "silent_arithmetic" "silent" "$(decision "echo \$((1 + 2))")"
assert_eq "silent_arithmetic_nested" "silent" "$(decision "echo \$(((1 + 2) * 3))")"

# ---- Silent: parameter expansion is not a command ----
assert_eq "silent_param_expansion" "silent" "$(decision "echo \${HOME}/bin")"
assert_eq "silent_param_default" "silent" "$(decision "echo \${VAR:-default}")"

# ---- Silent: process-sub syntax inside double quotes is literal ----
assert_eq "silent_proc_sub_in_double" "silent" "$(decision "echo \"<(x)\"")"

# ---- Silent: lone < / > that aren't process substitution ----
assert_eq "silent_lt_no_paren" "silent" "$(decision "sort < input.txt")"

echo "deny-command-substitution.py: all tests passed"
