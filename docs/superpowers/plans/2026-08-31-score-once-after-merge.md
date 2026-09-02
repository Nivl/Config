# Score Once After Merge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Score each unique review finding exactly once, after the cross-instance merge, instead of once per reviewer instance before it.

**Architecture:** `in-depth-review` gains `--defer-scoring` and returns unscored findings. Both multi-instance callers score the merged set themselves with one pinned Sonnet agent, behind a five-rung recovery ladder. The `max`-of-scores merge rule is deleted because only one score per finding now exists.

**Tech Stack:** Markdown skill prose under `shared_config/.claude/`. No code, no test runner. Verification is grep and replay against a real run log.

**Spec:** `docs/superpowers/specs/2026-08-31-score-once-after-merge-design.md` (untracked, read from disk)

## Global Constraints

- Prose follows `~/.claude/AGENTS.md`: plain ASCII (`->` not an arrow), no em-dash or ` - ` or `:` joining two clauses, one thought per sentence, name a set rather than quantifying it.
- Never commit anything under `docs/superpowers/`. It is gitignored by `.gitignore:1` and the user has asked for the spec and this plan to stay local.
- `--raw` semantics must not change. It skips the below-70 filter and returns all **scored** findings.
- Thresholds stay where they are: `in-depth-review` 70, `review-and-fix` 50, `pr-review` 60.
- Every reviewer is spawned by `subagent_type` with no `model` override, so agent files stay the single source of truth for tier.
- The three files that read the `scoring` block are `review-and-fix/AGGREGATING.md`, `review-and-fix/SUMMARY.md`, `pr-review/AGGREGATING.md`. No fourth reader exists.
- One commit per task, message in this repo's style: what was wrong, the measurement, what changed, why.

---

### Task 1: `--defer-scoring` on in-depth-review

**Files:**
- Modify: `shared_config/.claude/skills/in-depth-review/SKILL.md:40` (flag list), `:141` (flag parsing), `:441` (Step 2 scoring dispatch)
- Modify: `shared_config/.claude/skills/in-depth-review/OUTPUT-JSON.md:39-44` (scoring block), `:88` onward (the prose about it)

**Interfaces:**
- Consumes: nothing. This is the producer side and lands first.
- Produces: the flag name `--defer-scoring`; the emitted block `scoring: { "deferred": true }`; the per-finding field `scored_by` with values `scorer`, `scorer-retry`, `inline-fallback`. Tasks 3 and 5 read all three.

- [ ] **Step 1: Add the flag to the flag list**

In `SKILL.md`, after the `--raw` bullet at line 40, add:

```markdown
- `--defer-scoring` — score nothing. Return every finding with `confidence: null` and emit
  `scoring: { "deferred": true }`. For a caller running several instances, which merges them and
  then scores each unique finding once. Standalone runs must not pass this, because nothing
  downstream would ever score. Distinct from `--raw`, which still scores and only skips the filter.
```

- [ ] **Step 2: Add the flag to the parser**

In `SKILL.md`, beside the `^--raw$` line at 141, add:

```markdown
   - Matches `^--defer-scoring$` -> flag (read in Step 2). Incompatible with nothing. `--raw` may
     be passed alongside it and is then redundant, because a deferred run has no scores to filter.
```

- [ ] **Step 3: Make Step 2 skip the scorer under the flag**

In `SKILL.md` Step 2, at the scoring dispatch (line 441 area), add before the batch instruction:

```markdown
   **When `--defer-scoring` is in effect, skip this entire stage.** Spawn no scorer, leave every
   finding's `confidence` at `null`, do not set `unscored`, and emit `scoring: { "deferred": true }`.
   Leaving `unscored` off is deliberate. That field means a scorer was owed and did not deliver, and
   the caller excludes on it before its merge, so setting it here would make the caller discard
   every finding of a healthy deferred run. A deferred run is not a degraded run.
```

- [ ] **Step 4: Update the JSON contract**

In `OUTPUT-JSON.md`, replace the `scoring` block with:

```json
  "scoring": {
    "unique_findings": <count after pre-score dedup>,
    "findings_scored": <count of findings that came back with a score; omit when deferred>,
    "scorers_spawned": <count of scoring sub-agents actually spawned; informational only>,
    "complete": <true when findings_scored == unique_findings; omit when deferred>,
    "deferred": <true only when --defer-scoring was passed; omit otherwise>
  },
```

Then add after the existing `complete` prose:

```markdown
`deferred: true` is a THIRD case and is neither `complete: true` nor `complete: false`. It means the
caller asked for no scoring and will score the merged set itself. A deferred block carries no
`complete` and no `findings_scored`, because both would be zero and a reader keying on
`complete: false` would exclude every finding of a perfectly healthy instance. That is the whole
review. Any reader of this block handles three cases, not two.

Each finding carries `scored_by`, one of `scorer`, `scorer-retry`, or `inline-fallback`, naming which
rung of the caller's ladder produced its number. Absent on a deferred instance's findings, since it
produced none.
```

- [ ] **Step 5: Verify the flag is wired in all four places**

Run: `grep -c "defer-scoring" shared_config/.claude/skills/in-depth-review/SKILL.md`
Expected: at least `3` (list, parser, Step 2)

Run: `grep -n "deferred" shared_config/.claude/skills/in-depth-review/OUTPUT-JSON.md`
Expected: the JSON field plus the three-cases paragraph

- [ ] **Step 6: Verify `--raw` was not touched**

Run: `grep -n "skip the internal .< 70. confidence filter" shared_config/.claude/skills/in-depth-review/SKILL.md`
Expected: still present and still describing `--raw` as returning scored findings

- [ ] **Step 7: Commit**

```bash
git add shared_config/.claude/skills/in-depth-review/SKILL.md shared_config/.claude/skills/in-depth-review/OUTPUT-JSON.md
git commit -F <message file>
```

Message covers: each instance scored its own findings so duplicates were scored once per instance; the flag lets a merging caller own scoring; `deferred` is a third case and why `unscored` is deliberately not set.

---

### Task 2: The `review-scorer` agent

**Files:**
- Create: `shared_config/.claude/agents/review-scorer.md`

**Interfaces:**
- Consumes: nothing.
- Produces: `subagent_type: review-scorer`, pinned `model: sonnet` and `effort: low`. Tasks 3 and 5 spawn it by that name.

- [ ] **Step 1: Create the agent file**

```markdown
---
name: review-scorer
description: Scores merged review findings for confidence against SCORING.md's rubric. Invoked only by the pr-review and review-and-fix skills after their cross-instance merge, never directly by a user and never by auto-delegation.
model: sonnet
effort: low
tools: Bash, Read, Grep, Glob
color: yellow
---

You score review findings for confidence. You do not review code, and you do not propose findings.

For each finding you are given, return one integer 0 to 100 against the rubric in the caller's
`SCORING.md`, keyed by the finding's identifier.

**Score each finding against the rubric alone, never against the others in your batch.** The bands
are absolute. A batch of ten weak findings scores ten low numbers and a batch of ten strong ones
scores ten high numbers, and neither is evidence you erred. Ranking the batch against itself
produces numbers that are internally sensible and wrong against the bands, which is the one failure
mode this role has that nobody downstream can detect.

Return one score per finding identifier. Never a ranked list, never a summary, and never a score for
a finding you were not given.

**If you cannot score a finding, say so for that finding and score the rest.** A partial return is
useful and the caller handles it. Inventing a number for a finding you could not judge is not.

`model` and `effort` are pinned in this definition so scoring cost does not track whatever the user
last set with `/effort`. Sonnet rather than Haiku because the caller now spawns roughly one of you
per iteration instead of dozens, so the tier is nearly free, and you are the gate that decides which
findings reach a human. Do not reason about your own tier.

You are read-only. Never edit, create, or delete a file. Never run `git checkout -- <path>`,
`git checkout .`, `git restore`, `git reset --hard`, `git clean`, `rm`, or `git push`. You share one
checkout with the orchestrator and with every other agent in the run.
```

- [ ] **Step 2: Verify the frontmatter and that the name is free**

Run: `head -8 shared_config/.claude/agents/review-scorer.md`
Expected: `model: sonnet`, `effort: low`, `tools: Bash, Read, Grep, Glob`

Run: `grep -rl "name: review-scorer" shared_config/.claude/agents/ | wc -l`
Expected: `1`

- [ ] **Step 3: Commit**

```bash
git add shared_config/.claude/agents/review-scorer.md
git commit -F <message file>
```

Message covers: scoring moves to the callers so it needs a pinned agent; why Sonnet replaces Haiku here; the absolute-rubric rule and why it is the failure mode nobody downstream can detect.

---

### Task 3: review-and-fix scores the merged set

**Files:**
- Modify: `shared_config/.claude/skills/review-and-fix/AGGREGATING.md:116-130` (the pre-merge exclusion), `:148-152` (the merged finding's fields), and a new scoring step before the threshold at `:166`
- Modify: `shared_config/.claude/skills/review-and-fix/PROMPT-IN-DEPTH.md:9,15` (pass the flag)
- Modify: `shared_config/.claude/skills/review-and-fix/SUMMARY.md:77` (report provenance)

**Interfaces:**
- Consumes: `--defer-scoring`, `scoring: {deferred: true}`, `scored_by` from Task 1. `subagent_type: review-scorer` from Task 2.
- Produces: the five-rung ladder wording and `inline_fallback_count`, which Task 4 copies for `pr-review`.

- [ ] **Step 1: Stop the pre-merge exclusion from discarding a deferred run**

This step lands first within the task because the rest of it is dead without it. In
`AGGREGATING.md`, in "Excluding unscored and unverified findings", add after the
`scoring.complete` bullet:

```markdown
- **An instance reporting `deferred: true` is a third case and nothing here excludes it.** It was
  asked not to score, so every one of its findings arrives with `confidence: null`. Treating that as
  the `unscored` case below would discard the entire review, which is the whole point of the flag
  inverted. Carry its findings into the pool unchanged and let the scoring step below give them
  numbers. `citation_verified: false` still excludes, on a deferred instance exactly as on any
  other, because verification is unrelated to scoring.
- The `unscored: true` exclusion applies to an instance that scored and came up short, never to one
  that deferred. A deferred instance does not set the field, per
  [in-depth-review's contract](../in-depth-review/OUTPUT-JSON.md).
```

- [ ] **Step 2: Delete the max**

In `AGGREGATING.md`, in the merged-finding fields, replace the `confidence` line with:

```markdown
   - `confidence`: not set here. One score per unique finding is assigned by the scoring step below,
     after this merge. The old rule took the `max` of a duplicate group's scores, which biased
     confidence upward for any finding two instances found, because the larger of two noisy draws
     was kept. Corroboration was then counted twice, once there and once deliberately through
     `cross_instance_agreement` in the ordering below. There is no group of scores to reduce now.
```

- [ ] **Step 3: Add the scoring step and the ladder**

In `AGGREGATING.md`, insert as a new numbered step immediately before the `>= 50` filter:

```markdown
4. **Score the merged set, once.** Every finding in the pool now has `confidence: null`. Spawn
   `subagent_type: review-scorer`, passing no `model` override, with every unique finding, each
   finding's identifier, its `severity`, its `role_agreement`, its `cross_instance_agreement`, the
   diff for its lines, and the paths of any AGENTS.md or CLAUDE.md a reviewer cited for it.

   One agent for up to about 60 findings. Above that, batches of about 20, so a large iteration
   degrades into a few agents rather than one holding everything.

   **The ladder. Check after every rung whether findings remain unscored, and stop as soon as none
   do.**

   1. Spawn the scorer with every unique finding.
   2. Nothing came back at all -> nudge it once with `SendMessage`, asking it to finalize with
      whatever it genuinely has.
   3. Nothing came back, or only part did -> spawn ONE fresh scorer with only the findings still
      unscored. Exactly one relaunch. Unbounded relaunches are the same bug with extra bookkeeping,
      and this matches the one retry per reviewer kind per run that row 1b already allows. A stall
      is usually transient, which is why this rung sits before the inline one.
   4. Every finding has a score -> continue to the filter below.
   5. Findings are still unscored -> score those inline, and mark them.

   Rung 5 is not the self-scoring this repo bans. That ban exists because a proposer graded its own
   invention and a fabricated finding scored 100. The proposers here are role agents inside
   `in-depth-review`, and this orchestrator merged their output without proposing anything.

   It carries a different bias, which is why it is last and why it is marked. This orchestrator is
   the model that will be asked to FIX these findings, so it has an interest in their scores that a
   role agent does not have. A number from rung 5 is worth having and is not worth the same as a
   number from rung 1.

   **Record `scored_by` on each finding**, one of `scorer`, `scorer-retry`, or `inline-fallback`.
   Per finding rather than per iteration, because the ladder produces mixed provenance inside one
   iteration and a single label would hide which numbers to distrust. Record
   `inline_fallback_count` for the iteration, so a run that reached rung 5 says so without a reader
   scanning every finding.
```

- [ ] **Step 4: Pass the flag to the instances**

In `PROMPT-IN-DEPTH.md`, change line 9 to:

```markdown
Invoke the `in-depth-review` skill with the arguments: `<TARGET_ARG> --defer-scoring --roles <ACTIVE_ROLES>`.
```

and replace the `--raw` explanation at line 15 with:

```markdown
- `--defer-scoring` tells in-depth-review to score nothing and return every finding with
  `confidence: null`. This orchestrator merges the instances first and then scores each unique
  finding once, which is why `--raw` is not passed. `--raw` would ask for scores and skip only the
  filter, and scoring before the merge scores every duplicate twice.
```

- [ ] **Step 5: Report provenance in the summary**

In `SUMMARY.md`, after the unfiltered-leads bullet, add:

```markdown
- The scoring provenance for the iteration, as the `scored_by` spread across `scorer`,
  `scorer-retry` and `inline-fallback`, plus `inline_fallback_count`. One line. A nonzero
  `inline-fallback` means the scorer failed twice and this orchestrator scored the remainder itself,
  which is a weaker number and the summary is where that becomes visible.
```

- [ ] **Step 6: Replay GRO-18050 iteration 3 by hand**

Run: `grep -n "iter=3 instance" logs/review-and-fix/20260829-004744-api-ml-stripe-webhooks-GRO-18050.md`
Expected: instance 1 at 38 findings, instance 2 at 42.

Confirm by reading the new step: 80 findings would have been scored under the old rule and about 46
are scored now, one per unique finding, with no `max` taken. Write the two numbers into the commit
message.

- [ ] **Step 7: Verify no max survives and the deferral is handled**

Run: `grep -n "max of the group" shared_config/.claude/skills/review-and-fix/AGGREGATING.md`
Expected: no output

Run: `grep -c "deferred" shared_config/.claude/skills/review-and-fix/AGGREGATING.md`
Expected: at least `2`

- [ ] **Step 8: Commit**

```bash
git add shared_config/.claude/skills/review-and-fix/AGGREGATING.md shared_config/.claude/skills/review-and-fix/PROMPT-IN-DEPTH.md shared_config/.claude/skills/review-and-fix/SUMMARY.md
git commit -F <message file>
```

Message covers: the replayed 80-to-46 numbers; that the pre-merge `unscored` exclusion would have discarded a whole deferred review and why it now has a third case; the max deleted as a bias not a workaround; the ladder and why rung 5 is not the banned self-scoring.

---

### Task 4: pr-review scores the merged set

**Files:**
- Modify: `shared_config/.claude/skills/pr-review/AGGREGATING.md:98` (the `scoring.complete` reader), `:128-130` (the `max`), and a new scoring step before the `>= 60` filter at `:171`
- Modify: `shared_config/.claude/skills/pr-review/PROMPT-FINDER.md:15,17,20` (pass the flag)

**Interfaces:**
- Consumes: everything Tasks 1 to 3 produced. The ladder wording is copied from Task 3 with the threshold and instance count changed.
- Produces: nothing further.

- [ ] **Step 1: Add the deferred case to the reader**

In `AGGREGATING.md` at the `scoring.complete` check, add:

```markdown
**An instance reporting `deferred: true` is a third case.** It was asked not to score, so all of its
findings arrive with `confidence: null` and none of this exclusion applies to them. Reading it as
`complete: false` would drop every finding from every instance, which is the entire review. This
skill merges before excluding, so it needs the downward-merge rule for `citation_verified` either
way, and that rule is unchanged.
```

- [ ] **Step 2: Delete the max**

Replace the `confidence: max of the group's scores` lines with:

```markdown
   - `confidence`: not set here. The scoring step below assigns one score per unique finding after
     this merge. Taking the `max` of a group biased confidence upward for anything more than one
     instance found, because the larger of several noisy draws was kept, and
     `cross_instance_agreement` below already counts corroboration on purpose. With four instances
     the old rule took the max of up to four draws, so the bias here was larger than in
     `review-and-fix`.
```

- [ ] **Step 3: Add the scoring step and the ladder**

Insert a new numbered step immediately before the `confidence >= 60` filter. Use the same five rungs
and the same rung-5 justification as `review-and-fix/AGGREGATING.md`, with three differences:

- the threshold named afterwards is 60, not 50
- `cross_instance_agreement` ranges 1 to 4 here, not 1 to 2
- the rung-5 bias paragraph says this orchestrator POSTS these findings to GitHub rather than fixing
  them, so its interest is in what it will be seen to have posted

Repeat the rungs in full rather than pointing at the other file. An implementer reading this task
has not read Task 3.

```markdown
   **The ladder. Check after every rung whether findings remain unscored, and stop as soon as none
   do.**

   1. Spawn `subagent_type: review-scorer`, passing no `model` override, with every unique finding.
   2. Nothing came back at all -> nudge it once with `SendMessage`, asking it to finalize with
      whatever it genuinely has.
   3. Nothing came back, or only part did -> spawn ONE fresh scorer with only the findings still
      unscored. Exactly one relaunch.
   4. Every finding has a score -> continue to the `>= 60` filter below.
   5. Findings are still unscored -> score those inline, and mark them `inline-fallback`.
```

- [ ] **Step 4: Pass the flag to the four instances**

In `PROMPT-FINDER.md`, replace `--raw` with `--defer-scoring` at lines 15, 17 and 20, keeping the
`--skip-ticket` composition intact, and replace the `--raw` explanation with the same reasoning used
in Task 3 step 4.

- [ ] **Step 5: Verify**

Run: `grep -n "max of the group" shared_config/.claude/skills/pr-review/AGGREGATING.md`
Expected: no output

Run: `grep -c "defer-scoring" shared_config/.claude/skills/pr-review/PROMPT-FINDER.md`
Expected: at least `3`

Run: `grep -n "\-\-raw" shared_config/.claude/skills/pr-review/PROMPT-FINDER.md`
Expected: no output

- [ ] **Step 6: Commit**

```bash
git add shared_config/.claude/skills/pr-review/AGGREGATING.md shared_config/.claude/skills/pr-review/PROMPT-FINDER.md
git commit -F <message file>
```

Message covers: four instances meant the waste and the max bias were both larger here; the deferred third case; that this orchestrator posts rather than fixes, so rung 5's bias differs.

---

### Task 5: Cross-file consistency sweep

**Files:**
- Read only, then fix whatever the sweep finds in the file that owns it.

**Interfaces:**
- Consumes: every change above.
- Produces: nothing.

- [ ] **Step 1: Every reader handles three cases**

Run: `grep -rn "scoring.complete\|deferred" shared_config/.claude/skills/review-and-fix/AGGREGATING.md shared_config/.claude/skills/review-and-fix/SUMMARY.md shared_config/.claude/skills/pr-review/AGGREGATING.md`
Expected: each of the three files mentions `deferred`. A file that does not is a reader that would exclude a whole review.

- [ ] **Step 2: No max survives anywhere**

Run: `grep -rn "max of the group\|max of the group's scores" shared_config/.claude/skills/`
Expected: no output

- [ ] **Step 3: No stale agent-count comparison survives**

Run: `grep -rn "scorers_spawned ==" shared_config/.claude/skills/`
Expected: no output. `complete` keys on `findings_scored`, per `f4255e5`.

- [ ] **Step 4: `--raw` still means what it meant**

Run: `grep -rn "\-\-raw" shared_config/.claude/skills/in-depth-review/SKILL.md`
Expected: still documented as skipping the below-70 filter on scored findings. Neither caller passes it any more, and standalone use is unchanged.

- [ ] **Step 5: Standalone path untouched**

Read `in-depth-review/SKILL.md` Step 2 and confirm that with the flag absent the scoring stage runs
exactly as before, including the batching from `f4255e5`.

- [ ] **Step 6: Commit any fixes the sweep found**

If steps 1 to 5 were all clean, there is nothing to commit and this task ends without a commit. Say
so rather than inventing a change.

---

## Self-Review

**Spec coverage.** The contract change is Task 1. The scorer is Task 2. Scoring moving into the
callers, the deleted max, the ladder, provenance and `inline_fallback_count` are Tasks 3 and 4. The
narrowed ban is inside Tasks 3 and 4 beside rung 5. The two-scopes ambiguity the spec's own
self-review caught is Task 1 step 4 and Task 3 step 1 and Task 4 step 1. Verification is Task 3 step
6 and Task 5. The spec's deferred item, `pr-review` Step 2.7, is named in the spec as out of scope
and no task touches it.

**Placeholder scan.** Every prose insertion is written out in full. No task says "similar to Task
N", which is why Task 4 repeats the rungs rather than pointing at Task 3.

**Naming consistency.** `--defer-scoring`, `scoring: {deferred: true}`, `scored_by` with values
`scorer` / `scorer-retry` / `inline-fallback`, `inline_fallback_count`, and
`subagent_type: review-scorer` are spelled identically in Tasks 1 through 5.

**One gap found and closed while reviewing.** Task 3's original order put the scoring step before
the exclusion fix, which would have discarded every deferred finding before scoring could run. Step
1 of that task now lands first and says why.
