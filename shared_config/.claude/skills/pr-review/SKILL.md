---
name: pr-review
description: >
  Reviews a pull request by launching TEN concurrent reviewer sub-agents — FIVE instances
  of `in-depth-review` AND FIVE instances of `gh-style-review` — all running in parallel
  on the same PR. Each sub-agent is invoked with `--raw` so it skips its internal confidence
  filter and returns every scored finding to this orchestrator. The orchestrator merges +
  deduplicates the ten result sets into one flat pool, keeps findings with merged confidence
  score >= 60, then classifies each surviving finding as INLINE (narrow, points at specific
  lines in the diff) or GLOBAL (broad, multi-file, architectural). The `gh-style-review`
  instances also return Discussion Context (which prior human comments the diff resolves vs.
  still leaves open) — the orchestrator aggregates that across instances and surfaces it
  inside the posted PR review body. All findings + Discussion Context are posted as a
  SINGLE PR review: the review body contains the global findings, the Discussion Context
  section, and a meta-line counting any inline comments; inline comments are attached to
  the relevant diff lines. If nothing survives, posts NOTHING to GitHub and reports the
  clean result only in chat.
  Use this skill when the user asks to "review this PR", "pr review", "triple code review",
  "ensemble code review", "consensus review", a "thorough review of this PR", or wants
  higher-confidence PR feedback than a single `/in-depth-review` run would produce.
---

# PR Review (10x-reviewed)

This skill orchestrates **ten** parallel reviewer sub-agents against a single PR: **five
instances of `in-depth-review`** (each runs ten roles by default, or nine with `--skip-ticket`; all raw scored findings) and
**five instances of `gh-style-review`** (the `@claude review` GitHub Action prompt
replicated locally, which adds Discussion Context — prior-human-comment cross-referencing
— on top of standard findings). All ten are invoked with `--raw`; the orchestrator merges
+ deduplicates the ten result sets into one flat pool, applies a final **score ≥ 60**
filter, classifies each surviving finding as INLINE or GLOBAL, aggregates `discussion_context`
across the five gh-style instances, and posts a single PR review whose body carries the
GLOBAL findings + the aggregated Discussion Context. Inline comments are attached to the
diff lines they refer to.

The point: ten independent passes from two different prompt structures (specialized-role
vs. GitHub-Action mirror) catch different issues, AND converge on the real ones. One
review entry, ten reviewers' worth of recall, plus an explicit "what humans already raised
and whether the diff addresses it" section.

## Prerequisites

- The current branch (or the PR specified as the skill argument) must have an **open PR**.
  Draft PRs are accepted — reviewing a draft is a valid workflow (you get feedback before
  marking the PR ready). Both `in-depth-review` and `gh-style-review` accept drafts.
- Both the `in-depth-review` and `gh-style-review` skills must be installed and available
  (they live alongside this skill in `shared_config/.claude/skills/`).
- `gh` must be installed and authenticated.

**Flag:** pass `--skip-ticket` to disable ticket intent compliance (Role #10) across all
five `in-depth-review` instances and skip the Jira-tooling preflight.

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
4. Re-confirm that **both** `in-depth-review` AND `gh-style-review` are available — check the
   available-skills list for both entries. If either is missing, abort the orchestration
   and tell the user which one to install.
5. If the invocation included `--skip-ticket`, set `<SKIP_TICKET> = true` (default `false`).
   When `true`, every in-depth-review sub-agent is invoked with `--skip-ticket`, so Role #10
   never runs. When `false`, all five in-depth-review instances run Role #10 (five ticket
   reviewers). `gh-style-review` is unaffected either way — it has no ticket role.
6. **Jira-tooling preflight** (skip this step entirely if `<SKIP_TICKET>` is true). Before
   launching any reviewers, confirm a Jira reader is available AND authenticated:
   - acli: installed (`command -v acli`) and able to read Jira — run a lightweight
     authenticated acli call; if it fails with an auth/login error, treat acli as
     unauthenticated; OR
   - a Jira/Atlassian MCP: connected and authenticated — search available tools (e.g.
     ToolSearch "atlassian jira"); if the only exposed tool is an `authenticate` tool, it is
     connected but not yet authed.
   If neither is ready, ASK the user to choose:
     (a) install/authenticate acli or the Atlassian MCP, then continue — re-check after they confirm;
     (b) proceed now with `--skip-ticket` — set `<SKIP_TICKET> = true` and run the other
         reviewers without the ticket check;
     (c) abort the review.
   Do not launch any reviewers until this is resolved. If a re-check after choice (a) still
   fails, present the three choices again rather than proceeding.

## Step 1: Launch ten reviewer sub-agents in parallel

Spawn **ten sub-agents in a single message** (ten concurrent Agent tool calls). Sequential
launches defeat the purpose — never serialize. The split is:

- **Sub-agents 1–5:** invoke `in-depth-review`
- **Sub-agents 6–10:** invoke `gh-style-review`

### Sub-agents 1–5 prompt (in-depth-review)

```
You are sub-agent N of 10 in a pr-review orchestration (N is 1, 2, 3, 4, or 5). Your job:
perform one independent in-depth review of PR #<PR> by invoking the `in-depth-review`
skill, then return its result to me unchanged.

Concretely:

1. Invoke the `in-depth-review` skill with the arguments: `<PR> --raw` — and append
   ` --skip-ticket` when the orchestrator's `<SKIP_TICKET>` is true (so the args become
   `<PR> --raw --skip-ticket`). When `<SKIP_TICKET>` is false, pass `<PR> --raw` unchanged
   so Role #10 runs.
   - The `<PR>` arg puts it in PR mode against this PR.
   - The `--raw` flag tells in-depth-review to skip its internal <70 confidence filter so we
     get every scored finding. The orchestrator will apply its own >=60 threshold.
2. Wait for `in-depth-review` to finish and return its structured JSON output.
3. Return that JSON verbatim, with two additions at the top level:
   - `"sub_agent": N` (which of the 10 instances you are)
   - `"source": "in-depth-review"` (so the orchestrator can attribute findings)

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

### Sub-agents 6–10 prompt (gh-style-review)

```
You are sub-agent N of 10 in a pr-review orchestration (N is 6, 7, 8, 9, or 10). Your job:
perform one independent gh-style review of PR #<PR> by invoking the `gh-style-review`
skill, then return its result to me unchanged.

Concretely:

1. Invoke the `gh-style-review` skill with the arguments: `<PR> --raw`
   - The `<PR>` arg puts it in PR mode against this PR.
   - The `--raw` flag tells gh-style-review to skip its internal <70 confidence filter so we
     get every scored finding. The orchestrator will apply its own >=60 threshold.
2. Tell gh-style-review you are invoking it as a sub-agent — it must return the JSON shape
   documented in its "If invoked as a sub-agent" section, NOT its terminal-formatted output.
3. Wait for gh-style-review to finish and return its structured JSON output.
4. Return that JSON verbatim, with two additions at the top level:
   - `"sub_agent": N` (which of the 10 instances you are)
   - `"source": "gh-style-review"` (so the orchestrator can attribute findings)

Forbidden:
- `gh pr comment` (any form)
- `gh pr review` (any form)
- `gh pr edit`, `gh pr close`, `gh pr merge`
- `gh issue create`, `gh issue comment`
- Any other command that writes to GitHub

`gh-style-review` itself is read-only with respect to GitHub. If something inside it appears
about to issue a write, abort and surface the reason to me instead of proceeding.

If `gh-style-review` refuses to proceed (closed PR, missing skill, etc.), return its
`skipped_reason` field unchanged so the orchestrator can report it.
```

### Why each sub-agent uses `--raw`

Both `in-depth-review` and `gh-style-review` default to discarding anything `< 70`. This
orchestrator's threshold is **60** (lower than each sub-skill's default because the 10×
cross-instance triangulation — five from each prompt structure — raises confidence in
60-69 findings). `--raw` makes the sub-agents return all scored findings; we apply the
60 cutoff in Step 2 after merging.

## Step 2: Merge and deduplicate (findings)

Once all ten sub-agents have returned:

1. **Pool every finding** across the ten result sets into one flat pool. Each finding carries
   its `confidence`, `file`, `line_range`, originating `sub_agent` (1..10), and `source`
   (`"in-depth-review"` or `"gh-style-review"`). Don't pre-segregate by source — the cross-
   prompt triangulation is the point.

2. **Group duplicates.** Two findings are duplicates if they refer to the **same file** and have
   **overlapping line ranges** AND describe substantially the same problem (paraphrases count).
   Findings from different sources (one from in-depth-review, one from gh-style-review) that
   describe the same problem are duplicates — merge them.

3. **For each group, produce one merged finding:**
   - `confidence`: **max** of the group's scores (any one reviewer with high confidence is strong
     evidence; merging by max is intentionally non-conservative).
   - `agreement`: count of distinct sub-agents (1..10) that raised this finding.
   - `sources`: set of distinct sources (`{"in-depth-review"}`, `{"gh-style-review"}`, or both).
     A finding raised by both sources is stronger signal than a finding raised by only one;
     used as a tiebreaker in step 5.
   - `title`, `description`, `suggested_fix`: pick the clearest from the group; if suggested
     fixes differ meaningfully, mention the alternatives in the description.
   - `category`: union of categories.
   - `permalink`: take any one valid permalink from the group.
   - `ticket_id`: preserved from `ticket`-category findings (the Jira ID the gap traces to);
     `null` for all other findings. Never merge two findings that name different `ticket_id`s.

4. **Filter:** keep only findings with `confidence >= 60`. Discard the rest. (Threshold is 60.)

5. **Order the surviving findings:**
   1. `confidence` descending
   2. `agreement` descending (10/10 > 5/10 > 1/10 when scores tie)
   3. Both-sources first (a finding raised by both in-depth-review and gh-style-review beats
      a same-confidence-and-agreement finding from a single source)
   4. `category` priority: bug > AGENTS.md > history > prior PR > comment guidance > ticket

## Step 2.5: Aggregate Discussion Context (from gh-style sub-agents only)

`gh-style-review` sub-agents each return a `discussion_context` block with `resolved` and
`unaddressed` arrays. `in-depth-review` returns no such block — skip those instances here.

1. **Pool every entry** across the five `gh-style-review` result sets into two flat lists:
   `resolved_pool` and `unaddressed_pool`. Each entry carries `quote`, `author`, `url`, and
   either `resolution` or `gap`.

2. **Deduplicate by `url`** (the GitHub comment URL is the canonical identity of a discussion
   item). If the same URL appears in both `resolved` and `unaddressed` across instances —
   reviewers disagree on whether the diff fixes it — keep it in `unaddressed_pool` and note
   the disagreement count.

3. **For each deduplicated entry** pick the clearest `quote`, `resolution`, and `gap` text
   across instances (longest non-trivial wording usually wins). Record `agreement` =
   instance count that raised this exact discussion item.

4. **Retain all entries** — there's no confidence threshold here because every entry is
   grounded in a real human comment URL. Order each list by `agreement` desc, then by the
   comment's `created_at` ascending (oldest unresolved concern first).

5. **If either pool is empty after dedup, mark that subsection skipped** for the Step 3b
   body construction. If both pools are empty AND the findings list from Step 2 is also
   empty, Step 3 still applies — no review is posted.

## Step 3: Post the review (only if there is something worth posting)

**If BOTH the merged-and-filtered findings list AND both Discussion Context pools are
EMPTY, do NOT post anything to GitHub.** Skip directly to Step 4 and tell the user in chat
that the PR looks clean. No review, no comment, no PR state change — silence on GitHub is
the success signal.

**If at least one of {findings, resolved_pool, unaddressed_pool} is non-empty**, post a
single PR review that combines a global body with any inline comments. This is a single
atomic write to GitHub.

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

### Step 3a.5: Aggregate tickets_examined

`tickets_examined` = union by `id` of the `tickets_examined` arrays returned by the five
in-depth-review sub-agents. For each `id`, `status` is `gaps` if any instance reported gaps,
else `unread` if any reported unread, else `ok`. The `gaps` count = number of surviving ticket
findings for that `id` in the merged, deduplicated pool (after the Step 2 ≥60 filter).

### Step 3b: Build the global body

The global body **must start** with the disclosure line below — verbatim, as the first
paragraph, on its own line. Do not change its wording. Do not append any branding footer,
"Generated with Claude Code" line, or ensemble-stats `<sub>` tag at the end.

Let:
- `K_global` = count of GLOBAL findings
- `K_inline` = count of INLINE findings
- `K_resolved` = count of entries in `resolved_pool`
- `K_unaddressed` = count of entries in `unaddressed_pool`

```markdown
I used an AI agent with a custom prompt to generate this review.

### Code review

<<if K_global > 0:>>
Found <K_global> issue(s) across 10 independent reviews (5 × in-depth-review + 5 × gh-style-review, threshold ≥ 60):

1. <title> &nbsp;`[agreement: <N>/10, confidence: <score>, sources: <in-depth|gh-style|both>]`

   <description, including category and any suggested-fix alternatives>

   <permalink>

2. ...
   <<endif>>

<<if tickets_examined is non-empty:>>

### Tickets examined

- <id> — ✅ implemented &nbsp;`[no gaps]`
- <id> — ⚠️ <N> gap(s) found (see findings above)
- <id> — ❓ could not read
   <<endif>>

<<if K_unaddressed > 0:>>

### Still unaddressed in this PR

Concerns raised by reviewers earlier that this PR does not appear to address:

- > <quote> — @<author> ([link](<url>))
  ⚠️ <gap> &nbsp;`[agreement: <N>/5]`
- ...
   <<endif>>

<<if K_resolved > 0:>>

### Addressed by this PR

Earlier concerns that this PR resolves:

- > <quote> — @<author> ([link](<url>))
  ✅ <resolution> &nbsp;`[agreement: <N>/5]`
- ...
   <<endif>>

<<if K_global > 0 AND K_inline > 0:>>

I also left <K_inline> comment(s) for different findings in the diff.
<<elif K_global == 0 AND K_inline > 0:>>

I left <K_inline> comment(s) on the diff.
<<endif>>
```

A global body is ALWAYS produced when there is at least one finding, one inline comment, or
one Discussion Context entry. The disclosure + `### Code review` header are unconditional;
the section blocks (Still unaddressed, Addressed by this PR, inline-meta-line) are emitted
only when their respective counts are > 0.

**Section order is fixed:** code review findings → tickets examined → still unaddressed → addressed → inline
meta-line. "Still unaddressed" comes before "Addressed" because it's the actionable half.

**Ticket findings:** when a finding has a non-null `ticket_id`, prepend `[<ticket_id>] ` to
its `<title>` in both the global body and any inline comment, so the ticket is visible.

### Step 3c: Build each inline comment

Each inline comment carries a tighter body (no disclosure repetition, no permalink — GitHub
anchors the comment to the line for you):

```markdown
**<title>** `[agreement: <N>/10, confidence: <score>, sources: <in-depth|gh-style|both>]`

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

**If at least one finding or Discussion Context entry was posted (review issued):**

- The PR URL.
- How many findings each sub-agent originally returned (pre-merge counts), broken down by
  source: `5 × in-depth-review: [N1, N2, N3, N4, N5]`, `5 × gh-style-review: [N6..N10]`.
- How many unique findings survived the ≥ 60 filter (post-merge count), broken down as
  GLOBAL vs INLINE, and how many were both-source vs single-source.
- How many Discussion Context entries surfaced: `resolved: K`, `unaddressed: K`.
- Sub-agents that returned `skipped_reason` (if any) and why.
- The PR review's HTML URL (the `html_url` returned by the `gh api .../reviews` response).
- Any inline comments that had to be demoted to GLOBAL because GitHub rejected them
  (line not in diff), with a brief explanation.
- Tickets examined and their outcome: `<id> ✅ | ⚠️ N gaps | ❓ unread`. If any in-depth-review
  sub-agent reported `ticket_review.status` of `denied` (user denied access) or `unavailable`
  (no Jira tooling), say so explicitly.

**If no review was posted (clean PR):**

- Lead with a clear "all clear" line, e.g. ✅ `PR #<PR> looks good — ten independent reviewers
(5 × in-depth + 5 × gh-style) raised no findings at confidence ≥ 60 and no unaddressed
discussion items. Nothing posted to GitHub.`
- The PR URL.
- How many findings each sub-agent originally returned (pre-merge counts), broken down by
  source as above.
- How many were filtered out by the ≥ 60 threshold (so the user can see whether reviewers
  flagged anything below the bar).
- Sub-agents that returned `skipped_reason` (if any) and why.
- Tickets examined and their outcome: `<id> ✅ | ⚠️ N gaps | ❓ unread`. If any in-depth-review
  sub-agent reported `ticket_review.status` of `denied` (user denied access) or `unavailable`
  (no Jira tooling), say so explicitly.

## Constraints

- **At most one PR review per run.** The orchestrator posts a single `POST .../pulls/<PR>/reviews`
  call (which atomically carries the global body AND all inline comments) — only when there is
  at least one surviving finding ≥ 60 OR at least one Discussion Context entry. When all
  three pools are empty, the orchestrator posts NOTHING. Either way: no `gh pr comment`,
  no `gh pr edit`, no extra comments, no second review.
- **The review event MUST be `COMMENT`.** Never `APPROVE`, never `REQUEST_CHANGES`. This skill
  comments; it does not gate merges.
- **Sub-agents are read-only with respect to GitHub.** They invoke `in-depth-review` or
  `gh-style-review` (both write-free) and just relay scored findings back. If any sub-agent
  appears about to issue a GitHub write, abort the entire orchestration and surface to the
  user — do not proceed to post the merged review, since the inner skill could have posted
  from a sub-agent.
- **Ten parallel sub-agents (5 × in-depth-review + 5 × gh-style-review).** Launch all ten
  in a single message with concurrent Agent tool calls. Do not fall back to fewer sub-agents
  for "speed"; the cross-source triangulation is the point. Do not use only one source —
  both prompt structures contribute distinct findings.
- **Threshold is 60.** Do not raise or lower it on the fly.
- **Flat-pool merging.** Findings from in-depth-review and gh-style-review are merged into
  one pool, dedup'd by file+line+description regardless of source, then filtered. Do not
  keep the two sources separate in the final output (they're attributed via `sources` on
  each finding, but ranking/filtering treats them as one pool).
- **Discussion Context comes only from gh-style-review.** in-depth-review has no equivalent
  block. Do not attempt to synthesize Discussion Context from in-depth-review findings.
- **One PR per run.** This skill targets a single PR; do not iterate across multiple PRs in one
  invocation.
- **No fix application.** This skill posts feedback only. For the iterate-and-fix flow, use
  the `review-and-fix` skill instead.
- **Always produce a global body when there is anything to post.** Even if every surviving
  finding was inline-eligible and there are no Discussion Context entries, the review's
  `body` field must contain the disclosure + `### Code review` header + the meta-line
  "I left X comment(s) on the diff." Never send a review with an empty `body`.
