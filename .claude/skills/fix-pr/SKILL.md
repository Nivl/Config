---
name: fix-pr
description: >
  Automatically fix a GitHub PR for the current branch — handles review comments, CI failures, and open questions.
  Use this skill whenever the user wants to address PR feedback, fix CI, resolve review comments, or generally
  "clean up" or "fix" their PR. Also trigger when the user says things like "handle PR comments", "fix the build",
  "address review feedback", "what's failing on my PR", or any variation of wanting to get a PR into mergeable shape.
  If the user mentions a PR, review comments, or CI checks in the context of fixing things, use this skill.
---

# Fix PR

Automatically connect to GitHub, inspect the current branch's PR, and fix everything that needs fixing — review comments, CI failures, and open questions.

## Prerequisites

Check for GitHub access in this order:
1. Try `gh auth status` to see if the `gh` CLI is installed and authenticated
2. If `gh` is not available, check if a GitHub MCP tool is available and the user is logged in
3. If neither works, ask the user to install and authenticate the `gh` CLI (`gh auth login`)

Do not proceed until you have a working GitHub connection.

## Step 1: Gather PR State

Find the PR for the current branch:
```
gh pr view --json number,title,url,state,reviewDecision,statusCheckRollup,reviews,comments
```

If no PR exists for the current branch, tell the user and stop.

Gather all the information you need in parallel:
- **Review threads** (inline code review feedback with resolved status): use the GraphQL API to get threads including their `isResolved` field — REST comments do not expose this:
  ```
  gh api graphql -f query='
    query($owner:String!, $repo:String!, $pr:Int!, $cursor:String) {
      repository(owner:$owner, name:$repo) {
        pullRequest(number:$pr) {
          reviewThreads(first:100, after:$cursor) {
            pageInfo { hasNextPage endCursor }
            nodes {
              id
              isResolved
              isOutdated
              comments(first:10) {
                nodes { databaseId body path line author { login } }
              }
            }
          }
        }
      }
    }
  ' -F owner=OWNER -F repo=REPO -F pr=NUMBER -F cursor=null
  ```
  Repeat with `-F cursor=END_CURSOR` from `pageInfo.endCursor` until `pageInfo.hasNextPage` is false. Collect all pages before triaging.

  **Important:** Only process threads where `isResolved: false`. Threads where `isResolved: true` are already handled — skip them entirely, do not re-examine or re-fix them.
- **Issue-level comments** (general PR discussion): `gh api repos/{owner}/{repo}/issues/{number}/comments --paginate`
- **CI check status**: from `statusCheckRollup` in the PR view, or `gh pr checks`
- **Review state**: approvals, changes requested, pending reviews

## Step 2: Triage and Plan

Categorize everything into a clear plan and present it to the user before starting work:

1. **Code change requests** — unresolved review threads asking for specific code modifications (rename, refactor, add handling, etc.)
2. **Questions to answer** — unresolved review threads or discussion that ask questions
3. **CI failures** — failing checks that need investigation
4. **Flaky test failures** — tests that appear to be flaky (inconsistent failures, known flaky tests, failures unrelated to the PR changes). Keep these separate — they'll be handled last.

**Skip any review thread where `isResolved: true`** — these are already done, regardless of whether they contain code change requests or questions.

Present the plan as a numbered list so the user can see what you're about to do. This also gives them a chance to say "skip item 3" or "don't bother with that one" before you start.

## Step 3: Fix Code Change Requests

For each review comment requesting a code change:

1. Read the relevant file and understand the context around the comment
2. Make the requested change
3. Run any relevant linters on the changed files and fix lint errors
4. Run any relevant tests on the changed code and fix test failures
5. Commit with a clear message describing the fix (one commit per comment)
6. Reply to the review comment on GitHub explaining what you did and include a link to the commit:
   ```
   gh api repos/{owner}/{repo}/pulls/{number}/comments/{comment_id}/replies -f body="Fixed — <explanation>. See <commit_url>"
   ```
7. Resolve the comment thread if possible:
   ```
   gh api graphql -f query='mutation { minimizeComment(input: {subjectId: "<node_id>", classifier: RESOLVED}) { minimizedComment { isMinimized } } }'
   ```
   Note: resolving review threads requires the GraphQL API. Use the `node_id` from the comment to resolve it. If the API doesn't support resolving (permissions, etc.), skip this step — it's nice-to-have, not critical.

Push after all code change requests are handled (one push for all commits).

## Step 4: Answer Questions

For each question in the review comments:

1. Read the surrounding code and PR context to understand what's being asked
2. If you can confidently answer the question, reply on GitHub with a clear explanation
3. If you're not sure about the answer, ask the user:
   - Present the question and its context
   - Suggest an answer if you have a reasonable guess
   - Give the user the option to: (a) use your suggested answer, (b) provide their own answer, or (c) skip this question for now
4. If the user provides an answer, reply on GitHub on their behalf
5. If the user says to skip, move on — don't resolve the thread

## Step 5: Fix CI Failures

For each failing CI check:

1. Get the failure details:
   ```
   gh run view {run_id} --log-failed
   ```
2. Identify what's failing — test failure, lint error, build error, type check, etc.
3. Read the relevant code, understand the failure, and fix it
4. Run the fix locally to verify (run the specific test, linter, or build command)
5. Commit with a message describing the CI fix
6. Push

### Flaky Tests

If you identify tests that appear flaky (failing intermittently, unrelated to PR changes, known to be unreliable):

1. Keep these separate from the other CI fixes
2. After all other work is done, present the flaky tests to the user
3. Ask if they want you to fix them in a separate commit
4. If yes, fix them and commit separately with a message like "fix: stabilize flaky test <test_name>"
5. If no, leave them alone

## Step 6: Final Verification

After all fixes are pushed:

1. Run `gh pr checks` to see if CI is now passing (it may take a moment for checks to start)
2. Summarize what was done:
   - How many comments were addressed
   - How many questions were answered
   - How many CI failures were fixed
   - Any items that were skipped and why

## Important Principles

- **One commit per fix**: each review comment or CI fix gets its own commit with a descriptive message. This makes it easy for reviewers to see what changed in response to their feedback.
- **Always reply on GitHub**: when addressing a comment, reply explaining what was done and link to the commit. Reviewers shouldn't have to hunt through commits to see if their feedback was addressed.
- **Resolve conversations on GitHub**: if you successfully address a review comment, resolve the thread so we know we don't have to take care of it anymore. If the comment is still relevant or you weren't able to fully address it, leave it open.
- **Respect the codebase**: follow existing code style, run linters and tests, don't introduce new issues while fixing old ones.
- **Flaky tests are special**: they get their own commit and require explicit user approval because they're outside the scope of the PR's intended changes.
