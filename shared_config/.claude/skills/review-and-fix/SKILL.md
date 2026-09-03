---
name: review-and-fix
description: >
  Iteratively reviews recent code changes (a PR if one exists for the current branch,
  otherwise the branch's commit range) and fixes what the reviews find. Runs `in-depth-review`
  and `gh-style-review` sub-agents in parallel each iteration, merges and deduplicates their
  findings, and applies fixes one commit at a time. The loop stops when a pass finds nothing, when
  coverage is short, when only low-severity findings survive, when an iteration commits nothing,
  when the user says stop, or it aborts when no reviewer can run at all. No GitHub writes.
  Produces a final summary report.
  Use this skill when the user asks to "review and fix", "review my changes", "clean up my
  code", "improve my recent commits", or similar requests to audit and improve uncommitted or
  branch-local changes.
---

# Review and Fix

This skill wraps `in-depth-review` AND `gh-style-review` with an iterate-and-fix loop. The
per-iteration flow is the progress checklist below.

The triangulation lives **here**, not inside the sub-skills. Each `in-depth-review` pass is
itself a multi-role review, and three of its roles are gated on what the diff contains:
data-layer, security, and TypeScript. Each `gh-style-review` pass is the @claude review GitHub
Action prompt with full PR context (when in PR mode).

**This skill never writes to GitHub.** The full prohibition, the permitted read-only calls, and
the abort action are in Constraints at the end of this file.

**Every act-time field this file records is specified inline at the moment of the act.** The
run-log notes at `~/.melvin/config/docs/research/review-and-fix-run-log/NOTES.md` carry the
seven-run measurement of what happens to a field specified elsewhere for later assembly. Treat that
as a constraint on editing this file. Later references to "the run-log notes" mean that file.

## Working-tree side-effect policy

**Only this orchestrator touches the working tree. No reviewer sub-agent may.** Forbidden inside
any of them: `git checkout -- <path>`, `git checkout .`, `git restore`, `git reset --hard`,
`git clean`, `rm`, `git push`, and any edit, creation, or deletion of a file.

A **negative control** is a good check that needs the tree, and it belongs to this orchestrator, in
the fix phase, where it already happens. Two runs lost work to a reviewer that ran one itself,
recorded in the run-log notes.

**Re-check the tree before every reviewer launch, not only at Step 0.** A run can last hours and
the user can edit files while it is in flight, which is how the uncommitted edits in that record
were lost. If `git status --porcelain` is non-empty before a launch, stop and tell the user what
is uncommitted rather than launching agents over it.

**After every fan-out returns, and before staging anything, probe content rather than status.**
`git status` cannot detect a revert-and-restore. It reads clean before and after, and dirty only
in the window between, so a status check timed to the fan-out's return sees nothing. Probe for
something the diff deleted, for example `grep -c <removed-symbol> <file>` expecting 0. Do this
before every `git add`, because a reviewer killed between its revert and its restore leaves the
tree holding a revert of the change under review, and the next fix commit would silently include
it.

## Progress checklist

Copy this at Step 0. Copy the iteration block again for each iteration.

```
Run setup, once:
- [ ] Tree clean, or the user chose stash or include
- [ ] <RANGE> set, commit count above zero
- [ ] <HAS_PR>, <PR>, <TARGET_ARG>, <SKIP_TICKET> set
- [ ] Agent tool confirmed, else abort with REVIEW_UNAVAILABLE_NO_FANOUT
- [ ] Jira reader ready, or the user chose (a), (b) or (c)
- [ ] <ACTIVE_ROLES> and <ACTIVE_GH_STYLE> set to the full set
- [ ] run_log_path opened, header written

Iteration N:
- [ ] Tree re-checked immediately before the launch
- [ ] t0 plus range and file counts appended
- [ ] Active reviewers launched in ONE message
- [ ] Each instance's arrival stamped as its result was read
- [ ] Findings pooled, deduped, filtered to >=50
- [ ] Discussion Context taken or carried forward (PR mode)
- [ ] Seven accumulators reset
- [ ] Content probed after the fan-out returned, not git status alone
- [ ] Every launched reviewer reported or resolved, none still RUNNING
- [ ] t_fix appended
- [ ] Per finding: blame checked, fix applied, lint and tests green, staged-diff checks run, committed
- [ ] Per commit, appended AS IT LANDED: sha, title, class, category and roles, raised_by
- [ ] t2 appended, then the stamps: line
- [ ] Row picked, next active set computed (union, then subtract)
- [ ] Summary emitted to chat and appended to the log
```

## Step 0: Setup

1. Confirm the working tree is clean (`git status --porcelain`). If there are uncommitted
   changes, warn the user and ask whether to stash first or include them in the review.
2. Resolve the target per [SETUP.md](SETUP.md), and return with `<RANGE>`, `<HAS_PR>`, `<PR>`,
   `<TARGET_ARG>` and `<SKIP_TICKET>` set. Do not proceed with any of them unset.
3. Count how many commits the current branch is ahead of the default branch:
   ```
   git rev-list --count origin/<default-branch>..HEAD
   ```
   If 0, inform the user there are no new commits to review and stop.
4. **Confirm the `Agent` tool is available, and abort here if it is not.** Check your own tool
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
   abort runs before the Jira preflight below, and that order matters. Why: [RATIONALE.md](RATIONALE.md).

5. **Jira-tooling preflight** (skip this step entirely if `<SKIP_TICKET>` is true). Before the
   first review iteration, probe for a ready Jira reader per [SETUP.md](SETUP.md). A reader
   counts only if it is available AND authenticated.
   If neither is ready, ASK the user to choose:
     (a) install/authenticate acli or the Atlassian MCP, then continue. Re-check after they confirm;
     (b) proceed now with `--skip-ticket`. Set `<SKIP_TICKET> = true` and run the other
         reviewers without the ticket check;
     (c) abort.
   Do not start iteration 1 until this is resolved. If a re-check after choice (a) still
   fails, present the three choices again rather than proceeding.

6. **Initialize the active reviewer set** used by Step 1:
   - `<ACTIVE_ROLES>` = all in-depth-review roles `1..12` (drop `10` when `<SKIP_TICKET>` is
     true). This is the set of roles the in-depth-review instances will run.
   - `<ACTIVE_GH_STYLE>` = true.
   Step 3 recomputes both before each subsequent iteration. Iteration 1 always runs the full
   set.

7. **Open the run log.** Resolve `run_log_path` per [SETUP.md](SETUP.md), trying the preferred
   home before the fallback rather than assuming it is unavailable (why: [RATIONALE.md](RATIONALE.md)). **Write the header now**,
   naming the target, the mode, `<RANGE>`, `HEAD`'s short sha, the initial active set, and the
   start time.

   **Append to this file as the run goes, never assemble it at the end.** Step 3 appends each
   per-iteration block as it emits it, and Step 4 appends the Final Report. Why live rather than at
   the end: [RATIONALE.md](RATIONALE.md).

   When the repo under review IS `~/.melvin/config`, its `.gitignore` entry for `logs/` is what
   keeps a log out of a fix commit, so do not write the log anywhere else in that repo.

## Step 1: Review, with the in-depth roles behind a barrier and gh-style as a sub-agent

Launch the iteration's **active** reviewers only:
- If `<ACTIVE_ROLES>` is non-empty, invoke the `review-roles` workflow with `instances: 2` and
  `active_roles: <ACTIVE_ROLES>`. The two instances are the triangulation on the active set. They
  are two runs of each role inside one barrier, not two sub-agents.
- If `<ACTIVE_GH_STYLE>` is true, launch **1 gh-style-review** instance as an Agent-tool sub-agent.
- gh-style-review is **1x by design**, per the asymmetry note directly below.

**Why the two kinds are dispatched differently.** Nesting the in-depth roles inside a wrapper
sub-agent lost their results. A nested agent's completion notification is delivered to the session
root, not to the wrapper that spawned it. Measured on one run: two wrappers launched 24 roles between
them, every role finished, and the wrappers received 5 and 7 of their 12 results while one ran
`bash true` 111 times waiting. The workflow's `parallel()` is a barrier in code and has no
notification to route. gh-style spawns nothing, so it never had the problem, and this thread is the
session root, so its one notification arrives here.
- **Branch mode is not a reason to drop gh-style from a full set.** It still contributes findings
  with empty Discussion Context arrays. This does not override `<ACTIVE_GH_STYLE>`: a pruned
  iteration that computed false still launches nothing.

**Why gh-style-review is 1x and in-depth is 2x.** Measured on fixtures with planted issues,
gh-style's findings were a strict SUBSET of in-depth's, and it skipped test-coverage findings
entirely. It stays at one rather than zero because its real contribution is Discussion Context,
which in-depth cannot produce at all. This loop re-runs its fan-out every iteration with no cap,
so a redundant instance is paid again on every pass. Measured basis:
`~/.melvin/config/docs/research/pr-review-cost-efficiency/RESULTS.md`. Do not raise
gh-style back to parity, and do not drop it to zero.

Announce at iteration start, reflecting the ACTUAL active set, e.g.:

> Iter N: launching 2 in-depth-review passes (roles: AGENTS.md, comment guidance); gh-style-review skipped.
> Target: PR #<PR> [draft]  <-  or  Target: branch range <RANGE>

or, for a full iteration:

> Iter N: launching 3 reviewer passes in parallel (2 x in-depth-review [all roles], 1 x gh-style-review).
> Target: PR #<PR> [draft]  <-  or  Target: branch range <RANGE>

**Stamp `t0` before the launch**, with `date -u +%FT%TZ`, and append it to `run_log_path` on its
own line the moment you take it. Every stamp in this skill goes to the file when taken, never held
back for the end-of-iteration block, because a stamp that lived only in memory is gone when the run
is interrupted.

**Record `git rev-list --count <RANGE>` and `git diff --name-only <RANGE> | wc -l` beside `t0`.**
Two numbers, same line. Commits measure how deep the run went and files measure how wide the
reviewers had to read, which is what tells you whether a rerun was expensive because the diff grew
or because it spread.

Issue the workflow call and the gh-style launch **in a single message** (concurrent tool-use
blocks). Sequential launches defeat the purpose. Never serialize.

### The in-depth roles

Invoked only when `<ACTIVE_ROLES>` is non-empty. Read `in-depth-review/roles/_common-fragment.md`
and every `in-depth-review/roles/NN-<name>.md` in `<ACTIVE_ROLES>`, then:

```
Workflow({
  name: 'review-roles',
  args: {
    target: '<TARGET_ARG>',
    mode: '<pr | branch>',
    instances: 2,
    active_roles: <ACTIVE_ROLES>,
    role_prompts: { '<n>': '<contents of that role file>', ... },
    common_fragment: '<contents of _common-fragment.md>',
    skip_ticket: <SKIP_TICKET>,
    tag: 'iter<N>',
  },
})
```

Pass no `model` and no `effort`. The workflow spawns every role by
`agentType: 'in-depth-review-role'`, and that agent file pins `opus` at `low`. `tag` is the
iteration number, and the workflow writes it into the first line of every role's prompt so the
usage accounting below can tell this iteration's transcripts from the last iteration's on the same
target. See [USAGE.md](USAGE.md).

The call returns `{ results, instances, active_roles }`. Each `results` entry is
`{ instance, role, findings, tickets_examined }`, with `findings: null` for a role that returned
nothing twice. That return IS the in-depth kind's report. It cannot fall short as a kind, because the
barrier resolves every role before the call returns. A null role is a missing role, recorded in that
instance's `roles_missing`, and it is not a kind that failed to report.

**If the `Workflow` tool is absent from your tool list**, the in-depth kind cannot run at all. Do not
fall back to Agent-tool spawns of `in-depth-review`, because that is the nesting the defect lives in.
Treat it as row 0's no-fan-out abort.

### The gh-style sub-agent

Launched only when `<ACTIVE_GH_STYLE>` is true, by `subagent_type: pr-review-finder-ghstyle`, which
pins `opus` at `low`. Pass no `model` and no `effort`. gh-style-review has no roles, so it is rerun
as a whole unit or skipped entirely. See [PROMPT-GH-STYLE.md](PROMPT-GH-STYLE.md) for the exact
prompt.

**If that `subagent_type` does not resolve** (the agent file has not been synced to
`~/.claude/agents/` yet, or was renamed), do not abort the iteration. Fall back to a plain Agent
call with `model: opus`, and note in the per-iteration summary that effort could not be pinned and
therefore inherited the session value.

The fix step (Step 2) stays on the **session model**, since applying and committing code is where
the strong model earns its cost.

### gh-style-review sub-agent prompt

Launched only when `<ACTIVE_GH_STYLE>` is true. gh-style-review has no roles, so it is rerun as
a whole unit (or skipped entirely). See [PROMPT-GH-STYLE.md](PROMPT-GH-STYLE.md) for the exact
prompt the sub-agent receives.

### Aggregating across the active instances

**Stamp each instance's arrival as you read its result**, with `date -u +%FT%TZ`, appending one
line per instance that names the instance. `t1` is the last of them. `t1` minus `t0` is the
iteration's waiting time, and on a pruned iteration that number is most of the wall clock.

**Then append the usage lines, before anything else in this iteration.** Run the recipe in
[USAGE.md](USAGE.md) over the session's transcripts, keep the lines whose stamp carries this
iteration's `tag`, and append them to `run_log_path` as they come out. One line per agent, each
naming its kind, instance, role, attempt, model, turn count, token sums, and a priced estimate. This
is a record of what the fan-out just cost, taken at the moment the cost is knowable and before the
fix phase spends anything, so an interrupted run still has it. The per-iteration summary rolls these
lines up and never replaces them.

Note in the same breath if any line came back `kind=unstamped`. That is an agent this iteration did
not launch, or a prompt that lost its stamp, and either is worth a sentence.

**Do not try to stamp the moment collection ended**, because there is no such moment to observe.
Results arrive asynchronously on later turns, and you learn collection is over minutes after the
last arrival. Per-instance arrivals also record what a single `t1` cannot say, which is whether
every instance ran long or one straggler held up two that finished early. See
[STATE.md](STATE.md) for the measurement behind this.

Collect every active sub-agent's result, pool them, deduplicate, and merge into one flat list
filtered to `confidence >= 50`. Follow [AGGREGATING.md](AGGREGATING.md) exactly. Four rules
from it matter enough to repeat here: results arrive asynchronously in each sub-agent's own
final text (never in the Agent tool's launch result), a reviewer that falls short is retried once
per run before being marked `unavailable`, a missing reviewer's findings are never fabricated
or inferred, and an instance returning `coverage: "impossible"` aborts the run rather than
counting as `unavailable`. AGGREGATING.md also covers aggregating `tickets_examined`.

## Step 1.5: Aggregate Discussion Context (PR mode only)

If `<HAS_PR>` is false, **skip this step entirely.** gh-style-review returned empty
`discussion_context.resolved` and `discussion_context.unaddressed` arrays in branch mode.

If `<HAS_PR>` is true (and `<ACTIVE_GH_STYLE>` is true. When gh-style-review was not active
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
in the current diff, or persist until the user adds work that addresses them). It surfaces in the
per-iteration summary and the Final Report so the user can see the diff's effect on the PR's
discussion thread evolving across iterations.

**Do not synthesize Discussion Context from in-depth-review findings.** Only gh-style-review
produces it, and only in PR mode. **Do not fail an iteration because Discussion Context is
empty.** That is expected in branch mode, and an empty block is never a reviewer shortfall, so it
must not be counted as one when Step 3 evaluates coverage.

## Step 2: Fix

At the start of the iteration's fix phase, reset these seven per-iteration accumulators. Sub-step 7
fills the first five. `attribution` is carried from the merge rather than zeroed here, and
`self_inflicted_count` comes from sub-step 1. [STATE.md](STATE.md) carries each one's exact
definition and its readers, except `attribution`, which
[AGGREGATING.md](AGGREGATING.md)'s attribution ledger defines:
- `any_commit` = false. Set true the moment any fix is committed.
- `any_logic_change` = false. Set true if any committed fix classifies `logic`.
- `any_test_change` = false. Set true if any committed fix classifies `test`. This is what adds
  role 9 back to a pruned rerun.
- `productive_reviewers` = empty. The reviewers whose findings were fixed AND committed this
  iteration. This is the base of the pruned set a non-logic iteration reruns.
- `iteration_commits` = empty. One `(short sha, finding title)` pair per committed fix, in commit
  order. Step 3's commit table is this list.
- `attribution` = the ledger [AGGREGATING.md](AGGREGATING.md) built at merge time, one entry per
  active instance. Sub-step 7 completes it by marking which of an instance's `unique_kept` findings
  a commit fixed. Step 3's attribution table is this ledger.
- `self_inflicted_count` = 0. How many of this iteration's findings target a line an earlier commit
  of this same run wrote (sub-step 1).

**Stamp `t_fix` before the first finding**, with `date -u +%FT%TZ`, appending it on its own line as
you take it. This is the fix phase's real start and it is what waiting and fixing time are measured
against. `t_fix` minus `t0` is waiting. `t2` minus `t_fix` is fixing.

Do not measure either duration against `t1`. `t1` is the last arrival, and a straggler can report
after fixing began, so `t1` is a coverage record and not a duration boundary.

**No reviewer may still be RUNNING when `t_fix` is stamped.** This is about the agent, not its
notification, and the two come apart. A reviewer that has reported is finished, so a notification
still in transit from it is harmless and is exactly the straggler case above. A reviewer that has
not reported and is still working will read the tree while the fix phase edits it, and that is the
hazard. Before stamping `t_fix`, every launched reviewer must be reported or resolved. Resolve one
by `SendMessage`, asking it to finalize with only what it genuinely received, or by `TaskStop` when
a later full rerun supersedes it. Never begin fixing with one left spinning. The run where this was
skipped is in the run-log notes, under "A fix applied while a
reviewer was still reading". Coverage
was `complete` on every other axis, so a caveat in the Final Report was the only trace.

Recording an instance in `reviewers_missing` at [AGGREGATING.md](AGGREGATING.md)'s give-up bound is
the coverage half and it does not satisfy this (why: [RATIONALE.md](RATIONALE.md)). Both halves are owed, so record the shortfall and
resolve the agent.

Spawned reviewer agents do not show up in the session's task list. To see whether one is alive,
`find` its transcript under the session's `subagents` directory with `-mmin`, or `wc -c` its
`tasks/<id>.output`. Never `Read` or `tail` that file, which overflows context. Rising bytes prove
it is alive, and identical repeated increments do not prove a poll loop, so nudge before stopping.
Stopping discards whatever roles had already finished inside it.

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
   other, and never dismiss or deprioritise a finding for carrying the mark. No stop rule reads
   this count. It exists so the run log can say how much of a long run was spent on its own output.

2. **Assess confidence:**
   - If the fix is clear and unambiguous -> implement it directly.
   - If the fix is ambiguous or has multiple valid approaches -> use `AskUserQuestion` to present
     the options and wait for a decision before proceeding.
   - If the finding's `category` is `ticket` -> ALWAYS use `AskUserQuestion`, regardless of how
     clear the fix looks. Ticket gaps are intent decisions, not mechanical fixes. Present the
     ticket ID with the requirement it states, then the gap, meaning what the diff does against
     what the ticket asks. Offer three choices:
       (a) implement the missing intent (then proceed to implement + commit as usual),
       (b) defer, meaning surface only and make no change this run,
       (c) dismiss, meaning the gap is a false positive or out of scope.
     Only implement and commit when the user picks (a). Never auto-commit a ticket finding.
     When the user picks (b) or (c), record the finding in `resolved_ticket_findings` (keyed
     by `ticket_id` + title) so later iterations do not re-prompt for it.

   **When a finding names both a claim and the code the claim describes, either one can be the
   thing that is wrong.** Correcting the claim is a `prose` fix. Making the site match the claim is
   a `logic` fix, and it is frequently the right one. Decide which before you edit, because the
   two land in different commit classes in sub-step 7.

   **Once an approach is chosen, it is settled for this finding.** Relitigating it after code
   exists leaves the tree in a third state that is neither approach, which `git add -A` then
   commits.

   New information reopens the choice and rereading the same information does not. New information
   is a test that fails, a type error, a call site that makes the chosen approach impossible, or an
   answer from the user. Your own second thoughts on the same facts are not. If the doubt is real,
   finish the approach you chose, run sub-step 4, and let the result decide. The next iteration
   reviews it anyway, so a genuinely worse approach gets caught by the loop rather than here.

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
   this rule. A test that also passes against the unfixed code is worse than none. What an invented
   assertion costs: [RATIONALE.md](RATIONALE.md).

   If the test needs infrastructure the repo lacks (a live DB, a new mock harness, a running
   server), use `AskUserQuestion` and offer to fix without a test or to skip the finding. Record a
   skip in `skipped_findings`, keyed by the finding's `file` plus `title`, so later iterations do
   not re-prompt. An untestable finding never blocks the loop.

4. **Implement the fix** following all project coding standards:
   - Read the relevant `AGENTS.md` (root and sub-project) for mandatory conventions.
   - A finding asking for error handling does not authorize a `catch` that swallows. If your
     fix adds or edits a `catch`, it must either rethrow (bare, or wrapped with `cause`) or carry a
     comment naming why continuing is correct. A reviewer's request does not waive that comment.
     If neither shape fits the finding, use `AskUserQuestion` rather than guessing at the intent.
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
     search cannot match a phrase split across two lines. **Bound the search to tracked files the
     branch has ALREADY modified. Report a hit outside that set in one line and do not edit it**,
     because editing it pulls that file into the modified set where role 5 then reads all of its
     pre-existing comments. The judgement call is whether a hit is the same claim or a different
     one that shares wording. A fix confined to formatting or punctuation still skips both
     bullets. Why the bound is drawn there: [RATIONALE.md](RATIONALE.md).
   - **Correcting a claim about another file's mechanism means deleting it, not narrowing it.**
     A fix authored under review pressure reaches for the smallest edit that answers the finding,
     and for a mechanism sentence the smallest edit is a rescope, which fails again next iteration
     on a different reader. Replace it with a pointer, an invariant, a locally derivable fact, or
     nothing.
   - Run the project's linter/formatter if one exists and fix any violations it reports.
   - Run the project's tests (`pnpm run test:unit` for the web sub-project, or the equivalent
     for the relevant sub-project) to confirm no regressions.
   - **Do not commit if lint or tests fail.** Fix the failures first or escalate to the user.

5. **Stage the fix, then scan the staged diff.** Run `git add -A`, then `git diff --staged`, and
   read the added lines only. Sub-step 6 stages again, which is then a harmless no-op. Six
   checks. Each starts from a pattern match on the added lines, never from a review of the
   design. Four of them need a judgement call, and each is named where it arises. Fix whatever
   a check catches, re-stage, and rerun the scan. Never commit with a note to fix it later. What a
   noted violation costs: [RATIONALE.md](RATIONALE.md).
   - **Pattern scan, authored prose and added lines only.** No `→ ← … ≥ ≤ × — –` and no curly
     quotes. In comment bodies and prose or doc files only, no ` - ` and no `:` joining two
     independent clauses, both of which AGENTS.md bans as joiners. A `:` is fine as a
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
     correct or drop whatever does not. The judgement call is whether a token is a claim about
     this repo or an illustrative example, and only a claim has to resolve.
   - **Quantifier scan.** In authored prose, no `nothing`, `never`, `always`, `every`, `only`,
     `none`, `the one`, or `the only` ranging over a set the line does not name. Either the line
     names the set, or the word goes. AGENTS.md's "Claims in authored prose" section carries the
     rule and its carve-outs. Literal content is exempt here as it is in the pattern scan,
     so one of these words inside a string, a fixture, or a list of the words themselves is not a
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
     to in-depth role number(s) via the role table in
     [in-depth-review's Step 1](../in-depth-review/SKILL.md), and add the
     `gh-style-review` unit if the finding came (also) from that source. A finding merged
     across both sources adds both.
   - **Record the finding's `raised_by` set too.** It answers a different question and the two are
     easy to conflate. `productive_reviewers` says which ROLE found it, and drives row 5's pruned
     set. `raised_by` says which INSTANCE found it, and is what turns
     [AGGREGATING.md](AGGREGATING.md)'s ledger entry for a `unique` finding into a
     `unique_actionable` one.

     **When `raised_by` has exactly one member, append that to the run log on the commit's line**,
     as `actionable_unique=<sub_agent> conf=<confidence>`. A commit here is the only evidence that
     a finding one instance raised alone was worth having. Append it now rather than assembling it
     later, for the reason [AGGREGATING.md](AGGREGATING.md) gives under the ledger.

     **Append the category and the role numbers it mapped to, to the run log, on the same line as
     the commit's sha.** You are holding both here and nowhere else, and the next pruned set is not
     a usable proxy for per-role yield.

     **Three gh-style categories need an alias, because its vocabulary and in-depth's differ.**
     Without these a gh-style AGENTS-compliance finding attributes to no in-depth role, so the
     reviewer that would catch it again is not in the next pruned set.

     | gh-style category | maps to |
     |---|---|
     | `CLAUDE.md` | role 1 |
     | `style` | role 1, never role 5, which is in-file comments and narrower |
     | `discussion` | no role. Discussion Context is the one lens in-depth cannot produce |
   - **Classify the commit (diff-based).** Inspect the commit's own diff
     (`git show --format= <sha>`). **Match every changed path against the test-file patterns and
     the never-logic list in [CLASSIFIER.md](CLASSIFIER.md) before you classify.** Then put the
     commit in exactly ONE of three classes, evaluating them in this order and taking the first
     that matches:
     - **`prose`** if every changed hunk is confined to comments, docstrings / block comments,
       blank-line or whitespace-only edits, or a file on the never-logic list. This is a HUNK
       test rather than a file test. A commit that rewrites only the comments inside a live
       source file is `prose`, because nothing executable changed.
     - **`test`** if at least one changed file is a test file, and every other changed file is
       either a test file or on the never-logic list. One shape does not qualify. A file under a
       test path that is not itself a test, such as a shared helper or fixture module, is NOT a
       test file and IS `logic`, because non-test code may import it.
     - **`logic`** otherwise. Any change to executable code lands here, including a
       string/number literal that logic reads, a moved statement, an import, application
       configuration, and a code path behind a flag that is currently off. `tsconfig*.json` and
       `package.json` are `logic` despite the `*.json` entry on the never-logic list, because one
       decides what the compiler emits and the other decides which code is installed. A behavior
       fix is `logic` too, because its red test (sub-step 3) and the change to the code under
       test share one commit.

     Test-runner configuration is deliberately on the never-logic list, so do not classify it up.

     Then set the flags: `test` -> `any_test_change = true`; `logic` ->
     `any_logic_change = true`. `prose` sets neither. **First match governs.** A comment-only
     edit to a test file matches both `prose` and `test`, and it is `prose`, because nothing
     executable changed. **When you are genuinely unsure which class the facts put a commit in,
     classify UP** (`prose` -> `test` -> `logic`). That breaks ties in your knowledge, not ties in
     the order, so a commit that cleanly matches an earlier class is not a tie. Why up is the safe
     direction: [RATIONALE.md](RATIONALE.md).

     **Append the class to the run log beside the sha now, as the commit lands.** Not later in the
     per-iteration summary, which is a rollup of what this step already wrote. A run that stops
     emitting summaries mid-way still has to leave a derived class behind, because the next
     iteration's stop decision reads it. See [SUMMARY.md](SUMMARY.md)'s note under the commit table
     for the run where that failed.

     If the log has compressed to one line per iteration, the per-class counts are the floor that
     must survive, as in `commits=9 logic=5 test=3 prose=1`. The counts are enough for rows 4 and 5
     and enough for the stop gate in Step 3. Emitting nothing for an iteration is what is banned.

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
and does the arithmetic.

## Step 3: Loop Control

Re-running all three passes every iteration is wasteful when the iteration only touched
comments, formatting, or tests, so the next iteration's active set depends on what the last one
actually committed. The rows below carry that in full. in-depth-review roles are rerun
individually via its `--roles` flag, and `gh-style-review` is one indivisible unit, rerun whole or
not at all. An iteration that skips every finding stops on row 2, which is the intended outcome
of that rule and not a malfunction.

After processing all of the iteration's findings, evaluate these conditions **in order.**
The first that matches wins:

| # | Condition                                                                  | Action                                                        |
| - | -------------------------------------------------------------------------- | ------------------------------------------------------------- |
| 0 | No fan-out. Three triggers, and any one fires this row. Step 0 found no `Agent` tool in this skill's own tool list, every Step 1 launch that was attempted failed because the `Agent` tool was unavailable so no reviewer started, or an active reviewer instance returned `coverage: "impossible"`, OR the `REVIEW_UNAVAILABLE_NO_FANOUT` line instead of parseable JSON | **Abort the run.** Not a stop, not `partial` coverage, and no Final Report claiming a review happened. Surface the `REVIEW_UNAVAILABLE_NO_FANOUT` line verbatim, reason included, and tell the caller to re-run from the main thread. No `batch_clean` is computed and no per-iteration summary is emitted for that iteration, because the abort message is the whole record |
| 1 | Findings list empty, every reviewer **launched this iteration** reported, **`reviewer_unavailable` is empty**, **and the unioned `roles_missing` is empty** | **Stop.** Clean. Proceed to Final Report |
| 1b | Findings list empty BUT a reviewer launched this iteration is missing and still has retry budget | **Not clean.** Relaunch it next iteration; go to Step 1 |
| 1c | Every reviewer launched this iteration either reported or is `unavailable`, **and EITHER `reviewer_unavailable` is non-empty OR the unioned `roles_missing` is non-empty**. The findings list may be empty or not | **Stop.** Coverage is `partial`, not clean. Use the incomplete-coverage outcome, and list any surviving findings under Remaining Issues |
| 2 | `any_commit == false` (findings existed but nothing was committed)         | **Stop.** Proceed to Final Report                            |
| 2b | Every surviving finding is `suggestion` severity, AND none is in category `bug`, `db`, `security`, `error-handling`, or `types` | **Stop.** Severity floor reached. Proceed to Final Report and list them under Remaining Issues |
| 4 | `any_logic_change == true`                                                 | **Full rerun**: set active set to ALL reviewers; go to Step 1 |
| 5 | Otherwise (committed, but no logic change)                                 | **Pruned rerun**: set active set to `productive_reviewers`, plus role 9 when `any_test_change`; go to Step 1 |

A role-level shortfall is deliberately NOT retried. It blocks the clean exit and surfaces in
Coverage as `partial`, which is why row 1 requires an empty unioned `roles_missing`.

**There is no iteration cap, and the number column is a label column, not an index.** Rows 1, 1c,
2 and 2b are the only rows that stop. Rows 1b, 4 and 5 are the only rows that go back to Step 1,
and row 1b is bounded at one retry per reviewer kind per run. The user-directed stop below ends a
run without being a row, and an interrupt ends one without reaching Step 4 at all.

Row 0 is an abort rather than a stop, so the run ends there without a Final Report. Its Step 0
trigger aborts before any launch. Its other two triggers fire in Step 1, as soon as the last launch
has failed or an impossible result is aggregated, and in both cases before Step 2 applies a single
fix.

This table is not a termination proof. One iteration's fixes can introduce a defect the next
iteration finds, and that cycle can sustain itself. Row 2b bounds how long it can run on polish,
and nothing bounds it on substance. A run that is going nowhere on substance is stopped by the
user, which is why every iteration that reaches Step 3 emits a per-iteration summary.
`iteration` is a label for the announce line, that summary, and the Final Report header. No stop
rule reads it. Do not add a cap row, an iteration count of any size, a periodic check-in, or an
oscillation detector, and do not renumber the rows that remain.

**The user-directed stop is a terminal state and it is not a row.** The user is shown where the run
is going and says to stop, in band, and that run does reach Step 4, which is the operative
distinction from an interrupt. [FINAL-REPORT.md](FINAL-REPORT.md) carries its Outcome line. Use
that rather than bending row 1c or row 2 to fit, and rather than naming a stop of your own. The
nine logged runs behind this rule are in the run-log notes.

**Ask, do not decide.** When `self_inflicted_count` has been the majority of the kept findings for
three consecutive iterations, put that in front of the user before launching the next one, with the
per-iteration counts and the spend so far, and let them choose. That is the whole trigger. It does
not also wait for `any_logic_change` and `any_test_change` to go false.

It used to. The first version of this rule required the majority AND both booleans false, and one
measured run then went fourteen iterations at about $57 each with self-inflicted findings at 70 to
95 percent from iteration 2 onward, and the ask never fired. Every iteration committed one small
change the classifier correctly called `logic`, a moved statement, a helper extraction, a
`deleted_at` clause, a counter tag's position, so `any_logic_change` was true every time and the
gate stayed shut while the orchestrator wrote "self-inflicted findings were the majority from
iteration 3 onward" into its own Final Report. The passage below already says `self_inflicted_count`
is the direct signal and the booleans only corroborate it. Requiring the corroboration inverted that,
and it cost roughly $500 of the run.

If the user says continue, the signal keeps showing, and the ask fires again after three more
consecutive majority iterations. That is gated on the signal and not on a schedule, so it is not the
periodic check-in the paragraph above forbids.

Presenting a signal is not one of the four forbidden things. It counts no iterations, fires on no
schedule, and stops nothing by itself, and the paragraph above already makes the user the exit for
this case. What is forbidden is ending the run on that signal yourself. A plausible stop nobody
authorized is harder to argue with than a bad one.

The finding count is not the signal, because it falls monotonically the whole time this lane runs,
so a run can look like it is converging while it eats its own output. The lane's only exits are
row 1 once roles 1, 5 and 11 accept what the run wrote, row 2 once an iteration commits nothing,
and the user, either by direction or by interrupt.

**Row 2b is a severity floor, and it is none of the four things banned above.** It reads only the
severity and category of the findings in hand. It does not count iterations, does not compare an
iteration to an earlier one, does not fire on a schedule, and does not detect a cycle. An unwritten
stop varies per run, has no Outcome line, and cannot be audited. A written one can be argued with.
Measured basis: the run-log notes.

**A recommendation to stop is held to the same standard.** Row 2b covers the findings in hand. It
does not cover a sentence to the user saying nothing substantive is left. **Before writing one,
every commit in the range the next iteration would review needs a class derived in sub-step 7, and
none of them may be `logic`.** If a class is missing, derive it from the commit's diff first. If any
is `logic`, that sentence is not available, and the honest report is that unreviewed logic remains
and here is what it touches. A per-class count for the iteration satisfies this, such as
`commits=9 logic=5 test=3 prose=1`, since the gate asks only whether any commit in the range is
`logic`. The run this gate would have caught is in
`~/.melvin/config/docs/research/review-and-fix-run-log/NOTES.md`.

The floor is deliberately narrow. `suggestion` severity only, and it never fires while a `bug`,
`db`, `security`, `error-handling`, or `types` finding survives at any severity, because those are
the categories where a miss ships rather than costing a pass. A run that reaches row 2b is not a
clean result. See [FINAL-REPORT.md](FINAL-REPORT.md) for how it reports.

Computing the next active set, for every row that goes back to Step 1 (1b, 4, and 5):
- **Row 1b (retry):** the active set is the short kind ONLY. `<ACTIVE_ROLES>` = the roles this
  iteration ran if the `in-depth-review` kind fell short, otherwise empty. `<ACTIVE_GH_STYLE>` =
  true iff the `gh-style-review` kind fell short. The short kind relaunches at full multiplicity,
  and why the reviewers that did report are not relaunched is in [RATIONALE.md](RATIONALE.md).
- **Row 4 (full):** `<ACTIVE_ROLES>` = all roles `1..12` (drop `10` when `<SKIP_TICKET>`),
  `<ACTIVE_GH_STYLE>` = true.
- **Row 5 (pruned):** `<ACTIVE_ROLES>` = the in-depth role numbers in `productive_reviewers`,
  unioned with `{9}` when `any_test_change` is true. `<ACTIVE_GH_STYLE>` = **false**, with one
  floor below. Being in `productive_reviewers` does not activate it, and `any_test_change` never
  did.

  **The floor: `<ACTIVE_GH_STYLE>` = true when `<ACTIVE_ROLES>` would otherwise be empty.** An
  empty active set finds nothing, and an empty findings list with `reviewer_unavailable` and
  `roles_missing` both empty is row 1, so the run would stop and report `complete` coverage with no
  reviewer having read the final tree. **Compute the role-9 union HERE**, so the retry union below
  can still add a short kind back and the `reviewer_unavailable` subtraction below can still remove
  role 9 along with the rest of an unavailable `in-depth-review` kind.

**First union in any kind still owed a retry, whichever row fired.** Before launching, add back every
kind that fell short this iteration and still has retry budget, even when the row that fired computed
a set without it. The retry is keyed on a kind having fallen short with budget left, never on whether
the row's set already mentions it. A kind still owed a retry is added back at full multiplicity,
which supersedes a lone role 9. The union gives a shortfall the same one-retry treatment on every
path back to Step 1, which is what makes the retry rule's "at most once per run" a real guarantee
rather than "only when row 1b happened to fire". The worked case that motivates this union, and the
coverage hole without it, is in [PRUNING.md](PRUNING.md).

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

**A single clean iteration ends the loop**, and do not require two consecutive clean ones. The
independent reviewers already triangulate strongly.

**Why pruning is safe without a final full sweep.** Row 5 fires only after an iteration whose
fixes left program logic identical to what the logic reviewers last cleared, and sub-step 7's
classifier forces row 4 the instant a fix touches logic. So by the time the loop stops on row 1,
row 1c or row 2, every logic reviewer that was still available has validated the final logic. An
interrupted run carries no such guarantee, because an interrupt can land right after a logic-change
commit. A kind in `reviewer_unavailable` validated nothing, so a run that stops with one reports
`partial` coverage instead. Full argument: [PRUNING.md](PRUNING.md).

**Why a `test` commit is safe to prune on.** Nothing in production imports a test file or a
test-runner config, so production behavior is byte-identical to what the logic reviewers cleared.
What a `test` commit CAN introduce is a bad test, which is role 9's lens, and that is why row 5
unions role 9 in rather than trusting the productive set alone. See [PRUNING.md](PRUNING.md).

**What a pruned `test` iteration gives up.** Roles 1, 5 and 12 are not unioned in, so a pruned
rerun judges new test code through role 9 alone. Sub-step 5's staged-diff scan is the partial
backstop for the mechanical subset and is not a substitute. The next `logic` commit forces row 4
and they see the accumulated test code then. Cost trade: [PRUNING.md](PRUNING.md).

**The pruned `prose` lane is absorbing.** A `prose` commit usually hands roles 1, 5 and 11 their
own output. Row 4 needs a `logic` commit and a prose-only iteration has none, so no rerun row can
end this lane. Its exits are the three the signal above already names. Full analysis:
[PRUNING.md](PRUNING.md).

**There is a second absorbing lane, and it runs at full cost.** An iteration that commits one small
`logic` change to the run's own earlier fix, a moved statement, an extracted helper, a reordered
guard arm, fires row 4 and reruns every role, and the roles then review that change and find the
next small thing. The classifier is right to call those `logic`. The lane is wrong to keep running.
One measured run did this for twelve iterations at about $57 each with self-inflicted findings at 70
to 95 percent throughout, and it never reached the prose lane because it never had a prose-only
iteration. `self_inflicted_count` is the signal for both lanes, which is why the ask above reads it
alone.

**Never call a commit or an iteration `prose`, `test`, or `logic` unless that word is the
classifier's verdict for it.** Those three are classes derived from a diff in sub-step 7. A
finding can be about prose while its fix is `logic`, so the lane a finding came from never names
the class of the commit that fixed it. When no class was derived, say that rather than supplying
one from the finding's subject matter.

**A non-empty `unaddressed_pool` does not block a stop row.** The fix loop is driven by the
findings list, and no row's condition reads that pool.

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
  `gh pr close`, `gh issue create`, `gh issue comment`, or invoking any skill that does, notably
  the upstream `code-review` skill, whose terminal step posts a PR comment. Permitted read-only
  `gh` calls: `gh pr list`, `gh pr view`, `gh pr diff`, `gh search pulls`, `gh search issues`,
  plus the `gh api` reads gh-style-review uses to pull PR comments, review threads and prior
  reviews. The `gh pr view` this orchestrator runs during Step 0 target resolution is one of the
  permitted reads. If a sub-agent appears about to issue a write command, abort and surface the
  attempt to the user.
- **Iteration 1 runs the full set**, launched in a single message with concurrent tool calls. Do
  not fall back to fewer instances for speed on the first pass. The cross-source triangulation is
  the point.
- **Later iterations use the adaptive active set (Step 3), never an arbitrary reduction.** Step 3
  has exactly three reasons to run fewer reviewers: the pruned-rerun rule, the row-1b retry, and
  the `reviewer_unavailable` subtraction. Never drop a reviewer for speed outside those three.
- **Multiplicity is fixed while a kind is active.** in-depth-review at 2x, gh-style-review at 1x.
  An `unavailable` kind is not launched at all, so the rule binds only while the kind is active.
  There is no reduced-multiplicity path. A kind either relaunches in full or does not relaunch.
- **The two reviewer kinds are dispatched differently, and the asymmetry is the point.** The
  in-depth roles run inside the `review-roles` workflow, two instances behind one barrier, and come
  back unscored so this orchestrator scores each unique finding once after the merge.
  `gh-style-review` runs as one Agent-tool sub-agent with `--raw` and scores its own findings,
  because exactly one instance runs and it spawns nothing. Do not unify these. The dispatch follows
  the multiplicity, and the nesting that the workflow replaced is the one that lost role results.
- **Confidence threshold is 50.** Do not raise or lower it on the fly.
- **Comment-punctuation findings are in scope but low priority.** The shape is a ` - ` or a
  clause-splitting `:` in a comment the diff adds or edits, per AGENTS.md. They are `suggestion`
  severity. Fix them when the iteration surfaces nothing more important, and never spend a fix
  iteration on punctuation while real bugs are outstanding.
- **One commit per fix.** Never squash or amend.
- **Never commit broken code.** Lint and tests must pass before committing.
- **Never push.** Only local commits, and the user decides when to push.
- **Ask before acting on ambiguous findings**, with `AskUserQuestion`.
- **Respect project AGENTS.md rules.** Read the mandatory checklists before committing.
