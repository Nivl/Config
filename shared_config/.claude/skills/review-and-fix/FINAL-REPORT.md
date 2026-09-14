# Final report format

## Contents
- Report template
- Selecting the Outcome line
- Changes Made section
- Remaining Issues section
- Tickets examined section

## Report template

Summarise the entire session in a clear report to the user:

```
## Review and Fix Report

**Target:** PR #<PR>  <-  or  branch range <RANGE>
**Iterations completed:** N
**Total commits made:** N
**Run log:** <run_log_path>
**Ended:** <date -u +%FT%TZ, taken as this report is written>

### Changes Made (omit when the run committed nothing)

| Iteration | Commit | Fix |
|---|---|---|
| Iteration <n> | <short sha> | <the finding's title> |

### Tickets examined
- <id>: ✅ implemented | ⚠️ N gap(s) — <user decision> | ❓ unread

### Remaining Issues (omit when every surviving finding was fixed)
- <finding description> [severity, cross-instance N/M active, sources <in-depth|gh-style|both>, confidence X] — <file:line>
- ...

### Discussion Context (PR mode only; omit entire section if branch mode or both pools empty)

**Resolved by this branch:**
- > <quote> — @<author> ([link](<url>))
  ✅ <resolution>

**Still unaddressed:**
- > <quote> — @<author> ([link](<url>))
  ⚠️ <gap>

(This reflects the LATEST iteration's gh-style-review snapshot. If the loop ran many
iterations, intermediate snapshots are not reproduced here. The per-iteration summaries
above track them as counts plus what changed each iteration.)

### Coverage
complete | partial — `partial` whenever `reviewer_unavailable` is non-empty for the run, OR a kind's
shortfall was still outstanding when the run stopped, OR the unioned `roles_missing` is non-empty.
Consult all three. `reviewer_unavailable` is the per-run set and is the one that survives a reviewer
being dropped from later iterations, so a run that gave up on a reviewer still reports `partial` here
rather than looking complete. A shortfall is **outstanding** when a kind fell short and has not
reported since. The retry union above relaunches such a kind on the next pass, so the only way one
survives to the end of the run is a stop that fires before the retry can happen, meaning a row 2 or a
row 2b stop. Report it as `partial`, because that kind's lenses really were not applied
to the final tree.

Do NOT decide `partial` from the per-iteration `reviewers_missing` on its own. Report a shortfall that
a later relaunch cleared as history in the per-iteration summary, never as a coverage gap. A kind that
fell short once and then reported after its one relaunch left nothing unreviewed, so latching
`partial` on it would demand a sentence about unreviewed lenses that is not true. That is the
difference the "still outstanding" wording draws. A cleared shortfall is history, an uncleared one is
a coverage gap. When partial, name every reviewer kind and role involved and state which lenses the
branch was NOT reviewed against. Never omit this section; its absence reads as complete coverage.

A run that aborted on row 0 never reaches this report. Coverage has no value to report, because
nothing was reviewed.

### Spend
The run total, summed from every `usage` line in the run log, by kind and by model, with the agent
count and the turn count beside the dollar figure, and the most expensive single agent named. One
short block:

```
roles      48 agents  1,990 turns  $236.80  (opus-5)
gh-style    6 agents    318 turns   $41.70  (opus-5)
scorer      6 agents     54 turns    $2.52  (sonnet-5)
implementer  7 agents  520 turns   $22.10  (opus-5)
precommit   7 agents     70 turns    $4.20  (sonnet-5)
total      74 agents  2,952 turns  $307.32  over 6 iterations
most expensive: inst1:role7 iter2, 89 turns, $10.10
```

`$` is the estimate [USAGE.md](USAGE.md) defines, at the rates and date recorded there, and this
section says so in one clause. The orchestrator's own spend is not in it and the block says that
too, because a reader adding this up against a bill needs to know what is missing. Omit the section
only for a row 0 abort, where no agent ran. A run whose `usage` lines are missing reports
`spend: not recorded` rather than a guess, so the gap is visible.

### Severity
The `kept` counts from every iteration's `severity` line, one row per iteration, so the grade of
what the loop found is readable across the run at a glance:

```
iter  critical  major  minor  suggestion   fixed(crit/maj/min/sug)
1        1        6      9       4            1/6/7/2
2        0        2      5       1            0/2/4/0
3        0        0      2       3            0/0/1/0
```

The table is the run's own answer to whether later iterations are still finding what the first one
found, or are down to `minor` and `suggestion` on the run's own commits. Read it beside
`self_inflicted_count` in the iteration summaries. A run whose iterations all lack the `severity`
line reports `severity: not recorded` instead. Omit the section only for a row 0 abort.

### Precommit check
What the `fix-precommit-check` agent caught and what it let through, from the `precommit`,
`self-inflicted` and `commit` lines plus its `usage` lines:

```
checks 31   hits 19   fixed 17   left 2   cost $14.30 (sonnet-5)
misses 4 of 12 self-inflicted findings blamed to a checked commit:
  iter3 Q5  bucket 2  blamed 8776aed7a8 (iter2, precommit hits=0)
  iter4 R3  bucket 3  blamed c623f9d7a0 (iter3, precommit hits=1 fixed=1)
  ...
```

A miss is a `self-inflicted` line whose `blamed=` sha is a commit the agent checked. The join is
`blamed=<sha>` to the `commit iter=` line naming that sha, then that commit's finding id to the
`precommit` line in the same iteration whose `findings=` list contains it. One check runs per batch
and its line carries every committed id in the batch, so a commit is checked when its id is in that
list. A commit with `origin=precommit` is the check's own follow-up and is never a miss. Count a miss whether that check said `hits=0` or reported hits the fix did not include, and say
which. The model in this block comes from the `usage kind=fix-precommit` lines, not from the
`precommit` line's pin, so a pin that drifted from what ran is visible here. The
bucket is the agent's own numbering (1 comments, 2 logging and locks, 3 tests, 4 scope), read from
the finding's category. Findings blamed to iteration-1 commits made before the first check are not
misses and are not counted in the denominator. Misses per bucket over three or four runs are the
tier decision the agent file names. Omit the section when no check ran.

### Outcome
✅ Clean batch — the loop stopped on row 1, so `batch_clean` was true. The final iteration's active
reviewers ALL reported, they found nothing actionable, and Coverage is `complete`. Done.
— OR —
⚠️ Stopped with incomplete coverage — Coverage is `partial`, so this is not a clean result whatever
else the run achieved. Names what made it partial and says what went unreviewed, and names the row
that stopped the run. Row 1c fires on either an `unavailable` kind or a non-empty unioned
`roles_missing`, whether or not findings survived, and names any that did. A row 2 or row 2b stop
uses this outcome too whenever Coverage is `partial`.
— OR —
✅ Converged — the last iteration committed no changes and Coverage is `complete`. Every finding it
surfaced was deferred, dismissed, or left unfixed, and Remaining Issues (or Tickets examined, for
ticket gaps) lists them. Done. This is row 2, and it is row 2 however many findings the last
iteration kept. A report that says "row 1" here is wrong, because row 1 is an empty findings list.
One measured run kept seven findings, committed nothing, and wrote "stopped on row 1" in its report.
`any_commit == false` with findings in hand is this line, never the one above.
— OR —
⚠️ Stopped at the severity floor — row 2b fired. Findings survive and every one is `suggestion`
severity in a non-shipping category. Not a clean result and not converged, because the loop stopped
while it could still have committed. Names the count and points at Remaining Issues. Coverage may be
`complete` here and the outcome still carries no green check, because what ended the run was a
judgement about severity rather than an absence of findings.
— OR —
⚠️ Stopped at the user's direction — no row fired. The loop was still finding real things and could
have continued, and the user was shown where it was going and chose to stop. Names the iteration it
stopped after, names the signal it was shown, and points at Remaining Issues. Carries no green check
at any coverage value, for the same reason row 2b carries none. Nothing here measured an absence of
findings.
```

## Selecting the Outcome line

**A green check requires Coverage to be `complete`.** Key it on Coverage
itself, not on any single one of Coverage's inputs. `reviewer_unavailable` is only one of three things
that force `partial`, so a rule keyed on that variable alone would let a green check sit above a
`partial` Coverage line as soon as another input fired. This governs every stop, not just row 1c.
Whenever Coverage is `partial` the Outcome MUST be the incomplete-coverage line, naming what made it
partial and what went unreviewed. So "✅ Clean batch" is reachable only from row 1 with complete
coverage, and "✅ Converged" only from a row 2 stop with complete coverage. **Row 2b and the
user-directed stop never carry a green check, at any coverage value.** Row 1c can also fire with
findings in hand, so that is not what separates them. What separates those two is that they stop on
a judgement, about severity in one case and about whether to keep going in the other, rather than on
an absence of findings or a gap in coverage. A green check on either would claim a completeness
nothing measured supports. A user-directed stop at `partial` coverage uses the incomplete-coverage
line and says the user ended it, the same way a row 2 or row 2b stop does.

Never pair a green check with `partial` coverage either. The two sections are read together, and a
green check above a `partial` Coverage line is exactly the unearned clean result this machinery
exists to prevent.

## Changes Made section

It is `run_commits`, which is the per-iteration commit tables
concatenated with an `Iteration` column added, in commit order across the run. Every row of every
per-iteration table appears here exactly once, and the `Fix` cell is the same finding title, so
the final table never says anything an iteration did not already show. Repeat the iteration label
on every row instead of blank-filling the repeats, so a row read on its own still names its
iteration. **Total commits made** above must equal this table's row count. **Iterations
completed** can exceed the highest iteration label here, because any iteration that
committed nothing contributes no rows. The labels need not be contiguous either. Omit the
section when `run_commits` is empty. That is a run that committed nothing, and the Outcome
line already says so.

## Remaining Issues section

Emit it whenever a finding that passed the `>=50` filter did
not become a commit. The row 2 stop is the common case, since findings existed and nothing was
committed, so every one of them is still open. A fix abandoned because lint or tests failed belongs
here too. A finding skipped for want of a test belongs here as well, labelled as skipped so a
reader can tell it apart from a fix abandoned because lint or tests failed. Deferred and dismissed
ticket findings do not. They are listed under Tickets examined
with their decision. A finding made moot because an earlier commit this iteration already fixed it
does not either. Nothing about it is still open, and the per-iteration summary already recorded it
as moot. Omit the section when every surviving finding either became a commit, was made moot by
an earlier commit, or was a deferred or dismissed ticket finding.

## Tickets examined section

Omit it entirely when no ticket IDs were found or
`--skip-ticket` was passed. List any deferred or dismissed ticket findings (from
`resolved_ticket_findings`) with their decision. They were not re-prompted in later
iterations. If an in-depth-review sub-agent reported `ticket_review.status` of `denied` or
`unavailable`, note that ticket review did not run and why.
