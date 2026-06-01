#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/deny-json-tool.py: feeds it
# PreToolUse Bash payloads and asserts the permission decision.
#   python interpreter + `-m json.tool` (anywhere at command-position) -> deny
#   any other invocation                                                -> silent
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/deny-json-tool.py"

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

# ---- Deny: every shape of `python ... -m json.tool` ----
assert_eq "deny_python3" "deny" "$(decision "python3 -m json.tool /tmp/x.json")"
assert_eq "deny_python" "deny" "$(decision "python -m json.tool file.json")"
assert_eq "deny_python311" "deny" "$(decision "python3.11 -m json.tool x.json")"
assert_eq "deny_abs_path" "deny" "$(decision "/opt/homebrew/bin/python3 -m json.tool x")"
assert_eq "deny_packed_m" "deny" "$(decision "python3 -mjson.tool /tmp/x.json")"
assert_eq "deny_stdin_pipe" "deny" "$(decision "cat x.json | python3 -m json.tool")"
assert_eq "deny_chained_semi" "deny" "$(decision "echo start; python3 -m json.tool x.json")"
assert_eq "deny_chained_and" "deny" "$(decision "echo start && python3 -m json.tool x.json")"
assert_eq "deny_subshell" "deny" "$(decision "(python3 -m json.tool x.json)")"
assert_eq "deny_with_python_flags" "deny" "$(decision "python3 -B -m json.tool x.json")"
# Arg-taking flags before -m: -W and -X both consume the next positional. The
# walker has to skip that next token so the -m json.tool that follows trips.
assert_eq "deny_with_W_arg" "deny" "$(decision "python3 -W default -m json.tool x.json")"
assert_eq "deny_with_X_arg" "deny" "$(decision "python3 -X faulthandler -m json.tool x.json")"
# -W with no separate arg (next token is also a flag) still resolves correctly.
assert_eq "deny_with_packed_Wd" "deny" "$(decision "python3 -Wd -m json.tool x.json")"

# Reason must steer the agent at jq.
assert_contains "reason_jq" "jq" "$(reason "python3 -m json.tool x.json")"

# ---- Silent: not the json.tool form ----
assert_eq "silent_python_c" "silent" "$(decision "python3 -c 'print(1)'")"
assert_eq "silent_python_m_pip" "silent" "$(decision "python3 -m pip install foo")"
assert_eq "silent_python_m_other" "silent" "$(decision "python3 -m http.server")"
assert_eq "silent_python_script" "silent" "$(decision "python3 app.py")"
assert_eq "silent_bare_python" "silent" "$(decision "python3")"
assert_eq "silent_jq" "silent" "$(decision "jq . file.json")"
assert_eq "silent_echo" "silent" "$(decision "echo hi")"
assert_eq "silent_empty" "silent" "$(decision "")"

# ---- Silent: wrapper-prefixed and non-cpython forms (documented carve-outs) ----
# The hook only checks leading-token python; wrappers like env/sudo and
# non-cpython interpreters fall through. Locked in so a future "peel wrappers
# here too" change has to update these tests deliberately.
assert_eq "silent_env_wrapped" "silent" "$(decision "env python3 -m json.tool x.json")"
assert_eq "silent_sudo_wrapped" "silent" "$(decision "sudo python3 -m json.tool x.json")"
assert_eq "silent_varassign_wrapped" "silent" "$(decision "PYTHONIOENCODING=utf-8 python3 -m json.tool x.json")"
assert_eq "silent_pypy" "silent" "$(decision "pypy3 -m json.tool x.json")"

# ---- Silent: the literal string in a non-python context ----
# Quoted form: shlex collapses to one token; basename doesn't match PYTHON_RE.
assert_eq "silent_echo_quoted" "silent" "$(decision "echo 'python3 -m json.tool x'")"
# Echo as leading command, python as plain argument (not at command-position).
assert_eq "silent_echo_unquoted" "silent" "$(decision "echo python3 -m json.tool")"

echo "deny-json-tool.py: all tests passed"
