# Per-iteration summary format

Every iteration that reaches Step 3 ends by emitting one summary block to chat. A row 0 abort
emits none, because nothing was reviewed and the abort message is the whole record. Emit the block
last in Step 3, after the table has picked a row and after the union and the subtraction have
computed the next active set. That is the earliest point at which the ROW and the NEXT ACTIVE SET
exist. The table's "go to Step 1" and "proceed to Final Report" actions both happen after the block
is out. Never batch two iterations into one block, and never hold a block back to the end of the
run.

**The stamps are not among the items this block waits for.** Step 1 and Step 2 already appended
each one to the log as it was taken, so the block is ASSEMBLED from values already on disk rather
than authored here. Read them back. Do not retype them from memory, and do not treat block-close as
the moment a stamp comes into existence. An earlier version of this paragraph said the block-close
was the earliest point at which every item existed, which read as licence to author `t0` at
block-close, and that is exactly the pattern found in nine measured stamp pairs.

There is no iteration cap, so the user stops a run that is not converging by interrupting it, and
an interrupted run never reaches Step 4. Emit the block to chat, the same as the iteration-start
announcement, and append the identical text to `run_log_path` as you emit it. Step 0 opened that
file outside the repo under review. Do not write it to a file in the working tree instead, because
Step 2 commits with `git add -A` and would sweep that file into the next fix commit.

Appending live is what gives an interrupted run a record. That run is often the one worth reading
later, and its only other trace is chat scrollback plus `git log`.

Three checks on the stamps, and a block that fails any of them is not emitted until the real values
are recovered.

**Ordering, allowing equality: `previous t2 <= t0 <= t_fix <= t2`.** Iterations are sequential, so
this iteration's `t0` cannot precede the previous iteration's `t2`. Equality is legal and common,
because one iteration ending and the next beginning is a real back-to-back seam. An earlier version
demanded strict inequality and nine measured pairs were exactly equal, so the rule refused seams it
should have accepted.

**`t2` against git, not against a sibling stamp.** `t2` must not precede the author date of the last
commit in this iteration's own commit table. Both values are already in front of you. This is the
check with teeth, because an ordering chain compares stamps to each other and cannot see two stamps
that are wrong together. Two `t2` values in the corpus fail it, one preceding three of its own
commits and one naming a HEAD sixteen minutes in its future, and both satisfied every ordering rule.

**Seconds, or an explicit absence.** `date -u +%FT%TZ` always emits seconds, so a stamp with none was
typed rather than read. Refuse a minute-truncated stamp, a tilde, and any range. A stamp that cannot
be recovered is written as the literal token `unrecorded`, never as an approximation and never as a
pointer to another line. `- t0: see above` appeared in the corpus and pointed at a `t2` that was
never written. An approximation reads as a measurement, and a gap where a field should be cannot be
told apart from a field nobody specified.

**There is no summary table and no ledger row.** The stamps Step 1 and Step 2 appended are the
record, and a reader who wants durations subtracts them. That summary has been specified three ways
now, as an optional Final Report section, then a required one, then a fixed-shape row closing this
block, and all three produced zero rows across seven runs. The pattern across those runs is that a
rule stated inline in `SKILL.md` at the moment of the act gets followed and a rule stated in this
file for later assembly does not, so a fourth wording is not the missing piece. Recovering the
timeline is a grep for a stamp, not a grep for an authored shape.

Cover these in this order, one line each, and drop any line that has nothing to say:

- The `iteration` number and the active reviewer set that ran.
- Which kinds reported and which fell short, keeping the per-instance detail, plus the unioned
  `roles_missing` and the retry or `unavailable` state of any short kind. A shortfall that a later
  relaunch cleared is recorded here and nowhere else.
- The count of findings kept by the `>=50` filter, minus any already recorded in
  `skipped_findings`, so the trend across iterations is visible in one place. A run whose count
  is not falling is the signal to interrupt. Skipped findings are excluded because they persist
  by design and would otherwise hold the count at a floor that reads as a stall.
- Every finding kept by the `>=50` filter that did NOT become a commit, and what happened to
  it, meaning deferred, dismissed, abandoned because lint or tests failed, skipped because no
  test was possible, or moot because an earlier commit this iteration already fixed it.
  Committed findings are the table below instead.
- Unfiltered leads, naming the instance and which rule excluded them (`scoring.complete` false,
  `unscored`, or `citation_verified` false).
- `any_logic_change` and `any_test_change` for the iteration, which are what row 4 and row 5
  read. Both false means every commit was `prose`.
- Waiting time and fixing time, as `t_fix` minus `t0` and `t2` minus `t_fix`. Two numbers, one line.
  A long wait beside a near-zero fix is what a cheap iteration actually costs, and pruning cuts the
  reviewers launched without cutting the wait. Do not compute either from `t1`. `t1` is the last
  arrival and a straggler can arrive after fixing has started, which put two measured iterations at a
  fixing time of `0m00s` while git shows two commits landing inside each of them.
- `self_inflicted_count` out of the iteration's finding count, **always, including when it is
  zero**. These are findings whose target line an earlier commit of this same run wrote. A field
  allowed to go silent at zero cannot be told apart from a field that was dropped, which is what
  happened to this one on both of the first two real logs. Never present it as a reason a finding
  was handled differently, because it never is.
- Discussion Context in PR mode, as resolved and unaddressed counts plus whatever changed since
  the previous iteration. Say so instead when gh-style was not active and the previous snapshot
  carries forward unchanged.
- Any anomaly with no other home, such as a `subagent_type` that did not resolve and left effort
  inherited.
- The row that fired, and either the next iteration's active set or the stop.

Then close the block with the iteration's commit table, so the last thing every iteration emits
is the work it actually did:

One row per entry in `iteration_commits`, in commit order. **Five cells are required and the header
wording is yours.** Header the block however reads clearly, and write the rows as a markdown table so
the terminal draws the borders rather than hand-drawing a box, which freezes the column widths at
whatever the template guessed.

The five cells: the **short sha**, the **finding titles** it fixed, **how many findings** the commit
covers, the commit's **class** from sub-step 7, and the **category** with the role numbers it mapped
to from sub-step 7.

`class` and `category` are here because rows 4 and 5 read the first and per-role yield needs the
second, and eleven runs added a `class` column unasked, which is a better signal about what the
table wants than this template was.

**A commit covers as many findings as it covers.** The old text asserted one commit per finding, so
one title per row, and runs batch instead. One measured iteration committed eleven fixes against
about forty-five findings. That assertion is why the table could not be built at all, since a
batched commit has no single title to put in a one-title cell. So the count cell is required. Without
it a reader compares four commits against thirty findings and infers a thirteen percent fix rate
where the real one was near seventy.

Titles stay **verbatim** from the merged finding. That is the guard worth keeping, and it is now the
title cell alone that carries it. The sha is git output and the title is a string a reviewer wrote,
so neither can describe work that was not done. The count, the class and the category are derived,
so they can be wrong without being invented, which is a weaker property and the price of a table that
can actually be built. A paraphrased title would leave this file's ban on invented findings with no
enforcement anywhere. Do not paraphrase, and do not substitute the commit subject. Findings that
produced no commit are the bullet above, not a row here.

When the iteration committed nothing, say so in one line and emit no table. An empty-findings
iteration committed nothing, and neither did a row 2 stop. An empty table frame says less than the
line already does.

Keep it terse. The loop is uncapped, so this cost is paid on every iteration, and the Final Report
carries the detail. An interrupt during the fix phase leaves that iteration with no block at all.
The commits are already in `git log`, one per fix. Do not reconstruct a Final Report for an
interrupted run, and do not relaunch reviewers to recover state that was never printed.
