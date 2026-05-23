---
name: pr-review
description: >
  Reviews a pull request by launching FIVE concurrent instances of the `in-depth-review` skill
  as sub-agents, each performing an independent review of the same PR. Each sub-agent is
  invoked with `--raw` so it skips its internal <80 confidence filter; the sub-agents return
  their raw scored findings to this orchestrator. The orchestrator merges + deduplicates the
  five result sets, keeps findings with merged confidence score >= 60, then classifies each
  surviving finding as INLINE (narrow, points at specific lines in the diff) or GLOBAL
  (broad, multi-file, architectural). All findings are posted as a SINGLE PR review: the
  review body contains the global findings + a meta-line counting any inline comments, and
  inline comments are attached to the relevant diff lines. If nothing survives, posts NOTHING
  to GitHub and reports the clean result only in chat.
  Use this skill when the user asks to "review this PR", "pr review", "triple code review",
  "ensemble code review", "consensus review", a "thorough review of this PR", or wants
  higher-confidence PR feedback than a single `/in-depth-review` run would produce.
---

# PR Review (5x-reviewed)

This skill orchestrates five independent instances of `in-depth-review` against a single PR.
Each instance is invoked with `--raw` and returns its raw 0–100 confidence-scored findings; the
orchestrator merges + deduplicates the five result sets, applies a final **score ≥ 60** filter,
classifies each surviving finding as INLINE or GLOBAL, and posts a single PR review whose body
carries the GLOBAL findings + inline comments are attached to the diff lines they refer to.

The point: five independent passes catch different issues (LLM stochasticity) _and_ converge on
real issues (agreement is signal). One review entry, five reviewers' worth of recall, with
line-anchored comments where they belong.

## Prerequisites

- The current branch (or the PR specified as the skill argument) must have an **open PR**.
  Draft PRs are accepted — reviewing a draft is a valid workflow (you get feedback before
  marking the PR ready). `in-depth-review` accepts drafts too.
- The `in-depth-review` skill must be installed and available (it lives alongside this skill in
  `shared_config/.claude/skills/in-depth-review`).
- `gh` must be installed and authenticated.

## Step 0: Resolve the PR

1. If the skill received an argument that looks like a PR number (e.g. `123` or `#123`), use it
   directly.
2. Otherwise, detect the PR for the current branch:
   ```
   gh pr view --json number,state,isDraft,url
   ```
   If exit is non-zero or `state != "OPEN"`, abort and tell the user why. **Draft PRs are
   accepted**; note `isDraft = true` so the final-report step (Step 4) can mention "PR is in
   draft" alongside the review URL.
3. Save the resolved PR number as `<PR>` and the draft flag as `<IS_DRAFT>`.
4. Re-confirm the `in-depth-review` skill is available — check the available-skills list for an
   `in-depth-review` entry.

## Step 1: Launch five `in-depth-review` sub-agents in parallel

Spawn **five sub-agents in a single message** (five concurrent Agent tool calls). Each receives
this prompt verbatim, with `<PR>` substituted:

```
You are sub-agent N of 5 in a pr-review orchestration. Your job: perform one independent
in-depth review of PR #<PR> by invoking the `in-depth-review` skill, then return its result
to me unchanged.

Concretely:

1. Invoke the `in-depth-review` skill with the arguments: `<PR> --raw`
   - The `<PR>` arg puts it in PR mode against this PR.
   - The `--raw` flag tells in-depth-review to skip its internal <80 confidence filter so we
     get every scored finding. The orchestrator will apply its own >=60 threshold.
2. Wait for `in-depth-review` to finish and return its structured JSON output.
3. Return that JSON verbatim, with one addition: include `"sub_agent": N` at the top level so
   the orchestrator can tell which of the 5 instances you are.

Forbidden:
- `gh pr comment` (any form)
- `gh pr review` (any form)
- `gh pr edit`, `gh pr close`, `gh pr merge`
- `gh issue create`, `gh issue comment`
- Any other command that writes to GitHub

`in-depth-review` itself is read-only with respect to GitHub. If something inside it appears
about to issue a write, abort and surface the reason to me instead of proceeding.

If `in-depth-review` refuses to proceed (closed/merged PR, or other ineligibility), return its
`skipped_reason` field unchanged so the orchestrator can report it.
```

### Why each sub-agent uses `--raw`

`in-depth-review` defaults to discarding anything `< 80`. This orchestrator's threshold is
**60** (lower than the default because cross-instance triangulation across 5 passes raises our
confidence in 60-79 findings). `--raw` makes the sub-agents return all scored findings; we
apply the 60 cutoff in Step 2 after merging.

## Step 2: Merge and deduplicate

Once all five sub-agents have returned:

1. **Pool every finding** across the five result sets. Each finding carries its `confidence`,
   `file`, `line_range`, and originating `sub_agent` (1..5).

2. **Group duplicates.** Two findings are duplicates if they refer to the **same file** and have
   **overlapping line ranges** AND describe substantially the same problem (paraphrases count).

3. **For each group, produce one merged finding:**
   - `confidence`: **max** of the group's scores (any one reviewer with high confidence is strong
     evidence; merging by max is intentionally non-conservative).
   - `agreement`: count of distinct sub-agents (1..5) that raised this finding.
   - `title`, `description`, `suggested_fix`: pick the clearest from the group; if suggested
     fixes differ meaningfully, mention the alternatives in the description.
   - `category`: union of categories (one finding can be both "bug" and "CLAUDE.md adherence").
   - `permalink`: take any one valid permalink from the group.

4. **Filter:** keep only findings with `confidence >= 60`. Discard the rest. (Threshold is 60.)

5. **Order the surviving findings:**
   1. `confidence` descending
   2. `agreement` descending (5/5 > 4/5 > 3/5 > 2/5 > 1/5 when scores tie)
   3. `category` priority: bug > CLAUDE.md adherence > history > prior PR > comment guidance

## Step 3: Post the review (only if there are findings)

**If the merged-and-filtered findings list is EMPTY, do NOT post anything to GitHub.** Skip
directly to Step 4 and tell the user in chat that the PR looks clean. No review, no comment,
no PR state change — silence on GitHub is the success signal.

**If the list is non-empty**, post a single PR review that combines a global body with any
inline comments. This is a single atomic write to GitHub.

### Step 3a: Classify each surviving finding as INLINE or GLOBAL

A finding is **INLINE-eligible** if ALL of these hold:

- `file` is a single, valid path that is in the PR's changed files. Verify with:
  ```
  gh pr diff <PR> --name-only
  ```
- `line_range` parses to a `<start>..<end>` (or single line) range with `start <= end`.
- The lines lie inside an actual diff hunk on the **RIGHT side** (i.e. added or context lines
  in the new revision). If a finding's lines are not in the diff hunks at all, the GitHub API
  will reject the inline comment — demote to GLOBAL.
- The finding describes a defect at that specific location, not a cross-cutting / architectural
  concern that happens to be visible there.

A finding is **GLOBAL** if any of those checks fails — typical reasons:

- No file or no usable line range
- Lines aren't in the diff hunks
- Concern spans multiple files
- It's architectural ("split this into a separate module"), about missing tests, missing docs,
  or otherwise too broad to anchor at one location

Don't be overly aggressive demoting to GLOBAL: the whole point of inline comments is reviewer
ergonomics. When in doubt, prefer INLINE if a single line range is identifiable.

### Step 3b: Build the global body

The global body **must start** with the disclosure line below — verbatim, as the first
paragraph, on its own line. Do not change its wording. Do not append any branding footer,
"Generated with Claude Code" line, or ensemble-stats `<sub>` tag at the end.

Let `K_global` = count of GLOBAL findings, `K_inline` = count of INLINE findings.

```markdown
I used an AI agent with a custom prompt to generate this review.

### Code review

<<if K_global > 0:>>
Found <K_global> issue(s) across 5 independent reviews (threshold ≥ 60):

1. <title> &nbsp;`[agreement: <N>/5, confidence: <score>]`

   <description, including category and any suggested-fix alternatives>

   <permalink>

2. ...
   <<endif>>
   <<if K_global > 0 AND K_inline > 0:>>

I also left <K_inline> comment(s) for different findings in the diff.
<<elif K_global == 0 AND K_inline > 0:>>

I left <K_inline> comment(s) on the diff.
<<endif>>
```

A global body is ALWAYS produced when there is at least one finding (even when all findings are
INLINE — in that case the body contains only the disclosure, the `### Code review` header, and
the meta-line "I left X comment(s) on the diff.").

### Step 3c: Build each inline comment

Each inline comment carries a tighter body (no disclosure repetition, no permalink — GitHub
anchors the comment to the line for you):

```markdown
**<title>** `[agreement: <N>/5, confidence: <score>]`

<description, including any suggested-fix alternatives>
```

GitHub line-range encoding:

- Single line: `{ "path": "<file>", "line": <N>, "side": "RIGHT", "body": "<body>" }`
- Multi-line: `{ "path": "<file>", "start_line": <S>, "start_side": "RIGHT", "line": <E>, "side": "RIGHT", "body": "<body>" }`

`side` is always `RIGHT` (the new revision). Comments on the LEFT side (deleted lines) are
seldom useful for a forward-looking review and are out of scope here.

### Step 3d: Post the review (one API call)

Fetch the PR head SHA and the owner/repo:

```sh
PR_HEAD_SHA=$(gh pr view <PR> --json headRefOid --jq .headRefOid)
OWNER_REPO=$(gh repo view --json owner,name --jq '.owner.login + "/" + .name')
```

Build the JSON payload (single review with body + inline comments):

```json
{
  "event": "COMMENT",
  "commit_id": "<PR_HEAD_SHA>",
  "body": "<global body from Step 3b>",
  "comments": [
    { "path": "...", "line": ..., "side": "RIGHT", "body": "..." },
    { "path": "...", "start_line": ..., "start_side": "RIGHT", "line": ..., "side": "RIGHT", "body": "..." }
  ]
}
```

`event=COMMENT` is important — it leaves the review as comments, not as approval or
change-requested. This skill never approves a PR or requests changes; it only comments.

Post it:

```sh
gh api -X POST "/repos/$OWNER_REPO/pulls/<PR>/reviews" --input - <<EOF
<rendered JSON payload>
EOF
```

If the API responds 422 because one or more inline comments target lines not in the diff,
demote those specific comments to GLOBAL (append them to the global body in a "Couldn't anchor
inline" subsection) and re-issue the call. Do not silently drop findings.

**This is the ONLY GitHub write this skill is permitted to perform**, and only when there is at
least one surviving finding. Do not edit the PR, add reviewers, change labels, request changes,
approve, or alter any other PR state.

## Step 4: Final report (to the user, not GitHub)

Summarise to the user in chat — this report happens whether or not a review was posted.

If `<IS_DRAFT>` was true, prepend a short note: `ℹ️ PR #<PR> is still a draft.` so the user
remembers their PR isn't ready-for-review yet.

**If at least one finding was posted (review issued):**

- The PR URL.
- How many findings each sub-agent originally returned (pre-merge counts).
- How many unique findings survived the ≥ 60 filter (post-merge count), broken down as
  GLOBAL vs INLINE.
- Sub-agents that returned `skipped_reason` (if any) and why.
- The PR review's HTML URL (the `html_url` returned by the `gh api .../reviews` response).
- Any inline comments that had to be demoted to GLOBAL because GitHub rejected them
  (line not in diff), with a brief explanation.

**If no review was posted (clean PR):**

- Lead with a clear "all clear" line, e.g. ✅ `PR #<PR> looks good — five independent reviewers
raised no findings at confidence ≥ 60. Nothing posted to GitHub.`
- The PR URL.
- How many findings each sub-agent originally returned (pre-merge counts).
- How many were filtered out by the ≥ 60 threshold (so the user can see whether reviewers
  flagged anything below the bar).
- Sub-agents that returned `skipped_reason` (if any) and why.

## Constraints

- **At most one PR review per run.** The orchestrator posts a single `POST .../pulls/<PR>/reviews`
  call (which atomically carries the global body AND all inline comments) — only when there is
  at least one surviving finding ≥ 60. When the merged-filtered list is empty, the orchestrator
  posts NOTHING. Either way: no `gh pr comment`, no `gh pr edit`, no extra comments, no second
  review.
- **The review event MUST be `COMMENT`.** Never `APPROVE`, never `REQUEST_CHANGES`. This skill
  comments; it does not gate merges.
- **Sub-agents are read-only with respect to GitHub.** They invoke `in-depth-review` (which is
  itself GitHub-write-free) and just relay its scored findings back. If any sub-agent appears
  about to issue a GitHub write, abort the entire orchestration and surface to the user — do
  not proceed to post the merged review, since the inner skill could have posted from a
  sub-agent.
- **Five parallel sub-agents.** Launch in a single message with concurrent Agent tool calls.
  Do not fall back to fewer sub-agents.
- **Threshold is 60.** Do not raise or lower it on the fly.
- **One PR per run.** This skill targets a single PR; do not iterate across multiple PRs in one
  invocation.
- **No fix application.** This skill posts feedback only. For the iterate-and-fix flow, use
  the `review-and-fix` skill instead.
- **Always produce a global body when there are findings.** Even if every surviving finding
  was inline-eligible, the review's `body` field must contain the disclosure + `### Code review`
  header + the meta-line "I left X comment(s) on the diff." Never send a review with an empty
  `body`.
