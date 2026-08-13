---
name: review-and-fix
description: >
  Iteratively reviews recent code changes (a PR if one exists for the current branch,
  otherwise the branch's commit range) and fixes what the reviews find. Runs `in-depth-review`
  and `gh-style-review` sub-agents in parallel each iteration, merges and deduplicates their
  findings, and applies fixes one commit at a time until a pass finds nothing further or an
  iteration commits nothing. No GitHub writes. Produces a final summary report.
  Use this skill when the user asks to "review and fix", "review my changes", "clean up my
  code", "improve my recent commits", or similar requests to audit and improve uncommitted or
  branch-local changes.
---

# Review and Fix

This skill wraps `in-depth-review` AND `gh-style-review` with an iterate-and-fix loop. See
"Process Overview" below for the per-iteration flow.

The triangulation lives **here**, not inside the sub-skills. Each `in-depth-review` pass
is itself a multi-role review (9 to 12 roles, or 8 to 11 with `--skip-ticket`, and three of those
are gated on what the diff contains: data-layer, security, and TypeScript). Each
`gh-style-review` pass is the @claude review GitHub Action prompt with full PR context (when in
PR mode).

## GitHub side-effect policy

**This skill never writes to GitHub.** Neither does `in-depth-review` or `gh-style-review`.
The skill never invokes `gh pr comment`, `gh pr review`, `gh pr edit`, or any write command.
Read-only `gh` calls inside the sub-skills (in-depth-review's prior-PR-comment role, plus
gh-style-review's PR conversation/review-thread fetches) are fine.

## GitHub access (GitHub MCP with `gh` fallback)

This skill's only direct GitHub call is `gh pr view` (PR detection, Step 0.5), written as a `gh`
command for reference. **Prefer the GitHub MCP server when it is connected; use the `gh` command
only as a fallback when no GitHub MCP is available (or its tools don't cover the call).** Discover its tools
with `ToolSearch "github pull request"` and use a get-pull-request / list-pull-requests tool to
find the current branch's open PR. If neither a GitHub MCP nor `gh` is available, treat it as
"no PR" (`<HAS_PR> = false`) and run the sub-agents in branch mode. The loop still works. The
reviewer sub-skills (`in-depth-review`, `gh-style-review`) carry their own identical fallback
for the reads they do. Local `git` calls need no `gh`.

## Process Overview

1. Determine the target: PR if one exists for the current branch, else commit range
   (current branch vs. default branch)
2. Launch the iteration's **active reviewer sub-agents** in parallel (iteration 1: all 3 =
   2 x in-depth-review + 1 x gh-style-review; later iterations: possibly a subset, see Step 3's
   adaptive-rerun rule) against that target
3. Merge + deduplicate findings across the active instances into one flat pool
4. (PR mode only) Aggregate Discussion Context from the active gh-style instance
5. Fix each unique finding, asking for clarification on ambiguous items, committing each fix,
   and recording whether each committed fix changed logic and which reviewer it came from
6. Decide the next iteration's active set (full rerun / pruned / stop) and repeat from step 2
   until the findings list is empty with every launched reviewer kind reported, none
   `unavailable`, and the unioned `roles_missing` empty (row 1, clean), or the findings list is
   empty with a kind `unavailable` or the unioned `roles_missing` non-empty (row 1c, which stops
   with partial coverage), or an iteration commits nothing
7. Emit the per-iteration summary before the loop returns to step 2 or falls through to the
   report
8. Deliver a final summary report

## Step 0: Setup

1. Confirm the working tree is clean (`git status --porcelain`). If there are uncommitted
   changes, warn the user and ask whether to stash first or include them in the review.
2. Determine the default branch:
   ```
   git remote show origin | grep 'HEAD branch' | awk '{print $NF}'
   ```
   Fall back to `main` if unavailable.
3. Count how many commits the current branch is ahead of the default branch:
   ```
   git rev-list --count origin/<default-branch>..HEAD
   ```
   If 0, inform the user there are no new commits to review and stop.
4. Store the commit range as `<RANGE>` = `origin/<default-branch>..HEAD`.
5. **Detect whether an open PR exists for the current branch** so we can pass PR mode to the
   sub-agents when available (and unlock gh-style-review's Discussion Context):
   ```
   gh pr view --json number,state,isDraft,url 2>/dev/null
   ```
   - Exit non-zero or `state != "OPEN"` -> set `<HAS_PR> = false`. Sub-agents will receive
     `<RANGE>` and run in branch mode.
   - Exit zero and `state == "OPEN"` -> set `<HAS_PR> = true`, save `<PR>` and `<PR_URL>`.
     Sub-agents will receive `<PR>` and run in PR mode. Draft PRs are accepted.
   - Prefer the GitHub MCP to detect the PR (see **GitHub access**); fall back to `gh` only when
     no MCP is connected. If neither a GitHub MCP nor `gh` is available, set `<HAS_PR> = false` and continue.
6. Define `<TARGET_ARG>` for use in the sub-agent prompts:
   - If `<HAS_PR>` -> `<TARGET_ARG> = <PR>`
   - Else -> `<TARGET_ARG> = <RANGE>`
7. If the invocation included `--skip-ticket`, set `<SKIP_TICKET> = true` (default `false`).
   When `true`, every in-depth-review sub-agent is invoked with `--skip-ticket` so Role #10
   never runs. When `false`, both in-depth-review instances run Role #10 (two ticket
   reviewers). `gh-style-review` is unaffected.
8. **Jira-tooling preflight** (skip this step entirely if `<SKIP_TICKET>` is true). Before
   the first review iteration, confirm a Jira reader is available AND authenticated:
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
     (c) abort.
   Do not start iteration 1 until this is resolved. If a re-check after choice (a) still
   fails, present the three choices again rather than proceeding.

9. **Initialize the active reviewer set** used by Step 1:
   - `<ACTIVE_ROLES>` = all in-depth-review roles `1..12` (drop `10` when `<SKIP_TICKET>` is
     true). This is the set of roles the in-depth-review instances will run.
   - `<ACTIVE_GH_STYLE>` = true.
   Step 3 recomputes both before each subsequent iteration. Iteration 1 always runs the full
   set.

## Step 1: Review — Active Parallel Sub-Agents (up to 2 in-depth-review + 1 gh-style-review)

Launch the iteration's **active** reviewers only:
- If `<ACTIVE_ROLES>` is non-empty, launch **2 in-depth-review** instances, each passed
  `--roles <ACTIVE_ROLES>` (comma-separated role numbers). When `<ACTIVE_ROLES>` is the full
  default set, you may omit the flag (identical result). Passing it is fine either way.
- If `<ACTIVE_GH_STYLE>` is true, launch **1 gh-style-review** instance.
- Keep the 2x multiplicity for in-depth-review whenever it is active (triangulation on the
  active set). gh-style-review is **1x by design.** See the asymmetry note below. The count of
  live sub-agents is `2 x (in-depth active?) + 1 x (gh-style active?)`.

**Why gh-style-review is 1x and in-depth is 2x.** Measured on fixtures with planted issues,
gh-style's findings were a strict SUBSET of in-depth's on every fixture. It surfaced nothing
in-depth missed, and skipped test-coverage findings entirely. A second gh-style instance buys no
incremental finding recall. It stays at one rather than zero because its real contribution is
Discussion Context, which in-depth cannot produce at all. This matters more here than in
`pr-review`, which fans out once. This skill re-runs its fan-out on every iteration and nothing
caps the iteration count, so a redundant instance is paid again on every pass. Do not raise
gh-style back to parity, and do not drop it to zero.

Announce at iteration start, reflecting the ACTUAL active set, e.g.:

> Iter N: launching 2 in-depth-review passes (roles: AGENTS.md, comment guidance); gh-style-review skipped.
> Target: PR #<PR> [draft]  <-  or  Target: branch range <RANGE>

or, for a full iteration:

> Iter N: launching 3 reviewer passes in parallel (2 x in-depth-review [all roles], 1 x gh-style-review).
> Target: PR #<PR> [draft]  <-  or  Target: branch range <RANGE>

Spawn the active sub-agents **in a single message** (concurrent tool-use blocks). Sequential
launches defeat the purpose. Never serialize.

**Spawn each one by `subagent_type`, and pass no `model` override:**

| reviewer | `subagent_type` | model | effort |
|---|---|---|---|
| in-depth-review | `pr-review-finder-indepth` | `sonnet` | `medium` |
| gh-style-review | `pr-review-finder-ghstyle` | `sonnet` | `medium` |

Tier and effort are pinned in those files under `.claude/agents/`; the columns above are
documentation of what the files declare, not a second control point. Both agents are shared with
the `pr-review` skill.

**If a `subagent_type` above does not resolve** (the agent files have not been synced to
`~/.claude/agents/` yet, or were renamed), do not abort the iteration. Fall back to a plain Agent
call with `model: sonnet`, and note in the per-iteration summary that effort could not be pinned
and therefore inherited the session value.

**Why this matters more here than anywhere else.** The Agent tool has no `effort` parameter, so
unset effort **inherits the session effort**. This skill runs its fan-out in a *loop* with no cap
on the iteration count, so an inherited `xhigh` is multiplied by however many iterations the run
takes. Pinning it in the agent definition is the only way to fix both knobs together. Effort is
`medium` because that is the level the cost-efficiency study measured; recall was flat from `low`
through `xhigh` while latency scaled ~3.9x, so `low` is a plausible further saving worth trialling.

`in-depth-review` / `gh-style-review` still pin their own internal tiers (their inner reviewers ->
Sonnet, scorers -> Haiku). The fix step (Step 2) stays on the **session model.** Applying and
committing code is where the strong model earns its cost. The recall fan-out is not.

### In-depth-review sub-agent prompt

Launched only when `<ACTIVE_ROLES>` is non-empty. See [PROMPT-IN-DEPTH.md](PROMPT-IN-DEPTH.md)
for the exact prompt each of the (up to) two in-depth-review sub-agents receives.

### gh-style-review sub-agent prompt

Launched only when `<ACTIVE_GH_STYLE>` is true. gh-style-review has no roles, so it is rerun as
a whole unit (or skipped entirely). See [PROMPT-GH-STYLE.md](PROMPT-GH-STYLE.md) for the exact
prompt the sub-agent receives.

### Aggregating across the active instances

Collect every active sub-agent's result, pool them, deduplicate, and merge into one flat list
filtered to `confidence >= 50`. Follow [AGGREGATING.md](AGGREGATING.md) exactly — three rules
from it matter enough to repeat here: results arrive asynchronously in each sub-agent's own
final text (never in the Agent tool's launch result), a reviewer that falls short is retried once
per run before being marked `unavailable`, and a missing reviewer's findings are never fabricated
or inferred. AGGREGATING.md also covers aggregating `tickets_examined`.

## Step 1.5: Aggregate Discussion Context (PR mode only)

If `<HAS_PR>` is false, **skip this step entirely.** gh-style-review returned empty
`discussion_context.resolved` and `discussion_context.unaddressed` arrays in branch mode.

If `<HAS_PR>` is true (and `<ACTIVE_GH_STYLE>` is true — when gh-style-review was not active
this iteration there is no new Discussion Context, so carry forward the previous snapshot):

Because exactly ONE instance produces this block, there is no cross-instance dedup, no
disagreement detection, and no agreement count. That is the deliberate trade of the 1x gh-style
split. Discussion Context is a single-reviewer judgement. Treat entries as leads grounded in a
real comment URL, not as triangulated findings.

1. **Take the instance's blocks directly** as `resolved_pool` and `unaddressed_pool`.
   (in-depth-review has no equivalent and contributes nothing here.)
2. **Deduplicate by `url`** within each list (the GitHub comment URL is canonical). A single
   instance can still list the same comment twice; collapse those. If a URL somehow appears in
   both lists, keep it in `unaddressed_pool` (be conservative and surface anything the reviewer is
   uncertain about).
3. **Retain all entries.** There is no confidence filter here. Every entry is grounded in a real
   human comment URL.

This block does NOT trigger fix actions in Step 2 (resolved items are already addressed;
unaddressed items will be picked up by the next iteration's findings if they're actionable
in the current diff). It surfaces in the per-iteration summary and the Final Report so the
user can see the diff's effect on the PR's discussion thread evolving across iterations.

## Step 2: Fix

At the start of the iteration's fix phase, reset four per-iteration accumulators. The first
three are what Step 3 reads to decide the next active set. The fourth is what Step 3's commit
table is built from:
- `any_commit` = false — set true the moment any fix is committed.
- `any_logic_change` = false — set true if any committed fix changes program logic.
- `productive_reviewers` = empty — the reviewers whose findings were fixed AND committed this
  iteration (in-depth-review role numbers via each finding's `category`, and/or the
  `gh-style-review` unit). This is the pruned set a non-logic-only iteration reruns.
- `iteration_commits` = empty — one `(short sha, finding title)` pair per committed fix, in
  commit order. Step 3's commit table is this list.

Process each finding from the ordered work list (Step 1) one at a time. Skip any
`ticket`-category finding already recorded in `resolved_ticket_findings` (deferred or
dismissed in a prior iteration). Do not re-prompt. Deferred ones are carried to the Final
Report.

### For each finding:

1. **Read the relevant file(s)** to understand the context.

2. **Assess confidence:**
   - If the fix is clear and unambiguous -> implement it directly.
   - If the fix is ambiguous or has multiple valid approaches -> use `ask_user` to present the
     options and wait for a decision before proceeding.
   - If the finding's `category` is `ticket` -> ALWAYS use `ask_user`, regardless of how
     clear the fix looks. Ticket gaps are intent decisions, not mechanical fixes. Present:
     (1) the ticket ID and the requirement it states, (2) the gap — what the diff does vs.
     what the ticket asks. Offer three choices:
       (a) implement the missing intent (then proceed to implement + commit as usual),
       (b) defer — surface only, make no change this run,
       (c) dismiss — the gap is a false positive or out of scope.
     Only implement and commit when the user picks (a). Never auto-commit a ticket finding.
     When the user picks (b) or (c), record the finding in `resolved_ticket_findings` (keyed
     by `ticket_id` + title) so later iterations do not re-prompt for it.

3. **Implement the fix** following all project coding standards:
   - Read the relevant `AGENTS.md` (root and sub-project) for mandatory conventions.
   - Run the project's linter/formatter if one exists and fix any violations it reports.
   - Run the project's tests (`pnpm run test:unit` for the web sub-project, or the equivalent
     for the relevant sub-project) to confirm no regressions.
   - **Do not commit if lint or tests fail.** Fix the failures first or escalate to the user.

4. **Commit the fix:**

   ```
   git add -A
   git commit -m "<type>: <short description of what was fixed>

   <optional body explaining why>
   ```

   Use conventional commit types: defined in the `.github/semantic.yml` file (e.g., `fix`,
   `feat`, `refactor`, `docs`, etc.) and ensure the message is clear and concise. If the file
   is missing try to figure out what the correct type should be.

5. **Record what this commit was**, for Step 3's next-active-set decision and its commit table:
   - Set `any_commit = true`.
   - Append the commit's short sha plus the finding's `title` to `iteration_commits`. Take the
     title verbatim from the merged finding. It is the `Fix` cell in both tables.
   - Add the fixed finding's reviewer(s) to `productive_reviewers`: map its `category`(ies)
     to in-depth role number(s) via the table in in-depth-review's Step 1, and add the
     `gh-style-review` unit if the finding came (also) from that source. A finding merged
     across both sources adds both.
   - **Classify the commit as a logic change (diff-based).** Inspect the commit's own diff
     (`git show --format= <sha>`). It is **non-logic** only if every changed hunk is confined
     to comments, docstrings / block comments, blank-line or whitespace-only edits, or
     pure-documentation files (`*.md`, `docs/**`). Any change to executable code — including a
     string/number literal that logic reads, a moved statement, an import, config that alters
     behavior — is a **logic change**: set `any_logic_change = true`. **When in doubt, treat
     it as a logic change** (the cost is only rerunning more reviewers next iteration, never a
     missed defect).

6. After the bookkeeping, move to the next finding.

## Step 3: Loop Control

Re-running all three passes every iteration is wasteful when the iteration only touched
comments or formatting, so the next iteration's active set depends on what the last one
actually committed:

- **A committed fix changed program logic** -> the next iteration reruns the **full** set. A
  logic change can introduce a bug, a security hole, a broken test, etc. in any domain, so
  every reviewer must look again.
- **An iteration committed fixes but none changed logic** (comments, docstrings, formatting,
  doc files only) -> the next iteration reruns **only the reviewers whose findings were fixed
  this iteration** (the productive set). Logic reviewers cannot surface anything new because
  the logic they last cleared is unchanged. in-depth-review roles are rerun individually via
  its `--roles` flag; `gh-style-review` is one indivisible unit (rerun whole, or not at all).
- **An iteration committed nothing** (every finding was deferred or dismissed) -> **stop**.
  The diff is unchanged, so a rerun would resurface the identical findings.

After processing all of the iteration's findings, evaluate these conditions **in order.**
The first that matches wins:

| # | Condition                                                                  | Action                                                        |
| - | -------------------------------------------------------------------------- | ------------------------------------------------------------- |
| 1 | Findings list empty, every reviewer **launched this iteration** reported, **`reviewer_unavailable` is empty**, **and the unioned `roles_missing` is empty** | **Stop** — clean. Proceed to Final Report |
| 1b | Findings list empty BUT a reviewer launched this iteration is missing and still has retry budget | **Not clean.** Relaunch it next iteration; go to Step 1 |
| 1c | Findings list empty, every reviewer launched this iteration either reported or is `unavailable`, **and EITHER `reviewer_unavailable` is non-empty OR the unioned `roles_missing` is non-empty** | **Stop** — coverage is `partial`, not clean. Use the incomplete-coverage outcome. |
| 2 | `any_commit == false` (findings existed but nothing was committed)         | **Stop** — proceed to Final Report                            |
| 4 | `any_logic_change == true`                                                 | **Full rerun**: set active set to ALL reviewers; go to Step 1 |
| 5 | Otherwise (committed, but no logic change)                                 | **Pruned rerun**: set active set to `productive_reviewers`; go to Step 1 |

A role-level shortfall is deliberately NOT retried. It blocks the clean exit and surfaces in Coverage
as `partial`, which is why row 1 now requires an empty unioned `roles_missing`.

**There is no iteration cap, and the number column is a label column, not an index.** Rows 1, 1c,
and 2 are the only stops. Rows 1b, 4, and 5 are the only rows that go back to Step 1. Row 1b is
bounded at one retry per reviewer kind per run, and rows 4 and 5 both require `any_commit == true`,
because row 2 stops the run the moment an iteration commits nothing. So the loop continues only
while every single iteration commits at least one fix. That is real progress on almost every run.
It is not a termination proof. One iteration's fixes can introduce a defect the next iteration then
finds, and that cycle can sustain itself. A run that is going nowhere is stopped by the user
interrupting it, which is why every iteration emits a per-iteration summary. `iteration` is a label
for the announce line, that summary, and the Final Report header. No stop rule reads it. Do not add
a cap row, an iteration count of any size, a periodic check-in, or an oscillation detector, and do
not renumber the rows that remain.

Computing the next active set, for every row that goes back to Step 1 (1b, 4, and 5):
- **Row 1b (retry):** the active set is the short kind ONLY. `<ACTIVE_ROLES>` = the roles this
  iteration ran if the `in-depth-review` kind fell short, otherwise empty. `<ACTIVE_GH_STYLE>` =
  true iff the `gh-style-review` kind fell short. The diff is unchanged since the other reviewers
  cleared it, so relaunching them buys nothing. The short kind relaunches at full multiplicity.
- **Row 4 (full):** `<ACTIVE_ROLES>` = all roles `1..12` (drop `10` when `<SKIP_TICKET>`),
  `<ACTIVE_GH_STYLE>` = true.
- **Row 5 (pruned):** `<ACTIVE_ROLES>` = the in-depth role numbers in `productive_reviewers`;
  `<ACTIVE_GH_STYLE>` = true iff the `gh-style-review` unit is in `productive_reviewers`. (At
  least one is non-empty here, because `any_commit` was true and every committed fix
  attributes to a reviewer.)

**First union in any kind still owed a retry, whichever row fired.** Before launching, add back every
kind that fell short this iteration and still has retry budget, even when the row that fired computed
a set without it. Row 5 is why this is needed. It builds its set from `productive_reviewers`, and a
kind that reported nothing contributed no findings, so it cannot be in that set. Without the union, a
kind that fell short while OTHER reviewers had findings that got fixed is dropped from the next
launch, never retried, and never reaches the second shortfall that would mark it `unavailable`. The
run could then stop on row 1 and report `complete` coverage with a reviewer silently gone. The union
gives a shortfall the same one-retry treatment on every path back to Step 1. That is what makes the
retry rule's "at most once per run" a real guarantee rather than "only when row 1b happened to fire".

**Then subtract `reviewer_unavailable`, whichever row fired.** This runs after the union, so a kind
that just used its last retry cannot be added back. Before launching, drop every kind in
`reviewer_unavailable` from the set the row computed. If the `gh-style-review` kind is unavailable,
`<ACTIVE_GH_STYLE>` = false no matter which row set it. If the `in-depth-review` kind is
unavailable, `<ACTIVE_ROLES>` is empty and no in-depth instance launches. This subtraction is the
only enforcement point for the table's "never relaunch it this run", so it runs on every path back
to Step 1, not just rows 4 and 5. If the subtraction empties the set entirely, launch nothing and
evaluate Step 3 as usual. Row 1c is then the row that fires, and the run stops with partial
coverage.

Track state explicitly. See [STATE.md](STATE.md) for every variable's exact definition,
including the per-run vs. per-iteration distinction that the retry and dedup rules above depend
on.

**A single clean active batch ends the loop.** The independent reviewers already triangulate
strongly; requiring two consecutive clean batches would just waste an iteration.

**Why pruning is safe without a final full sweep.** The loop only reaches a pruned rerun
(row 5) after an iteration whose fixes left program logic identical to what the logic
reviewers last cleared. The diff-based classifier in Step 2 flips `any_logic_change` and
forces a full rerun (row 4) the instant any fix touches logic. So a logic reviewer is only
ever skipped while the logic it already approved is unchanged, and by the time the loop stops on
row 1, row 1c, or row 2 every logic reviewer that was still available has validated the final
logic. An interrupted run carries no such guarantee, because an interrupt can land right after a
logic-change commit. A kind in `reviewer_unavailable` validated nothing, so a run that stops with
one reports `partial` coverage instead. A fix that sneaks a logic change past a "comment" finding
is caught by the diff classifier, not by re-running everyone.

Note: a non-empty `unaddressed_pool` in PR mode does NOT prevent the loop from terminating.
"Still unaddressed" items are surfaced for the user to consider, but the fix loop is
driven by the findings list. Those items will either appear as findings in subsequent
iterations (if they're actionable in the current diff) or persist until the user adds
work that addresses them.

### Worked examples

See [EXAMPLES.md](EXAMPLES.md) for a normal full -> pruned -> clean run, and a run where a
reviewer flakes (tracing the retry union, row 1b, and row 1c).

### Per-iteration summary

Every iteration ends by emitting one summary block to chat, closed by that iteration's commit
table, emitted last in Step 3 after the table has picked a row and the next active set is
computed. See [SUMMARY.md](SUMMARY.md) for exactly what to cover, in what order, and the commit
table format.

## Step 4: Final Report

Summarize the entire session in a report to the user. See [FINAL-REPORT.md](FINAL-REPORT.md)
for the exact template, how to select the Outcome line, and the per-section rules for Changes
Made, Remaining Issues, and Tickets examined.

## Constraints

- **No GitHub writes, ever.** Forbidden: `gh pr comment`, `gh pr review`, `gh pr edit`,
  `gh pr close`, `gh issue create`, `gh issue comment`, or invoking any skill that does
  (notably the upstream `code-review` skill, whose terminal step posts a PR comment).
  Permitted read-only `gh` calls (inside the sub-skills): `gh pr list`, `gh pr view`,
  `gh pr diff`, `gh search pulls`, `gh search issues`, plus the `gh api` reads
  gh-style-review uses to pull PR comments / review threads / prior reviews. If a sub-agent
  appears about to issue a write command, abort and surface the attempt to the user.
- **Iteration 1 runs the full set: 3 parallel sub-agents (2 x in-depth-review + 1 x
  gh-style-review).** Launch them in a single message with concurrent tool calls. Do not
  fall back to fewer instances "for speed" on the first pass; the cross-source triangulation
  is the point. Do not skip gh-style-review when in branch mode. It still contributes
  findings even with empty Discussion Context arrays. The split is deliberately asymmetric.
  Do not "balance" gh-style back to 2x (see Step 1).
- **Later iterations use the adaptive active set (Step 3), never an arbitrary reduction.** Step 3
  has exactly three reasons to run fewer reviewers. The pruned-rerun rule (a committed,
  no-logic-change iteration reruns `productive_reviewers`, then the retry union adds back any kind
  still owed one, so it is not `productive_reviewers` alone), the row-1b retry (which
  relaunches only the kind that fell short), and the `reviewer_unavailable` subtraction (which
  never relaunches an unavailable kind). Any logic change forces a full 3-agent rerun. Never drop
  a reviewer for "speed" outside these rules. Keep in-depth-review at 2x whenever it is
  active, and gh-style-review at 1x. An `unavailable` in-depth kind is not launched at all, so this
  multiplicity rule applies only while the kind is active (Step 3). There is no reduced-multiplicity
  path. A kind either relaunches in full or does not relaunch.
- **Each sub-skill is invoked WITH `--raw`.** We want every scored finding (0–100), not
  the sub-skill's default `< 70` filtered output. The orchestrator applies its own
  `confidence >= 50` threshold after cross-instance, cross-source dedup.
- **Comment-punctuation findings are in scope but low priority.** The sub-skills flag comments
  the diff adds or edits that join clauses with ` - ` (space-hyphen-space) or a sentence-splitting
  `:`, per AGENTS.md. These are `suggestion`-severity: fix them if the batch surfaces nothing
  more important, but never spend a fix iteration on punctuation while real bugs are outstanding.
- **Flat-pool merging.** Findings from in-depth-review and gh-style-review are merged into
  one pool, dedup'd by file+line+description regardless of source, then filtered. Source
  is preserved as a `sources` field on each finding (used as a tiebreaker only).
- **Discussion Context comes only from gh-style-review** and only in PR mode. Do not
  attempt to synthesize Discussion Context from in-depth-review findings. Do not fail an
  iteration because Discussion Context is empty. That's expected in branch mode.
- **Confidence threshold is 50.** Do not raise or lower it on the fly.
- **Model and effort policy (cost): pinned in agent definitions, never inherited.** Every active
  reviewer sub-agent is addressed by `subagent_type` (`pr-review-finder-indepth`,
  `pr-review-finder-ghstyle`), and its tier and effort come from its file in `.claude/agents/` —
  Sonnet at effort `medium`. Pass no `model` override. Their inner reviewers/scorers self-tier
  (Sonnet/Haiku) per those skills. **Never let the reviewer fan-out inherit the session model or
  the session effort.** The model may be Opus or a `[1m]` variant, and unset effort silently
  tracks whatever the user last set with `/effort`. Nothing caps the iteration count, so either
  mistake is paid again on every pass. The fix step (Step 2) — reading, editing, lint/test,
  committing — stays on the **session model**,
  since applying code is where a strong model is worth its cost.
- **One commit per fix** — never squash or amend.
- **Never commit broken code** — lint and tests must pass before committing.
- **Never push** — only local commits; the user decides when to push.
- **Ask before acting on ambiguous findings** — use `ask_user` for anything unclear.
- **Respect project AGENTS.md rules** — always read mandatory checklists before committing.
