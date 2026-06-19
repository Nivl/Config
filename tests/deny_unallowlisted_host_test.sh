#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/deny-unallowlisted-host.py: feeds
# it PreToolUse curl/wget payloads against a crafted allowlist and
# asserts the permission decision.
#   http(s):// host not on the allowlist -> deny
#   allowlisted host / subdomain / loopback / no url -> silent
#   deniedDomains entry -> deny even when otherwise allowed
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/deny-unallowlisted-host.py"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

# A self-contained settings dir the hook reads via MELVIN_CLAUDE_SETTINGS_DIR.
# github.com (+ subdomains) and go.dev come from settings.json allow rules;
# sandboxed.example.com from sandbox.network.allowedDomains; local-only via
# settings.local.json (proves the per-machine file is merged); a subdomain
# of an allowed domain is denied via deniedDomains.
DIR="$(mktemp -d /tmp/dnah-test.XXXXXX)"
trap 'rm -rf "$DIR"' EXIT
cat > "$DIR/settings.json" <<'EOF'
{
  "permissions": { "allow": ["WebFetch(domain:github.com)", "WebFetch(domain:go.dev)"] },
  "sandbox": { "network": {
    "allowedDomains": ["sandboxed.example.com"],
    "deniedDomains": ["blocked.github.com"]
  } }
}
EOF
cat > "$DIR/settings.local.json" <<'EOF'
{ "permissions": { "allow": ["WebFetch(domain:local-only.example.com)"] } }
EOF
export MELVIN_CLAUDE_SETTINGS_DIR="$DIR"

decision() {
  local out
  out="$(jq -nc --arg c "$1" '{tool_input: {command: $c}}' | python3 "$HOOK")"
  if [[ -z "$out" ]]; then echo "silent"; else
    jq -r '.hookSpecificOutput.permissionDecision' <<<"$out"
  fi
}

reason() {
  jq -nc --arg c "$1" '{tool_input: {command: $c}}' | python3 "$HOOK" |
    jq -r '.hookSpecificOutput.permissionDecisionReason // ""'
}

# ---- Deny: host is not on the allowlist ----
assert_eq "deny_curl_unknown" "deny" "$(decision 'curl https://docs.datagrail.io/x')"
assert_eq "deny_wget_unknown" "deny" "$(decision 'wget https://evil.example.org/f')"
assert_eq "deny_http_scheme" "deny" "$(decision 'curl http://docs.datagrail.io')"
# One bad host among allowed ones still denies (errs toward asking).
assert_eq "deny_mixed_hosts" "deny" "$(decision "curl -H 'Referer: https://github.com' https://docs.datagrail.io/x")"
# deniedDomains wins over a matching allow rule.
assert_eq "deny_denied_subdomain" "deny" "$(decision 'curl https://blocked.github.com/x')"

# ---- Silent: host is covered ----
assert_eq "silent_allowed" "silent" "$(decision 'curl https://github.com/Nivl/config')"
assert_eq "silent_subdomain" "silent" "$(decision 'curl https://api.github.com/repos')"
assert_eq "silent_userinfo" "silent" "$(decision 'curl https://user:pass@github.com/x')"
assert_eq "silent_sandbox_domain" "silent" "$(decision 'curl https://sandboxed.example.com/p')"
assert_eq "silent_local_settings" "silent" "$(decision 'curl https://local-only.example.com/p')"

# ---- Silent: nothing to gate ----
assert_eq "silent_loopback" "silent" "$(decision 'curl http://localhost:8080/health')"
assert_eq "silent_loopback_ip" "silent" "$(decision 'curl http://127.0.0.1:3000/')"
assert_eq "silent_no_url" "silent" "$(decision 'curl --version')"
assert_eq "silent_empty" "silent" "$(decision '')"

# ---- Reason names the host and the exact allowlist command ----
assert_contains "reason_names_host" "docs.datagrail.io" "$(reason 'curl https://docs.datagrail.io/x')"
assert_contains "reason_fix_command" "perms allow add --fetch docs.datagrail.io" "$(reason 'curl https://docs.datagrail.io/x')"

echo "deny-unallowlisted-host.py: all tests passed"
