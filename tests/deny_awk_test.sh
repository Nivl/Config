#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/deny-awk.py: feeds it PreToolUse
# Bash payloads and asserts the permission decision.
#   awk/gawk/mawk/nawk at any command-position -> deny
#   anything else                              -> silent
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/deny-awk.py"

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

# ---- Deny: every awk-family invocation at command-position ----
assert_eq "deny_awk_field" "deny" "$(decision "awk '{print \$1}' file")"
assert_eq "deny_awk_nf" "deny" "$(decision "awk '{print \$NF}'")"
assert_eq "deny_gawk" "deny" "$(decision "gawk 'NR<=80'")"
assert_eq "deny_mawk" "deny" "$(decision "mawk '/x/ {print}'")"
assert_eq "deny_nawk" "deny" "$(decision "nawk '{print}'")"
assert_eq "deny_abs_awk" "deny" "$(decision "/usr/bin/awk '{print \$1}'")"
assert_eq "deny_abs_gawk" "deny" "$(decision "/opt/homebrew/bin/gawk '{print \$1}'")"
assert_eq "deny_with_F_flag" "deny" "$(decision "awk -F: '/r/ {print \$1}' /etc/passwd")"

# ---- Deny: command-position other than the leading token ----
assert_eq "deny_piped" "deny" "$(decision "cat file | awk '{print \$1}'")"
assert_eq "deny_canonical" "deny" "$(decision "git remote show origin | grep 'HEAD branch' | awk '{print \$NF}'")"
assert_eq "deny_chain_and" "deny" "$(decision "echo start && awk '{print \$1}'")"
assert_eq "deny_chain_or" "deny" "$(decision "echo start || awk '{print \$1}'")"
assert_eq "deny_chain_semi" "deny" "$(decision "echo start; awk '{print \$1}'")"
assert_eq "deny_subshell" "deny" "$(decision "(awk '{print \$1}')")"
assert_eq "deny_brace_group" "deny" "$(decision "{ awk '{print \$1}'; }")"

# ---- Reason must name the safer alternatives ----
assert_contains "reason_cut" "cut" "$(reason "awk '{print \$1}' file")"
assert_contains "reason_sed" "sed" "$(reason "awk '{print \$1}' file")"
assert_contains "reason_symbolic_ref" "git symbolic-ref" "$(reason "awk '{print \$1}' file")"

# ---- Silent: not the awk family ----
assert_eq "silent_cut" "silent" "$(decision "cut -d: -f1 /etc/passwd")"
assert_eq "silent_sed" "silent" "$(decision "sed -n 's/x/y/p' file")"
assert_eq "silent_grep" "silent" "$(decision "grep pattern file")"
assert_eq "silent_head" "silent" "$(decision "head -n 5 file")"
assert_eq "silent_jq" "silent" "$(decision "jq . file.json")"
assert_eq "silent_empty" "silent" "$(decision "")"

# ---- Silent: awk as plain argument (not at command-position) ----
# Quoted occurrence collapses to one shlex token; basename doesn't match AWK_RE.
assert_eq "silent_echo_quoted" "silent" "$(decision "echo 'awk {print \$1}'")"
# Echo as leading command, awk as plain argument.
assert_eq "silent_echo_unquoted" "silent" "$(decision "echo awk")"
# `which awk` — awk is the arg of which, not at command-position.
assert_eq "silent_which_awk" "silent" "$(decision "which awk")"

# ---- Silent: substring look-alikes ($ anchor in AWK_RE protects against these) ----
assert_eq "silent_awk_suffix" "silent" "$(decision "awk-tools --foo")"
assert_eq "silent_gawkward" "silent" "$(decision "gawkward 'x'")"

# ---- Silent: documented wrapper-prefix carve-outs ----
# Wrappers aren't peeled here; the agent is redirected on the simpler form.
# Locked in so a future "peel wrappers here too" change has to update these.
assert_eq "silent_env_wrapped" "silent" "$(decision "env awk '{print \$1}'")"
assert_eq "silent_sudo_wrapped" "silent" "$(decision "sudo awk '{print \$1}'")"
assert_eq "silent_xargs_wrapped" "silent" "$(decision "xargs awk '{print \$1}'")"
assert_eq "silent_varassign_wrapped" "silent" "$(decision "PATH=/x awk '{print \$1}'")"

echo "deny-awk.py: all tests passed"
