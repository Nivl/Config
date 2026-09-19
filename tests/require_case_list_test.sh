#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/require-case-list.py against a fixture
# repo, a fixture run log, and a marker under a fixture session id.
#   no marker for the session                                  -> silent
#   marker, code changed, no cases line for the iteration      -> deny
#   marker, code changed, cases ... reaches= line present      -> silent
#   marker, cases line for an EARLIER iteration only           -> deny
#   marker, prose-only / test-only / comment-only diff         -> silent
#   CASE_LIST_OK=1 prefix                                      -> silent
#   not a git commit                                           -> silent
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/require-case-list.py"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

FIX="/tmp/claude/require-case-list-fixture-$$"
SESSION="sess-case-$$"
MARKER_DIR="$HOME/.melvin/config/logs/review-and-fix"
MARKER="$MARKER_DIR/.active-$SESSION"
LOG="$FIX/run.md"
rm -rf "$FIX"
mkdir -p "$FIX/src" "$FIX/docs" "$FIX/src/__tests__" "$MARKER_DIR"
trap 'rm -f "$MARKER"; rm -rf "$FIX"' EXIT
cd "$FIX"
git init -q .
git config user.email t@example.com
git config user.name t
printf 'export const a = 1\n' > src/a.ts
printf '# notes\n' > docs/notes.md
printf 'test("x", () => {})\n' > src/__tests__/a.test.ts
git add -A
git commit -qm init

decision() {
  local payload out
  payload="$(jq -nc --arg c "$1" --arg d "$FIX" --arg s "$SESSION" '{cwd: $d, session_id: $s, tool_input: {command: $c}}')"
  out="$(printf '%s' "$payload" | python3 "$HOOK")"
  if [[ -z "$out" ]]; then echo "silent"; else jq -r '.hookSpecificOutput.permissionDecision' <<<"$out"; fi
}
stage() { printf '%s\n' "$2" >> "$1"; git add "$1"; }
unstage_all() { git reset -q --hard HEAD; }

# ---- No marker: untouched ----
stage src/a.ts 'export const b = 2'
assert_eq "silent_no_marker" "silent" "$(decision 'git commit -m x')"

# ---- Marker, iteration 2 open, no cases line ----
printf '# run log\nt0 iter=1 2026-09-18T00:00:00Z\ncases iter=1 findings=A reaches="x; y"\nt0 iter=2 2026-09-18T01:00:00Z\n' > "$LOG"
printf '%s' "$LOG" > "$MARKER"
assert_eq "deny_code_no_cases" "deny" "$(decision 'git commit -m x')"
assert_eq "silent_override" "silent" "$(decision 'CASE_LIST_OK=1 git commit -m x')"
assert_eq "silent_not_commit" "silent" "$(decision 'git status')"

# ---- A cases line for iteration 2 with reaches= lifts the deny ----
printf 'cases iter=2 findings=B changes="c"\ncases iter=2 findings=B keeps="k"\ncases iter=2 findings=B reaches="the case; the null path"\n' >> "$LOG"
assert_eq "silent_with_cases" "silent" "$(decision 'git commit -m x')"
unstage_all

# ---- Iteration 3 opens: the iteration-2 list no longer counts ----
printf 't0 iter=3 2026-09-18T02:00:00Z\n' >> "$LOG"
stage src/a.ts 'export const c = 3'
assert_eq "deny_stale_iteration_list" "deny" "$(decision 'git commit -m x')"
unstage_all

# ---- Prose, test and comment-only diffs pass without a list ----
stage docs/notes.md 'a doc line'
assert_eq "silent_prose_only" "silent" "$(decision 'git commit -m x')"
unstage_all
stage src/__tests__/a.test.ts 'test("y", () => {})'
assert_eq "silent_test_only" "silent" "$(decision 'git commit -m x')"
unstage_all
stage src/a.ts '// a comment about a'
assert_eq "silent_comment_only" "silent" "$(decision 'git commit -m x')"
unstage_all

echo "require-case-list.py: all tests passed"
