# Score each finding once, after the cross-instance merge

Design settled 2026-08-31. Covers `in-depth-review`, `review-and-fix` and `pr-review`.

## The problem

Each `in-depth-review` instance scores its own findings before returning them. The caller then
merges across instances and keeps `max` of each duplicate group's scores. So a finding two
instances both raised is scored twice, by two different Haiku agents, and one of the two numbers is
thrown away.

Measured on run `20260829-004744-api-ml-stripe-webhooks-GRO-18050`, iteration 3. Instance 1
returned 38 findings and instance 2 returned 42, so 80 findings were scored. The merge produced
about 46 unique findings. About 34 of the 80 scorings were of a finding the other instance had also
found and also scored, which is roughly 42 percent of the scoring work.

`pr-review` is worse because it runs four reviewer instances rather than two.

## Two defects, not one

**The waste.** Scoring happens before the only dedup that sees more than one instance, so
duplicates are scored once per instance.

**The inflation.** `AGGREGATING.md` sets `confidence` to the `max` of a duplicate group's scores.
Two independent draws from a noisy scorer, with the larger kept, biases the result upward. So a
finding gets a confidence boost for having been found twice, on top of the boost corroboration
already gets from `cross_instance_agreement` in the sort order. Corroboration is counted twice, and
one of the two was nobody's intent.

Scoring once removes both. There is no group of scores left to take a max of.

## Constraints the design has to respect

- `in-depth-review` runs standalone as `/in-depth-review`, where no caller exists to score for it.
- Three thresholds read the same scores. `in-depth-review` filters below 70, `review-and-fix` below
  50, `pr-review` below 60.
- `--raw` already separates scoring from filtering. It skips the below-70 filter and returns all
  scored findings. Both multi-instance callers pass it and apply their own threshold. The new flag
  goes one notch further and must not change what `--raw` means.
- The two-stage rule in `in-depth-review` Step 2 exists because of a measured incident. A
  fabricated finding scored 100 and survived the filter, because the model that invented the
  precedent was also the model that graded it. Any change here has to say how it avoids that.
- `COLLECTING.md` owes at least as many collecting turns as agents launched before its give-up
  counter may arm. Agent count is therefore also orchestrator turn count.
- Reading the `scoring` block: `review-and-fix/AGGREGATING.md`, `review-and-fix/SUMMARY.md`, and
  `pr-review/AGGREGATING.md`. `open-ticket` and `work-on` name the skill without reading its
  scoring, so they need no change.

## Design

### The contract

`in-depth-review` gains `--defer-scoring`. Under it the skill returns findings carrying `severity`,
`role_agreement` and `citation_verified` as usual, with `confidence: null`, and a `scoring` block of
`{ "deferred": true }`. It spawns no scorer.

The rule, stated once so it generalizes rather than reading as a special case: **score after your
own dedup, and do not score if you were told your findings merge upstream.** Standalone
`in-depth-review` is simply the case where it owns the final dedup, so it scores. A caller running
several instances owns the final dedup, so the caller scores.

`--raw` is untouched.

### Where scoring happens

A new step in both callers' `AGGREGATING.md`, between the cross-instance dedup and the threshold.
`review-and-fix` scores then filters below 50. `pr-review` scores then filters below 60.

`confidence: max of the group's scores` is deleted from both files. One score per unique finding
exists, so there is nothing to reduce. `cross_instance_agreement` stays as the sort key, which is
where corroboration was always meant to count.

### The scorer

A new agent file, `agents/review-scorer.md`, with `model: sonnet` and `effort: low` pinned so the
tier does not track whatever the user last set with `/effort`. Callers spawn it by
`subagent_type: review-scorer` and pass no `model` override, matching how every other reviewer in
these skills is addressed, so the agent file stays the single source of truth for its tier.

Sonnet rather than Haiku, changing a documented decision. The Haiku rationale was that scoring one
finding is a small structured judgment and Haiku is 15 to 20 times cheaper, which was correct when
scoring meant one agent per finding. After this change plus the batching in `f4255e5`, a
`review-and-fix` iteration spawns about one scoring agent instead of 70 to 86, so the tier is close
to free. Scoring is the gate that decides which findings reach a human, and one consistent
calibration of `SCORING.md`'s bands is worth more than the saving.

One agent handles up to about 60 findings. Above that, batches of about 20, so a large iteration
degrades into a few agents rather than one agent holding everything. The prompt carries the
absolute-rubric rule already written for batching: score each finding against `SCORING.md`'s bands
alone, never relative to the others in the batch, and return one score per finding identifier
rather than a ranked list.

### The ladder

Success is checked after every rung. A rung runs only if findings remain unscored.

1. Spawn the scorer with every unique finding.
2. If nothing came back at all, nudge it once with `SendMessage`, asking it to finalize with
   whatever it genuinely has.
3. If nothing came back, or only part did, spawn ONE fresh scorer and send it only the findings
   still unscored. Exactly one relaunch. `AGGREGATING.md` already records that unbounded relaunches
   are the same bug with extra bookkeeping, and this matches row 1b's one retry per reviewer kind
   per run.
4. If every finding now has a score, merge and continue to the threshold.
5. If findings are still unscored, score those inline and mark them.

Rung 3 exists because a stall is usually transient. The project memory
`kill-stalled-review-subagents` records three roles that never reported in one iteration and
finished in under a minute each when the next iteration reran them. Going from the nudge straight to
inline scoring skips the cheap repair.

### Provenance and accounting

Each finding carries `scored_by`, one of `scorer`, `scorer-retry`, or `inline-fallback`. Per finding
rather than per run, because the ladder produces mixed provenance within one iteration and a
run-level label would flatten it and hide which numbers to distrust.

There are now two scoring blocks at two scopes, and conflating them is the likeliest way this design
gets implemented wrong.

An `in-depth-review` instance run with `--defer-scoring` emits `scoring: { "deferred": true }` and
nothing else. It has no `findings_scored`, no `complete`, and no `scorers_spawned`, because it
scored nothing and a `complete: false` there would read as a degraded instance rather than a
deliberate deferral. Every reader of that block has to treat `deferred: true` as its own case,
distinct from both `complete: true` and `complete: false`.

The caller then keeps its own scoring block for the iteration, and that is where
`complete` lives. It keys on `findings_scored == unique_findings`, the same rule `f4255e5`
established, counting findings rather than agents. `inline_fallback_count` sits beside it, so a run
that reached rung 5 says so without a reader scanning per-finding marks. A run log can then be
grepped to find out whether rung 5 ever fires at all.

### Narrowing the two-stage ban

`in-depth-review`'s "never self-assign a confidence value" stays exactly as it is, for
`in-depth-review`.

Rung 5 lives in the callers, and the note beside it says why it is not the banned thing. The
incident behind the ban was a proposer grading its own invention. Under this design the proposers
are role agents inside `in-depth-review`, and the caller merging their output never proposed
anything, so the mechanism does not apply.

The note also names the bias that IS new, because it would otherwise go unrecorded. The caller is
the model that will be asked to fix these findings, so it has an interest in their scores that a
role agent does not have. That is why rung 5 is the last rung and why it is marked. A number from
it is worth having and is not worth the same as a number from rung 1.

## What this does not change

- `--raw` semantics.
- The three thresholds, and which caller applies which.
- `cross_instance_agreement` as a sort key.
- `citation_verified` and its pessimistic merge, which happens during in-depth-review's own dedup
  and is unrelated to scoring.
- Standalone `/in-depth-review`, which keeps scoring its own findings.
- The batching committed in `f4255e5`, which this builds on rather than replaces.

## Files

| file | change |
|---|---|
| `in-depth-review/SKILL.md` | `--defer-scoring` flag, parsing, and the skip in Step 2 |
| `in-depth-review/OUTPUT-JSON.md` | `scoring: {deferred: true}`, `scored_by`, `inline_fallback_count` |
| `review-and-fix/AGGREGATING.md` | scoring step after dedup, delete the max, the ladder |
| `review-and-fix/SUMMARY.md` | report `scored_by` spread and `inline_fallback_count` |
| `review-and-fix/PROMPT-IN-DEPTH.md` | pass `--defer-scoring` |
| `pr-review/AGGREGATING.md` | same scoring step and ladder at its own threshold |
| `pr-review/PROMPT-FINDER.md` | pass `--defer-scoring` |
| `agents/review-scorer.md` | new, `sonnet` / `low` |

## Verification

No test suite covers these skills, so verification is by replay and by inspection.

- Re-derive iteration 3 of the GRO-18050 run by hand. 80 findings scored today, about 46 under this
  design, and no `max` taken.
- Confirm every reader of the `scoring` block handles `deferred: true` as a third case, across the
  three files named in the constraints. A reader that treats it as `complete: false` would exclude
  every finding of every deferred instance, which is the whole review.
- Confirm the caller's own `complete` cannot be false on a healthy run. This is the defect that
  nearly shipped in `f4255e5`, where the field still compared agent count to finding count and would
  have reported every batched run degraded.
- Confirm the standalone path is unchanged by reading the flag's absence through Step 2.

## Deferred

The approach and nuanced pair in `pr-review` Step 2.7 post on agreement alone and do not clear the
confidence bar. They are outside this design and stay as they are.
