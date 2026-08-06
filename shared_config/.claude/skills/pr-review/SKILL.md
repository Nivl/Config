---
name: pr-review
description: >
  Reviews a pull request with THREE parallel reviewer sub-agents — TWO `in-depth-review` + ONE
  `gh-style-review`, all invoked `--raw`. The orchestrator merges and deduplicates the three
  result sets into one pool, keeps findings scoring >= 60, and classifies each as INLINE
  (specific diff lines) or GLOBAL (broad/architectural). The `gh-style-review` instances also
  return Discussion Context — prior human comments the diff still leaves open — surfaced as a
  "Still unaddressed" section. Everything posts as a SINGLE PR review: global findings in full,
  a names-only list of local findings (also left as inline diff comments), and the unaddressed
  concerns. Empty sections are never emitted; if nothing survives, NOTHING is posted and the
  clean result is reported only in chat. A single approach reviewer (judging design,
  architecture, and code placement, not bugs) then debates a more-nuanced agent; findings they
  agree need a change — and that no other reviewer already raised with confidence > 50 — post
  too, tagged `approach`. Use this skill when the user
  asks to "review this PR", "pr review", "triple code review", "ensemble code review",
  "consensus review", a "thorough review of this PR", or wants higher-confidence PR feedback
  than a single `/in-depth-review` run would produce.
---

# PR Review (3x-reviewed)

This skill orchestrates **three** parallel reviewer sub-agents against a single PR: **two
instances of `in-depth-review`** (each runs ten roles by default, or nine with `--skip-ticket`; all raw scored findings) and
**one instance of `gh-style-review`** (the `@claude review` GitHub Action prompt
replicated locally, which adds Discussion Context — prior-human-comment cross-referencing
— on top of standard findings). All three are invoked with `--raw`; the orchestrator merges and
deduplicates the three result sets into one flat pool, applies a final **score ≥ 60** filter,
classifies each surviving finding as INLINE (local) or GLOBAL, aggregates the still-unaddressed
`discussion_context` from the gh-style instance, and posts a single PR review whose body
carries the global findings in full, a names-only list of the local findings, and the
still-unaddressed prior concerns. Inline comments are attached to the diff lines they refer to.
Sections with no content are never emitted.

On top of the three reviewers, a single **approach reviewer** debates a single
**more-nuanced counterpart agent** over the issues it raises (Step 2.7). This reviewer judges
one thing only — is this the right way to build it (design, architecture, code placement,
over/under-engineering) — not bugs. Only the findings the pair *converges* on as genuinely
needing a code change survive, and only if no other reviewer already proposed the same thing
with confidence > 50. Survivors join the same posting pipeline, tagged `approach`. This pair
runs once (not 3×), and its findings post on agreement alone —
they do not have to clear the ≥ 60 confidence bar the other findings do.

The point: three independent passes from two different prompt structures (specialized-role
vs. GitHub-Action mirror) catch different issues, AND converge on the real ones. One
review entry, three reviewers' worth of recall, plus an explicit "what humans already raised
that the diff still hasn't addressed" section.

**Why the split is asymmetric (2 in-depth, 1 gh-style).** Measured on fixtures with planted
issues, `gh-style-review`'s findings were a strict SUBSET of `in-depth-review`'s on every
fixture — it surfaced nothing in-depth missed, and skipped test-coverage findings entirely.
So a second gh-style instance buys no incremental finding recall. It stays at **one** instance
because its real contribution is **Discussion Context**: cross-referencing prior human comments
against the diff, which `in-depth-review` cannot do at all and which the fixtures could not
measure. Spend the finder budget on in-depth; keep exactly one gh-style for the discussion
pass. Do not drop gh-style to zero, and do not raise it back to parity.

## Prerequisites

- The current branch (or the PR specified as the skill argument) must have an **open PR**.
  Draft PRs are accepted — reviewing a draft is a valid workflow (you get feedback before
  marking the PR ready). Both `in-depth-review` and `gh-style-review` accept drafts.
- Both the `in-depth-review` and `gh-style-review` skills must be installed and available
  (they live alongside this skill in `shared_config/.claude/skills/`).
- A GitHub MCP server must be connected and authenticated — OR `gh` must be installed and
  authenticated, which the skill falls back to when no GitHub MCP is available (see **GitHub access**).

**Flag:** pass `--skip-ticket` to disable ticket intent compliance (Role #10) across both
`in-depth-review` instances and skip the Jira-tooling preflight.

**Flag:** `--announce` / `--no-announce` control the optional "review in progress" comment
(Step 0.7). With neither flag the skill prompts once; the flag pre-answers and skips the
prompt (`--announce` = post it, `--no-announce` = do not). Announcing needs GitHub write
access — the same access posting the review needs. In a headless run pass one of these flags,
since the interactive prompt would otherwise block.

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
| `gh api -X POST …/pulls/<N>/reviews` | **create-and-submit pull request review** (or the pending-review + add-review-comment + submit trio for inline comments) — the review write |
| `gh pr comment <N> --body …` | add issue comment — the opt-in in-progress comment (Step 0.7) |
| `gh api -X DELETE …/issues/comments/<id>` | delete issue comment — removes the in-progress comment at the end of the run (Step 3e) |

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
   never runs. When `false`, both in-depth-review instances run Role #10 (two ticket
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
7. **Announcement decision.** Do this last — once every check above has passed and the
   reviewers are about to launch — so a preflight abort never leaves a stray comment. Resolve
   whether to post a "review in progress" comment:
   - `--announce` -> yes; `--no-announce` -> no; neither flag -> ask the user one yes/no
     question ("Post a 'review in progress' comment to the PR?").
   - If **no**, leave `<ANNOUNCE_COMMENT_ID>` empty and continue — nothing else changes.
   - If **yes**, post ONE PR conversation comment (a GitHub issue comment, not a review) with
     exactly this body:

     > I've started an automated review of this PR. It usually takes about 30 minutes to complete.

     Prefer the GitHub MCP add-issue-comment tool; fall back to `gh pr comment <PR> --body "..."`.
     Save the created comment's numeric id as `<ANNOUNCE_COMMENT_ID>` for deletion in Step 3e —
     the MCP response returns the id directly; the `gh pr comment` fallback returns only the
     comment URL, so take the numeric id from its `#issuecomment-<id>` fragment.
     If the post fails (e.g. no write access), warn the user in chat, leave
     `<ANNOUNCE_COMMENT_ID>` empty, and run the review anyway — the review is the deliverable and
     the comment is best-effort.

## Step 1: Launch three reviewer sub-agents in parallel

Spawn **three sub-agents in a single message** (three concurrent Agent tool calls). Sequential
launches defeat the purpose — never serialize. **Spawn all three on Sonnet** (Agent-tool
`model: sonnet`): each is a thin wrapper that invokes a recall-pass skill and relays its JSON,
and `in-depth-review` / `gh-style-review` already pin their own internal tiers (reviewers →
Sonnet, scorers → Haiku). Never let these inherit the session model (it may be Opus / `[1m]`).
The split is:

- **Sub-agents 1–2:** invoke `in-depth-review`
- **Sub-agent 3:** invokes `gh-style-review`

### Sub-agents 1–2 prompt (in-depth-review)

```
You are sub-agent N of 3 in a pr-review orchestration (N is 1 or 2). Your job:
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
   - `"sub_agent": N` (which of the 3 instances you are)
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

### Sub-agent 3 prompt (gh-style-review)

```
You are sub-agent 3 of 3 in a pr-review orchestration. Your job:
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
   - `"sub_agent": 3`
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
orchestrator's threshold is **60** (lower than each sub-skill's default because the 4×
cross-instance triangulation — two from each prompt structure — raises confidence in
60-69 findings). `--raw` makes the sub-agents return all scored findings; we apply the
60 cutoff in Step 2 after merging.

## Step 2: Merge and deduplicate (findings)

Once all three sub-agents have returned:

1. **Pool every finding** across the three result sets into one flat pool. Each finding carries
   its `confidence`, `file`, `line_range`, originating `sub_agent` (1..3), and `source`
   (`"in-depth-review"` or `"gh-style-review"`). Don't pre-segregate by source — the cross-
   prompt triangulation is the point.

2. **Group duplicates.** Two findings are duplicates if they refer to the **same file** and have
   **overlapping line ranges** AND describe substantially the same problem (paraphrases count).
   Findings from different sources (one from in-depth-review, one from gh-style-review) that
   describe the same problem are duplicates — merge them.

3. **For each group, produce one merged finding:**
   - `confidence`: **max** of the group's scores (any one reviewer with high confidence is strong
     evidence; merging by max is intentionally non-conservative).
   - `agreement`: count of distinct sub-agents (1..3) that raised this finding.
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
   the next step discards. Step 2.7 (approach dedup) needs it — once you apply the ≥60 cut
   you can no longer tell whether a 51–59 finding "was already proposed by another reviewer."
   `merged_all` is used only by Step 2.7; it never affects posting on its own.

5. **Filter:** keep only findings with `confidence >= 60`. Discard the rest — with one
   exception: retain any discarded `ticket`-category finding (confidence < 60) in a separate
   `sub_threshold_ticket_notes` list. Step 3a.5 uses it for the "Tickets examined" section.
   These never enter the findings pool, the INLINE/GLOBAL classification, or the ordering
   below — they are notes, not findings. (Threshold is 60.)

6. **Order the surviving findings:**
   1. `confidence` descending
   2. `agreement` descending (3/3 > 2/3 > 1/3 when scores tie)
   3. Both-sources first (a finding raised by both in-depth-review and gh-style-review beats
      a same-confidence-and-agreement finding from a single source)
   4. `category` priority: bug > AGENTS.md > history > prior PR > comment guidance > ticket

## Step 2.5: Aggregate Discussion Context (from gh-style sub-agents only)

The single `gh-style-review` sub-agent returns a `discussion_context` block with `resolved` and
`unaddressed` arrays. `in-depth-review` returns no such block — it contributes nothing here.
Only the **unaddressed** concerns are rendered (the "Addressed by this PR" section was dropped
as noise — listing what the diff already fixed is not actionable). The `resolved` entries are
not rendered at all.

Because exactly ONE instance produces this block, there is no cross-instance dedup, no
disagreement detection, and no agreement count. This is the deliberate trade of the asymmetric
split: Discussion Context is now a single-reviewer judgement. Treat its entries as leads
grounded in a real comment URL, not as triangulated findings.

1. **Take the instance's `unaddressed` array** as `unaddressed_pool` directly. Each entry
   carries `quote`, `author`, `url`, and `gap`.

2. **Deduplicate by `url`** within that array (the GitHub comment URL is the canonical identity
   of a discussion item). A single instance can still list the same comment twice; collapse those.

3. **Retain all entries** — there's no confidence threshold here because every entry is
   grounded in a real human comment URL. Order `unaddressed_pool` by the comment's `created_at`
   ascending (oldest unresolved concern first).

5. **If `unaddressed_pool` is empty after dedup, the "Still unaddressed" section is skipped.**
   If it is empty AND the findings list from Step 2 is also empty, Step 3 still applies — no
   review is posted.

## Step 2.7: Approach review stage (one pair, run once)

A single **approach reviewer** and a single **more-nuanced counterpart agent** debate the
issues the approach reviewer raises. This stage judges one thing only: **is this the right way
to build it?** The approach reviewer does not hunt for bugs, edge cases, races, security, or
error-handling gaps. The three reviewers already cover those. It weighs the design instead. Is
the approach and the implementation the best available? Is the code over- or under-engineered?
Is it in the right place (a DD count stat emitted in the wrong layer, say)? Is the architecture
solid or duct-taped? Does a pattern or utility for this already exist in the repo that the
author reinvented? Only the issues the two *converge* on as genuinely needing a code change
survive, and only if no other reviewer already raised the same thing with confidence > 50.
Survivors are added to the findings pool (Step 2 output) so Step 3 posts them exactly like
every other finding — distinguished only by an `approach` tag.

**Model: spawn the approach reviewer and the nuanced agent on Opus** (Agent-tool
`model: opus` — the standard 200k tier, NOT a `[1m]` variant). This is the one stage that is
genuine judgment rather than recall — the debate IS the filter — so it keeps the strongest
model. It is also cheap to keep there: one pair, run once (not 3×), ≤3 rounds. Everything
else in this skill runs on Sonnet/Haiku.

This stage runs **after** the merge because the dedup in Step 2.7b below compares against
`merged_all`, which only exists once Step 2 has merged the three reviewers. The pair runs **once**
(not 3×) — the internal debate is the filter, so cross-instance triangulation is not needed.

**Both agents are read-only with respect to GitHub** — same forbidden-write list as sub-agents
1–4. The approach reviewer **explores the wider codebase** to ground its judgment. It reads
neighboring modules, greps for existing patterns and utilities, and checks where similar code
already lives, not just the PR diff. Neither agent is given the **other reviewers'** findings,
so this pass stays independent (the dedup happens here, at the orchestrator, not inside the
agents).

### Step 2.7a: The debate (orchestrator-mediated, up to 3 rounds)

The orchestrator mediates the exchange, carrying the current state of every contested finding
between agents. **Round 1 is the propose+judge pass (count = 1); each later rebut iteration —
one approach re-spawn plus one nuanced re-spawn — is one more round.** Track, per finding,
the latest `approach_verdict` and `nuanced_verdict` (each `yes` = needs a code change, or `no`); a
finding's `rounds` is the 1-based round at which it converged.

1. **Round 1 — propose + judge.** Spawn the approach reviewer (prompt below); it returns a
   `findings` list, each with `{id, title, file, line_range, description, suggested_fix,
   confidence (0–100), argument}`. Every proposed finding starts at `approach_verdict = yes`. Then
   spawn the nuanced agent with the diff + that `findings` list; it returns a `needs_change`
   (`yes`/`no`) + `argument` per finding id → set `nuanced_verdict`.
2. **Classify each finding:** *converged-keep* (both `yes`), *converged-drop* (both `no`), or
   *contested* (verdicts differ).
3. **Rounds 2–3 — rebut.** While there is at least one *contested* finding AND fewer than 3
   total rounds have run: re-spawn the approach reviewer with, for each contested finding,
   the nuanced agent's latest `argument`; it returns an updated `{needs_change, argument}` per
   id (it may concede to `no` or hold at `yes`) → update `approach_verdict`. Then re-spawn the
   nuanced agent with the approach reviewer's latest `argument` per contested finding; it
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
approach findings that another reviewer already proposed with confidence > 50, *including*
the 51–59 ones that did not clear the ≥60 posting bar. A surviving finding that matches nothing
in `merged_all` over 50 is kept.

### Step 2.7c: Tag and fold into the findings pool

Each kept approach finding becomes a normal finding for Step 3 with:

- `source = "approach"`, `agreement = null` (it is not one of the 6 reviewers).
- `confidence` = the approach reviewer's own round-1 score — used **only** for
  ordering/display. The rebut rounds do not re-emit `confidence`, so carry the round-1 value
  forward.
- `rounds` = how many debate rounds it took to converge.
- no `permalink` — approach findings skip the Step 2 merge that assigns one, so the global
  `**N.**` block omits the permalink paragraph for them.

**Agreement is the gate, not the score.** Approach findings are added to the Step 2 findings
pool **without** re-applying the ≥60 filter — they already passed the mutual-agreement bar in
2.7a. Order them within the pool by `confidence` like the others (for the tiebreakers, treat
`agreement = null` as lowest, an approach finding as single-source — never both — for the
both-sources tiebreaker, and lowest for the category-priority tiebreaker). They then flow
through Step 3's INLINE/GLOBAL classification and posting unchanged, except for the annotation
in Step 3b.

If no approach finding survives 2.7a, or all survivors are dropped in 2.7b, this stage adds
nothing and the rest of the run is unchanged.

### Step 2.7 sub-agent prompts

**Approach reviewer (round 1 — propose):**

```
You are the APPROACH reviewer in a pr-review orchestration reviewing PR #<PR>. You judge ONE
thing: is this the right way to build it? You do NOT hunt for bugs, edge cases, race
conditions, security holes, or missing error handling — other reviewers cover those, and a
defect finding here will be ignored. Assume the author is new to this codebase. Their code may
run, yet still be the worst possible implementation. Your job is to catch that.

Weigh only design and structure:
- Is the approach the best approach, and the implementation the best implementation?
- Is it over-engineered (needless abstraction, indirection, config for a constant) or
  under-engineered (a fragile hack where the codebase already has a real mechanism)?
- Is the code in the right place? E.g. a Datadog count stat emitted in the wrong layer, logic
  in a controller that belongs in a service, a helper defined far from where it is used.
- Is the architecture solid, or is it duct-tape that will not hold?
- Does a pattern or utility for this ALREADY EXIST in the repo that the author reinvented?

To judge these you must look beyond the diff: read neighboring modules, grep for existing
patterns and utilities, and check where similar code already lives. Ground every finding in
what the codebase actually does, not a guess.

For every issue you can defend, return one finding. Return STRICT JSON:
{ "findings": [ {
  "id": "A1", "title": "...", "file": "<path>", "line_range": "<start>..<end>",
  "description": "<what is wrong with the approach and why>",
  "suggested_fix": "<the better approach, correct placement, or existing thing to reuse>",
  "confidence": <0-100>,
  "argument": "<why this genuinely needs a code change>"
} ] }

`confidence` is your own 0–100 estimate (used only for display ordering). Do not invent issues
you cannot defend — a weak finding will be argued down in the debate that follows. Return
`{"findings": []}` if the approach is sound.

Forbidden (you are read-only w.r.t. GitHub): `gh pr comment`, `gh pr review`, `gh pr edit`,
`gh pr close`, `gh pr merge`, `gh issue create`, `gh issue comment`, or any other GitHub write.
```

**Nuanced agent (each round — judge):**

```
You are the NUANCED reviewer in a pr-review orchestration, the pragmatic counterweight to an
approach reviewer on PR #<PR>. The approach reviewer critiques design and structure — whether
the code is over- or under-engineered, in the wrong place, duct-taped, or reinvents something
the repo already has. For each finding it raises, decide whether the rework GENUINELY warrants
a code change in THIS PR. Weigh the design tradeoff: how much better the proposed approach
really is, the cost and risk of the rework, whether "works but not ideal" is acceptable here,
and whether the change is in scope or better left to a follow-up. You are not here to
rubber-stamp and not here to reflexively dismiss — judge each on its merits, given the diff and
the surrounding codebase.

You receive the PR diff and a list of findings — the approach reviewer's full proposal set in
round 1, or only the still-contested findings in later rounds — each with the approach
reviewer's latest `argument`. Return STRICT JSON:
{ "verdicts": [ { "id": "A1", "needs_change": "yes" | "no", "argument": "<your reasoning>" } ] }

Use "yes" only when you agree the rework is warranted in this PR. Use "no" when it is not worth
a change. Respond to the approach reviewer's specific argument — concede when they are right,
push back when they overreach.

Forbidden (read-only w.r.t. GitHub): `gh pr comment`, `gh pr review`, `gh pr edit`,
`gh pr close`, `gh pr merge`, `gh issue create`, `gh issue comment`, or any other GitHub write.
```

**Approach reviewer (rounds 2–3 — rebut):** re-use the approach prompt above, but instead
of proposing fresh findings, pass it the contested findings plus the nuanced agent's latest
`argument` for each, and ask it to return
`{ "responses": [ { "id": "...", "needs_change": "yes" | "no", "argument": "..." } ] }` —
holding at `yes` only where it can still defend the issue, conceding to `no` otherwise.

## Step 3: Post the review (only if there is something worth posting)

**If the findings list AND `unaddressed_pool` are BOTH EMPTY, do NOT post anything to GitHub.**
The findings list here is the Step 2 filtered findings PLUS any approach survivors folded in
by Step 2.7c — so a single agreed approach finding is on its own enough to post a review.
When both pools are empty, skip to Step 3e (which deletes the in-progress comment if one was
posted) and then Step 4, and tell the user in chat that the PR looks clean. No review is
posted and no PR state changes — deleting the opt-in in-progress comment leaves the PR with
nothing on it, so silence on GitHub stays the success signal. A
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

Build `tickets_examined` = union by `id` of the `tickets_examined` arrays returned by the two
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

**Confidence label (what the reader sees).** Internally a finding has a numeric `confidence`
(0-100), used ONLY for filtering and ordering. The POSTED comment never shows the number, the
agreement count, or the source skill. Those are internal mechanics that confuse readers. It
shows one word, mapped from the score:

| score | label |
|---|---|
| 90-100 | `Critical` |
| 75-89 | `High` |
| 60-74 | `Medium` |
| below 60 | `Low` |

(Normal findings cleared the >=60 bar, so they read `Medium` or higher; only an approach
finding can bypass that bar and read `Low` -- e.g. a score of 42 shows as `Low`.) The whole tag
is just `` `[confidence: <Low|Medium|High|Critical>]` `` (plus the `approach` marker for
those findings, and a `[<ticket_id>]` title prefix for ticket findings).

```markdown
I used an AI agent with a custom prompt to generate this review.

<<if K_global + K_inline > 0:>>

### Code review

Found <K_global + K_inline> issue(s):

<<if K_global > 0:>>

#### <K_global> global issue(s)

**1.** <title> &nbsp;`[confidence: <Low|Medium|High|Critical>]`

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
  > ⚠️ <gap>
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

**Approach findings (`source = "approach"`):** these carry the `approach` marker.
Render their annotation as `` `[approach, confidence: <Low|Medium|High|Critical>]` ``
everywhere the other findings show `[confidence: ...]`: in the global `**N.**` list and the
inline comment. Bucket the approach finding's own score with the same mapping above (a
sub-60 approach finding reads `Low`). Do NOT show the round count; it is an internal debate
mechanic, not reader signal. The names-only local list shows just the `<title>`; the
`[approach, ...]` tag rides on the global entry and the inline comment. If an approach
finding also has a `ticket_id`, prepend `[<ticket_id>] ` to the title as above; that is
independent of the approach marker.

### Step 3c: Build each inline comment

Each inline comment carries a tighter body (no disclosure repetition, no permalink — GitHub
anchors the comment to the line for you):

```markdown
**<title>** `[confidence: <Low|Medium|High|Critical>]`

<description, including any suggested-fix alternatives>
```

**Approach findings:** for a finding with `source = "approach"`, use the
`` `[approach, confidence: <Low|Medium|High|Critical>]` `` tag here instead. See Step 3b.

GitHub line-range encoding:

- Single line: `{ "path": "<file>", "line": <N>, "side": "RIGHT", "body": "<body>" }`
- Multi-line: `{ "path": "<file>", "start_line": <S>, "start_side": "RIGHT", "line": <E>, "side": "RIGHT", "body": "<body>" }`

`side` is always `RIGHT` (the new revision). Comments on the LEFT side (deleted lines) are
seldom useful for a forward-looking review and are out of scope here.

### Step 3c.7: Human-review gate (before posting)

Before any GitHub write, dump every finding to a local file, pause, and post only what the
user approves. **This is a hard gate.** Do not post to GitHub until the user says to.

This step runs **only when Step 3 has something to post** (at least one finding or at least
one unaddressed concern). The clean-PR path is unchanged: when there is nothing to post, skip
straight to Step 3e as before. No file is written and there is no pause.

1. **Write** `/tmp/claude/pr-review-<PR>-comments.md`. Emit **one block per finding**, numbered
   `#1` through `#N` in the Step 2 order (GLOBAL and INLINE findings), followed by the
   unaddressed concerns. Each of these is its own block:
   - each GLOBAL finding,
   - each INLINE comment,
   - each entry in `unaddressed_pool`.

   Each block is:
   - a header line: `#<n> [GLOBAL]`, or `#<n> [INLINE] <file>:<line_range>`, or
     `#<n> [UNADDRESSED]`. Append the confidence label and any `approach` /
     `[<ticket_id>]` tag the finding carries, so the header reads the way the posted comment
     will (e.g. `#3 [INLINE] src/auth.ts:40..52  [confidence: High]`).
   - the rendered comment body below it: title + description for a finding, or the
     `quote` + `gap` for an unaddressed concern.

   Separate every block from the next with a divider line that is exactly thirteen `=`
   characters on its own line, nothing else:

   ```
   =============
   ```

   The divider makes each finding visually distinct so the user can scan and judge them one at
   a time.

2. **Pause.** Tell the user the file path and a one-line tally: `<K_global> global,
   <K_inline> inline, <K_unaddressed> unaddressed`. Ask them to review the file and confirm
   before you post. Then wait.

3. **Act on the user's response:**
   - **Approve all** -> continue to Step 3d and post everything.
   - **Keep a subset** (e.g. "drop #2 and #5", "only post #1 and #4") -> remove the dropped
     findings from the pools, re-run the Step 3b body assembly and Step 3c inline set from the
     survivors (so counts, numbering, and the global/local lists all reflect the reduced set),
     rewrite the file, then continue to Step 3d with the remainder. If dropping leaves nothing
     to post, treat it as a decline.
   - **Decline all** -> post nothing. Skip Step 3d entirely, still run Step 3e cleanup, and
     report in Step 4 that the comments were written to the file but not posted at the user's
     request.

This gate makes the skill interactive by design. In a headless run there is no one to approve,
so the run stops at this file with nothing posted. That is the intended safe default.

### Step 3d: Post the review (one API call)

Reached only after the Step 3c.7 gate has approved (all findings, or the kept subset). Post
exactly the surviving set.

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

**The posted review is the only PR review this skill writes** (via the GitHub MCP review-create
tool, or `gh api` when no MCP is connected), and only when at least one of {a surviving finding
≥ 60, a surviving approach finding (Step 2.7), an unaddressed prior concern} is present. The
skill's only other permitted writes are the opt-in in-progress comment (Step 0.7) and its
deletion (Step 3e) — both orchestrator-only. Do not edit the PR, add reviewers, change labels,
request changes, approve, or alter any other PR state.

## Step 3e: Delete the in-progress comment

If `<ANNOUNCE_COMMENT_ID>` is set (an in-progress comment was posted in Step 0.7), delete it
now — on every path, whether or not a review was posted. The clean-PR path routes here too, so
do not assume Step 3d ran or that `OWNER_REPO` is set. Prefer the GitHub MCP delete-issue-comment
tool (pass owner/repo plus the comment id). Fall back to
`gh api -X DELETE "/repos/<owner>/<repo>/issues/comments/<ANNOUNCE_COMMENT_ID>"`, resolving
owner/repo with `gh repo view --json owner,name` if not already known. If deletion fails, note
it in the Step 4 report and continue — the review is already delivered, so a failed cleanup
never fails the run. If `<ANNOUNCE_COMMENT_ID>` is empty, do nothing here.

## Step 4: Final report (to the user, not GitHub)

Summarise to the user in chat — this report happens whether or not a review was posted.

If `<IS_DRAFT>` was true, prepend a short note: `ℹ️ PR #<PR> is still a draft.` so the user
remembers their PR isn't ready-for-review yet.

If an in-progress comment was posted (Step 0.7), the run deleted it in Step 3e; if that
deletion failed, say so and include the comment URL so the user can remove it manually.

**Review file (Step 3c.7).** Whenever the gate ran, the assembled comments were written to
`/tmp/claude/pr-review-<PR>-comments.md`. Always report this path. If the user kept only a
subset, name which findings they dropped. If the user **declined** (or dropped everything),
lead with a clear line that the review was written to that file but NOT posted to GitHub at
the user's request, then still give the tallies below.

**If at least one finding or unaddressed prior concern was posted (review issued):**

- The PR URL.
- How many findings each sub-agent originally returned (pre-merge counts), broken down by
  source: `2 × in-depth-review: [N1, N2]`, `1 × gh-style-review: [N3]`.
- How many unique findings survived the ≥ 60 filter (post-merge count), broken down as
  GLOBAL vs INLINE, and how many were both-source vs single-source.
- How many unaddressed prior concerns surfaced: `unaddressed: K`.
- **Approach stage:** how many findings the approach reviewer raised, how many the pair
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

- Lead with a clear "all clear" line, e.g. ✅ `PR #<PR> looks good — three independent reviewers
(2 × in-depth + 1 × gh-style) raised no findings at confidence ≥ 60, the approach pair
converged on nothing, and there are no unaddressed discussion items. Nothing posted to GitHub.`
- The PR URL.
- How many findings each sub-agent originally returned (pre-merge counts), broken down by
  source as above.
- How many were filtered out by the ≥ 60 threshold (so the user can see whether reviewers
  flagged anything below the bar).
- **Approach stage:** how many findings the approach reviewer raised, how many the pair
  converged on as needing a change, and how many were dropped as `> 50` duplicates. If the
  approach stage is the reason there is anything at all, note that nothing else was posted
  only because the pair converged on nothing (or every survivor was a duplicate).
- Sub-agents that returned `skipped_reason` (if any) and why.
- Tickets examined and their outcome: `<id> ✅ | ⚠️ N gaps | ❓ unread`. If any in-depth-review
  sub-agent reported `ticket_review.status` of `denied` (user denied access) or `unavailable`
  (no Jira tooling), say so explicitly.

## Constraints

- **At most one PR review per run.** The orchestrator posts a single `POST .../pulls/<PR>/reviews`
  call (which atomically carries the global body AND all inline comments) — only when there is
  at least one surviving finding ≥ 60, at least one surviving approach finding (Step 2.7), OR
  at least one unaddressed prior concern. When all of those are empty, the orchestrator posts
  no review (a sub-threshold ticket note alone is not enough to post). The only other writes the
  orchestrator may make are the opt-in in-progress comment (Step 0.7) and its later deletion
  (Step 3e). Beyond those: no `gh pr edit`, no other comments, no second review.
- **Posting is gated on human approval (Step 3c.7).** Whenever there is something to post, the
  assembled findings are first written to `/tmp/claude/pr-review-<PR>-comments.md`, one block
  per finding split by `=============` dividers, and the run pauses. Only the findings the user
  approves are posted; a declined gate posts nothing. This makes the skill interactive — a
  headless run stops at the file with nothing posted, which is the intended safe default. The
  clean-PR path (nothing to post) does not write a file or pause.
- **The review event MUST be `COMMENT`.** Never `APPROVE`, never `REQUEST_CHANGES`. This skill
  comments; it does not gate merges.
- **Sub-agents are read-only with respect to GitHub.** They invoke `in-depth-review` or
  `gh-style-review` (both write-free) and just relay scored findings back. If any sub-agent
  appears about to issue a GitHub write, abort the entire orchestration and surface to the
  user — do not proceed to post the merged review, since the inner skill could have posted
  from a sub-agent. **The approach and nuanced agents (Step 2.7) are read-only too** — same
  forbidden-write list; abort the orchestration if either appears about to write to GitHub.
- **Three parallel sub-agents (2 × in-depth-review + 1 × gh-style-review).** Launch all three
  in a single message with concurrent Agent tool calls. Do not fall back to fewer sub-agents
  for "speed"; the cross-source triangulation is the point. Do not use only one source —
  both prompt structures contribute, and dropping gh-style also drops Discussion Context.
  The split is deliberately asymmetric; do not "balance" it back to 2 + 2 (see the overview).
- **Comment-punctuation findings are in scope but low priority.** The sub-skills flag comments
  the diff adds or edits that join clauses with ` - ` (space-hyphen-space) or a sentence-splitting
  `:`, per AGENTS.md. These are `suggestion`-severity: keep them if they survive the threshold,
  but never let them displace correctness findings in the posted review.
- **Model policy (cost):** the three reviewer sub-agents run on **Sonnet** (`model: sonnet`);
  their inner reviewers/scorers self-tier (Sonnet/Haiku) per those skills. Only the
  approach + nuanced pair (Step 2.7) runs on **Opus** (`model: opus`, standard 200k — never
  `[1m]`), because that stage is judgment, not recall. Never let any of these inherit the
  session model. The ≥60 triangulation and the converge stage are what protect quality — not a
  bigger model on every finder.
- **Threshold is 60.** Do not raise or lower it on the fly. This applies to the three reviewers'
  findings only. Approach findings (Step 2.7) do NOT use this threshold — they are gated on
  the two agents *converging* on "needs a code change," not on a confidence score.
- **Approach stage: one pair, agreement is the gate.** Run exactly one approach reviewer
  + one nuanced agent, once (not 3×). The approach reviewer judges design, architecture, and
  code placement only — not bugs — and explores the wider codebase to do so. A finding posts
  only if (a) the pair converges to both "needs change" within the 3-round cap AND (b) it does
  not duplicate a `merged_all` finding with confidence > 50. Approach findings carry
  `source = "approach"` and the `[approach, …]` tag, bypass the ≥60 filter, and otherwise post
  through the same single review as everything else. The approach stage runs **regardless of
  `--skip-ticket`** — it is independent of the ticket logic.
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
