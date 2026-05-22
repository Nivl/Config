---
name: pr-review
description: >
  Reviews a pull request by launching FIVE concurrent instances of the upstream `code-review`
  skill as sub-agents, each performing an independent review of the same PR. Each sub-agent
  is instructed to STOP before posting (so it never calls `gh pr comment`); the sub-agents
  return their scored findings to this orchestrator. The orchestrator merges + deduplicates
  the five result sets and keeps findings with merged confidence score >= 60. If at least
  one finding survives, it posts a single consolidated comment to the PR. If nothing
  survives, it posts NOTHING to GitHub and reports the clean result only in chat.
  Triangulates across five independent runs while producing at most one PR comment.
  Use this skill when the user asks to "review this PR", "pr review", "triple code review",
  "ensemble code review", "consensus review", a "thorough review of this PR", or wants
  higher-confidence PR feedback than a single `/code-review` run would produce.
---

# PR Review (5x-reviewed)

This skill orchestrates five independent instances of the upstream `code-review` skill against a
single PR. Each instance scores its findings 0–100; the orchestrator merges + deduplicates the
five result sets, applies a final **score ≥ 60** filter, and posts a single comment.

The point: five independent runs of the same skill catch different issues (LLM stochasticity)
*and* converge on real issues (agreement is signal). One comment, five reviewers' worth of recall.

## Prerequisites

- The current branch (or the PR specified as the skill argument) must have an **open, non-draft
  PR**. This is a hard requirement of the upstream `code-review` skill — it returns early on
  closed/merged/draft PRs.
- The upstream `code-review` skill must be installed and available. If it isn't, abort with a
  message telling the user to `/plugin install code-review@claude-code-plugins`.
- `gh` must be installed and authenticated.

## Step 0: Resolve the PR

1. If the skill received an argument that looks like a PR number (e.g. `123` or `#123`), use it
   directly.
2. Otherwise, detect the PR for the current branch:
   ```
   gh pr view --json number,state,isDraft,url
   ```
   If exit is non-zero, or `state != "OPEN"`, or `isDraft = true`, abort and tell the user why.
3. Save the resolved PR number as `<PR>`.
4. Re-confirm the upstream `code-review` skill is available — check the available-skills list for
   a `code-review` entry (under any plugin namespace).

## Step 1: Launch five `code-review` sub-agents in parallel

Spawn **five sub-agents in a single message** (five concurrent Agent tool calls). Each receives
this prompt verbatim, with `<PR>` substituted:

```
You are sub-agent N of 5 in a pr-review orchestration (a 5x-code-review ensemble). Your job is to perform an
independent code review of PR #<PR> by invoking the `code-review` skill, BUT you must stop
before any GitHub write step and return your structured findings to me instead of posting.

Concretely:

1. Invoke the `code-review` skill via the Skill tool, targeting PR #<PR>.
2. Let it execute its internal steps 1–5 normally:
     - eligibility check
     - relevant CLAUDE.md / AGENTS.md file path collection
     - PR summary
     - 5 parallel review-agent passes (CLAUDE.md compliance, shallow bug scan, git history,
       previous PR comments, in-file code comments)
     - per-finding confidence scoring (0–100)
3. **Intercept BEFORE step 6.** Do NOT apply the upstream skill's score-<80 filter. Do NOT
   run step 7 (re-eligibility) or step 8 (post comment).

Specifically forbidden:
- `gh pr comment` (any form)
- `gh pr review` (any form)
- `gh pr edit`, `gh pr close`, `gh pr merge`
- `gh issue create`, `gh issue comment`
- Any other command that writes to GitHub
If `code-review`'s internal logic appears to be about to invoke one of these, abort and
return the abort reason to me instead of proceeding.

Return to me a structured JSON object with this shape:

{
  "pr": <PR>,
  "sub_agent": N,
  "summary": "<short PR summary from step 3>",
  "findings": [
    {
      "id": "<stable id, e.g. file:line-or-range>",
      "title": "<one-line description>",
      "file": "<path>",
      "line_range": "<L<start>-L<end>>",
      "category": "<bug | CLAUDE.md adherence | history | prior PR | comment guidance | other>",
      "description": "<full text of the finding>",
      "suggested_fix": "<text or code snippet>",
      "confidence": <0..100>,
      "permalink": "<github blob URL with full SHA + line range, per code-review's format>"
    },
    ...
  ]
}

Include EVERY finding, regardless of confidence — the orchestrator applies the final
threshold. If the upstream skill returns "no issues" or refuses to proceed (closed/draft/
already-reviewed), return:

{ "pr": <PR>, "sub_agent": N, "summary": "...", "findings": [], "skipped_reason": "<text>" }
```

### Why each sub-agent must skip the upstream filter

The upstream `code-review` skill defaults to discarding anything < 80 at its step 6. This
orchestrator's threshold is **60**, so we need raw scored findings. The orchestrator applies the
60 cutoff *after* merging across the five runs.

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

## Step 3: Post the comment (only if there are findings)

**If the merged-and-filtered findings list is EMPTY, do NOT post anything to GitHub.** Skip
directly to Step 4 and tell the user in chat that the PR looks clean. No comment, no PR state
change, nothing — silence on GitHub is the success signal.

**If the list is non-empty**, build a single Markdown comment in the same shape as the upstream
`code-review` skill (so reviewers see a familiar format), with these adaptations:

- The first line is a disclosure that an AI agent generated the review (verbatim, see template).
- The summary line states the ensemble size and threshold ("across 5 independent reviews...").
- Findings are numbered.
- Each finding shows its agreement count and merged confidence score inline.
- Each finding includes a permalink for context.

### Comment template

The comment **must start** with the disclosure line below — verbatim, as the first paragraph,
on its own line. Do not change its wording. Do not append any branding footer, "Generated with
Claude Code" line, or ensemble-stats `<sub>` tag at the end.

```markdown
I used an AI agent with a custom prompt to generate this review.

### Code review

Found <K> issue(s) across 5 independent reviews (threshold ≥ 60):

1. <title> &nbsp;`[agreement: <N>/5, confidence: <score>]`

   <description, including category and any suggested-fix alternatives>

   <permalink>

2. ...
```

### Posting

Use `gh` via the Bash tool with a HEREDOC body to preserve formatting:

```sh
gh pr comment <PR> --body "$(cat <<'EOF'
<rendered template>
EOF
)"
```

**This is the ONLY GitHub write this skill is permitted to perform**, and only when there is at
least one finding ≥ 60. Do not edit the PR, add reviewers, change labels, or alter any other PR
state.

## Step 4: Final report (to the user, not GitHub)

Summarise to the user in chat — this report happens whether or not a comment was posted.

**If at least one finding was posted:**
- The PR URL.
- How many findings each sub-agent originally returned (pre-merge counts).
- How many unique findings survived the ≥ 60 filter (post-merge count).
- Sub-agents that returned `skipped_reason` (if any) and why.
- The exact PR comment URL that `gh pr comment` returned.

**If no comment was posted (clean PR):**
- Lead with a clear "all clear" line, e.g. ✅ `PR #<PR> looks good — five independent reviewers
  raised no findings at confidence ≥ 60. Nothing posted to GitHub.`
- The PR URL.
- How many findings each sub-agent originally returned (pre-merge counts).
- How many were filtered out by the ≥ 60 threshold (so the user can see whether reviewers
  flagged anything below the bar).
- Sub-agents that returned `skipped_reason` (if any) and why.

## Constraints

- **At most one GitHub write per run.** The orchestrator posts a single `gh pr comment` only
  when there is at least one finding ≥ 60. When the merged-filtered list is empty, the
  orchestrator posts NOTHING. Either way, no `gh pr review`, no `gh pr edit`, no extra
  comments.
- **Sub-agents are read-only with respect to GitHub.** They invoke `code-review` but intercept
  before its step 6 / 7 / 8 to prevent the upstream skill from posting. If any sub-agent appears
  about to issue a GitHub write, abort the entire orchestration and surface to the user — do not
  proceed to post the merged comment, since the upstream skill may have already posted from the
  sub-agent.
- **Three parallel sub-agents.** Launch in a single message with concurrent Agent tool calls.
  Do not fall back to fewer sub-agents.
- **Threshold is 60.** Do not raise or lower it on the fly.
- **One PR per run.** This skill targets a single PR; do not iterate across multiple PRs in one
  invocation.
- **No fix application.** This skill posts feedback only. For the iterate-and-fix flow, use
  the `review-and-fix` skill instead.
