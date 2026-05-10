---
name: review-and-fix
description: >
  Iteratively reviews recent code changes and fixes identified issues or implements improvements.
  Launches a sub-agent to review commits on the current branch compared to the default branch,
  applies fixes one commit at a time, and loops until two consecutive reviews find nothing to address
  (or a maximum of 10 iterations is reached). Produces a final summary report.
  Use this skill when the user asks to "review and fix", "review my changes", "clean up my code",
  "improve my recent commits", or similar requests to audit and improve uncommitted or branch-local changes.
---

# Review and Fix

This skill iteratively reviews recent code changes and fixes identified issues or improvements,
committing each fix individually. It loops until two consecutive review passes report no issues,
up to a maximum of 10 iterations.

## Process Overview

1. Determine the commit range to review (current branch vs. default branch)
2. Launch a **review sub-agent** that analyses the commits and reports findings
3. Fix each identified issue, asking for clarification on ambiguous items, committing each fix
4. Repeat from step 2 until two consecutive clean reviews OR 10 iterations are reached
5. Deliver a final summary report

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

## Step 1: Review (Sub-Agent)

### Determining the review method

Before launching the review sub-agent, check whether the `code-review` skill is available by invoking the `skill` tool with `skill: "code-review"`.

- **If the `code-review` skill is available:** Use it as the review mechanism (see "Option A" below).
- **If the `code-review` skill is NOT available:** Ask the user whether they want to install it using the `ask_user` tool with the following message:

  > The `code-review` skill is not installed. It provides higher-quality reviews. Would you like to install it?
  > It is available in the Copilot marketplace. To install, run:
  > `/plugin install code-review@claude-code-plugins` from `anthropics/claude-code`

  - If the user accepts → stop the current skill, let them install it, and ask them to re-run `review-and-fix` afterwards.
  - If the user declines → proceed with Option B (the built-in sub-agent review).

---

### Option A: Review using the `code-review` skill

Launch a **sub-agent** and instruct it to invoke the `code-review` skill to perform the review. The sub-agent should use the skill tool with `skill: "code-review"` and return the full list of findings reported by the skill.

Once the skill completes, collect the reported issues. Apply the following filter:

> **Ignore any issue with a confidence score of 30 or below.** These are considered too uncertain to act on.

After filtering:
- If **no issues remain** (either the skill found none, or all issues were at or below the confidence threshold of 30), the sub-agent should respond with exactly: `NO_ISSUES_FOUND`
- Otherwise, the sub-agent should return the filtered list of issues.

Read the sub-agent output. If the output is exactly `NO_ISSUES_FOUND`, increment the clean-pass counter and skip to the loop check in Step 3. Otherwise, reset the clean-pass counter to 0 and proceed to Step 2 with the returned issues.

---

### Option B: Built-in explore sub-agent review

Launch an **explore sub-agent** with the following prompt (substitute actual values):

```
You are a senior code reviewer. Review the following commits compared to the default branch.

Commit range: <COMMIT_RANGE>

Run `git log --no-pager <COMMIT_RANGE> --oneline` to list the commits, then
`git diff --no-pager <COMMIT_RANGE>` to see all changes.

Also read the project AGENTS.md and any sub-project AGENTS.md files that are relevant to the
changed files (they contain mandatory quality standards and coding conventions).

For each file changed, consider:
- Bugs, logic errors, off-by-one errors, null/undefined handling
- Security issues (injection, secret exposure, improper auth)
- Violations of project coding standards and conventions
- Unnecessary complexity, dead code, or missing error handling
- Missing or incorrect tests for the changed behaviour
- Performance concerns

Return a structured list of findings. For each finding include:
- File path and line number(s)
- Severity: critical | major | minor | suggestion
- Clear description of the issue or improvement
- Suggested fix (code snippet or approach)

If there are NO issues or improvements, respond with exactly: "NO_ISSUES_FOUND"
```

Read the sub-agent output. If the output is exactly `NO_ISSUES_FOUND`, increment the clean-pass counter and skip to the loop check in Step 3. Otherwise, reset the clean-pass counter to 0 and proceed to Step 2.

## Step 2: Fix

Process each finding from the sub-agent in order of descending severity (critical → major → minor → suggestion):

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
   Use conventional commit types: defined in the `.github/semantic.yml` file (e.g., `fix`, `feat`, `refactor`, `docs`, etc.) and ensure the message is clear and concise. if the file is missing try to figure out what the correct type should be.

5. After committing, move to the next finding.

## Step 3: Loop Control

After processing all findings (or after a clean pass):

| Condition | Action |
|---|---|
| 2 consecutive clean passes | Stop — proceed to Final Report |
| Iteration count reached 10 | Stop — proceed to Final Report (include limit notice) |
| Otherwise | Go back to Step 1 |

Track state explicitly:
- `iteration`: starts at 1, increments before each Step 1 launch
- `consecutive_clean`: starts at 0, incremented on `NO_ISSUES_FOUND`, reset to 0 on any findings

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
- <finding description> [severity] — <file:line>
- ...

### Outcome
✅ No issues found in 2 consecutive passes. Code is clean.
— OR —
⚠️ Stopped after 10 iterations. See remaining issues above.
```

## Constraints

- **One commit per fix** — never squash or amend.
- **Never commit broken code** — lint and tests must pass before committing.
- **Never push** — only local commits; the user decides when to push.
- **Ask before acting on ambiguous findings** — use `ask_user` for anything unclear.
- **Respect project AGENTS.md rules** — always read mandatory checklists before committing.
