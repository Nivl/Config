# review-and-fix: per-iteration and final commit tables

Date: 2026-08-03
Target file: `shared_config/.claude/skills/review-and-fix/SKILL.md` (single file, 888 lines)

## Goal

Every iteration of `review-and-fix` should end by showing the work it did, as a two-column
table of commits. After the loop stops, the Final Report should show the same rows again with
an added iteration column, so the whole run is one scannable ledger.

## Why this is not just a formatting change

The tables need a data source that does not exist today. Step 2 records `any_commit`,
`any_logic_change`, and `productive_reviewers`. It never keeps the commit sha or the finding it
came from, and Step 3 has no per-run commit list. Most of this change is adding those two
accumulators. The templates are the small part.

## Decisions taken

| Decision | Choice | Why |
|---|---|---|
| Table vs. existing bullet block | The table closes the block. The bullets stay. | The bullets carry coverage bookkeeping (`reviewers_missing`, `roles_missing`, unfiltered leads, which loop row fired) that the Final Report's Coverage section and an interrupted run both depend on. Dropping them to make room for the table would lose that. |
| Which bullet changes | Bullet 3 narrows to non-commit outcomes only. | It currently covers commits and non-commits together. The table takes the commits, so leaving the bullet as-is would print every commit twice in the same block. |
| Table syntax | Markdown pipe table. | The terminal renders GFM, so it draws the boxed output already. The file's other three tables are markdown. A hand-drawn box would freeze column widths at whatever the template guessed and would introduce the first box-drawing glyphs into these five files. |
| `Fix` cell content | The merged finding's `title`, verbatim. | Every cell is then either git output or a string a reviewer wrote. The file forbids invented findings and invented commits throughout, and a paraphrased cell would be the one place that guard is missing. |
| Rows for deferred / dismissed / abandoned findings | No. Commits only. | They stay in the narrowed bullet. A status column would leave the `Commit` cell empty on three of four statuses and widen every table for the rare case. |
| Final table vs. `### Changes Made` | Replaces the list under that heading. | Keeping both would list every commit twice in one report. |
| Header separator | ASCII hyphen, `Iteration N - M commits`. | Commit 4a0b06d swept ~160 em-dashes out of these files and left them only where the dash is part of a literal output shape. A hyphen reads identically and will not be re-flagged. |
| Frontmatter description | Unchanged. | "Produces a final summary report" is still accurate. The description is already long. |

## Edits

### Edit 1: Step 2 accumulator reset (currently line 491)

The lead-in sentence says "reset three per-iteration accumulators used by Step 3 to decide the
next active set". The count becomes four, and the framing has to widen because the new
accumulator feeds the table rather than the active-set decision.

Replace the lead-in with:

```
At the start of the iteration's fix phase, reset four per-iteration accumulators. The first
three are what Step 3 reads to decide the next active set. The fourth is what Step 3's commit
table is built from:
```

Add a fourth bullet after `productive_reviewers`:

```
- `iteration_commits` = empty — one `(short sha, finding title)` pair per committed fix, in
  commit order. Step 3's commit table is this list.
```

### Edit 2: Step 2 bookkeeping step 5 (currently line 543)

Widen the step heading, since one of the records now feeds the table:

```
5. **Record what this commit was**, for Step 3's next-active-set decision and its commit table:
```

Add a bullet immediately after `Set any_commit = true`:

```
   - Append the commit's short sha plus the finding's `title` to `iteration_commits`. Take the
     title verbatim from the merged finding. It is the `Fix` cell in both tables.
```

### Edit 3: Step 3 "Track state explicitly" list (currently lines 641-642)

Add two entries after the `any_commit`, `any_logic_change`, `productive_reviewers` entry, so the
Step 2 accumulators stay grouped:

```
- `iteration_commits`: per-iteration; the `(short sha, finding title)` pairs Step 2 appended, in
  commit order. The per-iteration commit table is this list.
- `run_commits`: per-RUN list of `(iteration, short sha, finding title)`. Append
  `iteration_commits` to it as each iteration's summary is emitted. The Final Report's Changes
  Made table is this list. Never reset it between iterations.
```

### Edit 4: per-iteration summary bullet 3 (currently lines 725-726)

Replace:

```
- Each finding kept by the `>=50` filter with its outcome, meaning the short commit hash, or
  deferred, or dismissed, or abandoned because lint or tests failed.
```

with:

```
- Every finding kept by the `>=50` filter that did NOT become a commit, and which it was:
  deferred, dismissed, or abandoned because lint or tests failed. Committed findings are the
  table below instead.
```

### Edit 5: the per-iteration table

Insert between the end of the bullet list and the "Keep it terse" paragraph:

```
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

When `M` is 0, emit the header line alone and no table. A row 2 stop committed nothing, and an
empty table frame says less than the header already does.
```

### Edit 6: Final Report template (currently lines 753-755)

Replace:

```
### Changes Made
- <commit hash (short)>: <commit message>
- ...
```

with:

```
### Changes Made (omit when the run committed nothing)

| Iteration | Commit | Fix |
|---|---|---|
| Iteration <n> | <short sha> | <the finding's title> |
```

### Edit 7: Final Report normative prose

Add alongside the existing "For the **Remaining Issues** section" and "For the **Tickets
examined** section" paragraphs at the end of Step 4:

```
For the **Changes Made** section: it is `run_commits`, which is the per-iteration commit tables
concatenated with an `Iteration` column added, in commit order across the run. Every row of
every per-iteration table appears here exactly once, and the `Fix` cell is the same finding
title, so the final table never says anything an iteration did not already show. Repeat the
iteration label on every row instead of blank-filling the repeats, so a row read on its own
still names its iteration. **Total commits made** above must equal this table's row count.
**Iterations completed** can exceed the highest iteration label here, because a clean iteration
or an all-deferred one contributes no rows. Omit the section when `run_commits` is empty. That
is a run that committed nothing, and the Outcome line already says so.
```

## Rendered result

What the user sees at the end of an iteration, after the existing bullet lines:

```
Iteration 6 - 3 commits
┌────────────┬──────────────────────────────────────────────┐
│   Commit   │                     Fix                      │
├────────────┼──────────────────────────────────────────────┤
│ b452ef9bff │ Non-null assertions used where value can be  │
│            │ undefined                                    │
├────────────┼──────────────────────────────────────────────┤
│ 6f13f09aec │ Deleted branch overwrites deletion timestamp │
├────────────┼──────────────────────────────────────────────┤
│ 699c8bbe14 │ Comment joins two clauses with a dash        │
└────────────┴──────────────────────────────────────────────┘
```

The borders above are the terminal renderer's output, not content in the skill file.

## Edge cases

| Case | Behaviour |
|---|---|
| Iteration committed nothing (row 2 stop, all deferred) | Header line only, no table frame. The narrowed bullet lists the deferred and dismissed findings. |
| Run committed nothing | `### Changes Made` omitted entirely. The Outcome line already reports it. |
| Fix produced no commit because lint or tests failed | No row. The finding is "abandoned" in the narrowed bullet, and it also appears under Remaining Issues in the Final Report. |
| Interrupted run | Never reaches Step 4, so there is no final table. The per-iteration tables already emitted are the record. The existing rule against reconstructing a Final Report for an interrupted run is unchanged. |
| Pruned iteration with one commit | Header reads "1 commit". Table has one row. |

## Verification

No executable code changes, so verification is grep-based against the edited file. Patterns use
`\s+` for every space, because the file is hand-wrapped and a literal-space pattern would pass
vacuously.

1. `rg -c 'iteration_commits' SKILL.md` returns 5 (Edit 1 reset, Edit 2 append, Edit 3 state
   entry, Edit 3 `run_commits` entry that appends it, Edit 5 row-order rule).
2. `rg -c 'run_commits' SKILL.md` returns 3 (Edit 3 entry, Edit 7 twice).
3. `rg 'reset\s+four\s+per-iteration\s+accumulators' SKILL.md` matches. Confirms the count in
   Edit 1's lead-in was updated and no stale "three" remains.
4. `rg -c '[─│┌┐└┘├┤┬┴┼]' SKILL.md` returns nothing. Confirms no box-drawing glyphs were added.
5. `rg 'Each\s+finding\s+kept\s+by' SKILL.md` returns nothing. Confirms Edit 4 replaced the old
   bullet rather than adding beside it.
6. `rg -n 'commit\s+hash\s+\(short\)' SKILL.md` returns nothing. Confirms Edit 6 replaced the old
   list.
7. Read the per-iteration summary section and Step 4 end to end once, checking that the table
   spec sits before the "Keep it terse" paragraph and that no sentence joins two clauses with a
   dash or a splitting colon.

## Known follow-ups (parked after the final review, not applied)

Recorded here because the SDD workspace that held the review ledger has been deleted and
these rulings are not in git history. F1 and F2 have since been applied. Only F3 remains open,
and it is cosmetic.

**F1. The render/append/empty order was pinned by inference, not explicitly. APPLIED.** The
`run_commits` entry used to anchor both the append and the emptying to "as each iteration's
summary is emitted". That is the same window the per-iteration table renders in, so on the
letter the pair could float ahead of the render and print an empty table. Four separate
statements contradicted that outcome, so the probability was low and the blast radius
cosmetic, but it was pinned by inference rather than stated. The entry now reads "Append
`iteration_commits` to it once that iteration's commit table has been emitted", which puts
the render before the append. The existing "as soon as its pairs are appended here" already
put the append before the empty. All three sub-steps are now ordered explicitly.

**F2. Remaining Issues did not carve out the "moot" outcome. APPLIED.** The per-iteration
bullet labels a finding made moot because an earlier commit in the same iteration already
fixed its file. Step 4's Remaining Issues paragraph used to emit every finding that did not
become a commit, omitting only commits and deferred or dismissed ticket findings, so a moot
finding was reported as still open even though it was fixed. The same run said both things at
once. The behaviour predated this change. Step 2 has always committed with `git add -A`, such
a finding has always produced no commit, and Remaining Issues has always emitted it. Naming
the case did not create the gap, it made the gap visible. That paragraph now excludes a moot
finding explicitly and its omit condition carries all three carve-outs, so a run is silent
about a finding only when nothing about it is still open.

**F3. Two lines wrap short**, around 84 and 87-88 columns against the file's roughly 98-column
norm. Purely cosmetic. Rewrapping would churn plan-mandated literal text.

## Out of scope

- `pr-review`, `in-depth-review`, `gh-style-review`. None of them commit, so none of them has a
  commit table to show. Verified: no sibling skill references the per-iteration summary.
- The frontmatter description.
- Any change to loop control, the confidence threshold, coverage reporting, or the reviewer
  fan-out.
