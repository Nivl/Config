#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/bash-allow-trusted.py against
# fixture JSON pointed to by BASH_ALLOW_DIR. Verifies prefix-matched
# lone-allow, compound-excluded deny, write-redirect bail, and the
# committed+local union (with dedupe).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/bash-allow-trusted.py"
. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

FIX="$(mktemp -d)"
trap 'rm -rf "$FIX"' EXIT
export BASH_ALLOW_DIR="$FIX"
# write-redirect roots come from these env vars (mirrors the hook)
export WORKTREES_ROOT="$FIX/wt" REPOS_ROOT="$FIX/repos"
mkdir -p "$FIX/wt" "$FIX/repos"

cat >"$FIX/bash-allow-trusted.json" <<'JSON'
{
  "excluded": [["git"], ["gh"], ["rushx", "test"]],
  "trusted": [["git", "symbolic-ref", "--short"], ["rushx", "test"]]
}
JSON

decision() {
  local payload out
  payload="$(jq -nc --arg c "$1" --arg w "$2" '{tool_input:{command:$c}, cwd:$w}')"
  out="$(printf '%s' "$payload" | python3 "$HOOK")"
  if [[ -z "$out" ]]; then echo "silent"; else jq -r '.hookSpecificOutput.permissionDecision' <<<"$out"; fi
}

# lone trusted -> allow
assert_eq "allow_symref"  "allow"  "$(decision "git symbolic-ref --short HEAD" "$REPOS_ROOT")"
assert_eq "allow_rushx"   "allow"  "$(decision "rushx test models/x 2>&1" "$REPOS_ROOT")"
# trusted but compound -> deny
assert_eq "deny_pipe"     "deny"   "$(decision "git symbolic-ref --short HEAD | cat" "$REPOS_ROOT")"
# excluded-but-not-trusted, compound -> deny
assert_eq "deny_gh_pipe"  "deny"   "$(decision "gh pr view 1 | head" "$REPOS_ROOT")"
# not trusted, lone -> silent
assert_eq "silent_ghview" "silent" "$(decision "gh pr view 1" "$REPOS_ROOT")"
# trusted but write outside roots -> silent (falls through to prompt)
assert_eq "silent_badwr"  "silent" "$(decision "rushx test x > /etc/hosts" "$REPOS_ROOT")"
# injection: -c shifts tokens, no trusted match -> silent
assert_eq "silent_dashc"  "silent" "$(decision "git -c core.pager=x symbolic-ref --short HEAD" "$REPOS_ROOT")"

# ---- redirect-target safety: quoted / $VAR targets must not bypass the write-root check ----
assert_eq "silent_quoted_abs"   "silent" "$(decision 'git symbolic-ref --short HEAD > "/etc/hosts"' "$REPOS_ROOT")"
assert_eq "silent_squoted_abs"  "silent" "$(decision "git symbolic-ref --short HEAD > '/etc/hosts'" "$REPOS_ROOT")"
assert_eq "silent_var_target"   "silent" "$(decision 'git symbolic-ref --short HEAD > $HOME/zzz' "$REPOS_ROOT")"
# a quoted target that still resolves under a write root is allowed
assert_eq "allow_quoted_inroot" "allow"  "$(decision 'git symbolic-ref --short HEAD > "ok.txt"' "$REPOS_ROOT")"

# ---- newline / command-substitution must not be treated as a lone trusted command ----
nl_cmd=$'git symbolic-ref --short HEAD\nrm -rf /tmp/zzz'
assert_eq "deny_newline"      "deny"   "$(decision "$nl_cmd" "$REPOS_ROOT")"
assert_eq "deny_subst_chain"  "deny"   "$(decision 'git symbolic-ref --short HEAD && rm $(whoami)' "$REPOS_ROOT")"
assert_eq "silent_subst_lone" "silent" "$(decision 'git symbolic-ref --short HEAD $(whoami)' "$REPOS_ROOT")"

# ---- >&word redirects both stdout+stderr to a FILE: must honor the write-root check; fd-dups stay allowed ----
assert_eq "silent_amp_file"    "silent" "$(decision 'git symbolic-ref --short HEAD >&/etc/hosts' "$REPOS_ROOT")"
assert_eq "allow_fd_dup_1"     "allow"  "$(decision "git symbolic-ref --short HEAD 2>&1" "$REPOS_ROOT")"
assert_eq "allow_fd_dup_2"     "allow"  "$(decision "git symbolic-ref --short HEAD >&2" "$REPOS_ROOT")"
# a digit-leading >&word is still a FILE redirect (not an fd-dup): escapes must be silent, under-root allowed
assert_eq "silent_amp_escape"  "silent" "$(decision 'git symbolic-ref --short HEAD >&0/../../../../../../etc/hosts' "$REPOS_ROOT")"
assert_eq "allow_amp_underroot" "allow" "$(decision 'git symbolic-ref --short HEAD >&out.txt' "$REPOS_ROOT")"

# ---- ANY leading env assignment falls through to a prompt (allowlist posture): git/gh honor an
#      open-ended set of code-injecting env vars (GIT_CONFIG_*, GIT_PAGER, …) so none are auto-allowed ----
assert_eq "silent_env_extdiff"   "silent" "$(decision 'GIT_EXTERNAL_DIFF=/tmp/x git symbolic-ref --short HEAD' "$REPOS_ROOT")"
assert_eq "silent_env_pager"     "silent" "$(decision 'GIT_PAGER=evil git symbolic-ref --short HEAD' "$REPOS_ROOT")"
assert_eq "silent_env_gitconfig" "silent" "$(decision 'GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.pager GIT_CONFIG_VALUE_0=evil git symbolic-ref --short HEAD' "$REPOS_ROOT")"
assert_eq "silent_env_param"     "silent" "$(decision 'GIT_CONFIG_PARAMETERS=core.pager=evil git symbolic-ref --short HEAD' "$REPOS_ROOT")"
assert_eq "silent_env_benign"    "silent" "$(decision 'FOO=bar git symbolic-ref --short HEAD' "$REPOS_ROOT")"

# ---- a trailing separator/newline on a lone trusted command stays allowed (not denied) ----
assert_eq "allow_trailing_amp" "allow"  "$(decision "git symbolic-ref --short HEAD &" "$REPOS_ROOT")"
nl_trail=$'git symbolic-ref --short HEAD\n'
assert_eq "allow_trailing_nl"  "allow"  "$(decision "$nl_trail" "$REPOS_ROOT")"

# ---- process substitution is never auto-allowed (bails like command substitution) ----
assert_eq "silent_proc_subst"  "silent" "$(decision 'git symbolic-ref --short HEAD <(echo x)' "$REPOS_ROOT")"

# ---- committed + local union (local adds rushx via a different shape + dedup) ----
cat >"$FIX/bash-allow-trusted.local.json" <<'JSON'
{ "excluded": [["docker"]], "trusted": [["docker", "compose"], ["rushx", "test"]] }
JSON
assert_eq "local_allow"   "allow"  "$(decision "docker compose up -d" "$REPOS_ROOT")"
assert_eq "local_union_rushx" "allow" "$(decision "rushx test y" "$REPOS_ROOT")"

# ---- malformed local is ignored; committed still applies ----
printf 'not json{' >"$FIX/bash-allow-trusted.local.json"
assert_eq "malformed_local_ok" "allow" "$(decision "git symbolic-ref --short HEAD" "$REPOS_ROOT")"

# ---- safe_assignments: a whitelisted env-var name may prefix a trusted command ----
cat >"$FIX/bash-allow-trusted.json" <<'JSON'
{
  "excluded": [["rushx", "test"]],
  "trusted": [["rushx", "test"]],
  "safe_assignments": ["ENV_TIER"]
}
JSON
rm -f "$FIX/bash-allow-trusted.local.json"
assert_eq "safe_env_allow"     "allow"  "$(decision "ENV_TIER=local rushx test x" "$REPOS_ROOT")"
assert_eq "safe_env_redirect"  "allow"  "$(decision "ENV_TIER=local rushx test x > ok.txt" "$REPOS_ROOT")"
# an env var NOT on the safe list still falls through, even on a trusted command
assert_eq "unsafe_env_silent"  "silent" "$(decision "GIT_PAGER=evil rushx test x" "$REPOS_ROOT")"
# ALL leading assignments must be safe; one unknown name bails to a prompt
assert_eq "mixed_env_silent"   "silent" "$(decision "ENV_TIER=local FOO=bar rushx test x" "$REPOS_ROOT")"
# a safe prefix does NOT rescue a compound excluded command -> still deny
assert_eq "safe_env_pipe_deny" "deny"   "$(decision "ENV_TIER=local rushx test x | grep ok" "$REPOS_ROOT")"

# ---- both files absent -> fail closed (silent) ----
rm -f "$FIX/bash-allow-trusted.json" "$FIX/bash-allow-trusted.local.json"
assert_eq "failclosed" "silent" "$(decision "rushx test x" "$REPOS_ROOT")"

echo "bash-allow-trusted.py: all tests passed"
