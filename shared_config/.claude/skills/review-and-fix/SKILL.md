---
name: review-and-fix
description: >
  Iteratively reviews recent code changes and fixes identified issues or implements
  improvements. Each iteration spawns THREE concurrent `in-depth-review` sub-agents (invoked
  with `--raw`) against the current branch (commits ahead of the default branch); their raw
  scored findings are merged and deduplicated across instances, filtered to keep anything
  with confidence >= 50, then fixes are applied and committed one at a time. The loop stops
  as soon as one batch finds nothing actionable above the threshold, or after 10 iterations.
  No GitHub write commands are ever issued. Produces a final summary report.
  Use this skill when the user asks to "review and fix", "review my changes", "clean up my
  code", "improve my recent commits", or similar requests to audit and improve uncommitted or
  branch-local changes.
---

# Review and Fix

This skill wraps `in-depth-review` with an iterate-and-fix loop. Each iteration:

1. Runs **3 `in-depth-review` instances in parallel** (each invoked with `--raw`) against the
   branch's commit range.
2. Cross-instance dedups their findings (each instance already pre-dedupes internally; the
   triangulation across 3 independent passes catches the rest).
3. Applies the orchestrator's own **`confidence >= 50`** filter — more permissive than the
   in-depth-review default of 70 because we want to spend the fix loop's iteration budget on
   moderately-confident findings too. The 3× triangulation gives us enough cross-pass evidence
   that 50–69 findings are worth attempting.
4. Fixes each unique finding, one commit per fix.
5. Loops until a batch is clean (or 10 iterations).

The 3× triangulation lives **here**, not inside `in-depth-review`. Each `in-depth-review`
pass is itself a 9-role multi-perspective review (see that skill for details).

## GitHub side-effect policy

**This skill never writes to GitHub.** Neither does `in-depth-review`. The skill never invokes
`gh pr comment`, `gh pr review`, `gh pr edit`, or any write command. Read-only `gh` calls
inside `in-depth-review`'s role #4 (prior-PR-comment lookup) are fine.

## Process Overview

1. Determine the commit range to review (current branch vs. default branch)
2. Launch **3 parallel `in-depth-review` sub-agents** on that range
3. Merge + deduplicate findings across the 3 instances
4. Fix each unique finding, asking for clarification on ambiguous items, committing each fix
5. Repeat from step 2 until one batch is clean OR 10 iterations are reached
6. Deliver a final summary report

## Step 0: Setup

1. Confirm the working tree is clean (`git status --porcelain`). If there are uncommitted
   changes, warn the user and ask whether to stash first or include them in the review.
2. Determine the default branch:
   ```
   git remote show origin | grep 'HEAD branch' | awk '{print $NF}'
   ```
   Fall back to `main` if unavailable.
3. Count how many commits the current branch is ahead of the default branch:
   ```
   git rev-list --count origin/<default-branch>..HEAD
   ```
   If 0, inform the user there are no new commits to review and stop.
4. Store the commit range as `<RANGE>` = `origin/<default-branch>..HEAD` for all subsequent
   operations.

## Step 1: Review — 3 Parallel `in-depth-review` Sub-Agents

Announce at iteration start:

> Iter N: launching 3 in-depth-review passes (parallel).

Spawn **3 sub-agents in a single message** (3 concurrent tool-use blocks). Sequential
launches defeat the purpose — never serialize.

Each sub-agent receives this prompt:

```
Invoke the `in-depth-review` skill with the arguments: `<RANGE> --raw`

- The `<RANGE>` arg puts in-depth-review in branch mode against this commit range.
- The `--raw` flag tells in-depth-review to skip its internal <70 confidence filter so we
  get every scored finding (0–100). The orchestrator will apply its own >=50 threshold
  after cross-instance dedup.

Return in-depth-review's structured JSON output to me unchanged.

Specifically forbidden:
- `gh pr comment` (any form)
- `gh pr review` (any form)
- `gh pr edit`, `gh pr close`, `gh pr merge`
- `gh issue create`, `gh issue comment`
- Any other command that writes to GitHub
If `in-depth-review`'s internal logic appears to be about to invoke one of these, abort and
return the abort reason to me instead of proceeding.
```

### Aggregating across the 3 instances

After all 3 sub-agents return:

1. **Pool every finding** from the 3 result sets. Each finding carries its raw `confidence`
   (0–100, since `--raw` was passed), `agreement` (within-instance, 1..9), `file`,
   `line_range`, and category.

2. **Cross-instance dedup.** Two findings are duplicates if they refer to the **same file**
   and have **overlapping line ranges** AND describe substantially the same problem
   (paraphrases count).

3. **For each duplicate group, produce one merged finding:**
   - `confidence`: **max** of the group's scores.
   - `cross_instance_agreement`: count of distinct in-depth-review instances (1..3) that
     raised this finding. (Distinct from the per-instance `agreement` 1..9.)
   - `title`, `description`, `suggested_fix`: pick the clearest from the group; if suggested
     fixes differ meaningfully, mention the alternatives in the description.
   - `category`: union of categories.

4. **Apply the orchestrator's confidence threshold: discard everything with `confidence < 50`.**
   This is the review-and-fix-specific threshold, lower than in-depth-review's default of 70
   because cross-instance triangulation across 3 passes raises our confidence in 50–69
   findings.

5. **If the post-filter list is empty** (either all 3 sub-agents returned zero findings, or
   every finding scored < 50), mark the batch **clean** and skip to Step 3 (loop control).

6. **Otherwise** proceed to Step 2 with the filtered + deduplicated list, ordered by:
   1. Severity descending (critical → major → minor → suggestion)
   2. `cross_instance_agreement` descending (3/3 > 2/3 > 1/3 when severity ties)
   3. `confidence` descending

## Step 2: Fix

Process each finding from the ordered work list (Step 1) one at a time.

### For each finding:

1. **Read the relevant file(s)** to understand the context.

2. **Assess confidence:**
   - If the fix is clear and unambiguous → implement it directly.
   - If the fix is ambiguous or has multiple valid approaches → use `ask_user` to present the
     options and wait for a decision before proceeding.

3. **Implement the fix** following all project coding standards:
   - Read the relevant `AGENTS.md` (root and sub-project) for mandatory conventions.
   - Run the project's linter/formatter if one exists and fix any violations it reports.
   - Run the project's tests (`pnpm run test:unit` for the web sub-project, or the equivalent
     for the relevant sub-project) to confirm no regressions.
   - **Do not commit if lint or tests fail.** Fix the failures first or escalate to the user.

4. **Commit the fix:**

   ```
   git add -A
   git commit -m "<type>: <short description of what was fixed>

   <optional body explaining why>
   ```

   Use conventional commit types: defined in the `.github/semantic.yml` file (e.g., `fix`,
   `feat`, `refactor`, `docs`, etc.) and ensure the message is clear and concise. If the file
   is missing try to figure out what the correct type should be.

5. After committing, move to the next finding.

## Step 3: Loop Control

After processing all findings (or after a clean batch):

| Condition                                                           | Action                                                |
| ------------------------------------------------------------------- | ----------------------------------------------------- |
| This iteration's batch was clean (deduplicated findings list empty) | Stop — proceed to Final Report                        |
| `iteration` reached 10                                              | Stop — proceed to Final Report (include limit notice) |
| Otherwise                                                           | Go back to Step 1                                     |

Track state explicitly:

- `iteration`: starts at 1, increments before each Step 1 launch
- `batch_clean`: per-iteration flag, true iff the deduplicated findings list was empty

**A single clean batch ends the loop.** With 3 independent `in-depth-review` instances all
agreeing (each itself a 9-role specialized review), the signal is already strong; requiring
two consecutive clean batches would just waste an iteration.

## Step 4: Final Report

Summarise the entire session in a clear report to the user:

```
## Review and Fix Report

**Iterations completed:** N / 10
**Total commits made:** N

### Changes Made
- <commit hash (short)>: <commit message>
- ...

### Remaining Issues (if iteration limit reached)
- <finding description> [severity, cross-instance N/3, confidence X] — <file:line>
- ...

### Outcome
✅ Clean batch — all 3 in-depth-review instances agreed there is nothing actionable. Done.
— OR —
⚠️ Stopped after 10 iterations. See remaining issues above.
```

## Constraints

- **No GitHub writes, ever.** Forbidden: `gh pr comment`, `gh pr review`, `gh pr edit`,
  `gh pr close`, `gh issue create`, `gh issue comment`, or invoking any skill that does
  (notably the upstream `code-review` skill, whose terminal step posts a PR comment).
  Permitted read-only `gh` calls (inside `in-depth-review`'s role #4 only): `gh pr list`,
  `gh pr view`, `gh search pulls`, `gh search issues`. If a sub-agent appears about to issue
  a write command, abort and surface the attempt to the user.
- **3 parallel `in-depth-review` instances per iteration** — launch them in a single message
  with concurrent tool calls. Do not fall back to fewer instances "for speed"; the
  triangulation is the point.
- **Each `in-depth-review` is invoked WITH `--raw`** — we want every scored finding (0–100),
  not in-depth-review's default `< 70` filtered output. The orchestrator applies its own
  `confidence >= 50` threshold after cross-instance dedup.
- **Confidence threshold is 50.** Do not raise or lower it on the fly.
- **One commit per fix** — never squash or amend.
- **Never commit broken code** — lint and tests must pass before committing.
- **Never push** — only local commits; the user decides when to push.
- **Ask before acting on ambiguous findings** — use `ask_user` for anything unclear.
- **Respect project AGENTS.md rules** — always read mandatory checklists before committing.
