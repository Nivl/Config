---
name: gh-style-review
description: >
  Locally mirrors the prompt sent by `@claude review` (the
  `anthropics/claude-code-action` GitHub Action). Supports two modes:
  PR mode (arg is a PR number / URL or auto-detected from the current branch) — pre-fetches
  the same rich PR context the Action injects (formatted_context, PR body, conversation
  comments, inline review-thread comments, prior submitted reviews, changed files, the diff)
  and assembles them into the same XML-tagged structure used in `src/create-prompt/index.ts`;
  branch mode (arg is a git revision range like `origin/main..HEAD`) — degrades gracefully
  by dropping the comment-derived sections and reviewing just the diff + commit log + changed
  files. Applies the same review-mode instructions in both modes, then either prints the
  review to the terminal (when invoked directly) or returns a structured JSON shape (when
  invoked as a sub-agent by `pr-review` or `review-and-fix`). Output stays local: never
  posts to GitHub, never opens a PR comment, never updates a Claude comment via MCP.
  Used as a parallel reviewer primitive alongside `in-depth-review` by `pr-review` (1
  instance) and `review-and-fix` (1 instance per iteration). Use this skill when the user
  asks for "gh-style review", "@claude review locally", "review like the GitHub Action
  would", "local mirror of @claude review", or wants the same prompt as the Action without
  the GitHub round-trip.
---

# GitHub-Style Local PR Review

A local fidelity-replica of `@claude review`. Builds the same prompt the
[anthropics/claude-code-action](https://github.com/anthropics/claude-code-action/blob/main/src/create-prompt/index.ts)
sends to Claude when a comment matches the trigger phrase, then runs that prompt in **this**
session and prints the review.

## Why this exists vs. the built-in `/review`

| | built-in `/review` | this skill (`gh-style-review`) | `@claude review` on GitHub |
|---|---|---|---|
| Prompt body | ~20 lines, generic checklist | Same as right column (locally assembled) | ~150 lines, structured XML context |
| PR conversation comments | ❌ | ✅ | ✅ |
| Inline review-thread comments | ❌ | ✅ | ✅ |
| Prior submitted reviews | ❌ | ✅ | ✅ |
| Changed-files list with SHAs | ❌ | ✅ | ✅ |
| Image attachments | ❌ | ❌ (deliberately dropped) | ✅ |
| CLAUDE.md compliance directive | implicit | ✅ explicit | ✅ explicit |
| Output destination | terminal | terminal | GitHub PR comment (via MCP) |

## Argument

Single positional arg, optional. Auto-detects mode:

- **PR mode** — arg looks like `123`, `#123`, or a GitHub PR URL. Diff source = `gh pr diff`.
  Pre-fetches the full PR conversation context (comments, review threads, prior reviews) so
  the review can cross-reference what humans have already raised. **Draft PRs are accepted**
  — reviewing a draft is a valid workflow.
- **Branch mode** — arg is a git revision range (e.g. `origin/main..HEAD`, `HEAD~5..HEAD`).
  Diff source = `git diff <RANGE>`. No PR required. The comment-derived XML sections
  (`<comments>`, `<review_comments>`, `<prior_reviews>`) and the `## Discussion Context`
  output section are **omitted** in this mode — there's nothing to fetch.

If no arg is supplied:
1. First try PR mode by detecting an open PR for the current branch via
   `gh pr view --json number,state,isDraft,url`. If one exists (any state OPEN), use it.
2. Otherwise fall back to branch mode with range `origin/<default-branch>..HEAD`
   (default branch detected via `git remote show origin | grep 'HEAD branch' | awk '{print $NF}'`,
   fallback `main`).

### Modifier flags

- `--raw` — skip the default `< 70` confidence filter. Print every scored finding (0–100)
  with its confidence tag. Useful when you want to see the moderate-confidence stuff the
  filter would normally hide, or when sanity-checking why a review came back "Clean".
  Callers that apply their own threshold (`pr-review` uses 60, `review-and-fix` uses 50)
  pass this flag.

Example invocations:

```
/gh-style-review                       # auto-detect: PR if branch has one, else branch mode
/gh-style-review 1234                  # PR mode, default filter (>= 70)
/gh-style-review #1234 --raw           # PR mode, no filter (every scored finding shown)
/gh-style-review origin/main..HEAD     # branch mode, default filter
/gh-style-review HEAD~5..HEAD --raw    # branch mode, no filter
```

## GitHub access (GitHub MCP with `gh` fallback)

Every GitHub call below is written as a `gh` command for reference. **Prefer the GitHub MCP
server when it is connected; use the `gh` command only as a fallback when no GitHub MCP is
available (or its tools don't cover the call).** Discover the MCP
tools with `ToolSearch "github pull request"` and call the operation matching the `gh` call:

| `gh` call used here | GitHub MCP equivalent (confirm exact name via ToolSearch) |
|---|---|
| `gh auth status` | n/a — instead confirm a GitHub PR tool is exposed and authenticated (not just an `authenticate` stub) |
| `gh pr view <N> --json …` | get pull request (metadata) |
| `gh pr diff <N>` | get pull request diff |
| `gh pr view <N> --json files` | get pull request files (changed files) |
| `gh api …/issues/<N>/comments` | get issue comments (PR conversation) |
| `gh api …/pulls/<N>/comments` | get pull request review comments (inline threads) |
| `gh api …/pulls/<N>/reviews` | get pull request reviews (prior reviews) |
| `gh api …/contents/<path>?ref=<SHA>` | get file contents at a ref |

Prefer the GitHub MCP when connected; fall back to `gh` only when no MCP is available. Both
paths are **read-only** here — read calls map to read tools, so the "never writes to GitHub"
constraint is unchanged. If NEITHER a GitHub MCP nor `gh` is available, surface that and stop:
PR mode cannot proceed without PR context. Local `git` calls (`git show`, `git fetch`,
`git diff`, `git log`) need no `gh` and are unaffected.

## Step 0: Setup

1. Confirm GitHub access: prefer a GitHub MCP PR tool (per the **GitHub access** section above);
   if no MCP is connected, fall back to `gh auth status`. If neither is available, surface the
   failure and stop.
2. Resolve the repo root with `git rev-parse --show-toplevel` (everything below assumes the
   shell CWD is somewhere inside it).
3. Determine `<MODE>` (`pr` or `branch`) per the Argument rules above. Resolve mode-specific
   variables:

   **PR mode:**
   ```
   gh pr view <PR_NUM> --json number,baseRefName,headRefName,headRefOid,url,author,title,body,state,isDraft,createdAt
   ```
   Save `<OWNER>`, `<REPO>`, `<PR_NUM>`, `<BASE_REF>`, `<HEAD_REF>`, `<HEAD_SHA>` (= `headRefOid`), `<IS_DRAFT>`.

   Make the reviewed code readable locally **without disturbing the working tree** — the PR
   head may not be the branch you're on, and parallel sub-agents share one checkout, so do
   NOT `gh pr checkout`. Best-effort fetch the head commit (idempotent; ignore a failure from
   a concurrent run):
   ```
   git fetch origin pull/<PR_NUM>/head
   ```
   Files at the reviewed revision are then read with `git show <HEAD_SHA>:<path>`. If the
   object is missing, fall back to
   `gh api repos/<OWNER>/<REPO>/contents/<path>?ref=<HEAD_SHA> -H "Accept: application/vnd.github.raw"`.

   - If `state != "OPEN"`, **abort early** and (if invoked as a sub-agent) return a
     `skipped_reason` of `"PR <N> is <state>; gh-style-review requires an open PR."`.
   - If exit non-zero (no PR found despite an arg pointing at one), abort with
     `skipped_reason` `"could not resolve PR <arg>"`.

   **Branch mode:**
   ```
   git rev-list --count <RANGE>
   ```
   If the count is 0, abort early with `skipped_reason` `"empty range <RANGE>"`.
   Save `<RANGE>` and `<BASE_REF>` = the left side of the range (`origin/main` in
   `origin/main..HEAD`).

4. Read the repo's `CLAUDE.md` (if any) so the review honors project-specific guidance —
   the GitHub Action enforces this and so should we.

## Step 1: Pre-fetch context

Fetch the sections relevant to the active `<MODE>` in parallel (single Bash message with
multiple calls), then format into the matching XML section. **Drop any section whose fetch
returns empty** — don't inject `<comments></comments>` when there are no comments; just
omit the tag. When a GitHub MCP is connected (the preferred path), each `gh`/`gh api` call in the tables below uses its
GitHub MCP equivalent (see **GitHub access**); the MCP returns JSON, so format from that
instead of the `gh` output.

### PR mode fetches

| XML section | gh call | Notes |
|---|---|---|
| `<formatted_context>` | from the `gh pr view` JSON in Step 0 | title, author, base→head, state, createdAt |
| `<pr_or_issue_body>` | `.body` from the same JSON | render markdown verbatim |
| `<comments>` | `gh api repos/<O>/<R>/issues/<N>/comments` | issue-thread comments on the PR |
| `<review_comments>` | `gh api repos/<O>/<R>/pulls/<N>/comments` | inline diff-thread comments |
| `<prior_reviews>` | `gh api repos/<O>/<R>/pulls/<N>/reviews` | already-submitted formal reviews |
| `<changed_files>` | `gh pr view <N> --json files` | path + sha + +/- counts |
| `<diff>` | `gh pr diff <N>` | the full diff |
| `<metadata>` | constructed locally | repo, pr_number, base_ref, head_ref |

### Branch mode fetches

| XML section | command | Notes |
|---|---|---|
| `<formatted_context>` | constructed locally | range, base_ref, current_branch, commit_count |
| `<commit_log>` | `git log --pretty=format:'%h %s%n%b' <RANGE>` | replaces `<pr_or_issue_body>`; one entry per commit |
| `<changed_files>` | `git diff --name-status <RANGE>` | status + path; no SHAs (range-relative) |
| `<diff>` | `git diff <RANGE>` | the full diff |
| `<metadata>` | constructed locally | range, base_ref, head_ref |

**Branch mode deliberately omits** `<comments>`, `<review_comments>`, and `<prior_reviews>`
— there's no PR to fetch them from. The reviewer is told (in Step 2) that these tags are
absent so it won't search for a Discussion Context to surface.

### Common formatting

Each comment-list section should be a flat sequence like:

```xml
<comments>
  <comment author="…" created_at="…" url="…">
  …body…
  </comment>
  …
</comments>
```

`<review_comments>` additionally carries `path`, `line` (or `start_line`+`line` for ranged
comments), and `diff_hunk`. `<prior_reviews>` carries `state` (APPROVED / CHANGES_REQUESTED
/ COMMENTED) and the review body.

**Skip image downloading.** The Action saves attached images to disk and inlines paths;
locally that's expensive plumbing for limited gain. If the user explicitly needs vision,
they can re-run with `--with-images` (not implemented in v1; punt to a future iteration).

## Step 2: Assemble the prompt

Concatenate in this exact order — the order mirrors `create-prompt/index.ts` and matters
because the trailing instructions reference earlier tags by name. **Sections marked
"PR-only" are omitted in branch mode.**

```
You are Claude, performing a local code review that mirrors the prompt
the @claude review GitHub Action uses. Output goes to a terminal (or to an
orchestrator sub-agent, depending on the caller), not to a PR comment.

<formatted_context>
…
</formatted_context>

<pr_or_issue_body>                      <!-- PR-only; branch mode uses <commit_log> instead -->
…
</pr_or_issue_body>

<commit_log>                            <!-- branch-only -->
…
</commit_log>

<comments>                              <!-- PR-only -->
…
</comments>

<review_comments>                       <!-- PR-only -->
…
</review_comments>

<prior_reviews>                         <!-- PR-only -->
…
</prior_reviews>

<changed_files>
…
</changed_files>

<diff>
…
</diff>

<metadata>
mode: <pr | branch>
repository: <OWNER>/<REPO>                  # PR mode
pr_number: <PR_NUM>                         # PR mode
base_ref: origin/<BASE_REF>
head_ref: <HEAD_REF>                        # PR mode (PR head branch)
head_sha: <HEAD_SHA>                        # PR mode (reviewed revision; read files via `git show <HEAD_SHA>:<path>`)
range: <RANGE>                              # branch mode
</metadata>

Review instructions — mirror how the @claude review Action works: you are reviewing a
LIVE repository with full read access, NOT a static diff. The pre-fetched context above is
your STARTING POINT, not the boundary of the review. This is a REVIEW, not an implementation
task: do NOT edit files, commit, push, or post to GitHub. Read and report only.

1. Gather context.
   - The diff against `origin/<BASE_REF>` (NOT main/master) shows WHAT changed.
   - Use the Read tool to look at the relevant files for better context — read the FULL
     changed files, not just the diff hunks, so each change is seen in its real surroundings.
   - (PR mode) The reviewed code is at the PR head `<HEAD_SHA>`, which may differ from your
     local working tree. Read the authoritative version with `git show <HEAD_SHA>:<path>`,
     not whatever branch happens to be checked out.
   - Read the repo's CLAUDE.md and honor it.

2. Verify the PR delivers its stated benefit — reason from MOTIVATION, not just the diff.
   The diff shows WHAT changed; the <pr_or_issue_body> / <commit_log> / linked ticket say
   WHY. A diff can be locally correct yet still NOT deliver its headline benefit — e.g. it
   fixes a helper that has no live callers while the real behavior runs through a different,
   untouched path that still carries the bug.
   - Restate the PR's stated goal in one sentence. If it names an observable effect (a
     metric/tag value, a query result, an email, an event, an endpoint response), note it.
   - Find where that effect must actually manifest at runtime — the live call site, query,
     metric emit, or handler the goal names. `git grep` for it. It is frequently NOT in the
     diff, and that is exactly what diff-anchored review misses.
   - Confirm the diff makes the benefit land THERE. If the changed code has no live callers
     (grep proves zero), do NOT stop at "forward-looking, no change needed" — that is the
     near-miss trap. Pull the thread: where does the live behavior run today, and does it
     still carry the bug the PR set out to fix? Flag that site (REQUEST CHANGES) even though
     it is outside the diff — delivering the stated benefit there is in scope.

3. Investigate impact — this is where diff-only review fails.
   - For each change, trace it into the code it touches: the functions it calls, the callers
     that reach it, and any previously-dormant, conditional, or dead code paths the change
     newly activates or makes reachable.
   - Read those surrounding and downstream files (Read/Grep/Glob) EVEN WHEN THEY ARE NOT IN
     THE DIFF. A correct diff can still surface or activate a latent bug elsewhere — that
     defect is in scope, and is exactly the kind diff-anchored review misses.
   - Follow the data flow to its real endpoints (DB writes, emails, partner/external calls,
     state transitions) and confirm the change's effect two hops out, not just locally.

4. Reachability proof for any branch the change depends on — the check that catches dead-guard
   / latent-typo bugs, done ADVERSARIALLY: default to the branch being BROKEN until you can
   prove it fires, with quoted evidence. When the change relies on a branch firing (to send an
   email, emit an event, persist, or clean up) and that branch is guarded by `X.field ===
   'LITERAL'`:
   - `git grep` the FIELD to list (i) every literal COMPARED against it and (ii) every site that
     ASSIGNS or persists it — and QUOTE the assignment line(s).
   - If the field is written verbatim from a source object (e.g. `model.status =
     apiObject.status`), it holds THAT source's values — trace what the source actually
     produces. Do NOT assume the field carries a separate "internal" value just because a
     guard's literal looks plausible. A literal sitting next to a near-duplicate of itself
     (`'PastDue'` vs a persisted `'Past Due'`; `'in_trial'` vs `'inTrial'`) is almost always a
     typo, not a deliberate distinction — prove which from the assignment, don't rationalize.
   - A guard whose literal never appears among the values actually written is a DEAD branch: the
     dependent email / event / cleanup silently never happens. Flag it (REQUEST CHANGES) with
     the exact line and the one-line fix. Reject any "it's intentional / it's a different field"
     explanation you cannot back with a quoted write line.

5. Review thoroughly. Look for: correctness bugs, security issues, performance problems,
   missing edge cases, project-convention violations (CLAUDE.md), error handling, and test
   coverage — including coverage gaps for unchanged code the change newly exercises.
   - (PR mode only) Cross-reference <comments>, <review_comments>, and <prior_reviews>:
     confirm a prior human concern is resolved by the diff, or flag it unresolved; don't
     duplicate a point a human already made. In branch mode these tags are absent — skip the
     Discussion Context output section.
   - On comments the diff ADDS or edits, flag AGENTS.md comment-punctuation violations
     (severity `suggestion`): a comment that joins two independent clauses with ` - `
     (space-hyphen-space, standing in for an em-dash) or a `:` that splits a claim from its
     elaboration. The fix is two sentences. Flag ONLY the clause-joiner use — never hyphenated
     words (`read-only`), flags (`-c`), ranges (`1-10`), label prefixes (`TODO:`, `NOTE:`),
     ratios/times (`3:1`), or code/paths/URLs.
   - Reference specific code with file paths and line numbers.

<!-- OUTPUT FORMAT — see "Output Format" section of the skill -->
```

The "Output Format" section below replaces that placeholder.

---

## Output Format

Each finding is graded on **two orthogonal axes** — same scheme as `in-depth-review`:

1. **Severity** — `critical | major | minor | suggestion` (qualitative category)
2. **Confidence** — `0..100` (how sure you are the finding is real, with this rubric):

| Score | Meaning |
| ----- | ------- |
| 0     | False positive that doesn't survive light scrutiny, or a pre-existing issue unrelated to the diff |
| 25    | Somewhat confident — might be real, might not; couldn't verify either way |
| 50    | Moderately confident — the mechanism is real, but residual uncertainty remains about whether it truly applies here |
| 75    | Highly confident — verified the code definitively does this; OR a provable convention / CLAUDE.md violation |
| 100   | Absolutely certain — the diff directly confirms the problem |

**Calibration — confidence is the TRUTH axis, not current impact.** Confidence answers "how
sure are we this finding is real and valid," NOT "how big is the blast radius today." Two
consequences:
- A finding whose truth is *provable and binary* — a convention or safety violation (e.g. a
  non-`CONCURRENTLY` index build on a pre-existing table, checkable against repo convention) —
  scores by provability ALONE. Do NOT discount it because the current blast radius is small:
  an empty or feature-gated table, low live traffic, or a cheap fix. That low impact belongs
  in the **severity** field (`minor` / `suggestion`), not in the confidence number. Deflating
  a provably-true finding by today's table size is the calibration error to avoid — it buries
  real, cheap-to-fix findings below the caller's threshold.
- This is NOT a blanket "score every real-ish finding high." For a latent *bug*, confidence
  still reflects whether it is genuinely a defect and whether its path is reachable at all — a
  rare, marginal conjunction that may not even constitute a real defect legitimately sits near
  or below the line. Reachability (can the path EVER execute) is a truth question and bounds
  confidence; frequency (how OFTEN, how big the blast radius) is impact and does not.

**Scope note for the motivation-delivery step (step 2).** A finding that the PR's stated
benefit fails to land at its live call site is NOT disqualified as a "pre-existing issue
unrelated to the diff" — the PR's stated purpose puts that site in scope. Score it on whether
the benefit is genuinely undelivered, not on whether the line sits inside the diff.

**Default filter:** discard findings with `confidence < 70`. (Matches `in-depth-review`'s
default. Lower threshold than upstream `code-review`'s 80 because the richer pre-fetched
context — prior reviews, conversation comments — tends to surface verified-but-modest
findings that score in the 70–79 band.)

**When invoked with `--raw`:** skip the filter entirely. Print every scored finding,
regardless of confidence. The `[confidence X]` tag stays on each finding so the reader
can still see the score. Section headers and ordering rules are unchanged.

Print to terminal as GitHub-flavored markdown, in **this exact structure**:

```
## Verdict
<APPROVE | REQUEST CHANGES | COMMENT> — <one-sentence justification, <=25 words>.

## Summary
2–4 sentences describing what the PR actually does. Plain prose, no bullets.
Do NOT restate the PR title or body verbatim; this is your interpretation of the diff.

## Findings

### Critical
- `path/to/file.ext:LINE` — <one-line problem statement> `[confidence 85]`
  <2–4 sentence explanation: why it's wrong, what it'll do at runtime>
  **Fix:** <concrete suggested change — code block if non-trivial, sentence otherwise>

### Major
Same per-finding shape as Critical. Design issues, missing edge cases,
project-convention violations (per CLAUDE.md), test gaps for risky changes.

### Minor
Same per-finding shape. Local issues that don't block merge but should be addressed.

### Suggestion
Optional improvements. One-liner per finding — no multi-paragraph explanations.
Format: `- \`file:line\` — <suggestion> \`[confidence X]\``

Within each section, order findings by **confidence descending**.
Omit any severity section that has zero surviving findings (the active filter is `<70`
by default, none under `--raw`). Do not print empty headers.

## Discussion Context           <!-- PR mode only; omit entire section in branch mode -->
Use the pre-fetched <comments>, <review_comments>, and <prior_reviews>.

- **Resolved by this PR** — concerns raised by humans that the diff addresses.
  Format: `> <quote or paraphrase> — @<author>` followed by `✅ <how the diff resolves it>`.
- **Still unaddressed** — concerns raised by humans that the diff does NOT address.
  Format: `> <quote or paraphrase> — @<author>` followed by `⚠️ <what's still missing>`.

Omit either subsection if it's empty. Omit the whole `## Discussion Context` heading
if both are empty (e.g. brand-new PR with no comments yet, or branch mode).

## Footer
End with one line. The wording depends on whether `--raw` was passed:
- Default mode: `Findings: N total, M after the confidence >= 70 filter.`
- `--raw` mode:  `Findings: N total (no filter applied; --raw).`

## Empty-case override
Trigger condition depends on mode:
- Default mode: zero findings AFTER the `<70` filter AND no unresolved discussion items.
- `--raw` mode: zero findings AT ALL (no filter to fall back on) AND no unresolved
  discussion items.

When the trigger condition holds, replace the entire output above with exactly:

    ✅ Clean — nothing to flag <at confidence >= 70 | in raw mode>.
    <one sentence on what the PR does, for confirmation>

(Pick the bracketed phrase that matches the mode.) …and stop. Don't print empty
headers, don't print the verdict, don't pad.
```

**Notes on style:**
- Keep `file:line` citations tight — single backticks, no full URLs.
- For ranged findings (e.g. a whole function), use `file:LINE_START-LINE_END`.
- Code blocks in fixes: use the file's actual language, not pseudo-code.
- Never invent line numbers — if you can't pin a finding to specific lines, classify it
  one severity bucket lower, drop the `:LINE` suffix, and explain in prose where it lives.
- The `[confidence X]` tag is mandatory on every finding (including Suggestions).
  Don't omit it because you're "pretty sure" — pick a rubric number.
- Severity headings are plain words (`### Critical`, etc.) — no emoji, no bold, no colons.

## If invoked as a sub-agent (by `pr-review` or `review-and-fix`)

When the caller's prompt asks for structured JSON output (typically alongside `--raw`),
**skip the terminal-formatted output above** and return this exact JSON shape:

```json
{
  "scope": "<SCOPE_DESCRIPTION>",
  "mode": "pr" | "branch",
  "pr_number": <int or null>,
  "pr_head_sha": "<sha or null>",
  "range": "<range or null>",
  "raw": <true if --raw was set, else false>,
  "summary": "<short summary of what changed (mirrors the 'Summary' section above)>",
  "verdict": "APPROVE | REQUEST CHANGES | COMMENT",
  "findings": [
    {
      "id": "<file:line-range>",
      "title": "<one-line description>",
      "file": "<path>",
      "line_range": "<L<start>-L<end> or single L<N>; null if not pinnable>",
      "severity": "critical | major | minor | suggestion",
      "category": "<bug | CLAUDE.md | history | prior PR | discussion | security | error-handling | test coverage | style>",
      "description": "<full text>",
      "suggested_fix": "<text or code snippet>",
      "confidence": <0..100>,
      "permalink": "<github blob URL with full SHA, if PR mode and available; null otherwise>"
    }
  ],
  "discussion_context": {
    "resolved": [
      {
        "quote": "<verbatim or paraphrase of the human comment>",
        "author": "<github_login>",
        "url": "<github comment URL>",
        "resolution": "<how the diff resolves it>"
      }
    ],
    "unaddressed": [
      {
        "quote": "<verbatim or paraphrase>",
        "author": "<github_login>",
        "url": "<github comment URL>",
        "gap": "<what's still missing>"
      }
    ]
  },
  "skipped_reason": "<if the skill bailed out early per Step 0, why; otherwise omit>"
}
```

Notes on the JSON contract:

- **Order of `findings`:** same as terminal mode — severity desc (critical → major →
  minor → suggestion), then confidence desc within each severity.
- **`--raw` semantics:** when `raw=true`, include every scored finding (no filter applied).
  When `raw=false`, only findings with `confidence >= 70` appear in the array. The caller
  (`pr-review` at >=60, `review-and-fix` at >=50) will apply its own threshold post-merge.
- **`discussion_context` in branch mode:** both arrays are always empty `[]` (no PR comments
  to fetch). Don't omit the key — orchestrators expect a stable shape.
- **`verdict` in branch mode:** still computed (`APPROVE` / `REQUEST CHANGES` / `COMMENT`),
  treating the commit range as if it were a hypothetical PR. Orchestrators may ignore it
  and roll their own per-iteration verdict.
- **No terminal output** when returning JSON. Don't print the markdown report and the JSON;
  the caller will be confused.
- **`skipped_reason`** is present iff Step 0 aborted (closed PR, empty range, etc.). When
  present, `findings` and `discussion_context` may be omitted entirely.

## Constraints

- **No GitHub writes.** `gh pr comment`, `gh pr review`, `gh pr edit`, `gh pr merge`,
  `gh pr close`, `gh issue create`, `gh issue comment` are all forbidden. Only read-only
  `gh` calls (the ones in Step 1) are permitted. The GitHub MCP path is read-only too —
  use only its PR-read tools; never a review-create / comment / merge / edit tool.
- **Compare against `origin/<BASE_REF>`**, never `main`/`master` directly — the PR may be
  targeting a non-default branch (release branch, stacked PR, etc.).
- **Always follow the target repo's `CLAUDE.md`** if present.
- **Print to the terminal only.** Do not write the review to a file unless the user asks.
- **Single review pass.** This skill is not iterative — for the iterate-and-fix loop see
  `review-and-fix`; for cross-instance triangulation see `pr-review` or `in-depth-review`.
- **Model policy (cost):** this skill spawns no sub-agents — it runs in one agent, so its
  tier is set by whoever invokes it. When a caller (`pr-review`, `review-and-fix`) spawns it
  as a sub-agent it MUST run on **Sonnet** (`model: sonnet`): a single bounded recall pass,
  not worth Opus or a `[1m]` variant. Run directly by a user, it just uses the session model.
- **Sub-agent mode contract.** When invoked as a sub-agent and asked for JSON, return the
  exact shape in "If invoked as a sub-agent" — including `discussion_context` (with empty
  arrays in branch mode). Don't drop fields the orchestrator depends on.
