---
name: open-pr
description: >
  Opens a new pull request for the current branch with a SIMPLE, scannable title and a short
  description. If the repo has a PR template (.github/PULL_REQUEST_TEMPLATE.md or similar),
  fills it in; otherwise uses a default "Goal / how / key-changes-list" format. Designed to
  avoid the wall-of-text PR descriptions that nobody reads. Keep titles single-line, use
  bullet lists, never enumerate every file in the diff.
  Use this skill when the user asks to "open a PR", "create a PR", "make a pull request",
  "submit a PR", "draft a PR", or similar.
---

# Open PR

This skill creates a new pull request via `gh pr create`. Its job is to produce a PR people will
actually read. Short, simple, scannable.

The prose comes from `writing-work-docs`. What follows is the shape that prose has to fit.

## Principles (read before drafting anything)

1. **Title is simple.** One line, prefer < 70 chars. State the change, not the justification.
   ✅ "Add SQS queue for invoice generation". ❌ "feat(billing): refactor the invoice subsystem to
   enable asynchronous PDF rendering via SQS".
2. **Description is short.** The default format is a `Goal` line, an OPTIONAL `how` line, and
   one `Changes` bullet list. The list is **not exhaustive**: only the things a reader needs
   to know.
3. **Lists beat prose.** A bulleted list of 4 key changes is far more scannable than a 3-
   paragraph essay covering the same ground. Use lists liberally.
4. **Template wins.** If the repo has a PR template, USE IT. Don't substitute the default
   format just because you prefer it.
5. **Scale to the change.** A tiny or obvious PR (a few lines, a config bump, a doc fix) gets a
   one-line description and NO section headers. Don't manufacture `Goal` / `how` / `Changes`
   scaffolding for a 5-line diff. Reserve the full format for changes big enough to need it.
6. **Always a one-line summary.** Open the body with one plain sentence saying what the change
   is about and why, so anyone gets it at a glance. If the template has a description / summary
   / overview field, it goes there. If the template has NO such field, put the sentence at the
   very top of the body, above the first section.

## Writing: `writing-work-docs` owns it

**Invoke `writing-work-docs` for the title and the body.** It owns voice, banned words, ASCII
punctuation, invent-nothing, and no-hard-wrapping. Structure is this file's job. Prose is that
file's job. Never draft PR prose without it, and never restate its rules here.

Three overrides, and only three:

- **It never publishes. This skill does.** Ignore its Publishing section. Step 7 runs
  `gh pr create`, gated on the Step 5 preview.
- **Its "add no headings" rule yields to Principle 6.** The one-line summary always goes in. It is
  a sentence rather than a heading, so the template stays intact.
- **A `TODO(user):` line never ships.** Right in a draft, wrong in a live PR. Resolve each at the
  Step 5 preview.

Everything else applies as written, including leaving a template's HTML comments in place. GitHub
renders those invisibly, so fill in around them.

## GitHub access (GitHub MCP with `gh` fallback)

Every GitHub call below is written as a `gh` command for reference. **Prefer the GitHub MCP
server when it is connected; use the `gh` command only as a fallback when no GitHub MCP is
available (or its tools don't cover the call).** Discover the MCP tools with
`ToolSearch "github pull request"` and call the operation matching the `gh` call:

| `gh` call used here | GitHub MCP equivalent (confirm exact name via ToolSearch) |
|---|---|
| `gh repo view --json defaultBranchRef` | get repository (default branch) |
| `gh pr view --json ...` | get pull request (existing-PR check) |
| `gh pr list --limit N --json title` | list pull requests (recent titles, for convention) |
| `gh pr create ...` | **create pull request**. The write (Step 7) |

Prefer the GitHub MCP when connected; fall back to `gh` only when no MCP is available. If
NEITHER a GitHub MCP nor `gh` is available, tell the user to connect a GitHub MCP or
install/authenticate `gh` (`gh auth login`), and stop. This skill cannot open a PR without one.
Local `git` calls (`git log`, `git diff`, `git push`, `git rev-parse`) need no `gh` and are
unaffected.

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
   If one exists with `state == "OPEN"`, abort, tell the user, and point them at the URL.
   They probably want to push more commits, not open a duplicate. (Closed or merged is fine.
   Proceed.)

## Step 1: Detect PR template

Look for a template, in this order:

1. `.github/PULL_REQUEST_TEMPLATE.md`
2. `.github/pull_request_template.md`
3. `docs/PULL_REQUEST_TEMPLATE.md`
4. `PULL_REQUEST_TEMPLATE.md` (repo root)
5. `.github/PULL_REQUEST_TEMPLATE/` directory containing multiple templates

If found:

- Read the template. Pass its path to `writing-work-docs` in Step 4 so it reads it too.
- An `<!-- HTML comment -->` is an instruction to the author, not a slot. Write your content below
  it and leave the comment where it is.
- A `{{ placeholder }}` is a slot. Replace it.
- For a multi-template directory: pick the file whose name best matches what the change is
  about (e.g. `feature.md` for new functionality, `bugfix.md` for fixes, `chore.md` for
  refactors). If unclear, ask the user which to use.

If NO template exists, fall back to the default format in Step 4.

## Step 2: Gather context

Gather it once here and hand it to `writing-work-docs` in Steps 3 and 4, so the title and the body
are written against identical facts.

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

**Invoke `writing-work-docs`** and give it the Step 2 context plus these constraints. A PR title is
one line of prose and it is the part most people read, so it gets the same treatment as the body.

- Single line, prefer < 70 chars, hard max 100.
- Imperative voice. "Add X", "Fix Y", "Refactor Z". Not "Adding X", not "I added X", not
  past-tense "Added X".
- State the change at user or system level, not the implementation detail.
- No PR number, no branch name, no ticket key, unless the project's existing PRs consistently
  include one.

## Step 4: Draft the description

**Invoke `writing-work-docs` for the body.** Hand it the Step 2 context, the Step 1 template path
(or the default format below when there is none), and the constraints from this step. Pass the path,
not a summary. It reads the template itself.

What comes back is paste-ready. Take it to Step 5 unchanged.

### If a template was found (Step 1)

Fill it in. These constraints go to `writing-work-docs` along with the template:

- Use bullets where the template offers a list.
- **Always include the one-line summary (Principle 6).** If the template has a description,
  summary, or overview field, put it there. If it has none, place the sentence at the very top of
  the body, above the first section. This is the one addition to a template's structure that is
  allowed, and it is the override named in the Writing section above.
- **A "How to test me" / "Testing" / "QA" field means MANUAL steps, not automated tests.**
  Give the steps a human follows to trigger the change and confirm it worked in a running
  environment. Shape: numbered steps such as (1) deploy to the env, (2) send `curl ...`,
  (3) expect response `...`, (4) check table X or the Amplitude event or the log line for Y.
  Cover both halves: how to trigger the change, and how to verify it landed. NEVER put
  unit-test or e2e-test run commands here (no `rushx test ...`, no `pnpm test`). If you can't
  infer concrete trigger-and-verify steps from the diff, ASK the user; they may tell you to
  drop the section entirely.

### If NO template was found

Use this exact format (note the lowercase `how`, which is intentional):

```markdown
### Goal

<1-2 sentences: the outcome we want. Link a ticket / Jira / Datadog / Linear / design doc when
one exists. Do NOT restate the title.>

### how      <!-- OMIT this whole section when the approach is obvious from the changes -->

<1 sentence: the key approach or decision. Skip it entirely if it adds nothing over the list.>

### Changes

- <key change 1>
- <key change 2>
- <key change 3>
```

Lead with `### Changes` directly, with no preamble sentence introducing the list. For a small
change, drop `### how` and often `### Goal` too. A one-liner above the list is enough (see
Principle 5).

### Rules for the `Changes` list

- **Not exhaustive.** Only key changes. If a refactor touches 12 files with the same kind of
  edit, that's ONE bullet, not 12.
- **5-7 bullets max.** If you have more, you're including too much detail.
- **One line per bullet.** Human-readable, what changed.
- **Call out unrelated bug fixes explicitly.** ✅ "Fix an unrelated bug where Y was returning
  Z instead of A".
- **Don't pad with refactor noise.** Renames, import cleanups, and format tweaks are not key
  changes.

#### Good examples

- Create a new SQS queue for invoice generation
- When an invoice is created, push a message to the new queue
- Consume the queue in the existing billing worker
- Fix an unrelated bug where invoice totals included tax twice

#### Bad examples (and why)

- ❌ "Rename `i` to `invoice` in `BillingWorker.process()`". Too granular, that's diff noise.
- ❌ "Update the billing module". Too vague, says nothing.
- ❌ "Refactor the entire invoice subsystem to support asynchronous PDF rendering via the
  newly-introduced SQS queue, including changes to the consumer, producer, message format,
  retry logic, and observability dashboards". A wall of text masquerading as a bullet.

### Goal section guidance

- 1-3 sentences. State the user-facing or system-facing outcome.
- Link the ticket when one exists.
- If the change is purely internal (refactor, infra cleanup), still describe WHAT it achieves.
  ✅ "Reduce billing worker memory footprint". ❌ "Improve code quality".

### how section guidance

- **Omit the whole section when the approach is obvious from the changes.** Don't pad.
- When kept: 1 sentence, the key architectural decision or approach only.
- Do NOT enumerate files or functions. The diff has those.

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

**Resolve every `TODO(user):` line here.** `writing-work-docs` emits one for anything it could not
verify, which is the right behavior for a draft and the wrong thing to publish. Ask the user for
each, put the answer in the body, and delete the marker. A body still carrying one is not ready.

Loop on edits until the user confirms or aborts. Do not skip this step. Opening a PR is a GitHub
write and the title and description outlive the conversation.

Send edits back through `writing-work-docs` rather than patching its prose by hand. Hand-editing is
how a banned word or a stray em-dash creeps back in after the skill already removed it.

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

**Prefer the GitHub MCP** to create the PR (see **GitHub access**): use a create-pull-request
tool with base = `<default-branch>`, head = the current branch, the drafted title, the drafted
body, and draft = true when requested, and report the URL it returns the same way. Fall back to
`gh pr create` only when no GitHub MCP is connected.

After `gh pr create` returns the URL, report it to the user:

```
✅ Opened PR: <url>
```

If `gh pr create` returns an error (network failure, base branch protected, etc.), surface
the exact error. Do not retry blindly.

## Constraints

- **Title under 70 chars when possible**, hard max 100.
- **Description short.** Default format = a `Goal` line, an OPTIONAL `how` line, and a
  non-exhaustive `Changes` list (<= 7 bullets). Tiny/obvious PRs: one line, no headers
  (Principle 5).
- **`writing-work-docs` writes the title and the body.** Never draft PR prose without it. It owns
  voice, banned words, ASCII punctuation, and wrapping. This skill owns structure. The only three
  places this skill overrides it are named in the Writing section.
- **Use the repo's PR template if one exists.** Don't substitute the default.
- **Always preview before posting** (Step 5). User must confirm.
- **Never create a PR if one already exists open** for the current branch.
- **No `TODO(user):` lines in a published body.** They are correct in a draft and wrong in a live
  PR. Resolve each one at the Step 5 preview.
- **No Co-Authored-By footers, no "🤖 Generated with Claude Code" footers, no agent
  branding** in the PR description. The user wants PRs that read like a human wrote them.
- **No conventional-commit prefixes** (`feat:`, `fix:`, etc.) unless the project's existing
  PRs consistently use them. Check via `gh pr list --limit 20`.
- **Don't enumerate every file changed.** The diff is right there.
- **No fabricated ticket links.** If you don't know whether a ticket exists, ask.
- **Single PR per invocation.** Don't open multiple PRs in one skill run.
