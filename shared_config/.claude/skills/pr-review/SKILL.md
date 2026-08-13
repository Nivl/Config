---
name: pr-review
description: >
  Reviews a pull request with four parallel reviewer sub-agents — two `in-depth-review` on
  Sonnet, one `in-depth-review` on Opus (the subtle-bug catcher), and one `gh-style-review` —
  merges and deduplicates their findings, and posts a SINGLE PR review combining global
  findings, a names-only list of inline findings (also left as inline diff comments), and any
  prior discussion the diff still leaves unaddressed. A separate approach-vs-nuanced reviewer
  pair debates design/architecture findings and adds only what they converge on. Nothing is
  posted for a clean PR; posting is gated on human approval of a local draft file first.
  Use this skill when the user asks to "review this PR", "pr review", "triple code review",
  "ensemble code review", "consensus review", a "thorough review of this PR", or wants
  higher-confidence PR feedback than a single `/in-depth-review` run would produce.
---

# PR Review (4x-reviewed, mixed-tier)

This skill orchestrates **four** parallel reviewer sub-agents against a single PR: **two
instances of `in-depth-review` on Sonnet** (each runs nine to twelve roles depending on what the diff contains, one fewer with `--skip-ticket`; all raw scored findings),
**one instance of `in-depth-review` on Opus** (the subtle-bug catcher — see Mixed-tier finders
below), and
**one instance of `gh-style-review`** (the `@claude review` GitHub Action prompt
replicated locally, which adds Discussion Context — prior-human-comment cross-referencing
— on top of standard findings). All four are invoked with `--raw`; the orchestrator merges and
deduplicates the four result sets into one flat pool, applies a final **score >= 60** filter,
classifies each surviving finding as INLINE (local) or GLOBAL, aggregates the still-unaddressed
`discussion_context` from the gh-style instance, and posts a single PR review whose body
carries the global findings in full, a names-only list of the local findings, and the
still-unaddressed prior concerns. Inline comments are attached to the diff lines they refer to.
Sections with no content are never emitted.

On top of the four reviewers, a single **approach reviewer** debates a single
**more-nuanced counterpart agent** over the issues it raises (Step 2.7). This reviewer judges
one thing only — is this the right way to build it (design, architecture, code placement,
over/under-engineering) — not bugs. Only the findings the pair *converges* on as genuinely
needing a code change survive, and only if no other reviewer already proposed the same thing
with confidence > 50. Survivors join the same posting pipeline, tagged `approach`. This pair
runs once (not 3x), and its findings post on agreement alone.
They do not have to clear the >= 60 confidence bar the other findings do.

The point: four independent passes from two different prompt structures (specialized-role
vs. GitHub-Action mirror) and two model tiers catch different issues, AND converge on the real
ones. One review entry, four reviewers' worth of recall, plus an explicit "what humans already
raised that the diff still hasn't addressed" section.

**Mixed-tier finders (2 Sonnet + 1 Opus in-depth).** Reviewer cost-efficiency turned out to be
**difficulty-dependent**, which is why the finder pool is deliberately not uniform:

- On easy or routine diffs, 1–2 Sonnet reviewers already saturate recall. Opus adds nothing
  there, so most of the pool stays on Sonnet.
- On hard diffs with subtle bugs, **one Opus reviewer beat three Sonnet reviewers** on recall
  (93.8% vs 87.5%), at lower cost, with roughly a fifth of the false positives. Opus was the
  only configuration to catch the subtlest planted bugs at all.

A fixed all-Sonnet pool is provisioned for the easy case and quietly loses recall on the hard
one. Adding a single Opus in-depth instance covers the hard case without paying Opus rates on
every finder. Keep it at exactly one. The point is coverage of the subtle tail, not tier parity.
Do NOT promote the Sonnet finders to Opus, and do NOT drop the Opus one to save cost. It is the
cheapest recall in the pool on the diffs that matter most.

**Why the source split is asymmetric (3 in-depth, 1 gh-style).** Measured on fixtures with planted
issues, `gh-style-review`'s findings were a strict SUBSET of `in-depth-review`'s on every
fixture. It surfaced nothing in-depth missed, and skipped test-coverage findings entirely.
So a second gh-style instance buys no incremental finding recall. It stays at **one** instance
because its real contribution is **Discussion Context**: cross-referencing prior human comments
against the diff, which `in-depth-review` cannot do at all and which the fixtures could not
measure. Spend the finder budget on in-depth; keep exactly one gh-style for the discussion
pass. Do not drop gh-style to zero, and do not raise it back to parity.

## Prerequisites

- The current branch (or the PR specified as the skill argument) must have an **open PR**.
  Draft PRs are accepted. Reviewing a draft is a valid workflow (you get feedback before
  marking the PR ready). Both `in-depth-review` and `gh-style-review` accept drafts.
- Both the `in-depth-review` and `gh-style-review` skills must be installed and available
  (they live alongside this skill in `shared_config/.claude/skills/`).
- A GitHub MCP server must be connected and authenticated — OR `gh` must be installed and
  authenticated, which the skill falls back to when no GitHub MCP is available (see **GitHub access**).

**Flag:** pass `--skip-ticket` to disable ticket intent compliance (Role #10) across all three
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
a GitHub MCP nor `gh` is available, abort in Step 0 and tell the user. This skill cannot
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
4. Re-confirm that **both** `in-depth-review` AND `gh-style-review` are available. Check the
   available-skills list for both entries. If either is missing, abort the orchestration
   and tell the user which one to install.
5. If the invocation included `--skip-ticket`, set `<SKIP_TICKET> = true` (default `false`).
   When `true`, every in-depth-review sub-agent is invoked with `--skip-ticket`, so Role #10
   never runs. When `false`, all three in-depth-review instances run Role #10 (three ticket
   reviewers). `gh-style-review` is unaffected either way. It has no ticket role.
6. **Jira-tooling preflight** (skip this step entirely if `<SKIP_TICKET>` is true). Before
   launching any reviewers, confirm a Jira reader is available AND authenticated:
   - acli: installed (`command -v acli`) and able to read Jira. Run a lightweight
     authenticated acli call; if it fails with an auth/login error, treat acli as
     unauthenticated. In a sandboxed session, skip the acli probe and treat acli as
     unavailable, since sandboxed acli fails even when installed and authenticated, and rely
     on the MCP check below; OR
   - a Jira/Atlassian MCP: connected and authenticated. Search available tools (e.g.
     ToolSearch "atlassian jira"). If the only exposed tool is an `authenticate` tool, it is
     connected but not yet authed.
     If neither is ready, ASK the user to choose:
     (a) install/authenticate acli or the Atlassian MCP, then continue. Re-check after they confirm;
     (b) proceed now with `--skip-ticket` — set `<SKIP_TICKET> = true` and run the other
     reviewers without the ticket check;
     (c) abort the review.
     Do not launch any reviewers until this is resolved. If a re-check after choice (a) still
     fails, present the three choices again rather than proceeding.
7. **Announcement decision.** Do this last, once every check above has passed and the
   reviewers are about to launch, so a preflight abort never leaves a stray comment. Resolve
   whether to post a "review in progress" comment:
   - `--announce` -> yes; `--no-announce` -> no; neither flag -> ask the user one yes/no
     question ("Post a 'review in progress' comment to the PR?").
   - If **no**, leave `<ANNOUNCE_COMMENT_ID>` empty and continue. Nothing else changes.
   - If **yes**, post ONE PR conversation comment (a GitHub issue comment, not a review) with
     exactly this body:

     > I've started an automated review of this PR. It usually takes about 30 minutes to complete.

     Prefer the GitHub MCP add-issue-comment tool; fall back to `gh pr comment <PR> --body "..."`.
     Save the created comment's numeric id as `<ANNOUNCE_COMMENT_ID>` for deletion in Step 3e.
     The MCP response returns the id directly. The `gh pr comment` fallback returns only the
     comment URL, so take the numeric id from its `#issuecomment-<id>` fragment.
     If the post fails (e.g. no write access), warn the user in chat, leave
     `<ANNOUNCE_COMMENT_ID>` empty, and run the review anyway. The review is the deliverable and
     the comment is best-effort.

## Step 1: Launch four reviewer sub-agents in parallel

Spawn **four sub-agents in a single message** (four concurrent Agent tool calls). Sequential
launches defeat the purpose. Never serialize. Each is a thin wrapper that invokes a recall-pass
skill and relays its JSON.

**Spawn each one by `subagent_type`, and do NOT pass a `model` override.** Tier and effort are
pinned in the agent definitions under `.claude/agents/`, which is the single source of truth for
both. The split is:

| sub-agent | `subagent_type` | skill it invokes | model | effort |
|---|---|---|---|---|
| 1 | `pr-review-finder-indepth` | `in-depth-review` | `sonnet` | `medium` |
| 2 | `pr-review-finder-indepth` | `in-depth-review` | `sonnet` | `medium` |
| 3 | `pr-review-finder-indepth-deep` | `in-depth-review` | **`opus`** | `medium` |
| 4 | `pr-review-finder-ghstyle` | `gh-style-review` | `sonnet` | `medium` |

The model and effort columns are **documentation of what those files declare**, not a second
control point. Change the agent file, not this table.

**If a `subagent_type` above does not resolve** (the agent files have not been synced to
`~/.claude/agents/` yet, or were renamed), do not abort the review. Fall back to a plain Agent
call with the matching `model:` from the table, and state plainly in the Step 4 report that
effort could not be pinned and therefore inherited the session value. A review that runs at the
wrong effort is recoverable; a review that fails to launch is not.

**Why `subagent_type` rather than an inline `model:`.** The Agent tool has no `effort` parameter,
so effort cannot be pinned at the call site the way `model` can. Left unset it **inherits the
session effort**, which means review cost silently tracks whatever the user last typed into
`/effort`. A session at `xhigh` ran this entire fan-out at `xhigh`. Agent definitions are the
only place both knobs can be fixed together. Effort is `medium` on the finders because that is
the level the whole cost-efficiency study was run at, so it is the only level whose recall numbers
are actually evidenced here. Measured recall was flat from `low` through `xhigh` while latency
scaled ~3.9x, so `low` is a plausible further saving. Trial it before adopting it.

Sub-agent 3 is the mixed-tier finder described in the overview. It is the only finder on Opus.
Note that `in-depth-review` and `gh-style-review` also pin their own *internal* tiers (their
inner reviewers -> Sonnet, scorers -> Haiku); the agent definition governs the wrapper sub-agent.

### Sub-agents 1-3 prompt (in-depth-review)

Identical prompt for all three; only the `subagent_type` differs. See
[PROMPT-FINDER.md](PROMPT-FINDER.md) for the exact prompt each receives.

### Sub-agent 4 prompt (gh-style-review)

See [PROMPT-GH-STYLE.md](PROMPT-GH-STYLE.md) for the exact prompt this sub-agent receives.

### Why each sub-agent uses `--raw`

Both `in-depth-review` and `gh-style-review` default to discarding anything `< 70`. This
orchestrator's threshold is **60** (lower than each sub-skill's default because the 4x
cross-instance triangulation — three from the specialized-role structure, one from the
GitHub-Action mirror — raises confidence in 60-69 findings). `--raw` makes the sub-agents
return all scored findings; we apply the 60 cutoff in Step 2 after merging.

## Step 2: Merge and deduplicate (findings)

Account for all four sub-agents, pool their findings, deduplicate across sources, merge each
duplicate group, filter to `confidence >= 60` (dropping any unverified-citation or unscored
finding regardless of score), and order the survivors. Follow
[AGGREGATING.md](AGGREGATING.md) exactly — three rules from it matter enough to repeat here:
results arrive asynchronously in each sub-agent's own final text (never in the Agent tool's
launch result), a missing reviewer is reported immediately rather than retried (this skill is
one-shot, unlike `review-and-fix`), and a missing reviewer's findings are never fabricated or
inferred. Snapshot the full merged set as `merged_all` before filtering — Step 2.7's dedup
needs it.

## Step 2.5: Aggregate Discussion Context (from the gh-style sub-agent only)

The single `gh-style-review` sub-agent returns a `discussion_context` block with `resolved` and
`unaddressed` arrays. `in-depth-review` returns no such block, so it contributes nothing here.
Only the **unaddressed** concerns are rendered (the "Addressed by this PR" section was dropped
as noise, because listing what the diff already fixed is not actionable). The `resolved` entries are
not rendered at all.

Because exactly ONE instance produces this block, there is no cross-instance dedup, no
disagreement detection, and no agreement count. This is the deliberate trade of the asymmetric
split. Discussion Context is now a single-reviewer judgement. Treat its entries as leads
grounded in a real comment URL, not as triangulated findings.

1. **Take the instance's `unaddressed` array** as `unaddressed_pool` directly. Each entry
   carries `quote`, `author`, `url`, and `gap`.

2. **Deduplicate by `url`** within that array (the GitHub comment URL is the canonical identity
   of a discussion item). A single instance can still list the same comment twice; collapse those.

3. **Retain all entries.** There's no confidence threshold here because every entry is
   grounded in a real human comment URL. Order `unaddressed_pool` by the comment's `created_at`
   ascending (oldest unresolved concern first).

4. **If `unaddressed_pool` is empty after dedup, the "Still unaddressed" section is skipped.**
   If it is empty AND the findings list from Step 2 is also empty, Step 3 still applies. No
   review is posted.

## Step 2.7: Approach review stage (one pair, run once)

A single **approach reviewer** and a single **more-nuanced counterpart agent** debate the
issues the approach reviewer raises. This stage judges one thing only: is this the right way to
build it — not bugs, edge cases, security, or error handling, which the four reviewers already
cover. Only issues the two *converge* on (both say "needs a code change," within a 3-round cap)
survive, and only if no other reviewer already raised the same thing in `merged_all` with
confidence > 50. Survivors are tagged `source = "approach"` and folded into the Step 2 findings
pool, bypassing the >=60 filter — agreement is the gate for this stage, not the score. A missing
proposer or judge means the stage did NOT run (report `approach stage did not run: <reason>`);
never treat silence as "no approach findings" or post an unjudged proposal.

The proposer runs on Sonnet, the judge on Opus at effort `high` (the debate is genuine judgment;
proposing is recall, and measured recall was identical across tiers). See [APPROACH.md](APPROACH.md)
for the full debate protocol (up to 3 rounds, a worked example), the dedup rule against
`merged_all`, the tagging rule, and both sub-agents' exact prompts.

## Step 3: Post the review (only if there is something worth posting)

**If the findings list AND `unaddressed_pool` are BOTH EMPTY, do NOT post anything to GitHub.**
The findings list here is the Step 2 filtered findings PLUS any approach survivors folded in
by Step 2.7c, so a single agreed approach finding is on its own enough to post a review.
When both pools are empty, skip to Step 3e (which deletes the in-progress comment if one was
posted) and then Step 4, and tell the user in chat that the PR looks clean. No review is
posted and no PR state changes. Deleting the opt-in in-progress comment leaves the PR with
nothing on it, so silence on GitHub stays the success signal. A
sub-threshold ticket note on its own is NOT enough to post. If there are no findings and
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
  will reject the inline comment, so demote to GLOBAL.
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

Build `tickets_examined` = union by `id` of the `tickets_examined` arrays returned by the three
in-depth-review sub-agents (each entry has `id` and `status` ∈ {`ok`, `gaps`, `unread`}). For
each `id`, `status` is `gaps` if any instance reported gaps, else `unread` if any reported
unread, else `ok`.

From that, derive ONLY the notes worth showing a human. Never emit a roll-call of what passed:

- **Above-threshold ticket gaps** are already Code review findings (category `ticket`, prefixed
  `[<ticket_id>]`). Do NOT repeat them in the Tickets examined section.
- **Sub-threshold ticket observations** = the `sub_threshold_ticket_notes` retained in Step 2
  (ticket findings that scored < 60) — an AC a reviewer flagged that did not clear the >=60 bar.
- **Unread/unverified tickets** = any `id` whose aggregated `status` is `unread` (no instance
  could read it).

Set `ticket_notes_present` = true if there is at least one sub-threshold observation OR at
least one unread ticket. Tickets with `status: ok` and no sub-threshold note contribute
nothing and are never named.

### Step 3b/3c: Build the global body and each inline comment

The global body must start with the disclosure line verbatim, never repeats internal mechanics
(confidence numbers, agreement counts, source skills, or the convention-file name a finding
cites), and never emits a section with nothing in it. See [POST-BODY.md](POST-BODY.md) for the
exact template, the confidence-label mapping, the rule-attribution rule, and the formatting
rules (pluralization, the bold-marker rule, section order, ticket/approach tagging) that both
the global body and each inline comment follow.

### Step 3c.7: Human-review gate (before posting)

Before any GitHub write, dump every finding to a local file, pause, and post only what the
user approves. **This is a hard gate.** Do not post to GitHub until the user says to.

This step runs **only when Step 3 has something to post** (at least one finding or at least
one unaddressed concern). The clean-PR path is unchanged: when there is nothing to post, skip
straight to Step 3e as before. No file is written and there is no pause.

Write `/tmp/claude/pr-review-<PR>-comments.md`, tell the user the path plus a one-line tally,
and wait for their response (approve all / keep a subset / decline all). See
[POST-BODY.md](POST-BODY.md)'s "Step 3c.7 file format" section for the exact block format, the
divider, and how each response is handled.

This gate makes the skill interactive by design. In a headless run there is no one to approve,
so the run stops at this file with nothing posted. That is the intended safe default.

### Step 3d: Post the review (one API call)

Reached only after the Step 3c.7 gate has approved (all findings, or the kept subset). Fetches
the PR head SHA, builds the single JSON payload (`event=COMMENT`, never `APPROVE` or
`REQUEST_CHANGES`), posts it as one atomic write, and demotes any inline comment GitHub rejects
with a 422 to GLOBAL rather than dropping it. See [POST-BODY.md](POST-BODY.md) for the exact
commands and payload. This is the only PR review this skill writes — see **Constraints** for
the exact posting conditions.

## Step 3e: Delete the in-progress comment

If `<ANNOUNCE_COMMENT_ID>` is set (an in-progress comment was posted in Step 0.7), delete it
now, on every path, whether or not a review was posted. The clean-PR path routes here too, so
do not assume Step 3d ran or that `OWNER_REPO` is set. Prefer the GitHub MCP delete-issue-comment
tool (pass owner/repo plus the comment id). Fall back to
`gh api -X DELETE "/repos/<owner>/<repo>/issues/comments/<ANNOUNCE_COMMENT_ID>"`, resolving
owner/repo with `gh repo view --json owner,name` if not already known. If deletion fails, note
it in the Step 4 report and continue. The review is already delivered, so a failed cleanup
never fails the run. If `<ANNOUNCE_COMMENT_ID>` is empty, do nothing here.

## Step 4: Final report (to the user, not GitHub)

Summarize to the user in chat, whether or not a review was posted. See
[FINAL-REPORT.md](FINAL-REPORT.md) for exactly what to cover on the posted-review path and the
clean-PR path — both need the pre-merge finding counts by source/tier, the approach stage's
outcome, `skipped_reason` vs. silent-miss attribution, and the `complete`/`partial` Coverage
line, which must never be omitted.

## Constraints

- **At most one PR review per run.** The orchestrator posts a single `POST .../pulls/<PR>/reviews`
  call (which atomically carries the global body AND all inline comments), only when there is
  at least one surviving finding >= 60, at least one surviving approach finding (Step 2.7), OR
  at least one unaddressed prior concern. When all of those are empty, the orchestrator posts
  no review (a sub-threshold ticket note alone is not enough to post). The only other writes the
  orchestrator may make are the opt-in in-progress comment (Step 0.7) and its later deletion
  (Step 3e). Beyond those: no `gh pr edit`, no other comments, no second review.
- **Posting is gated on human approval (Step 3c.7).** Whenever there is something to post, the
  assembled findings are first written to `/tmp/claude/pr-review-<PR>-comments.md`, one block
  per finding split by `=============` dividers, and the run pauses. Only the findings the user
  approves are posted; a declined gate posts nothing. This makes the skill interactive. A
  headless run stops at the file with nothing posted, which is the intended safe default. The
  clean-PR path (nothing to post) does not write a file or pause.
- **The review event MUST be `COMMENT`.** Never `APPROVE`, never `REQUEST_CHANGES`. This skill
  comments; it does not gate merges.
- **Sub-agents are read-only with respect to GitHub.** They invoke `in-depth-review` or
  `gh-style-review` (both write-free) and just relay scored findings back. If any sub-agent
  appears about to issue a GitHub write, abort the entire orchestration and surface to the
  user. Do not proceed to post the merged review, since the inner skill could have posted
  from a sub-agent. **The approach and nuanced agents (Step 2.7) are read-only too,** with the same
  forbidden-write list; abort the orchestration if either appears about to write to GitHub.
- **Four parallel sub-agents (2 x in-depth-review on Sonnet + 1 x in-depth-review on Opus +
  1 x gh-style-review).** Launch all four in a single message with concurrent Agent tool calls.
  Do not fall back to fewer sub-agents for "speed"; the cross-source triangulation is the point.
  Do not use only one source. Both prompt structures contribute, and dropping gh-style also
  drops Discussion Context. The pool is deliberately asymmetric in BOTH source and tier. Do not
  "balance" the sources back to 2 + 2, and do not make the tiers uniform in either direction
  (see the overview).
- **Comment-punctuation findings are in scope but low priority.** The sub-skills flag comments
  the diff adds or edits that join clauses with ` - ` (space-hyphen-space) or a sentence-splitting
  `:`, per AGENTS.md. These are `suggestion`-severity: keep them if they survive the threshold,
  but never let them displace correctness findings in the posted review.
- **Model and effort policy (cost): pinned in agent definitions, never inherited.** Every agent
  this skill spawns is addressed by `subagent_type`, and its tier and effort come from its file
  in `.claude/agents/`. Pass no `model` override from this skill. The pool is **mixed-tier by
  design**: sub-agents 1, 2, and 4 on Sonnet, sub-agent 3 on Opus as the subtle-bug catcher, the
  approach proposer on Sonnet (its recall measured identical to Opus), the nuanced judge on Opus
  at effort `high` (judging is judgment; proposing is recall). Finders sit at effort `medium`.
  Their inner reviewers/scorers self-tier (Sonnet/Haiku) per those skills. **Never let any of
  these inherit the session model or the session effort.** Inheritance is what let a `/effort
  xhigh` session silently run the whole fan-out at `xhigh`. What protects quality is the >=60
  triangulation, the converge stage, and exactly one Opus finder, not a bigger model or more
  thinking on every agent.
- **Threshold is 60.** Do not raise or lower it on the fly. This applies to the four reviewers'
  findings only. Approach findings (Step 2.7) do NOT use this threshold. They are gated on
  the two agents *converging* on "needs a code change," not on a confidence score.
- **Approach stage: one pair, agreement is the gate, split tiers.** Run exactly one approach
  reviewer (proposer, **Sonnet**) + one nuanced agent (judge, **Opus**), once (not 3x). The
  approach reviewer judges design, architecture, and
  code placement only, not bugs, and explores the wider codebase to do so. A finding posts
  only if (a) the pair converges to both "needs change" within the 3-round cap AND (b) it does
  not duplicate a `merged_all` finding with confidence > 50. Approach findings carry
  `source = "approach"` and the `[approach, …]` tag, bypass the >=60 filter, and otherwise post
  through the same single review as everything else. The approach stage runs **regardless of
  `--skip-ticket`.** It is independent of the ticket logic.
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
