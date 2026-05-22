---
name: review-and-fix
description: >
  Iteratively reviews recent code changes and fixes identified issues or implements improvements.
  Each iteration launches FIVE specialized reviewer roles (AGENTS.md compliance, shallow bug scan,
  git history context, prior PR comments, in-file code comments), running EACH role THREE TIMES
  in parallel for a total of 15 concurrent review sub-agents per iteration. Findings are then
  scored 0–100 for confidence, filtered at <50, and deduplicated. Fixes are applied and committed
  one at a time. The loop stops as soon as one batch finds nothing actionable, or after 20
  iterations. No GitHub write commands are ever issued. Produces a final summary report.
  Use this skill when the user asks to "review and fix", "review my changes", "clean up my code",
  "improve my recent commits", or similar requests to audit and improve uncommitted or branch-local changes.
---

# Review and Fix

Each iteration launches **15 parallel review sub-agents**: 5 specialized roles, each instantiated
3 times. Their findings are scored 0–100 for confidence, filtered at <50, then deduplicated.
Fixes are applied and committed one at a time. The loop stops as soon as one batch reports nothing
actionable (a single clean pass), or after 20 iterations.

## Why 5 × 3 ?

- **5 specialized roles** give cross-domain coverage: each role surfaces a different class of
  issue (style/standards, raw bugs, historical context, prior PR feedback, in-file guidance).
- **3 runs per role** give within-role triangulation: agreement across the 3 runs raises
  confidence; issues only one of the 3 catches still get surfaced.
- Together: high recall (rare issues still surface) AND high precision (confidence scoring +
  the < 50 filter discards noise).

## GitHub side-effect policy

**This skill never writes to GitHub.** Reviews run as local sub-agents that read `git log` /
`git diff`. The skill must not invoke `gh pr comment`, `gh pr review`, `gh pr edit`,
`gh pr close`, `gh issue create`, `gh issue comment`, or any other write command — even
transitively through other skills (notably the upstream `code-review` skill, whose terminal
step posts a PR comment).

Permitted read-only `gh` invocations (only Reviewer #4 below needs these):
`gh pr list`, `gh pr view`, `gh search pulls`, `gh search issues`.

If a sub-agent appears about to issue any write command, abort the iteration and surface the
attempt to the user.

## Process Overview

1. Determine the commit range to review (current branch vs. default branch)
2. Launch **15 parallel review sub-agents** (5 roles × 3 runs each)
3. Score each raised finding 0–100 in parallel; filter < 50
4. Merge and deduplicate surviving findings
5. Fix each unique finding, asking for clarification on ambiguous items, committing each fix
6. Repeat from step 2 until one batch is clean OR 20 iterations are reached
7. Deliver a final summary report

## Step 0: Setup

1. Confirm the working tree is clean (`git status --porcelain`). If there are uncommitted changes, warn the user and ask whether to stash first or include them in the review.
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
4. Store the commit range as `origin/<default-branch>..HEAD` for all subsequent operations.

## Step 1: Review — 15 Parallel Sub-Agents

Announce at iteration start:

> Iter N: launching 15 reviewers (5 roles × 3 runs).

Launch all 15 reviewers **in a single message** (15 concurrent tool-use blocks). Sequential
launches defeat the purpose of this design — never serialize the reviewers. For each of the 5
roles below, spawn 3 sub-agents with the same prompt; the 3 runs are independent and must not
coordinate.

### Common reviewer prompt fragment

Every reviewer prompt starts with this block. Substitute the actual commit range:

```
Commit range: <COMMIT_RANGE>

Run `git log --no-pager <COMMIT_RANGE> --oneline` to list the commits, then
`git diff --no-pager <COMMIT_RANGE>` to see all changes.

Also read the project AGENTS.md and any sub-project AGENTS.md files relevant to the changed files
(they contain mandatory quality standards and coding conventions).

Discount or downscore findings that look like:
- Pre-existing issues (on lines the diff didn't touch)
- Things that look like bugs but aren't on closer inspection
- Pedantic nitpicks a senior engineer wouldn't raise
- Issues a linter / typechecker / compiler would catch (assume CI runs these — don't run them)
- General code-quality complaints (test coverage, generic security advice, doc) unless explicitly
  required by AGENTS.md
- Issues called out in AGENTS.md but explicitly silenced in code (e.g. a lint-ignore comment)
- Changes in functionality that are obviously intentional or directly related to the broader change

Return a structured list of findings. For each finding include:
- File path and line number(s)
- Severity: critical | major | minor | suggestion
- Clear description of the issue or improvement
- Suggested fix (code snippet or approach)

If you find NO issues, respond with exactly: "NO_ISSUES_FOUND"

You are one of 15 reviewers running concurrently. Do NOT coordinate with the others.

IMPORTANT: Do not invoke the `code-review` skill. Do not run `gh pr comment`, `gh pr review`,
`gh pr edit`, or any command that writes to GitHub. Read-only gh commands (gh pr list / view /
search) are permitted only if your role below explicitly requires them.
```

### Reviewer Role #1 — AGENTS.md compliance  (× 3 runs)

```
Your job: audit the changes for compliance with the relevant AGENTS.md files (root + any
sub-project AGENTS.md whose directory the diff touches). Read each one in full.

Note that AGENTS.md is guidance for Claude as it WRITES code, so not every rule is a review
criterion — ignore any that clearly only apply at authoring time (e.g. "use TodoWrite for tasks").
Focus on rules about what the resulting code must look like, contain, or avoid.

When flagging, cite the specific AGENTS.md file and the rule text.
```

### Reviewer Role #2 — Shallow bug scan  (× 3 runs)

```
Your job: read ONLY the diff itself. Do NOT read surrounding context unless you absolutely must.
Look for obvious, big-impact bugs:
- Logic errors, off-by-one, wrong operator, inverted condition
- Null/undefined/None handling
- Resource leaks, missing cleanup
- Race conditions visible from the diff alone

Skip nitpicks. Skip anything a linter would catch. Skip "could be cleaner" complaints. If a bug
isn't a real production risk, don't flag it.
```

### Reviewer Role #3 — Git history context  (× 3 runs)

```
Your job: for the modified lines, run `git blame --no-pager <COMMIT_RANGE> -- <file>` and
`git log --no-pager -p -L <line>,<line>:<file>` to understand WHY the original code was written
that way.

Flag bugs that are visible only in light of that history. Common patterns:
- A fix is being reverted (search the log for the commit that introduced the line being deleted)
- A change reintroduces a previously fixed bug
- A change contradicts a documented invariant from a past commit message
```

### Reviewer Role #4 — Prior PR comments (read-only)  (× 3 runs)

```
Your job: for each file in the diff (get the list via `git diff --name-only <COMMIT_RANGE>`),
search for past PRs that touched the same files:

  gh pr list --search "<file-path>" --state merged --limit 10 --json number,title,url

For the top few hits, read review comments:

  gh pr view <pr-number> --comments

Surface any past feedback that applies to the current change. Past reviewers may have already
flagged the same class of issue, or there may be agreed-upon conventions documented in the
discussion.

You are READ-ONLY. Do not run `gh pr comment`, `gh pr review`, or any write command. If gh is
not available or unauthenticated, respond with "NO_ISSUES_FOUND" and note the limitation.
```

### Reviewer Role #5 — In-file code comments  (× 3 runs)

```
Your job: read the inline code comments and docstrings in the modified files (not just the diff —
the whole file is fair game). Surface any place where the change contradicts guidance written in
those comments.

Typical signals:
- A "// IMPORTANT:" or "// WARNING:" comment that the change ignores
- A function-level docstring whose invariant the change violates
- A TODO whose resolution the change implicitly makes urgent
```

### Confidence scoring

After all 15 reviewers return:

1. Pool every non-clean response. Each response is a list of findings.
2. **Pre-score deduplication** — group findings that look like duplicates (same file +
   overlapping line range + substantially the same problem) so we don't waste scoring calls on
   trivially-duplicate findings. For each group, keep one canonical entry and record how many of
   the 15 reviewers raised it (the "agreement count"). Highest severity in the group wins.
3. **Launch a scoring sub-agent for each unique finding in parallel** (one Skill call per finding,
   all in a single message). Give each scorer:
   - The finding (file, line, severity, description, suggested fix)
   - The path of every AGENTS.md file referenced by any reviewer that raised it
   - The diff for the relevant lines
   - The agreement count (so the scorer can weight it as one piece of evidence)

Each scorer returns a number 0–100 with this rubric:

| Score | Meaning |
|---|---|
| 0 | False positive that doesn't survive light scrutiny, or a pre-existing issue |
| 25 | Somewhat confident — might be real, might be false; couldn't verify either way. For AGENTS.md issues: the cited rule doesn't actually call this out. |
| 50 | Moderately confident — verified real, but might be a nitpick or rarely hits in practice |
| 75 | Highly confident — verified, will likely be hit in practice; OR an explicit AGENTS.md violation |
| 100 | Absolutely certain — evidence directly confirms it, happens frequently |

**Filter findings with score < 80 and discard them.** This threshold is intentionally aggressive:
with 15 reviewers, recall is high and noise is the primary risk.

### Aggregation

1. **Count clean responses.** A reviewer's response is "clean" if it is exactly `NO_ISSUES_FOUND`.

2. **If all 15 reviewers were clean** → mark batch clean; skip to Step 3 (loop control). No
   scoring needed.

3. Otherwise, the surviving findings (post-scoring, score ≥ 80) form the work list. Order by:
   1. Severity descending (critical → major → minor → suggestion)
   2. Agreement count descending (more reviewers raised it → higher priority)
   3. Confidence score descending

4. **If the surviving list is empty** (all findings scored < 80), treat the batch as clean and
   skip to Step 3.

5. Otherwise → Step 2 with the ordered list.

## Step 2: Fix

Process each finding from the ordered work list (Step 1) one at a time.

### For each finding:

1. **Read the relevant file(s)** to understand the context.

2. **Assess confidence:**
   - If the fix is clear and unambiguous → implement it directly.
   - If the fix is ambiguous or has multiple valid approaches → use `ask_user` to present the options and wait for a decision before proceeding.

3. **Implement the fix** following all project coding standards:
   - Read the relevant `AGENTS.md` (root and sub-project) for mandatory conventions.
   - Run the project's linter/formatter if one exists and fix any violations it reports.
   - Run the project's tests (`pnpm run test:unit` for the web sub-project, or the equivalent for the relevant sub-project) to confirm no regressions.
   - **Do not commit if lint or tests fail.** Fix the failures first or escalate to the user.

4. **Commit the fix:**
   ```
   git add -A
   git commit -m "<type>: <short description of what was fixed>

   <optional body explaining why>
   ```
   Use conventional commit types: defined in the `.github/semantic.yml` file (e.g., `fix`, `feat`, `refactor`, `docs`, etc.) and ensure the message is clear and concise. If the file is missing try to figure out what the correct type should be.

5. After committing, move to the next finding.

## Step 3: Loop Control

After processing all findings (or after a clean batch):

| Condition | Action |
|---|---|
| This iteration's batch was clean (surviving findings list empty) | Stop — proceed to Final Report |
| `iteration` reached 10 | Stop — proceed to Final Report (include limit notice) |
| Otherwise | Go back to Step 1 |

Track state explicitly:
- `iteration`: starts at 1, increments before each Step 1 launch
- `batch_clean`: per-iteration flag, true iff the surviving findings list was empty

**A single clean batch ends the loop.** With 15 independent reviewers all agreeing (or every
raised finding failing the 80-confidence threshold), the signal is already strong; requiring two
consecutive clean batches would just waste an iteration.

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
- <finding description> [severity, agreement N/15, confidence X] — <file:line>
- ...

### Outcome
✅ Clean batch — all 15 reviewers agreed there is nothing actionable. Done.
— OR —
⚠️ Stopped after 10 iterations. See remaining issues above.
```

## Constraints

- **No GitHub writes, ever.** Forbidden: `gh pr comment`, `gh pr review`, `gh pr edit`,
  `gh pr close`, `gh issue create`, `gh issue comment`, or invoking any skill that does
  (notably the upstream `code-review` skill, whose terminal step posts a PR comment).
  Permitted read-only `gh` calls: `gh pr list`, `gh pr view`, `gh search pulls`,
  `gh search issues`. If a sub-agent appears about to issue a write command, abort and surface
  the attempt to the user.
- **15 parallel reviewers per iteration** (5 roles × 3 runs each) — launch them in a single
  message with concurrent tool calls. Do not fall back to fewer agents "for speed"; the
  triangulation + specialization is the point.
- **Scoring is per-finding and parallel** — one scorer per unique finding, all launched in a
  single message. Never score serially.
- **Confidence threshold is 50** — discard everything below. Do not lower the threshold to
  surface more findings.
- **One commit per fix** — never squash or amend.
- **Never commit broken code** — lint and tests must pass before committing.
- **Never push** — only local commits; the user decides when to push.
- **Ask before acting on ambiguous findings** — use `ask_user` for anything unclear.
- **Respect project AGENTS.md rules** — always read mandatory checklists before committing.
