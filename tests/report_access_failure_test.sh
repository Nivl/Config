#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/report-access-failure.py: feeds it
# PermissionDenied / PostToolUseFailure payloads for WebFetch and asserts
# the injected additionalContext names the unreachable host, tells the
# agent to stop and ask, and echoes the firing event.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/report-access-failure.py"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

# $1 event, $2 tool, $3 url -> emitted additionalContext
ctx() {
  jq -nc --arg e "$1" --arg t "$2" --arg u "$3" \
    '{hook_event_name:$e, tool_name:$t, tool_input:{url:$u}}' |
    python3 "$HOOK" | jq -r '.hookSpecificOutput.additionalContext // ""'
}

# $1 event, $2 tool, $3 url -> echoed hookEventName
evt() {
  jq -nc --arg e "$1" --arg t "$2" --arg u "$3" \
    '{hook_event_name:$e, tool_name:$t, tool_input:{url:$u}}' |
    python3 "$HOOK" | jq -r '.hookSpecificOutput.hookEventName // ""'
}

URL="https://docs.datagrail.io/guide"

# ---- Names the host, steers to stop+ask, blocks workarounds, gives the fix ----
out="$(ctx PermissionDenied WebFetch "$URL")"
assert_contains "denied_names_host" "docs.datagrail.io" "$out"
assert_contains "denied_ask" "ask how they want to proceed" "$out"
assert_contains "denied_no_workaround" "Do NOT work around" "$out"
assert_contains "denied_fix_cmd" "perms allow add --fetch docs.datagrail.io" "$out"

# ---- Echoes the firing event so the steer is attributed correctly ----
assert_eq "echo_denied" "PermissionDenied" "$(evt PermissionDenied WebFetch "$URL")"
assert_eq "echo_failure" "PostToolUseFailure" "$(evt PostToolUseFailure WebFetch "$URL")"

# ---- PostToolUseFailure path also names the host ----
assert_contains "failure_names_host" "docs.datagrail.io" "$(ctx PostToolUseFailure WebFetch "$URL")"

# ---- userinfo + port are stripped to the bare host ----
assert_contains "host_userinfo_port" "docs.datagrail.io" \
  "$(ctx PostToolUseFailure WebFetch 'https://user:pass@docs.datagrail.io:8443/x')"

# ---- No url: still steers to stop+ask, but offers no host-specific fix ----
no_url="$(ctx PermissionDenied WebFetch '')"
assert_contains "nourl_ask" "ask how they want to proceed" "$no_url"
if [[ "$no_url" == *"perms allow add"* ]]; then
  printf '[nourl_no_fix] did not expect an allowlist hint without a host:\n%s\n' "$no_url" >&2
  exit 1
fi

# ---- Missing event name falls back so output stays valid ----
fallback="$(jq -nc '{tool_name:"WebFetch", tool_input:{url:"https://x.example.com"}}' | python3 "$HOOK" | jq -r '.hookSpecificOutput.hookEventName // ""')"
assert_eq "missing_event_fallback" "PostToolUseFailure" "$fallback"

echo "report-access-failure.py: all tests passed"
