---
name: open-pr
description: >
  Opens a new pull request for the current branch with a SIMPLE, scannable title and a short
  description. If the repo has a PR template (.github/PULL_REQUEST_TEMPLATE.md or similar),
  fills it in; otherwise uses a default "Goal / how / key-changes-list" format. Designed to
  avoid the wall-of-text PR descriptions that nobody reads — keep titles single-line, use
  bullet lists, never enumerate every file in the diff.
  Use this skill when the user asks to "open a PR", "create a PR", "make a pull request",
  "submit a PR", "draft a PR", or similar.
---

# Open PR

This skill creates a new pull request via `gh pr create`. Its job is to produce a PR people
will actually read — short, simple, scannable.

## Principles (read before drafting anything)

1. **Title is simple.** One line, prefer < 70 chars. State the change, not the justification.
   ✅ "Add SQS queue for invoice generation" — not ❌ "feat(billing): refactor the invoice
   subsystem to enable asynchronous PDF rendering via SQS".
2. **Description is short.** The default format has two narrative sections (`Goal`, `how`)
   plus one bullet list of key changes. The list is **not exhaustive** — only the things a
   reader needs to know.
3. **Lists beat prose.** A bulleted list of 4 key changes is far more scannable than a 3-
   paragraph essay covering the same ground. Use lists liberally.
4. **Template wins.** If the repo has a PR template, USE IT. Don't substitute the default
   format just because you prefer it.

## Step 0: Preflight

1. Verify we're in a git repo: `git rev-parse --is-inside-work-tree`. Abort if not.
2. Determine the current branch: `git rev-parse --abbrev-ref HEAD`. Abort if it's `HEAD`
   (detached) or matches the default branch (can't PR from main to main).
3. Determine the default branch:
   ```
   gh repo view --json defaultBranchRef --jq .defaultBranchRef.name
   ```
   Fall back to `git remote show origin | grep 'HEAD branch' | awk '{print $NF}'`.
4. Verify commits exist ahead of default:
   ```
   git rev-list --count origin/<default>..HEAD
   ```
   If 0, abort: "no new commits to PR".
5. Check whether a PR already exists for this branch:
   ```
   gh pr view --json number,state,url
   ```
   If one exists with `state == "OPEN"`, abort and tell the user — point them at the URL.
   They probably want to push more commits, not open a duplicate. (Closed or merged is fine
   — proceed.)

## Step 1: Detect PR template

Look for a template, in this order:

1. `.github/PULL_REQUEST_TEMPLATE.md`
2. `.github/pull_request_template.md`
3. `docs/PULL_REQUEST_TEMPLATE.md`
4. `PULL_REQUEST_TEMPLATE.md` (repo root)
5. `.github/PULL_REQUEST_TEMPLATE/` directory containing multiple templates

If found:

- Read the template.
- Treat `<!-- comments -->` and `{{ placeholder }}` markers as fill-in slots.
- For a multi-template directory: pick the file whose name best matches what the change is
  about (e.g. `feature.md` for new functionality, `bugfix.md` for fixes, `chore.md` for
  refactors). If unclear, ask the user which to use.

If NO template exists, fall back to the default format (Step 3).

## Step 2: Gather context

1. Branch commit list:
   ```
   git --no-pager log origin/<default>..HEAD --oneline
   ```
2. Stats:
   ```
   git diff --stat origin/<default>..HEAD
   ```
3. Files changed:
   ```
   git diff --name-only origin/<default>..HEAD
   ```
4. **The user's intent always outranks the diff.** If the user described what this PR is for
   in the conversation, treat that as the source of truth. Use the diff to confirm or refine,
   not override.
5. Inspect recent PR titles for project conventions:
   ```
   gh pr list --limit 20 --json title --jq '.[].title'
   ```
   If the project consistently uses conventional-commit prefixes (`feat:`, `fix:`, etc.),
   follow that convention. If not, skip prefixes.

## Step 3: Draft the title

Rules:

- Single line, prefer < 70 chars, hard max 100.
- Imperative voice: "Add X", "Fix Y", "Refactor Z" — not "Adding X", "I added X", or
  past-tense "Added X".
- State the change at user/system level, not the implementation detail.
- No PR number, no branch name, no ticket key — unless the project's existing PRs
  consistently include one of those.

## Step 4: Draft the description

### If a template was found (Step 1)

Fill it in. Same principles apply:

- Each field stays short.
- Use bullets where the template offers a list.
- If a template field doesn't apply to this change, write `N/A` rather than padding with
  fabricated content.
- Don't reorder, rename, or delete template sections. Only fill them.

### If NO template was found

Use this exact format (note the lowercase `how` — that's intentional):

```markdown
### Goal

<short explanation of what we're trying to achieve. 1–3 sentences. Add links to relevant
tickets / Jira / Datadog dashboards / Linear issues / design docs when available.>

### how

<short explanation of HOW we achieve it. 1–2 sentences. High level — what's the approach,
not every implementation detail.>

This PR does the following:

- <key change 1>
- <key change 2>
- <key change 3>
- ...
```

### Rules for the "This PR does the following" list

- **Not exhaustive.** Only key changes. If a refactor touches 12 files with the same kind of
  edit, that's ONE bullet, not 12.
- **5–7 bullets max.** If you have more, you're including too much detail.
- **One line per bullet.** Human-readable, what changed.
- **Call out unrelated bug fixes explicitly.** ✅ "Fix an unrelated bug where Y was returning
  Z instead of A".
- **Don't pad with refactor noise.** Renames, import cleanups, format tweaks — none of those
  are key changes.

#### Good examples

- Create a new SQS queue for invoice generation
- When an invoice is created, push a message to the new queue
- Consume the queue in the existing billing worker
- Fix an unrelated bug where invoice totals included tax twice

#### Bad examples (and why)

- ❌ "Rename `i` to `invoice` in `BillingWorker.process()`" — too granular, that's diff noise
- ❌ "Update the billing module" — too vague, says nothing
- ❌ "Refactor the entire invoice subsystem to support asynchronous PDF rendering via the
  newly-introduced SQS queue, including changes to the consumer, producer, message format,
  retry logic, and observability dashboards" — wall of text masquerading as a bullet

### Goal section guidance

- 1–3 sentences. State the user-facing or system-facing outcome.
- Link to a ticket if there is one — don't fabricate links.
- If the change is purely internal (refactor, infra cleanup), still describe WHAT it
  achieves. ✅ "Reduce billing worker memory footprint" — not "Improve code quality".

### how section guidance

- 1–2 sentences.
- Mention the key architectural decision or approach.
- Do NOT enumerate files or functions — the diff has those.

## Step 5: Preview and confirm

Before issuing `gh pr create`, show the title + description to the user and wait for
confirmation. Use `ask_user`:

```
**Title:** <drafted title>

**Description:**

<rendered description>

---

Ready to open this PR? Reply "yes" to create it, or tell me what to change.
```

Loop on edits until the user confirms or aborts. Do not skip this step — opening a PR is a
GitHub write, and the title + description outlive the conversation.

## Step 6: Push the branch if needed

Check upstream tracking:

```
git rev-parse --abbrev-ref --symbolic-full-name @{upstream} 2>/dev/null
```

If no upstream is configured, or `git status` shows commits ahead of the remote, push:

```
git push -u origin HEAD
```

Confirm with the user before pushing. Pushing is a write action visible to others (it
publishes your commits to the remote); a single `ask_user` "ok to push?" is enough.

## Step 7: Create the PR

```sh
gh pr create \
  --base "<default-branch>" \
  --title "<drafted title>" \
  --body "$(cat <<'EOF'
<drafted body>
EOF
)"
```

Add `--draft` if the user asked for a draft (either via skill argument `--draft` or by
saying "open as draft" in the conversation).

After `gh pr create` returns the URL, report it to the user:

```
✅ Opened PR: <url>
```

If `gh pr create` returns an error (network failure, base branch protected, etc.), surface
the exact error — don't retry blindly.

## Constraints

- **Title under 70 chars when possible**, hard max 100.
- **Description short.** Default format = two narrative sections + one non-exhaustive
  bullet list (≤ 7 bullets).
- **Use the repo's PR template if one exists.** Don't substitute the default.
- **Always preview before posting** (Step 5). User must confirm.
- **Never create a PR if one already exists open** for the current branch.
- **No Co-Authored-By footers, no "🤖 Generated with Claude Code" footers, no agent
  branding** in the PR description. The user wants PRs that read like a human wrote them.
- **No conventional-commit prefixes** (`feat:`, `fix:`, etc.) unless the project's existing
  PRs consistently use them — check via `gh pr list --limit 20`.
- **Don't enumerate every file changed.** The diff is right there.
- **No fabricated ticket links.** If you don't know whether a ticket exists, ask.
- **Single PR per invocation.** Don't open multiple PRs in one skill run.
