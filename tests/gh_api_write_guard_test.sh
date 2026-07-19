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

# GraphQL always POSTs, but the document decides read vs write: a body
# made only of query/fragment definitions is read-only and auto-allows.
assert_eq "graphql_query"      "allow"  "$(decision "gh api graphql -f query='query { viewer { login } }'")"
assert_eq "graphql_anon"       "allow"  "$(decision "gh api graphql -f query='{ viewer { login } }'")"
assert_eq "graphql_multiline"  "allow"  "$(decision "gh api graphql -f query='
  query {
    repository(owner: \"o\", name: \"r\") {
      pullRequest(number: 1) { title }
    }
  }' > /tmp/out.json")"
assert_eq "graphql_fragment"   "allow"  "$(decision "gh api graphql -f query='query { viewer { ...F } } fragment F on User { login }'")"
assert_eq "graphql_vars"       "allow"  "$(decision "gh api graphql -f query='query(\$n: Int!) { r(n: \$n) { x } }' -F n=5")"

# Mutations, subscriptions, and anything unclassifiable stay a write.
assert_eq "graphql_mutation"   "ask"    "$(decision "gh api graphql -f query='mutation { addStar(input: {starrableId: \"x\"}) { clientMutationId } }'")"
assert_eq "graphql_trailing"   "ask"    "$(decision "gh api graphql -f query='query A { x } mutation B { y }'")"
assert_eq "graphql_sub"        "ask"    "$(decision "gh api graphql -f query='subscription { s { x } }'")"
assert_eq "graphql_junk"       "ask"    "$(decision 'gh api graphql -f query=x')"
assert_eq "graphql_blockstr"   "ask"    "$(decision "gh api graphql -f query='query { f(s: \"\"\"x\"\"\") { y } }'")"
assert_eq "graphql_input"      "ask"    "$(decision 'gh api graphql --input body.json')"
assert_eq "graphql_packed"     "ask"    "$(decision "gh api graphql -fquery='query { x }'")"
assert_eq "graphql_at_file"    "ask"    "$(decision 'gh api graphql -F query=@doc.graphql')"
assert_eq "graphql_no_query"   "ask"    "$(decision 'gh api graphql -f foo=bar')"

# Things the hook can't classify stay silent and fall through to the
# normal permission flow.
assert_eq "not_gh_api"         "silent" "$(decision 'echo gh api stuff')"
assert_eq "chained"            "silent" "$(decision 'gh api repos/o/r && rm -rf /tmp/x')"

echo "gh-api-write-guard.py: all tests passed"
