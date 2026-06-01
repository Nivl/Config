#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/deny-multi-command.py: feeds it
# PreToolUse Bash payloads and asserts the permission decision.
#   `;` or newline-separated batch -> deny
#   single command / && / || / |   -> silent
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/deny-multi-command.py"

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

# ---- Deny: `;` chain (every shape) ----
assert_eq "deny_semi" "deny" "$(decision "git status; git log")"
assert_eq "deny_semi_no_space" "deny" "$(decision "git status;git log")"
assert_eq "deny_semi_three" "deny" "$(decision "a; b; c")"
assert_eq "deny_semi_with_pipe" "deny" "$(decision "cat x | grep y; rm /tmp/z")"
assert_eq "deny_double_semi" "deny" "$(decision "case x in a) echo a;; esac")"

# ---- Deny: `&` background separator (chains in spirit, same as `;`) ----
assert_eq "deny_amp_chain" "deny" "$(decision "sleep 1 & echo done")"
assert_eq "deny_amp_alone" "deny" "$(decision "sleep 1 &")"

# ---- Deny: command substitution containing chained statements ----
# The substitution body DOES execute multiple commands; the conservative
# read is to refuse the whole invocation. Lock in so a future "ignore
# substitution bodies" tweak has to address it.
assert_eq "deny_paren_sub_with_semi" "deny" "$(decision "echo \$(a; b)")"
assert_eq "deny_backtick_with_semi" "deny" "$(decision "echo \`a; b\`")"

# ---- Deny: newline chain (every shape) ----
nl_two=$'git status\ngit log'
nl_three=$'cmd1\ncmd2\ncmd3'
nl_with_indent=$'git status\n  git log'
nl_then_semi=$'git status\ngit log; echo done'
assert_eq "deny_newline_two" "deny" "$(decision "$nl_two")"
assert_eq "deny_newline_three" "deny" "$(decision "$nl_three")"
assert_eq "deny_newline_indented" "deny" "$(decision "$nl_with_indent")"
assert_eq "deny_newline_then_semi" "deny" "$(decision "$nl_then_semi")"

# ---- Deny: heredoc (acceptable trade-off — agent should use Write tool) ----
hd=$'cat <<EOF > /tmp/x\nbody\nEOF'
assert_eq "deny_heredoc" "deny" "$(decision "$hd")"

# ---- Reason must steer the agent at one-command-at-a-time alternatives ----
assert_contains "reason_one_at_a_time" "ONE" "$(reason "a; b")"
assert_contains "reason_conditional" "&&" "$(reason "a; b")"
assert_contains "reason_pipeline" "|" "$(reason "a; b")"

# ---- Silent: single command ----
assert_eq "silent_single" "silent" "$(decision "git status")"
assert_eq "silent_with_flags" "silent" "$(decision "git log --oneline -n 10")"
assert_eq "silent_with_redirect" "silent" "$(decision "git log > /tmp/log")"
assert_eq "silent_empty" "silent" "$(decision "")"

# ---- Silent: conditional chains and pipelines (semantics worth preserving) ----
assert_eq "silent_and_chain" "silent" "$(decision "git status && git log")"
assert_eq "silent_or_chain" "silent" "$(decision "git status || echo fail")"
assert_eq "silent_pipe" "silent" "$(decision "cat file | grep x")"
assert_eq "silent_pipe_chain" "silent" "$(decision "cat file | grep x | wc -l")"
assert_eq "silent_and_or_mixed" "silent" "$(decision "a && b || c")"
# `&&` and `&>` are distinct tokens from bare `&` under punctuation_chars=True.
assert_eq "silent_amp_redirect" "silent" "$(decision "cmd &> /tmp/log")"
assert_eq "silent_amp_amp_redirect" "silent" "$(decision "cmd &>> /tmp/log")"

# ---- Silent: `;` inside quoted strings is content, not a separator ----
assert_eq "silent_semi_in_double" "silent" "$(decision "echo \"a; b\"")"
assert_eq "silent_semi_in_single" "silent" "$(decision "echo 'a; b'")"
assert_eq "silent_semi_in_filename" "silent" "$(decision "cat '/tmp/file;weird'")"

# ---- Silent: line continuation (`\<newline>`) is a single command ----
cont=$'git log \\\n  --oneline \\\n  -n 10'
assert_eq "silent_line_continuation" "silent" "$(decision "$cont")"

# ---- Silent: tabs and spaces aren't command separators ----
tab_sep=$'echo first\techo second'
assert_eq "silent_tab" "silent" "$(decision "$tab_sep")"

echo "deny-multi-command.py: all tests passed"
