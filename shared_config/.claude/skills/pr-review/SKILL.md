---
name: pr-review
description: >
  Reviews a pull request with TEN parallel reviewer sub-agents — FIVE `in-depth-review` + FIVE
  `gh-style-review`, all invoked `--raw`. The orchestrator merges and deduplicates the ten
  result sets into one pool, keeps findings scoring >= 60, and classifies each as INLINE
  (specific diff lines) or GLOBAL (broad/architectural). The `gh-style-review` instances also
  return Discussion Context — prior human comments the diff still leaves open — surfaced as a
  "Still unaddressed" section. Everything posts as a SINGLE PR review: global findings in full,
  a names-only list of local findings (also left as inline diff comments), and the unaddressed
  concerns. Empty sections are never emitted; if nothing survives, NOTHING is posted and the
  clean result is reported only in chat. A single adversarial reviewer then debates a
  more-nuanced agent; findings they agree need a change — and that no other reviewer already
  raised with confidence > 50 — post too, tagged `adversarial`. Use this skill when the user
  asks to "review this PR", "pr review", "triple code review", "ensemble code review",
  "consensus review", a "thorough review of this PR", or wants higher-confidence PR feedback
  than a single `/in-depth-review` run would produce.
---

# PR Review (10x-reviewed)

This skill orchestrates **ten** parallel reviewer sub-agents against a single PR: **five
instances of `in-depth-review`** (each runs ten roles by default, or nine with `--skip-ticket`; all raw scored findings) and
**five instances of `gh-style-review`** (the `@claude review` GitHub Action prompt
replicated locally, which adds Discussion Context — prior-human-comment cross-referencing
— on top of standard findings). All ten are invoked with `--raw`; the orchestrator merges and
deduplicates the ten result sets into one flat pool, applies a final **score ≥ 60** filter,
classifies each surviving finding as INLINE (local) or GLOBAL, aggregates the still-unaddressed
`discussion_context` across the five gh-style instances, and posts a single PR review whose body
carries the global findings in full, a names-only list of the local findings, and the
still-unaddressed prior concerns. Inline comments are attached to the diff lines they refer to.
Sections with no content are never emitted.

On top of the ten reviewers, a single **adversarial reviewer** debates a single
**more-nuanced counterpart agent** over the issues it raises (Step 2.7). Only the findings the
pair *converges* on as genuinely needing a code change survive, and only if no other reviewer
already proposed the same thing with confidence > 50. Survivors join the same posting pipeline,
tagged `adversarial`. This pair runs once (not 5×), and its findings post on agreement alone —
they do not have to clear the ≥ 60 confidence bar the other findings do.

The point: ten independent passes from two different prompt structures (specialized-role
vs. GitHub-Action mirror) catch different issues, AND converge on the real ones. One
review entry, ten reviewers' worth of recall, plus an explicit "what humans already raised
that the diff still hasn't addressed" section.

## Prerequisites

- The current branch (or the PR specified as the skill argument) must have an **open PR**.
  Draft PRs are accepted — reviewing a draft is a valid workflow (you get feedback before
  marking the PR ready). Both `in-depth-review` and `gh-style-review` accept drafts.
- Both the `in-depth-review` and `gh-style-review` skills must be installed and available
  (they live alongside this skill in `shared_config/.claude/skills/`).
- A GitHub MCP server must be connected and authenticated — OR `gh` must be installed and
  authenticated, which the skill falls back to when no GitHub MCP is available (see **GitHub access**).

**Flag:** pass `--skip-ticket` to disable ticket intent compliance (Role #10) across all
five `in-depth-review` instances and skip the Jira-tooling preflight.

## GitHub access (GitHub MCP with `gh` fallback)

Every GitHub call below is written as a `gh` command for reference. **Prefer the GitHub MCP
server when it is connected; use the `gh` command only as a fallback when no GitHub MCP is
available (or its tools don't cover the call).** Discover the MCP
tools with `ToolSearch "github pull request"` and call the operation matching the `gh` call:

| `gh` call used here | GitHub MCP equivalent (confirm exact name via ToolSearch) |
|---|---|
| `gh pr view <N> --json …` | get pull request (metadata, headRefOid) |
| `gh pr diff <N> --name-only` | get pull request files (changed files) |
| `gh repo view --json owner,name` | get repository (owner/name) |
| `gh api -X POST …/pulls/<N>/reviews` | **create-and-submit pull request review** (or the pending-review + add-review-comment + submit trio for inline comments) — the one write |

Prefer the GitHub MCP when connected; fall back to `gh` only when no MCP is available. If NEITHER
a GitHub MCP nor `gh` is available, abort in Step 0 and tell the user — this skill cannot
resolve or post to the PR without one. The reviewer sub-agents (`in-depth-review`,
`gh-style-review`) carry their own identical fallback for the reads they do. Local `git` calls
need no `gh`.

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
     unauthenticated. In a sandboxed session, skip the acli probe and treat acli as
     unavailable — sandboxed acli fails even when installed and authenticated — and rely
     on the MCP check below; OR
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
     used as a tiebreaker in step 6.
   - `title`, `description`, `suggested_fix`: pick the clearest from the group; if suggested
     fixes differ meaningfully, mention the alternatives in the description.
   - `category`: union of categories.
   - `permalink`: take any one valid permalink from the group.
   - `ticket_id`: preserved from `ticket`-category findings (the Jira ID the gap traces to);
     `null` for all other findings. Never merge two findings that name different `ticket_id`s.

4. **Snapshot the full merged set as `merged_all` BEFORE filtering.** `merged_all` is the
   complete list of merged findings with their max `confidence`, including the 51–59 ones that
   the next step discards. Step 2.7 (adversarial dedup) needs it — once you apply the ≥60 cut
   you can no longer tell whether a 51–59 finding "was already proposed by another reviewer."
   `merged_all` is used only by Step 2.7; it never affects posting on its own.

5. **Filter:** keep only findings with `confidence >= 60`. Discard the rest — with one
   exception: retain any discarded `ticket`-category finding (confidence < 60) in a separate
   `sub_threshold_ticket_notes` list. Step 3a.5 uses it for the "Tickets examined" section.
   These never enter the findings pool, the INLINE/GLOBAL classification, or the ordering
   below — they are notes, not findings. (Threshold is 60.)

6. **Order the surviving findings:**
   1. `confidence` descending
   2. `agreement` descending (10/10 > 5/10 > 1/10 when scores tie)
   3. Both-sources first (a finding raised by both in-depth-review and gh-style-review beats
      a same-confidence-and-agreement finding from a single source)
   4. `category` priority: bug > AGENTS.md > history > prior PR > comment guidance > ticket

## Step 2.5: Aggregate Discussion Context (from gh-style sub-agents only)

`gh-style-review` sub-agents each return a `discussion_context` block with `resolved` and
`unaddressed` arrays. `in-depth-review` returns no such block — skip those instances here.
Only the **unaddressed** concerns are rendered (the "Addressed by this PR" section was dropped
as noise — listing what the diff already fixed is not actionable). The `resolved` entries are
read solely to detect reviewer disagreement in step 2.

1. **Pool every `unaddressed` entry** across the five `gh-style-review` result sets into one
   flat `unaddressed_pool`. Each entry carries `quote`, `author`, `url`, and `gap`.

2. **Deduplicate by `url`** (the GitHub comment URL is the canonical identity of a discussion
   item). If the same URL appears in some instances' `resolved` and others' `unaddressed` —
   reviewers disagree on whether the diff fixes it — keep it in `unaddressed_pool` and note
   the disagreement count.

3. **For each deduplicated entry** pick the clearest `quote` and `gap` text across instances
   (longest non-trivial wording usually wins). Record `agreement` = instance count that raised
   this exact concern as unaddressed.

4. **Retain all entries** — there's no confidence threshold here because every entry is
   grounded in a real human comment URL. Order `unaddressed_pool` by `agreement` desc, then by
   the comment's `created_at` ascending (oldest unresolved concern first).

5. **If `unaddressed_pool` is empty after dedup, the "Still unaddressed" section is skipped.**
   If it is empty AND the findings list from Step 2 is also empty, Step 3 still applies — no
   review is posted.

## Step 2.7: Adversarial review stage (one pair, run once)

A single **adversarial reviewer** and a single **more-nuanced counterpart agent** debate the
issues the adversarial reviewer raises. Only the issues the two *converge* on as genuinely
needing a code change survive, and only if no other reviewer already raised the same thing with
confidence > 50. Survivors are added to the findings pool (Step 2 output) so Step 3 posts them
exactly like every other finding — distinguished only by an `adversarial` tag.

This stage runs **after** the merge because the dedup in Step 2.7b below compares against
`merged_all`, which only exists once Step 2 has merged the ten reviewers. The pair runs **once**
(not 5×) — the internal debate is the filter, so cross-instance triangulation is not needed.

**Both agents are read-only with respect to GitHub** — same forbidden-write list as sub-agents
1–10. They see only the PR diff and context, **never** the other reviewers' findings, so their
pass stays independent (the dedup happens here, at the orchestrator, not inside the agents).

### Step 2.7a: The debate (orchestrator-mediated, up to 3 rounds)

The orchestrator mediates the exchange, carrying the current state of every contested finding
between agents. **Round 1 is the propose+judge pass (count = 1); each later rebut iteration —
one adversarial re-spawn plus one nuanced re-spawn — is one more round.** Track, per finding,
the latest `adv_verdict` and `nuanced_verdict` (each `yes` = needs a code change, or `no`); a
finding's `rounds` is the 1-based round at which it converged.

1. **Round 1 — propose + judge.** Spawn the adversarial reviewer (prompt below); it returns a
   `findings` list, each with `{id, title, file, line_range, description, suggested_fix,
   confidence (0–100), argument}`. Every proposed finding starts at `adv_verdict = yes`. Then
   spawn the nuanced agent with the diff + that `findings` list; it returns a `needs_change`
   (`yes`/`no`) + `argument` per finding id → set `nuanced_verdict`.
2. **Classify each finding:** *converged-keep* (both `yes`), *converged-drop* (both `no`), or
   *contested* (verdicts differ).
3. **Rounds 2–3 — rebut.** While there is at least one *contested* finding AND fewer than 3
   total rounds have run: re-spawn the adversarial reviewer with, for each contested finding,
   the nuanced agent's latest `argument`; it returns an updated `{needs_change, argument}` per
   id (it may concede to `no` or hold at `yes`) → update `adv_verdict`. Then re-spawn the
   nuanced agent with the adversarial reviewer's latest `argument` per contested finding; it
   returns an updated `{needs_change, argument}` per id (it may concede to `yes` or hold at
   `no`) → update `nuanced_verdict`. Re-classify. Only *contested* findings are carried
   forward; once a finding is *converged-keep* or *converged-drop* its verdicts are frozen and
   it is not re-sent.
4. **Convergence cap = 3 rounds.** A finding **survives only if it converges to BOTH `yes`**.
   Anything converged to `no`, or still *contested* after round 3, is **dropped** — agreement
   is required, ties go to drop.

### Step 2.7b: Dedup survivors against the existing findings

For each surviving finding, **drop it if it duplicates any entry in `merged_all` (Step 2)
whose max `confidence` > 50** — "duplicate" uses the same test as Step 2: same `file` +
overlapping `line_range` + substantially the same problem (paraphrases count). This suppresses
adversarial findings that another reviewer already proposed with confidence > 50, *including*
the 51–59 ones that did not clear the ≥60 posting bar. A surviving finding that matches nothing
in `merged_all` over 50 is kept.

### Step 2.7c: Tag and fold into the findings pool

Each kept adversarial finding becomes a normal finding for Step 3 with:

- `source = "adversarial"`, `agreement = null` (it is not one of the 10 reviewers).
- `confidence` = the adversarial reviewer's own round-1 score — used **only** for
  ordering/display. The rebut rounds do not re-emit `confidence`, so carry the round-1 value
  forward.
- `rounds` = how many debate rounds it took to converge.
- no `permalink` — adversarial findings skip the Step 2 merge that assigns one, so the global
  `**N.**` block omits the permalink paragraph for them.

**Agreement is the gate, not the score.** Adversarial findings are added to the Step 2 findings
pool **without** re-applying the ≥60 filter — they already passed the mutual-agreement bar in
2.7a. Order them within the pool by `confidence` like the others (for the tiebreakers, treat
`agreement = null` as lowest, an adversarial finding as single-source — never both — for the
both-sources tiebreaker, and lowest for the category-priority tiebreaker). They then flow
through Step 3's INLINE/GLOBAL classification and posting unchanged, except for the annotation
in Step 3b.

If no adversarial finding survives 2.7a, or all survivors are dropped in 2.7b, this stage adds
nothing and the rest of the run is unchanged.

### Step 2.7 sub-agent prompts

**Adversarial reviewer (round 1 — propose):**

```
You are the ADVERSARIAL reviewer in a pr-review orchestration. Be maximally skeptical of
PR #<PR>. Assume the change is wrong, risky, or incomplete until proven otherwise. Hunt hard
for real defects: bugs, unhandled edge cases, regressions, race conditions, security and
performance footguns, broken invariants, and missing error handling. Review ONLY the PR diff
and its context — you are NOT given any other reviewer's findings.

For every issue you can defend, return one finding. Return STRICT JSON:
{ "findings": [ {
  "id": "A1", "title": "...", "file": "<path>", "line_range": "<start>..<end>",
  "description": "...", "suggested_fix": "...", "confidence": <0-100>,
  "argument": "<why this genuinely needs a code change>"
} ] }

`confidence` is your own 0–100 estimate (used only for display ordering). Do not invent issues
you cannot defend — a weak finding will be argued down in the debate that follows. Return
`{"findings": []}` if you genuinely find nothing.

Forbidden (you are read-only w.r.t. GitHub): `gh pr comment`, `gh pr review`, `gh pr edit`,
`gh pr close`, `gh pr merge`, `gh issue create`, `gh issue comment`, or any other GitHub write.
```

**Nuanced agent (each round — judge):**

```
You are the NUANCED reviewer in a pr-review orchestration, the pragmatic counterweight to an
adversarial reviewer on PR #<PR>. For each finding the adversarial reviewer raises, decide
whether it GENUINELY warrants a code change in this PR. Weigh intent, severity, real-world
impact, existing safeguards, scope, and whether it is acceptable as-is or out of scope. You are
not here to rubber-stamp and not here to reflexively dismiss — judge each on its merits, given
the diff and context.

You receive the PR diff and a list of findings — the adversarial reviewer's full proposal set in
round 1, or only the still-contested findings in later rounds — each with the adversarial
reviewer's latest `argument`. Return STRICT JSON:
{ "verdicts": [ { "id": "A1", "needs_change": "yes" | "no", "argument": "<your reasoning>" } ] }

Use "yes" only when you agree a code change is warranted. Use "no" when it is not worth a change.
Respond to the adversarial reviewer's specific argument — concede when they are right, push back
when they overreach.

Forbidden (read-only w.r.t. GitHub): `gh pr comment`, `gh pr review`, `gh pr edit`,
`gh pr close`, `gh pr merge`, `gh issue create`, `gh issue comment`, or any other GitHub write.
```

**Adversarial reviewer (rounds 2–3 — rebut):** re-use the adversarial prompt above, but instead
of proposing fresh findings, pass it the contested findings plus the nuanced agent's latest
`argument` for each, and ask it to return
`{ "responses": [ { "id": "...", "needs_change": "yes" | "no", "argument": "..." } ] }` —
holding at `yes` only where it can still defend the issue, conceding to `no` otherwise.

## Step 3: Post the review (only if there is something worth posting)

**If the findings list AND `unaddressed_pool` are BOTH EMPTY, do NOT post anything to GitHub.**
The findings list here is the Step 2 filtered findings PLUS any adversarial survivors folded in
by Step 2.7c — so a single agreed adversarial finding is on its own enough to post a review.
When both pools are empty, skip directly to Step 4 and tell the user in chat that the PR looks
clean. No review, no comment, no PR state change — silence on GitHub is the success signal. A
sub-threshold ticket note on its own is NOT enough to post — if there are no findings and
nothing unaddressed, there is nothing worth saying on GitHub (the note is reported only in the
Step 4 chat summary).

**If at least one of {findings, unaddressed_pool} is non-empty**, post a single PR review
that combines a global body with any inline comments. This is a single atomic write to
GitHub.

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

### Step 3a.5: Identify ticket notes worth surfacing (no "all clear" roll-call)

Build `tickets_examined` = union by `id` of the `tickets_examined` arrays returned by the five
in-depth-review sub-agents (each entry has `id` and `status` ∈ {`ok`, `gaps`, `unread`}). For
each `id`, `status` is `gaps` if any instance reported gaps, else `unread` if any reported
unread, else `ok`.

From that, derive ONLY the notes worth showing a human — never a roll-call of what passed:

- **Above-threshold ticket gaps** are already Code review findings (category `ticket`, prefixed
  `[<ticket_id>]`). Do NOT repeat them in the Tickets examined section.
- **Sub-threshold ticket observations** = the `sub_threshold_ticket_notes` retained in Step 2
  (ticket findings that scored < 60) — an AC a reviewer flagged that did not clear the ≥60 bar.
- **Unread/unverified tickets** = any `id` whose aggregated `status` is `unread` (no instance
  could read it).

Set `ticket_notes_present` = true if there is at least one sub-threshold observation OR at
least one unread ticket. Tickets with `status: ok` and no sub-threshold note contribute
nothing and are never named.

### Step 3b: Build the global body

The global body **must start** with the disclosure line below — verbatim, as the first
paragraph, on its own line. Do not change its wording. Do not append any branding footer,
"Generated with Claude Code" line, or ensemble-stats `<sub>` tag at the end.

**Never emit a section that has no content, and never report what passed.** Every
`###`/`####` block below is conditional on its count — when the count is zero, omit the
header and the block entirely. No "all clear", no "no gaps", no roll-call of green tickets.
The review surfaces only what needs attention.

Let:

- `K_global` = count of GLOBAL findings
- `K_inline` = count of INLINE findings (each is also posted as an inline diff comment in Step 3c)
- `K_unaddressed` = count of entries in `unaddressed_pool`

```markdown
I used an AI agent with a custom prompt to generate this review.

<<if K_global + K_inline > 0:>>

### Code review

Found <K_global + K_inline> issue(s):

<<if K_global > 0:>>

#### <K_global> global issue(s)

**1.** <title> &nbsp;`[agreement: <N>/10, confidence: <score>, sources: <in-depth|gh-style|both>]`

<description, including category and any suggested-fix alternatives>

<permalink>

**2.** ...
<<endif>>

<<if K_inline > 0:>>

#### <K_inline> local issue(s)

- <title>
- <title>
- ...

I left <K_inline> comment(s) directly in the diff for those.
<<endif>>
<<endif>>

<<if ticket_notes_present (Step 3a.5):>>

### Tickets examined

<One short paragraph. Surface ONLY: sub-threshold ticket observations — name the ticket and
the AC, and say it did not clear the ≥60 bar and why — plus any unread/unverified ticket. Do
NOT list tickets that passed. Do NOT repeat above-threshold ticket gaps; those already appear
as Code review findings above, prefixed [<ticket_id>].>
<<endif>>

<<if K_unaddressed > 0:>>

### Still unaddressed in this PR

Concerns raised by reviewers earlier that this PR does not appear to address:

- > <quote> — ([link](url))
  >
  > ⚠️ <gap> &nbsp;`[agreement: <N>/5]`
- ...
  <<endif>>
```

A global body is produced whenever Step 3 reaches here (at least one finding or one
unaddressed concern). The disclosure line is the only unconditional content. The `### Code
review` section — and its `#### global` / `#### local` subsections — "Tickets examined", and
"Still unaddressed" are each emitted only when their count is non-zero. A body carrying just
the disclosure line is impossible: Step 3 gates entry on having something to say.

**Pluralize the headers.** In the `#### ... issue(s)` headers, render the real word: "issue"
when the count is 1, "issues" otherwise — e.g. `#### 1 global issue`, `#### 3 local issues`.
The `Found <N> issue(s):` count is `K_global + K_inline` (the (s) shorthand is fine there).

**Global issues use a bold `**N.**` marker, NOT a real Markdown ordered list.** Each global
finding spans several paragraphs (title, description, permalink), and a multi-paragraph ("loose")
`1.` list item gets its marker re-printed before every paragraph by some renderers — so the
whole block shows up as `1.` on every line. A bold `**1.**` marker with flush-left paragraphs
renders identically on GitHub and elsewhere (a line starting with `**` is never parsed as a list
item, so nothing re-numbers). Number them `**1.**`, `**2.**`, … by hand. Do not convert this
back to a `1.`/`2.` ordered list.

**Local issues are names only.** Under `#### local issue(s)`, list each INLINE finding's
`<title>` as a bullet — no description, no permalink (single-line bullets render cleanly, so an
ordinary `-` list is fine here). The full text rides on the inline diff comment (Step 3c); the
trailing "I left <K_inline> comment(s) directly in the diff for those." line points the reader
at them.

**Section order is fixed:** code review (global then local) → tickets examined → still
unaddressed.

**Ticket findings:** when a finding has a non-null `ticket_id`, prepend `[<ticket_id>] ` to
its `<title>` in both the global body (global list and local list) and any inline comment, so
the ticket is visible.

**Adversarial findings (`source = "adversarial"`):** these have no `agreement: N/10` (they did
not come from the 10 reviewers) and carry the `adversarial` tag instead. Render their
annotation as `` `[adversarial, confidence: <score>, rounds: <R>]` `` everywhere the other
findings would show `[agreement: …, confidence: …, sources: …]` — in the global `**N.**` list
and the inline comment. The names-only local list shows just the `<title>`; the
`[adversarial, …]` tag rides on the global entry and the inline comment, same as the score tag
does for the other findings. (If an adversarial finding also has a `ticket_id`,
prepend `[<ticket_id>] ` to the title as above — that is independent of the adversarial tag.)

### Step 3c: Build each inline comment

Each inline comment carries a tighter body (no disclosure repetition, no permalink — GitHub
anchors the comment to the line for you):

```markdown
**<title>** `[agreement: <N>/10, confidence: <score>, sources: <in-depth|gh-style|both>]`

<description, including any suggested-fix alternatives>
```

**Adversarial findings:** for a finding with `source = "adversarial"`, use the
`` `[adversarial, confidence: <score>, rounds: <R>]` `` tag here instead of the
`[agreement: …]` form — see Step 3b.

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

**When a GitHub MCP is connected (the preferred path), post the review through it** (see
**GitHub access**): use a create-and-submit-pull-request-review tool with the same `body` and
`event=COMMENT`; when there are inline comments, use the pending-review trio (create a pending
review → add one review comment per INLINE finding at its `path`/`line`/`side` → submit the
pending review as `COMMENT`). This is still ONE logical review. The 422 handling below applies
either way.

If the API responds 422 because one or more inline comments target lines not in the diff,
demote those specific comments to GLOBAL (append them to the global body in a "Couldn't anchor
inline" subsection) and re-issue the call. Do not silently drop findings.

**This is the ONLY GitHub write this skill is permitted to perform** (via the GitHub MCP
review-create tool, or `gh api` when no MCP is connected), and only when at least one
of {a surviving finding ≥ 60, a surviving adversarial finding (Step 2.7), an unaddressed prior
concern} is present. Do not edit the PR, add reviewers, change labels, request changes,
approve, or alter any other PR state.

## Step 4: Final report (to the user, not GitHub)

Summarise to the user in chat — this report happens whether or not a review was posted.

If `<IS_DRAFT>` was true, prepend a short note: `ℹ️ PR #<PR> is still a draft.` so the user
remembers their PR isn't ready-for-review yet.

**If at least one finding or unaddressed prior concern was posted (review issued):**

- The PR URL.
- How many findings each sub-agent originally returned (pre-merge counts), broken down by
  source: `5 × in-depth-review: [N1, N2, N3, N4, N5]`, `5 × gh-style-review: [N6..N10]`.
- How many unique findings survived the ≥ 60 filter (post-merge count), broken down as
  GLOBAL vs INLINE, and how many were both-source vs single-source.
- How many unaddressed prior concerns surfaced: `unaddressed: K`.
- **Adversarial stage:** how many findings the adversarial reviewer raised, how many the pair
  *converged* on as needing a change, how many of those were dropped as `> 50` duplicates of an
  existing finding, and how many were posted (and the round count). If it surfaced or posted
  nothing, say so.
- Sub-agents that returned `skipped_reason` (if any) and why.
- The PR review's HTML URL (the `html_url` returned by the `gh api .../reviews` response).
- Any inline comments that had to be demoted to GLOBAL because GitHub rejected them
  (line not in diff), with a brief explanation.
- Tickets examined and their outcome: `<id> ✅ | ⚠️ N gaps | ❓ unread`. If any in-depth-review
  sub-agent reported `ticket_review.status` of `denied` (user denied access) or `unavailable`
  (no Jira tooling), say so explicitly.

**If no review was posted (clean PR):**

- Lead with a clear "all clear" line, e.g. ✅ `PR #<PR> looks good — ten independent reviewers
(5 × in-depth + 5 × gh-style) raised no findings at confidence ≥ 60, the adversarial pair
converged on nothing, and there are no unaddressed discussion items. Nothing posted to GitHub.`
- The PR URL.
- How many findings each sub-agent originally returned (pre-merge counts), broken down by
  source as above.
- How many were filtered out by the ≥ 60 threshold (so the user can see whether reviewers
  flagged anything below the bar).
- **Adversarial stage:** how many findings the adversarial reviewer raised, how many the pair
  converged on as needing a change, and how many were dropped as `> 50` duplicates. If the
  adversarial stage is the reason there is anything at all, note that nothing else was posted
  only because the pair converged on nothing (or every survivor was a duplicate).
- Sub-agents that returned `skipped_reason` (if any) and why.
- Tickets examined and their outcome: `<id> ✅ | ⚠️ N gaps | ❓ unread`. If any in-depth-review
  sub-agent reported `ticket_review.status` of `denied` (user denied access) or `unavailable`
  (no Jira tooling), say so explicitly.

## Constraints

- **At most one PR review per run.** The orchestrator posts a single `POST .../pulls/<PR>/reviews`
  call (which atomically carries the global body AND all inline comments) — only when there is
  at least one surviving finding ≥ 60, at least one surviving adversarial finding (Step 2.7), OR
  at least one unaddressed prior concern. When all of those are empty, the orchestrator posts
  NOTHING (a sub-threshold ticket note alone is not enough to post). Either way: no
  `gh pr comment`, no `gh pr edit`, no extra comments, no second review.
- **The review event MUST be `COMMENT`.** Never `APPROVE`, never `REQUEST_CHANGES`. This skill
  comments; it does not gate merges.
- **Sub-agents are read-only with respect to GitHub.** They invoke `in-depth-review` or
  `gh-style-review` (both write-free) and just relay scored findings back. If any sub-agent
  appears about to issue a GitHub write, abort the entire orchestration and surface to the
  user — do not proceed to post the merged review, since the inner skill could have posted
  from a sub-agent. **The adversarial and nuanced agents (Step 2.7) are read-only too** — same
  forbidden-write list; abort the orchestration if either appears about to write to GitHub.
- **Ten parallel sub-agents (5 × in-depth-review + 5 × gh-style-review).** Launch all ten
  in a single message with concurrent Agent tool calls. Do not fall back to fewer sub-agents
  for "speed"; the cross-source triangulation is the point. Do not use only one source —
  both prompt structures contribute distinct findings.
- **Threshold is 60.** Do not raise or lower it on the fly. This applies to the ten reviewers'
  findings only. Adversarial findings (Step 2.7) do NOT use this threshold — they are gated on
  the two agents *converging* on "needs a code change," not on a confidence score.
- **Adversarial stage: one pair, agreement is the gate.** Run exactly one adversarial reviewer
  + one nuanced agent, once (not 5×). A finding posts only if (a) the pair converges to both
  "needs change" within the 3-round cap AND (b) it does not duplicate a `merged_all` finding
  with confidence > 50. Adversarial findings carry `source = "adversarial"` and the
  `[adversarial, …]` tag, bypass the ≥60 filter, and otherwise post through the same single
  review as everything else. The adversarial stage runs **regardless of `--skip-ticket`** — it
  is independent of the ticket logic.
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
- **Always produce a non-empty global body when posting.** If every surviving finding is
  inline-eligible (no global findings), the `body` still carries the disclosure line +
  `### Code review` + `Found <N> issue(s):` + the `#### local issue(s)` name list + "I left
  <N> comment(s) directly in the diff for those." If the only thing to post is an unaddressed
  prior concern (no findings at all), the `body` carries the disclosure line + the
  `### Still unaddressed in this PR` section. Never send a review with an empty `body`, and
  never emit a section header with nothing under it.
