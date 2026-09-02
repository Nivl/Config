# Remove the review-and-fix Iteration Cap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete the 10-iteration cap from the `review-and-fix` skill with no replacement bound, repair every claim whose truth depended on that bound, and define the per-iteration summary the file references ten times and never specifies.

**Architecture:** One markdown file, edited in place across 24 sites. The file is a Claude Code skill, meaning its prose IS the program that an LLM orchestrator reads top to bottom and follows. There is no build and no test suite, so the verification in each task is a wrap-tolerant search plus a read, and the correctness test for any sentence is whether two orchestrators reading it would behave the same way.

**Tech Stack:** Markdown. `rg` for verification. `git` for commits.

**Spec:** `docs/superpowers/specs/2026-07-31-remove-iteration-cap-design.md`

## Global Constraints

- Plain ASCII in authored prose. Do not join two independent clauses with an em-dash, an ASCII space-hyphen-space, or a clause-splitting colon. Split into sentences instead. Those forms stay allowed as label prefixes, definition dashes at the start of a list item, list introductions, ratios, ranges, CLI flags, and inside literal templates.
- **Do NOT renumber loop-control rows 4 and 5.** Eleven lines reference them by number: 545, 550, 552, 559, 573, 590, 605, 607, 637, 638, 653.
- **The Final Report is a fenced code block spanning roughly lines 660 to 725.** Never insert instruction text inside it. The orchestrator reproduces that block verbatim.
- **Always use `rg -U`.** Two cap references straddle a line break and are invisible to a line-oriented grep. This file has had four separate defects escape line-oriented greps.
- No new cap of any size, no periodic check-in, and no oscillation detector. Each was offered to the human and explicitly declined.
- Repo rules: run plain `git` from inside `/Users/melvin/.melvin/config`, never top-level `git -C`, never pipe or chain a git command, no inline scripting, no command substitution, no `$TMPDIR`, redirects last.

## There is no test framework

This file is prose. Each task's "failing test" is a `rg -U` run BEFORE editing that confirms the current text matches what this plan quotes. If it does not match, STOP and report drift rather than guessing at a merge. Each task's "passing test" is a `rg -U` after editing that confirms the removal or the replacement.

## File Structure

- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md` (795 lines before the change)

No files are created. No other file changes. `pr-review`, `in-depth-review`, and `gh-style-review` are NOT touched, because the cap is local to `review-and-fix`.

## Task ordering and the one intended intermediate state

Ordering is forced by three cross-references:

- Site 18 (Discussion Context footnote) points at the per-iteration summary's delta rule, so it must land after Task 4.
- Site 21 (Converged branch) points at a re-gated Remaining Issues section, so it must land with Task 5.
- Sites 9, 10, 14, 19, 20, 22 become false or dangling the moment row 3 goes, so they must land WITH Task 1, not after it.

**Intended intermediate state, do not flag it as a defect.** Task 1 inserts a note that references "a per-iteration summary", which Task 4 defines. Between Task 1 and Task 4 that reference is unresolved. This is no worse than the status quo, where the file already references the per-iteration summary about ten times without defining it anywhere, and Task 4 closes it.

---

### Task 1: Remove the bound and repair every claim that depended on it

This is the semantic core. After this task the file must be internally consistent with no cap, with no dangling reference to row 3 and no surviving claim that the loop is guaranteed to terminate.

**Files:**
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md` (row 3 at 541, plus sites at 353-354, 359-362, 545, 604-612, 698-701, 717-719, 723-724)

**Interfaces:**
- Consumes: nothing.
- Produces: row 3 no longer exists. Rows 1, 1c and 2 are the only stops. Rows 1b, 4 and 5 are the only rows returning to Step 1. Later tasks may rely on there being no cap and on `iteration` being a label with no stop rule reading it.

- [ ] **Step 1: Confirm the current text of all eight sites**

Run:
```
rg -n -U 'iteration` reached 10|The loop\s+always terminates|10-iteration cap as usual|row 2 or\s+row 3 stop|A row 2 or row 3\s+stop|Stopped after 10 iterations|stops on\s+row 1 or row 2' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: hits at 541, 353-354, 362, 699-700, 717-718, 724, 605-606. If any is missing or reads differently from the text quoted below, STOP and report drift.

- [ ] **Step 2: Delete loop-control table row 3**

Delete this entire line at 541. Do NOT renumber rows 4 and 5.
```
| 3 | `iteration` reached 10                                                     | **Stop** — proceed to Final Report (include limit notice)     |
```

- [ ] **Step 3: Insert the prohibition note under the table**

Insert this immediately before the existing line `Computing the next active set, for every row that goes back to Step 1 (1b, 4, and 5):`, followed by a blank line.

```
**There is no iteration cap, and the number column is a label column, not an index.** Rows 1, 1c,
and 2 are the only stops. Rows 1b, 4, and 5 are the only rows that go back to Step 1. Row 1b is
bounded at one retry per reviewer kind per run, and rows 4 and 5 both require `any_commit == true`,
because row 2 stops the run the moment an iteration commits nothing. So the loop continues only
while every single iteration commits at least one fix. That is real progress on almost every run.
It is not a termination proof. One iteration's fixes can introduce a defect the next iteration then
finds, and that cycle can sustain itself. A run that is going nowhere is stopped by the user
interrupting it, which is why every iteration emits a per-iteration summary. `iteration` is a label
for the announce line, that summary, and the Final Report header. No stop rule reads it. Do not add
a cap row, an iteration count of any size, a periodic check-in, or an oscillation detector, and do
not renumber the rows that remain.
```

- [ ] **Step 4: Repair the always-terminates absolute at 353-354**

This is the site no keyword search finds. It holds no digit and no cap keyword.

Replace:
```
  outcome, naming the reviewer and what went unreviewed. Both properties hold at once. The loop
  always terminates, and it never claims a clean result it did not earn.
```
With:
```
  outcome, naming the reviewer and what went unreviewed. Both properties hold at once. A lost
  reviewer can never block the loop from stopping, and the loop never claims a clean result it did
  not earn.
```

- [ ] **Step 5: Drop the cap sentence from the relaunch bullet at 359-362**

Replace:
```
  has retry budget, never relaunch a kind already in
  `reviewer_unavailable`, and let row 1c terminate the run once a kind is `unavailable`. A relaunch
  iteration counts against the 10-iteration cap as usual.
```
With:
```
  has retry budget, never relaunch a kind already in `reviewer_unavailable`, and let row 1c
  terminate the run once a kind is `unavailable`.
```

- [ ] **Step 6: Add row 1c and the interrupt caveat to the pruning-safety argument at 604-612**

Replace:
```
ever skipped while the logic it already approved is unchanged, and by the time the loop stops on
row 1 or row 2 every logic reviewer that was still available has validated the final logic. A kind
in `reviewer_unavailable` validated nothing, so a run that stops with one reports `partial`
coverage instead.
```
With:
```
ever skipped while the logic it already approved is unchanged, and by the time the loop stops on
row 1, row 1c, or row 2 every logic reviewer that was still available has validated the final
logic. An interrupted run carries no such guarantee, because an interrupt can land right after a
logic-change commit. A kind in `reviewer_unavailable` validated nothing, so a run that stops with
one reports `partial` coverage instead.
```
Adding row 1c also corrects a pre-existing omission. Row 1c is a stop and was not listed.

- [ ] **Step 7: Remove the row-3 reference from the Coverage prose at 698-701**

Replace:
```
reported since. The retry union above relaunches such a kind on the next pass, so the only way one
survives to the end of the run is a stop that fires before the retry can happen, which is a row 2 or
row 3 stop. Report that as `partial`, because that kind's lenses really were not applied to the final
tree.
```
With:
```
reported since. The retry union above relaunches such a kind on the next pass, so the only way one
survives to the end of the run is a stop that fires before the retry can happen. That is a row 2
stop, and nothing else. Report it as `partial`, because that kind's lenses really were not applied
to the final tree.
```

- [ ] **Step 8: Remove the row-3 reference from the incomplete-coverage outcome prose at 717-719**

Replace:
```
that stopped the run. Row 1c is the empty-findings case with an `unavailable` kind. A row 2 or row 3
stop uses this outcome too whenever Coverage is `partial`, adding that nothing was committed or that
the iteration cap was reached.
```
With:
```
that stopped the run. Row 1c is the empty-findings case with an `unavailable` kind. A row 2 stop
uses this outcome too whenever Coverage is `partial`, adding that nothing was committed.
```

- [ ] **Step 9: Delete the unreachable row-3 outcome branch at 723-724**

Delete both lines, the separator and the outcome:
```
— OR —
⚠️ Stopped after 10 iterations. See remaining issues above.
```

- [ ] **Step 10: Verify no row-3 or termination-guarantee claim survives**

Run:
```
rg -n -U 'row 3|iteration` reached|always terminates|10-iteration|Stopped after 10' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: zero hits.

Run:
```
rg -n -U '\| 4 \||\| 5 \||rows 4 and 5|\(1b, 4, and 5\)' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: rows 4 and 5 still present with their original numbers, and the references to them intact.

- [ ] **Step 11: Commit**

Write the message to a unique scratch file, then commit. Never pipe or chain a git command.
```
git add shared_config/.claude/skills/review-and-fix/SKILL.md
git commit -F /private/tmp/claude/cap-t1.txt
```
Message body to write to that file:
```
review-and-fix: remove the iteration cap and repair the claims it propped up

Deletes loop-control row 3. Rows 1, 1c and 2 are now the only stops, and
rows 1b, 4 and 5 the only rows that return to Step 1. Rows 4 and 5 keep
their numbers, because eleven lines reference them by number.

The loop now continues only while every iteration commits at least one
fix, since row 2 stops it the moment an iteration commits nothing. That is
progress on almost every run and it is not a termination proof. One
iteration's fixes can introduce a defect the next iteration finds. A run
going nowhere is stopped by the user interrupting it.

Six claims depended on the bound and are repaired here. The important one
is "The loop always terminates", which was true only because row 3
existed, and which holds no digit and no cap keyword so no search for the
cap surfaces it. Two dangling row-3 references inside decision procedures
are gone, as is an unreachable outcome branch. The pruning-safety argument
gains row 1c, which was a pre-existing omission, and the caveat that an
interrupted run carries no such guarantee.
```

---

### Task 2: Reverse the five arguments that leaned on the bound

Each of these derives its force from the loop being bounded. Deleting the number alone would read as SOFTENING the rule each one protects. Each must be reworded to lean on the ABSENCE of a bound, which makes every one of them stronger.

**Files:**
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md` (sites at 192-194, 225-228, 347-348, 591-592, 787-789)

**Interfaces:**
- Consumes: Task 1's removal of the cap.
- Produces: nothing later tasks depend on.

- [ ] **Step 1: Confirm the current text of all five sites**

Run:
```
rg -n -U 'up to 10 times|in a \*loop\*, up to 10|still permit ten relaunches|recreates the ten-retry loop|multiplied by up to 10 loop' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: five hits. If any reads differently from the text quoted below, STOP and report drift.

- [ ] **Step 2: Reverse the gh-style 1x cost argument at 192-194**

Replace:
```
Discussion Context, which in-depth cannot produce at all. This matters more here than in
`pr-review`. This skill re-runs its fan-out up to 10 times, so a redundant instance is paid per
iteration. Do not raise gh-style back to parity, and do not drop it to zero.
```
With:
```
Discussion Context, which in-depth cannot produce at all. This matters more here than in
`pr-review`, which fans out once. This skill re-runs its fan-out on every iteration and nothing
caps the iteration count, so a redundant instance is paid again on every pass. Do not raise
gh-style back to parity, and do not drop it to zero.
```

- [ ] **Step 3: Reverse the effort-pinning argument at 225-228**

Replace:
```
unset effort **inherits the session effort**. This skill runs its fan-out in a *loop*, up to 10
iterations, so an inherited `xhigh` is multiplied by every iteration. Pinning it in the agent
definition is the only way to fix both knobs together.
```
With:
```
unset effort **inherits the session effort**. This skill runs its fan-out in a *loop* with no cap
on the iteration count, so an inherited `xhigh` is multiplied by however many iterations the run
takes. Pinning it in the agent definition is the only way to fix both knobs together.
```

- [ ] **Step 4: Reverse the retry-budget argument at 347-348**

Replace:
```
  The retry budget is **one per reviewer per RUN, not per iteration.** A per-iteration budget would
  still permit ten relaunches across ten iterations, which is the bug with extra bookkeeping.
```
With:
```
  The retry budget is **one per reviewer per RUN, not per iteration.** A per-iteration budget would
  grant a fresh relaunch on every pass, and nothing caps the iteration count, so it would permit
  unbounded relaunches. That is the same bug with extra bookkeeping.
```

- [ ] **Step 5: Reverse the `reviewer_retries` state note at 591-592**

Replace:
```
  between iterations. Resetting it recreates the ten-retry loop.
```
With:
```
  between iterations. Resetting it turns a per-run budget into a per-iteration one, and nothing
  caps the iteration count, so the relaunches become unbounded.
```

- [ ] **Step 6: Reverse the Constraints model-and-effort policy at 787-789**

Replace:
```
  tracks whatever the user last set with `/effort` — multiplied by up to 10 loop iterations. The
  fix step (Step 2) — reading, editing, lint/test, committing — stays on the **session model**,
```
With:
```
  tracks whatever the user last set with `/effort`. Nothing caps the iteration count, so either
  mistake is paid again on every pass. The fix step (Step 2) — reading, editing, lint/test,
  committing — stays on the **session model**,
```

- [ ] **Step 7: Verify all five reversed, and none softened**

Run:
```
rg -n -U 'nothing\s+caps the iteration count|no cap\s+on the iteration count|Nothing caps the iteration count' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: five hits, one per site.

Read each of the five passages and confirm the argument now reads as STRONGER without a bound, not weaker. A rewrite that merely drops the number is a failed step.

- [ ] **Step 8: Commit**
```
git add shared_config/.claude/skills/review-and-fix/SKILL.md
git commit -F /private/tmp/claude/cap-t2.txt
```
Message body:
```
review-and-fix: point the five bounded-loop arguments at the missing bound

Five rules justified themselves by the loop running at most ten times. The
gh-style 1x split, effort pinning and its Constraints twin, and the retry
budget and its state-note twin. Removing the cap makes every one of them
stronger, since an unbounded loop is a worse place to pay for a redundant
instance, to inherit expensive effort, or to hand out a per-iteration retry
budget.

Each now leans on the absence of a bound. Deleting the number alone would
have read as softening the rule it protects.
```

---

### Task 3: Remove the cap from the descriptive prose

Mechanical removals with no argument to preserve. Includes one list renumber.

**Files:**
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md` (sites at 16-19, 39-42, 48-50, 112-117, 664)

**Interfaces:**
- Consumes: Task 1's removal of the cap.
- Produces: Process Overview gains a step 7 that names the per-iteration summary emission, which Task 4 defines.

- [ ] **Step 1: Confirm the current text of all five sites**

Run:
```
rg -n -U 'or after 10 iterations|iteration budget|or 10 iterations\.|OR 10\s+iterations are reached|Iterations completed:\*\* N / 10' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: five hits at 18, 40, 50, 115-116, 664. If any reads differently, STOP and report drift.

- [ ] **Step 2: Frontmatter description at 16-19**

Replace:
```
  summary. The loop stops as soon as an iteration's active reviewers find nothing actionable,
  or an iteration commits nothing, or a reviewer kind is unavailable and the findings list is
  empty (which ends the run with partial coverage), or after 10 iterations. No GitHub write
  commands are ever issued. Produces a final summary report.
```
With:
```
  summary. The loop stops as soon as an iteration's active reviewers find nothing actionable,
  or an iteration commits nothing, or a reviewer kind is unavailable and the findings list is
  empty (which ends the run with partial coverage). No GitHub write commands are ever issued.
  Produces a final summary report.
```
Continuation lines under `description: >` must stay indented exactly 2 spaces. A 1-space or 3-space line silently turns the folded scalar into a literal block and breaks skill selection.

- [ ] **Step 3: Numbered loop summary item 3 at 39-42**

"iteration budget" names a resource that stops existing, so a reader hunting for it lands back at the cap.

Replace:
```
3. Applies the orchestrator's own **`confidence >= 50`** filter — more permissive than the
   sub-skills' default of 70 because we want to spend the fix loop's iteration budget on
   moderately-confident findings too. The triangulation gives us enough cross-pass
   evidence that 50–69 findings are worth attempting.
```
With:
```
3. Applies the orchestrator's own **`confidence >= 50`** filter. It is more permissive than the
   sub-skills' default of 70 because the fix loop attempts moderately-confident findings too.
   The triangulation gives us enough cross-pass evidence that 50–69 findings are worth
   attempting.
```

- [ ] **Step 4: Numbered loop summary item 6 at 48-50**

Replace:
```
6. Loops until the findings list is empty with every launched reviewer kind reported and none
   `unavailable`, or an iteration commits nothing, or a kind is `unavailable` with an empty
   findings list (which stops with partial coverage), or 10 iterations.
```
With:
```
6. Loops until the findings list is empty with every launched reviewer kind reported and none
   `unavailable`, or an iteration commits nothing, or a kind is `unavailable` with an empty
   findings list (which stops with partial coverage).
```

- [ ] **Step 5: Process Overview steps 6 and 7 at 112-117**

Replace:
```
6. Decide the next iteration's active set (full rerun / pruned / stop) and repeat from step 2
   until the findings list is empty with every launched reviewer kind reported and none
   `unavailable` (row 1, clean), or the findings list is empty with a kind `unavailable`
   (row 1c, which stops with partial coverage), or an iteration commits nothing, OR 10
   iterations are reached
7. Deliver a final summary report
```
With:
```
6. Decide the next iteration's active set (full rerun / pruned / stop) and repeat from step 2
   until the findings list is empty with every launched reviewer kind reported and none
   `unavailable` (row 1, clean), or the findings list is empty with a kind `unavailable`
   (row 1c, which stops with partial coverage), or an iteration commits nothing
7. Emit the per-iteration summary before the loop returns to step 2 or falls through to the
   report
8. Deliver a final summary report
```
Renumbering step 7 to 8 is safe. Nothing cross-references these list items by number. The only internal reference is "repeat from step 2" on the line above, which is unaffected.

- [ ] **Step 6: Final Report header at 664**

Replace `**Iterations completed:** N / 10` with `**Iterations completed:** N`

- [ ] **Step 7: Verify**

Run:
```
rg -n -U '10 iterations|N / 10|iteration budget' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: zero hits.

Run:
```
rg -n '^  [^ ]' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Read the first 25 hits and confirm every frontmatter continuation line is at exactly 2 spaces of indent.

- [ ] **Step 8: Commit**
```
git add shared_config/.claude/skills/review-and-fix/SKILL.md
git commit -F /private/tmp/claude/cap-t3.txt
```
Message body:
```
review-and-fix: drop the cap from the descriptive prose

Removes the cap from the frontmatter description, both numbered loop
summaries, the Process Overview, and the Final Report header. Also drops
the phrase "the fix loop's iteration budget", which named a resource that
stops existing and would send a reader hunting back for the cap.

Process Overview gains a step for emitting the per-iteration summary, and
its final step renumbers from 7 to 8. Nothing cross-references those list
items by number.
```

---

### Task 4: Define the per-iteration summary

The phrase appears about ten times as a destination for information and is never defined. Five call sites route information that exists in NO other artifact. With no cap, a hand interrupt is the normal way a long run ends, and these blocks plus the commits are then the only record.

**Files:**
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md` (insert after the second worked example and before `## Step 4: Final Report`; plus the rename at 307-308)

**Interfaces:**
- Consumes: Task 1's removal of the cap, which the new section's second paragraph references.
- Produces: a defined `### Per-iteration summary` subsection at the end of Step 3, whose Discussion Context rule (counts plus delta) Task 5 site 18 depends on.

- [ ] **Step 1: Confirm the insertion point and the rename site**

Run:
```
rg -n -U 'retry, reached by a different row|## Step 4: Final Report|keep the per-instance detail for the summary' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: the worked-example closing line, the Step 4 heading immediately after it, and the rename site at 307-308.

- [ ] **Step 2: Rename the ambiguous reference at 307-308**

"the summary" becomes ambiguous between two artifacts once one of them is defined. This is the ONLY one of the ten call sites that needs renaming. The others read "in the per-iteration summary" already and resolve on their own once the section exists.

Replace:
```
fell short in `reviewers_missing`, keep the per-instance detail for the summary, and union the
`roles_missing` arrays the in-depth-review instances report.
```
With:
```
fell short in `reviewers_missing`, keep the per-instance detail for the per-iteration summary, and
union the `roles_missing` arrays the in-depth-review instances report.
```

- [ ] **Step 3: Insert the new subsection**

Insert this after the worked-example line ending `retry, reached by a different row. That is the path the union generalizes to every other row.` and its following blank line, and BEFORE `## Step 4: Final Report`. It must be a `###` subsection of Step 3, not a `## Step 3.5`. The loop-control table's actions say "go to Step 1", which jumps past a step placed after Step 3, so the emission has to be part of Step 3.

```
### Per-iteration summary

Every iteration ends by emitting one summary block to chat. Emit it last in Step 3, after the table
has picked a row and after the union and the subtraction have computed the next active set. That is
the earliest point at which every item below exists. The table's "go to Step 1" and "proceed to
Final Report" actions both happen after the block is out. Never batch two iterations into one
block, and never hold a block back to the end of the run.

There is no iteration cap, so the user stops a run that is not converging by interrupting it, and an
interrupted run never reaches Step 4. These blocks plus the commits are then the only record of what
the run did. Emit it to chat, the same as the iteration-start announcement. Do not write it to a
file in the working tree, because Step 2 commits with `git add -A` and would sweep that file into
the next fix commit.

Cover these in this order, one line each, and drop any line that has nothing to say:

- The `iteration` number and the active reviewer set that ran.
- Which kinds reported and which fell short, keeping the per-instance detail, plus the unioned
  `roles_missing` and the retry or `unavailable` state of any short kind. A shortfall that a later
  relaunch cleared is recorded here and nowhere else.
- Each finding kept by the `>=50` filter with its outcome, meaning the short commit hash, or
  deferred, or dismissed, or abandoned because lint or tests failed.
- Unfiltered leads, naming the instance and which rule excluded them (`scoring.complete` false,
  `unscored`, or `citation_verified` false).
- `any_logic_change` for the iteration.
- Discussion Context in PR mode, as resolved and unaddressed counts plus whatever changed since the
  previous iteration. Say so instead when gh-style was not active and the previous snapshot carries
  forward unchanged.
- Any anomaly with no other home, such as a `subagent_type` that did not resolve and left effort
  inherited.
- The row that fired, and either the next iteration's active set or the stop.

Keep it terse. The loop is uncapped, so this cost is paid on every iteration, and the Final Report
carries the detail. An interrupt during the fix phase leaves that iteration with no block at all.
The commits are already in `git log`, one per fix. Do not reconstruct a Final Report for an
interrupted run, and do not relaunch reviewers to recover state that was never printed.
```

- [ ] **Step 4: Verify the section landed in the right place and covers the orphaned call sites**

Run:
```
rg -n '^### Per-iteration summary|^## Step 4: Final Report|^## Step 3' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: `### Per-iteration summary` appears AFTER the Step 3 heading and immediately BEFORE `## Step 4: Final Report`. There must be no `## Step 3.5`.

Run:
```
rg -c 'per-iteration summary' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: at least 11 hits, meaning the original references plus the new section.

Confirm by reading that the new section's bullets cover all five otherwise-homeless items: the per-instance shortfall detail, a cleared shortfall, unfiltered leads, the effort-not-pinned note, and intermediate Discussion Context.

- [ ] **Step 5: Commit**
```
git add shared_config/.claude/skills/review-and-fix/SKILL.md
git commit -F /private/tmp/claude/cap-t4.txt
```
Message body:
```
review-and-fix: define the per-iteration summary

The phrase appeared about ten times as a destination for information and
was never defined anywhere. No section said it had to be emitted or what
it contained. Five call sites route information that exists in no other
artifact: the per-instance shortfall detail, a shortfall a later relaunch
cleared (which Coverage is explicitly forbidden to report), unfiltered
leads, the effort-not-pinned note, and intermediate Discussion Context
that the Final Report already pointed back at.

It matters more now that the loop is uncapped, because a hand interrupt is
the normal way a long run ends and an interrupted run never reaches Step 4.
These blocks plus the commits are then the whole record.

Placed as the last subsection of Step 3 rather than a Step 3.5, because the
loop-control table says "go to Step 1" and would jump past a step placed
after Step 3. Specified as an ordered field list rather than a literal
template, because most fields are conditional and a template would need an
omit-rule per field. Discussion Context is counts plus a delta, since it is
the only field whose size grows with the PR's comment count.
```

---

### Task 5: Re-gate the Final Report's Remaining Issues section

With the cap gone, gating a report section on "if iteration limit reached" is unreachable. Remaining unfixed findings are still real, most commonly on a row 2 stop. This task also fixes a pre-existing contradiction in the Converged branch.

**Files:**
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md` (sites at 674, 688-690, 721-722, and an insertion after the Outcome-selection paragraph)

**Interfaces:**
- Consumes: Task 4's per-iteration summary and its Discussion Context delta rule.
- Produces: nothing.

- [ ] **Step 1: Confirm the current text and locate the fenced block boundaries**

Run:
```
rg -n -U 'Remaining Issues \(if iteration limit reached\)|available in the\s+per-iteration logs|Nothing left to\s+fix|For the \*\*Tickets examined\*\* section' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: four hits.

Run:
```
rg -n '^```' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Note the fence pair that brackets the Final Report template. **The gating prose in Step 5 of this task must land OUTSIDE that pair.** Two design proposals placed it inside, which would make the orchestrator reproduce the instructions verbatim into the report it posts.

- [ ] **Step 2: Re-gate the Remaining Issues heading at 674**

The gate moves into the heading, matching how the Discussion Context heading is gated a few lines below.

Replace:
```
### Remaining Issues (if iteration limit reached)
```
With:
```
### Remaining Issues (omit when every surviving finding was fixed)
```

- [ ] **Step 3: Reword the Discussion Context footnote at 688-690**

This also fixes the file's only use of the name "per-iteration logs" against ten uses of "per-iteration summary".

Replace:
```
(This reflects the LATEST iteration's gh-style-review snapshot. If the loop ran many
iterations, intermediate snapshots are not reproduced. They're available in the
per-iteration logs above.)
```
With:
```
(This reflects the LATEST iteration's gh-style-review snapshot. If the loop ran many
iterations, intermediate snapshots are not reproduced here. The per-iteration summaries
above track them as counts plus what changed each iteration.)
```

- [ ] **Step 4: Fix the Converged branch at 721-722**

"Nothing left to fix" is already wrong today. Rows 1, 1b and 1c intercept every empty findings list, so row 2 fires only when findings existed and none were committed.

Replace:
```
✅ Converged — the last iteration committed no changes and Coverage is `complete`. Nothing left to
fix. Done.
```
With:
```
✅ Converged — the last iteration committed no changes and Coverage is `complete`. Every finding it
surfaced was deferred, dismissed, or left unfixed, and Remaining Issues lists them. Done.
```

- [ ] **Step 5: Insert the Remaining Issues gating prose AFTER the fence**

Insert this immediately before the existing paragraph beginning `For the **Tickets examined** section:`, followed by a blank line. Confirm from Step 1 that this position is outside the fenced template.

```
For the **Remaining Issues** section: emit it whenever a finding that passed the `>=50` filter did
not become a commit. The row 2 stop is the common case, since findings existed and nothing was
committed, so every one of them is still open. A fix abandoned because lint or tests failed belongs
here too. Deferred and dismissed ticket findings do not. They are listed under Tickets examined
with their decision. Omit the section when every surviving finding became a commit.
```

Unfiltered leads do NOT go in this section. The human ruled they stay in the per-iteration summary only, because the file deliberately keeps leads out of the fix path and routing them to the report would be a behaviour change beyond this scope.

- [ ] **Step 6: Verify, including that nothing landed inside the fence**

Run:
```
rg -n -U 'iteration limit reached|per-iteration logs|Nothing left to\s+fix' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: zero hits.

Run:
```
rg -n 'For the \*\*Remaining Issues\*\* section' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: one hit. Note its line number.

Run:
```
rg -n '^```' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: the new paragraph's line number from the previous command must fall OUTSIDE the fence pair that brackets the Final Report template. Then read the 5 lines around it and confirm it sits next to the Tickets examined paragraph, after the closing fence.

- [ ] **Step 7: Commit**
```
git add shared_config/.claude/skills/review-and-fix/SKILL.md
git commit -F /private/tmp/claude/cap-t5.txt
```
Message body:
```
review-and-fix: re-gate Remaining Issues on content, not on the cap

The section was gated on "if iteration limit reached", which is
unreachable now the cap is gone. Unfixed findings are still real, most
often on a row 2 stop, where findings existed and nothing was committed.
It now emits whenever a finding that cleared the >=50 filter did not
become a commit, with the gate in the heading and the rule stated after
the template fence rather than inside it.

Also fixes the Converged branch, which claimed "Nothing left to fix" on
exactly the stop that can only fire when findings were left unfixed. And
reworks the Discussion Context footnote, which promised intermediate
snapshots were reproduced in the per-iteration logs. They are tracked as
counts plus a delta, and "per-iteration logs" was the file's only use of
that name against ten uses of "per-iteration summary".
```

---

## Final verification, after all five tasks

- [ ] **Step 1: No trace of the removed mechanism**

```
rg -n -U '10 iteration|iteration.*10|ten-retry|iteration limit|/ 10|row 3|iteration cap as usual' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: the only hits mention the ABSENCE of a cap, in the table note and the reversed arguments. Zero hits describing a cap that exists.

- [ ] **Step 2: Rows 4 and 5 intact**

```
rg -n -U 'rows 4 and 5|\(1b, 4, and 5\)|not just rows 4 and 5|Row 4 \(full\)|Row 5 \(pruned\)' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: all eleven referencing lines still resolve to existing rows.

- [ ] **Step 3: Frontmatter still parses**

```
rg -n '^---$' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: the opening and closing fence of the frontmatter block. Read lines 1 to the closing fence and confirm every continuation line under `description: >` is indented exactly 2 spaces.

- [ ] **Step 4: The read that the greps cannot replace**

Read the whole file top to bottom. Look for any remaining claim whose truth depends on the loop being bounded. Step 1's grep CANNOT find sentences like the former "The loop always terminates", because they contain no digit and no cap keyword. That sentence is exactly why this step exists and is not optional. Four separate defects in this file have escaped line-oriented greps.

Report anything found rather than fixing it silently.

## Self-review of this plan

**Spec coverage.** All 24 spec sites are assigned: Task 1 takes 9, 10, 11, 12, 14, 19, 20, 22 (8 sites). Task 2 takes 5, 6, 8, 13, 24 (5). Task 3 takes 1, 2, 3, 4, 16 (5). Task 4 takes 7, 15 (2). Task 5 takes 17, 18, 21, 23 (4). Total 24. The spec's three accepted tradeoffs are carried into Task 4's inserted text and Task 5's Step 5 note. The spec's four verification requirements are the Final Verification section.

**Placeholder scan.** Every step carries the exact text to write or delete. No "TBD", no "add appropriate handling", no "similar to Task N".

**Consistency.** Site 21 moved from Task 1 into Task 5 because its replacement text points at a re-gated Remaining Issues section, so the two must land together. Site 18 sits in Task 5 rather than Task 3 because it points at Task 4's Discussion Context delta rule. Both dependencies are recorded in the Interfaces blocks.

**Known intermediate state.** Task 1 inserts a note referencing "a per-iteration summary" that Task 4 defines. Flagged at the top of this plan so a reviewer gating Task 1 does not reject it. It is no worse than the status quo, where the file already references that artifact about ten times without defining it.
