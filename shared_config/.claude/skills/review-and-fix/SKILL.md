---
name: review-and-fix
description: >
  Iteratively reviews recent code changes and fixes identified issues or implements
  improvements. Each iteration spawns SIX concurrent reviewer sub-agents — THREE
  `in-depth-review` and THREE `gh-style-review` — all invoked with `--raw` against the
  current branch (PR mode if an open PR exists for the branch, branch mode otherwise).
  Their raw scored findings are merged and deduplicated across all six instances into
  one flat pool, filtered to keep anything with confidence >= 50, then fixes are applied
  and committed one at a time. In PR mode, the gh-style instances also return Discussion
  Context (which prior human comments the diff resolves vs. still leaves open); the
  orchestrator surfaces this in the per-iteration summary so the user can see what
  reviewer feedback is being addressed by the fix loop. The loop stops as soon as one
  batch finds nothing actionable above the threshold, or after 10 iterations. No GitHub
  write commands are ever issued. Produces a final summary report.
  Use this skill when the user asks to "review and fix", "review my changes", "clean up my
  code", "improve my recent commits", or similar requests to audit and improve uncommitted or
  branch-local changes.
---

# Review and Fix

This skill wraps `in-depth-review` AND `gh-style-review` with an iterate-and-fix loop.
Each iteration:

1. Runs **3 `in-depth-review` + 3 `gh-style-review` instances in parallel** (6 total, each
   invoked with `--raw`). The target is the branch's PR if one exists for the current
   branch, otherwise the branch's commit range.
2. Cross-instance dedups their findings into one flat pool (each instance already
   pre-dedupes internally; the triangulation across 6 independent passes from two prompt
   structures catches the rest).
3. Applies the orchestrator's own **`confidence >= 50`** filter — more permissive than the
   sub-skills' default of 70 because we want to spend the fix loop's iteration budget on
   moderately-confident findings too. The 6× triangulation gives us enough cross-pass
   evidence that 50–69 findings are worth attempting.
4. (PR mode only) Aggregates Discussion Context across the 3 gh-style instances and shows
   it in the per-iteration summary — useful when the loop is iterating on a PR that already
   has human reviewer comments.
5. Fixes each unique finding, one commit per fix.
6. Loops until a batch is clean (or 10 iterations).

The 6× triangulation lives **here**, not inside the sub-skills. Each `in-depth-review` pass
is itself a 9-role review and each `gh-style-review` pass is the @claude review GitHub
Action prompt with full PR context (when in PR mode).

## GitHub side-effect policy

**This skill never writes to GitHub.** Neither does `in-depth-review` or `gh-style-review`.
The skill never invokes `gh pr comment`, `gh pr review`, `gh pr edit`, or any write command.
Read-only `gh` calls inside the sub-skills (in-depth-review's prior-PR-comment role, plus
gh-style-review's PR conversation/review-thread fetches) are fine.

## Process Overview

1. Determine the target: PR if one exists for the current branch, else commit range
   (current branch vs. default branch)
2. Launch **6 parallel reviewer sub-agents** (3 × in-depth-review + 3 × gh-style-review)
   against that target
3. Merge + deduplicate findings across all 6 instances into one flat pool
4. (PR mode only) Aggregate Discussion Context across the 3 gh-style instances
5. Fix each unique finding, asking for clarification on ambiguous items, committing each fix
6. Repeat from step 2 until one batch is clean OR 10 iterations are reached
7. Deliver a final summary report

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
4. Store the commit range as `<RANGE>` = `origin/<default-branch>..HEAD`.
5. **Detect whether an open PR exists for the current branch** so we can pass PR mode to the
   sub-agents when available (and unlock gh-style-review's Discussion Context):
   ```
   gh pr view --json number,state,isDraft,url 2>/dev/null
   ```
   - Exit non-zero or `state != "OPEN"` → set `<HAS_PR> = false`. Sub-agents will receive
     `<RANGE>` and run in branch mode.
   - Exit zero and `state == "OPEN"` → set `<HAS_PR> = true`, save `<PR>` and `<PR_URL>`.
     Sub-agents will receive `<PR>` and run in PR mode. Draft PRs are accepted.
6. Define `<TARGET_ARG>` for use in the sub-agent prompts:
   - If `<HAS_PR>` → `<TARGET_ARG> = <PR>`
   - Else → `<TARGET_ARG> = <RANGE>`

## Step 1: Review — 6 Parallel Sub-Agents (3 in-depth-review + 3 gh-style-review)

Announce at iteration start:

> Iter N: launching 6 reviewer passes in parallel (3 × in-depth-review, 3 × gh-style-review).
> Target: PR #<PR> [draft]  ←  or  Target: branch range <RANGE>

Spawn **6 sub-agents in a single message** (6 concurrent tool-use blocks). Sequential
launches defeat the purpose — never serialize.

### Sub-agents 1–3 prompt (in-depth-review)

Each of the three in-depth-review sub-agents receives:

```
You are sub-agent N of 6 in a review-and-fix iteration (N is 1, 2, or 3).

Invoke the `in-depth-review` skill with the arguments: `<TARGET_ARG> --raw`

- `<TARGET_ARG>` is either a PR number (PR mode) or a commit range like `origin/main..HEAD`
  (branch mode). in-depth-review auto-detects.
- `--raw` tells in-depth-review to skip its internal <70 confidence filter so we get every
  scored finding (0–100). The orchestrator applies its own >=50 threshold after merge.

Return in-depth-review's structured JSON output to me unchanged, with two top-level
additions:
- `"sub_agent": N`
- `"source": "in-depth-review"`

Specifically forbidden:
- `gh pr comment` (any form)
- `gh pr review` (any form)
- `gh pr edit`, `gh pr close`, `gh pr merge`
- `gh issue create`, `gh issue comment`
- Any other command that writes to GitHub
If `in-depth-review`'s internal logic appears to be about to invoke one of these, abort and
return the abort reason to me instead of proceeding.
```

### Sub-agents 4–6 prompt (gh-style-review)

Each of the three gh-style-review sub-agents receives:

```
You are sub-agent N of 6 in a review-and-fix iteration (N is 4, 5, or 6).

Invoke the `gh-style-review` skill with the arguments: `<TARGET_ARG> --raw`

- `<TARGET_ARG>` is either a PR number (PR mode — unlocks Discussion Context) or a commit
  range (branch mode — Discussion Context arrays will be empty). gh-style-review auto-detects.
- `--raw` tells gh-style-review to skip its internal <70 confidence filter so we get every
  scored finding. The orchestrator applies its own >=50 threshold after merge.

Tell gh-style-review you are invoking it as a sub-agent — it must return the JSON shape
documented in its "If invoked as a sub-agent" section, NOT its terminal-formatted output.

Return that JSON to me unchanged, with two top-level additions:
- `"sub_agent": N`
- `"source": "gh-style-review"`

Specifically forbidden:
- `gh pr comment` (any form)
- `gh pr review` (any form)
- `gh pr edit`, `gh pr close`, `gh pr merge`
- `gh issue create`, `gh issue comment`
- Any other command that writes to GitHub
If `gh-style-review`'s internal logic appears to be about to invoke one of these, abort and
return the abort reason to me instead of proceeding.
```

### Aggregating across the 6 instances

After all 6 sub-agents return:

1. **Pool every finding** from the 6 result sets into one flat pool. Each finding carries its
   raw `confidence` (0–100), `file`, `line_range`, `category`, originating `sub_agent`
   (1..6), and `source` (`"in-depth-review"` or `"gh-style-review"`). Don't pre-segregate
   by source — cross-prompt triangulation is the point.

2. **Cross-instance dedup.** Two findings are duplicates if they refer to the **same file**
   and have **overlapping line ranges** AND describe substantially the same problem
   (paraphrases count). Cross-source duplicates (one from in-depth, one from gh-style)
   count as duplicates — merge them.

3. **For each duplicate group, produce one merged finding:**
   - `confidence`: **max** of the group's scores.
   - `cross_instance_agreement`: count of distinct instances (1..6) that raised this finding.
   - `sources`: set of distinct source skills (one of `{in-depth-review}`, `{gh-style-review}`,
     or both). Used as a tiebreaker — a both-source finding is stronger signal.
   - `title`, `description`, `suggested_fix`: pick the clearest from the group; if suggested
     fixes differ meaningfully, mention the alternatives in the description.
   - `category`: union of categories.

4. **Apply the orchestrator's confidence threshold: discard everything with `confidence < 50`.**
   This is the review-and-fix-specific threshold, lower than each sub-skill's default of 70
   because cross-instance triangulation across 6 passes raises our confidence in 50–69
   findings.

5. **If the post-filter list is empty** (every finding scored < 50), mark the **findings**
   batch clean. The iteration is **fully clean** only if Step 1.5's Discussion Context
   aggregation is also empty (PR mode) — see Step 1.5 below.

6. **Otherwise** proceed to Step 2 with the filtered + deduplicated list, ordered by:
   1. Severity descending (critical → major → minor → suggestion)
   2. `cross_instance_agreement` descending (6/6 > 3/6 > 1/6 when severity ties)
   3. Both-sources first (a both-source finding beats a same-confidence single-source one)
   4. `confidence` descending

## Step 1.5: Aggregate Discussion Context (PR mode only)

If `<HAS_PR>` is false, **skip this step entirely** — gh-style-review returned empty
`discussion_context.resolved` and `discussion_context.unaddressed` arrays in branch mode.

If `<HAS_PR>` is true:

1. **Pool every entry** across the 3 gh-style-review `discussion_context` blocks into two
   flat lists: `resolved_pool` and `unaddressed_pool`. (in-depth-review has no equivalent;
   skip those 3 sub-agents here.)
2. **Deduplicate by `url`** (the GitHub comment URL is canonical). If the same URL appears
   in both `resolved` and `unaddressed` across instances, keep it in `unaddressed_pool`
   (be conservative — surface anything a reviewer is uncertain about).
3. **For each deduplicated entry** pick the clearest `quote`/`resolution`/`gap` text across
   instances. Record `agreement` (1..3).
4. **Retain all entries** — no confidence filter; every entry is grounded in a real human
   comment URL.

This block does NOT trigger fix actions in Step 2 (resolved items are already addressed;
unaddressed items will be picked up by the next iteration's findings if they're actionable
in the current diff). It surfaces in the per-iteration summary and the Final Report so the
user can see the diff's effect on the PR's discussion thread evolving across iterations.

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

| Condition                                                                 | Action                                                |
| ------------------------------------------------------------------------- | ----------------------------------------------------- |
| This iteration's findings batch was clean (deduplicated list empty)       | Stop — proceed to Final Report                        |
| `iteration` reached 10                                                    | Stop — proceed to Final Report (include limit notice) |
| Otherwise                                                                 | Go back to Step 1                                     |

Track state explicitly:

- `iteration`: starts at 1, increments before each Step 1 launch
- `batch_clean`: per-iteration flag, true iff the deduplicated findings list was empty
- `discussion_context_snapshot`: per-iteration snapshot of resolved/unaddressed pools (PR
  mode only) — useful for the per-iteration summary

**A single clean findings batch ends the loop.** With 6 independent reviewers all agreeing
(3 × in-depth-review's 9 roles + 3 × gh-style-review's @claude-mirror prompt), the signal is
already strong; requiring two consecutive clean batches would just waste an iteration.

Note: a non-empty `unaddressed_pool` in PR mode does NOT prevent the loop from terminating.
"Still unaddressed" items are surfaced for the user to consider, but the fix loop is
driven by the findings list — those items will either appear as findings in subsequent
iterations (if they're actionable in the current diff) or persist until the user adds
work that addresses them.

## Step 4: Final Report

Summarise the entire session in a clear report to the user:

```
## Review and Fix Report

**Target:** PR #<PR>  ←  or  branch range <RANGE>
**Iterations completed:** N / 10
**Total commits made:** N

### Changes Made
- <commit hash (short)>: <commit message>
- ...

### Remaining Issues (if iteration limit reached)
- <finding description> [severity, cross-instance N/6, sources <in-depth|gh-style|both>, confidence X] — <file:line>
- ...

### Discussion Context (PR mode only; omit entire section if branch mode or both pools empty)

**Resolved by this branch:**
- > <quote> — @<author> ([link](<url>))
  ✅ <resolution>

**Still unaddressed:**
- > <quote> — @<author> ([link](<url>))
  ⚠️ <gap>

(This reflects the LATEST iteration's gh-style-review snapshot. If the loop ran many
iterations, intermediate snapshots are not reproduced — they're available in the
per-iteration logs above.)

### Outcome
✅ Clean batch — all 6 reviewer instances (3 × in-depth-review + 3 × gh-style-review)
agreed there is nothing actionable in the findings pool. Done.
— OR —
⚠️ Stopped after 10 iterations. See remaining issues above.
```

## Constraints

- **No GitHub writes, ever.** Forbidden: `gh pr comment`, `gh pr review`, `gh pr edit`,
  `gh pr close`, `gh issue create`, `gh issue comment`, or invoking any skill that does
  (notably the upstream `code-review` skill, whose terminal step posts a PR comment).
  Permitted read-only `gh` calls (inside the sub-skills): `gh pr list`, `gh pr view`,
  `gh pr diff`, `gh search pulls`, `gh search issues`, plus the `gh api` reads
  gh-style-review uses to pull PR comments / review threads / prior reviews. If a sub-agent
  appears about to issue a write command, abort and surface the attempt to the user.
- **6 parallel sub-agents per iteration (3 × in-depth-review + 3 × gh-style-review)** —
  launch them in a single message with concurrent tool calls. Do not fall back to fewer
  instances "for speed"; the cross-source triangulation is the point. Do not skip
  gh-style-review when in branch mode — it still contributes findings even with empty
  Discussion Context arrays.
- **Each sub-skill is invoked WITH `--raw`** — we want every scored finding (0–100), not
  the sub-skill's default `< 70` filtered output. The orchestrator applies its own
  `confidence >= 50` threshold after cross-instance, cross-source dedup.
- **Flat-pool merging.** Findings from in-depth-review and gh-style-review are merged into
  one pool, dedup'd by file+line+description regardless of source, then filtered. Source
  is preserved as a `sources` field on each finding (used as a tiebreaker only).
- **Discussion Context comes only from gh-style-review** and only in PR mode. Do not
  attempt to synthesize Discussion Context from in-depth-review findings. Do not fail an
  iteration because Discussion Context is empty — that's expected in branch mode.
- **Confidence threshold is 50.** Do not raise or lower it on the fly.
- **One commit per fix** — never squash or amend.
- **Never commit broken code** — lint and tests must pass before committing.
- **Never push** — only local commits; the user decides when to push.
- **Ask before acting on ambiguous findings** — use `ask_user` for anything unclear.
- **Respect project AGENTS.md rules** — always read mandatory checklists before committing.
