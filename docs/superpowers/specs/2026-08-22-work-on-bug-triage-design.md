# Impact and effort triage in the validation phase

Design for adding a worth-it assessment to the ticket-validation phase of the `work-on` skill, for
bug and security tickets only.

Date: 2026-08-22.
Status: approved in brainstorming, not yet implemented.

## Problem

The validation phase establishes whether a ticket is still real. It answers "does this reproduce at
HEAD", "did a merged commit fix it", "does an open PR cover it", and "does another ticket own it".
It never asks whether the problem is worth fixing.

For a bug or a security ticket those are different questions, and the second one decides whether the
rest of the run should happen at all. The four things a reader actually wants:

1. Is anyone affected right now?
2. If the code path is not live yet, will anyone be affected when it goes live?
3. Is this a real defect, or a just-in-case precaution?
4. What are the realistic odds of it happening? For a security issue, could a real attacker act on
   it? A vulnerability that needs a token with a sixty second lifetime is a different proposition
   from one that needs a sequential id.

Tickets rarely carry any of this. The cost of the fix is also absent, and the two numbers are only
useful together.

## What already exists

Three of the four questions above are already routed, and not to a new agent.

`SKILL.md:86-88` maps them:

| Question shape | Where the answer already is |
|---|---|
| Does this really happen in production, how often | A Datadog probe subagent |
| Do users actually hit this path, how many | An Amplitude probe subagent |
| How many rows, accounts, or records are affected | A query skill if one works, else the user |

So the measurement half of this design needs a trigger rather than a lens. What no existing agent
answers is whether the code path is live, whether an attacker could realistically act, and what the
fix costs.

## Decisions

1. The verdict enum stays four-valued. `valid`, `invalid`, `superseded`, `partial` are answers to
   "is the problem real". Worth-it is a separate axis and gets a separate block on the verdict
   object. Folding it into the enum would make `valid` mean two things at once, and Step 4's
   per-verdict evidence rules would stop mapping cleanly.
2. Step 1 records the ticket kind and says so out loud. On `bug` or `security` it also forces the
   telemetry probe flags on.
3. Three new agents, conditionally enabled, at most two per run. They are a third probe kind and not
   `LENSES` entries.
4. The benefit agents and the cost agent are blind to each other.
5. Dismissals get attacked by skeptics from their own small budget. Findings arguing for a fix do
   not.
6. A recommendation of "not worth fixing" requires evidence that reaches. Otherwise the answer is
   "unclear".
7. "Unclear" becomes a blocking question, phrased as a priority call.
8. Step 4 presents the block and asks. It is not a new verdict and it does not stop the run on its
   own initiative.

## Design

### Step 1: kind detection and the telemetry trigger

Step 1 already fetches the ticket with `fields: ["*all"]`, so `issuetype`, `labels`, `components`,
and `priority` are in hand at no extra cost. It reads those and the description, then records:

```json
"kind": "bug" | "security" | "other",
"kind_because": "label=wiz-finding; description describes an IDOR on /invoices/:id"
```

In doubt it records `bug`. Two extra agents is cheap. A skipped assessment is silent, which is the
expensive direction.

It states the kind in the Step 2 handoff, so an `other` on an obvious bug is visible rather than
quiet.

The telemetry flags widen. Today they are set from what the ticket asserts, per `SKILL.md:146-151`.
A triage run makes the production claim itself, even when the ticket does not, so:

- `kind` of `bug` or `security` sets `datadog: true`.
- The same, plus a user-facing code path, sets `amplitude: true`.

This single change answers questions 1 and 2 above using machinery that already has briefs, a
schema, an `available: false` coverage-gap path, and the never-paste-raw-rows rule.

### The triage pass

Three agents. `kind` of `bug` runs `reachability` and `fix-cost`. `kind` of `security` runs
`exploit-realism` and `fix-cost`. `kind` of `other` runs neither, and the block is absent rather
than empty, because those are different things to a reader.

**`reachability`.** Is the path live at HEAD right now? Land it on a `path:line`. Feature flag
state, route registration, whether callers exist, whether it is dead code. If it is not live, name
what turns it on. A not-yet-live path is a schedule, not an absence. What preconditions must
coincide for the defect to fire? Is there evidence it has ever actually happened, or was it reasoned
out from reading the code?

**`exploit-realism`.** Which attacker model does this need? Unauthenticated internet, any logged-in
user, another tenant, or an insider. Then the precondition chain, meaning everything the attacker
must hold at the same time. For each one, how they would obtain it and how long it stays valid. Is
the secret enumerable or leaked anywhere, such as a sequential id, an error message, a log, or a
referrer header? What is the payoff? What existing control already blocks it? An existing control is
the finding that closes this fastest.

**`fix-cost`.** The files and the lines. The test burden, including whether the suite already covers
the path. Any migration, backfill, or data change, and whether it reverses. Rollout needs, such as a
flag or coordination with another team. The risk of the fix itself. A t-shirt size with its reason,
not a count of hours.

**Blindness.** `fix-cost` is never told whether anyone is affected. The benefit agents are never
told what the fix costs. `fix-cost` overlaps `solution-feasibility` and `blast-radius` on purpose,
and it reads the tree itself rather than being handed their conclusions, because being handed a
conclusion is the anchoring the whole phase exists to prevent.

**Why they are not lenses.** A code lens may not query Datadog or Amplitude. `SKILL.md:113` says
"Never query either one from the main thread, and never from a code lens", and `rules()` at
`VALIDATION.md:772` repeats it inside every lens prompt with the handoff to use instead. A new
telemetry probe is also unavailable, because `TELEMETRY_SCHEMA.source` at `VALIDATION.md:568` is a
closed two-value enum and a probe returning a third value fails its own schema. A schema failure
reads as an empty result, which is the silent emptiness `available: false` exists to prevent.

Beyond that, adding entries to `LENSES` breaks nine things:

- wholesale `LENSES.map` dispatch with no enabled filter, `VALIDATION.md:825`
- `LENSES.length` in the run log, `VALIDATION.md:830`
- `meta.phases[0].detail`, which reads "seven independent lenses", `VALIDATION.md:141`
- the completeness critic's own prompt premise, "Seven lenses validated a Jira ticket",
  `VALIDATION.md:990`
- the six-and-the-seventh anchoring argument, duplicated in both files, `SKILL.md:403`
- `REFUTE_PRIORITY`'s ordering comment, "The two scope lenses come last", `VALIDATION.md:235`
- `MAX_REFUTED`'s floor argument, which derives six from nine sources, `VALIDATION.md:151`, and
  which a test pins at `tests/work_on_validation_test.sh:136-153`
- Step 8's leaf-reader safety argument, `SKILL.md:809`
- the agent roster and its leaf invariant, `shared_config/.claude/AGENTS.md:81`

A `TRIAGE` array joining `kind: 'lens' | 'telemetry'` in the existing `probes` array costs one
filter and breaks none of them. "Seven lenses" stays true.

### Refutation of dismissals

A false "not worth fixing" ships a live defect and nobody revisits it. A false "worth fixing" wastes
a day and is immediately visible. So only findings that argue against fixing get attacked.

Gate findings never enter `allFindings`, never carry `verdict_critical`, and never reach
`composeStanding`. `MAX_REFUTED` and `REFUTE_PRIORITY` are untouched, which keeps the test's floor
case green. Three reasons the existing machinery cannot simply be called twice:

- `composeStanding`'s label ladder tests `!f.verdict_critical` first, at `VALIDATION.md:356`, so a
  gate finding short-circuits to `not-verdict-critical`. The synthesizer prompt at
  `VALIDATION.md:1065` then reads that label as "Nothing is missing... do NOT discount them as
  unchallenged". A real gap would arrive immunised against being treated as one.
- `accountedFor` is built from one pass's arrays by object identity, at `VALIDATION.md:341`, and the
  function is called once at `VALIDATION.md:971`. A second pass either double-lists a finding or
  revives one whose panel killed it.
- `selectForRefutation` mutates the objects it is handed, at `VALIDATION.md:282` and `:298`, despite
  the "Pure" comment above it. Overlapping input double-appends `also_reported_by`.

`tallyVotes` is genuinely panel-size agnostic and is reused unchanged.

So the gate stage keeps its own array, sorts by priority over at most two sources (a round-robin is
pointless at two), reuses `tallyVotes`, and gets its own compose function and its own return fields.
Its cap, `MAX_GATE_REFUTED`, is 2, meaning one slot per triage agent that can run.
Separate return fields matter because the five existing refute-derived fields are read with
single-pass meaning at `VALIDATION.md:1178`, and merging two passes into them would make a verdict
gap and a gate gap indistinguishable.

Two mechanical constraints on the new compose function. It must be a top-level `function name(`
whose body closes on a column-zero `}`, declared before anything that touches injected globals, or
the `awk` extraction at `tests/work_on_validation_test.sh:56-60` cannot see it and it ships untested
while the suite passes. And nothing may disturb the `^export const meta = {` and `^return {$`
anchors that the parse check rewrites.

### The contract

A `triage` block on the verdict object, required only when `kind` is `bug` or `security`:

```
affected_now      what the probes measured, or unverifiable with the number that was wanted
reachable         live, not-live-yet with what turns it on, or dead, each with a path:line
exploit_realism   security only. The attacker model and the precondition chain
loe               t-shirt size with its reason
worth_fixing      yes | no | unclear
confidence        derived from whether the evidence reached
```

The synthesizer assembles this block. The triage agents return findings and are blind to each other,
so nothing below the synthesizer holds both halves of the trade.

Every field carries a locator or is marked unverifiable. This is not a carve-out from the
locator rule at `SKILL.md:989`, it is compliance with it. A flag state is a `path:line`. A probe
number is a query with its result. A t-shirt size is the files it counted.

`worth_fixing: 'no'` is only available when the evidence reaches. Thin evidence returns `unclear`.
A code agent that wants a production number it cannot query returns a finding marked
`unverifiable` naming the number it wanted, per the handoff already specified at
`VALIDATION.md:772`, and the synthesizer reconciles it against the probe digests it holds.

### Step 4: the priority call

`worth_fixing` of `no` or `unclear` produces a `blocking_questions` entry. It must be phrased as a
priority call, which is the only legal shape.

"How many users are affected?" is banned verbatim as a user question at `SKILL.md:96`, under the
heading "These are never questions for the user. Each one is a search you skipped". A warehouse
number is never a question in prose either, per `SKILL.md:106`. But "a scope or priority call" is on
the legitimate list at `SKILL.md:101`, and so is a judgement between two workable approaches.

So the question looks like this:

> Reachability says the path is live at billing/retry.ts:88 but nothing measures how often. Fixing
> it is M, meaning three files plus a backfill. Fix it, defer it, or run query 1 in
> /tmp/claude/work-on-KEY-queries.sql to settle it first?

It carries `sql` wherever a warehouse query would settle it, and goes down the existing pooling
protocol in `DB-QUERIES.md`. It must be classed blocking, because `open_questions` is an array of
plain strings at `VALIDATION.md:666` and would silently destroy the `sql` field.

The gate is not a new mechanism. `blocking` has no reader anywhere, nothing branches on it, and
Step 4's gate reads only the verdict enum. Step 4 presents the block and asks. Consequences:

- No new verdict value.
- None of the three "stop the run" copies need widening. They are at `SKILL.md:236`, `:526`, and
  `:985`.
- The stash-settlement rule's existing (a), (b), (c) branches already cover the user stopping there.

### Step 5: what gets published

The facts go into the comment, which is defined as the validation trail at `SKILL.md:630`. A
triage fact has the same shape as the lines already there. "Still reproduces at
src/billing/tax.ts:88" and "path is live at src/billing/retry.ts:88 behind FLAG_X, off in prod" read
identically.

The recommendation is not published at all. It stays in chat at Step 4 where the human decides.
Three rules force this:

- Every claim carries a locator, `SKILL.md:989`, and a judgement has none of the five accepted
  kinds.
- Never invent a metric, `SKILL.md:1014`. An unmeasured number becomes a `TODO(user):` line.
- There is no wording confirmation at any step, `SKILL.md:604`. A published recommendation on
  someone else's ticket would ship with no human having read it, and the read-back only proves the
  text rendered.

No table. `JIRA-FORMAT.md:67` calls a markdown table the highest-risk construct and the most common
way a ticket arrives mangled. Use a bullet per row.

Every `TODO(user):` line the assessment produces joins the Step 5 running list and the Final report
roll-call, and the report forbids collapsing them into a count.

## Parity between the two paths

The inline path cannot reach full parity, so it reports the gap instead of claiming it.

It has no lens enumeration to extend, since `SKILL.md:416` is one sentence naming four reading
activities. It has no schema. It has no skeptics, so no gate refutation is possible there.

So the inline path dispatches the triage agents as concurrent `Agent` calls, exactly as it already
does for the telemetry briefs, supplying four things:

- the brief
- `triageRules()`
- the context block, including the `<<<UNTRUSTED_INPUT_BEGIN>>>` and `<<<UNTRUSTED_INPUT_END>>>`
  delimiters, which the rules text refers to by name
- `TRIAGE_SCHEMA`

Then it records that the dismissal went unchallenged, as a coverage gap. Claiming parity would be
the silent degradation this skill's own abort rules exist to prevent.

## Edit list

Twenty-nine sites across four files. The machinery is small. The prose duplication is the work.

### `work-on/SKILL.md`

| Site | Why |
|---|---|
| 3-15 | Frontmatter description, a closed four-axis enumeration, and the routing text that decides whether the skill fires |
| 283-397 | Step 1 records `kind` and `kind_because`, forces the telemetry flags, states the kind in the handoff |
| 398-455 | Step 2, the triage pass on both paths |
| 456 | "the output is the same shape", which enumerates exactly three parts |
| 133 | Says "three more things... all three" where `VALIDATION.md:673` says "all four" for the same three items. Pre-existing, fix while here |
| 459-492 | Step 3, one line saying the triage ask is a priority call and not a measurement request |
| 493-575 | Step 4, present the block, ask the priority call, state that this is not a verdict |
| 630-648 | Step 5, triage facts join the validation trail, unmeasured numbers become TODO lines, no table |
| 86-88 | Add the worth-it row to the Earn every question table, so the new route is explicit rather than a silent second route for an already-routed ask |
| 809 | "Its seven lenses are leaf readers that spawn nothing", load-bearing for Step 8's safety argument |
| 968-1035 | Constraints, the recommendation is never published, and `worth_fixing: 'no'` needs a locator |

`SKILL.md:403`'s anchoring argument needs no edit. That is the payoff for triage not being a lens.

### `work-on/VALIDATION.md`

All script changes go inside the single fenced block. The test hard-fails when the file contains more
than one javascript fence, at `tests/work_on_validation_test.sh:30`.

- `meta.description` at 139, a second copy of the four-axis enumeration
- `meta.phases` at 141, add a Triage phase, keep the lens count true
- the `TRIAGE` array with three briefs
- `triageRules()`
- `TRIAGE_SCHEMA`
- `probes` dispatch at 825, add `kind: 'triage'` with an enabled filter
- the run log at 830, count triage separately so `LENSES.length` stays honest
- the gate refute stage, `MAX_GATE_REFUTED`, the priority sort, the `tallyVotes` reuse, the new
  compose function
- the gate's own return fields
- `coverageGaps` at 983, keep gate gaps distinguishable from verdict gaps
- `VERDICT_SCHEMA` at 614, the `triage` block
- the synthesizer prompt, how to assemble the block and reconcile an unverifiable number against the
  probe digests
- the critic prompt at 990, its lens-count premise, and hand it the triage findings
- "Reading the result", a bullet matching the shape of the existing eight
- "Afterwards", what a triage agent re-run must carry
- the brief-companion count at 673

### Outside the skill

- `shared_config/.claude/AGENTS.md:81`, the agent roster and the "every one of those is a leaf"
  invariant that the deny-hook carve-out is justified on. This file loads into every session.
- `tests/work_on_validation_test.sh`, cases for the new compose function.

### One prohibition

No new comment or prompt text inside the fenced script may spell the skill's own name at a word
boundary. `deny-review-in-workflow.py` scans the script body, and the call passes today only because
`work-on-validate` carries a trailing hyphen. This has already broken once from a commit subject,
per `shared_config/.claude/AGENTS.md`.

## Verification

Only `VALIDATION.md` has automated coverage. `SKILL.md`, `DB-QUERIES.md`, and `JIRA-FORMAT.md` have
none. The pre-commit hook names no markdown and no test, and the CI shell-syntax job runs `zsh -n`
over three shell files. A half-finished edit across the two skill files ships green.

Testable in the existing harness:

- the script still parses, via `node --check`
- exactly one javascript fence
- the new compose function's behavior
- that no triage finding ever carries `verdict_critical`

Worth adding, and cheap: grep assertions tying the duplicated counts together. The critic prompt's
lens count against `LENSES.length`, and every `TRIAGE` key appearing in `SKILL.md`'s inline-path
section.

One trap on those assertions. This skill's prose is hand-wrapped, so a pattern with literal spaces
can match zero lines and the assertion passes vacuously. Every cross-file pattern needs `\s+` for
each space, and needs a negative control proving it fails when the invariant is actually broken.

## Rejected alternatives

**A fifth verdict value.** Simplest plumbing, one enum entry. Rejected because the enum would mix
"is the problem real" with "is it worth the money", so a valid defect nobody should fix and a defect
that never existed become the same token. Step 4's evidence rules are written per verdict and would
need a new paragraph anyway.

**Advisory only, never gating.** Cheapest to build. Rejected because a low-impact, high-effort defect
would walk into Step 5's ticket rewrite and Step 6's implementation with nothing asking the user.

**One combined worth-fixing agent.** One prompt instead of three. Rejected because the same agent
would hold both sides of the trade, which is the motivated-reasoning trap this phase is built
against. An agent that has just found the fix is trivial will grade the defect as worth fixing.

**No new agents, extend the existing lenses.** Zero new agents. Rejected because telemetry is
flag-gated and off on a run with no production claim, so the impact block would come back empty
exactly when it was needed, and an empty block reads as low impact. Realistic exploitability also
fits none of the seven existing lenses.

**Folding gate findings into `verdict_critical` and raising `MAX_REFUTED` to eight.** One number to
change. Rejected because impact claims would compete with superseded claims for the same slots, the
floor argument would need rewriting, a test pins the current floor, and the worst-case skeptic count
grows from twelve to sixteen.

**Letting `unclear` pass through silently.** Rejected by the user in favour of a blocking question.
Recorded because the concern that motivated it is real and the mitigation below addresses it.

## Checked and dismissed

An audit flagged a conflict between `writing-work-docs`'s template rule, "use its headings and add
none", and the "drop any section the work does not touch" rule at `SKILL.md:616-628`. On reading both, this is not
a conflict. The drop rule governs `SKILL.md`'s own fallback structure, which is used only when the
project has no template of its own. `writing-work-docs` governs a template it was handed. Not
carried into this design.

## Open items

**The question may fire often.** Most bug tickets have no telemetry pointed at them, so `unclear`
will be a common outcome, and a per-run question sits awkwardly beside the Earn every question rule.
The mitigation to implement: suppress the question when the effort is small enough that the answer
could not change the decision. Only the synthesizer holds both numbers, because `fix-cost` is blind
to impact, so the suppression belongs there. Note that `QUESTION_ITEM` has no field for "suppress
when the answer cannot change the outcome". Findings have `verdict_critical` for exactly this
purpose and questions have no analogue, so this has to live in the synthesizer's prompt rules rather
than in the schema.

**Provenance note.** The audit behind several of the locators in this document ran with its `args`
delivered as the string `undefined`, so its readers judged reconstructed designs rather than this
one. Every locator quoted here was verified directly against the files afterwards. The audit's
severity ratings were not carried over.
