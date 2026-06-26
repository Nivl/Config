#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/deny-find-root.py: feeds it PreToolUse
# Bash payloads and asserts the permission decision.
#   find rooted at `/` (a search root) -> deny
#   anything else                      -> silent
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/deny-find-root.py"

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
  if [[ -z "$out" ]]; then echo ""; else jq -r '.hookSpecificOutput.permissionDecisionReason' <<<"$out"; fi
}

# ---- Deny: a `/` search root ----
assert_eq "deny_bare"        "deny" "$(decision "find /")"
assert_eq "deny_with_expr"   "deny" "$(decision "find / -name '*.ts'")"
assert_eq "deny_type"        "deny" "$(decision "find / -type f -mtime -1")"
assert_eq "deny_global_opt"  "deny" "$(decision "find -L / -type f")"
assert_eq "deny_abs_find"    "deny" "$(decision "/usr/bin/find / -name x")"
assert_eq "deny_gfind"       "deny" "$(decision "gfind / -name x")"
assert_eq "deny_double_slash" "deny" "$(decision "find // -name x")"
assert_eq "deny_dot_slash"   "deny" "$(decision "find /. -name x")"
assert_eq "deny_root_among"  "deny" "$(decision "find . / -name x")"
assert_eq "deny_group_after" "deny" "$(decision "find / \\( -name a -o -name b \\)")"

# ---- Deny: command-position other than the leading token ----
assert_eq "deny_piped"     "deny" "$(decision "echo x | find /")"
assert_eq "deny_chain_and" "deny" "$(decision "cd /tmp && find / -name x")"
assert_eq "deny_chain_semi" "deny" "$(decision "echo start; find / -name x")"

# ---- Reason must steer toward a focused find ----
assert_contains "reason_focus" "focused" "$(reason "find /")"
assert_contains "reason_dir"   "<dir>"   "$(reason "find /")"

# ---- Silent: focused finds ----
assert_eq "silent_dot"      "silent" "$(decision "find . -name '*.ts'")"
assert_eq "silent_repo"     "silent" "$(decision "find /Users/melvin/Dev/repos -type f")"
assert_eq "silent_etc"      "silent" "$(decision "find /etc -name hosts")"
assert_eq "silent_rel"      "silent" "$(decision "find src -name x")"
assert_eq "silent_no_path"  "silent" "$(decision "find -name x")"
assert_eq "silent_bare_find" "silent" "$(decision "find")"
assert_eq "silent_empty"    "silent" "$(decision "")"

# ---- Silent: `/` as an expression value, not a search root ----
assert_eq "silent_path_val"  "silent" "$(decision "find . -path / -prune")"
assert_eq "silent_newer_val" "silent" "$(decision "find src -newer /")"

# ---- Silent: find as a plain argument (not at command-position) ----
assert_eq "silent_echo"  "silent" "$(decision "echo find /")"
assert_eq "silent_which" "silent" "$(decision "which find")"

# ---- Silent: look-alikes ($ anchor in FIND_RE protects against these) ----
assert_eq "silent_findutils" "silent" "$(decision "findutils / x")"
assert_eq "silent_myfind"    "silent" "$(decision "myfind / x")"

# ---- Silent: documented wrapper-prefix carve-outs (mirrors deny-awk.py) ----
assert_eq "silent_env_wrapped"   "silent" "$(decision "env find /")"
assert_eq "silent_sudo_wrapped"  "silent" "$(decision "sudo find /")"
assert_eq "silent_xargs_wrapped" "silent" "$(decision "xargs find /")"
assert_eq "silent_varassign"     "silent" "$(decision "FOO=1 find /")"

echo "deny-find-root.py: all tests passed"
