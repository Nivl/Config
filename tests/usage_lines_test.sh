#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/usage-lines.py with fixture transcripts.
#   not a Workflow, or a Workflow with no args.tag   -> silent
#   tagged Workflow, no marker                      -> additionalContext carrying the lines
#   tagged Workflow, marker -> run log              -> lines appended, second firing appends 0
#   lines for another tag                           -> not included
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/usage-lines.py"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

FIX="/tmp/claude/usage-lines-fixture-$$"
SESSION="sess-$$"
rm -rf "$FIX"
mkdir -p "$FIX/$SESSION/subagents/workflows/wf1"
: > "$FIX/$SESSION.jsonl"

# A transcript: first record carries the stamp, then one assistant turn with usage.
mk() {
  local file="$1" stamp="$2"
  jq -nc --arg s "$stamp" '{type:"user", message:{role:"user", content:$s}}' > "$file"
  jq -nc '{type:"assistant", message:{model:"claude-opus-5", usage:{input_tokens:10, cache_read_input_tokens:1000000, cache_creation_input_tokens:100000, output_tokens:2000}}}' >> "$file"
}
mk "$FIX/$SESSION/subagents/workflows/wf1/agent-aaaa1111bbbb.jsonl" "<!-- review-roles inst=1 role=2 attempt=1 tag=iter1 target=15 -->"
mk "$FIX/$SESSION/subagents/workflows/wf1/agent-cccc2222dddd.jsonl" "<!-- review-roles inst=2 role=9 attempt=1 tag=iter1 target=15 -->"
mk "$FIX/$SESSION/subagents/agent-eeee3333ffff.jsonl" "<!-- gh-style tag=iter2 target=15 -->"

run() {
  local tool="$1" tag="$2"
  jq -nc --arg t "$tool" --arg g "$tag" --arg tp "$FIX/$SESSION.jsonl" --arg sid "$SESSION" \
    '{session_id:$sid, transcript_path:$tp, tool_name:$t, tool_input:{args:(if $g=="" then {} else {tag:$g} end)}}' \
    | python3 "$HOOK"
}
ctx() { jq -r '.hookSpecificOutput.additionalContext'; }

# ---- Silent cases ----
assert_eq "silent_not_workflow" "" "$(run Bash iter1)"
assert_eq "silent_no_tag" "" "$(run Workflow '')"
assert_eq "silent_unknown_tag" "" "$(run Workflow iter9)"

# ---- No marker: lines come back as context, filtered by tag ----
OUT="$(run Workflow iter1 | ctx)"
assert_contains "ctx_has_inst1" "usage kind=review-roles inst=1 role=2 attempt=1 tag=iter1 target=15 id=aaaa1111" "$OUT"
assert_contains "ctx_has_inst2" "inst=2 role=9" "$OUT"
assert_contains "ctx_priced" "est_usd=" "$OUT"
assert_eq "ctx_excludes_other_tag" "0" "$(grep -c 'tag=iter2' <<<"$OUT" || true)"

# ---- Marker: lines appended to the run log, idempotently ----
MARKER_DIR="$HOME/.melvin/config/logs/review-and-fix"
mkdir -p "$MARKER_DIR"
LOG="$FIX/run.md"
printf '# run log\n' > "$LOG"
printf '%s' "$LOG" > "$MARKER_DIR/.active-$SESSION"
trap 'rm -f "$MARKER_DIR/.active-$SESSION"; rm -rf "$FIX"' EXIT

OUT="$(run Workflow iter1 | ctx)"
assert_contains "marker_appended_two" "appended 2 usage line(s)" "$OUT"
assert_eq "log_has_two" "2" "$(grep -c '^usage kind=' "$LOG")"
OUT="$(run Workflow iter1 | ctx)"
assert_contains "marker_second_firing_zero" "appended 0 usage line(s)" "$OUT"
assert_eq "log_still_two" "2" "$(grep -c '^usage kind=' "$LOG")"
OUT="$(run Workflow iter2 | ctx)"
assert_contains "marker_other_tag" "appended 1 usage line(s) for tag=iter2" "$OUT"
assert_eq "log_has_three" "3" "$(grep -c '^usage kind=' "$LOG")"

echo "usage-lines.py: all tests passed"
