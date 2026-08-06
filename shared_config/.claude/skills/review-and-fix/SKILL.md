---
name: review-and-fix
description: >
  Iteratively reviews recent code changes and fixes identified issues or implements
  improvements. The first iteration spawns THREE concurrent reviewer sub-agents — TWO
  `in-depth-review` and ONE `gh-style-review` — all invoked with `--raw` against the
  current branch (PR mode if an open PR exists for the branch, branch mode otherwise).
  Their raw scored findings are merged and deduplicated across all three instances into
  one flat pool, filtered to keep anything with confidence >= 50, then fixes are applied
  and committed one at a time. Later iterations adapt the reviewer set: if any committed
  fix changed program logic the next iteration reruns ALL reviewers, but if an iteration's
  fixes were non-logic only (comments/formatting/docs) it reruns just the reviewers whose
  findings were fixed — role-level for in-depth-review via its `--roles` flag. In PR mode,
  the gh-style instance also returns Discussion Context (which prior human comments the diff
  resolves vs. still leaves open); the orchestrator surfaces this in the per-iteration
  summary. The loop stops as soon as an iteration's active reviewers find nothing actionable,
  or an iteration commits nothing, or a reviewer kind is unavailable and the findings list is
  empty (which ends the run with partial coverage). No GitHub write commands are ever issued.
  Produces a final summary report.
  Use this skill when the user asks to "review and fix", "review my changes", "clean up my
  code", "improve my recent commits", or similar requests to audit and improve uncommitted or
  branch-local changes.
---

# Review and Fix

This skill wraps `in-depth-review` AND `gh-style-review` with an iterate-and-fix loop.
Each iteration:

1. Runs a set of `in-depth-review` + `gh-style-review` instances in parallel, each invoked
   with `--raw`. **Iteration 1 runs the full set** (2 in-depth + 1 gh-style). Later
   iterations may run a smaller set. See the adaptive-rerun rule below. The 2 in-depth
   instances also run Role #10 (ticket intent compliance) unless `--skip-ticket` was passed.
   The target is the branch's PR if one exists for the current branch, otherwise the branch's
   commit range.
2. Cross-instance dedups their findings into one flat pool (each instance already
   pre-dedupes internally; the triangulation across independent passes from two prompt
   structures catches the rest).
3. Applies the orchestrator's own **`confidence >= 50`** filter. It is more permissive than the
   sub-skills' default of 70 because the fix loop attempts moderately-confident findings too.
   The triangulation gives us enough cross-pass evidence that 50–69 findings are worth
   attempting.
4. (PR mode only) Aggregates Discussion Context from the gh-style instance and shows
   it in the per-iteration summary — useful when the loop is iterating on a PR that already
   has human reviewer comments.
5. Fixes each unique finding, one commit per fix, and classifies each committed fix as a
   logic change or not.
6. Loops until the findings list is empty with every launched reviewer kind reported, none
   `unavailable`, and the unioned `roles_missing` empty, or an iteration commits nothing, or the
   findings list is empty with a kind `unavailable` or the unioned `roles_missing` non-empty
   (which stops with partial coverage).

The triangulation lives **here**, not inside the sub-skills. Each `in-depth-review` pass
is itself a multi-role review (9 to 12 roles, or 8 to 11 with `--skip-ticket`, and three of those
are gated on what the diff contains: data-layer, security, and TypeScript). Each
`gh-style-review` pass is the
@claude review GitHub Action prompt with full PR context (when in PR mode).

## Adaptive rerun (why later iterations run fewer reviewers)

Re-running all three passes every iteration is wasteful when the iteration only touched
comments or formatting. The loop adapts based on what a completed iteration actually changed:

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

This is safe without any final full-sweep because the pruned state is only ever entered after
a no-logic-change iteration, and the moment any fix touches logic the loop re-escalates to a
full rerun. So every logic reviewer that was still available has validated the final logic by the
time the loop stops on row 1, row 1c, or row 2. An interrupted run carries no such guarantee. A kind
in `reviewer_unavailable` validated nothing, and Coverage reports such a run as `partial`. See the
worked example in Step 3.

**Flag:** pass `--skip-ticket` to disable ticket intent compliance (Role #10) in both
`in-depth-review` instances and skip the Jira-tooling preflight.

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
   2 x in-depth-review + 1 x gh-style-review; later iterations: possibly a subset, see the
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

### In-depth-review sub-agents prompt

Launched only when `<ACTIVE_ROLES>` is non-empty. Each of the (up to) two in-depth-review
sub-agents receives:

```
You are an in-depth-review sub-agent in a review-and-fix iteration.

Invoke the `in-depth-review` skill with the arguments: `<TARGET_ARG> --raw --roles <ACTIVE_ROLES>`.
Append ` --skip-ticket` when the orchestrator's `<SKIP_TICKET>` is true. `<ACTIVE_ROLES>`
is the comma-separated list of role numbers this iteration is rerunning.

- `<TARGET_ARG>` is either a PR number (PR mode) or a commit range like `origin/main..HEAD`
  (branch mode). in-depth-review auto-detects.
- `--raw` tells in-depth-review to skip its internal <70 confidence filter so we get every
  scored finding (0–100). The orchestrator applies its own >=50 threshold after merge.
- `--roles` restricts the review to the active roles (the productive ones from the previous
  iteration). On the first iteration this is all roles.

Return in-depth-review's structured JSON output to me unchanged, with two top-level
additions:
- `"sub_agent": N`
- `"source": "in-depth-review"`

Specifically forbidden:
- `gh pr comment` (any form)
- `gh pr review` (any form)
- `gh pr edit`, `gh pr close`, `gh pr merge`
- `gh issue create`, `gh issue comment`
- Any other command that writes to GitHub
If `in-depth-review`'s internal logic appears to be about to invoke one of these, abort and
return the abort reason to me instead of proceeding.
```

### gh-style-review sub-agent prompt

Launched only when `<ACTIVE_GH_STYLE>` is true. gh-style-review has no roles, so it is
rerun as a whole unit (or skipped entirely). The single sub-agent receives:

```
You are a gh-style-review sub-agent in a review-and-fix iteration.

Invoke the `gh-style-review` skill with the arguments: `<TARGET_ARG> --raw`

- `<TARGET_ARG>` is either a PR number (PR mode — unlocks Discussion Context) or a commit
  range (branch mode — Discussion Context arrays will be empty). gh-style-review auto-detects.
- `--raw` tells gh-style-review to skip its internal <70 confidence filter so we get every
  scored finding. The orchestrator applies its own >=50 threshold after merge.

Tell gh-style-review you are invoking it as a sub-agent. It must return the JSON shape
documented in its "If invoked as a sub-agent" section, NOT its terminal-formatted output.

Return that JSON to me unchanged, with two top-level additions:
- `"sub_agent": N`
- `"source": "gh-style-review"`

Specifically forbidden:
- `gh pr comment` (any form)
- `gh pr review` (any form)
- `gh pr edit`, `gh pr close`, `gh pr merge`
- `gh issue create`, `gh issue comment`
- Any other command that writes to GitHub
If `gh-style-review`'s internal logic appears to be about to invoke one of these, abort and
return the abort reason to me instead of proceeding.
```

### Aggregating across the active instances

**First, account for every sub-agent this iteration launched.** Classify each as *reported* or
*missing* (nothing returned, errored, unparseable output, or never notified before the give-up
bound). Then roll the per-instance verdict up
to the reviewer kind, since that is what the bookkeeping below is keyed on. Record every kind that
fell short in `reviewers_missing`, keep the per-instance detail for the per-iteration summary, and
union the `roles_missing` arrays the in-depth-review instances report. Never reason that running several
in-depth instances means every lens ran at least once. Measured: in one run two roles were silent in BOTH
instances, so the union of what the instances DID return covered neither. A lens that no instance reported on
is a hole, not a covered lens, and the union of findings can never be used to claim `complete`.
Results arrive in each sub-agent's own final text, on a later turn than the launch, never in the
Agent tool's launch result. Record every sub-agent's `agentId` at launch, then take turns and
harvest each `<task-notification>`, matching its `<task-id>` to a recorded `agentId`. Keep taking
turns until every sub-agent this iteration launched is accounted for, OR until THREE CONSECUTIVE
COLLECTING TURNS have brought zero new arrivals. A collecting turn is ONE substantive tool call
naming the sub-agent it checked on, so re-read the diff for a sub-agent you are still waiting on.
Three repeats of the same no-op are not three turns.
Do NOT start the zero-arrival counter until you have taken at least as many collecting turns as you
launched sub-agents, with a floor of five, whether or not anything has arrived. The three zero-arrival
turns are counted FRESH from the moment the counter arms, so turns taken before arming never count
toward them. Sub-agents take minutes, not seconds, so a counter armed at launch measures your own
polling speed rather than a failure. If you reach the bound and have not yet re-armed the counter,
re-arm it EXACTLY ONCE and keep collecting. After that, honor the bound. If you have no other work,
re-read the diff for a sub-agent you are still waiting on. Never end your turn with a recorded
`agentId` unaccounted for unless the give-up bound's single re-arm has already been used and you are
recording it in `reviewers_missing` on that same turn. If you stop while one is outstanding you will
not receive it at all. A notification that arrives after the bound is still
that sub-agent's report. Fold it into the pool and remove it from `reviewers_missing`. The
accounting is final only when you emit your report. A sub-agent that reported, but whose returned
text is empty, has reported nothing, whatever it may have sent over any other channel.

**Never fabricate a missing reviewer's findings**, and never fix on an inferred finding. Do not
write what a missing reviewer would have found, do not run its lens in the parent and attribute it,
and do not reuse a previous iteration's output for it. This skill commits code, so an invented
finding becomes an invented commit.

**Unavailability is keyed by reviewer KIND, not by instance.** Two kinds exist, the
`in-depth-review` unit and the `gh-style-review` unit. A kind **fell short** in an iteration when
fewer of its instances reported than were launched. A shortfall is retried by relaunching the kind
at FULL multiplicity, once per run. On a second shortfall the kind is marked `unavailable` for the
rest of the run. `unavailable` affects only future launches. It never discards a report already
received. `reviewers_missing`, `reviewer_unavailable`, and `reviewer_retries` are all keyed by kind.

**Why kind and not instance.** A deterministic `skipped_reason` is derived from the invocation
arguments, and both in-depth instances receive identical arguments, so both refuse identically.
Deterministic unavailability is inherently per-kind. The 2x in-depth multiplicity exists for
triangulation, not to supply two different lenses, so a shortfall is a coverage question about the
kind rather than the loss of a distinct lens. Keying by kind also keeps `<ACTIVE_ROLES>` plus
`<ACTIVE_GH_STYLE>` sufficient to express every launch decision. Keying by instance would need a
new launch variable and an exception to the multiplicity rule in the Constraints. The cost is that
one flaky instance triggers a full 2x relaunch of its kind. That is mildly wasteful and it is the
accepted tradeoff, and the path is rare. Do not add reduced-multiplicity support.

**A missing reviewer is not a clean reviewer**, and the stakes are higher here than in `pr-review`
because this skill acts on findings rather than just posting them:

- **Separate a deterministic refusal from a transient failure.** A reviewer that returned a
  `skipped_reason` bailed out on purpose (an empty `--roles` set, PR mode with no `gh`, a closed
  PR). Re-running it cannot change the outcome. A reviewer that returned nothing at all, or
  unparseable output, may simply have flaked.

  | reviewer state | meaning | action |
  |---|---|---|
  | missing WITH `skipped_reason` | deterministic refusal | mark `unavailable` immediately; never relaunch it this run |
  | missing WITHOUT `skipped_reason` | possibly transient | relaunch **at most once per run**; on the second miss mark `unavailable` |

  The retry budget is **one per reviewer per RUN, not per iteration.** A per-iteration budget would
  grant a fresh relaunch on every pass, and nothing caps the iteration count, so it would permit
  unbounded relaunches. That is the same bug with extra bookkeeping.
- **An `unavailable` reviewer does not have to report for the loop to stop, and the loop MAY stop
  with one.** It has to be able to, or the loop never terminates. But stopping that way is not a
  clean result. An `unavailable` kind forces `batch_clean` false rather than being excluded from
  that test, Coverage stays `partial`, and the run ends on the "Stopped with incomplete coverage"
  outcome, naming the reviewer and what went unreviewed. Both properties hold at once. A lost
  reviewer can never block the loop from stopping, and the loop never claims a clean result it did
  not earn.
- Never count a non-response as "found nothing".
- **A short kind must not satisfy the loop-exit condition.** A batch is only clean when every kind
  launched this iteration reported, reported nothing, `reviewer_unavailable` is empty, AND the
  unioned `roles_missing` is empty. All four are required. Step 3's table is the authority on which
  row fires. Whether to relaunch a short
  kind follows the retry rule above rather than being automatic. Relaunch it only while it still
  has retry budget, never relaunch a kind already in `reviewer_unavailable`, and let row 1c
  terminate the run once a kind is `unavailable`.
- Surface `reviewers_missing` and the unioned `roles_missing` in the per-iteration summary. The
  unioned `roles_missing` also feeds the Final Report's Coverage section. A shortfall that a later
  relaunch cleared stays in the per-iteration summary and goes no further, since it left nothing
  unreviewed. A run that fixed everything the reviewers that *did* report found is not a run that
  fixed everything.

**Check `scoring.complete` on every in-depth-review result.** An instance reporting `false` did not
run its two-stage confidence filter, so its numbers are self-assessments by the same model that
proposed the findings, not scores.

- **Do not run them through the >=50 filter, and do not fix on them.** Report them in the
  per-iteration summary as unfiltered leads, name the instance, and leave them unfixed.
- Findings carrying `unscored: true`, and any finding with `citation_verified: false`, are treated
  the same way. **This applies to any such finding from any instance**, not only to instances whose
  `scoring.complete` came back `false`. Exclude them per instance, BEFORE the cross-instance pool
  and merge below. Excluding first is what keeps this skill's merge safe without needing a
  downward-merge rule for the field. `pr-review` merges first and so needs one.
- **Fixing on an unscored finding is how a fabricated finding becomes a commit.** This skill edits
  and commits code, so the cost of acting on a self-graded finding is a commit that encodes an
  invented problem. When in doubt, leave it and report it.

Once collection has ended, whether every active sub-agent reported or the give-up bound above was
reached (up to 3 active; fewer when the iteration is pruned):

1. **Pool every finding** from the active result sets into one flat pool. Each finding carries
   its raw `confidence` (0–100), `file`, `line_range`, `category`, originating `sub_agent`,
   and `source` (`"in-depth-review"` or `"gh-style-review"`). Don't pre-segregate
   by source. Cross-prompt triangulation is the point.

2. **Cross-instance dedup.** Two findings are duplicates if they refer to the **same file**
   and have **overlapping line ranges** AND describe substantially the same problem
   (paraphrases count). Cross-source duplicates (one from in-depth, one from gh-style)
   count as duplicates, so merge them.

3. **For each duplicate group, produce one merged finding:**
   - `confidence`: **max** of the group's scores.
   - `cross_instance_agreement`: count of distinct active instances that raised this finding.
   - `sources`: set of distinct source skills (one of `{in-depth-review}`, `{gh-style-review}`,
     or both). Used as a tiebreaker. A both-source finding is stronger signal.
   - `title`, `description`, `suggested_fix`: pick the clearest from the group; if suggested
     fixes differ meaningfully, mention the alternatives in the description.
   - `category`: union of categories.
   - `ticket_id`: preserved from `ticket`-category findings (the Jira ID the gap traces to);
     `null` for all other findings. Never merge two findings with different `ticket_id`s.

4. **Apply the orchestrator's confidence threshold: discard everything with `confidence < 50`.**
   This is the review-and-fix-specific threshold, lower than each sub-skill's default of 70
   because cross-instance triangulation across the active passes raises our confidence in
   50–69 findings.

5. **If the post-filter list is empty** (every finding scored < 50), mark the **findings**
   batch clean. The iteration is **fully clean** only if Step 1.5's Discussion Context
   aggregation is also empty (PR mode). See Step 1.5 below.

6. **Otherwise** proceed to Step 2 with the filtered + deduplicated list, ordered by:
   1. Severity descending (critical -> major -> minor -> suggestion)
   2. `cross_instance_agreement` descending (more instances agreeing wins when severity ties)
   3. Both-sources first (a both-source finding beats a same-confidence single-source one)
   4. `confidence` descending

### Aggregating tickets_examined (in-depth-review only)

Union the `tickets_examined` arrays from the active in-depth-review sub-agents (gh-style-review
has none; there are none this iteration if in-depth-review was not active or role #10 was not
in `<ACTIVE_ROLES>`). Union by `id`; for each `id`, `status` is `gaps` if any instance reported gaps, else
`unread` if any reported unread, else `ok`. The `gaps` count is the number of surviving ticket
findings for that `id` in the merged pool after the >=50 filter. Also collect each instance's
`ticket_review.status`: if any returned `denied` or `unavailable`, record it for the Final
Report so the user knows ticket review did not fully run.

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

Track state explicitly:

- `iteration`: starts at 1, increments before each Step 1 launch
- `batch_clean`: per-iteration flag, true iff the deduplicated findings list was empty AND
  `reviewers_missing` was empty AND `reviewer_unavailable` was empty AND the unioned `roles_missing`
  was empty. An empty findings list with a short kind sets `batch_incomplete`, not `batch_clean`. A
  non-empty `reviewer_unavailable` forces `batch_incomplete` too, because an `unavailable` kind is
  never launched and so never shows up in the per-iteration `reviewers_missing`. A non-empty unioned
  `roles_missing` forces `batch_incomplete` the same way, because a lens no instance reported on
  never shows up in `reviewers_missing` either. Without those conditions `batch_clean` would compute
  true on the very iteration row 1c stops as partial.
- `reviewers_missing`, `roles_missing`: per-iteration; carried into the per-iteration summary and
  Final Report
- `reviewer_unavailable`: per-RUN set of reviewer kinds that will not be relaunched (deterministic
  refusal, or a transient shortfall that already used its one retry)
- `reviewer_retries`: per-RUN count per kind, capped at 1. Increment it on any relaunch of a kind
  that fell short in a prior iteration, whichever row triggered that relaunch. A row-4 full rerun
  that happens to relaunch a previously-short kind consumes that kind's budget too. Never reset it
  between iterations. Resetting it turns a per-run budget into a per-iteration one, and nothing
  caps the iteration count, so the relaunches become unbounded.
- `any_commit`, `any_logic_change`, `productive_reviewers`: per-iteration accumulators from
  Step 2, consumed by the table above
- `iteration_commits`: per-iteration; the `(short sha, finding title)` pairs Step 2 appended, in
  commit order. The per-iteration commit table is this list.
- `run_commits`: per-RUN list of `(iteration, short sha, finding title)`. Append
  `iteration_commits` to it once that iteration's commit table has been emitted. The Final
  Report's Changes Made table is this list. Never reset it between iterations. Empty
  `iteration_commits` as soon as its pairs are appended here. Step 1 sends a clean batch straight
  to Step 3, so Step 2's reset never runs on that iteration, and a stale list would reprint the
  previous iteration's rows under this iteration's header.
- `<ACTIVE_ROLES>`, `<ACTIVE_GH_STYLE>`: the next iteration's active reviewer set
- `discussion_context_snapshot`: per-iteration snapshot of resolved/unaddressed pools (PR
  mode only) — useful for the per-iteration summary
- `resolved_ticket_findings`: ticket findings the user deferred or dismissed (keyed by
  `ticket_id` + title), so later iterations skip them instead of re-prompting.

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

### Worked example

- **Iter 1** (full: all roles + gh-style). Findings from Role #2 (`bug`) and Role #5
  (`comment guidance`). Fixing #2 edits executable code (logic change); fixing #5 edits a
  comment (non-logic). `any_logic_change = true` -> **Iter 2 is a full rerun**.
- **Iter 2** (full). Only Role #5 fires now. Its fix is comment-only. `any_commit = true`,
  `any_logic_change = false`, `productive_reviewers = {5}` -> **Iter 3 is pruned** to
  `--roles 5`, gh-style skipped.
- **Iter 3** (pruned: 2 x in-depth-review `--roles 5`). Clean batch -> **Stop** (row 1). Logic
  reviewers last ran in Iter 2 on logic identical to the final tree, so nothing was missed.

### Worked example, a reviewer that flakes

Traces the retry union, row 1b, and row 1c. All three are absent from the example above.

- **Iter 1** (full). The gh-style instance returns nothing, no `skipped_reason`, so the
  `gh-style-review` kind **fell short**. in-depth reports one `comment guidance` finding, which is
  fixed and committed. `any_commit = true`, `any_logic_change = false`, so row 5 fires and computes
  `productive_reviewers = {5}`. Row 5 alone would drop gh-style, because a kind that reported
  nothing produced no committed fix and so cannot be in that set. **The retry union adds it back**,
  since it fell short and still has budget. `reviewer_retries[gh-style] = 1`. Iter 2 launches
  in-depth `--roles 5` plus gh-style at full multiplicity.
- **Iter 2.** gh-style reports this time, and everything is clean. The shortfall is **cleared**, so
  it is history in the per-iteration summary and NOT a coverage gap. Row 1 fires. Coverage is
  `complete` and the Outcome is "✅ Clean batch". This is the case that must not latch `partial`,
  because every lens did get applied.
- **Iter 2, alternative.** gh-style falls short a second time. Its budget is gone, so it is marked
  `unavailable` and the subtract step drops it for the rest of the run. With the findings list empty
  and every launched kind either reported or `unavailable`, row 1 fails on the non-empty
  `reviewer_unavailable` and **row 1c** fires. The run stops. Coverage is `partial`, naming
  gh-style, and the Outcome is "⚠️ Stopped with incomplete coverage". No green check, because
  Coverage is not `complete`.
- **Row 1b, for contrast.** Had Iter 1's findings list been empty when gh-style fell short, row 1b
  would have fired instead of row 5, and the active set would have been the short kind ONLY. Same
  retry, reached by a different row. That is the path the union generalizes to every other row.

### Per-iteration summary

Every iteration ends by emitting one summary block to chat. Emit it last in Step 3, after the
table has picked a row and after the union and the subtraction have computed the next active set.
That is the earliest point at which every item below exists. The table's "go to Step 1" and
"proceed to Final Report" actions both happen after the block is out. Never batch two iterations
into one block, and never hold a block back to the end of the run.

There is no iteration cap, so the user stops a run that is not converging by interrupting it, and
an interrupted run never reaches Step 4. These blocks plus the commits are then the only record of
what the run did. Emit it to chat, the same as the iteration-start announcement. Do not write it to
a file in the working tree, because Step 2 commits with `git add -A` and would sweep that file into
the next fix commit.

Cover these in this order, one line each, and drop any line that has nothing to say:

- The `iteration` number and the active reviewer set that ran.
- Which kinds reported and which fell short, keeping the per-instance detail, plus the unioned
  `roles_missing` and the retry or `unavailable` state of any short kind. A shortfall that a later
  relaunch cleared is recorded here and nowhere else.
- Every finding kept by the `>=50` filter that did NOT become a commit, and what happened to it,
  meaning deferred, dismissed, abandoned because lint or tests failed, or moot because an
  earlier commit this iteration already fixed it. Committed findings are the table below instead.
- Unfiltered leads, naming the instance and which rule excluded them (`scoring.complete` false,
  `unscored`, or `citation_verified` false).
- `any_logic_change` for the iteration.
- Discussion Context in PR mode, as resolved and unaddressed counts plus whatever changed since
  the previous iteration. Say so instead when gh-style was not active and the previous snapshot
  carries forward unchanged.
- Any anomaly with no other home, such as a `subagent_type` that did not resolve and left effort
  inherited.
- The row that fired, and either the next iteration's active set or the stop.

Then close the block with the iteration's commit table, so the last thing every iteration emits
is the work it actually did:

**Iteration N - M commits**

| Commit | Fix |
|---|---|
| <short sha> | <the finding's title> |

`M` is the row count, and it reads "1 commit" when there is one. One row per entry in
`iteration_commits`, in commit order. Write it as a markdown table and let the terminal draw the
borders. Do not hand-draw a box. A hand-drawn one freezes the column widths at whatever the
template guessed.

`Fix` is the merged finding's `title` verbatim. Step 2 commits one fix per finding, so the
mapping is 1:1 and there is always exactly one title per row. Taking it verbatim is deliberate.
Every cell is then either git output or a string a reviewer wrote, so the table has no room to
describe work that was not done. This file forbids invented findings and invented commits
everywhere else, and a paraphrased `Fix` cell would be the one place that guard is missing. Do
not paraphrase the title, do not substitute the commit subject, and do not add a status column.
Findings that produced no commit are the bullet above, not a row here.

When `M` is 0, emit the header line alone and no table. An empty-findings iteration committed
nothing, and neither did a row 2 stop. An empty table frame says less than the header already
does.

Keep it terse. The loop is uncapped, so this cost is paid on every iteration, and the Final Report
carries the detail. An interrupt during the fix phase leaves that iteration with no block at all.
The commits are already in `git log`, one per fix. Do not reconstruct a Final Report for an
interrupted run, and do not relaunch reviewers to recover state that was never printed.

## Step 4: Final Report

Summarise the entire session in a clear report to the user:

```
## Review and Fix Report

**Target:** PR #<PR>  <-  or  branch range <RANGE>
**Iterations completed:** N
**Total commits made:** N

### Changes Made (omit when the run committed nothing)

| Iteration | Commit | Fix |
|---|---|---|
| Iteration <n> | <short sha> | <the finding's title> |

### Tickets examined
- <id>: ✅ implemented | ⚠️ N gap(s) — <user decision> | ❓ unread

### Remaining Issues (omit when every surviving finding was fixed)
- <finding description> [severity, cross-instance N/M active, sources <in-depth|gh-style|both>, confidence X] — <file:line>
- ...

### Discussion Context (PR mode only; omit entire section if branch mode or both pools empty)

**Resolved by this branch:**
- > <quote> — @<author> ([link](<url>))
  ✅ <resolution>

**Still unaddressed:**
- > <quote> — @<author> ([link](<url>))
  ⚠️ <gap>

(This reflects the LATEST iteration's gh-style-review snapshot. If the loop ran many
iterations, intermediate snapshots are not reproduced here. The per-iteration summaries
above track them as counts plus what changed each iteration.)

### Coverage
complete | partial — `partial` whenever `reviewer_unavailable` is non-empty for the run, OR a kind's
shortfall was still outstanding when the run stopped, OR the unioned `roles_missing` is non-empty.
Consult all three. `reviewer_unavailable` is the per-run set and is the one that survives a reviewer
being dropped from later iterations, so a run that gave up on a reviewer still reports `partial` here
rather than looking complete. A shortfall is **outstanding** when a kind fell short and has not
reported since. The retry union above relaunches such a kind on the next pass, so the only way one
survives to the end of the run is a stop that fires before the retry can happen. That is a row 2
stop, and nothing else. Report it as `partial`, because that kind's lenses really were not applied
to the final tree.

Do NOT decide `partial` from the per-iteration `reviewers_missing` on its own. Report a shortfall that
a later relaunch cleared as history in the per-iteration summary, never as a coverage gap. A kind that
fell short once and then reported after its one relaunch left nothing unreviewed, so latching
`partial` on it would demand a sentence about unreviewed lenses that is not true. That is the
difference the "still outstanding" wording draws. A cleared shortfall is history, an uncleared one is
a coverage gap. When partial, name every reviewer kind and role involved and state which lenses the
branch was NOT reviewed against. Never omit this section; its absence reads as complete coverage.

### Outcome
✅ Clean batch — the loop stopped on row 1, so `batch_clean` was true. The final iteration's active
reviewers ALL reported, they found nothing actionable, and Coverage is `complete`. Done.
— OR —
⚠️ Stopped with incomplete coverage — Coverage is `partial`, so this is not a clean result whatever
else the run achieved. Names what made it partial and says what went unreviewed, and names the row
that stopped the run. Row 1c is the empty-findings case with either an `unavailable` kind or a
non-empty unioned `roles_missing`. A row 2 stop
uses this outcome too whenever Coverage is `partial`, adding that nothing was committed.
— OR —
✅ Converged — the last iteration committed no changes and Coverage is `complete`. Every finding it
surfaced was deferred, dismissed, or left unfixed, and Remaining Issues (or Tickets examined, for
ticket gaps) lists them. Done.
```

**Selecting the Outcome line: a green check requires Coverage to be `complete`.** Key it on Coverage
itself, not on any single one of Coverage's inputs. `reviewer_unavailable` is only one of three things
that force `partial`, so a rule keyed on that variable alone would let a green check sit above a
`partial` Coverage line as soon as another input fired. This governs every stop, not just row 1c.
Whenever Coverage is `partial` the Outcome MUST be the incomplete-coverage line, naming what made it
partial and what went unreviewed. So "✅ Clean batch" is reachable only from row 1 with complete
coverage, and "✅ Converged" only from a row 2 stop with complete coverage. Never pair a green check
with `partial` coverage. The two sections are read together, and a green check above a `partial`
Coverage line is exactly the unearned clean result this machinery exists to prevent.

For the **Changes Made** section: it is `run_commits`, which is the per-iteration commit tables
concatenated with an `Iteration` column added, in commit order across the run. Every row of every
per-iteration table appears here exactly once, and the `Fix` cell is the same finding title, so
the final table never says anything an iteration did not already show. Repeat the iteration label
on every row instead of blank-filling the repeats, so a row read on its own still names its
iteration. **Total commits made** above must equal this table's row count. **Iterations
completed** can exceed the highest iteration label here, because any iteration that
committed nothing contributes no rows. The labels need not be contiguous either. Omit the
section when `run_commits` is empty. That is a run that committed nothing, and the Outcome
line already says so.

For the **Remaining Issues** section: emit it whenever a finding that passed the `>=50` filter did
not become a commit. The row 2 stop is the common case, since findings existed and nothing was
committed, so every one of them is still open. A fix abandoned because lint or tests failed belongs
here too. Deferred and dismissed ticket findings do not. They are listed under Tickets examined
with their decision. A finding made moot because an earlier commit this iteration already fixed it
does not either. Nothing about it is still open, and the per-iteration summary already recorded it
as moot. Omit the section when every surviving finding either became a commit, was made moot by
an earlier commit, or was a deferred or dismissed ticket finding.

For the **Tickets examined** section: omit it entirely when no ticket IDs were found or
`--skip-ticket` was passed. List any deferred or dismissed ticket findings (from
`resolved_ticket_findings`) with their decision. They were not re-prompted in later
iterations. If an in-depth-review sub-agent reported `ticket_review.status` of `denied` or
`unavailable`, note that ticket review did not run and why.

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
