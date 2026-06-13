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
assert_eq "deny_indirection_op" "deny" "$(decision 'echo ${!TMPDIR}')"
assert_eq "deny_mid_pipeline" "deny" "$(decision 'ls $TMPDIR | head -3')"
# A non-banned expansion earlier in the command must not stop the scan.
assert_eq "deny_after_other_var" "deny" "$(decision 'cp $HOME/notes $TMPDIR')"
# $$ consumes two chars; a third $ starts a live expansion.
assert_eq "deny_pid_then_live" "deny" "$(decision 'echo $$$TMPDIR')"
# Inside an ANSI-C string, backslash-quote does not end the string —
# the expansion after the real closing quote is live.
assert_eq "deny_ansi_c_escaped_quote_then_live" "deny" "$(decision $'echo $\'\\\'\' $TMPDIR')"
# The shell lexer joins backslash-newline continuations before
# tokenizing, so a split name still expands.
assert_eq "deny_line_continuation_bare" "deny" "$(decision $'ls $TMP\\\nDIR')"
assert_eq "deny_line_continuation_braced" "deny" "$(decision $'ls ${TMP\\\nDIR}')"
# An escaped backslash before the newline is NOT a continuation: the
# shell keeps a literal backslash plus a real newline, and the
# expansion on the next line is live.
assert_eq "deny_escaped_backslash_newline" "deny" "$(decision $'echo "\\\\\n$TMPDIR"')"
# The Bash tool executes via zsh, where bare $#NAME is csh-style
# length expansion — a real read of the variable (bash would read $#).
assert_eq "deny_zsh_length_prefix" "deny" "$(decision 'echo $#TMPDIR')"
# zsh flag groups and operator prefixes also read the variable.
assert_eq "deny_zsh_flag_braced" "deny" "$(decision 'echo ${(P)TMPDIR}')"
assert_eq "deny_zsh_op_braced" "deny" "$(decision 'ls ${=TMPDIR}')"
assert_eq "deny_zsh_op_bare" "deny" "$(decision 'ls $=TMPDIR')"
assert_eq "deny_zsh_glob_bare" "deny" "$(decision 'ls $~TMPDIR/probe')"
# A paren-delimited flag argument must not end the flag group early.
assert_eq "deny_zsh_flag_nested_paren" "deny" "$(decision 'echo ${(j(,))TMPDIR}')"
assert_eq "deny_zsh_split_nested_paren" "deny" "$(decision 'echo ${(s(,))TMPDIR}')"
# Inside double quotes $' is not ANSI-C quoting — the content after it
# still expands.
assert_eq "deny_ansi_c_marker_in_double" "deny" "$(decision 'echo "$'\''$TMPDIR'\''"')"
# A closing apostrophe inside double quotes must not flip single-quote
# state — the expansion after it is live.
assert_eq "deny_apostrophe_in_double" "deny" "$(decision "echo \"it's here\" \$TMPDIR")"
# The first backslash escapes the second, leaving the expansion live.
assert_eq "deny_double_backslash" "deny" "$(decision 'echo \\$TMPDIR')"

# ---- Deny: known false positive, documented in the hook header ----
# A quoted-delimiter heredoc body is literal to the shell, but the
# scanner has no heredoc awareness and denies anyway. Pinned so a future
# heredoc-aware scanner flips this case deliberately, not by accident.
assert_eq "deny_quoted_heredoc_body" "deny" "$(decision $'cat <<\'EOF\' > /tmp/claude/note.md\nls $TMPDIR\nEOF')"

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

# ---- Silent: bare $!/$$ are special parameters, the name after them is
# literal text in bash and zsh alike (bare $#NAME denies — see the
# deny section — and the braced ${#TMPDIR}/${!TMPDIR} forms also read
# TMPDIR) ----
assert_eq "silent_bg_pid_prefix" "silent" "$(decision 'echo $!TMPDIR')"
assert_eq "silent_pid_prefix" "silent" "$(decision 'echo $$TMPDIR')"
assert_eq "silent_pid_braced" "silent" "$(decision 'echo $${TMPDIR}')"

# ---- Silent: the line-continuation join also fires inside single
# quotes, but the joined name still sits in a skipped literal region ----
assert_eq "silent_line_continuation_in_single" "silent" "$(decision $'ls \'$TMP\\\nDIR\'')"

# ---- Silent: ANSI-C strings never expand their content ----
assert_eq "silent_ansi_c_literal" "silent" "$(decision "echo \$'\$TMPDIR'")"

# ---- Silent: only the banned list is denied ----
assert_eq "silent_other_var" "silent" "$(decision 'echo $HOME')"
assert_eq "silent_zsh_flag_other_var" "silent" "$(decision 'echo ${(P)HOME}')"
assert_eq "silent_plain_path" "silent" "$(decision 'ls /tmp/claude')"
assert_eq "silent_empty" "silent" "$(decision '')"

# ---- Reason names the variable, steers at the full path, and gives
# the sanctioned destination ----
assert_contains "reason_full_path" "use the full path directly" "$(reason 'ls $TMPDIR')"
assert_contains "reason_names_var" "TMPDIR" "$(reason 'ls $TMPDIR')"
assert_contains "reason_names_destination" "/tmp/claude/" "$(reason 'ls $TMPDIR')"

echo "deny-env-vars.py: all tests passed"
