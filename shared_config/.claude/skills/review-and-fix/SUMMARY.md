# Per-iteration summary format

Every iteration that reaches Step 3 ends by emitting one summary block to chat. A row 0 abort
emits none, because nothing was reviewed and the abort message is the whole record. Emit the block
last in Step 3, after the table has picked a row and after the union and the subtraction have
computed the next active set. That is the earliest point at which every item below exists. The
table's "go to Step 1" and "proceed to Final Report" actions both happen after the block is out.
Never batch two iterations into one block, and never hold a block back to the end of the run.

There is no iteration cap, so the user stops a run that is not converging by interrupting it, and
an interrupted run never reaches Step 4. Emit the block to chat, the same as the iteration-start
announcement, and append the identical text to `run_log_path` as you emit it. Step 0 opened that
file outside the repo under review. Do not write it to a file in the working tree instead, because
Step 2 commits with `git add -A` and would sweep that file into the next fix commit.

Appending live is what gives an interrupted run a record. That run is often the one worth reading
later, and its only other trace is chat scrollback plus `git log`.

**Refuse to emit a block whose stamps fail `previous t2 < t0 < t1 < t2`.** Go back and find the real
values instead. The chain runs across the whole run, not just inside one block. Iterations are
sequential, so this iteration's `t0` cannot precede the previous iteration's `t2`, and a pair that
overlaps means at least one of the two was reconstructed rather than stamped.

Both halves of that chain are there because both have failed. On the first two real logs a `t1`
printed fifteen minutes before its own `t0`, reusing the previous iteration's value, and the block
was emitted anyway with a sentence explaining the reuse. On the third log every `t2` after iteration
1 landed on `:00` seconds and five consecutive iterations started before the previous one ended.
Round seconds are the tell, and an in-block check cannot see a cross-iteration overlap at all.

A stamp that cannot be recovered is recorded as absent, never as an approximation, because an
approximated stamp reads as a measurement.

**Close the block with one ledger row, in exactly this shape:**

```
| iter <n> | <commits in range at t0> | <t1-t0> | <t2-t1> | <findings kept> | <self_inflicted_count> | <commits> |
```

The literal `iter ` prefix is what makes the run's ledger `rg '^\| iter '` over the log. Keep the
cell order fixed and emit a cell for every column, using `-` where a value is genuinely absent, so
the row parses even when a field could not be measured.

**This row is the whole ledger, and nothing assembles a table later.** The Final Report adds a totals
row and reconciles it, and that is all it does with these numbers. Three runs in a row omitted the
old "Where the run went" table, and the third omitted the entire Final Report, because it stopped on
its own judgement and never reached Step 4. A summary that lives only in the report a
non-terminating run never writes is in the wrong place. So the rows are stamped as they are earned
and the table is derived from them, which also keeps this clear of the rule against reconstructing a
Final Report for a run that was cut short.

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
- Waiting time and fixing time, as `t1` minus `t0` and `t2` minus `t1`. Two numbers, one line. A
  long wait beside a near-zero fix is what a cheap iteration actually costs, and pruning cuts the
  reviewers launched without cutting the wait.
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

**Iteration N - M commits**

| Commit | Fix |
|---|---|
| <short sha> | <the finding's title> |

`M` is the row count, and it reads "1 commit" when there is one. One row per entry in
`iteration_commits`, in commit order. Write it as a markdown table and let the terminal draw the
borders. Do not hand-draw a box. A hand-drawn one freezes the column widths at whatever the
template guessed.

`Fix` is the merged finding's `title` verbatim. Step 2 commits one fix per finding, so the
mapping is 1:1 and there is always exactly one title per row. Taking it verbatim is deliberate.
Every cell is then either git output or a string a reviewer wrote, so the table has no room to
describe work that was not done. This file forbids invented findings and invented commits
everywhere else, and a paraphrased `Fix` cell would be the one place that guard is missing. Do
not paraphrase the title, do not substitute the commit subject, and do not add a status column.
Findings that produced no commit are the bullet above, not a row here.

When `M` is 0, emit the header line alone and no table. An empty-findings iteration committed
nothing, and neither did a row 2 stop. An empty table frame says less than the header already
does.

Keep it terse. The loop is uncapped, so this cost is paid on every iteration, and the Final Report
carries the detail. An interrupt during the fix phase leaves that iteration with no block at all.
The commits are already in `git log`, one per fix. Do not reconstruct a Final Report for an
interrupted run, and do not relaunch reviewers to recover state that was never printed.
