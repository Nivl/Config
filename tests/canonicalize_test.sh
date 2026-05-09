#!/bin/zsh
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
FILTER="$SCRIPT_DIR/.githooks/canonicalize.jq"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

run() { jq -cf "$FILTER" <<<"$1"; }

assert_eq "sort_strings" '["a","b","c"]' "$(run '["c","a","b"]')"
assert_eq "dedupe" '["a","b"]' "$(run '["b","a","b"]')"
assert_eq "nested_object" '{"x":["a","b"]}' "$(run '{"x":["b","a","a"]}')"
assert_eq "objects_untouched" '[{"x":1},{"x":0}]' "$(run '[{"x":1},{"x":0}]')"
assert_eq "empty_array" '[]' "$(run '[]')"
assert_eq "mixed_primitives" '[null,true,1,"a","b"]' "$(run '["b",1,null,true,"a",1]')"
assert_eq "permissions_allow" \
  '{"permissions":{"allow":["Bash(* --help *)","Bash(git *)"]}}' \
  "$(run '{"permissions":{"allow":["Bash(git *)","Bash(* --help *)","Bash(git *)"]}}')"

once="$(run '{"a":["c","b","a","b"]}')"
twice="$(run "$once")"
assert_eq "idempotent" "$once" "$twice"

echo "canonicalize.jq: all tests passed"
