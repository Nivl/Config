# Remove the review-and-fix iteration cap

**Date:** 2026-07-31
**File under change:** `shared_config/.claude/skills/review-and-fix/SKILL.md` (795 lines, single file)

## Goal

Delete the 10-iteration cap from the `review-and-fix` loop, with no replacement bound of any kind.
Repair every claim elsewhere in the file whose truth depended on that bound. Define the
"per-iteration summary", which the file references about ten times and never specifies, because with
no cap a hand interrupt becomes the normal way a long run ends and that summary is then the only
record it leaves.

## Architecture

One markdown file. Its prose IS the program, because an LLM orchestrator reads it top to bottom and
follows it. The correctness test for any sentence is whether two orchestrators reading it would
behave the same way. There is no build and no test suite, so review of the prose is the only gate.

## Global constraints

- Plain ASCII in authored prose. Do not join two independent clauses with an em-dash, an ASCII
  space-hyphen-space, or a clause-splitting colon. Split into sentences. Those forms stay allowed as
  label prefixes, definition dashes at the start of a list item, list introductions, and inside
  literal templates.
- Do NOT renumber loop-control rows 4 and 5. Eleven lines reference them by number: 545, 550, 552,
  559, 573, 590, 605, 607, 637, 638, 653. Renumbering breaks all eleven.
- The Final Report is a fenced code block spanning lines 660 to 725. Instructions must never be
  inserted inside it, because the orchestrator reproduces that block verbatim.
- Use `rg -U` for every verification search. Two cap references straddle a line break and are
  invisible to a line-oriented grep.

## Decisions taken

Three settled by the human during brainstorming:

1. **No ceiling of any kind.** A higher number, a periodic check-in, and an oscillation detector were
   each offered and declined. The run is stopped by the other rows or by the human interrupting.
2. **Unfiltered leads stay in the per-iteration summary only.** They do not reach the Final Report's
   Remaining Issues section. Lines 373 to 382 deliberately keep leads out of the fix path, and
   routing them to the report would be a behaviour change beyond the approved scope.
3. **Discussion Context appears as counts plus what changed since the previous iteration.** Not the
   full pools every iteration. It is the only field whose size grows with the PR's comment count, and
   the loop is now uncapped. This forces a consequent edit at line 690.

Two settled by the design panel:

4. **The PROHIBITION lives in exactly one place**, a note under the loop-control table. That is the
   normative sentence, the one telling a future editor not to add a cap row, a count, a check-in, or
   an oscillation detector. It is not repeated in the frontmatter, the numbered loop summary, or the
   Constraints list. Behaviour is identical whether or not those also assert it, and each extra
   assertion is another place to keep consistent. The table is the one place a future editor would
   re-add a cap row, so that is where the prohibition belongs.

   The FACT that the loop is uncapped may be referenced where it explains behaviour, which it is at
   sites 5, 6, 8, 13, 15 and 24. Those are consequences of the absence rather than restatements of
   the rule, and five of them are the reversed arguments that need it to make sense. Site 15 is the
   one to watch: its opening explains why an interrupted run needs a record, so it references the
   absence rather than legislating it.
5. **The per-iteration summary is one `###` subsection at the end of Step 3**, about 33 lines. Not a
   `## Step 3.5`. That is rejected on control flow rather than taste, because the table's actions say
   "go to Step 1", which jumps past a step placed after Step 3. As Step 3's last subsection, Step 3
   is not finished until the block is out.

## The finding that drives the change

`review-and-fix/SKILL.md:353-354` currently reads:

> Both properties hold at once. The loop always terminates, and it never claims a clean result it did
> not earn.

That claim is true today ONLY because row 3 exists. It becomes false the moment row 3 is deleted. It
contains no digit and no cap keyword, so no search for the cap surfaces it in the same diff. This is
the single most likely site to be left behind, and it is the reason the site count is 24 rather than
the 8 a keyword sweep finds.

Two further claims were verified as already wrong today:

- **Lines 76-78** assert that any logic change forces a full rerun before the loop ends. Rows are
  evaluated in order and row 3 precedes row 4, so a cap stop at iteration 10 can follow an unreviewed
  logic-change commit. Removing the cap REPAIRS this. No edit is needed at that site.
- **Lines 721-722** say "Nothing left to fix" on the row 2 branch. Rows 1, 1b and 1c intercept every
  empty findings list, so row 2 fires only when findings existed and none were committed. The
  sentence is already self-contradictory and the re-gated Remaining Issues section will sit directly
  above it.

## Termination, stated plainly

Without row 3 the loop returns to Step 1 only via rows 1b, 4 and 5. Row 1b is bounded at one retry
per reviewer kind per run across two kinds. Rows 4 and 5 both require `any_commit == true`, because
row 2 stops the run when an iteration commits nothing. So the loop continues only while every single
iteration commits at least one fix.

That is real progress on almost every run. It is not a termination proof. One iteration's fixes can
introduce a defect the next iteration then finds, and that cycle can sustain itself. This is not
hypothetical: in the session that motivated the change, one iteration's fixes introduced three
defects the next iteration found. The accepted answer is that the human interrupts such a run, which
is why every iteration must emit a summary.

## Five arguments reverse direction

These derive their force from the bound. Deleting the number alone would read as softening the rule
each one protects, so each must be reworded to lean on the ABSENCE of a bound.

| line | argument | after the change |
|---|---|---|
| 192-194 | gh-style stays at 1x because a redundant instance is paid per iteration | stronger, nothing caps the count |
| 225-228 | pin effort, because inherited `xhigh` is multiplied per iteration | stronger, paid on every pass |
| 347-348 | retry budget is per run, because per-iteration would permit ten | stronger, would permit unbounded |
| 591-592 | never reset `reviewer_retries` between iterations | stronger, same reason |
| 787-789 | Constraints twin of the effort argument | stronger, same reason |

## The 24 edit sites

Exact current and proposed text for every site. Sites marked INSERT add new text.

### 1. Lines 16-19, frontmatter description

CURRENT:
```
  summary. The loop stops as soon as an iteration's active reviewers find nothing actionable,
  or an iteration commits nothing, or a reviewer kind is unavailable and the findings list is
  empty (which ends the run with partial coverage), or after 10 iterations. No GitHub write
  commands are ever issued. Produces a final summary report.
```
PROPOSED:
```
  summary. The loop stops as soon as an iteration's active reviewers find nothing actionable,
  or an iteration commits nothing, or a reviewer kind is unavailable and the findings list is
  empty (which ends the run with partial coverage). No GitHub write commands are ever issued.
  Produces a final summary report.
```

### 2. Lines 39-42, numbered loop summary item 3

CURRENT:
```
3. Applies the orchestrator's own **`confidence >= 50`** filter — more permissive than the
   sub-skills' default of 70 because we want to spend the fix loop's iteration budget on
   moderately-confident findings too. The triangulation gives us enough cross-pass
   evidence that 50–69 findings are worth attempting.
```
PROPOSED:
```
3. Applies the orchestrator's own **`confidence >= 50`** filter. It is more permissive than the
   sub-skills' default of 70 because the fix loop attempts moderately-confident findings too.
   The triangulation gives us enough cross-pass evidence that 50–69 findings are worth
   attempting.
```
Reason: "iteration budget" names a resource that stops existing, so a reader hunting for it lands
back at the cap.

### 3. Lines 48-50, numbered loop summary item 6

CURRENT:
```
6. Loops until the findings list is empty with every launched reviewer kind reported and none
   `unavailable`, or an iteration commits nothing, or a kind is `unavailable` with an empty
   findings list (which stops with partial coverage), or 10 iterations.
```
PROPOSED:
```
6. Loops until the findings list is empty with every launched reviewer kind reported and none
   `unavailable`, or an iteration commits nothing, or a kind is `unavailable` with an empty
   findings list (which stops with partial coverage).
```

### 4. Lines 112-117, Process Overview steps 6 and 7

CURRENT:
```
6. Decide the next iteration's active set (full rerun / pruned / stop) and repeat from step 2
   until the findings list is empty with every launched reviewer kind reported and none
   `unavailable` (row 1, clean), or the findings list is empty with a kind `unavailable`
   (row 1c, which stops with partial coverage), or an iteration commits nothing, OR 10
   iterations are reached
7. Deliver a final summary report
```
PROPOSED:
```
6. Decide the next iteration's active set (full rerun / pruned / stop) and repeat from step 2
   until the findings list is empty with every launched reviewer kind reported and none
   `unavailable` (row 1, clean), or the findings list is empty with a kind `unavailable`
   (row 1c, which stops with partial coverage), or an iteration commits nothing
7. Emit the per-iteration summary before the loop returns to step 2 or falls through to the
   report
8. Deliver a final summary report
```
Renumbering step 7 to 8 is safe. Nothing cross-references these list items by number. Only line 112
contains an internal "step 2" reference, which is unaffected.

### 5. Lines 192-194, why gh-style-review is 1x

CURRENT:
```
Discussion Context, which in-depth cannot produce at all. This matters more here than in
`pr-review`. This skill re-runs its fan-out up to 10 times, so a redundant instance is paid per
iteration. Do not raise gh-style back to parity, and do not drop it to zero.
```
PROPOSED:
```
Discussion Context, which in-depth cannot produce at all. This matters more here than in
`pr-review`, which fans out once. This skill re-runs its fan-out on every iteration and nothing
caps the iteration count, so a redundant instance is paid again on every pass. Do not raise
gh-style back to parity, and do not drop it to zero.
```

### 6. Lines 225-228, why effort pinning matters here

CURRENT:
```
unset effort **inherits the session effort**. This skill runs its fan-out in a *loop*, up to 10
iterations, so an inherited `xhigh` is multiplied by every iteration. Pinning it in the agent
definition is the only way to fix both knobs together.
```
PROPOSED:
```
unset effort **inherits the session effort**. This skill runs its fan-out in a *loop* with no cap
on the iteration count, so an inherited `xhigh` is multiplied by however many iterations the run
takes. Pinning it in the agent definition is the only way to fix both knobs together.
```

### 7. Lines 307-308, accounting for sub-agents

CURRENT:
```
fell short in `reviewers_missing`, keep the per-instance detail for the summary, and union the
`roles_missing` arrays the in-depth-review instances report.
```
PROPOSED:
```
fell short in `reviewers_missing`, keep the per-instance detail for the per-iteration summary, and
union the `roles_missing` arrays the in-depth-review instances report.
```
Reason: "the summary" is genuinely ambiguous between the two artifacts once one of them is defined.
This is the only one of the ten call sites that needs renaming.

### 8. Lines 347-348, retry budget argument

CURRENT:
```
  The retry budget is **one per reviewer per RUN, not per iteration.** A per-iteration budget would
  still permit ten relaunches across ten iterations, which is the bug with extra bookkeeping.
```
PROPOSED:
```
  The retry budget is **one per reviewer per RUN, not per iteration.** A per-iteration budget would
  grant a fresh relaunch on every pass, and nothing caps the iteration count, so it would permit
  unbounded relaunches. That is the same bug with extra bookkeeping.
```

### 9. Lines 353-354, the always-terminates absolute

CURRENT:
```
  outcome, naming the reviewer and what went unreviewed. Both properties hold at once. The loop
  always terminates, and it never claims a clean result it did not earn.
```
PROPOSED:
```
  outcome, naming the reviewer and what went unreviewed. Both properties hold at once. A lost
  reviewer can never block the loop from stopping, and the loop never claims a clean result it did
  not earn.
```
This is the site the keyword sweep cannot find. The narrowed claim is still true and is still the
property the surrounding bullet is arguing for.

### 10. Lines 359-362, relaunch bullet last sentence

CURRENT:
```
  has retry budget, never relaunch a kind already in
  `reviewer_unavailable`, and let row 1c terminate the run once a kind is `unavailable`. A relaunch
  iteration counts against the 10-iteration cap as usual.
```
PROPOSED:
```
  has retry budget, never relaunch a kind already in `reviewer_unavailable`, and let row 1c
  terminate the run once a kind is `unavailable`.
```
Naive searches miss this because "10-iteration" is hyphenated.

### 11. Line 541, loop-control table row 3

CURRENT:
```
| 3 | `iteration` reached 10                                                     | **Stop** — proceed to Final Report (include limit notice)     |
```
PROPOSED: delete the line entirely. Do NOT renumber rows 4 and 5.

### 12. INSERT at line 545, between the table and "Computing the next active set"

PROPOSED, inserted before the existing "Computing the next active set" line:
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
This is the single place the absence is stated. It also protects `iteration` at line 579, which
loses its only control-flow consumer and would otherwise read as dead code inviting deletion, or as
a free hook for a new cap.

### 13. Lines 591-592, `reviewer_retries` state note

CURRENT:
```
  between iterations. Resetting it recreates the ten-retry loop.
```
PROPOSED:
```
  between iterations. Resetting it turns a per-run budget into a per-iteration one, and nothing
  caps the iteration count, so the relaunches become unbounded.
```

### 14. Lines 604-612, why pruning is safe without a final full sweep

CURRENT:
```
ever skipped while the logic it already approved is unchanged, and by the time the loop stops on
row 1 or row 2 every logic reviewer that was still available has validated the final logic. A kind
in `reviewer_unavailable` validated nothing, so a run that stops with one reports `partial`
coverage instead.
```
PROPOSED:
```
ever skipped while the logic it already approved is unchanged, and by the time the loop stops on
row 1, row 1c, or row 2 every logic reviewer that was still available has validated the final
logic. An interrupted run carries no such guarantee, because an interrupt can land right after a
logic-change commit. A kind in `reviewer_unavailable` validated nothing, so a run that stops with
one reports `partial` coverage instead.
```
This is the single site where the interrupt caveat lands. Adding row 1c also corrects an existing
omission, since row 1c is a stop and was not listed.

### 15. INSERT at lines 654-656, new subsection at the end of Step 3

PROPOSED, inserted after the second worked example and before `## Step 4: Final Report`:
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

Five call sites route information that exists in no other artifact, which is the evidence this
section is required rather than nice to have. Line 307's per-instance detail is deliberately not
rolled up. A shortfall a later relaunch cleared lives only here, because lines 703 to 707 forbid
Coverage from reporting it. Unfiltered leads never become a commit and never reach Coverage. The
effort-not-pinned note at 222 has no other home. Lines 688 to 690 make the Final Report point back
at this artifact for intermediate Discussion Context, so the file already promises something it
never specified.

### 16. Line 664, Final Report header

CURRENT: `**Iterations completed:** N / 10`
PROPOSED: `**Iterations completed:** N`

### 17. Line 674, Remaining Issues heading

CURRENT: `### Remaining Issues (if iteration limit reached)`
PROPOSED: `### Remaining Issues (omit when every surviving finding was fixed)`

The gate moves into the heading, matching how line 678 gates Discussion Context. The prose goes
after the fence, at site 23. Two panel proposals placed that prose inside the fenced template, which
would have made the orchestrator reproduce the instructions verbatim into the report.

### 18. Lines 688-690, Discussion Context footnote

CURRENT:
```
(This reflects the LATEST iteration's gh-style-review snapshot. If the loop ran many
iterations, intermediate snapshots are not reproduced. They're available in the
per-iteration logs above.)
```
PROPOSED:
```
(This reflects the LATEST iteration's gh-style-review snapshot. If the loop ran many
iterations, intermediate snapshots are not reproduced here. The per-iteration summaries
above track them as counts plus what changed each iteration.)
```
Forced by decision 3. It also fixes the file's only use of the name "per-iteration logs" against ten
uses of "per-iteration summary".

### 19. Lines 698-701, Coverage prose

CURRENT:
```
reported since. The retry union above relaunches such a kind on the next pass, so the only way one
survives to the end of the run is a stop that fires before the retry can happen, which is a row 2 or
row 3 stop. Report that as `partial`, because that kind's lenses really were not applied to the final
tree.
```
PROPOSED:
```
reported since. The retry union above relaunches such a kind on the next pass, so the only way one
survives to the end of the run is a stop that fires before the retry can happen. That is a row 2
stop, and nothing else. Report it as `partial`, because that kind's lenses really were not applied
to the final tree.
```
This names row 3 by number inside a decision procedure, so leaving it would tell an orchestrator to
look for a row that does not exist.

### 20. Lines 717-719, incomplete-coverage outcome prose

CURRENT:
```
that stopped the run. Row 1c is the empty-findings case with an `unavailable` kind. A row 2 or row 3
stop uses this outcome too whenever Coverage is `partial`, adding that nothing was committed or that
the iteration cap was reached.
```
PROPOSED:
```
that stopped the run. Row 1c is the empty-findings case with an `unavailable` kind. A row 2 stop
uses this outcome too whenever Coverage is `partial`, adding that nothing was committed.
```
Second dangling row 3 reference, plus an instruction to report a stop reason that can no longer
occur.

### 21. Lines 721-722, Converged branch

CURRENT:
```
✅ Converged — the last iteration committed no changes and Coverage is `complete`. Nothing left to
fix. Done.
```
PROPOSED:
```
✅ Converged — the last iteration committed no changes and Coverage is `complete`. Every finding it
surfaced was deferred, dismissed, or left unfixed, and Remaining Issues lists them. Done.
```
Fixes a pre-existing contradiction. Row 2 can only fire when findings existed and none were
committed, so "Nothing left to fix" was already wrong, and the re-gated Remaining Issues section
will now sit directly above it.

### 22. Lines 723-724, the row-3 outcome branch

CURRENT:
```
— OR —
⚠️ Stopped after 10 iterations. See remaining issues above.
```
PROPOSED: delete both lines, the separator and the outcome. The branch is unreachable.

### 23. INSERT at lines 735-737, after the Outcome-selection paragraph

PROPOSED, inserted before the existing "For the **Tickets examined** section:" paragraph:
```
For the **Remaining Issues** section: emit it whenever a finding that passed the `>=50` filter did
not become a commit. The row 2 stop is the common case, since findings existed and nothing was
committed, so every one of them is still open. A fix abandoned because lint or tests failed belongs
here too. Deferred and dismissed ticket findings do not. They are listed under Tickets examined
with their decision. Omit the section when every surviving finding became a commit.
```

### 24. Lines 787-789, Constraints model and effort policy

CURRENT:
```
  tracks whatever the user last set with `/effort` — multiplied by up to 10 loop iterations. The
  fix step (Step 2) — reading, editing, lint/test, committing — stays on the **session model**,
```
PROPOSED:
```
  tracks whatever the user last set with `/effort`. Nothing caps the iteration count, so either
  mistake is paid again on every pass. The fix step (Step 2) — reading, editing, lint/test,
  committing — stays on the **session model**,
```

## Accepted tradeoffs

- **A pathological loop can run forever.** Explicitly accepted. The human interrupts it. No count,
  check-in, or oscillation detector is added.
- **An interrupt during the fix phase leaves that iteration with no summary block.** The commits are
  in `git log` with one per fix, so the work is recoverable. That iteration's reviewer shortfalls and
  unfiltered leads are lost. Documented in the new section rather than mitigated.
- **A lead raised in an early iteration lives only in that iteration's block.** Consequence of
  decision 2.

## Verification required after implementation

1. `rg -U` for `10 iteration`, `iteration.*10`, `ten-retry`, `iteration limit`, `/ 10`, `row 3`,
   `cap` across the file. Expect zero hits that refer to the removed mechanism.
2. Confirm rows 4 and 5 keep their numbers, and that all eleven referencing lines still resolve.
3. Confirm no instruction text landed inside the fenced Final Report block spanning roughly 660 to
   725.
4. Read the file top to bottom for any remaining claim that depends on the loop being bounded. The
   grep in step 1 cannot find sentences like the one at 353-354, which is exactly why that step is
   not sufficient on its own.

## Out of scope

- Line 92 references "Step 0.5", which is not a heading. PR detection is item 5 of Step 0.
  Pre-existing and unrelated to the cap.
- The repo-root `AGENTS.md` em-dash sweep and the clause-splitting-colon audit, both carried from
  earlier work.
