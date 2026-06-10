#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/deny-bash-script.py: feeds it
# PreToolUse Bash payloads against a fixture allowedRoots config
# (pointed to by DENY_BASH_SCRIPT_DIR) and asserts the decision.
#   shell + script under an allowed root, run alone -> allow
#   shell + script anywhere else                    -> deny
#   compound/piped/stdin shell invocations          -> deny
#   -c shapes, non-shells, arg-position shells      -> silent
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/deny-bash-script.py"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

FIX="$(mktemp -d /tmp/deny_bash_script_test.XXXXXX)"
ROOT="$FIX/allowed"
OUTSIDE="$FIX/outside"
mkdir -p "$ROOT/nested" "$OUTSIDE" "$FIX/empty"
printf '#!/bin/bash\ntrue\n' >"$ROOT/test.sh"
printf '#!/bin/bash\ntrue\n' >"$ROOT/nested/inner.sh"
printf '#!/bin/bash\ntrue\n' >"$OUTSIDE/evil.sh"
export DENY_BASH_SCRIPT_DIR="$FIX"
cat >"$FIX/deny-bash-script.json" <<JSON
{"allowedRoots": ["$ROOT"]}
JSON

decision() {
  local payload out
  payload="$(jq -nc --arg c "$1" --arg d "${2:-$FIX}" '{tool_input: {command: $c}, cwd: $d}')"
  out="$(printf '%s' "$payload" | python3 "$HOOK")"
  if [[ -z "$out" ]]; then
    echo "silent"
  else
    jq -r '.hookSpecificOutput.permissionDecision' <<<"$out"
  fi
}

reason() {
  local payload out
  payload="$(jq -nc --arg c "$1" --arg d "${2:-$FIX}" '{tool_input: {command: $c}, cwd: $d}')"
  out="$(printf '%s' "$payload" | python3 "$HOOK")"
  if [[ -z "$out" ]]; then
    echo ""
  else
    jq -r '.hookSpecificOutput.permissionDecisionReason' <<<"$out"
  fi
}

# ---- Allow: lone script under the allowed root ----
assert_eq "allow_abs" "allow" "$(decision "bash $ROOT/test.sh")"
assert_eq "allow_nested" "allow" "$(decision "bash $ROOT/nested/inner.sh")"
assert_eq "allow_relative_cwd" "allow" "$(decision "bash test.sh" "$ROOT")"
assert_eq "allow_sh" "allow" "$(decision "sh $ROOT/test.sh")"
assert_eq "allow_zsh" "allow" "$(decision "zsh $ROOT/test.sh")"
assert_eq "allow_abs_shell" "allow" "$(decision "/bin/bash $ROOT/test.sh")"
assert_eq "allow_with_flag" "allow" "$(decision "bash -x $ROOT/test.sh")"
assert_eq "allow_with_args" "allow" "$(decision "bash $ROOT/test.sh /tmp/fixture.json verbose")"
# Redirects after the script stay allowed: the script runs sandboxed, so
# the kernel already blocks writes outside the write roots.
assert_eq "allow_with_redirect" "allow" "$(decision "bash $ROOT/test.sh > /tmp/out.txt")"
assert_eq "allow_with_stderr_dup" "allow" "$(decision "bash $ROOT/test.sh 2>&1")"

# ---- Deny: script outside the allowed root ----
assert_eq "deny_outside" "deny" "$(decision "bash $OUTSIDE/evil.sh")"
assert_eq "deny_tmp" "deny" "$(decision "bash /tmp/whatever.sh")"
assert_eq "deny_relative_outside" "deny" "$(decision "bash evil.sh" "$OUTSIDE")"
assert_eq "deny_traversal" "deny" "$(decision "bash $ROOT/../outside/evil.sh")"

# ---- Deny: no vettable script file ----
assert_eq "deny_bare" "deny" "$(decision "bash")"
assert_eq "deny_stdin_redirect" "deny" "$(decision "bash < $ROOT/test.sh")"

# ---- Deny: compound — allowed-root scripts must run alone ----
assert_eq "deny_piped" "deny" "$(decision "bash $ROOT/test.sh | head -3")"
assert_eq "deny_chain_and" "deny" "$(decision "true && bash $ROOT/test.sh")"
assert_eq "deny_chain_semi" "deny" "$(decision "echo x; bash $ROOT/test.sh")"
assert_eq "deny_pipe_into" "deny" "$(decision "echo true | bash")"
assert_eq "deny_subshell" "deny" "$(decision "(bash $ROOT/test.sh)")"

# ---- Deny: fail closed when the config is missing ----
assert_eq "deny_no_config" "deny" "$(DENY_BASH_SCRIPT_DIR="$FIX/empty" decision "bash $ROOT/test.sh")"

# ---- Silent: -c shapes belong to deny-shell-wrapper.py ----
assert_eq "silent_dash_c" "silent" "$(decision "bash -c 'echo hi'")"
assert_eq "silent_dash_lc" "silent" "$(decision "bash -lc 'echo hi'")"

# ---- Silent: shell not at command-position ----
assert_eq "silent_echo_bash" "silent" "$(decision "echo bash")"
assert_eq "silent_which_bash" "silent" "$(decision "which bash")"
assert_eq "silent_grep_bash" "silent" "$(decision "grep bash /etc/shells")"

# ---- Silent: documented assignment-prefix carve-out (falls to the prompt) ----
assert_eq "silent_assign_prefix" "silent" "$(decision "FOO=1 bash $ROOT/test.sh")"

# ---- Silent: lookalikes and non-shells ----
assert_eq "silent_ssh" "silent" "$(decision "ssh host uptime")"
assert_eq "silent_bash_suffix" "silent" "$(decision "bash-upgrade --check")"
assert_eq "silent_empty" "silent" "$(decision "")"

# ---- Reasons steer at the escape and the split pattern ----
assert_contains "reason_names_config" "deny-bash-script.json" "$(reason "bash $OUTSIDE/evil.sh")"
assert_contains "reason_run_directly" "directly" "$(reason "bash $OUTSIDE/evil.sh")"
assert_contains "reason_alone" "ALONE" "$(reason "bash $ROOT/test.sh | head -3")"

echo "deny-bash-script.py: all tests passed"
