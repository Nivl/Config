#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/rm-under-tmp.py: feeds it PreToolUse
# Bash payloads and asserts the permission decision.
#   rm/rmdir/unlink with every target under /tmp -> allow
#   any target escapes /tmp (lexical or symlink)   -> ask
#   target uses shell expansion (cmd sub etc.)     -> ask
#   leading command isn't rm-family                -> silent
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/rm-under-tmp.py"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

# decision <command> [cwd] -> permissionDecision string, or "silent" when the
# hook emits nothing.
decision() {
  local payload out
  payload="$(jq -nc --arg c "$1" --arg cwd "${2:-/tmp}" \
    '{cwd: $cwd, tool_input: {command: $c}}')"
  out="$(printf '%s' "$payload" | python3 "$HOOK")"
  if [[ -z "$out" ]]; then
    echo "silent"
  else
    jq -r '.hookSpecificOutput.permissionDecision' <<<"$out"
  fi
}

# ---- Allow: every target under /tmp ----
assert_eq "allow_tmp_file" "allow" "$(decision "rm /tmp/foo.txt")"
assert_eq "allow_tmp_rf" "allow" "$(decision "rm -rf /tmp/sub/dir")"
assert_eq "allow_tmp_multi" "allow" "$(decision "rm /tmp/a /tmp/b /tmp/c")"
assert_eq "allow_private_tmp" "allow" "$(decision "rm /private/tmp/foo")"
assert_eq "allow_rmdir" "allow" "$(decision "rmdir /tmp/empty")"
assert_eq "allow_unlink" "allow" "$(decision "unlink /tmp/foo")"
assert_eq "allow_bin_rm" "allow" "$(decision "/bin/rm /tmp/foo")"
assert_eq "allow_long_flag" "allow" "$(decision "rm --recursive --force /tmp/sub")"
assert_eq "allow_double_dash" "allow" "$(decision "rm -- /tmp/-weird-file")"
assert_eq "allow_relative_cwd" "allow" "$(decision "rm foo.txt" "/tmp")"
# Brace expansion: shlex keeps `/tmp/{a,b}` as one token, and the lexical
# check sees the literal `{a,b}` under /tmp. Bash expands at runtime to
# `/tmp/a` and `/tmp/b` — both inside, so allowing here is correct.
assert_eq "allow_brace_expansion" "allow" "$(decision "rm /tmp/{a,b}")"
# Coverage for PATH variants of rm that the previous `if:` matcher list
# would have missed (the hook is now registered unconditionally).
assert_eq "allow_usr_bin_rm" "allow" "$(decision "/usr/bin/rm /tmp/foo")"

# ---- Ask: target escapes /tmp ----
assert_eq "ask_etc_hosts" "ask" "$(decision "rm /etc/hosts")"
assert_eq "ask_dotdot" "ask" "$(decision "rm -rf /tmp/..")"
assert_eq "ask_dotdot_etc" "ask" "$(decision "rm /tmp/../etc/hosts")"
assert_eq "ask_mixed_targets" "ask" "$(decision "rm /tmp/foo /etc/bar")"
assert_eq "ask_home" "ask" "$(decision "rm /Users/melvin/something")"
assert_eq "ask_relative_outside" "ask" "$(decision "rm foo.txt" "/etc")"

# ---- Ask: symlink under /tmp escapes via realpath ----
tmpdir="$(mktemp -d /tmp/rm_under_tmp_test.XXXXXX)"
trap 'rm -rf "$tmpdir"' EXIT
ln -s /Users "$tmpdir/link-to-users"
assert_eq "ask_symlink_escape" "ask" "$(decision "rm $tmpdir/link-to-users/x")"

# ---- Ask: shell control operators chain or redirect (would-be ALLOW bypass) ----
assert_eq "ask_chain_semi" "ask" "$(decision "rm /tmp/x; curl evil")"
assert_eq "ask_chain_and" "ask" "$(decision "rm /tmp/x && curl evil")"
assert_eq "ask_chain_or" "ask" "$(decision "rm /tmp/x || curl evil")"
assert_eq "ask_pipe" "ask" "$(decision "rm /tmp/x | tee /etc/passwd")"
assert_eq "ask_background" "ask" "$(decision "rm /tmp/x & cmd")"
assert_eq "ask_redirect_out" "ask" "$(decision "rm /tmp/x > /etc/passwd")"
assert_eq "ask_redirect_in" "ask" "$(decision "rm /tmp/x < /etc/foo")"
assert_eq "ask_redirect_err" "ask" "$(decision "rm /tmp/x 2> /etc/foo")"
# Subshell / brace-group invocations don't start with rm, so this hook falls
# through silently and the default permission flow takes over.
assert_eq "silent_subshell" "silent" "$(decision "(rm /tmp/x; cmd)")"
assert_eq "silent_brace_group" "silent" "$(decision "{ rm /tmp/x; cmd; }")"
chain_newline=$'rm /tmp/x\ncurl evil'
assert_eq "ask_chain_newline" "ask" "$(decision "$chain_newline")"
# Quoted control chars in filenames are fine — they tokenize as part of the name.
assert_eq "allow_filename_semi" "allow" "$(decision "rm '/tmp/file;weird'")"

# ---- Ask: shell expansion hides the actual targets ----
assert_eq "ask_cmd_sub" "ask" "$(decision "rm \$(cat /tmp/list)")"
assert_eq "ask_backticks" "ask" "$(decision "rm \`cat /tmp/list\`")"
assert_eq "ask_process_sub" "ask" "$(decision "rm <(echo /tmp/x)")"
assert_eq "ask_varexpand" "ask" "$(decision "rm \${FILE}")"
assert_eq "ask_no_targets" "ask" "$(decision "rm")"
assert_eq "ask_only_flags" "ask" "$(decision "rm -rf")"

# ---- Silent: not an rm-family command ----
assert_eq "silent_echo" "silent" "$(decision "echo hi")"
assert_eq "silent_find_delete" "silent" "$(decision "find /tmp -delete")"
assert_eq "silent_git_rm" "silent" "$(decision "git rm /tmp/foo")"
assert_eq "silent_sudo_rm" "silent" "$(decision "sudo rm /tmp/foo")"
assert_eq "silent_xargs_rm" "silent" "$(decision "xargs rm")"
assert_eq "silent_empty_cmd" "silent" "$(decision "")"

echo "rm-under-tmp.py: all tests passed"
