#!/usr/bin/env bash
# Exercises the tallyVotes() reducer inside the workflow script embedded in
# shared_config/.claude/skills/work-on/VALIDATION.md.
#
# That reducer decides whether a verdict-critical finding survives its skeptic panel, and a wrong
# answer silently changes a Jira ticket's verdict. The panel members are agents, so any of them can
# return null, and the interesting cases are all about a panel that came back short.
#
# The script itself cannot be imported: it runs inside the Workflow tool with injected globals and
# no module system. So this extracts the fenced javascript block, strips everything from the first
# top-level statement that needs those globals, and evaluates just the pure function.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
# SKILL holds the path to VALIDATION.md, and SKILL_MD holds the path to SKILL.md. The name SKILL
# predates this file having two paths, and it is kept as is rather than renamed out of scope.
SKILL="$SCRIPT_DIR/shared_config/.claude/skills/work-on/VALIDATION.md"
SKILL_MD="$SCRIPT_DIR/shared_config/.claude/skills/work-on/SKILL.md"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

# mktemp, matching every sibling test here, because the EXIT trap below is an rm -rf. A fixed path
# would be overridable from the environment, which points that rm at whatever the caller chose, and
# two concurrent runs of the suite would delete each other's extracted files mid-test.
WORK="$(mktemp -d /tmp/work_on_validation_test.XXXXXX)"
trap 'rm -rf "$WORK"' EXIT

# Pull the fenced javascript block out of the markdown.
#
# Exactly one block is required first. `sed -n '/fence/,/fence/p' | sed '1d;$d'` restarts its range at
# every opening fence and then trims only the very first and very last line, so with two blocks it
# concatenates them with a stray pair of ``` lines in between. `node --check` then fails on a fence, and
# the line number it reports is the one useful thing that check produces.
FENCES="$(grep -c '^```javascript$' "$SKILL" || true)"
if [[ "$FENCES" != 1 ]]; then
  echo "expected exactly 1 fenced javascript block in $SKILL, found $FENCES" >&2
  echo "the extraction below concatenates multiple blocks, so it must not run on more than one" >&2
  exit 1
fi

sed -n '/^```javascript$/,/^```$/p' "$SKILL" | sed '1d;$d' > "$WORK/script.js"

if [[ ! -s "$WORK/script.js" ]]; then
  echo "could not extract a javascript block from $SKILL" >&2
  exit 1
fi

# The whole script must parse. This is the cheap regression net for a stray unescaped backtick inside one
# of the prompt template literals, which would silently truncate a prompt.
#
# Only two rewrites, both needed because `node --check` treats the file as a module: `export const meta`
# would be legal but a bare top-level `return` is not, and the Workflow runtime wraps the body in a
# function where it is. No stub globals are prepended: `--check` parses and never executes, so nothing
# would consult them, and every line they added shifted the line number `--check` reports on a real syntax
# error, which is the one thing this check exists to hand you.
#
# Redirect rather than `sed -i`, whose in-place flag needs an argument on BSD and rejects one on GNU.
sed -e 's/^export const meta = {/const meta = {/' -e 's/^return {$/export default {/' "$WORK/script.js" > "$WORK/parse.mjs"
node --check "$WORK/parse.mjs"

# Isolate the pure logic: the priority table and the two functions. All three are defined before
# anything that touches the injected globals, which is what makes them extractable at all.
awk '/^const REFUTE_PRIORITY = \[/,/^\]$/' "$WORK/script.js" > "$WORK/tally.mjs"
awk '/^function roundRobin\(/,/^}$/' "$WORK/script.js" >> "$WORK/tally.mjs"
awk '/^function selectForRefutation\(/,/^}$/' "$WORK/script.js" >> "$WORK/tally.mjs"
awk '/^function composeStanding\(/,/^}$/' "$WORK/script.js" >> "$WORK/tally.mjs"
awk '/^function tallyVotes\(/,/^}$/' "$WORK/script.js" >> "$WORK/tally.mjs"
awk '/^function orderGateFindings\(/,/^}$/' "$WORK/script.js" >> "$WORK/tally.mjs"
awk '/^function composeGateStanding\(/,/^}$/' "$WORK/script.js" >> "$WORK/tally.mjs"
assert_contains "extracted_priority" "const REFUTE_PRIORITY" "$(cat "$WORK/tally.mjs")"
assert_contains "extracted_round_robin" "function roundRobin" "$(cat "$WORK/tally.mjs")"
assert_contains "extracted_select" "function selectForRefutation" "$(cat "$WORK/tally.mjs")"
assert_contains "extracted_compose" "function composeStanding" "$(cat "$WORK/tally.mjs")"
assert_contains "extracted_tally" "function tallyVotes" "$(cat "$WORK/tally.mjs")"
assert_contains "extracted_order_gate" "function orderGateFindings" "$(cat "$WORK/tally.mjs")"
assert_contains "extracted_compose_gate" "function composeGateStanding" "$(cat "$WORK/tally.mjs")"

# The priority table drives which source keeps its refute slot, so a silent truncation of the
# extraction would make every ordering assertion below meaningless.
assert_contains "priority_has_datadog" "'datadog'" "$(cat "$WORK/tally.mjs")"
assert_contains "priority_has_blast_radius" "'blast-radius'" "$(cat "$WORK/tally.mjs")"

# Both refute caps are pinned to their documented values, extracted the same way REFUTE_PRIORITY is.
#
# MAX_REFUTED's own comment says a test pins its floor. Nothing did. The floor case below passes a
# literal 6 as its cap, so raising the constant to 8 left the whole suite green while a verdict-flipping
# source went unchallenged on every real run. MAX_GATE_REFUTED was not covered at all, and it is what
# guarantees each of the two triage agents one skeptic slot.
#
# awk with no match yields an empty string rather than exiting non-zero, so a rename fails these
# assertions loudly instead of passing on nothing.
MAX_REFUTED="$(awk '/^const MAX_REFUTED = /{print $4}' "$WORK/script.js")"
assert_eq "max_refuted_is_six" "6" "$MAX_REFUTED"
MAX_GATE_REFUTED="$(awk '/^const MAX_GATE_REFUTED = /{print $4}' "$WORK/script.js")"
assert_eq "max_gate_refuted_is_two" "2" "$MAX_GATE_REFUTED"

cat >> "$WORK/tally.mjs" <<'PROBE'

const f = { claim: 'c', verdict_critical: true }
const kill = { refuted: true }
const keep = { refuted: false }
const out = (votes, panel) => {
  const r = tallyVotes(f, votes, panel)
  return [r.survived, r.contested, r.panelShort, r.refutationRan].join(',')
}
console.log('both_refute=' + out([kill, kill], 2))
console.log('both_keep=' + out([keep, keep], 2))
console.log('split=' + out([kill, keep], 2))
console.log('split_reversed=' + out([keep, kill], 2))
console.log('one_dead_refutes=' + out([null, kill], 2))
console.log('one_dead_keeps=' + out([keep, null], 2))
console.log('both_dead=' + out([null, null], 2))
console.log('claim_preserved=' + tallyVotes(f, [keep, keep], 2).claim)
console.log('votes_exclude_null=' + tallyVotes(f, [null, keep], 2).votes.length)

// An undecided skeptic abstains. It must not count as a refutation, which is what it would have to
// express before `undecided` existed, and it must not count as a vote to keep either.
const dunno = { refuted: true, undecided: true }
console.log('both_undecided=' + out([dunno, dunno], 2))
console.log('one_undecided_one_kill=' + out([dunno, kill], 2))
console.log('undecided_not_in_votes=' + tallyVotes(f, [dunno, keep], 2).votes.length)

// selectForRefutation: the budget must not be monopolized by one lens, and telemetry must not be
// starved just because its findings arrive after every code lens.
const c = (lens, claim, locator, status = 'confirmed') => ({ lens, claim, locator, status, verdict_critical: true })
const sel = (arr, cap) => {
  const r = selectForRefutation(arr, cap)
  return {
    picked: r.selected.map((x) => x.lens + ':' + x.claim).join('|'),
    deferred: r.deferred.map((x) => x.lens + ':' + x.claim).join('|'),
    dupes: r.duplicates.length,
  }
}

// Three lenses report the SAME claim at the same locator. It must collapse to one slot, not three.
const dup = sel([
  c('reproduce', 'tax applied twice', 'tax.ts:88'),
  c('claim-audit', 'TAX APPLIED TWICE', ' tax.ts:88 '),
  c('solution-feasibility', 'tax applied twice', 'tax.ts:88'),
  c('datadog', 'stopped when 812 merged', 'query:1'),
], 4)
console.log('dup_picked=' + dup.picked)
console.log('dup_count=' + dup.dupes)

// Six lenses each with a critical finding, telemetry last. Round-robin must reach datadog inside a
// budget of 4, which a positional slice never would.
const starve = sel([
  c('reproduce', 'a', 'l1'),
  c('merged-delta', 'b', 'l2'),
  c('open-pr-overlap', 'c', 'l3'),
  c('blast-radius', 'd', 'l4'),
  c('solution-feasibility', 'e', 'l5'),
  c('claim-audit', 'f', 'l6'),
  c('datadog', 'g', 'l7'),
], 6)
console.log('starve_has_datadog=' + starve.picked.includes('datadog:g'))
console.log('starve_first=' + starve.picked.split('|')[0])
console.log('starve_deferred=' + starve.deferred)

// MAX_REFUTED is documented as sitting exactly at its floor: six sources can flip the verdict on
// their own, and a cap of six is what gives each of them a slot. Every one of the nine sources reports
// one critical finding here, so the first round-robin pass has to spend all six slots on the six
// flippers and defer claim-audit plus the two scope lenses. If this ever fails, either a flipping
// source is going unchallenged or the cap moved.
const floorCase = sel([
  c('blast-radius', 'a', 'l1'),
  c('solution-feasibility', 'b', 'l2'),
  c('claim-audit', 'c', 'l3'),
  c('reproduce', 'd', 'l4'),
  c('merged-delta', 'e', 'l5'),
  c('open-pr-overlap', 'f', 'l6'),
  c('ticket-overlap', 'g', 'l7'),
  c('amplitude', 'h', 'l8'),
  c('datadog', 'i', 'l9'),
], 6)
console.log('floor_picked=' + floorCase.picked)
console.log('floor_deferred=' + floorCase.deferred)

// One lens with many criticals must not take every slot while other lenses wait.
const hog = sel([
  c('reproduce', 'a', 'l1'),
  c('reproduce', 'b', 'l2'),
  c('reproduce', 'c', 'l3'),
  c('reproduce', 'd', 'l4'),
  c('datadog', 'e', 'l5'),
], 4)
console.log('hog_picked=' + hog.picked)

// Under budget: everything is selected and nothing deferred.
const few = sel([c('reproduce', 'a', 'l1'), c('datadog', 'b', 'l2')], 4)
console.log('few_picked=' + few.picked)
console.log('few_deferred=[' + few.deferred + ']')

// Empty input must not hang the round-robin loop.
const none = sel([], 4)
console.log('none_picked=[' + none.picked + ']')

// Two lenses reaching OPPOSITE conclusions about the same line must NOT collapse. Keyed on claim and
// locator alone they hashed the same, so the refuting copy was dropped and the survivor reported the
// disagreement as agreement.
const contra = selectForRefutation([
  c('reproduce', 'tax applied twice', 'tax.ts:88', 'confirmed'),
  c('claim-audit', 'tax applied twice', 'tax.ts:88', 'refuted'),
], 6)
console.log('contra_kept=' + contra.selected.length)
console.log('contra_dupes=' + contra.duplicates.length)
console.log('contra_flagged=' + (contra.selected[0].contradicted_by || []).map((x) => x.lens + ':' + x.status).join(','))
console.log('contra_not_agreement=' + (contra.selected[0].also_reported_by === undefined))

// Same claim, same locator, SAME status is real agreement and still collapses to one slot.
const agree = selectForRefutation([
  c('reproduce', 'same', 'l1', 'confirmed'),
  c('claim-audit', 'same', 'l1', 'confirmed'),
], 6)
console.log('agree_kept=' + agree.selected.length)
console.log('agree_no_contradiction=' + (agree.selected[0].contradicted_by === undefined))

// A dead panel must carry a whyUnrefuted value, since the synthesizer is told to read it whenever
// refutationRan is false.
console.log('dead_panel_why=' + tallyVotes(f, [null, null], 2).whyUnrefuted)
console.log('live_panel_why=' + (tallyVotes(f, [keep, keep], 2).whyUnrefuted === undefined))

// The kept copy records who else said it, so agreement is not lost by the collapse.
console.log('also_reported=' + (selectForRefutation([
  c('reproduce', 'x', 'l1'),
  c('claim-audit', 'x', 'l1'),
], 4).selected[0].also_reported_by || []).join(','))

// A collapsed duplicate must never re-enter `standing`. This calls the script's OWN composeStanding
// rather than rebuilding the composition here, so deleting the duplicates guard from the real script
// fails this assertion instead of leaving it green.
const dupA = c('reproduce', 'same claim', 'same:1')
const dupB = c('claim-audit', 'same claim', 'same:1')
const allF = [dupA, dupB]
const picked = selectForRefutation(allF, 6)
const refuted = picked.selected.map((f) => tallyVotes(f, [kill, kill], 2))
console.log('standing_after_refuted_dup=' + composeStanding(allF, picked.selected, picked.duplicates, refuted, []).length)

// The same composition with a SURVIVING claim keeps exactly one copy, not the twin as well.
const survived = picked.selected.map((f) => tallyVotes(f, [keep, keep], 2))
console.log('standing_after_survived_dup=' + composeStanding(allF, picked.selected, picked.duplicates, survived, []).length)

// A finding the cap never reached is kept, and marked as never challenged.
const spare = c('blast-radius', 'untouched', 'l9')
const withSpare = composeStanding([dupA, dupB, spare], picked.selected, picked.duplicates, survived, [], [spare])
console.log('spare_kept=' + withSpare.length)
console.log('spare_unrefuted=' + withSpare.filter((f) => f.claim === 'untouched')[0].refutationRan)
console.log('spare_why=' + withSpare.filter((f) => f.claim === 'untouched')[0].whyUnrefuted)

// A finding that is not verdict_critical was never eligible for a skeptic, so it must NOT be reported
// as one the budget failed to reach. Conflating the two makes the synthesizer discount ordinary
// evidence, which is most of a normal run.
const plain = { lens: 'blast-radius', claim: 'ordinary', locator: 'l8', verdict_critical: false }
const withPlain = composeStanding([plain], [], [], [], [], [])
console.log('plain_why=' + withPlain[0].whyUnrefuted)
console.log('plain_kept=' + withPlain.length)

// The third arm must be reachable and distinct. A verdict-critical finding that is neither selected
// nor deferred is an invariant break, and labelling it 'budget-exhausted' would hide that. Passing an
// empty `deferred` is what makes the two arms distinguishable at all.
const orphanCritical = c('reproduce', 'critical but unaccounted', 'l7')
console.log('orphan_why=' + composeStanding([orphanCritical], [], [], [], [], [])[0].whyUnrefuted)

// --- gate pass ---
// A gate finding argues about whether the work is worth doing, not about whether the ticket is real,
// so it never carries verdict_critical. `gate_critical` marks the ones that argue AGAINST fixing,
// which are the only ones a skeptic attacks.
const g = (lens, claim, gate_critical = true) => ({ lens, claim, locator: 'l', status: 'confirmed', gate_critical })

// One slot per source. Two dismissals from one agent must not starve the other agent entirely.
const gateOrder = orderGateFindings([
  g('reachability', 'flag is off in prod'),
  g('reachability', 'no callers outside tests'),
  g('fix-cost', 'three files plus a backfill'),
])
console.log('gate_order=' + gateOrder.map((x) => x.lens + ':' + x.claim).join('|'))

// Empty input must terminate rather than spin.
console.log('gate_order_empty=[' + orderGateFindings([]).map((x) => x.claim).join('|') + ']')

// A finding that argues FOR fixing was never eligible for a skeptic. It must say so, and must not
// claim the budget ran out.
const forFixing = g('reachability', 'live and reached by every request', false)
const gateStandingPlain = composeGateStanding([forFixing], [], [], [], [])
console.log('gate_plain_why=' + gateStandingPlain[0].whyUnrefuted)
console.log('gate_plain_ran=' + gateStandingPlain[0].refutationRan)

// A dismissal the cap could not reach is a real gap and gets its own reason, distinct from the above.
const spared = g('fix-cost', 'a month of work')
const gateStandingSpared = composeGateStanding([spared], [], [], [], [spared])
console.log('gate_spare_why=' + gateStandingSpared[0].whyUnrefuted)

// A dismissal that was refuted must not come back. It is in toRefute and in tallied, and the killed
// copy is excluded by the survived filter, so it must appear zero times.
const killedGate = g('reachability', 'dead code')
const killedTally = tallyVotes(killedGate, [kill, kill], 2)
const gateStandingKilled = composeGateStanding([killedGate], [killedGate], [killedTally], [], [])
console.log('gate_killed_count=' + gateStandingKilled.length)

// A dismissal that survived its panel appears exactly once, from the tallied copy.
const heldGate = g('fix-cost', 'needs a migration')
const heldTally = tallyVotes(heldGate, [keep, keep], 2)
const gateStandingHeld = composeGateStanding([heldGate], [heldGate], [heldTally], [], [])
console.log('gate_held_count=' + gateStandingHeld.length)
console.log('gate_held_ran=' + gateStandingHeld[0].refutationRan)

// The third arm must be reachable and distinct here too, exactly as it is in composeStanding. A
// dismissal in neither toRefute nor deferred is an invariant break, and labelling it
// 'gate-budget-exhausted' would hide it behind a budget that was never spent. This is the case that
// proves the three gate labels are three states rather than two plus a synonym.
const orphanGate = g('fix-cost', 'dismissive but unaccounted')
console.log('gate_orphan_why=' + composeGateStanding([orphanGate], [], [], [], [])[0].whyUnrefuted)
PROBE

node "$WORK/tally.mjs" > "$WORK/out.txt"
result() { grep "^$1=" "$WORK/out.txt" | cut -d= -f2-; }

# survived,contested,panelShort,refutationRan

# A full panel agreeing it is refuted is the ONLY way a finding dies.
assert_eq "both_refute_kills" "false,false,false,true" "$(result both_refute)"

# A full panel that agrees it holds keeps it, uncontested.
assert_eq "both_keep_survives" "true,false,false,true" "$(result both_keep)"

# A split panel keeps the finding and flags it, so the synthesizer turns it into a question.
# Order must not matter.
assert_eq "split_is_contested" "true,true,false,true" "$(result split)"
assert_eq "split_order_agnostic" "true,true,false,true" "$(result split_reversed)"

# One dead skeptic must NOT let the survivor decide alone, in either direction. Skeptics are told
# to default to refuted when uncertain, so a lone refuting vote would otherwise delete a
# well-evidenced finding. One vote did land, so refutationRan stays true.
assert_eq "one_dead_refutes_survives" "true,false,true,true" "$(result one_dead_refutes)"
assert_eq "one_dead_keeps_survives" "true,false,true,true" "$(result one_dead_keeps)"

# The regression this test exists for: zero live votes is an infrastructure failure, not a
# unanimous refutation. It must survive, be marked panelShort, and report refutationRan false,
# because nothing attacked it and a reader must be able to tell that from a finding that held.
assert_eq "both_dead_survives_unchallenged" "true,false,true,false" "$(result both_dead)"

# The finding's own fields survive the tally, and dead votes are not counted as votes.
assert_eq "claim_preserved" "c" "$(result claim_preserved)"
assert_eq "votes_exclude_null" "1" "$(result votes_exclude_null)"

# An abstention is not a vote in either direction. A whole panel abstaining leaves the finding standing
# and flagged, exactly like a panel that never reported, and it must never read as a refutation even
# though the abstaining skeptic also set refuted:true.
assert_eq "both_undecided_survives" "true,false,true,false" "$(result both_undecided)"
assert_eq "one_undecided_cannot_be_outvoted" "true,false,true,true" "$(result one_undecided_one_kill)"
assert_eq "undecided_excluded_from_votes" "1" "$(result undecided_not_in_votes)"

# --- selectForRefutation ---

# Duplicate claims collapse on claim+locator, case and whitespace insensitive, so one claim cannot
# consume the refute budget just because several lenses agreed on it.
assert_eq "dup_collapses_to_one_slot" "datadog:stopped when 812 merged|reproduce:tax applied twice" "$(result dup_picked)"
assert_eq "dup_counted" "2" "$(result dup_count)"

# The regression this half of the test exists for. Telemetry findings arrive after every code lens, so
# a positional cut could never reach them. Priority order must put datadog first, and the lens whose
# findings only shape scope is the one that gets deferred instead.
assert_eq "telemetry_not_starved" "true" "$(result starve_has_datadog)"
assert_eq "telemetry_goes_first" "datadog:g" "$(result starve_first)"
assert_eq "scope_lens_deferred_instead" "blast-radius:d" "$(result starve_deferred)"

# One noisy source must not take every slot while another source waits with nothing challenged.
assert_eq "no_single_lens_hogs_budget" "datadog:e|reproduce:a|reproduce:b|reproduce:c" "$(result hog_picked)"

# Every verdict-flipping source keeps a refute slot at the documented cap, and only the three that
# cannot flip the verdict on their own get deferred.
assert_eq "all_six_flippers_refuted" \
  "datadog:i|amplitude:h|ticket-overlap:g|open-pr-overlap:f|merged-delta:e|reproduce:d" \
  "$(result floor_picked)"
assert_eq "only_non_flippers_deferred" "claim-audit:c|solution-feasibility:b|blast-radius:a" \
  "$(result floor_deferred)"

# Under budget, everything is picked and nothing is deferred.
assert_eq "under_budget_picks_all" "datadog:b|reproduce:a" "$(result few_picked)"
assert_eq "under_budget_defers_none" "[]" "$(result few_deferred)"

# An empty critical list must terminate rather than spin.
assert_eq "empty_terminates" "[]" "$(result none_picked)"

# Collapsing a duplicate keeps the agreement rather than discarding it.
assert_eq "agreement_preserved" "claim-audit" "$(result also_reported)"

# Opposite statuses on the same line are a contradiction, not a duplicate. Both must survive, neither
# may be recorded as agreement, and both must be flagged so the synthesizer has to resolve it.
assert_eq "contradiction_both_survive" "2" "$(result contra_kept)"
assert_eq "contradiction_not_deduped" "0" "$(result contra_dupes)"
assert_eq "contradiction_flagged" "claim-audit:refuted" "$(result contra_flagged)"
assert_eq "contradiction_not_agreement" "true" "$(result contra_not_agreement)"

# Genuine agreement still collapses, and is not mislabelled a contradiction.
assert_eq "agreement_still_collapses" "1" "$(result agree_kept)"
assert_eq "agreement_not_flagged" "true" "$(result agree_no_contradiction)"

# A finding whose whole panel failed carries its own whyUnrefuted, and a healthy panel carries none.
assert_eq "dead_panel_has_reason" "panel-failed" "$(result dead_panel_why)"
assert_eq "live_panel_has_no_reason" "true" "$(result live_panel_why)"

# A unanimously refuted claim must not come back through the twin that was collapsed into it. Both
# copies have to be accounted for, or the synthesizer sees the refuted claim as a live one. This runs
# the script's own composeStanding, so removing the guard there fails here.
assert_eq "refuted_dup_not_revived" "0" "$(result standing_after_refuted_dup)"

# A surviving claim appears exactly once, not twice via its collapsed twin.
assert_eq "survived_dup_appears_once" "1" "$(result standing_after_survived_dup)"

# A finding the cap never reached still reaches the synthesizer, flagged as never challenged.
assert_eq "uncapped_finding_kept" "2" "$(result spare_kept)"
assert_eq "uncapped_finding_marked" "false" "$(result spare_unrefuted)"

# A critical finding the budget could not reach is a real coverage gap and says so.
assert_eq "deferred_reason_is_budget" "budget-exhausted" "$(result spare_why)"

# A non-critical finding was never eligible for a skeptic, so it must report that reason rather than
# claiming the budget ran out. Otherwise the synthesizer discounts most of a normal run's evidence.
assert_eq "ineligible_reason_is_not_critical" "not-verdict-critical" "$(result plain_why)"
assert_eq "ineligible_finding_kept" "1" "$(result plain_kept)"

# The three arms must be three distinct values. If 'unaccounted' ever collapses into
# 'budget-exhausted', `deferred` becomes dead weight again and this assertion is what notices.
assert_eq "unaccounted_is_its_own_state" "unaccounted" "$(result orphan_why)"

# --- gate pass ---

# One refute slot per source. A source with two dismissals must not consume both slots while the
# other source has nothing challenged, which is the same starvation the verdict pass guards against.
assert_eq "gate_one_slot_per_source" \
  "reachability:flag is off in prod|fix-cost:three files plus a backfill|reachability:no callers outside tests" \
  "$(result gate_order)"
assert_eq "gate_order_empty_terminates" "[]" "$(result gate_order_empty)"

# A finding arguing FOR the fix is never eligible for a skeptic, by design. It must report that
# rather than looking like a budget casualty, or the synthesizer discounts it.
assert_eq "gate_for_fixing_is_ineligible" "argues-for-fixing" "$(result gate_plain_why)"
assert_eq "gate_for_fixing_unrefuted" "false" "$(result gate_plain_ran)"

# A dismissal the cap could not reach is a real coverage gap and says so distinctly.
assert_eq "gate_deferred_reason_is_budget" "gate-budget-exhausted" "$(result gate_spare_why)"

# A refuted dismissal must not reach the synthesizer at all. Otherwise a killed "nobody is affected"
# claim rides into the verdict as a live one, which is the whole reason dismissals get attacked.
assert_eq "gate_refuted_not_revived" "0" "$(result gate_killed_count)"

# A dismissal that held appears exactly once, and is marked as actually challenged.
assert_eq "gate_survivor_appears_once" "1" "$(result gate_held_count)"
assert_eq "gate_survivor_was_challenged" "true" "$(result gate_held_ran)"

# The three gate arms must be three distinct values, same as the verdict pass. If 'unaccounted' ever
# collapses into 'gate-budget-exhausted', `gateDeferred` becomes dead weight and this is what notices.
assert_eq "gate_unaccounted_is_its_own_state" "unaccounted" "$(result gate_orphan_why)"

# --- the verdict_critical firewall ---
#
# A triage finding that carried verdict_critical would enter selectForRefutation, spend a slot the
# verdict pass needs, and break MAX_REFUTED's floor argument. Two things keep it out, and both are
# asserted here rather than over a local fixture. The old check ran `!x.verdict_critical` over findings
# this file builds itself, which never set the field, and neither function under test reads it. It could
# not fail.
#
# Every grep -c below ends in `|| true`, for the same reason the counts further down do. grep exits 1 on
# zero matches and this file runs under `set -euo pipefail`, so an unguarded count terminates the run
# instead of asserting zero.

# The schema is the first half. `verdict_critical` is absent from TRIAGE_FINDING_ITEM's required list on
# purpose, so no triage agent is ever asked for it.
awk '/^const TRIAGE_FINDING_ITEM = \{/,/^\}$/' "$SKILL" | grep '^  required: \[' > "$WORK/triage_required.txt" || true

# Prove the extraction landed before counting an absence in it. An awk range that matched nothing yields
# zero hits for verdict_critical too, which is exactly the vacuous shape this section replaces.
TRIAGE_REQUIRED_FOUND="$(grep -c "'gate_critical'" "$WORK/triage_required.txt" || true)"
assert_eq "triage_required_extracted" "1" "$TRIAGE_REQUIRED_FOUND"
TRIAGE_REQUIRED_LEAK="$(grep -c 'verdict_critical' "$WORK/triage_required.txt" || true)"
assert_eq "triage_required_omits_verdict_critical" "0" "$TRIAGE_REQUIRED_LEAK"

# The dispatch is the second half. Even if an agent volunteered the field, allFindings drops every
# triage result before `critical` is computed, so nothing reaches selectForRefutation. Grepped with the
# declaration line as its anchor, so the filter cannot be credited to some other call on `reported`.
grep -A1 '^const allFindings = reported$' "$WORK/script.js" > "$WORK/all_findings.txt" || true
ALL_FINDINGS_DECL="$(grep -c '^const allFindings = reported$' "$WORK/all_findings.txt" || true)"
assert_eq "all_findings_decl_found" "1" "$ALL_FINDINGS_DECL"
ALL_FINDINGS_FILTER="$(grep -c "probeKind !== 'triage'" "$WORK/all_findings.txt" || true)"
assert_eq "all_findings_excludes_triage" "1" "$ALL_FINDINGS_FILTER"

# --- cross-file invariants ---
#
# These three facts live in two files each and nothing else checks them. The wrap risk below
# applies differently to each of the three checks, so they are worth naming separately.
#
# The required-token loop checks single tokens with no space in them, so a line wrap cannot split
# one and there is nothing to mitigate.
#
# The lens-count pattern matches indentation inside the fenced JavaScript block. That text is not
# hand-wrapped prose, so the same is true there.
#
# The companion-count check is the one multi-word phrase living in wrapped prose. That is why it
# is asserted positively instead of checked for absence. A wrap that splits "all three" makes the
# check fail instead of pass, and a false failure is the safe direction because someone looks. A
# zero-hit check on the same phrase would have been a guaranteed pass.

# Everything the inline triage path needs from VALIDATION.md must be named in SKILL.md, or that path
# silently skips it while the workflow path runs it.
#
# The three agent names alone did not check that. They are a whole-file grep, and all three appear in
# Step 1's classifier rule, so the entire Step 2 inline-triage paragraph could be deleted and this loop
# would still pass. `TRIAGE_SCHEMA` and `triageRules` live in SKILL.md only inside that paragraph, so
# losing it now fails here. Dropping the schema is the worst of the three companions, because
# `gate_critical` lives there and without it nothing separates an argument against the work from one
# for it.
for required in reachability exploit-realism fix-cost TRIAGE_SCHEMA triageRules; do
  if ! grep -q "$required" "$SKILL_MD"; then
    echo "'$required' is required by VALIDATION.md's triage pass but never named in SKILL.md" >&2
    exit 1
  fi
done
echo "cross_file_triage_agents=ok"

# The completeness critic's premise counts the code lenses. If LENSES grows and that prompt does not,
# the one pass whose job is finding omissions starts from a false premise.
#
# Scoped to the LENSES array literal on purpose. A bare grep for the key lines matches TELEMETRY and
# TRIAGE too, which have the same indentation, and would return 12 rather than 7.
#
# Every grep -c here ends in `|| true`. This file runs under `set -euo pipefail` and grep exits 1 on
# zero matches, so an unguarded count terminates the run instead of asserting zero. pipefail means the
# guard has to sit on the whole substitution, not on the grep alone.
LENS_COUNT="$(awk '/^const LENSES = \[/,/^\]$/' "$SKILL" | grep -c "^    key: '" || true)"
assert_eq "lens_count_is_seven" "7" "$LENS_COUNT"

# The two files must agree on how many companions a brief needs. They disagreed once, three against
# four, for the same three items.
#
# Asserted POSITIVELY, on purpose. The obvious form is a zero-hit check for the old wrong string, and
# that form cannot fail. Both files are hand-wrapped, so a future rewrap that splits "all four" across
# a line break also yields zero, and grep is line-based so no pattern catches it. A zero-hit check on a
# multi-word phrase in wrapped prose is a guaranteed pass. Asserting the right string is present fails
# loudly if a rewrap splits it, and a false failure is the safe direction because someone looks.
SKILL_THREE="$(grep -c 'all three' "$SKILL_MD" || true)"
VALIDATION_THREE="$(grep -c 'all three' "$SKILL" || true)"
if [[ "$SKILL_THREE" == "0" || "$VALIDATION_THREE" == "0" ]]; then
  echo "the two files must both say a brief needs three companions" >&2
  echo "SKILL.md hits: $SKILL_THREE, VALIDATION.md hits: $VALIDATION_THREE" >&2
  exit 1
fi
echo "companion_count_agrees=ok"

echo "work_on_validation_test: ok"
