#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/gh-api-write-guard.py: feeds it
# PreToolUse payloads and asserts the permission decision it emits.
#   read-only gh api  -> allow
#   writing gh api     -> ask
#   non-gh-api / chained / unparseable -> silent (falls through)
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/gh-api-write-guard.py"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

# decision <command> prints the hook's permissionDecision, or "silent"
# when the hook emits nothing (no stdout).
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

# Read-only GETs auto-allow — including the absolute-path form and one
# with a shell redirect, which must not be mistaken for a chained command.
assert_eq "read_comments"      "allow"  "$(decision 'gh api repos/calm/api/issues/14822/comments')"
assert_eq "read_with_redirect" "allow"  "$(decision 'gh api repos/calm/api/pulls/14822/comments 2>&1')"
assert_eq "read_abs_path"      "allow"  "$(decision '/opt/homebrew/bin/gh api repos/o/r/pulls/1/reviews')"
assert_eq "explicit_get"       "allow"  "$(decision 'gh api repos/o/r -X GET')"

# Writes force a confirmation prompt instead of falling through.
assert_eq "post_method"        "ask"    "$(decision 'gh api repos/o/r/issues -X POST -f title=x')"
assert_eq "field_implies_post" "ask"    "$(decision 'gh api repos/o/r/issues -f title=x')"
assert_eq "delete_method"      "ask"    "$(decision 'gh api repos/o/r/x --method DELETE')"
assert_eq "graphql"            "ask"    "$(decision 'gh api graphql -f query=x')"

# Things the hook can't classify stay silent and fall through to the
# normal permission flow.
assert_eq "not_gh_api"         "silent" "$(decision 'echo gh api stuff')"
assert_eq "chained"            "silent" "$(decision 'gh api repos/o/r && rm -rf /tmp/x')"

echo "gh-api-write-guard.py: all tests passed"
