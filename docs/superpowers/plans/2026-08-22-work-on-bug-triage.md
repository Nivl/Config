# Impact and Effort Triage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an impact-and-effort assessment to the `work-on` skill's ticket-validation phase, for bug and security tickets only, so a reader can judge whether a fix is worth doing before any code gets written.

**Architecture:** Three new leaf agents form a third probe kind (`TRIAGE`) alongside the existing seven code lenses and two telemetry probes. They answer only what no existing agent can: whether the code path is live, whether an attacker could realistically act, and what the fix costs. The measurement half ("is anyone affected right now") is answered by the existing Datadog and Amplitude probes, which Step 1 now switches on for any bug or security ticket. A separate small refutation pass attacks only findings that argue against fixing. The synthesizer assembles a `triage` block on the verdict object, and Step 4 presents it and asks the user a priority call.

**Tech Stack:** Markdown skill files. One embedded JavaScript workflow script inside a single fenced block in `VALIDATION.md`. A `zsh` test harness at `tests/work_on_validation_test.sh` that extracts the script, parses it with `node --check`, and runs its pure functions under `node`.

**Spec:** `docs/superpowers/specs/2026-08-22-work-on-bug-triage-design.md`

The spec counts 29 edit sites. This plan implements 30. The extra one is `TRIAGE_FINDING_ITEM`, a
sibling of `FINDING_ITEM` that drops `verdict_critical` from the required list, found while working
out how a triage agent could be prevented from setting that flag at all rather than merely told not
to. Task 2 covers it.

## Global Constraints

- **Repo root is `/Users/melvin/.melvin/config`.** All paths below are relative to it.
- **`docs/superpowers/` is line 1 of `.gitignore`.** This plan and its spec are deliberately untracked. Do not `git add -f` them.
- **Authored prose uses plain ASCII.** `->` not the arrow glyph, `...` not the ellipsis glyph, straight quotes, and two sentences instead of an em-dash, an en-dash, a ` - `, or a clause-joining `:`. This governs every word written into the skill files and every commit message. It does not govern quoted file content.
- **`VALIDATION.md` must contain exactly one ` ```javascript ` fence.** `tests/work_on_validation_test.sh:30` hard-exits otherwise, before a single assertion runs. Every script change goes inside the existing fence.
- **Two sed anchors in the script must stay byte-identical:** the line `export const meta = {` and the line `return {`, each at column zero. The test rewrites them to make the body parseable as a module. See `tests/work_on_validation_test.sh:55`.
- **Any new pure function must be a top-level `function name(` whose body closes on a `}` at column zero,** and must not reference the injected globals `args`, `log`, `agent`, `parallel`, or `phase`. The test extracts functions with line-anchored `awk` ranges. A function declared as a `const` arrow, or one with an indented closing brace, is silently un-extractable and `awk` emits nothing rather than failing.
- **Never write the eight characters `work-on` inside the fenced script block,** in code or in a comment. The `deny-review-in-workflow.py` hook scans the persisted script body for that name at a word boundary and would deny the skill's own `Workflow` call. The call passes today only because `work-on-validate` carries a trailing hyphen, which the hook's post-boundary excludes. Outside the fence, in ordinary markdown prose, the name is fine.
- **`git` runs outside the sandbox and must run alone.** No pipe, no chaining, no `cd x && git ...`. Run `cd` as its own call first. Never use top-level `git -C`.
- **Do not run `melvin-config claude sync`.** It writes outside the sandbox and will fail. The skills are symlinked into `~/.claude/skills`, so an edit to the repo file is live immediately.
- **Run the test with `zsh tests/work_on_validation_test.sh`** from the repo root. It prints `work_on_validation_test: ok` on success. It is allow-listed under `hooks/deny-bash-script.json`, so it runs without a prompt, but it must run alone with no pipe.

---

### Task 1: The two pure gate functions

The only genuinely unit-testable part of this change. Do it first so later tasks can call it.

`orderGateFindings` decides which dismissals get skeptics when the budget binds. It round-robins by source so one agent cannot take both slots. Without it, a positional slice lets `reachability` spend both slots on two of its own findings while `fix-cost`'s "this is a three week job" goes unchallenged, which is exactly the starvation the main pass exists to prevent.

`composeGateStanding` assembles what the synthesizer sees. It is a separate function from `composeStanding` rather than a second call into it, because `composeStanding`'s label ladder tests `!f.verdict_critical` first and would stamp every gate finding `not-verdict-critical`, a label the synthesizer prompt reads as "nothing is missing".

**Files:**
- Modify: `shared_config/.claude/skills/work-on/VALIDATION.md` (inside the fence, immediately after `composeStanding` ends and before the `tallyVotes` comment begins, around line 366)
- Test: `tests/work_on_validation_test.sh`

**Interfaces:**
- Consumes: nothing.
- Produces: `orderGateFindings(gateCritical) -> Array<finding>` and `composeGateStanding(allGateFindings, toRefute, tallied, tallyLost, deferred) -> Array<finding>`. Task 3 calls both. `composeGateStanding` returns findings each carrying `refutationRan: boolean` and, when that is false, `whyUnrefuted` set to one of `'argues-for-fixing'`, `'gate-budget-exhausted'`, or `'unaccounted'`.

- [ ] **Step 1: Add the awk extraction for the two new functions**

In `tests/work_on_validation_test.sh`, after the existing `awk` line for `tallyVotes` (line 60), add:

```bash
awk '/^function orderGateFindings\(/,/^}$/' "$WORK/script.js" >> "$WORK/tally.mjs"
awk '/^function composeGateStanding\(/,/^}$/' "$WORK/script.js" >> "$WORK/tally.mjs"
```

Then after the existing `extracted_tally` assertion, add:

```bash
assert_contains "extracted_order_gate" "function orderGateFindings" "$(cat "$WORK/tally.mjs")"
assert_contains "extracted_compose_gate" "function composeGateStanding" "$(cat "$WORK/tally.mjs")"
```

- [ ] **Step 2: Write the failing probe cases**

Append to the `PROBE` heredoc in `tests/work_on_validation_test.sh`, after the existing `orphan_why` case:

```javascript
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

// A gate finding must never carry verdict_critical, or it would leak into the verdict refute pass.
console.log('gate_no_verdict_critical=' + [forFixing, spared, killedGate].every((x) => !x.verdict_critical))
PROBE
```

Note: the closing `PROBE` above marks the end of the existing heredoc. Insert the cases before it rather than adding a second heredoc.

- [ ] **Step 3: Add the assertions**

In `tests/work_on_validation_test.sh`, before the final `echo "work_on_validation_test: ok"`:

```bash
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

# Gate findings must never carry verdict_critical. If one did it would enter selectForRefutation and
# spend a slot the verdict pass needs, and MAX_REFUTED's floor argument would stop holding.
assert_eq "gate_never_verdict_critical" "true" "$(result gate_no_verdict_critical)"
```

- [ ] **Step 4: Run the test to verify it fails**

Run: `zsh tests/work_on_validation_test.sh`

Expected: FAIL at `extracted_order_gate`, because the function does not exist yet and `awk` emitted nothing.

- [ ] **Step 5: Add the two functions to the script**

In `shared_config/.claude/skills/work-on/VALIDATION.md`, inside the fence, immediately after the closing brace of `composeStanding` and before the comment block that introduces `tallyVotes`:

```javascript
// Orders the dismissals a skeptic could attack, one per source before any source gets a second slot.
// Pure, and a named function so the test exercises this code rather than a copy of it.
//
// A plain slice would let one agent take the whole gate budget. Two dismissals from the reachability
// agent would leave "the fix is a month of work" unchallenged, and a cost dismissal is exactly as
// capable of killing the work as an impact one. This is the same round-robin as the verdict pass, minus
// the priority table. There are only two possible sources here and neither outranks the other.
function orderGateFindings(gateCritical) {
  const bySource = new Map()
  for (const f of gateCritical) {
    if (!bySource.has(f.lens)) bySource.set(f.lens, [])
    bySource.get(f.lens).push(f)
  }
  const queues = [...bySource.values()]
  const ordered = []
  let round = 0
  while (ordered.length < gateCritical.length) {
    let movedAny = false
    for (const q of queues) {
      if (q.length > round) {
        ordered.push(q[round])
        movedAny = true
      }
    }
    if (!movedAny) break
    round += 1
  }
  return ordered
}

// Assembles the triage findings the synthesizer gets to see. Pure, and deliberately separate from
// composeStanding rather than a second call into it.
//
// Three reasons it cannot be the same function. That one's label ladder tests `!f.verdict_critical`
// first, and no triage finding carries that flag, so every one of them would come back labelled
// 'not-verdict-critical', which the synthesize prompt reads as "nothing is missing". Its `accountedFor`
// set is built from one pass's arrays by object identity and it is called once, so a second call would
// either double-list a finding or revive one whose panel killed it. And the labels themselves differ:
// what matters here is whether a finding argued against the work, not whether it could flip the verdict.
function composeGateStanding(allGateFindings, toRefute, tallied, tallyLost, deferred) {
  const accountedFor = new Set(toRefute)
  const deferredSet = new Set(deferred || [])
  return [
    ...allGateFindings
      .filter((f) => !accountedFor.has(f))
      .map((f) => ({
        ...f,
        refutationRan: false,
        // Only a finding arguing AGAINST the work is ever attacked, so the ineligible case is the
        // common one and must not read as a gap. A dismissal the cap could not reach is a real gap.
        whyUnrefuted: !f.gate_critical
          ? 'argues-for-fixing' // never eligible for a skeptic. Nothing is missing.
          : deferredSet.has(f)
            ? 'gate-budget-exhausted' // eligible, and the cap could not reach it. A real gap.
            : 'unaccounted', // dismissive, not selected, not deferred. Should be impossible.
      })),
    ...tallied.filter((f) => f.survived),
    ...tallyLost,
  ]
}
```

- [ ] **Step 6: Run the test to verify it passes**

Run: `zsh tests/work_on_validation_test.sh`

Expected: PASS, printing `work_on_validation_test: ok`.

If `gate_one_slot_per_source` fails with the three findings in input order, the round-robin loop is wrong. The expected order interleaves sources: first `reachability`, then `fix-cost`, then `reachability` again.

- [ ] **Step 7: Commit**

```bash
git add shared_config/.claude/skills/work-on/VALIDATION.md tests/work_on_validation_test.sh
git commit -m "feat(claude,skill): add the pure gate-refutation helpers" -m "Two functions for a second, smaller refutation pass over triage findings. Ordering round-robins by source so one agent cannot take both slots. Composing is deliberately separate from composeStanding, whose label ladder tests verdict_critical first and would stamp every triage finding as never-eligible.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: The TRIAGE array, its rules, and its schema

**Files:**
- Modify: `shared_config/.claude/skills/work-on/VALIDATION.md` (inside the fence, after the `TELEMETRY` array and `telemetryRules()`, before `const rules = () =>`, around line 754)

**Interfaces:**
- Consumes: `args.kind` (Task 7 makes Step 1 supply it), `UNTRUSTED` (already defined at line 204).
- Produces: `TRIAGE` (an array of `{ key, enabled, brief }`), `triageRules()`, and `TRIAGE_SCHEMA`. Task 3 dispatches them.

- [ ] **Step 1: Add the triage finding item first**

Order matters. `TRIAGE_SCHEMA` in Step 2 references `TRIAGE_FINDING_ITEM`, and a `const` referenced before its definition in file order throws at runtime rather than hoisting. `FINDING_ITEM` sits at ~506 and `TRIAGE` at ~754, so adding the item here puts it comfortably ahead of its use.

`FINDING_ITEM` requires `verdict_critical`, and a triage agent must never set it. Add a sibling immediately after `FINDING_ITEM` closes (around line 524), before `QUESTION_ITEM`:

```javascript
// Same shape as FINDING_ITEM with one field swapped. `verdict_critical` is absent on purpose rather
// than optional. A triage agent that set it would put a finding about whether the work is worth doing
// into selectForRefutation, spend a slot the verdict pass needs, and break MAX_REFUTED's floor.
const TRIAGE_FINDING_ITEM = {
  type: 'object',
  required: ['claim', 'status', 'locator', 'gate_critical', 'reasoning'],
  properties: {
    claim: { type: 'string', description: 'One sentence. What you determined.' },
    status: { enum: ['confirmed', 'refuted', 'unverifiable'] },
    locator: {
      type: 'string',
      description:
        'path:line, a commit sha, PR #N, or the telemetry query that produced the number. Use "none" only when unverifiable.',
    },
    gate_critical: {
      type: 'boolean',
      description:
        'True only if this argues AGAINST doing the work. Those get attacked by skeptics. A finding arguing the work matters does not.',
    },
    reasoning: { type: 'string' },
  },
}
```

- [ ] **Step 2: Add the TRIAGE array**

Insert after the closing `]` of the `TELEMETRY` array and its `telemetryRules()` definition:

```javascript
// The triage pass. Bug and security tickets only, and at most two of these three ever run, because the
// benefit agent is chosen by kind. A feature ticket or a refactor runs none of them and the verdict's
// triage block is absent rather than empty, which are different things to a reader.
//
// These are NOT entries in LENSES, and the distinction is load-bearing rather than tidy. LENSES is
// dispatched wholesale with no enabled filter, its length is logged as the lens count, the phase label
// and the completeness critic's prompt both assert seven of them, MAX_REFUTED's floor argument counts
// nine sources with six that can flip, and a test pins that floor. A conditionally-enabled member of
// that array falsifies every one of those. A third probe kind costs one filter and falsifies none.
//
// None of these may query Datadog or Amplitude. They inherit that ban through triageRules(), and it is
// the same ban every code lens gets. The measured half of impact belongs to the two probes, which Step 1
// switches on for any ticket of these kinds. A number an agent here wants and cannot reach comes back as
// an unverifiable finding naming the number, and the synthesizer reconciles it against the digests.
const TRIAGE = [
  {
    key: 'reachability',
    enabled: args.kind === 'bug',
    brief: `You are the reachability agent. One question: would anybody notice if this were never fixed?

You are NOT judging whether the fix is hard. Another agent owns that and it cannot see your answer.

Answer each of these, and land every one on a path with a line number:

- Is the code path live at HEAD right now? Check the feature flag and its default, whether the route
  or job is registered, whether any caller exists outside tests, and whether this is dead code.
- If it is not live, what turns it on? Name the flag, the release, or the config value. A path that is
  not live yet is a schedule, not an absence, and saying "not live" without saying what flips it is
  half an answer.
- What must coincide for the defect to actually fire? "Every request" and "a retry, on a legacy tier,
  across a month boundary" are both real answers and they mean opposite things.
- Is there evidence in the tree that this has ever happened? A regression test, a bug-fix commit
  nearby, a guard someone added, an error handler naming this case. Distinguish that from a defect
  reasoned out by reading the code and never observed.

You cannot query Datadog or Amplitude, and the probes that can are running beside you. When your
answer turns on a production number you do not have, return it as an unverifiable finding that names
the number you wanted. Do not estimate it and do not treat its absence as a zero.

Set \`gate_critical: true\` on a finding only when it argues AGAINST doing the work, such as a flag
that is off, a path with no callers, or dead code. Those get attacked by skeptics. A finding arguing
that the work matters does not need one.`,
  },
  {
    key: 'exploit-realism',
    enabled: args.kind === 'security',
    brief: `You are the exploit-realism agent. One question: could a real attacker actually do this?

You are NOT judging whether the fix is hard. Another agent owns that and it cannot see your answer.

Answer all of these:

- Which attacker does this need? Unauthenticated internet, any logged-in user, a different tenant, or
  an insider. Pick one and say what the code requires to reach that position.
- The precondition chain. Everything the attacker must hold AT THE SAME TIME. For each item, how they
  would obtain it, and how long it stays valid. A value with a sixty second lifetime is a different
  proposition from a sequential id, and the window is the finding rather than a detail of it.
- Is the secret enumerable or leaked anywhere? A sequential or guessable id, an error message that
  returns it, a log line, a URL a third party sees, a referrer header, a cache key.
- What is the payoff? Reading one record and taking over an account are not the same ticket.
- What already blocks it? An authorization check upstream, a rate limit, one-time use, a short expiry,
  a WAF rule. An existing control is the fastest way this ticket ends, so look for one first.

Land every precondition on a path with a line number. A chain you assert without reading the code is
unverifiable, and reporting it that way is correct.

Set \`gate_critical: true\` on a finding only when it argues AGAINST doing the work, such as a control
that already blocks the attack or a precondition no realistic attacker could satisfy in the window.
Those get attacked by skeptics.`,
  },
  {
    key: 'fix-cost',
    enabled: args.kind === 'bug' || args.kind === 'security',
    brief: `You are the fix-cost agent. One question: what does fixing this cost?

**You are not told whether anybody is affected, and you must not go looking.** That is deliberate. An
agent that knows the defect is harmless talks itself into a cheap estimate, and one that knows it is
serious talks itself into an expensive one. Estimate the work as if the decision to do it had already
been made by somebody else.

Answer all of these, with a path for each:

- Which files and which lines change? Name them. If the ticket proposes no approach, cost the one the
  code makes most obvious and say that is what you costed.
- What is the test burden? Does the suite already cover this path? Name the test file. New fixtures,
  a new harness, or a test that needs production-shaped data all cost more than a new assertion.
- Is there a migration, a backfill, or any data change? Does it reverse? An irreversible data change
  is the single largest cost multiplier here.
- What does rollout need? A flag, a staged release, coordination with another team or service, an
  ordering constraint against another deploy.
- What is the risk of the fix itself? The code you would touch may be load-bearing for something the
  ticket never mentions.

Return a t-shirt size, S, M, L, or XL, with the reason. Never hours or days. A size whose reason is
"it seems small" is unverifiable. A size whose reason is "three files, one of which needs a backfill
over a table with no index on the filter column, at path:line" is evidence.

Set \`gate_critical: true\` on a finding only when it argues AGAINST doing the work, which here means a
cost large enough to change the decision. An S with a clear reason does not need a skeptic.`,
  },
]

const triageRules = () => `
You answer one question and one only. The other triage agents are blind to your answer and you are
blind to theirs, which is the point. Do not reason about their half.

Ground every claim in something checkable. A path with a line number, a commit sha, a PR number, or a
Datadog or Amplitude query with its result. A claim you cannot land on one of those is
"unverifiable", and reporting it that way is correct and useful.

**Do not query Datadog or Amplitude yourself.** Dedicated probes run beside you and own that. If your
answer turns on a production number you do not have, say so in a finding as unverifiable and name the
number you wanted. The synthesizer has the probes' digests and will reconcile it.

**You do not query the warehouse, whatever tools you can see.** Put the exact SQL in a question's
\`sql\` field instead, with what each plausible result would mean, and write \`SELECT\` only with a
bound on what it scans. The main thread pools every agent's queries and decides how to run them.

**Absence of a measurement is never a zero.** "No telemetry covers this path" and "nobody reaches this
path" are opposite conclusions and the second one closes a ticket. If you cannot tell them apart, that
is the finding.

**\`gate_critical\` is not \`verdict_critical\`.** You are not deciding whether the ticket is real. Never
set \`verdict_critical\` on anything you return. Set \`gate_critical: true\` only on a finding that argues
against doing the work, because those are the ones a skeptic attacks. Being wrong about a dismissal
ships a live defect and nobody revisits it. Being wrong in the other direction wastes a day and is
obvious immediately, so the two are not attacked equally.

You cannot reach the user. Return what you could not settle as a question, with \`searched\` listing
what you actually tried. A question here is a priority call or a query to run, never a request for a
number you were supposed to find yourself.
${UNTRUSTED}
`

const TRIAGE_SCHEMA = {
  type: 'object',
  required: ['agent', 'findings', 'questions'],
  properties: {
    agent: { enum: ['reachability', 'exploit-realism', 'fix-cost'] },
    // Only fix-cost fills this. A size with no reason is not usable, so both or neither.
    loe: {
      type: 'object',
      required: ['size', 'because'],
      properties: {
        size: { enum: ['S', 'M', 'L', 'XL'] },
        because: { type: 'string', description: 'What makes it that size. Name the files, the migration, the test burden.' },
      },
    },
    findings: { type: 'array', items: TRIAGE_FINDING_ITEM },
    questions: { type: 'array', items: QUESTION_ITEM },
  },
}
```

- [ ] **Step 3: Verify the script still parses**

Run: `zsh tests/work_on_validation_test.sh`

Expected: PASS. The parse check covers a stray unescaped backtick inside the new template literals, which is the realistic failure here. If `node --check` reports a syntax error, the line number it gives is relative to the extracted script and is the one useful thing it produces.

- [ ] **Step 4: Verify the hook constraint by hand**

Run: `grep -n "work-on" shared_config/.claude/skills/work-on/VALIDATION.md`

Expected: every hit is outside the fenced block, meaning in the prose sections or in a markdown link. If any hit falls between the ` ```javascript ` line and its closing fence, rewrite that text before committing. A hit inside the fence makes the skill's own `Workflow` call deny at runtime, and nothing in the test suite catches it.

- [ ] **Step 5: Commit**

```bash
git add shared_config/.claude/skills/work-on/VALIDATION.md
git commit -m "feat(claude,skill): add the triage briefs, rules, and schema" -m "Three leaf agents for bug and security tickets. Reachability and exploit-realism are chosen by kind so at most two run. fix-cost is blind to impact on purpose, because an agent holding both halves of the trade talks itself into a number.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Dispatch the triage agents and refute their dismissals

**Files:**
- Modify: `shared_config/.claude/skills/work-on/VALIDATION.md` (the `probes` array at ~825, the run log at ~830, the result stamping at ~886, and a new stage after the existing Refute stage at ~971)

**Interfaces:**
- Consumes: `TRIAGE`, `triageRules()`, `TRIAGE_SCHEMA` from Task 2. `orderGateFindings`, `composeGateStanding` from Task 1. `tallyVotes`, `SKEPTICS`, `CONTEXT`, `SKEPTIC_CONTEXT` already exist.
- Produces: `gateStanding` and `gateCoverage`, consumed by Task 4's synthesizer prompt and Task 6's return object.

- [ ] **Step 1: Add the cap next to the existing one**

After the `MAX_REFUTED` block and `SKEPTICS` (line 161), add:

```javascript
// The gate budget, separate from MAX_REFUTED on purpose. Two, because at most two triage agents run on
// any ticket and this gives each of them one slot before either gets a second.
//
// It is deliberately NOT folded into MAX_REFUTED. That cap sits exactly at its floor, six sources that
// can each flip the verdict against a budget of six, and a test pins the arrangement. Adding triage
// findings to the same pass would make them compete with a duplicate-ticket finding for a slot, and a
// wrongly-closed ticket costs more than an unchallenged cost estimate.
const MAX_GATE_REFUTED = 2
```

- [ ] **Step 2: Add triage to the probes array**

Replace the `probes` array and the log line beneath it (lines 824 to 830):

```javascript
const probes = [
  ...LENSES.map((lens) => ({ kind: 'lens', key: lens.key, spec: lens })),
  ...TELEMETRY.filter((t) => t.enabled).map((t) => ({ kind: 'telemetry', key: t.key, spec: t })),
  ...TRIAGE.filter((t) => t.enabled).map((t) => ({ kind: 'triage', key: t.key, spec: t })),
]

const enabledTelemetry = TELEMETRY.filter((t) => t.enabled).map((t) => t.key)
const enabledTriage = TRIAGE.filter((t) => t.enabled).map((t) => t.key)
log(`investigating with ${LENSES.length} code lenses, ${enabledTelemetry.length} telemetry probes${enabledTelemetry.length ? ` (${enabledTelemetry.join(', ')})` : ''}, and ${enabledTriage.length} triage agents${enabledTriage.length ? ` (${enabledTriage.join(', ')})` : ''}`)
```

`LENSES.length` stays honest because `LENSES` is still dispatched wholesale. Triage is counted separately because it is not.

- [ ] **Step 3: Add the triage branch to the dispatch**

In the `parallel` call over `probes`, the ternary currently has two arms. Add a third in the middle, so the existing lens arm stays first and the telemetry arm becomes the final fallback. Both existing prompt bodies stay byte-identical. The result:

```javascript
const raw = await parallel(
  probes.map((p) => () =>
    p.kind === 'lens'
      ? agent(
          // the existing lens prompt, unchanged
          `You are the "${p.key}" lens validating a Jira ticket. Answer ONLY your question.
...`,
          { label: `lens:${p.key}`, phase: 'Investigate', schema: FINDING_SCHEMA },
        )
      : p.kind === 'triage'
        ? agent(
            `${p.spec.brief}

## Rules

${triageRules()}

## Context

${CONTEXT}

Return your findings and your questions per the schema.`,
            { label: `triage:${p.key}`, phase: 'Investigate', schema: TRIAGE_SCHEMA },
          )
        : agent(
            // the existing telemetry prompt, unchanged
            `${p.spec.brief}
...`,
            { label: `telemetry:${p.key}`, phase: 'Investigate', schema: TELEMETRY_SCHEMA },
          ),
  ),
)
```

Do not paste the elided prompt bodies from this plan. Keep the real ones that are already in the file. The only new text is the triage arm.

The triage prompt has no role line of its own because each brief opens with "You are the ... agent", the way the telemetry briefs do. The lens arm is the only one that needs a role line built around its `ask`.

- [ ] **Step 4: Split triage findings out of the verdict pass**

After the existing line that builds `allFindings` (around line 897), the triage results must not enter it. Replace the two lines:

```javascript
const triageResults = reported.filter((r) => r.probeKind === 'triage')

// Triage findings are deliberately kept out of allFindings. They answer whether the work is worth
// doing, not whether the ticket is real, so they must never reach selectForRefutation or
// composeStanding. Both are keyed to verdict_critical and would mislabel every one of them.
const allFindings = reported
  .filter((r) => r.probeKind !== 'triage')
  .flatMap((r) => (r.findings || []).map((f) => ({ ...f, lens: r.probeKey })))
const critical = allFindings.filter((f) => f.verdict_critical)

const allGateFindings = triageResults.flatMap((r) => (r.findings || []).map((f) => ({ ...f, lens: r.probeKey })))
```

- [ ] **Step 5: Add the gate refute stage**

After `const standing = composeStanding(...)` (line 971), add:

```javascript
// The gate pass. Only dismissals are attacked. A wrong dismissal ships a live defect and nobody looks
// again, while a wrong endorsement wastes a day and is visible immediately, so spending skeptics
// symmetrically would be spending them on the cheap error.
const gateCritical = allGateFindings.filter((f) => f.gate_critical)
const gateOrdered = orderGateFindings(gateCritical)
const gateToRefute = gateOrdered.slice(0, MAX_GATE_REFUTED)
const gateDeferred = gateOrdered.slice(MAX_GATE_REFUTED)
if (gateDeferred.length) log(`${gateCritical.length} dismissals, gate budget is ${MAX_GATE_REFUTED}. Unattacked: ${gateDeferred.map((f) => `${f.lens}: ${f.claim}`).join(' | ')}`)

const gateJudged = await parallel(
  gateToRefute.map((f) => () =>
    parallel(
      SKEPTICS.map((angle) => () =>
        agent(
          `Try to REFUTE this dismissal. Somebody has argued that a defect is not worth fixing, and
you are here to find out whether that is wrong.

Dismissal: ${f.claim}
Status: ${f.status}
Locator: ${f.locator}
Reasoning: ${f.reasoning}

Your angle is "${angle}".
- "evidence": go to the locator and read it. Is the flag really off in production, or only off in the
  default the repo checks in? Does the guard really cover every path, or only the one that was read?
  Is the precondition really as hard to satisfy as claimed? Is the cost estimate counting work that a
  helper already does?
- "alternative-explanation": accept the evidence and find a reading that makes the dismissal wrong.
  Another caller that does reach the path. A second entry point. A tenant or plan tier the reasoning
  did not consider. An attacker who obtains the value a different way, or who does not need it at all.
  A cheaper fix that makes the cost objection moot.

**A dismissal is the expensive thing to get wrong**, so hold it to a higher standard than you would a
finding. "The flag is off" needs the production value, not the checked-in default. "Nobody can get
that token in sixty seconds" needs a reason nobody can, not an assertion that it sounds hard.

Refute it if you can. If your attack lands, set \`refuted: true\`. If it clearly does not, set
\`refuted: false\`. If you cannot tell, set \`undecided: true\` rather than guessing either way.

## Rules

${rules()}

## Context

${SKEPTIC_CONTEXT}`,
          { label: `gate-refute:${angle}`, phase: 'Refute', schema: REFUTE_SCHEMA },
        ),
      ),
    ).then((votes) => tallyVotes(f, votes, SKEPTICS.length)),
  ),
)

const gateTallied = gateJudged.filter(Boolean)
const gateKilled = gateTallied.filter((f) => !f.survived)
const gateContested = gateTallied.filter((f) => f.contested)
const gateTallyLost = gateToRefute.filter((f, i) => !gateJudged[i]).map((f) => tallyVotes(f, [], SKEPTICS.length))
const gateUnchallenged = [...gateTallied.filter((f) => f.panelShort), ...gateTallyLost]

if (gateKilled.length) log(`dismissals refuted and dropped: ${gateKilled.map((f) => f.claim).join(' | ')}`)
if (gateContested.length) log(`dismissals contested, kept and escalated: ${gateContested.map((f) => f.claim).join(' | ')}`)
if (gateUnchallenged.length) log(`dismissals kept without a full challenge: ${gateUnchallenged.map((f) => f.claim).join(' | ')}`)

const gateStanding = composeGateStanding(allGateFindings, gateToRefute, gateTallied, gateTallyLost, gateDeferred)

// Kept separate from coverageGaps rather than merged into it. The reader contract in "Reading the
// result" reads every existing refute-derived field with single-pass meaning, so a gate gap folded in
// there would be indistinguishable from a verdict gap.
const gateCoverage = {
  triage_agents_that_returned_nothing: missing.filter((m) => m.startsWith('triage:')),
  dismissals_never_attacked: gateDeferred.map((f) => f.claim),
  dismissals_kept_without_a_full_challenge: gateUnchallenged.map((f) => f.claim),
}
```

- [ ] **Step 6: Verify the script parses and the pure tests still pass**

Run: `zsh tests/work_on_validation_test.sh`

Expected: PASS. This step catches an unbalanced brace or a stray backtick in the new template literal. It does not exercise the dispatch, which has no test harness.

- [ ] **Step 7: Re-check the hook constraint**

Run: `grep -n "work-on" shared_config/.claude/skills/work-on/VALIDATION.md`

Expected: no hit between the fence markers.

- [ ] **Step 8: Commit**

```bash
git add shared_config/.claude/skills/work-on/VALIDATION.md
git commit -m "feat(claude,skill): dispatch the triage agents and attack their dismissals" -m "Triage findings are kept out of allFindings so they never reach selectForRefutation or composeStanding, both of which are keyed to verdict_critical. Only findings arguing against the work get skeptics, because a wrong dismissal ships a live defect while a wrong endorsement wastes a visible day.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: The verdict contract and the synthesizer

**Files:**
- Modify: `shared_config/.claude/skills/work-on/VALIDATION.md` (`VERDICT_SCHEMA` at ~614, the synthesizer prompt at ~1045)

**Interfaces:**
- Consumes: `gateStanding`, `gateCoverage`, `triageResults` from Task 3.
- Produces: `verdict.triage`, read by Task 9's Step 4 prose and Task 6's reader contract.

- [ ] **Step 1: Add the triage block to VERDICT_SCHEMA**

Inside `VERDICT_SCHEMA.properties`, after `side_effects` and before `needs_rebase`:

```javascript
    // Absent, not empty, when the ticket is neither a bug nor a security issue. An empty block and a
    // block nobody filled read identically, and only one of them means "we looked".
    triage: {
      type: 'object',
      required: ['worth_fixing', 'confidence', 'reasoning'],
      properties: {
        affected_now: {
          type: 'string',
          description:
            'What the probes measured, with the query. "unverifiable: no telemetry covers this path" is a real answer and is not the same as none.',
        },
        reachable: {
          type: 'string',
          description:
            'live, not-live-yet with what turns it on, or dead. Carry the path:line the reachability agent landed on.',
        },
        exploit_realism: {
          type: 'string',
          description:
            'Security tickets only. The attacker model and the precondition chain, including how long each precondition stays valid.',
        },
        loe: { type: 'string', description: 'The t-shirt size and the reason for it. Never hours.' },
        worth_fixing: { enum: ['yes', 'no', 'unclear'] },
        confidence: { enum: ['low', 'medium', 'high'] },
        reasoning: { type: 'string', description: 'Why, in two sentences, naming what carried the most weight.' },
        dismissals_that_held: {
          type: 'array',
          items: { type: 'string' },
          description: 'Arguments against the work that survived a skeptic panel. Only these may be described as having held.',
        },
        dismissals_refuted: {
          type: 'array',
          items: { type: 'string' },
          description: 'Arguments against the work a skeptic killed. Worth reporting, because a ticket nearly dismissed on a wrong basis is a useful thing for a human to see.',
        },
      },
    },
```

- [ ] **Step 2: Add the synthesizer's rules for it**

In the synthesize prompt, after the bullet beginning "Prefer "partial" over "valid"", add:

```
- **Fill \`triage\` when \`triage_agents_enabled\` is non-empty, and omit it entirely when that list is
  empty.** That list is the only thing that actually means this was not a bug or security ticket. Do
  not infer it from the findings being empty, because a bug ticket whose agents all died, and one
  whose agents each returned zero findings, both produce an empty findings list and neither means
  "not applicable". When the list is non-empty but there is nothing to reason from, emit the block
  with \`worth_fixing: 'unclear'\`, \`confidence: 'low'\`, and a \`reasoning\` naming the triage pass as
  having produced nothing. Omitting it there would report "we did not look" about a ticket where we
  looked and failed. Never fill it from the lenses' findings, which answer a different question.
- **You are the only agent holding both halves of the trade.** The reachability or exploit-realism
  agent never saw the cost, and the cost agent never saw the impact. Combining them is your job and
  neither of them may be blamed for the combination.
- **\`worth_fixing: 'no'\` needs evidence that reaches.** A query with a result, a flag state at a
  path:line, or a precondition chain read out of the code. Absent that, the answer is \`'unclear'\`,
  and \`confidence\` is \`low\`. "No telemetry covers this path" is never evidence that nobody is
  affected. Those are opposite conclusions and only one of them closes a ticket.
- **A dismissal a skeptic refuted is gone. Do not revive it.** Read \`refutationRan\` and
  \`whyUnrefuted\` on each triage finding the same way you read them on the lens findings.
  \`whyUnrefuted: 'argues-for-fixing'\` means it was never eligible for a skeptic because it argues the
  work matters, so nothing is missing and you must not discount it.
  \`whyUnrefuted: 'gate-budget-exhausted'\` means a dismissal went unattacked, which is a real gap.
  Name it in \`coverage_gaps\` and weight it lower.
- **\`worth_fixing\` of \`'no'\` or \`'unclear'\` produces exactly one blocking question, and it is a
  priority call.** Never phrase it as a request for a number. "How many users are affected" is a
  search somebody else already owns, and asking it is forbidden. Phrase it as the decision it is:
  state what reachability and cost found, then offer fixing it, deferring it, and running a named
  query to settle it. Set \`theme\` to "priority". Where a warehouse query would settle it, carry the
  exact SQL in \`sql\`, because a question classed non-blocking loses that field.
- **Suppress that question when the answer could not change the decision.** An S-sized fix gets done
  regardless of who is affected, so asking is spending the user's attention on a decision that is
  already made. Record it in \`dropped_questions\` in that field's second form, saying what made the
  question moot rather than where an answer could be found. There is no search location to give here,
  and inventing one to satisfy the field would be worse than the omission.
```

- [ ] **Step 3: Hand the synthesizer the triage data**

In the synthesize prompt, after the "## Findings still standing" block, add two sections:

```
## Triage findings (check refutationRan and whyUnrefuted on each)

${JSON.stringify(forPrompt(gateStanding), null, 2)}

## Triage coverage gaps. These bound what the triage block may claim.

${JSON.stringify(gateCoverage, null, 2)}
```

`forPrompt` strips the full ballots and keeps `voteSummary`, exactly as it does for the lens findings.

- [ ] **Step 4: Merge the gate gaps into the verdict's gap view**

Replace the `verdictGaps` line (around 1041):

```javascript
const verdictGaps = { ...coverageGaps, ...gateCoverage, completeness_critique_ran: !!critique }
```

The two objects have disjoint keys by construction, so nothing is overwritten, and the synthesizer's instruction to fill `coverage_gaps` from this block now covers both axes.

- [ ] **Step 5: Stop a dead triage agent being reported twice**

`coverageGaps.probes_that_returned_nothing` passes the unfiltered `missing`, which now contains `triage:*` entries, while `gateCoverage.triage_agents_that_returned_nothing` lists the same entries again. A reader told a probe returned nothing cannot then tell whether verdict evidence or gate evidence went missing, which is the same indistinguishability the two objects are kept separate to avoid. Filter the verdict-side field to the verdict-side probes:

```javascript
  probes_that_returned_nothing: missing.filter((m) => !m.startsWith('triage:')),
```

Leave `gateCoverage` alone. It already filters for the `triage:` prefix, so between them every dead probe is reported exactly once, on the axis it belongs to.

- [ ] **Step 5: Verify**

Run: `zsh tests/work_on_validation_test.sh`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add shared_config/.claude/skills/work-on/VALIDATION.md
git commit -m "feat(claude,skill): carry a triage block on the verdict" -m "The verdict enum stays four-valued. Worth-it is a separate axis, so a valid defect nobody should fix and a defect that never existed stay distinguishable. A no needs evidence that reaches, and no-telemetry is never evidence of no impact.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: The counts that are live prompt text

Four strings assert how many agents ran. Three are handed to an agent at runtime, so a stale count is not a documentation problem.

**Files:**
- Modify: `shared_config/.claude/skills/work-on/VALIDATION.md` (`meta.description` at 139, `meta.phases` at 140-145, the critic prompt at 990, and the critic's inputs)

- [ ] **Step 1: Update meta.description and meta.phases**

`meta.name` must stay exactly `'work-on-validate'`. The hook's carve-out depends on that string.

```javascript
export const meta = {
  name: 'work-on-validate',
  description: 'Validate a Jira ticket against HEAD, missing merged PRs, open PRs, and related tickets, and triage a bug or security ticket for impact and effort',
  phases: [
    { title: 'Investigate', detail: 'seven independent lenses, each pinned to one question, plus telemetry and triage' },
    { title: 'Refute', detail: 'skeptics attack every verdict-critical finding and every dismissal' },
    { title: 'Critique', detail: 'what did nobody look at' },
    { title: 'Synthesize', detail: 'verdict plus the questions that block it' },
  ],
}
```

- [ ] **Step 2: Fix the critic's premise and give it the triage findings**

The critic's whole job is finding what nobody looked at, so a wrong count degrades the one check designed to catch the omission this change could introduce. Replace its opening line and add to what it is told to check:

```
  `Seven code lenses validated a Jira ticket, alongside whichever telemetry probes and triage agents
this run enabled. Your job is to find what none of them looked at.
```

Then after the paragraph beginning "Check the telemetry block too", add:

```
Check the triage block too, when it is present. A dismissal nobody attacked, a code path nobody
established as live or dead, a precondition chain with a step nobody read, or a cost estimate that
never named a file are all gaps worth naming. If the triage block is absent, this ticket is neither a
bug nor a security issue and there is nothing to check here.
```

- [ ] **Step 3: Add the triage findings to the critic's input**

In the critic's "## Probe output" interpolation, add `triage: forPrompt(gateStanding)` to the object alongside `findings`, `questions`, and `telemetry`.

- [ ] **Step 3b: Two comments that went stale when the third probe kind landed**

Both describe the code as it was rather than as it stands, which this repo's comment rules forbid.

The block above the `probes` array opens "Code lenses and telemetry probes go out in ONE parallel batch". That array now has three kinds. Name the third, and note that the no-telemetry ban applies to the triage agents too, through `triageRules()`, for the same reason it applies to the lenses.

The comment introducing the gate stage explains why only dismissals are attacked. Add one sentence recording that the asymmetry lives in each skeptic's own bar and not in the aggregation: `tallyVotes` is shared with the verdict pass, so a dismissal still needs a full unanimous panel to die and a single refuting vote only marks it contested. Without that sentence a later reader assumes the tally is asymmetric too.

- [ ] **Step 4: Verify no stale count remains in the script**

Run: `grep -n "[Ss]even\|[Ss]ix\|nine sources" shared_config/.claude/skills/work-on/VALIDATION.md`

Expected: every remaining hit is either about the seven code lenses, which is still true, or about `MAX_REFUTED`'s nine-source floor, which is untouched because no triage finding ever carries `verdict_critical`. Read each hit and confirm it belongs to one of those two cases.

- [ ] **Step 5: Verify**

Run: `zsh tests/work_on_validation_test.sh`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add shared_config/.claude/skills/work-on/VALIDATION.md
git commit -m "fix(claude,skill): stop the agent-facing counts going stale" -m "Three of the four counts are handed to an agent at runtime, and the completeness critic's is the premise of the one pass whose job is spotting an omission. The seven-lens count stays true because triage is a separate probe kind.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: The reader contract

The main thread reads the workflow's return value against a documented checklist. A field with no bullet is a field nobody checks.

**Files:**
- Modify: `shared_config/.claude/skills/work-on/VALIDATION.md` (the `return {}` object at ~1140, "Reading the result" at ~1157, "Afterwards" at ~1205, and the brief-companion count at 673)

- [ ] **Step 1: Add the gate fields to the return object**

Do not touch the line `return {`, which is a sed anchor. Add inside it, after `coverage_gaps`:

```javascript
  triage: gateStanding,
  triage_loe: triageResults.map((t) => ({ agent: t.probeKey, loe: t.loe })).filter((t) => t.loe),
  dismissals_refuted: gateKilled.map((f) => ({ claim: f.claim, votes: f.votes })),
  dismissals_contested: gateContested.map((f) => ({ claim: f.claim, votes: f.votes })),
  dismissals_unattacked: gateDeferred.map((f) => ({ claim: f.claim, lens: f.lens })),
  gate_coverage: gateCoverage,
```

- [ ] **Step 2: Add the reader bullets**

In "Reading the result", after the `refuted` bullet, add three bullets matching the existing shape:

```markdown
- **`dismissals_refuted`** non-empty means a skeptic killed an argument that this work is not worth
  doing. Read it even though those findings are dropped. A ticket that was nearly dismissed on a
  basis that turned out to be wrong is worth a line to the user in Step 4, because the near miss is
  itself the finding.
- **`dismissals_contested`** non-empty means the skeptics split on whether the work is worth doing.
  That belongs in front of the user as a priority call, never settled by whichever agent spoke last.
- **`dismissals_unattacked`** and **`gate_coverage`** together bound what the `triage` block may
  claim. An unattacked dismissal is load-bearing and unchallenged, so a `worth_fixing` of `no`
  resting on one is a `no` nobody tested. Say so in Step 4 rather than presenting it as settled.
- **`triage`** carries the findings behind the verdict's own `triage` block, each with its
  `refutationRan` and `whyUnrefuted`. Read it against `verdict.triage` to catch a synthesizer that
  claimed more confidence than its findings support. A dismissal marked
  `whyUnrefuted: 'gate-budget-exhausted'` under a `worth_fixing` of `no` is the specific thing to
  look for, because that is a recommendation not to do the work resting on an argument nothing
  attacked. Present it to the user as exactly that.
- **`triage_loe`** carries each cost agent's size and the reason for it. A size whose reason names no
  file is an estimate rather than evidence, so present it to the user as an estimate, under the same
  rule that governs any unlocated claim in this skill.
```

Six fields go into the return object and every one of them needs a bullet here. Three of the six
carry the refutation outcome and three carry the triage output itself, and a field with no bullet is
a field nobody checks, which is how four pre-existing properties in this schema ended up with no
named reader.

- [ ] **Step 3: Add the triage re-run instructions to "Afterwards"**

After the four-item list for re-running a single lens, add:

```markdown
Re-running a triage agent works the same way and needs the same four pieces, with `triageRules()` in
place of `rules()` and `TRIAGE_SCHEMA` in place of `FINDING_SCHEMA`. Its brief is the whole of what it
knows, so dropping the rules costs it the no-telemetry ban and the `gate_critical` calibration, and
dropping the schema costs it the distinction between a finding that argues against the work and one
that does not.

**Re-run the cost agent without the impact answer, and the impact agent without the cost.** The
blindness is the reason those two agents are separate, and an answer that arrived from the user does
not license collapsing them. If the user's answer settles the impact, the cost estimate you already
have is still the right one to combine it with.
```

- [ ] **Step 4: Fix the companion count**

`VALIDATION.md:673` says "the inline path has to supply all four too" while `SKILL.md:133` says "three more things" and "supply all three itself". Both name the same three items. Change the `VALIDATION.md` copy to `all three too`.

- [ ] **Step 5: Verify**

Run: `zsh tests/work_on_validation_test.sh`

Expected: PASS.

Run: `grep -n "all four\|all three" shared_config/.claude/skills/work-on/VALIDATION.md shared_config/.claude/skills/work-on/SKILL.md`

Expected: the two files now agree.

- [ ] **Step 6: Commit**

```bash
git add shared_config/.claude/skills/work-on/VALIDATION.md
git commit -m "feat(claude,skill): document the gate fields in the reader contract" -m "A returned field with no bullet in Reading the result is a field nobody checks, which is how four existing properties ended up with no named reader. Also settles a three-versus-four disagreement between the two files about what a brief needs alongside it.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Step 1 records the kind and switches the probes on

This is the task that answers "are people currently affected". It adds no agent. It widens a trigger.

**Files:**
- Modify: `shared_config/.claude/skills/work-on/SKILL.md` (Step 1's telemetry paragraph at 146-151, and the `args` documentation in `VALIDATION.md` at 33-129)

- [ ] **Step 1: Add the kind decision to Step 1**

In `SKILL.md`, immediately before the paragraph beginning "**Decide which telemetry probes to launch.**", insert:

```markdown
**Decide what kind of ticket this is.** You have read it and nothing downstream has, so this call is
yours, exactly like the telemetry flags below. The ticket fetch already used `fields: ["*all"]`, so
`issuetype`, `labels`, `components`, and `priority` are in the payload you have. Read those and the
description, then record `{ kind, kind_because }` where `kind` is `bug`, `security`, or `other`.

Judge it, do not read it off one field. Bugs get filed as Tasks and Stories constantly, and a scanner
finding pasted into a Story with no label is exactly the ticket where this matters most. A `security`
kind is anything about authorization, authentication, tokens, secrets, tenant isolation, or data
exposure, however it was filed.

**A ticket that is both takes `security`.** An auth bypass that also double-charges is a security
ticket. `exploit-realism` is the assessment nothing else supplies, and the two errors are not
symmetric. A security issue labelled `bug` loses the attacker model silently and still returns a
populated block that looks complete, because `fix-cost` runs either way. A bug labelled `security`
gets `exploit-realism`, which reports that no attacker model applies, and that is a cheap and visible
wrong answer rather than a missing one.

**In doubt between a bug and `other`, record `bug`.** Two extra agents is cheap. A skipped assessment
is silent, and silence here reads as "nobody thought this was worth checking". This tiebreaker settles
that question only. It does not settle a bug-versus-security doubt, which the rule above decides.

**Say which kind you chose in the Step 2 handoff**, with the reason. An `other` on an obvious bug is
the failure mode, and it is invisible unless the choice is stated.
```

- [ ] **Step 2: Widen the telemetry trigger**

The existing paragraph ends with "Both off for a refactor or an internal cleanup, which is the common case." Append to it:

```markdown
**A `bug` or `security` kind turns Datadog on regardless of what the ticket asserts**, and turns
Amplitude on too when the path is user-facing. This is the one case where the flags are not set from
the ticket's own claims, because the triage pass makes a production claim the ticket never made. It
asks whether anybody is actually affected, and that question has exactly one honest source. Without
the flags the triage block comes back unmeasured on the tickets it exists for, and an unmeasured
impact reads as a small one.
```

- [ ] **Step 3: Document the new args fields**

In `VALIDATION.md`'s args example, add after `"telemetry"`:

```json
  "kind": "bug",
  "kind_because": "issuetype=Task but the description reports a double charge on retry"
```

And after the paragraph explaining `telemetry`, add:

```markdown
`kind` decides whether the triage agents run and which benefit agent is chosen. `bug` runs
`reachability` and `fix-cost`. `security` runs `exploit-realism` and `fix-cost`. `other` runs neither
and the verdict's `triage` block is absent, which is not the same as empty. Set it from your own read
of the ticket rather than from `issuetype` alone. A ticket that is both a bug and a security issue
takes `security`, and in doubt between a bug and `other` set `bug`, because two extra agents is
cheaper than an assessment nobody notices was skipped.

`kind_because` is an audit trail and nothing reads it programmatically. It rides in `args` so every
agent can see the classification call was made and on what basis, and it is what Step 1 states in its
handoff. Put the evidence that decided the call in it, especially when that evidence overrode the
issue type.
```

- [ ] **Step 4: Verify**

Run: `grep -n "kind_because" shared_config/.claude/skills/work-on/SKILL.md shared_config/.claude/skills/work-on/VALIDATION.md`

Expected: at least one hit in each file. The two documents describe the same field and both are read by a running agent.

- [ ] **Step 5: Commit**

```bash
git add shared_config/.claude/skills/work-on/SKILL.md shared_config/.claude/skills/work-on/VALIDATION.md
git commit -m "feat(claude,skill): record the ticket kind and switch the probes on for it" -m "The measurement half of impact is already routed to the Datadog and Amplitude probes. What was missing is the trigger. A triage run makes a production claim the ticket never made, so the flags cannot be set from the ticket's claims alone.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: Step 2 runs triage on both paths

The inline path is the common case for a small bug. An addition that lands only on the workflow path lands on the wrong half.

**Files:**
- Modify: `shared_config/.claude/skills/work-on/SKILL.md` (Step 2, lines 398 to 457)

- [ ] **Step 1: Add the triage obligation to the inline path**

After the paragraph beginning "**Telemetry still goes to subagents on this path.**", insert:

```markdown
**Triage goes to subagents on this path too.** If Step 1 recorded a `bug` or `security` kind,
dispatch the triage agents in one message, alongside the telemetry probes, using the briefs from
[VALIDATION.md](VALIDATION.md)'s `TRIAGE` array. Dispatch the entries whose `enabled` matches the
kind you recorded in Step 1, which is two of the three. The inline path has no `args`, so nothing
evaluates those expressions for you, and dispatching all three hands a security ticket the
`reachability` agent that Step 1 forbids it. A brief is not the whole prompt: pair each one with
`triageRules()`, the context block including its `<<<UNTRUSTED_INPUT_BEGIN>>>` and
`<<<UNTRUSTED_INPUT_END>>>` markers, and `TRIAGE_SCHEMA`. Dropping the schema is the worst of the
three, because `gate_critical` lives there and without it nothing separates an argument against the
work from an argument for it.

**Keep the cost agent blind to the impact answer, and the impact agent blind to the cost.** On this
path you are holding both, which makes it your job not to leak either. Dispatching them in one message
is what enforces it.

**You have no skeptics here, so say so.** The workflow path attacks every dismissal before it reaches
a verdict. Inline, nothing does. A `worth_fixing` of `no` from this path is a dismissal nobody tested,
and it goes to Step 4 marked that way, as a coverage gap. Presenting it as settled would be the silent
degradation this skill refuses everywhere else.
```

- [ ] **Step 2: Update the same-shape sentence**

`SKILL.md:456` enumerates the output as three things and Step 4 now reads a fourth. Replace it:

```markdown
Either way the output is the same shape and Step 4 reads it identically: a verdict, evidence with
locators, the questions investigation could not settle, and on a bug or security ticket the triage
block, with whatever the path could not establish named as a coverage gap.
```

- [ ] **Step 3: Note the count in the workflow paragraph**

The paragraph beginning "**Run the workflow when the surface is real.**" describes the batch. After "Seven lenses plus whichever telemetry probes Step 1 enabled all go out in one parallel batch", add "plus the triage agents on a bug or security ticket".

- [ ] **Step 4: Verify**

Run: `grep -n "TRIAGE_SCHEMA\|triageRules" shared_config/.claude/skills/work-on/SKILL.md`

Expected: both appear in the inline-path paragraph. The inline path has no schema of its own, so naming them here is the only thing that makes it reproducible.

- [ ] **Step 5: Commit**

```bash
git add shared_config/.claude/skills/work-on/SKILL.md
git commit -m "feat(claude,skill): run triage on the inline path too" -m "The inline path is the common case for a small bug, and it has no lens enumeration to extend, so an addition lands workflow-only by default and silently. It also has no skeptics, so a dismissal from that path reports itself as untested rather than claiming a parity it does not have.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 9: Steps 3, 4, and 5

**Files:**
- Modify: `shared_config/.claude/skills/work-on/SKILL.md` (Step 3 at 459-492, Step 4 at 493-575, Step 5's comment structure at 630-648)

- [ ] **Step 1: Add the priority-call note to Step 3**

After the paragraph beginning "Ask conversationally.", insert:

```markdown
**A triage question is a priority call, not a request for a number.** "How many users are affected"
is a search somebody else owns and this skill forbids asking it. What reaches the user is the
decision: here is what is reachable, here is what it costs, do we fix it, defer it, or run a query
first. If it carries SQL, it goes down the warehouse path below like any other query, and the
priority call is what remains after the query comes back.
```

- [ ] **Step 2: Add the triage presentation to Step 4**

After the **Valid** paragraph and before **Superseded**, insert:

```markdown
**The triage block, on a bug or security ticket.** Present it before the options, whatever the verdict
is. Four lines: whether anybody is affected now and how that was measured, whether the path is live
and at which `path:line`, what a fix costs and why, and the recommendation with its confidence. On a
security ticket the second line is the precondition chain instead, including how long each
precondition stays valid, because a value that expires in a minute and a sequential id are different
tickets.

Name what was not established. An unattacked dismissal, an unreachable telemetry source, or a cost
estimate that never named a file all bound what this block can claim, and every one of them is in the
workflow's `gate_coverage`. A confident recommendation over thin evidence is the worst thing this pass
can produce, and it is worse than no recommendation because it looks like one.

**This is not a verdict and it does not stop the run.** Nothing here closes a ticket, and nothing here
writes. A defect that reproduces is still a defect that reproduces, and whether it is worth the money
is the user's call and not this skill's.

**The priority call is asked once, at Step 3, and never again here.** A `worth_fixing` of `no` or
`unclear` reaches Step 3 as a blocking question, so by the time you are here the answer is in hand.
Present the block together with that answer and act on it. The a/b/c list below belongs to the verdict
gate, not to this. Map the answer instead. Fixing it continues to Step 5 on the ticket as it stands.
Deferring it ends the run the way option (a) does, so post the comment carrying the impact and cost
evidence, and the stash rule's (a) branch applies. A query that has to run first resolves through the
warehouse handoff, and its answer then decides between the other two.

**When Step 3 asked nothing, there is nothing to decide here either.** The synthesizer suppresses the
question when the answer could not change the decision, such as an S-sized fix that gets done
regardless of who is affected. Present the block and continue.

**If `worth_fixing` is `yes`, say so in one line and move on.** The block still gets presented,
because the cost is worth knowing before Step 6, but a recommendation to proceed does not need a
decision from anybody.
```

- [ ] **Step 3: Extend the options for the triage case**

The three options are written for a dead ticket. Add a sentence after the option list:

```markdown
On a triage priority call the same three options carry different content: (a) posts a comment with
the impact and cost evidence so the user can deprioritize the ticket themselves, (b) narrows it to
whatever part is worth doing, and (c) proceeds because the user disagrees with the assessment. The
stash rule below reads these exactly as it already does, since (a) or the user stopping there ends
the run and (b) or (c) continues it.
```

- [ ] **Step 4: Add triage facts to the Step 5 comment structure**

After the sample comment block, add:

````markdown
On a bug or security ticket the trail gains the triage facts, in the same shape as every other line
here, each with its locator:

```
- path is live at src/billing/retry.ts:88, behind FLAG_RETRY_V2, on in prod since 2026-07-02
- the error fires 340 times a day, from the Datadog query in this run
- a fix is size M, three files plus a backfill over billing_invoice, which has no index on the filter column
```

**The recommendation itself is not published.** It has no locator of any accepted kind, and nothing
previews this write, so a judgement about somebody else's ticket would ship with no human having read
it. The facts belong in the trail because the next reader would otherwise redo them. The conclusion
belongs in the conversation at Step 4, where a human is present to disagree with it.

**A triage number nobody ran is a `TODO(user):` line**, exactly like any other unrun query. Do not
soften it into prose instead. `writing-work-docs` forbids hedging qualifiers, so there is no register
between a locator and a TODO line, and reaching for one produces a claim that reads as measured.

**Never a table.** A triage assessment is the natural table and a table is the single most common way
a ticket arrives mangled. One bullet per row, per [JIRA-FORMAT.md](JIRA-FORMAT.md).
````

- [ ] **Step 5: Verify**

Run: `grep -n "worth_fixing" shared_config/.claude/skills/work-on/SKILL.md`

Expected: hits in Step 4 only. Step 5 deliberately never branches on it, because Step 5 publishes facts rather than the conclusion.

- [ ] **Step 6: Commit**

```bash
git add shared_config/.claude/skills/work-on/SKILL.md
git commit -m "feat(claude,skill): present triage at the gate and publish only its facts" -m "The priority call reuses Step 4's three options rather than inventing a fourth verdict, so none of the three stop-the-run copies and none of the stash branches have to widen. The recommendation stays in the conversation because it has no locator and nothing previews the ticket write.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 10: The four remaining prose sites

**Files:**
- Modify: `shared_config/.claude/skills/work-on/SKILL.md` (frontmatter 3-15, the question table at 86-88, Step 8's argument at 809, Constraints at 968-1035)

- [ ] **Step 1: Widen the frontmatter description**

This is the text that decides whether the skill is invoked at all, and it currently enumerates four validation axes as a closed list. In the `description`, after "the related Jira tickets that may already duplicate it or have carved scope out of it.", add:

```
  On a bug or a security ticket it also establishes the real impact and the effort to fix, so a
  fix that is not worth doing gets caught before any code is written.
```

Then in the trigger list, after "is this ticket still valid", add `"is this bug worth fixing"` and `"how bad is this security issue really"`.

- [ ] **Step 2: Add the row to the question table**

The table's three measurement rows already route impact. Adding a fourth row makes the new route explicit rather than a second contradictory one. After the "How many rows, accounts, or records are affected" row:

```markdown
| Is this bug or security issue worth fixing at all | The triage agents in Step 2, then a priority call to the user |
```

The `never questions for the user` list below stays exactly as it is. "How many users are affected?" remains banned, and the triage question is a priority call, which the list beneath it already permits.

- [ ] **Step 3: Extend Step 8's leaf argument**

`SKILL.md:809` reads "Its seven lenses are leaf readers that spawn nothing, so they lose nothing". Change to:

```markdown
Its seven lenses, its telemetry probes, and its triage agents are all leaf readers that spawn
nothing, so they lose nothing
```

- [ ] **Step 4: Add two constraints**

In the Constraints list, after the "Never invent" bullet:

```markdown
- **A triage recommendation never gets published.** The facts behind it go into the Jira comment with
  their locators. The recommendation itself stays in the conversation at Step 4. It has no locator of
  any accepted kind, nothing previews the ticket write, and a judgement about somebody else's ticket
  is not this skill's to put on the record.
- **`worth_fixing: 'no'` requires evidence that reaches.** A query with a result, a flag state at a
  `file:line`, or a precondition chain read out of the code. An unmeasured impact is `unclear`, never
  `no`. "No telemetry covers this path" and "nobody is affected" are opposite conclusions and only one
  of them stops work.
```

- [ ] **Step 5: Verify the frontmatter still parses**

Run: `head -20 shared_config/.claude/skills/work-on/SKILL.md`

Expected: the YAML block still opens and closes with `---`, and the `description:` block scalar keeps its two-space indentation on every continuation line. A broken indent here makes the whole skill unloadable, and nothing in the test suite catches it.

- [ ] **Step 6: Commit**

```bash
git add shared_config/.claude/skills/work-on/SKILL.md
git commit -m "feat(claude,skill): widen the routing text and the constraints for triage" -m "The frontmatter enumerates the validation axes as a closed list and is what decides whether the skill fires, so a narrower description than the skill's behavior is a routing bug rather than stale prose. The question table gains an explicit row so the new route is visible beside the three that already handle measurement.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 11: The agent roster in AGENTS.md

This file becomes `~/.claude/AGENTS.md` and is loaded into every session. Its roster is not documentation. The deny-hook carve-out that lets the validation workflow run at all is justified on the claim that every agent in it is a leaf.

**Files:**
- Modify: `shared_config/.claude/AGENTS.md` (the paragraph naming the roster, around line 81)

- [ ] **Step 1: Add the triage agents to the roster**

The sentence currently reads "That phase drives a workflow whose agents are the seven lenses, the telemetry probes, the skeptics, the completeness critic, and one synthesizer, and every one of those is a leaf that reads and reports." Replace with:

```markdown
That phase drives a workflow whose agents are the seven lenses, the telemetry probes, the triage
agents on a bug or security ticket, the skeptics, the completeness critic, and one synthesizer, and
every one of those is a leaf that reads and reports.
```

- [ ] **Step 2: Verify the invariant still holds**

Run: `grep -n "agentType\|Agent(" shared_config/.claude/skills/work-on/VALIDATION.md`

Expected: no hit inside the fence that would give a triage agent a custom `agentType` or let one spawn a sub-agent. The three new agents use the default workflow subagent and only read, so the leaf claim in the sentence above stays true and no `DENIED_AGENT_TYPES` entry is needed.

- [ ] **Step 3: Commit**

```bash
git add shared_config/.claude/AGENTS.md
git commit -m "docs(claude): add the triage agents to the validation roster" -m "The roster is what the deny-hook carve-out is justified on, since the argument is that every agent in that workflow is a leaf. A roster that omits three agents makes the justification unverifiable against the thing it describes.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 12: Cross-file assertions for the counts

Eleven of this change's edits are in `SKILL.md`, which has no automated coverage of any kind. The pre-commit hook names no markdown and CI runs `zsh -n` over three shell files, so a half-finished edit ships green. This task converts three of the duplications into checked ones.

**Files:**
- Modify: `tests/work_on_validation_test.sh`

- [ ] **Step 1: Write the failing assertions**

The existing test reads only `VALIDATION.md`. Add a second path near the top, beside the existing `SKILL=` line:

```bash
SKILL_MD="$SCRIPT_DIR/shared_config/.claude/skills/work-on/SKILL.md"
```

Then before the final `echo`, add:

```bash
# --- cross-file invariants ---
#
# These three facts live in two files each and nothing else checks them.
#
# A zero-hit grep and a passing assertion look identical, so each check below has to be one that can
# actually fail. The three patterns face that risk differently.
#
# The agent-name patterns are single tokens with no space, so no line wrap can split them.
# The lens-count pattern matches indentation in the JavaScript inside the fence, which is not
# hand-wrapped prose, so the same applies.
# The companion-count pattern is the one multi-word phrase in wrapped prose, and it is asserted
# positively for that reason. A wrap that splits the phrase makes it fail rather than pass, and a false
# failure is the safe direction because someone looks. A zero-hit check on the same phrase could never
# fail at all.

# Every triage agent named in the script must also be named in the inline-path section of SKILL.md,
# or the inline path silently skips it while the workflow path runs it.
for agent_key in reachability exploit-realism fix-cost; do
  if ! grep -q "$agent_key" "$SKILL_MD"; then
    echo "triage agent '$agent_key' is dispatched by VALIDATION.md but never named in SKILL.md" >&2
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
# that form cannot fail: both files are hand-wrapped, so a future rewrap that splits "all four" across
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
```

- [ ] **Step 2: Run to verify the lens count assertion is meaningful**

Run: `zsh tests/work_on_validation_test.sh`

Expected: PASS with `lens_count_is_seven` equal to 7.

Then prove the assertion can fail. Temporarily add a `key: 'temp',` line at the same indentation inside `LENSES`, re-run, and confirm it fails with 8. Remove the temporary line. An assertion never observed failing is an assertion that may be matching nothing.

- [ ] **Step 3: Prove the triage-agent check can fail**

Temporarily replace the string `reachability` in `SKILL.md` with `REDACTED`, re-run, and confirm the loop reports the missing name. Restore it. This is the negative control for the hand-wrapping trap.

Two things about this control, both of which an earlier draft got wrong and which are worth stating because they generalize.

**Break it in `SKILL.md`, not in the `TRIAGE` array.** The loop iterates over hardcoded literals and greps `SKILL.md`. Editing `VALIDATION.md` would leave the check's inputs untouched and prove nothing.

**Do not "rename" by appending a suffix.** `grep -q reachability` matches a file containing `reachabilityx`, because grep matches substrings. A control that adds characters to the searched-for name cannot make a substring search fail, so it would pass and appear to validate the check. Replace the name with a string that does not contain it.

- [ ] **Step 4: Run the full suite**

Run: `zsh tests/work_on_validation_test.sh`

Expected: PASS, printing `work_on_validation_test: ok`.

- [ ] **Step 5: Commit**

```bash
git add tests/work_on_validation_test.sh
git commit -m "test(claude,skill): check the counts that live in two files" -m "SKILL.md has no automated coverage, so a partial edit across the two skill files ships green. Every pattern uses \\s+ per space because the prose is hand-wrapped, and each check counts matches rather than testing absence, since a zero-hit grep and a passing assertion are indistinguishable.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Verification after the last task

- [ ] Run `zsh tests/work_on_validation_test.sh`. Expect `work_on_validation_test: ok`.
- [ ] Run `zsh tests/deny_review_in_workflow_test.sh`. Expect it to pass unchanged. Nothing in this plan touches the hook or its denied lists, so a failure here means a skill name leaked into a place the hook scans.
- [ ] Run `grep -c '^```javascript$' shared_config/.claude/skills/work-on/VALIDATION.md`. Expect exactly `1`.
- [ ] Run `grep -n "work-on" shared_config/.claude/skills/work-on/VALIDATION.md` and confirm no hit falls inside the fenced block.
- [ ] Read `SKILL.md` Steps 1 through 5 end to end, in one pass, as a reader who has not seen this plan. The prose edits touch five steps and the only check on whether they read as one document is a person reading them as one document.

## What this plan does not do

- **No mitigation is implemented for a triage agent that returns nothing.** `probes_that_returned_nothing` already covers it and `gate_coverage` surfaces it, so the gap is reported. Re-running it is the reader's call, exactly as it is for a lens.
- **The question-suppression rule lives in a prompt, not a schema.** `QUESTION_ITEM` has no analogue to `verdict_critical`, so "suppress when the answer cannot change the decision" is a synthesizer instruction and nothing enforces it. The spec records this as an open item.
- **No test exercises the dispatch, the prompts, or the schemas.** The harness parses the script and runs its pure functions. Everything else in the workflow is verified by reading.
