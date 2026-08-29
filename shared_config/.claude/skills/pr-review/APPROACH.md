# Step 2.7: Approach review stage (one pair, run once)

## Contents
- Overview: what this stage judges, and the model/effort split
- Step 2.7a: The debate (orchestrator-mediated, up to 3 rounds)
- Step 2.7b: Dedup survivors against the existing findings
- Step 2.7c: Tag and fold into the findings pool
- Sub-agent prompts

## Overview

A single **approach reviewer** and a single **more-nuanced counterpart agent** debate the
issues the approach reviewer raises. This stage judges one thing only: **is this the right way
to build it?** The approach reviewer does not hunt for bugs, edge cases, races, security, or
error-handling gaps. The four reviewers already cover those. It weighs the design instead. Is
the approach and the implementation the best available? Is the code over- or under-engineered?
Is it in the right place (a DD count stat emitted in the wrong layer, say)? Is the architecture
solid or duct-taped? Does a pattern or utility for this already exist in the repo that the
author reinvented? Only the issues the two *converge* on as genuinely needing a code change
survive, and only if no other reviewer already raised the same thing with confidence > 50.
Survivors are added to the findings pool (Step 2 output) so Step 3 posts them exactly like
every other finding — distinguished only by an `approach` tag.

**Model and effort: split the pair by role, pinned in agent definitions.** Spawn the proposer as
`subagent_type: pr-review-approach-proposer` (Sonnet, effort `medium`) and the judge as
`subagent_type: pr-review-nuanced-judge` (Opus, effort `high`). Pass no `model` override; both
knobs live in `.claude/agents/`. The judge is deliberately the one stage held at the top tier and
at high effort.

**If either `subagent_type` does not resolve**, apply the same fallback Step 1 uses for the
finders. Fall back to a plain Agent call with the matching model, Sonnet for the proposer and Opus
for the judge, and state in the Step 4 report that effort could not be pinned for that role and
therefore inherited the session value.

**If the proposer or the judge returns nothing**, errored output, or output you cannot parse, the
approach stage did NOT run. Report it in Step 4 as `approach stage did not run: <reason>`, naming
which role was missing. Do not treat a non-response as "no approach findings", and do not run the
missing role's lens yourself and attribute it to the pair. Silence from the proposer means nothing
was proposed to judge, so no approach findings post. Silence from the judge means nothing was
adjudicated, so the proposer's findings do NOT post on their own. The debate is the filter for this
stage, and posting an unjudged proposal would skip the filter entirely. A missing approach stage is
an absence of coverage, exactly as a missing finder is, and it is reported rather than read as a
clean result.

The proposer's job is recall over design issues, and measured approach-finding recall was
**identical on Sonnet and Opus** (50% on one fixture, 100% on the other, for both tiers). A
general Sonnet reviewer had already caught the same design issues unaided, so Opus was paying
1.67x for nothing at the proposing step. The judge stays on Opus because the debate IS the
filter, and that is genuine judgment rather than recall. Note this specific split is
**untested**. The experiments measured the proposer's recall, never the debate's convergence
quality, so if agreed-but-wrong findings start appearing, promote the proposer back to Opus and
say so here.

It is cheap either way: one pair, run once (not 3x), <=3 rounds.

This stage runs **after** the merge because the dedup in Step 2.7b below compares against
`merged_all`, which only exists once Step 2 has merged the four reviewers. The pair runs **once**
(not 3x). The internal debate is the filter, so cross-instance triangulation is not needed. A run
that aborted in Step 2 for no fan-out never reaches this stage. There is no `merged_all` to dedup
against, and an approach pair debating alone is not a review.

**Both agents are read-only with respect to GitHub** — same forbidden-write list as sub-agents
1–4. **Both are read-only in the working tree too**, which is a separate rule: no file edits, and
none of `git checkout -- <path>`, `git checkout .`, `git restore`, `git reset --hard`,
`git clean`, `rm`, or `git push`. Their agent definitions pin `tools` to `Bash, Read, Grep, Glob`
so `Edit` and `Write` are not merely forbidden but absent. `Bash` stays because grounding a
finding needs git reads, so the prose rule still carries the rest.

The approach reviewer **explores the wider codebase** to ground its judgment. It reads
neighboring modules, greps for existing patterns and utilities, and checks where similar code
already lives, not just the PR diff. That exploration is also what once gave it a reason to run
`rm -rf` against an unrelated worktree, which the rm permission gate stopped. Neither agent is
given the **other reviewers'** findings, so this pass stays independent (the dedup happens here,
at the orchestrator, not inside the agents).

## Step 2.7a: The debate (orchestrator-mediated, up to 3 rounds)

The orchestrator mediates the exchange, carrying the current state of every contested finding
between agents. **Round 1 is the propose+judge pass (count = 1); each later rebut iteration —
one approach re-spawn plus one nuanced re-spawn — is one more round.** Track, per finding,
the latest `approach_verdict` and `nuanced_verdict` (each `yes` = needs a code change, or `no`); a
finding's `rounds` is the 1-based round at which it converged.

1. **Round 1 — propose + judge.** Spawn the approach reviewer (prompt below); it returns a
   `findings` list, each with `{id, title, file, line_range, description, suggested_fix,
   confidence (0–100), argument}`. Every proposed finding starts at `approach_verdict = yes`. Then
   spawn the nuanced agent with the diff + that `findings` list; it returns a `needs_change`
   (`yes`/`no`) + `argument` per finding id -> set `nuanced_verdict`.
2. **Classify each finding:** *converged-keep* (both `yes`), *converged-drop* (both `no`), or
   *contested* (verdicts differ).
3. **Rounds 2–3 — rebut.** While there is at least one *contested* finding AND fewer than 3
   total rounds have run: re-spawn the approach reviewer with, for each contested finding,
   the nuanced agent's latest `argument`; it returns an updated `{needs_change, argument}` per
   id (it may concede to `no` or hold at `yes`) -> update `approach_verdict`. Then re-spawn the
   nuanced agent with the approach reviewer's latest `argument` per contested finding; it
   returns an updated `{needs_change, argument}` per id (it may concede to `yes` or hold at
   `no`) -> update `nuanced_verdict`. Re-classify. Only *contested* findings are carried
   forward; once a finding is *converged-keep* or *converged-drop* its verdicts are frozen and
   it is not re-sent.
4. **Convergence cap = 3 rounds.** A finding **survives only if it converges to BOTH `yes`**.
   Anything converged to `no`, or still *contested* after round 3, is **dropped.** Agreement
   is required, and ties go to drop.

### Worked example

Three findings, to make the round accounting concrete.

- **Round 1.** The proposer raises A, B, and C, all at `approach_verdict = yes`. The judge returns
  `yes` on A, `no` on B, `no` on C. Classify: A is *converged-keep* and frozen. B and C are
  *contested*, since the proposer says yes and the judge says no.
- **Round 2.** Only B and C are re-sent. A is frozen and is NOT re-sent, so its verdicts never
  change again. The proposer concedes on B, setting `approach_verdict = no`, and holds on C. The
  judge then holds `no` on both. Re-classify: B is now *converged-drop*, both `no`. C is still
  *contested*.
- **Round 3.** Only C is re-sent. Both sides hold. Rounds reach the cap of 3 with C still
  *contested*, so **C is dropped** under "ties go to drop".
- **Result.** A survives to Step 2.7b. B and C do not. The debate ran 3 rounds because C stayed
  contested, even though A settled in round 1 and B in round 2.

The count that matters is TOTAL rounds run, not rounds per finding. A finding that converges in
round 1 costs no further rounds, and one finding still contested keeps the loop going for the rest.

## Step 2.7b: Dedup survivors against the existing findings

For each surviving finding, **drop it if it duplicates any entry in `merged_all` (Step 2)
whose max `confidence` > 50.** "Duplicate" uses the same test as Step 2: same `file` +
overlapping `line_range` + substantially the same problem (paraphrases count). This suppresses
approach findings that another reviewer already proposed with confidence > 50, *including*
the 51–59 ones that did not clear the >=60 posting bar. A surviving finding that matches nothing
in `merged_all` over 50 is kept.

## Step 2.7c: Tag and fold into the findings pool

Each kept approach finding becomes a normal finding for Step 3 with:

- `source = "approach"`, `cross_instance_agreement = null` (it is not one of the four reviewers).
- `confidence` = the approach reviewer's own round-1 score — used **only** for
  ordering/display. The rebut rounds do not re-emit `confidence`, so carry the round-1 value
  forward.
- `rounds` = how many debate rounds it took to converge.
- no `permalink`, because approach findings skip the Step 2 merge that assigns one, so the global
  `**N.**` block omits the permalink paragraph for them.

**Agreement is the gate, not the score.** Approach findings are added to the Step 2 findings
pool **without** re-applying the >=60 filter. They already passed the mutual-agreement bar in
2.7a. Order them within the pool by `confidence` like the others (for the tiebreakers, treat
`cross_instance_agreement = null` as lowest, an approach finding as single-source — never both — for the
both-sources tiebreaker, `role_agreement = null` as lowest since approach findings never pass through
the Step 2 merge that would assign one, and lowest for the category-priority tiebreaker). They then flow
through Step 3's INLINE/GLOBAL classification and posting unchanged, except for the annotation
in Step 3b.

If no approach finding survives 2.7a, or all survivors are dropped in 2.7b, this stage adds
nothing and the rest of the run is unchanged.

## Sub-agent prompts

**Approach reviewer (round 1 — propose):**

```
You are the APPROACH reviewer in a pr-review orchestration reviewing PR #<PR>. You judge ONE
thing: is this the right way to build it? You do NOT hunt for bugs, edge cases, race
conditions, security holes, or missing error handling. Other reviewers cover those, and a
defect finding here will be ignored. Assume the author is new to this codebase. Their code may
run, yet still be the worst possible implementation. Your job is to catch that.

Weigh only design and structure:
- Is the approach the best approach, and the implementation the best implementation?
- Is it over-engineered (needless abstraction, indirection, config for a constant) or
  under-engineered (a fragile hack where the codebase already has a real mechanism)?
- Is the code in the right place? E.g. a Datadog count stat emitted in the wrong layer, logic
  in a controller that belongs in a service, a helper defined far from where it is used.
- Is the architecture solid, or is it duct-tape that will not hold?
- Does a pattern or utility for this ALREADY EXIST in the repo that the author reinvented?

To judge these you must look beyond the diff: read neighboring modules, grep for existing
patterns and utilities, and check where similar code already lives. Ground every finding in
what the codebase actually does, not a guess.

For every issue you can defend, return one finding. Return STRICT JSON:
{ "findings": [ {
  "id": "A1", "title": "...", "file": "<path>", "line_range": "<start>..<end>",
  "description": "<what is wrong with the approach and why>",
  "suggested_fix": "<the better approach, correct placement, or existing thing to reuse>",
  "confidence": <0-100>,
  "argument": "<why this genuinely needs a code change>"
} ] }

`confidence` is your own 0–100 estimate (used only for display ordering). Do not invent issues
you cannot defend. A weak finding will be argued down in the debate that follows. Return
`{"findings": []}` if the approach is sound.

Forbidden (you are read-only w.r.t. GitHub): `gh pr comment`, `gh pr review`, `gh pr edit`,
`gh pr close`, `gh pr merge`, `gh issue create`, `gh issue comment`, or any other GitHub write.
```

**Nuanced agent (each round — judge):**

```
You are the NUANCED reviewer in a pr-review orchestration, the pragmatic counterweight to an
approach reviewer on PR #<PR>. The approach reviewer critiques design and structure, meaning whether
the code is over- or under-engineered, in the wrong place, duct-taped, or reinvents something
the repo already has. For each finding it raises, decide whether the rework GENUINELY warrants
a code change in THIS PR. Weigh the design tradeoff: how much better the proposed approach
really is, the cost and risk of the rework, whether "works but not ideal" is acceptable here,
and whether the change is in scope or better left to a follow-up. You are not here to
rubber-stamp and not here to reflexively dismiss. Judge each on its merits, given the diff and
the surrounding codebase.

You receive the PR diff and a list of findings — the approach reviewer's full proposal set in
round 1, or only the still-contested findings in later rounds — each with the approach
reviewer's latest `argument`. Return STRICT JSON:
{ "verdicts": [ { "id": "A1", "needs_change": "yes" | "no", "argument": "<your reasoning>" } ] }

Use "yes" only when you agree the rework is warranted in this PR. Use "no" when it is not worth
a change. Respond to the approach reviewer's specific argument. Concede when they are right,
push back when they overreach.

Forbidden (read-only w.r.t. GitHub): `gh pr comment`, `gh pr review`, `gh pr edit`,
`gh pr close`, `gh pr merge`, `gh issue create`, `gh issue comment`, or any other GitHub write.
```

**Approach reviewer (rounds 2–3 — rebut):** re-use the approach prompt above, but instead
of proposing fresh findings, pass it the contested findings plus the nuanced agent's latest
`argument` for each, and ask it to return
`{ "responses": [ { "id": "...", "needs_change": "yes" | "no", "argument": "..." } ] }` —
holding at `yes` only where it can still defend the issue, conceding to `no` otherwise.
