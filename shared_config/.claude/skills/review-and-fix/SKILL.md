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

## Working-tree side-effect policy

A separate policy from the one above, and the one that has actually cost work. **Only this
orchestrator touches the working tree. No reviewer sub-agent may.** Forbidden inside any of them:
`git checkout -- <path>`, `git checkout .`, `git restore`, `git reset --hard`, `git clean`, `rm`,
`git push`, and any edit, creation, or deletion of a file.

Two runs lost work. One reviewer reverted a source file mid-review and concurrent roles read the
reverted state. Another ran `git checkout`-style cleanup, reported "Worktree is now clean", and
discarded the user's own uncommitted edits with no stash entry to recover from. Both agents were
trying to run a **negative control**, which is a good check that needs the tree. Negative controls
belong to this orchestrator, in the fix phase, where they already happen.

**Re-check the tree before every reviewer launch, not only at Step 0.** A run can last hours and
the user can edit files while it is in flight, which is exactly how the uncommitted edits above
were lost. If `git status --porcelain` is non-empty before a launch, stop and tell the user what
is uncommitted rather than launching agents over it. Their work is not yours to stage or to risk.

**After every fan-out returns, and before staging anything, probe content rather than status.**
`git status` cannot detect a revert-and-restore. It reads clean before and after, and dirty only
in the window between, so a status check timed to the fan-out's return sees nothing. Probe for
something the diff deleted, for example `grep -c <removed-symbol> <file>` expecting 0. Do this
before every `git add`, because a reviewer killed between its revert and its restore leaves the
tree holding a revert of the change under review, and the next fix commit would silently include
it.

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
   and recording each committed fix's class (`prose` / `test` / `logic`) and which reviewer it
   came from
6. Decide the next iteration's active set (full rerun / pruned / stop) and repeat from step 2
   until the findings list is empty with every launched reviewer kind reported, none
   `unavailable`, and the unioned `roles_missing` empty (row 1, clean), or the findings list is
   empty with a kind `unavailable` or the unioned `roles_missing` non-empty (row 1c, which stops
   with partial coverage), or an iteration commits nothing, or no reviewer can start at all, or an
   active reviewer instance returns `coverage: "impossible"` (row 0, which aborts the run without a
   report), or the user is shown where the run is going and says to stop (not a row, see Step 3)
7. Emit the per-iteration summary before the loop returns to step 2 or falls through to the
   report. A row 0 abort emits none
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
8. **Confirm the `Agent` tool is available, and abort here if it is not.** Check your own tool
   list. Every reviewer this skill runs is a sub-agent, so without that tool not one of them
   launches and there is nothing to merge or fix. A workflow agent is the usual context that
   lacks it. If the tool is listed but every launch in Step 1 fails because the tool is
   unavailable, so that no reviewer starts, abort the same way. A launch that failed for any other
   reason is not this trigger, and neither is an iteration that deliberately launches nothing.
   Abort with this line, and tell the user to re-run from the main thread:

   ```
   REVIEW_UNAVAILABLE_NO_FANOUT: this skill launches every reviewer as a sub-agent, and this context has no Agent tool, so no reviewer could run. Re-run it from the main thread. A workflow agent is the usual cause.
   ```

   Emit no Final Report, and do not read the diff yourself. That line is the whole output. This
   abort comes before the Jira preflight, because that preflight asks the user a question a
   context with no Agent tool cannot usefully answer.

9. **Jira-tooling preflight** (skip this step entirely if `<SKIP_TICKET>` is true). Before
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

10. **Initialize the active reviewer set** used by Step 1:
    - `<ACTIVE_ROLES>` = all in-depth-review roles `1..12` (drop `10` when `<SKIP_TICKET>` is
      true). This is the set of roles the in-depth-review instances will run.
    - `<ACTIVE_GH_STYLE>` = true.
    Step 3 recomputes both before each subsequent iteration. Iteration 1 always runs the full
    set.

11. **Open the run log.** Resolve its home by running these two commands in order and reading the
    exit code of the first:

    ```
    mkdir -p ~/.melvin/config/logs/review-and-fix
    mkdir -p /tmp/claude-skills-logs/review-and-fix
    ```

    Exit 0 on the first makes that directory the home. Any non-zero exit falls back to the second,
    which is `/tmp` and therefore works on any machine. Run the fallback only after a non-zero
    exit, and never decide the preferred home is unavailable without having tried it. A preference
    with no check behind it becomes the fallback on every run, and a log that silently landed in
    `/tmp` looks the same as one that was meant to.

    Then set `run_log_path` to `<home>/<YYYYMMDD-HHMMSS>-<repo>-<branch>.md`, taking the stamp from
    `date -u +%Y%m%d-%H%M%S`, the repo from the remote's basename, and the branch from
    `git rev-parse --abbrev-ref HEAD`. The stamp leads so a directory listing sorts by run. Write
    the header now, naming the target, the mode, `<RANGE>`, `HEAD`'s short sha, the initial active
    set, and the start time.

    Append to this file as the run goes, never assemble it at the end. Step 3 appends each
    per-iteration block as it emits it, and Step 4 appends the Final Report. The loop has no
    iteration cap, so a run that is not converging ends when the user interrupts it, and an
    interrupted run never reaches Step 4. Appending live is what leaves that run a record, and
    those are the runs most worth reading afterwards.

    The home is deliberately outside the repo under review. Step 2 commits with `git add -A`, so a
    log inside the working tree would land in a fix commit and the next iteration would review it.
    When the repo under review IS `~/.melvin/config`, its `.gitignore` entry for `logs/` is what
    keeps that from happening, so do not write the log anywhere else in that repo.

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

The same measurement is why Step 3 gates it on a logic change. A pass that re-reads unchanged logic
through the weaker lens surfaces a subset of what in-depth already cleared. So gh-style runs on
row 4, and on row 5 only through the floor that keeps the active set from emptying. Its
Discussion Context is why it is not zero, and Step 1.5 carries the previous snapshot forward on an
iteration where it did not run.

Announce at iteration start, reflecting the ACTUAL active set, e.g.:

> Iter N: launching 2 in-depth-review passes (roles: AGENTS.md, comment guidance); gh-style-review skipped.
> Target: PR #<PR> [draft]  <-  or  Target: branch range <RANGE>

or, for a full iteration:

> Iter N: launching 3 reviewer passes in parallel (2 x in-depth-review [all roles], 1 x gh-style-review).
> Target: PR #<PR> [draft]  <-  or  Target: branch range <RANGE>

**Stamp `t0` before the launch**, with `date -u +%FT%TZ`, and append it to `run_log_path` on its
own line the moment you take it. Every stamp in this skill goes to the file when taken, never held
back for the end-of-iteration block. A stamp that lived only in memory is gone when the run is
interrupted, and it gets approximated from recall when the block is finally written.

Record `git rev-list --count <RANGE>` beside `t0`. The range grows as the run commits, and the
count is what turns that into a number rather than an impression.

Record `git diff --name-only <RANGE> | wc -l` beside it. Two numbers, same line. Commits measure how
much the run added and files measure how wide the reviewers had to read, and the second is what
tells you whether a rerun was expensive because the diff grew or because it spread.

Spawn the active sub-agents **in a single message** (concurrent tool-use blocks). Sequential
launches defeat the purpose. Never serialize.

**Spawn each one by `subagent_type`, and pass no `model` override:**

| reviewer | `subagent_type` | model | effort |
|---|---|---|---|
| in-depth-review | `pr-review-finder-indepth` | `sonnet` | `medium` |
| gh-style-review | `pr-review-finder-ghstyle` | `opus` | `low` |

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

`in-depth-review` / `gh-style-review` still pin their own internal tiers (in-depth-review's inner
reviewers -> Opus at `low` and its scorers -> Haiku; gh-style-review's single pass -> Sonnet). The
fix step (Step 2) stays on the **session model.** Applying and
committing code is where the strong model earns its cost. The recall fan-out is not.

### In-depth-review sub-agent prompt

Launched only when `<ACTIVE_ROLES>` is non-empty. See [PROMPT-IN-DEPTH.md](PROMPT-IN-DEPTH.md)
for the exact prompt each of the (up to) two in-depth-review sub-agents receives.

### gh-style-review sub-agent prompt

Launched only when `<ACTIVE_GH_STYLE>` is true. gh-style-review has no roles, so it is rerun as
a whole unit (or skipped entirely). See [PROMPT-GH-STYLE.md](PROMPT-GH-STYLE.md) for the exact
prompt the sub-agent receives.

### Aggregating across the active instances

**Stamp each instance's arrival as you read its result**, with `date -u +%FT%TZ`, appending one
line per instance that names the instance. `t1` is the last of them. `t1` minus `t0` is the
iteration's waiting time, and on a pruned iteration that number is most of the wall clock.

Do not try to stamp the moment collection ended, because there is no such moment to observe.
Results arrive asynchronously on later turns, and you learn collection is over by noticing that
three turns brought nothing new, which is minutes after the last arrival. Measured on the first two
real logs, `t1` was a usable stamp in one iteration out of four, while every `t2` was exact. `t2`
falls where you are acting and the old `t1` fell where you were waiting. Per-instance arrivals
replace one unobservable moment with three observable ones, and they record what a single `t1`
cannot say, which is whether every instance ran long or one straggler held up two that finished
early.

Collect every active sub-agent's result, pool them, deduplicate, and merge into one flat list
filtered to `confidence >= 50`. Follow [AGGREGATING.md](AGGREGATING.md) exactly — four rules
from it matter enough to repeat here: results arrive asynchronously in each sub-agent's own
final text (never in the Agent tool's launch result), a reviewer that falls short is retried once
per run before being marked `unavailable`, a missing reviewer's findings are never fabricated
or inferred, and an instance returning `coverage: "impossible"` aborts the run rather than
counting as `unavailable`. AGGREGATING.md also covers aggregating `tickets_examined`.

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

At the start of the iteration's fix phase, reset six per-iteration accumulators. The first
four are what Step 3 reads to decide the next active set. The fifth is what Step 3's commit
table is built from. The sixth is for the run log, and no stop rule reads it:
- `any_commit` = false — set true the moment any fix is committed.
- `any_logic_change` = false. Set true if any committed fix is classified `logic`
  (sub-step 7).
- `any_test_change` = false. Set true if any committed fix is classified `test`
  (sub-step 7). This is what adds role 9 back to a pruned rerun.
- `productive_reviewers` = empty — the reviewers whose findings were fixed AND committed this
  iteration (in-depth-review role numbers via each finding's `category`, and/or the
  `gh-style-review` unit). This is the base of the pruned set a non-logic iteration reruns, and
  `any_test_change` can add role 9 on top of it.
- `iteration_commits` = empty — one `(short sha, finding title)` pair per committed fix, in
  commit order. Step 3's commit table is this list.
- `attribution` = the ledger [AGGREGATING.md](AGGREGATING.md) built at merge time, one entry per
  active instance. Sub-step 7 is what completes it, by marking which of an instance's `unique_kept`
  findings a commit fixed. Step 3's attribution table is this ledger.
- `self_inflicted_count` = 0. How many of this iteration's findings target a line an earlier
  commit of this same run wrote (sub-step 1).

**Stamp `t_fix` before the first finding**, with `date -u +%FT%TZ`, appending it on its own line as
you take it. This is the fix phase's real start and it is what waiting and fixing time are measured
against. `t_fix` minus `t0` is waiting. `t2` minus `t_fix` is fixing.

Do not measure either against `t1`. `t1` is the last arrival, and a straggler's report can arrive
after fixing has already begun, which put two measured iterations at a fixing time of `0m00s` while
git shows two commits landing inside each. `t1` stays as the coverage record of when collection
finished, and it is no longer a duration boundary.

**No reviewer may still be RUNNING when `t_fix` is stamped.** This is about the agent, not its
notification, and the two come apart. A reviewer that has reported is finished, so a notification
still in transit from it is harmless and is exactly the straggler case above. A reviewer that has
not reported and is still working will read the tree while the fix phase edits it, and that is the
hazard. Before stamping `t_fix`, every launched reviewer must be reported or resolved. Resolve one
by `SendMessage`, asking it to finalize with only what it genuinely received, or by `TaskStop` when
a later full rerun supersedes it. Never begin fixing with one left spinning.

Recording an instance in `reviewers_missing` at [AGGREGATING.md](AGGREGATING.md)'s give-up bound is
the coverage half and it does not satisfy this. That entry says its findings are not in the pool. It
says nothing about whether the process is still reading files, and a written-off reviewer goes on
running until something stops it. Both halves are owed, so record the shortfall and resolve the
agent.

Spawned agents do not appear in `TaskList`. To see whether one is alive, `find` its transcript
under the session's `subagents` directory with `-mmin`, or `wc -c` its `tasks/<id>.output`. Never
`Read` or `tail` that file, which overflows context. Rising bytes prove it is alive, and identical
repeated increments do not prove a poll loop, so nudge before stopping. Stopping discards whatever
roles had already finished inside it.

Measured cost of skipping this: one iteration applied a fix between the gh-style arrival and both
in-depth arrivals, and the Final Report had to caveat that an instance read a tree mutated
mid-review. Coverage was `complete` on every other axis, so the caveat was the only trace.

Process each finding from the ordered work list (Step 1) one at a time. Skip any
`ticket`-category finding already recorded in `resolved_ticket_findings` (deferred or
dismissed in a prior iteration), and any finding of any category recorded in
`skipped_findings` (examined, but no test was possible, see sub-step 3). Do not re-prompt for
either. Both are carried to the Final Report.

### For each finding:

1. **Read the relevant file(s)** to understand the context.

   Then decide whether this finding is one the run created. Run
   `git blame -L <line_range> -- <file>` and check every sha it returns against `run_commits`. A
   hit means an earlier commit of this same run wrote the line now being reported, so increment
   `self_inflicted_count` and note it on the finding. Do this BEFORE the fix, because afterwards
   blame shows the fix instead. Skip it when `run_commits` is empty, which is every finding in
   iteration 1.

   **This changes nothing about how the finding is handled.** Fix it exactly as you would any
   other, and never dismiss or deprioritise a finding for carrying the mark. A defect the run
   introduced is still a defect, and the loop's stop rules do not read this count. It exists so
   the run log can say how much of a long run was spent on its own output.

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

   **Once an approach is chosen, it is settled for this finding.** The choice above happens once,
   before the edit. Reopening it after code exists is the failure mode this clause is here to stop,
   and it looks like this: implement approach A, decide mid-edit that B is better, implement B, then
   conclude A was right after all. Every lap costs a full implementation and leaves the tree in a
   third state that is neither A nor B.

   So the rule is that new information reopens the choice and rereading the same information does
   not. New information is a test that fails, a type error, a call site that makes A impossible, or
   an answer from the user. Your own second thoughts on the same facts are not new information. If
   the doubt is real, finish A, run sub-step 4, and let the result decide. A working A beats an
   unbuilt B, and the next iteration reviews A anyway, so a genuinely worse approach gets caught by
   the loop rather than by relitigating it here.

   When you do reopen a choice, say which new fact reopened it, in one line, before writing code.
   If you cannot name the fact, that is the answer.

3. **Behavior findings get a red test before the edit.** A behavior finding is one where you
   can name an input and the wrong output the current code gives for it. Write that test
   first, run it, and confirm it fails on the assertion the finding names rather than on an
   import error or a missing fixture. Then fix until it passes. Record the failing
   assertion's first line in the commit body as `Red: <line>`. The red run happens before the
   commit, so the never-commit-broken-code rule is untouched. You still run the existing
   suite in sub-step 4. That run is not evidence the finding is fixed, because it only covers
   behavior that already worked.

   Everything else gets no new test. Comment punctuation, a cast turned into a type guard, a
   log removed from beside a throw, a dropped metric, and doc wording are verified by the
   linter, the type checker, or by reading the diff. Never invent an assertion to satisfy
   this rule. A test that also passes against the unfixed code is worse than none, and the
   test-coverage reviewer flags it as ceremony next iteration.

   If the test needs infrastructure the repo lacks (a live DB, a new mock harness, a running
   server), use `ask_user` and offer to fix without a test or to skip the finding. Record a
   skip in `skipped_findings`, keyed by the finding's `file` plus `title`, so later iterations do
   not re-prompt. An
   untestable finding never blocks the loop.

4. **Implement the fix** following all project coding standards:
   - Read the relevant `AGENTS.md` (root and sub-project) for mandatory conventions.
   - A finding asking for error handling does not authorize a `catch` that swallows. If your
     fix adds or edits a `catch`, it must either rethrow (bare, or wrapped with `cause`) or carry a
     comment naming why continuing is correct. AGENTS.md requires that comment and a
     reviewer's request does not waive it. If neither shape fits the finding, use `ask_user`
     rather than guessing at the reviewer's intent.
   - Decide whether the fix changes a signature, a return value, what the code throws, or
     anything else a caller can observe. When it does, list the call sites first with a
     reference search (`mcp__serena__find_referencing_symbols`, or `rg` on the symbol name),
     report the count in one line, and read every call site the change reaches. Any call site
     that needs a matching change goes in the same commit.
   - **A fix that corrects a factual claim gets the same treatment, in prose as much as in
     code.** Search for the claim elsewhere before committing, report the count in one line, and
     correct every occurrence in the same commit. Record the result in the commit body as
     `Swept: <fragment> (<n> sites)`. Search by a distinctive FRAGMENT rather than the whole
     phrase, with `rg -U` or `\s+` for every space, because prose wraps and a line-oriented
     search cannot match a phrase split across two lines. That wrap is how the miss this rule
     exists to prevent actually happened. Bound the search to tracked files the branch has
     ALREADY modified. Report a hit outside that set in one line and do not edit it. The bound
     is what keeps this rule from making things worse. Every lens that raises a prose finding is
     scoped to the branch's own changes, so an occurrence in an unmodified file could never have
     become a finding, and editing it pulls that file into the modified set where role 5 then
     reads all of its pre-existing comments. The judgement call is whether a hit is the same
     claim or a different one that shares wording. A fix confined to formatting or punctuation
     still skips both bullets.
   - **Correcting a claim about another file's mechanism means deleting it, not narrowing it.**
     AGENTS.md's "Claims in authored prose" carries the rule. It matters most here, because a fix
     authored under review pressure reaches for the smallest edit that answers the finding, and for
     a mechanism sentence the smallest edit is a rescope. That leaves a sentence of the same kind,
     which fails again next iteration on a different reader. Replace it with a pointer, an
     invariant, a locally derivable fact, or nothing.
   - Run the project's linter/formatter if one exists and fix any violations it reports.
   - Run the project's tests (`pnpm run test:unit` for the web sub-project, or the equivalent
     for the relevant sub-project) to confirm no regressions.
   - **Do not commit if lint or tests fail.** Fix the failures first or escalate to the user.

5. **Stage the fix, then scan the staged diff.** Run `git add -A`, then `git diff --staged`, and
   read the added lines only. Sub-step 6 stages again, which is then a harmless no-op. Six
   checks. Each starts from a pattern match on the added lines, never from a review of the
   design. Four of them need a judgement call, and each is named where it arises. Fix whatever
   a check catches, re-stage, and rerun the scan. Never commit with a note to fix it later. A
   noted violation is next iteration's finding, which is the cost this scan exists to remove.
   - **Pattern scan, authored prose and added lines only.** No `→ ← … ≥ ≤ × — –` and no curly
     quotes. In comment bodies and prose or doc files only, no ` - ` and no `:` joining two
     independent clauses. AGENTS.md bans both as joiners and this check covers both, because a
     colon joiner caught by role 1 instead costs a whole iteration. A `:` is fine as a
     line-leading label prefix such as `TODO:` or `NOTE:`, and in a ratio, a time, a path, or a
     URL. Arithmetic, YAML and markdown list markers, and CLI examples are not violations.
     No ticket key matching
     `\b[A-Z][A-Z0-9]{1,9}-\d{1,6}\b`, excluding protocol names such as UTF-8, SHA-256,
     RFC-7231, ISO-8601, and CVE-2024. None of the changelog literals `added this`,
     `changed from`, `new logic`, `was previously`, `remove old impl`. No added `as any` and
     no added `as unknown as`. Literal content is exempt, so one of these glyphs inside a
     string, a fixture, or quoted output is not a violation.
   - **Catch artifact.** Every added `catch` rethrows or carries a why-comment (sub-step 4).
   - **`Red:` presence.** The commit message you are about to write in sub-step 6 carries a
     `Red:` line when this was a behavior finding (sub-step 3).
   - **Scope.** `git diff --staged --stat` lists only files the finding named or either search
     in sub-step 4 turned up. For any other file, state why in one line.
   - **Paths and identifiers resolve.** Every file path and every code identifier written into
     authored prose on an added line must resolve. Resolve each one before the commit lands, and
     correct or drop whatever does not. Prose written during a fix is unreviewed prose, and a
     path that no longer resolves is next iteration's finding. The judgement call is whether a
     token is a claim about this repo or an illustrative example, and only a claim has to
     resolve.
   - **Quantifier scan.** In authored prose, no `nothing`, `never`, `always`, `every`, `only`,
     `none`, `the one`, or `the only` ranging over a set the line does not name. Either the line
     names the set, or the word goes. AGENTS.md's "Claims in authored prose" section carries the
     rule and its carve-outs. Literal content is exempt here as it is in the pattern scan, so one
     of these words inside a string, a fixture, or a list of the words themselves is not a
     violation. The judgement call is whether a hit is a carve-out or a real claim.

6. **Commit the fix:**

   ```
   git add -A
   git commit -m "<type>: <short description of what was fixed>

   <optional body explaining why>
   Red: <first line of the failing assertion, behavior findings only>
   Swept: <fragment> (<n> sites), claim corrections only"
   ```

   Use conventional commit types: defined in the `.github/semantic.yml` file (e.g., `fix`,
   `feat`, `refactor`, `docs`, etc.) and ensure the message is clear and concise. If the file
   is missing try to figure out what the correct type should be.

7. **Record what this commit was**, for Step 3's next-active-set decision and its commit table:
   - Set `any_commit = true`.
   - Append the commit's short sha plus the finding's `title` to `iteration_commits`. Take the
     title verbatim from the merged finding. It is the `Fix` cell in both tables.
   - Add the fixed finding's reviewer(s) to `productive_reviewers`: map its `category`(ies)
     to in-depth role number(s) via the table in in-depth-review's Step 1, and add the
     `gh-style-review` unit if the finding came (also) from that source. A finding merged
     across both sources adds both.
   - **Record the finding's `raised_by` set too**, which is a different question from
     `productive_reviewers` and the two are easy to conflate. `productive_reviewers` says which
     ROLE found it, and drives row 5's pruned set. `raised_by` says which INSTANCE found it, and is
     what turns [AGGREGATING.md](AGGREGATING.md)'s ledger entry for a `unique` finding into a
     `unique_actionable` one. A commit here is the only evidence that a finding only one instance
     raised was worth having, so this is the moment the counterfactual becomes measurable.

     **When `raised_by` has exactly one member, append that to the run log on the commit's line**,
     as `actionable_unique=<sub_agent> conf=<confidence>`. This is the counterfactual becoming a
     fact, and it is the whole reason the ledger exists. Nine logged runs could not answer whether
     the second in-depth pass earns its cost, because none recorded which instance a surviving
     finding came from. Append it here rather than assembling it later, for the reason
     [AGGREGATING.md](AGGREGATING.md) gives under the ledger.

     **Append the category and the role numbers it mapped to, to the run log, on the same line as
     the commit's sha.** You are holding both here and nowhere else. Without it, per-role yield can
     only be inferred from which roles the next pruned set contains, and that inference is blind to
     a role whose finding was dismissed, blind to a role that did its whole job on iteration 1, and
     blind to a category that maps to no role at all. All three have already happened. A recorded
     category makes the question "which lens earns its cost" answerable from one run instead of
     argued from a proxy.

     **Three gh-style categories need an alias, because its vocabulary and in-depth's differ.**
     `gh-style-review` emits `CLAUDE.md` where in-depth's table says `AGENTS.md`, and `style`
     for a convention the same role covers. Both map to **role 1**. A `style` finding is not
     role 5, which is specifically in-file comments and narrower. Its `discussion` category
     maps to no role at all, because Discussion Context is the one lens in-depth cannot
     produce. Without these aliases a gh-style AGENTS-compliance finding attributes to no
     in-depth role, so the reviewer that would catch it again is not in the next pruned set.
   - **Classify the commit (diff-based).** Inspect the commit's own diff
     (`git show --format= <sha>`) and put it in exactly ONE of three classes. Evaluate them in
     this order and take the first that matches:
     - **`prose`** if every changed hunk is confined to comments, docstrings / block comments,
       blank-line or whitespace-only edits, or a file on the never-logic list below. This is a
       HUNK test rather than a file test, and the distinction is the whole point. A commit that
       rewrites only the comments inside a live source file is `prose`, because nothing
       executable changed.
     - **`test`** if at least one changed file is a test file, and every other changed file is
       either a test file or on the never-logic list. A test file is one whose name matches
       `*.test.<code-ext>`, `*.spec.<code-ext>`, `*_test.<code-ext>`, or `test_*.<code-ext>`,
       or one that sits under a `__tests__`, `__mocks__`, `__snapshots__`, `tests`, `test`, or
       `spec` directory. Add the equivalent convention for whatever language the repo uses, so
       a genuine test file is not missed. `<code-ext>` means a source-code extension, so
       `parse.spec.ts` is a test file and `openapi.spec.yaml` is not. Directory names match a
       path SEGMENT anywhere in the path, not just at the repo root, so
       `packages/api/tests/parse.test.ts` counts. One shape does not qualify. A file under a
       test path that is not itself a test, such as a shared helper or fixture module, is a
       doubt case for the rule below, because non-test code may import it.
     - **`logic`** otherwise. Any change to executable code lands here. That includes a
       string/number literal that logic reads, a moved statement, and an import. So does a
       behavior fix, because its red test (sub-step 3) and the change to the code under test
       share one commit. Application configuration is `logic` too, meaning whatever the running
       app reads. In this repo that is `config/src/**`, `config/*.yml` and `config/*.yaml`, plus
       any constants or env-default module the app imports. A code path behind a flag that is
       currently off is `logic` as well. The flag is flipped remotely, so no commit marks the
       moment that path goes live.

     **The never-logic list.** A file here never makes a commit `logic` on its own, and never
     disqualifies a commit from `prose` or `test`:

     - Documentation and data: `*.md`, `*.mdx`, `*.txt`, `docs/**`, `*.snap`, `LICENSE`.
       Markdown is on this list unconditionally. A fenced code block inside a `*.md` file does
       not change its class, including in a repo whose tooling extracts and runs those fences.
     - Build, lint and CI configuration: `jest.config.*`, `vitest.config.*`,
       `playwright.config.*`, `pytest.ini`, `conftest.py`, `.eslintrc*`, `.oxlint/**`,
       `.prettierrc*`, `.github/workflows/**`, `Dockerfile*`, `*.lock`, `pnpm-lock.yaml`. Match
       the config basenames wherever they sit.
     - `*.json`, except `tsconfig*.json` and `package.json`. Those two are `logic`, because one
       decides what the compiler emits and the other decides which code is installed.

     Test-runner configuration sits on that list rather than being forced to `logic`. The cost
     is real. A change that breaks which tests run no longer triggers a full rerun, so a suite
     that stopped executing can reach a clean batch. Row 4 exists to revalidate application
     logic, and test wiring is not that.

     Then set the flags: `test` -> `any_test_change = true`; `logic` ->
     `any_logic_change = true`. `prose` sets neither. **First match governs.** A comment-only
     edit to a test file matches both `prose` and `test`, and it is `prose`, because nothing
     executable changed. **When you are genuinely unsure which class the facts put a commit in,
     classify UP** (`prose` -> `test` -> `logic`). That rule breaks ties in your knowledge, not
     ties in the order, and a commit that cleanly matches an earlier class is not a tie. The
     cost of classifying up is only rerunning more reviewers next iteration, never a missed
     defect.

     **Append the class to the run log beside the sha now, as the commit lands.** Not later in the
     per-iteration summary, which is a rollup of what this step already wrote. A run that stops
     emitting summaries mid-way still has to leave a derived class behind, because the next
     iteration's stop decision reads it. See [SUMMARY.md](SUMMARY.md)'s note under the commit table
     for the run where that failed.

     If the log has compressed to one line per iteration, the per-class counts are the floor that
     must survive, as in `commits=9 logic=5 test=3 prose=1`. A measured six-iteration run emitted
     summaries for iteration 1 only, wrote per-sha classes inline for iteration 3, nothing at all
     for iteration 4, and counts for iterations 5 and 6. The counts are enough for rows 4 and 5 and
     enough for the stop gate in Step 3. Iteration 4's nothing is what is banned.

8. After the bookkeeping, move to the next finding.

**Stamp `t2` when the fix phase ends**, after the last finding is processed, appending it on its own
line as you take it. `t2` minus `t_fix` is the iteration's fixing time. An iteration whose findings
were all deferred or skipped still gets a `t2`, and a near-zero fixing time next to a long wait is
itself the finding.

Then append one line carrying the iteration's raw stamps, in this shape:

```
stamps: iter=<n> t0=<...> t_fix=<...> t1=<...> t2=<...> range=<commits> files=<count>
```

Raw values, no subtraction, no derived duration. A reader who wants a timeline greps for `stamps:`
and does the arithmetic. This replaces the summary table that was specified three separate ways and
produced zero rows across seven runs. Every field that landed in those runs was an inline imperative
in this file at the moment of the act, and every field that did not was specified elsewhere for
later assembly, so the line goes here rather than in a sub-file.

## Step 3: Loop Control

Re-running all three passes every iteration is wasteful when the iteration only touched
comments, formatting, or tests, so the next iteration's active set depends on what the last one
actually committed:

- **A committed fix changed program logic** (any `logic` commit) -> the next iteration reruns
  the **full** set. A logic change can introduce a bug, a security hole, a broken test, etc. in
  any domain, so every reviewer must look again.
- **An iteration committed fixes but none changed logic** (every commit was `prose` or `test`)
  -> the next iteration reruns the **reviewers whose findings were fixed this iteration** (the
  productive set), which the two rules below can add to. Logic reviewers cannot surface anything
  new because the logic they last cleared is unchanged. in-depth-review roles are rerun
  individually via its `--roles` flag. `gh-style-review` is one indivisible unit, rerun whole or
  not at all.
- **A `test` commit adds role 9 to that pruned set**, even when no test-coverage finding was
  what got fixed. A test-only commit still adds executable code, and role 9 is the only
  reviewer that judges tests. `gh-style-review` is NOT added. Its findings were measured as a
  strict subset of in-depth's and it skipped test-coverage findings entirely (see Step 1), so on
  a pruned rerun it runs only through row 5's floor, which fires when the in-depth set would
  otherwise be empty.
- **An iteration committed nothing** (every finding was deferred, dismissed, or skipped for
  want of a test) -> **stop**. The diff is unchanged, so a rerun would resurface the
  identical findings. An iteration that skips every finding is the intended outcome of that
  rule, not a malfunction.

After processing all of the iteration's findings, evaluate these conditions **in order.**
The first that matches wins:

| # | Condition                                                                  | Action                                                        |
| - | -------------------------------------------------------------------------- | ------------------------------------------------------------- |
| 0 | No fan-out. Three triggers, and any one fires this row. Step 0 found no `Agent` tool in this skill's own tool list, every Step 1 launch that was attempted failed because the `Agent` tool was unavailable so no reviewer started, or an active reviewer instance returned `coverage: "impossible"`, OR the `REVIEW_UNAVAILABLE_NO_FANOUT` line instead of parseable JSON | **Abort the run.** Not a stop, not `partial` coverage, and no Final Report claiming a review happened. Surface the `REVIEW_UNAVAILABLE_NO_FANOUT` line verbatim, reason included, and tell the caller to re-run from the main thread. No `batch_clean` is computed and no per-iteration summary is emitted for that iteration, because the abort message is the whole record |
| 1 | Findings list empty, every reviewer **launched this iteration** reported, **`reviewer_unavailable` is empty**, **and the unioned `roles_missing` is empty** | **Stop** — clean. Proceed to Final Report |
| 1b | Findings list empty BUT a reviewer launched this iteration is missing and still has retry budget | **Not clean.** Relaunch it next iteration; go to Step 1 |
| 1c | Every reviewer launched this iteration either reported or is `unavailable`, **and EITHER `reviewer_unavailable` is non-empty OR the unioned `roles_missing` is non-empty**. The findings list may be empty or not | **Stop** — coverage is `partial`, not clean. Use the incomplete-coverage outcome, and list any surviving findings under Remaining Issues |
| 2 | `any_commit == false` (findings existed but nothing was committed)         | **Stop** — proceed to Final Report                            |
| 2b | Every surviving finding is `suggestion` severity, AND none is in category `bug`, `db`, `security`, `error-handling`, or `types` | **Stop** — severity floor reached. Proceed to Final Report and list them under Remaining Issues |
| 4 | `any_logic_change == true`                                                 | **Full rerun**: set active set to ALL reviewers; go to Step 1 |
| 5 | Otherwise (committed, but no logic change)                                 | **Pruned rerun**: set active set to `productive_reviewers`, plus role 9 when `any_test_change`; go to Step 1 |

A role-level shortfall is deliberately NOT retried. It blocks the clean exit and surfaces in Coverage
as `partial`, which is why row 1 now requires an empty unioned `roles_missing`.

**There is no iteration cap, and the number column is a label column, not an index.** Rows 1, 1c, 2,
and 2b are the only rows that stop. The user-directed stop below ends a run without being a row, and
an interrupt ends one without reaching Step 4 at all. Row 0 is an abort rather than a stop, so the
run ends there without a Final Report. Row 0 is a backstop. The abort fires in Step 1 the moment an
impossible result is aggregated, before Step 2 applies a single fix. The Step 0 trigger aborts there instead, before
any launch. The launch-failure trigger also fires in Step 1, as soon as the last launch has failed. Rows 1b, 4, and 5 are the only rows that go back to Step 1. Row 1b is
bounded at one retry per reviewer kind per run, and rows 4 and 5 both require `any_commit == true`,
because row 2 stops the run the moment an iteration commits nothing. So the loop continues while an
iteration still commits a fix that row 2b would not have floored. That is real progress only when
the fix targets something the branch did not author during this run.
It is not a termination proof. One iteration's fixes can introduce a defect the next iteration then
finds, and that cycle can sustain itself. Row 2b bounds how long that cycle can run on polish, and
nothing bounds it on substance. A run that is going nowhere on substance is still stopped by the
user, which is why every iteration that reaches Step 3 emits a per-iteration summary.
`iteration` is a label for the announce line, that summary, and the Final Report header. No stop
rule reads it. Do not add a cap row, an iteration count of any size, a periodic check-in, or an
oscillation detector, and do not renumber the rows that remain.

**The user-directed stop is a terminal state and it is not a row.** The paragraph above assumed the
user ends such a run by interrupting it, and an interrupted run never reaches Step 4, so no Outcome
line was ever written for one. What happens instead is that the user is shown where the run is going
and says to stop, in band, and that run does reach Step 4. Measured across nine logged runs: two
ended that way and wrote an outcome by hand, four stopped on a criterion their orchestrator invented
and no row defines, and one of those four named it `converged-on-self`. The rows were never the
problem. The graceful human exit had nowhere to land, so each run built its own.
[FINAL-REPORT.md](FINAL-REPORT.md) now carries its Outcome line. Use that rather than bending row 1c
or row 2 to fit, and rather than naming a stop of your own.

**Ask, do not decide.** The four invented stops decided. The two clean ones asked. When the absorbing
lane's signal is showing, meaning `self_inflicted_count` in the majority with `any_logic_change` and
`any_test_change` both false across three consecutive iterations, put that in front of the user and
let them choose. Presenting a signal is not one of the four forbidden things. It counts no
iterations, fires on no schedule, and stops nothing by itself, and the paragraph above already makes
the user the exit for this case. What is forbidden is ending the run on that signal yourself. Every
invented stop in those four runs was a defensible reading of real evidence, which is what makes them
worth banning. A plausible stop nobody authorized is harder to argue with than a bad one.

**Row 2b is a severity floor, and it is none of the four forbidden things above.** It reads the
severity and category of the findings currently in hand. It does not count iterations, does not
compare an iteration to an earlier one, does not fire on a schedule, and does not detect a cycle. It
is added because measured across seven runs no stop this table defines ever fired, and every one of
those runs stopped anyway on a criterion its own orchestrator invented and recorded nowhere. One
announced row 1c and denied that row's own precondition in the next clause. An unwritten stop varies
per run, has no Outcome line, and cannot be audited. A written one can be argued with.

**A recommendation to stop is held to the same standard, and it is where the invented criterion
now shows up.** Row 2b covers the findings in hand. It does not cover a sentence to the user saying
nothing substantive is left. Before writing one, every commit in the range the next iteration would
review needs a class derived in sub-step 7, and none of them may be `logic`. If a class is missing,
derive it from the commit's diff first. If any is `logic`, that sentence is not available, and the
honest report is that unreviewed logic remains and here is what it touches.

A per-class count for the iteration satisfies this, such as `commits=9 logic=5 test=3 prose=1`. The
gate asks only whether any commit in the range is `logic`, so a count of zero answers it as well as
a per-sha list does, and a count above zero closes the sentence off whichever commit it was. The
per-sha class is still what the commit table and per-role yield need. It is not what this check
needs.

This is the check that would have caught a measured failure. An orchestrator recommended stopping
on the premise that the two unreviewed commits were prose corrections. Neither had a derived class,
both were `logic`, and one had restructured error propagation across a function boundary. The base
rate pointed the other way the whole time. Each of the three prior iterations had found defects in
its predecessor's own logic, ten of them in one case.

The floor is deliberately narrow. `suggestion` severity only, and it never fires while a `bug`, `db`,
`security`, `error-handling`, or `types` finding survives at any severity, because those are the
categories where a miss ships rather than costing a pass. A run that reaches row 2b has findings left
and says so under Remaining Issues. It is not a clean result and does not get a green check.

Computing the next active set, for every row that goes back to Step 1 (1b, 4, and 5):
- **Row 1b (retry):** the active set is the short kind ONLY. `<ACTIVE_ROLES>` = the roles this
  iteration ran if the `in-depth-review` kind fell short, otherwise empty. `<ACTIVE_GH_STYLE>` =
  true iff the `gh-style-review` kind fell short. The diff is unchanged since the other reviewers
  cleared it, so relaunching them buys nothing. The short kind relaunches at full multiplicity.
- **Row 4 (full):** `<ACTIVE_ROLES>` = all roles `1..12` (drop `10` when `<SKIP_TICKET>`),
  `<ACTIVE_GH_STYLE>` = true.
- **Row 5 (pruned):** `<ACTIVE_ROLES>` = the in-depth role numbers in `productive_reviewers`,
  unioned with `{9}` when `any_test_change` is true. `<ACTIVE_GH_STYLE>` = **false**, with one
  floor below. Row 5 fires only when no commit changed logic, and gh-style's findings were
  measured as a strict subset of in-depth's, so a pass that re-reads unchanged logic through the
  weaker lens buys nothing. Being in `productive_reviewers` no longer activates it, and
  `any_test_change` never did.

  **The floor: `<ACTIVE_GH_STYLE>` = true when `<ACTIVE_ROLES>` would otherwise be empty.** That
  is the one case where the alternative is launching nothing at all. An empty active set finds
  nothing, and an empty findings list with `reviewer_unavailable` and `roles_missing` both empty
  is row 1, so the run would stop and report `complete` coverage with no reviewer having read the
  final tree. The category aliases in Step 2 sub-step 7 narrow this to a fixed `discussion`
  finding as an iteration's only commit, since every other gh-style category now attributes to an
  in-depth role too. (`<ACTIVE_ROLES>` plus the floor is what keeps at least one non-empty here.
  `any_commit` was true and every committed fix attributes to a reviewer, but after this change
  the reviewer it attributes to can be the gh-style unit alone.) The role-9 union is computed
  HERE, so the
  retry union below can still add a short kind back and the `reviewer_unavailable` subtraction
  below can still remove role 9 along with the rest of an unavailable `in-depth-review` kind.

**First union in any kind still owed a retry, whichever row fired.** Before launching, add back every
kind that fell short this iteration and still has retry budget, even when the row that fired computed
a set without it. Row 5 is why this is needed. It builds its set from `productive_reviewers`, and a
kind that reported nothing contributed no findings, so it cannot be in that set on its own merit.
The retry is keyed on a kind having fallen short with budget left, never on whether the row's set
already mentions it. That distinction matters because row 5's role-9 union can put role 9 in
`<ACTIVE_ROLES>` while the `in-depth-review` kind reported nothing at all, which happens when
gh-style raised the finding whose fix classified `test`. The kind is still owed its retry there, and
adding it back at full multiplicity supersedes the lone role 9. Without the union, a
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

**Why a `test` commit is safe to prune on.** A `prose` commit is safe because it adds nothing
executable. A `test` commit does add executable code, so it earns its place on the non-logic side
for a different reason. Nothing in production imports a test file, so production behavior after
the commit is byte-identical to what the logic reviewers cleared. That is why the classifier's
`test` bucket excludes the one shape where that claim fails, a shared helper under a test path
that non-test code may import. That shape is `logic`. A test-runner config file is on the
never-logic list instead. Nothing in production imports one of those either, so the
byte-identical claim holds for it, and what it can break is which tests run rather than what the
app does. What a `test` commit CAN introduce is a bad test, which is exactly role 9's lens, and
that is why row 5 unions role 9 in rather than trusting the productive set alone.

**What a pruned `test` iteration gives up.** Roles 1, 5, and 12 are not unioned in, so a pruned
rerun judges the new test code through role 9 alone. Sub-step 5's staged-diff scan is the partial
backstop. It runs before the commit exists and catches the mechanical subset, meaning added
`as any` and `as unknown as`, the banned glyphs, ` - ` clause joiners in comments, ticket keys,
changelog literals, paths or identifiers that do not resolve, and quantifiers over a set the
line does not name. It is NOT a substitute for role 1's full AGENTS.md read or role 12's type
analysis. This is a deliberate cost trade, since
the loop is uncapped and those three roles would be paid on every test iteration. The next
`logic` commit forces a full rerun (row 4) and they see the accumulated test code then.

**What a pruned `prose` iteration gives up, and why that lane is absorbing.** A `prose` commit
usually hands roles 1, 5 and 11 their own output, because `productive_reviewers` is built from the
findings that got fixed, and fixing one of their findings often authors prose. Role 11 belongs in
that list and is not interchangeable with the other two. Roles 1 and 5 check authored prose against
rule text, and role 11 checks a claim in a commit message or a PR body against the runtime site it
names, which is the shape of error this lane actually produces. Row 4 needs a `logic` commit and a
prose-only iteration has none, so there is no route from this lane back to a full rerun. The only
exits are row 1 once roles 1, 5 and 11 accept what the run wrote, row 2 once an iteration commits
nothing, and the user, either by direction or by interrupt. This lane is the case the user-directed
stop exists for, and its signal is the one to put in front of them.

Sub-step 4's claim sweep and sub-step 5's scan are what shorten the lane. They review the authored
prose before it lands rather than an iteration later, and they do not close the lane.
`self_inflicted_count` is the direct signal, because it counts findings whose target line the run
itself wrote. The two booleans corroborate it. Three consecutive iterations with
`any_logic_change` and `any_test_change` both false, on a run whose logic commits have stopped
changing, is this lane. The finding count is not the signal, because it falls monotonically the
whole time the lane runs, which is why a run can look like it is converging while it eats its own
output.

**Role 11 is the seam, and "fixing their findings authors prose" does not hold for it.** A role 11
finding names two things, a claim and the runtime site it describes, and either one can be the
thing that is wrong. Correcting the claim is a `prose` fix. Making the site match the claim is a
`logic` fix, and it is frequently the right one. So an iteration whose findings came from roles 1,
5 and 11 can legitimately commit `logic` and sit on row 4, and it is not in this lane at all.
Measured: one iteration fixed two role-11-shaped findings by changing a function's return type
from `Promise<void>` to an error union, converting two throws into returns, and moving a
control-flow gate that decides who receives a one-shot email. Both commits were `logic`. Both were
reported to the user as corrections to prose, because the findings had been about claims, and the
stop recommendation that followed was wrong.

**Never call a commit or an iteration `prose`, `test`, or `logic` unless that word is the
classifier's verdict for it.** Those three are classes derived from a diff in sub-step 7, and the
same words are used loosely elsewhere in this file for authored text and for what these three roles
review. A finding can be about prose while its fix is `logic`, so the lane a finding came from
never names the class of the commit that fixed it. When no class was derived, say that rather than
supplying one from the finding's subject matter.

Note: a non-empty `unaddressed_pool` in PR mode does NOT prevent the loop from terminating.
"Still unaddressed" items are surfaced for the user to consider, but the fix loop is
driven by the findings list. Those items will either appear as findings in subsequent
iterations (if they're actionable in the current diff) or persist until the user adds
work that addresses them.

### Worked examples

See [EXAMPLES.md](EXAMPLES.md) for a normal full -> pruned -> clean run, and a run where a
reviewer flakes (tracing the retry union, row 1b, and row 1c).

### Per-iteration summary

Every iteration that reaches Step 3 ends by emitting one summary block to chat, closed by
that iteration's commit table, emitted last in Step 3 after the table has picked a row and the
next active set is computed. See [SUMMARY.md](SUMMARY.md) for exactly what to cover, in what
order, and the commit table format.

Append that same block to `run_log_path` as you emit it, byte for byte. Two destinations, one
text. Do not write a different version for the file, and do not hold blocks back to batch them,
because an interrupt between two iterations has to leave the earlier one on disk.

## Step 4: Final Report

Summarize the entire session in a report to the user. See [FINAL-REPORT.md](FINAL-REPORT.md)
for the exact template, how to select the Outcome line, and the per-section rules for Changes
Made, Remaining Issues, and Tickets examined.

Append the report to `run_log_path` too, then tell the user where the log is. That path is the last
line of the run, so it is there whether they want to read the run back or hand several logs to
another agent. A run that aborted on row 0 gets neither, because nothing was reviewed and the log
holds only its header.

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
  no-logic-change iteration reruns `productive_reviewers`, then role 9 is unioned in when any
  commit was `test`, then the retry union adds back any kind still owed one, so it is not
  `productive_reviewers` alone), the row-1b retry (which relaunches only the kind that fell
  short), and the `reviewer_unavailable` subtraction (which never relaunches an unavailable
  kind). Any logic change forces a full 3-agent rerun. Never drop
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
  (in-depth-review: Opus at `low` / Haiku) per those skills. **Never let the reviewer fan-out
  inherit the session model or the session effort.** The model may be Opus or a `[1m]` variant, and unset effort silently
  tracks whatever the user last set with `/effort`. Nothing caps the iteration count, so either
  mistake is paid again on every pass. The fix step (Step 2) — reading, editing, lint/test,
  committing — stays on the **session model**,
  since applying code is where a strong model is worth its cost.
- **One commit per fix** — never squash or amend.
- **Never commit broken code** — lint and tests must pass before committing.
- **Never push** — only local commits; the user decides when to push.
- **Ask before acting on ambiguous findings** — use `ask_user` for anything unclear.
- **Respect project AGENTS.md rules** — always read mandatory checklists before committing.
