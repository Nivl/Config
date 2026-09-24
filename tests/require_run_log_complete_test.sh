#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/require-run-log-complete.py.
#   deleting this session's marker with a complete log     -> silent
#   the same with a summary, scoring or report missing     -> deny naming the gap
#   an iteration that found nothing needs no scoring       -> silent
#   a row 0 abort log                                      -> silent
#   RUN_LOG_OK=1, another session's marker, not a delete   -> silent
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/require-run-log-complete.py"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

FIX="/tmp/claude/run-log-complete-fixture-$$"
SESSION="sess-rlc-$$"
MARKER_DIR="$HOME/.melvin/config/logs/review-and-fix"
MARKER="$MARKER_DIR/.active-$SESSION"
mkdir -p "$FIX" "$MARKER_DIR"
LOG="$FIX/run.md"
printf '%s' "$LOG" > "$MARKER"
trap 'rm -f "$MARKER"; rm -rf "$FIX"' EXIT

RM="rm -f ~/.melvin/config/logs/review-and-fix/.active-$SESSION ~/.melvin/config/logs/review-and-fix/.active-$SESSION.seen"

decision() {
  local out
  out="$(jq -nc --arg c "$1" --arg s "${2:-$SESSION}" '{session_id: $s, tool_input: {command: $c}}' | python3 "$HOOK")"
  if [[ -z "$out" ]]; then echo "silent"; else jq -r '.hookSpecificOutput.permissionDecision' <<<"$out"; fi
}
reason() {
  jq -nc --arg c "$1" --arg s "$SESSION" '{session_id: $s, tool_input: {command: $c}}' | python3 "$HOOK" | jq -r '.hookSpecificOutput.permissionDecisionReason'
}

complete_log() {
  cat > "$LOG" <<'EOF'
# review-and-fix run log
## Iteration 1
t0 iter=1 2026-09-24T00:00:00Z
usage kind=review-roles inst=1 role=2 attempt=1 tag=iter1 target=x id=aaaa model=opus-5-5 turns=5 est_usd=0.4
scored iter=1 by=scorer F1=80
stamps: iter=1 t0=a t_fix=b t1=c t2=d
severity iter=1 kept: critical=0 major=1 minor=0 suggestion=0 | dropped: critical=0 major=0 minor=0 suggestion=0 | fixed: critical=0 major=1 minor=0 suggestion=0
### Iteration 1 summary
Iter 1: roles 1-9,11.
## Iteration 2
t0 iter=2 2026-09-24T00:10:00Z
usage kind=review-roles inst=1 role=9 attempt=1 tag=iter2 target=x id=bbbb model=opus-5-5 turns=5 est_usd=0.4
stamps: iter=2 t0=a t_fix=b t1=c t2=d
severity iter=2 kept: critical=0 major=0 minor=0 suggestion=0 | dropped: critical=0 major=0 minor=0 suggestion=0 | fixed: critical=0 major=0 minor=0 suggestion=0
### Iteration 2 summary
Iter 2: role 9, clean.
## Review and Fix Report
### Coverage
complete
### Spend
total $1
### Severity
table
### Outcome
Clean batch.
EOF
}

complete_log
assert_eq "silent_not_a_delete" "silent" "$(decision 'git status')"
assert_eq "silent_other_session" "silent" "$(decision "$RM" "someone-else")"
assert_eq "silent_complete_log" "silent" "$(decision "$RM")"

sd '^### Iteration 2 summary$' '' "$LOG"
assert_eq "deny_missing_summary" "deny" "$(decision "$RM")"
assert_contains "reason_names_summary" "iteration 2: the \`### Iteration 2 summary\` block" "$(reason "$RM")"
assert_eq "silent_override" "silent" "$(decision "RUN_LOG_OK=1 $RM")"

complete_log
sd '^scored iter=1 .*$' '' "$LOG"
assert_contains "reason_names_scoring" "iteration 1: a scoring record" "$(reason "$RM")"
assert_eq "zero_finding_iter_needs_no_scoring" "0" "$(reason "$RM" | grep -c 'iteration 2' || true)"

complete_log
sd '^### Spend$' '' "$LOG"
assert_contains "reason_names_section" "\`### Spend\`" "$(reason "$RM")"

complete_log
sd '^usage kind=review-roles .*tag=iter1 .*$' '' "$LOG"
assert_contains "reason_names_usage" "iteration 1: \`usage kind=review-roles ... tag=iter1\` lines" "$(reason "$RM")"

printf '# review-and-fix run log\nREVIEW_UNAVAILABLE_NO_FANOUT: no Agent tool\n' > "$LOG"
assert_eq "silent_row0_abort" "silent" "$(decision "$RM")"

rm -f "$MARKER"
assert_eq "silent_no_marker" "silent" "$(decision "$RM")"

echo "require-run-log-complete.py: all tests passed"
