#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/deny-review-in-workflow.py: feeds it
# PreToolUse Workflow payloads and asserts the permission decision.
#   a fan-out review skill named anywhere in the call -> deny
#   anything else                                     -> silent
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/deny-review-in-workflow.py"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

# Per-process, so two concurrent runs cannot delete each other's fixtures.
FIXTURES="/tmp/claude/deny-review-in-workflow.$$"
rm -rf "$FIXTURES"
mkdir -p "$FIXTURES/saved/.claude/workflows" "$FIXTURES/outside"
trap 'rm -rf "$FIXTURES"' EXIT

run_hook() {
  printf '%s' "$1" | python3 "$HOOK"
}

# One tool_input key set to a string, which covers script / scriptPath / name.
payload_for() {
  jq -nc --arg k "$1" --arg v "$2" '{tool_input: {($k): $v}}'
}

decision_of() {
  local out
  out="$(run_hook "$1")"
  if [[ -z "$out" ]]; then
    echo "silent"
  else
    jq -r '.hookSpecificOutput.permissionDecision' <<<"$out"
  fi
}

reason_of() {
  local out
  out="$(run_hook "$1")"
  if [[ -z "$out" ]]; then echo ""; else jq -r '.hookSpecificOutput.permissionDecisionReason' <<<"$out"; fi
}

decision() { decision_of "$(payload_for "$1" "$2")"; }
reason() { reason_of "$(payload_for "$1" "$2")"; }

# ---- Deny: the inline script names each fan-out skill ----
assert_eq "deny_in_depth_review" "deny" \
  "$(decision script "await agent({ skill: 'in-depth-review', prompt: brief })")"
assert_eq "deny_review_and_fix" "deny" \
  "$(decision script "await agent({ skill: 'review-and-fix' })")"
assert_eq "deny_pr_review" "deny" \
  "$(decision script "phases: [{ title: 'pr-review' }]")"
assert_eq "deny_work_on" "deny" \
  "$(decision script "run the work-on skill on WMP-837")"

# open-ticket is denied for a different reason than the four above. It does not
# lose a fan-out inside a workflow. It loses the human who approves the Jira
# creation, so it would either block forever or create unapproved issues.
assert_eq "deny_open_ticket" "deny" \
  "$(decision script "await agent({ skill: 'open-ticket', prompt: req })")"
assert_eq "deny_open_ticket_in_name" "deny" \
  "$(decision name "open-ticket")"
assert_eq "deny_open_ticket_capitalized" "deny" \
  "$(decision script "run Open-Ticket for this requirement")"

# BOUNDARY_BEFORE excludes letters and digits only, so a leading hyphen still
# matches and a prefixed name denies.
assert_eq "deny_nightly_open_ticket" "deny" \
  "$(decision script "await agent({ skill: 'nightly-open-ticket' })")"

# BOUNDARY_AFTER also excludes a hyphen, so a longer name sharing the prefix
# falls through. This is what lets a future open-ticket-validate workflow run.
assert_eq "silent_open_ticket_validate" "silent" \
  "$(decision script "await agent({ skill: 'open-ticket-validate' })")"

# ---- Deny: case and punctuation variants of the real spelling ----
assert_eq "deny_slash_prefix" "deny" "$(decision script "/pr-review 4821")"
assert_eq "deny_mixed_case" "deny" "$(decision script "skill: 'In-Depth-Review'")"

# ---- Deny: scriptPath contents, since a Workflow can be re-run by path ----
printf 'export const meta = { name: %s };\n' "'review-and-fix runner'" \
  >"$FIXTURES/persisted.js"
assert_eq "deny_script_path" "deny" "$(decision scriptPath "$FIXTURES/persisted.js")"

# ---- Deny: a saved workflow whose own name carries a fan-out skill ----
assert_eq "deny_saved_name" "deny" "$(decision name "nightly-pr-review")"

# ---- Deny: a saved workflow resolved under .claude/workflows/ ----
printf 'await agent({ skill: %s });\n' "'in-depth-review'" \
  >"$FIXTURES/saved/.claude/workflows/nightly-audit.js"
assert_eq "deny_saved_resolved" "deny" "$(decision_of "$(jq -nc \
  --arg n "nightly-audit" --arg c "$FIXTURES/saved" \
  '{cwd: $c, tool_input: {name: $n}}')")"

# ---- Silent: args is data, so it is never scanned, even naming a skill outright ----
assert_eq "silent_args_brief" "silent" "$(decision_of "$(jq -nc \
  '{tool_input: {script: "export const meta = {};", args: {brief: "hand this to work-on"}}}')")"

# ---- Deny: the two wrappers that run in-depth-review, so they fan out too ----
assert_eq "deny_finder_indepth" "deny" \
  "$(decision script "await agent({ agentType: 'pr-review-finder-indepth' })")"
assert_eq "deny_finder_indepth_deep" "deny" \
  "$(decision script "await agent({ agentType: 'pr-review-finder-indepth-deep' })")"

# ---- Reason must name the wrapper that matched, not the bare skill ----
assert_contains "reason_names_wrapper" 'run `pr-review-finder-indepth` inside' \
  "$(reason script "await agent({ agentType: 'pr-review-finder-indepth' })")"
assert_contains "reason_names_deep_wrapper" 'run `pr-review-finder-indepth-deep` inside' \
  "$(reason script "await agent({ agentType: 'pr-review-finder-indepth-deep' })")"

# ---- Reason must name the matched skill and steer to the main thread ----
assert_contains "reason_names_skill" "in-depth-review" \
  "$(reason script "skill: 'in-depth-review'")"
assert_contains "reason_main_thread" "main thread" \
  "$(reason script "skill: 'in-depth-review'")"
assert_contains "reason_names_match" "review-and-fix" \
  "$(reason script "skill: 'review-and-fix'")"

# ---- Reason must explain the work-on case, which is a second-hand loss ----
assert_contains "reason_work_on_case" "work-on hands" \
  "$(reason script "run the work-on skill on WMP-837")"

# ---- Silent: gh-style-review is a leaf pass and spawns nothing ----
assert_eq "silent_gh_style" "silent" \
  "$(decision script "await agent({ skill: 'gh-style-review' })")"
assert_eq "silent_gh_style_name" "silent" "$(decision name "nightly-gh-style-review")"

# ---- Silent: a workflow that names none of them ----
assert_eq "silent_unrelated" "silent" \
  "$(decision script "await agent({ prompt: 'summarize the release notes' })")"
assert_eq "silent_telemetry" "silent" \
  "$(decision script "await agent({ skill: 'datadog-probe' })")"

# ---- Silent: an identifier that only happens to contain a skill name ----
assert_eq "silent_network_only" "silent" "$(decision script "const NETWORK_ONLY = true;")"
assert_eq "silent_work_on_queue" "silent" \
  "$(decision script "def work_on_queue(items): return items")"
assert_eq "silent_framework_only" "silent" \
  "$(decision script "if (framework_only) return;")"

# ---- Silent: work-on's own Step 2 workflow, a legitimate fan-out of leaves ----
assert_eq "silent_work_on_validate" "silent" \
  "$(decision script "export const meta = { name: 'work-on-validate' };")"
assert_eq "silent_work_on_scratch_args" "silent" "$(decision_of "$(jq -nc \
  '{tool_input: {script: "export const meta = {};", args: {branch_commits: "work-on-WMP-837-branch-commits.txt"}}}')")"
# Step 2's args carry other people's prose. One commit subject from this repo's
# own history names work-on on a word boundary, which is why args stays out of
# the scan. Scanning it made that single subject enough to kill the call.
assert_eq "silent_step2_real_shape" "silent" "$(decision_of "$(jq -nc \
  '{tool_input: {script: "export const meta = { name: \"work-on-validate\" };", args: {repo: {branch_commits: "feat(claude,skill): work-on: write to Jira and open the PR without asking"}}}}')")"

# ---- Silent: leaf agent types, which a workflow script is meant to launch ----
assert_eq "silent_leaf_role" "silent" \
  "$(decision script "await agent({ agentType: 'in-depth-review-role' })")"
assert_eq "silent_leaf_ghstyle" "silent" \
  "$(decision script "await agent({ agentType: 'pr-review-finder-ghstyle' })")"
assert_eq "silent_leaf_proposer" "silent" \
  "$(decision script "await agent({ agentType: 'pr-review-approach-proposer' })")"
assert_eq "silent_leaf_judge" "silent" \
  "$(decision script "await agent({ agentType: 'pr-review-nuanced-judge' })")"

# ---- Silent: a name that is not a plain filename resolves to no workflow ----
printf 'await agent({ skill: %s });\n' "'in-depth-review'" >"$FIXTURES/outside/notes.md"
printf 'await agent({ skill: %s });\n' "'in-depth-review'" >"$FIXTURES/saved/escaped.md"
assert_eq "silent_etc_hosts_name" "silent" "$(decision name "/etc/hosts")"
assert_eq "silent_absolute_name" "silent" "$(decision name "$FIXTURES/outside/notes.md")"
assert_eq "silent_traversal_name" "silent" "$(decision_of "$(jq -nc \
  --arg n "../../escaped" --arg c "$FIXTURES/saved" \
  '{cwd: $c, tool_input: {name: $n}}')")"

# ---- Silent: nothing to inspect ----
assert_eq "silent_empty_input" "silent" "$(decision_of '{"tool_input": {}}')"
assert_eq "silent_no_input" "silent" "$(decision_of '{}')"
assert_eq "silent_empty_script" "silent" "$(decision script "")"

# ---- Silent: an unreadable scriptPath must not crash and must not deny ----
assert_eq "silent_missing_path" "silent" \
  "$(decision scriptPath "$FIXTURES/absent/persisted.js")"
assert_eq "silent_dir_path" "silent" "$(decision scriptPath "$FIXTURES")"

# ---- The escape hatch, for a workflow that must name these skills ----
# A reason on the same line is required. A bare marker is not a bypass, and the
# next line of code must not be able to serve as the reason.
assert_eq "silent_hatch_with_reason" "silent" \
  "$(decision script "// no-fanout-ok: this workflow edits the review skills
await agent({ prompt: 'rewrite pr-review/SKILL.md' })")"
assert_eq "deny_hatch_bare_marker" "deny" \
  "$(decision script "// no-fanout-ok:
await agent({ skill: 'pr-review' })")"

# ---- Silent: a body past the read cap is not scanned ----
# Locks in the cap that keeps a huge or unending file from stalling the tool
# call. Delete MAX_READ_BYTES and this case denies.
head -c 1000000 /dev/zero | tr '\0' 'x' >"$FIXTURES/huge.js"
printf "\nawait agent({ skill: 'in-depth-review' });\n" >>"$FIXTURES/huge.js"
assert_eq "silent_past_read_cap" "silent" "$(decision scriptPath "$FIXTURES/huge.js")"
assert_eq "deny_within_read_cap" "deny" \
  "$(decision scriptPath "$FIXTURES/persisted.js")"

rm -rf "$FIXTURES"
echo "deny-review-in-workflow.py: all tests passed"
