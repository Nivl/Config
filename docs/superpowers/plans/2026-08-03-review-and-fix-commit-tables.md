# review-and-fix Commit Tables Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every `review-and-fix` iteration end with a table of the commits it made, and make the Final Report show those same rows again with an iteration column.

**Architecture:** Three sequential edits to one markdown file. Task 1 adds the two accumulators that record `(sha, finding title)` per commit, because nothing records them today. Task 2 spends `iteration_commits` on the per-iteration table. Task 3 spends `run_commits` on the Final Report table. Tasks 2 and 3 both depend on Task 1 and are independent of each other.

**Tech Stack:** Markdown only. No executable code, so no test runner. The red/green cycle is a `grep` that must miss before an edit and hit after it.

**Spec:** `docs/superpowers/specs/2026-08-03-review-and-fix-commit-tables-design.md`

## Global Constraints

- **Target file, every task:** `shared_config/.claude/skills/review-and-fix/SKILL.md`. No other file is touched.
- **Repo root:** `/Users/melvin/.melvin/config`. Run `git` from inside it. Never use top-level `git -C` (a hook denies it).
- **Wrap new prose at 95-98 columns.** The file's prose tops out near 100 and only its markdown table rows exceed it. Never rewrap a line you did not otherwise change.
- **Plain ASCII prose (AGENTS.md).** Do not join two clauses with `—`, `–`, ` - ` (space-hyphen-space), or a clause-splitting `:`. Split into two sentences instead. Hyphenated words, flags, ranges, and label prefixes are fine.
- **One house-style exception, already in the file:** the accumulator bullets use `- \`name\` = value — description`. Commit 4a0b06d deliberately left that dash as a "scope-variable and flag definition". The new `iteration_commits` bullet in Task 1 matches it. Do not "fix" the neighbours.
- **Zero box-drawing glyphs (`─ │ ┌ ┐ └ ┘ ├ ┤ ┬ ┴ ┼`) anywhere in the file.** Tables are markdown pipe tables. The terminal renderer draws the borders.
- **Every new rule carries a short why plus an anti-regression note.** That is this file's house style across all 888 lines, and this repo's own reviewers flag rules that lack one.
- **Commit subject prefix:** `review-and-fix: ` (matches recent history: `review-and-fix: re-gate Remaining Issues...`).
- **Commit footer, every commit:**
  ```
  Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
  ```
- **Verification greps use `\s+` for every space.** The file is hand-wrapped, so a literal-space pattern can straddle a line break and pass vacuously by matching nothing.
- **Run each `git` command alone.** No pipes, no `&&`. A hook denies a chained git command in this environment.

---

### Task 1: Record each fix commit and the finding it closed

Adds `iteration_commits` (per-iteration) and `run_commits` (per-run). Nothing renders yet. This
task exists on its own because Tasks 2 and 3 are pure formatting once these two lists exist, and
a reviewer could accept the plumbing while rejecting either table.

**Files:**
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md:491-497` (Step 2 accumulator reset)
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md:543-544` (Step 2 bookkeeping step 5)
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md:641-643` (Step 3 state list)

**Interfaces:**
- Consumes: nothing.
- Produces: `iteration_commits`, a per-iteration list of `(short sha, finding title)` pairs in commit order, reset at the start of each fix phase. And `run_commits`, a per-run list of `(iteration, short sha, finding title)` that is never reset. Task 2 reads `iteration_commits` by that exact name. Task 3 reads `run_commits` by that exact name.

- [ ] **Step 1: Confirm the pre-edit state (the "failing test")**

Run, from `/Users/melvin/.melvin/config`:
```
grep -n 'reset\s\+three\s\+per-iteration\s\+accumulators' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: exactly one hit, at line 492.

```
grep -c 'iteration_commits' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: `0`.

```
grep -c 'run_commits' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: `0`.

Record the line-width baseline that Task 3 checks against:
```
grep -c '.\{101,\}' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: `30`. All thirty are pre-existing markdown table rows and long prose lines. Note the
number down. If it is not 30, use whatever it actually is as the baseline and say so in the Task 3
verification.

If any expectation differs, stop. The file has moved on from the spec and the plan needs rebasing.

- [ ] **Step 2: Widen the reset block and add the fourth accumulator**

The lead-in says "three" and frames all of them as feeding the active-set decision. Both have to
change, because the new accumulator feeds the table instead.

Find this exact text:
```
At the start of the iteration's fix phase, reset three per-iteration accumulators used by
Step 3 to decide the next active set:
- `any_commit` = false — set true the moment any fix is committed.
- `any_logic_change` = false — set true if any committed fix changes program logic.
- `productive_reviewers` = empty — the reviewers whose findings were fixed AND committed this
  iteration (in-depth-review role numbers via each finding's `category`, and/or the
  `gh-style-review` unit). This is the pruned set a non-logic-only iteration reruns.
```

Replace it with:
```
At the start of the iteration's fix phase, reset four per-iteration accumulators. The first
three are what Step 3 reads to decide the next active set. The fourth is what Step 3's commit
table is built from:
- `any_commit` = false — set true the moment any fix is committed.
- `any_logic_change` = false — set true if any committed fix changes program logic.
- `productive_reviewers` = empty — the reviewers whose findings were fixed AND committed this
  iteration (in-depth-review role numbers via each finding's `category`, and/or the
  `gh-style-review` unit). This is the pruned set a non-logic-only iteration reruns.
- `iteration_commits` = empty — one `(short sha, finding title)` pair per committed fix, in
  commit order. Step 3's commit table is this list.
```

- [ ] **Step 3: Append to `iteration_commits` on every commit**

Find this exact text:
```
5. **Record what this commit was**, for Step 3's next-active-set decision:
   - Set `any_commit = true`.
```

Replace it with:
```
5. **Record what this commit was**, for Step 3's next-active-set decision and its commit table:
   - Set `any_commit = true`.
   - Append the commit's short sha plus the finding's `title` to `iteration_commits`. Take the
     title verbatim from the merged finding. It is the `Fix` cell in both tables.
```

- [ ] **Step 4: Declare both accumulators in the state list**

Keep them next to the other Step 2 accumulators so the list stays grouped by owner.

Find this exact text:
```
- `any_commit`, `any_logic_change`, `productive_reviewers`: per-iteration accumulators from
  Step 2, consumed by the table above
- `<ACTIVE_ROLES>`, `<ACTIVE_GH_STYLE>`: the next iteration's active reviewer set
```

Replace it with:
```
- `any_commit`, `any_logic_change`, `productive_reviewers`: per-iteration accumulators from
  Step 2, consumed by the table above
- `iteration_commits`: per-iteration; the `(short sha, finding title)` pairs Step 2 appended, in
  commit order. The per-iteration commit table is this list.
- `run_commits`: per-RUN list of `(iteration, short sha, finding title)`. Append
  `iteration_commits` to it as each iteration's summary is emitted. The Final Report's Changes
  Made table is this list. Never reset it between iterations.
- `<ACTIVE_ROLES>`, `<ACTIVE_GH_STYLE>`: the next iteration's active reviewer set
```

- [ ] **Step 5: Verify (the "passing test")**

```
grep -c 'iteration_commits' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: `4`. Three from this task's three edits, plus the `run_commits` entry that names it.

```
grep -c 'run_commits' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: `1`.

```
grep -c 'reset\s\+four\s\+per-iteration\s\+accumulators' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: `1`.

```
grep -c 'reset\s\+three\s\+per-iteration\s\+accumulators' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: `0`. Confirms no stale "three" survived.

- [ ] **Step 6: Commit**

```
git add shared_config/.claude/skills/review-and-fix/SKILL.md
```

```
git commit -m "review-and-fix: record every fix commit and the finding it closed

Step 2 tracked whether a commit happened and which reviewer earned it, but never
which commit or which finding. Two accumulators now carry that. iteration_commits
is reset per fix phase, run_commits spans the whole run.

Nothing renders them yet.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: End each iteration with a table of its commits

Narrows the per-iteration summary's outcome bullet to non-commits, then closes the block with the
table. Both edits ship together on purpose. Doing the bullet alone drops the commit outcomes with
nothing to replace them. Doing the table alone reports every commit twice in one block.

**Files:**
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md:725-726` (summary bullet 3)
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md:736-737` (insert after the bullet list, before "Keep it terse")

**Interfaces:**
- Consumes: `iteration_commits` from Task 1.
- Produces: the `**Iteration N - M commits**` header plus a two-column `| Commit | Fix |` markdown table, emitted last in every per-iteration summary block.

- [ ] **Step 1: Confirm the pre-edit state**

```
grep -c 'Each\s\+finding\s\+kept\s\+by' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: `1`.

```
grep -c 'Iteration\s\+N\s\+-\s\+M\s\+commits' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: `0`.

- [ ] **Step 2: Narrow the outcome bullet to non-commits**

Find this exact text:
```
- Each finding kept by the `>=50` filter with its outcome, meaning the short commit hash, or
  deferred, or dismissed, or abandoned because lint or tests failed.
```

Replace it with:
```
- Every finding kept by the `>=50` filter that did NOT become a commit, and what happened to it,
  meaning deferred, dismissed, or abandoned because lint or tests failed. Committed findings are
  the table below instead.
```

- [ ] **Step 3: Add the table and its rules**

This goes after the last bullet and before the "Keep it terse" paragraph, so the table is the
final thing the block emits.

Find this exact text:
```
- The row that fired, and either the next iteration's active set or the stop.

Keep it terse.
```

Replace it with:
```
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

When `M` is 0, emit the header line alone and no table. A row 2 stop committed nothing, and an
empty table frame says less than the header already does.

Keep it terse.
```

- [ ] **Step 4: Verify**

```
grep -c 'Each\s\+finding\s\+kept\s\+by' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: `0`. Confirms the bullet was replaced, not duplicated.

```
grep -c 'Iteration\s\+N\s\+-\s\+M\s\+commits' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: `1`.

```
grep -n 'Keep\s\+it\s\+terse' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: one hit, and its line number is now greater than the line number of the
`Iteration N - M commits` header. Confirms the table sits before that paragraph, not after it.

```
grep -c 'iteration_commits' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: `5`. Task 1's four plus the row-order rule added here.

- [ ] **Step 5: Commit**

```
git add shared_config/.claude/skills/review-and-fix/SKILL.md
```

```
git commit -m "review-and-fix: end each iteration with a table of its commits

The outcome bullet covered commits and non-commits together. It now covers only
the findings that did not become a commit, and the block closes with a
Commit / Fix table built from iteration_commits.

Fix is the finding title verbatim, so every cell is either git output or a string
a reviewer wrote. A paraphrased cell would be the one place this file does not
already guard against describing work that was not done.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Turn Changes Made into a cross-iteration table

Replaces the Final Report's flat commit list with the three-column table, adds the prose that says
how it derives from `run_commits`, and runs the whole-file checks that span all three tasks.

**Files:**
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md:753-755` (Changes Made template)
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md:822` (insert before the Remaining Issues paragraph)

**Interfaces:**
- Consumes: `run_commits` from Task 1, and the per-iteration table shape from Task 2 (same `Fix` cell, same row set).
- Produces: nothing downstream. This is the last task.

- [ ] **Step 1: Confirm the pre-edit state**

```
grep -c 'commit\s\+hash\s\+(short)' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: `1`.

```
grep -c 'For\s\+the\s\+\*\*Changes\s\+Made\*\*\s\+section' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: `0`.

- [ ] **Step 2: Replace the commit list with the table**

Find this exact text:
```
### Changes Made
- <commit hash (short)>: <commit message>
- ...
```

Replace it with:
```
### Changes Made (omit when the run committed nothing)

| Iteration | Commit | Fix |
|---|---|---|
| Iteration <n> | <short sha> | <the finding's title> |
```

The parenthesised omit note matches how the template already marks its other optional sections,
for example `### Remaining Issues (omit when every surviving finding was fixed)`.

- [ ] **Step 3: Add the derivation prose**

It goes immediately before the Remaining Issues paragraph, so these normative paragraphs stay in
the same order as the sections they govern.

Find this exact text:
```
For the **Remaining Issues** section: emit it whenever a finding that passed the `>=50` filter did
```

Replace it with:
```
For the **Changes Made** section: it is `run_commits`, which is the per-iteration commit tables
concatenated with an `Iteration` column added, in commit order across the run. Every row of every
per-iteration table appears here exactly once, and the `Fix` cell is the same finding title, so
the final table never says anything an iteration did not already show. Repeat the iteration label
on every row instead of blank-filling the repeats, so a row read on its own still names its
iteration. **Total commits made** above must equal this table's row count. **Iterations
completed** can exceed the highest iteration label here, because a clean iteration or an
all-deferred one contributes no rows. Omit the section when `run_commits` is empty. That is a run
that committed nothing, and the Outcome line already says so.

For the **Remaining Issues** section: emit it whenever a finding that passed the `>=50` filter did
```

- [ ] **Step 4: Verify this task**

```
grep -c 'commit\s\+hash\s\+(short)' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: `0`. Confirms the old list is gone.

```
grep -c 'run_commits' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: `3`. Task 1's state entry plus the two references added here.

- [ ] **Step 5: Verify the whole file**

These span all three tasks, so they run once, here, at the end.

```
grep -c '[─│┌┐└┘├┤┬┴┼]' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: `0`. Confirms no box-drawing glyph was introduced. If this is non-zero, someone
hand-drew a table and it must be converted to a markdown pipe table.

```
grep -c '.\{101,\}' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: the baseline from Task 1 Step 1, which is `30`. A higher number means a line this plan
added broke the wrap budget. Find it and rewrap it. Do not rewrap the pre-existing thirty.

```
git status --porcelain shared_config/
```
Expected: exactly one line, ` M shared_config/.claude/skills/review-and-fix/SKILL.md`. Confirms
nothing else under `shared_config/` was touched. Scope the path so the untracked `docs/` tree,
which is unrelated to this plan, does not show up.

Then read two regions end to end and confirm no sentence joins two independent clauses with a
dash or a splitting colon:
- the per-iteration summary section, from "Cover these in this order" through "Keep it terse"
- Step 4, from `### Changes Made` through the end of the Tickets examined paragraph

- [ ] **Step 6: Commit**

```
git add shared_config/.claude/skills/review-and-fix/SKILL.md
```

```
git commit -m "review-and-fix: turn Changes Made into a cross-iteration table

The Final Report listed commits as hash plus commit message. It now shows the
per-iteration tables concatenated with an Iteration column, built from
run_commits, so the report and the iteration blocks agree row for row.

The section is omitted when the run committed nothing. The Outcome line already
reports that case.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Spec coverage

| Spec item | Task |
|---|---|
| Edit 1, Step 2 reset block | Task 1, Step 2 |
| Edit 2, Step 2 bookkeeping step 5 | Task 1, Step 3 |
| Edit 3, Step 3 state list | Task 1, Step 4 |
| Edit 4, summary bullet 3 | Task 2, Step 2 |
| Edit 5, per-iteration table | Task 2, Step 3 |
| Edit 6, Final Report template | Task 3, Step 2 |
| Edit 7, Final Report prose | Task 3, Step 3 |
| Verification 1, 2, 3 | Task 1 Step 5, Task 2 Step 4, Task 3 Step 4 |
| Verification 4, no box glyphs | Task 3, Step 5 |
| Verification 5, 6, old text gone | Task 2 Step 4, Task 3 Step 4 |
| Verification 7, read-through | Task 3, Step 5 |
| Edge case, M is 0 | Task 2, Step 3 (the closing rule) |
| Edge case, run committed nothing | Task 3, Steps 2 and 3 (the omit rule) |
| Out of scope, frontmatter and sibling skills | No task. Task 3 Step 5's `git diff --stat` enforces it. |

## Deviation from the spec's verification counts

The spec says `iteration_commits` should end at 5 and gives that as one check. This plan splits it
across two tasks, so Task 1 expects 4 and Task 2 expects 5. Same final state, checked twice.
