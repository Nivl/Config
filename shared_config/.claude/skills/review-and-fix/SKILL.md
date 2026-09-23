---
name: review-and-fix
description: >
  Iteratively reviews recent code changes (a PR if one exists for the current branch,
  otherwise the branch's commit range) and fixes what the reviews find. Runs `in-depth-review`
  roles behind a workflow barrier each iteration (gh-style-review too, in iteration 1, when
  invoked with `--gh-style`), merges and deduplicates their
  findings, and applies fixes one commit at a time. The loop stops when a pass finds nothing, when
  coverage is short, when only low-severity findings survive, when an iteration commits nothing,
  when the user says stop, or when it reaches the `--review-limit X` cap if the invocation set one.
  `--full` turns off the mid-run asks to stop early, so only those conditions end the run.
  It aborts when no reviewer can run at all. No GitHub writes.
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

**Only this orchestrator touches the working tree. No sub-agent may.** Forbidden inside every
reviewer, scorer and checker sub-agent: `git checkout -- <path>`, `git checkout .`, `git restore`,
`git reset --hard`, `git clean`, `rm`, `git push`, and any edit, creation, or deletion of a file.

A **negative control** is a good check that needs the tree, and it belongs to this orchestrator, in
the fix phase, where the edit it controls for was just made. Two runs lost work to a reviewer that
ran one itself, recorded in the run-log notes.

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
- [ ] <HAS_PR>, <PR>, <TARGET_ARG>, <SKIP_TICKET>, <REVIEW_LIMIT>, <FULL> set
- [ ] Workflow and Agent tools confirmed, else abort with REVIEW_UNAVAILABLE_NO_FANOUT
- [ ] Jira reader ready, or the user chose (a), (b) or (c)
- [ ] <ACTIVE_ROLES> (roles 1-9 and 11), <INSTANCE_2_ROLES> and <ACTIVE_GH_STYLE> (true only with --gh-style) set for iteration 1
- [ ] run_log_path opened, header written, active-run marker written

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
- [ ] Step 3 ask: when a trigger fires, `AskUserQuestion` with the four facts, or under `<FULL>` an `ask-suppressed iter=` line and no question
- [ ] Per finding: blame checked (`self-inflicted iter=` line with the blamed sha when it hits, then a `rootcause iter=` line before any approach), `reopened` line when the fix changes a run commit's behaviour and a second reopen of the locus asked rather than fixed, `prose-locus` line before a self-inflicted prose fix and that fix a deletion or a pointer only, `outside-branch` line and no edit when the fix is in a file the branch had not changed (call sites excepted), approach settled
- [ ] Per class group: `cases` lines appended before any logic edit (`invariant=` and every exit in `reaches=` when the edit touches paired state), fix written per IMPLEMENTER.md, `quantifier-scan` and `negative-control` lines appended as they happen, one commit, class checked against the group's expected class, recorded
- [ ] Per six commits and at the end: `<PRE_BATCH>` recorded; when the batch holds a `class=logic` commit, `fix-precommit-check` run over `<PRE_BATCH>..HEAD`, hits landed as one follow-up commit with `origin=precommit`, `precommit iter=` line appended; otherwise the `skipped=no-logic` line appended and no check launched
- [ ] Per commit, appended AS IT LANDED as a `commit iter=` line in the sub-step 7 shape: class is logic|test|prose only, origin is its own field, `findings=` lists the group's members and the commit holds one class
- [ ] Per commit, before it landed: `quantifier-scan iter=<N> findings=<ids> hits=<n>` appended, each hit named or resolved
- [ ] t2 appended, then the stamps: line
- [ ] One `usage kind=` line per role agent confirmed in the log (the hook appends them); usage.jq run by hand only if short
- [ ] One `attribution iter=` line per instance appended, after merge and threshold
- [ ] One line appended in exactly this shape, all twelve buckets present: `severity iter=<N> kept: critical=<a> major=<b> minor=<c> suggestion=<d> | dropped: critical=<a> major=<b> minor=<c> suggestion=<d> | fixed: critical=<a> major=<b> minor=<c> suggestion=<d>`
- [ ] Row picked, next active set computed (union, then subtract)
- [ ] Summary emitted to chat and appended to the log
```

## Step 0: Setup

1. Confirm the working tree is clean (`git status --porcelain`). If there are uncommitted
   changes, warn the user and ask whether to stash first or include them in the review.
2. Resolve the target per [SETUP.md](SETUP.md), and return with `<RANGE>`, `<HAS_PR>`, `<PR>`,
   `<TARGET_ARG>`, `<SKIP_TICKET>`, `<REVIEW_LIMIT>` and `<FULL>` set. Do not proceed with any of
   them unset. When `<REVIEW_LIMIT>` is 1 or more, say so in the announce line, so the cap is
   visible from the start rather than only at the ending it produces. When `<FULL>` is true, say
   that too, and say the run will not ask to stop early.
3. Count how many commits the current branch is ahead of the default branch:
   ```
   git rev-list --count origin/<default-branch>..HEAD
   ```
   If 0, inform the user there are no new commits to review and stop.
4. **Confirm both the `Workflow` and the `Agent` tool are available, and abort here if either is
   not.** Check your own tool list. The in-depth roles run inside the `review-roles` workflow, so
   without `Workflow` no role runs. gh-style and the scorer are Agent-tool sub-agents, so without
   `Agent` nothing scores and there is no second reviewer kind. Either absence leaves nothing to
   merge or fix. A sub-agent context is the usual cause of the first, a workflow agent of both. If a
   tool is listed but every launch in Step 1 fails because it is unavailable, so that no reviewer
   starts, abort the same way. A launch that failed for any other reason is not this trigger, and
   neither is an iteration that deliberately launches nothing. Abort with this line, and tell the
   user to re-run from the main thread:

   ```
   REVIEW_UNAVAILABLE_NO_FANOUT: this skill runs its roles through a workflow and its other reviewers as sub-agents, and this context lacks the <Workflow | Agent> tool, so no review could run. Re-run it from the main thread. A sub-agent or workflow-agent context is the usual cause.
   ```

   Do not fall back to spawning the roles as Agent-tool sub-agents when `Workflow` is absent. That
   nesting lost role results, and it is the dispatch the workflow replaced.

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

6. **Initialize the active reviewer set** used by Step 1. Three variables, and they are not
   symmetric:
   - `<ACTIVE_ROLES>` = the roles instance 1 runs. Iteration 1: in-depth-review roles `1..9` and
     `11`. Role 10 does not run in this skill, per the note below. `<SKIP_TICKET>` still gates the Jira preflight and the ticket-category decisions
     in Step 2, and it removes nothing from the set because role 10 is already absent.
   - `<INSTANCE_2_ROLES>` = the roles instance 2 runs. Iteration 1: `{11, 9, 2}`, motivation, test
     coverage and bug scan. Not the full set. From iteration 2 the default is `{}`, per Step 3.
   - `<ACTIVE_GH_STYLE>` = false, unless the invocation carried `--gh-style`, in which case true
     in iteration 1 only.
   Step 3 recomputes all three before each subsequent iteration.

   **Why instance 2 is narrow.** Measured across 45 attributed fixes in three runs, 41 were raised
   by two or more roles, so a second full instance mostly re-finds what the first instance already
   found, and it costs half the fan-out. The three roles it keeps are the ones the data singled
   out. Role 9 had the most sole-raiser fixes of any role. Role 11 is the highest-contributing role
   and the one whose findings can go either way, prose or logic, so a second read of it is worth
   having. Role 2 is the bug scan, which is the lens a second opinion on a fix is for. The
   cross-instance signal, `cross_instance_agreement` and the ledger's `shared` column, now exists
   only for those three roles, and every other role's findings are single-instance by construction.
   That is expected and [SUMMARY.md](SUMMARY.md) says how to read it.

   **Why instance 2 runs in iteration 1 only by default.** Measured after the narrowing. In the one
   run that wrote attribution lines, six iterations, instance 2's `unique_kept` was 1 in iteration 1
   and 0 in iterations 2 through 6. In the other run, four iterations logged in prose, instance 2 at
   `{11, 9, 2}` raised nothing unshared in iterations 1 through 3, and its one unshared finding in
   iteration 4 came from role 5, a set the orchestrator chose with a reason. So on the nine
   iterations after an iteration 1, the default set contributed no finding instance 1 lacked. It cost
   three of fourteen agents, about $8 to $10 an iteration, and bought ordering signal. So from iteration 2 the default is no instance 2, and the discretion rule in Step
   3 is how the orchestrator adds one back when the iteration gives it a reason.

   **Why gh-style is off by default.** Across the priced runs it cost $178 over 36 launches and was
   the sole raiser of one committed fix, in 50 it contributed to. Its distinct product is Discussion
   Context, the PR's prior human comments read against the diff, and every logged run of this skill
   was in branch mode, where there is none. `--gh-style` turns it on for iteration 1, for a PR whose
   discussion is worth reading in.

   **Why role 10 does not run here.** $184 over 77 launches, 53 contributions, zero sole-raiser
   fixes. Its lens is the ticket's intent, and in this skill's usual caller, work-on, that read
   already happened in validation, with the user answering the questions it raises. Role 11 carries
   intent inside the diff. Role 10 still runs in `in-depth-review` and `pr-review`, where no
   validation preceded the review.

   **Why role 7 runs in every full iteration.** At Opus 5 prices it cost $173 over 131 launches for
   two sole-raiser fixes, the worst ratio of any lens, and it ran in iteration 1 only. On Opus 5.5 a
   role-7 launch priced at $0.37 in the first runs, about a quarter of its old $1.32, and the
   security lens on payment code misses in a different shape from the others, so it is back in row
   4's set from 2026-09-23 on the user's decision. Row 5 keeps it only when it was productive, like
   any role.

7. **Open the run log.** Resolve `run_log_path` per [SETUP.md](SETUP.md), trying the preferred
   home before the fallback rather than assuming it is unavailable (why: [RATIONALE.md](RATIONALE.md)). **Write the header now**,
   naming the target, the mode, `<RANGE>`, `HEAD`'s short sha, the initial active set, and the
   start time.

   **Append to this file as the run goes, never assemble it at the end.** Step 3 appends each
   per-iteration block as it emits it, and Step 4 appends the Final Report. Why live rather than at
   the end: [RATIONALE.md](RATIONALE.md).

   **Then write the active-run marker**, so the `usage-lines` hook can find this log:

   ```
   printf '%s' <run_log_path> > ~/.melvin/config/logs/review-and-fix/.active-<session-id>
   ```

   `<session-id>` is the directory name in this session's scratchpad path, the same id
   [USAGE.md](USAGE.md) uses to locate the transcripts. While the marker exists, every tagged
   `review-roles` return appends its priced `usage` lines to the log through the hook, with no
   turn spent here. Step 4 deletes it. A marker left behind by an interrupted run points at a
   finished log, and the hook's id dedupe makes a stale append harmless, but delete it anyway.

   When the repo under review IS `~/.melvin/config`, its `.gitignore` entry for `logs/` is what
   keeps a log out of a fix commit, so do not write the log anywhere else in that repo.

## Step 1: Review, with the in-depth roles behind a barrier and gh-style as a sub-agent

Launch the iteration's **active** reviewers only:
- If `<ACTIVE_ROLES>` or `<INSTANCE_2_ROLES>` is non-empty, invoke the `review-roles` workflow
  with `instances: 2`, `active_roles: <ACTIVE_ROLES>`, and
  `instance_roles: { '2': <INSTANCE_2_ROLES> }`. Instance 1 runs the active set. Instance 2 runs its
  own, narrower set, and when `<INSTANCE_2_ROLES>` is empty pass `instances: 1` instead so no agent
  is dispatched for it. The two instances are asymmetric on purpose, per Step 0.
- If `<ACTIVE_GH_STYLE>` is true, launch **1 gh-style-review** instance as an Agent-tool sub-agent.
  It is true only in iteration 1 of an invocation that passed `--gh-style`.

**Why the two kinds are dispatched differently.** Nesting the in-depth roles inside a wrapper
sub-agent lost their results. A nested agent's completion notification is delivered to the session
root, not to the wrapper that spawned it. Measured on one run: two wrappers launched 24 roles between
them, every role finished, and the wrappers received 5 and 7 of their 12 results while one ran
`bash true` 111 times waiting. The workflow's `parallel()` is a barrier in code and has no
notification to route. gh-style spawns nothing, so it never had the problem, and this thread is the
session root, so its one notification arrives here.
**Why gh-style is opt-in and one pass.** Measured on fixtures
with planted issues, gh-style's findings were a strict SUBSET of in-depth's. Measured since on seven
real runs, it has been the sole raiser of a committed fix zero times. Every finding of its that became
a commit was also raised by an in-depth role. Its distinct contribution is Discussion Context, the
PR's prior human comments cross-referenced against the diff, which in-depth cannot produce and which
does not change while a run is in flight, so one pass captures all of it. In branch mode there is no
Discussion Context and the one pass is a corroborator. Across the priced runs it was the sole raiser
of one fix for $178, so it is off unless `--gh-style` asks for the PR-mode read, and then it gets
one iteration and not a second, because a corroborator paid every pass costs about $5.50 an
iteration.
Measured basis: `~/.melvin/config/docs/research/pr-review-cost-efficiency/RESULTS.md` for the
fixtures, `~/.melvin/config/docs/research/review-and-fix-run-log/NOTES.md` for the runs.

Announce at iteration start, reflecting the ACTUAL active set, e.g.:

> Iter 1: roles 1-9,11 (instance 1) + roles 11,9,2 (instance 2); gh-style off (no --gh-style).
> Target: PR #<PR> [draft]  <-  or  Target: branch range <RANGE>

> Iter 4: roles 1-9,11 (instance 1, full) + roles 2,9 (instance 2: new SQL rewritten, per the log line); gh-style off.

> Iter 5: roles 1,5,9 (instance 1, pruned) + none (instance 2: default); gh-style off.

Name both instances' sets every time, because they differ and the difference is the cost story.

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
    instance_roles: { '2': <INSTANCE_2_ROLES> },
    role_prompts: { '<n>': '<contents of that role file>', ... },
    common_fragment: '<contents of _common-fragment.md>',
    skip_ticket: <SKIP_TICKET>,
    tag: 'iter<N>',
  },
})
```

Pass `args` as a JSON object, not as a string holding JSON. The workflow reads either, so a string
does not fail loudly, and one run passed a string for four iterations while the `usage-lines` hook,
which reads `args.tag`, appended no lines the whole run.

Pass no `model` and no `effort`. The workflow spawns every role by
`agentType: 'in-depth-review-role'`, and that agent file pins `opus` at `low`. `tag` is the
iteration number, and the workflow writes it into the first line of every role's prompt so the
usage accounting below can tell this iteration's transcripts from the last iteration's on the same
target. See [USAGE.md](USAGE.md).

The call returns `{ results, instances, roles_by_instance }`. Each `results` entry is
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

**Then check the usage lines landed, before anything else in this iteration.** The `usage-lines`
hook runs after every Bash call while the marker from Step 0 exists, and appends a line for each
stamped transcript that changed since its last pass, so the first Bash call after the workflow's
notification is where this iteration's lines land. Its `additionalContext` says how many it
appended. It also runs on the `Workflow` return, which is too early to see anything now that the
tool returns at launch. Read the log's tail and confirm one `usage kind=review-roles` line per role agent
dispatched. If the count is short, or the hook reported nothing, run the filter yourself over the
session's transcripts, where `<session>` is the directory holding this session's `.jsonl`:

```
jq -r -n -f ~/.claude/skills/review-and-fix/usage.jq \
  <session>/subagents/agent-*.jsonl \
  <session>/subagents/workflows/*/agent-*.jsonl
```

Keep the lines whose stamp carries this iteration's `tag` and whose `id=` the log does not already
hold, and append them to `run_log_path`. The gh-style and scorer lines arrive this way too, because
those agents finish after the workflow returns, so the hook catches them on the next iteration's
return and the last iteration's on nothing. Run the filter once more at the Final Report for those.
The rates and the filter's rules are in [USAGE.md](USAGE.md), and the command is here because two
of three logged runs skipped it and wrote a token total from the Workflow tool's return instead, one
of them saying it had no rates to price with. A token total is not this line. It carries no
per-agent stamp and no price, so nothing downstream can read it. One line per agent, each
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

If `<GH_STYLE>` is false, skip this step too: no gh-style pass ran, so there is no Discussion
Context in this run at all, and the Final Report's section says so in one line.

If `<HAS_PR>` is true and `<GH_STYLE>` is true (and `<ACTIVE_GH_STYLE>` is true this iteration.
When gh-style-review was not active this iteration there is no new Discussion Context, so carry
forward the previous snapshot):

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

### For each finding, sub-steps 1 and 2. Then sub-step 3 per group, sub-step 7 per commit, sub-step 4 after six commits.

The fixes are written here, by this orchestrator. They were handed to a `fix-implementer`
sub-agent for two runs in September 2026, on the measurement that this context's fix phase cost
more than the review fan-out. At Fable's real rates the hand-off removed about $16.50 an iteration
from this context and the Opus-low implementer cost about $16, and self-inflicted findings per
iteration did not fall. The writing came back here, because the judgement that picks the fix and
the hands that write it are cheaper and no worse in one place. The agent file is kept unwired.

**Commits are grouped by class, not one per finding.** Run sub-steps 1 and 2 for every finding
first. Then sort the findings whose approach is settled into groups: every `prose` fix in one
group, every `test` fix in one group, and `logic` fixes one group per locus, meaning the same
function or the same block, the same sense of locus the reopen rule uses. A finding the reopen
rule sent to the user is in no group until the user has answered, and then in the group its
answer implies. The expected class is the one sub-step 2's approach implies, and
sub-step 7 still classifies the landed commit from its diff, and the diff wins. When the two
disagree, a `prose` or `test` group that landed as `logic` because one member's fix touched code,
record the commit as what it is, add `class_expected=<prose|test>` to its commit line, and for the
rest of the iteration put that member's kind of fix in its own group. Do not stage a group whose
edits you can already see span classes. Split it before the commit, because a mixed commit is the
failure this rule exists to prevent and it fires row 4 for the whole group. Each group is one pass
through sub-step 3 and one commit. The reason is measured twice over. One commit per finding made 22
commits in one iteration where bundling made 4 or 5, and each commit carries a test run, the
seven checks, a negative control, the hook, and four log lines, so that iteration's fix phase
cost $121 and an hour against about $26 before. Bundling across classes, the other way round, put
one logic hunk in with six prose fixes, made the whole commit `logic`, fired row 4 every time
and never let row 5 prune. Grouping by class keeps `class` pure per commit and lands 3 to 5
commits an iteration.
What did survive from that work is the rest of this step: [IMPLEMENTER.md](IMPLEMENTER.md) is the
fix sub-steps as a checklist, `fix-precommit-check` reads every six commits, and commits are
grouped by class.

1. **Read the relevant file(s)** to understand the context.

   Then decide whether this finding is one the run created. Run
   `git blame -L <line_range> -- <file>` and check every sha it returns against `run_commits`. A
   hit means an earlier commit of this same run wrote the line now being reported, so increment
   `self_inflicted_count`, note it on the finding, and append
   `self-inflicted iter=<N> finding=<id> blamed=<sha>` to the run log, one line per blamed run
   commit. The sha is what joins this finding to the `precommit` line that passed that commit,
   which is how the Final Report counts the pre-commit check's misses. Do this BEFORE the fix,
   because afterwards blame shows the fix instead. Skip it when `run_commits` is empty, which is
   every finding in iteration 1.

   **A self-inflicted finding gets a root cause before sub-step 2 chooses anything.** Read the
   blamed commit's diff and the `cases`, `invariant` and `reopened` lines its iteration wrote for
   that locus. Then append `rootcause iter=<N> finding=<id> blamed=<sha> because="<why the
   previous fix was wrong>"`. "It released on the normal return and not on the throw at the
   hydration await" is a root cause. "It should release in finally" is a plan, and it goes in
   sub-step 2's approach instead. When the blamed commit wrote no `invariant=` line and the code
   pairs state, the root cause is that the property was never stated, and the fix starts by
   stating it in this iteration's case list. The point is that the second fix on a locus starts
   from the property and the reason the first one missed it. On the run that motivates this, one
   claim/release region written in iteration 1 was fixed in iterations 2, 3 and 4, each fix closing
   the exit the finding named, and none of the four said what the region had to guarantee. Nothing
   downstream reads the `rootcause` line. It exists for the writer, the way the case list does, and
   for a reader of the log tracing how a chain of fixes reasoned.

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
   finish the approach you chose, run its checks, and let the result decide. The next iteration
   reviews it anyway, so a genuinely worse approach gets caught by the loop rather than here.

   When you do reopen a choice, say which new fact reopened it, in one line, before writing code.
   If you cannot name the fact, that is the answer.

   **The same rule holds across iterations, and it is logged.** When blame in sub-step 1 marked
   this finding self-inflicted and the fix you are about to write changes the behaviour that
   blamed commit introduced at the same locus, rather than a defect in how it was written, that is
   a reopen. Append `reopened iter=<N> finding=<id> locus=<file:range> prior=<sha> fact=<the new
   fact>` before editing. A behaviour change is one the blamed commit's own test would have to
   change to pass. Fixing an unreleased lock, a wrong log level, or a stale comment in that commit
   is not a reopen. Replacing its retry with a release, or its release with a report, is.

   **The second reopen of the same locus goes to the user, not to the tree.** Grep the run log for
   `reopened` lines with the same `locus`. If one exists, do not edit. Put both decisions, the
   fact each claimed, and the finding in front of the user with `AskUserQuestion`, and record their
   answer as `user-decision iter=<N> locus=<file:range> chose=<...>`. Measured on one ten-iteration
   run, the fix phase changed its mind about one delivery's retry semantics three times, in
   iterations 5, 7 and 10, each as a logic commit that drew 13 findings against the previous
   decision, and that was about $240 of a $385 run. Two decisions on one locus is a design
   question, and a design question is the user's.

   **A fix outside the branch's files is a follow-up by default.** Before editing a file, check it
   is one the branch changed before this run started:
   `git diff --name-only <merge-base>..<the HEAD sha in the log header> -- <file>`. Empty output
   means it is outside. Two cases may still edit it: a call site or test the branch's own change
   makes wrong, per IMPLEMENTER.md's call-site bullet, and a file this run already had to edit under
   that same exception. For anything else, do not edit. List the finding under Remaining Issues as
   a follow-up and append `outside-branch iter=<N> finding=<id> file=<path> action=followup`. When
   the finding is critical or major, ask instead, with "Leave it as a follow-up (Recommended)" as
   the first option, and log `action=asked` plus the `user-decision` line. The rule covers
   pre-commit hits in sub-step 4 the same way. Measured on GRO-18007: the branch changed two
   controllers, a pre-commit hit in iteration 2 led into the shared `libraries/promises` helper,
   iterations 3 to 8 each redesigned that helper's error message on a role 8 or role 11 finding, and
   iteration 8 restored it to master. Six of nine iterations went to a file the ticket never touched.

   **A self-inflicted prose finding has its own version of this rule, in IMPLEMENTER.md section
   2.** The fix is a deletion or a pointer, the second finding on the same sentence is a deletion,
   and each is logged as a `prose-locus` line. The reopen rule above is for behaviour and does not
   cover a sentence, and the run that motivated the prose rule rewrote five sentences four to six
   times each without a single `reopened` line, because none of the rewrites changed behaviour.

3. **Write the fix, following [IMPLEMENTER.md](IMPLEMENTER.md) in order.** Its sections are the
   case list for a logic group, the red test for a behavior finding, the implementation rules, the
   seven staged-diff checks with the negative control, and the commit. One group, one commit. The commit line's `findings=` lists
   every member of the group, and its log lines carry the same list. `git add -A` is for this
   group's files, so when the tree holds edits for another group, commit this one first.
   Each check that IMPLEMENTER.md says to
   log is appended to `run_log_path` as it happens: `quantifier-scan iter=<N> findings=<ids> hits=<n>`
   with one line per hit, and `negative-control iter=<N> findings=<ids> test="<name>" reverted=<what> fails_without_fix=<yes|no>`.
   A `no` means the test does not get committed as it stands.

   When the `ask-prose-claims` hook denies the commit and every hit is a carve-out you can name,
   name each one on a line under the `quantifier-scan` line, then commit with the
   `PROSE_CLAIMS_OK=1` prefix, which puts the commit in front of the user at that moment. A hit
   that is a claim gets rewritten, not overridden.

   An untestable finding (the test needs infrastructure the repo lacks) goes to the user with
   `AskUserQuestion`, fix without a test or skip, and a skip lands in `skipped_findings` keyed by
   `file` plus `title`. A fact that defeats the approach chosen in sub-step 2, meaning a failing
   test, a type error, or a call site the approach cannot handle, is the one thing that reopens
   it, once. Say the fact in one line and choose again.

4. **After every six commits, and at the end of the iteration's fixes, run the pre-commit check
   over them, when the batch holds a `class=logic` commit.** Record `git rev-parse --short HEAD`
   as `<PRE_BATCH>` before the first commit of each batch. A batch whose `commit iter=` lines are
   all `class=prose` or `class=test` is not checked. Append
   `precommit iter=<N> batch=<b> findings=<committed ids> skipped=no-logic` and go on. Measured on
   the two runs where the check read prose-only batches, 65 of 65 self-inflicted findings in the
   next iteration sat on commits it had passed, and its follow-up commits were the blamed commit
   for 23 of one run's 56 self-inflicted findings, twice as the source of that iteration's majors.
   On a prose batch the check is a third writer on the same sentences, and the roles read its
   output next iteration as they read yours. Its hits on logic batches in the same runs were real
   (a warn on a rejected-payload arm, a locale-aware binding check), so it stays for those. For a
   batch with a logic commit, launch `fix-precommit-check`, the stamp first:

   ```
   <!-- fix-precommit tag=iter<N> batch=<b> findings=<committed ids> target=<TARGET_ARG> -->

   Commits being checked: <PRE_BATCH>..HEAD, one per class group: <sha: ids title, ...>
   Case lists for the logic commits: <the cases lines from the run log, verbatim>
   Files changed: <git diff --stat <PRE_BATCH>..HEAD>

   Run `git diff <PRE_BATCH>..HEAD` in this checkout and check it per your instructions. Name the
   commit each hit belongs to. Return PRECOMMIT_HITS as specified.
   ```

   Wait for its task notification. A hit whose fix is in a file outside the branch follows the
   outside-branch rule in sub-step 2. Fix each other hit that is a defect, run the seven checks on the
   staged result, and land them as one follow-up commit whose message names the commits it
   corrects, recorded in sub-step 7 with `origin=precommit` and `findings=<the ids whose commits
   it corrected>`. It closes no finding of its own, so its reviewers for `productive_reviewers`
   come from the hits' buckets: bucket 1 adds roles 1 and 5, bucket 2 adds roles 2 and 8, bucket 3
   adds role 9, bucket 4 adds role 1. Those are the lenses that would have raised the same defects
   next iteration. Amending the originals is forbidden. Then append
   `precommit iter=<N> batch=<b> findings=<committed ids> model=<model pinned in its agent file> hits=<n> fixed=<n> left=<n>`
   with each left hit's reason on the next line. `model` is the agent file's pin, not a usage
   line, because the usage line for an Agent-tool launch arrives on the next workflow return or at
   the Final Report, and the Final Report reads the actual model from there. If the check returns
   nothing or output that does not parse, the line is `model=none hits=? fixed=0 left=0 state=missing`.
   It is a check, not a gate.

   Why a separate agent, and why per six commits: across five runs, thirty of sixty-nine fix
   commits were fixes to the run's own earlier fixes, and every one had passed lint, tests and the
   staged-diff checks. A reader who did not write the diff catches what the writer cannot, and a
   launch per commit would cost a round trip through this context each time. The price of the
   batch is that a caught defect sits in history for a few commits before the follow-up corrects
   it. Its tier is pinned in its agent file and measured by the Final Report's Precommit block.

5. Reserved. Sub-steps 5 and 6 belonged to the implementer hand-off and are gone. The numbering
   stays so the cross-references to sub-step 7 hold.

6. Reserved.

7. **Record what the commit was**, once per commit, the follow-up commit included, for Step 3's
   next-active-set decision and its commit table.
   The bullets below collect the fields. The commit line at the end of this sub-step is the one
   place they are written to the run log, as a single line per commit.
   - Set `any_commit = true`.
   - Append the commit's short sha plus every closed finding's `title` to `iteration_commits`.
     Take each title verbatim from the merged finding. They are the `Fix` cell in both tables,
     joined with `; `, and the count cell is their number.
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

     **When `raised_by` has exactly one member, the commit line carries
     `actionable_unique=<sub_agent> conf=<confidence>`.** A commit here is the only evidence that
     a finding one instance raised alone was worth having. It goes on the commit line as the commit
     lands rather than being assembled later, for the reason [AGGREGATING.md](AGGREGATING.md) gives
     under the ledger.

     **The category and the role numbers it mapped to go on the same commit line.** You are holding
     both here and nowhere else, and the next pruned set is not a usable proxy for per-role yield.

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
       either a test file, on the never-logic list, or changed only in hunks that `prose` would
       accept, meaning comments, docstrings, and whitespace. That last clause is the hunk principle
       `prose` already uses, applied to the files a `test` commit carries alongside its tests. A
       production file whose only change is a rewritten comment leaves production behaviour
       byte-identical, which is the whole reason `test` is safe to prune on, so it cannot be the
       thing that disqualifies the class. Measured: one commit added 20 executable lines to a test
       file and rewrote comments in two production files, was classified `logic` on the file-level
       reading, and forced a full rerun of 24 roles for a commit that changed no behaviour. The user
       caught it. One shape still does not qualify. A file under a test path that is not itself a
       test, such as a shared helper or fixture module, is NOT a test file and IS `logic`, because
       non-test code may import it.
     - **`logic`** otherwise. Any change to executable code lands here, including a
       string/number literal that logic reads, a moved statement, an import, application
       configuration, and a code path behind a flag that is currently off. `tsconfig*.json` and
       `package.json` are `logic` despite the `*.json` entry on the never-logic list, because one
       decides what the compiler emits and the other decides which code is installed. A behavior
       fix is `logic` too, because its red test (IMPLEMENTER.md section 1) and the change to the code under
       test share one commit.

     Test-runner configuration is deliberately on the never-logic list, so do not classify it up.

     Then set the flags: `test` -> `any_test_change = true`; `logic` ->
     `any_logic_change = true`. `prose` sets neither. **First match governs.** A comment-only
     edit to a test file matches both `prose` and `test`, and it is `prose`, because nothing
     executable changed. **When you are genuinely unsure which class the facts put a commit in,
     classify UP** (`prose` -> `test` -> `logic`). That breaks ties in your knowledge, not ties in
     the order, so a commit that cleanly matches an earlier class is not a tie. Why up is the safe
     direction: [RATIONALE.md](RATIONALE.md).

     **Append the commit line to the run log now, as the commit lands, in this exact shape:**

     ```
     commit iter=<N> sha=<short sha> class=<logic|test|prose> origin=<self-inflicted|branch|precommit> findings=<ids> category=<...> roles=<...> raised_by=<...> title="<title>"
     ```

     `class` takes one of the three values above and nothing else. It is what rows 4 and 5 read,
     and one run wrote `class=self-inflicted` and `class=new-from-branch` on twelve commits, which
     left the classifier's input unreadable for those. Whether the fix targets the run's own earlier
     commits is `origin`, a separate field. It is `branch` when any finding the commit closes did
     not count toward `self_inflicted_count`, `self-inflicted` when every one of them did, and
     `precommit` for the follow-up commit that lands the pre-commit check's hits, whose `findings=`
     names the findings whose commits it corrected. A
     commit that touches the branch's own work at all is branch work, whatever else it tidies.
     `findings` names the merged ids the commit closed. Add `actionable_unique=<id>:<sub_agent>:<conf>` once per closed finding the rule above
     applies to, comma-separated when there are several. Not later in
     the per-iteration summary, which is a rollup of what this step already wrote. A run that stops
     emitting summaries mid-way still has to leave a derived class behind, because the next
     iteration's stop decision reads it. See [SUMMARY.md](SUMMARY.md)'s note under the commit table
     for the run where that failed.

     If the log has compressed to one line per iteration, the per-class counts are the floor that
     must survive, as in `commits=9 logic=5 test=3 prose=1`. The counts are enough for rows 4 and 5
     and enough for the stop gate in Step 3. Emitting nothing for an iteration is what is banned.

8. After the bookkeeping, move to the next group, or to sub-step 4 when six commits have landed
   since the last check or the groups are done.

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
| 2b | Every surviving finding is `suggestion` severity, AND none is in category `bug`, `db`, `security`, or `error-handling` | **Stop.** Severity floor reached. Proceed to Final Report and list them under Remaining Issues |
| 4 | `any_logic_change == true`                                                 | **Full rerun**: set active set to ALL reviewers; go to Step 1 |
| 5 | Otherwise (committed, but no logic change)                                 | **Pruned rerun**: set active set to `productive_reviewers`, plus role 9 when `any_test_change`; go to Step 1 |

A role-level shortfall is deliberately NOT retried. It blocks the clean exit and surfaces in
Coverage as `partial`, which is why row 1 requires an empty unioned `roles_missing`.

**This skill sets no iteration cap of its own, and the number column is a label column, not an
index.** Rows 1, 1c, 2 and 2b are the only rows that stop. Rows 1b, 4 and 5 are the only rows that
go back to Step 1, and row 1b is bounded at one retry per reviewer kind per run. The user-directed
stop below ends a run without being a row, the `<REVIEW_LIMIT>` cap below ends one without being a
row, and an interrupt ends one without reaching Step 4 at all.

Row 0 is an abort rather than a stop, so the run ends there without a Final Report. Its Step 0
trigger aborts before any launch. Its other two triggers fire in Step 1, as soon as the last launch
has failed or an impossible result is aggregated, and in both cases before Step 2 applies a single
fix.

This table is not a termination proof. One iteration's fixes can introduce a defect the next
iteration finds, and that cycle can sustain itself. Row 2b bounds how long it can run on polish,
and nothing bounds it on substance. A run that is going nowhere on substance is stopped by the
user, which is why every iteration that reaches Step 3 emits a per-iteration summary.
`iteration` is a label for the announce line, that summary, and the Final Report header. The
`<REVIEW_LIMIT>` check below is the one rule that reads it. Do not add a cap row, a cap this skill
chose for itself at any size, a periodic check-in, or an oscillation detector, and do not renumber
the rows that remain.

**The `<REVIEW_LIMIT>` cap is a terminal state and it is not a row.** When `<REVIEW_LIMIT>` is 1 or
more and the table above picked a row that goes back to Step 1, and the iteration just finished is
the `<REVIEW_LIMIT>`th, stop instead. Reach Step 4 and write the Final Report. The check runs after
the table, never instead of it, so a run that was going to stop on row 1, 1c, 2 or 2b still stops
there and reports that ending rather than this one. A cap of 1 therefore ends the run after one
iteration whatever the table said, which is the cap doing exactly what the number asks.

That value comes from the invocation, and this skill never picks it. The ban in the paragraph above
is on a cap nobody asked for, because a run cut short by its own arithmetic reports less coverage
than it had reason to and nothing tells the user why. A cap the user typed is the user spending less
on purpose, which is theirs to decide and gets disclosed on the Outcome line. Those are opposite
situations and the second one does not reopen the first.

**A run the cap stopped is never clean.** It stopped while the loop still had somewhere to go, and
what it would have found in the iteration it did not run is unknown rather than absent. A run that
carried a cap and stopped on row 1 before reaching it is a different run, and it reports row 1.
[FINAL-REPORT.md](FINAL-REPORT.md) carries the Outcome line, which is its own line and not the
user-directed stop's. The two endings are both the user choosing to spend less, and they are not
interchangeable. One was decided before the run started, and the other after seeing where it went.

Remaining Issues stays gated on its own content, which is a surviving finding that did not become a
commit. A capped run whose last iteration committed every finding it kept has nothing for that
section and omits it, the same as any other run would. The cap is reported on the Outcome line and
not by an empty section, because a section listing nothing says the run found nothing left, and what
this ending actually knows is that it stopped before looking again.

**The user-directed stop is a terminal state and it is not a row.** The user is shown where the run
is going and says to stop, in band, and that run does reach Step 4, which is the operative
distinction from an interrupt. [FINAL-REPORT.md](FINAL-REPORT.md) carries its Outcome line. Use
that rather than bending row 1c or row 2 to fit, and rather than naming a stop of your own. The
nine logged runs behind this rule are in the run-log notes.

**Ask, do not decide.** Two triggers, and either one fires the ask. When `self_inflicted_count`
has been the majority of the kept findings for three consecutive iterations, or when
`any_logic_change` has been false for two consecutive iterations, put that in front of the user
before launching the next one, and let them choose. Neither trigger waits for the other, and the
first does not wait for `any_logic_change` and `any_test_change` to go false.

The second trigger is the prose lane's exit. Two iterations that committed prose and tests and no
logic mean roles 1, 5, 9 and 11 are reading what this run wrote, and the rootcause lines of the
run that motivated the trigger say what that reading produces: a rewrite of a rewrite. That run
went nine iterations with no logic commit after iteration 1, at about $55 of roles each plus the
orchestrator, with five loci rewritten four to six times, and the first trigger did not fire
until iteration 9 because the majority dipped below half in iterations 5 and 6 while the absolute
self-inflicted count kept rising, 6, 6, 8, 9, 9, 8. The second trigger would have asked after
iteration 3. Read `any_logic_change` from the iterations' own `any_logic_change=` lines, so the
count survives a compaction.

The ask carries four things, so the decision is made on yield rather than on a percentage alone:
the per-iteration self-inflicted counts, the spend so far, the last three iterations' `severity`
lines verbatim, and the number of `origin=branch` commits with `class=logic` since the previous ask
(or since iteration 1 on the first ask), with their titles. That last number is what the run is
still finding in the branch as opposed to in its own output. Measured on a twelve-iteration run
that asked after iterations 4, 7, 9 and 12, the windows between asks held 2, 2 and 4 such commits
at $73, $54 and $148 of spend, most of them role 11 naming call sites the fix's intent implied. The
yield was real and its price varied threefold, and both numbers are the user's to weigh.

It used to. The first version of this rule required the majority AND both booleans false, and one
measured run then went fourteen iterations at about $57 each with self-inflicted findings at 70 to
95 percent from iteration 2 onward, and the ask never fired. Every iteration committed one small
change the classifier correctly called `logic`, a moved statement, a helper extraction, a
`deleted_at` clause, a counter tag's position, so `any_logic_change` was true every time and the
gate stayed shut while the orchestrator wrote "self-inflicted findings were the majority from
iteration 3 onward" into its own Final Report. The passage below already says `self_inflicted_count`
is the direct signal and the booleans only corroborate it. Requiring the corroboration inverted that,
and it cost roughly $500 of the run.

**Under `<FULL>`, neither trigger asks.** The user said up front that the run ends on a stop row,
the cap, or an interrupt, and that answer covers every ask this section would make. When a trigger
fires, append `ask-suppressed iter=<N> trigger=<majority|no-logic> self_inflicted=<counts> spend=<so far> branch_logic_commits=<n>`
and launch the next iteration without `AskUserQuestion`. The four facts the ask would carry still
go in the per-iteration summary, so the signal is on record for the Final Report, and the counter
restarts as it does after a "continue". This covers these two asks and nothing else. Step 0's
dirty-tree question, a missing Jira reader, an untestable finding and the second reopen of a locus
still go to the user, because each asks what to do with this iteration's work and none asks
whether to stop. `<FULL>` also rules out the sentence "A recommendation to stop" below describes,
while the loop runs.

If the user says continue, the signal keeps showing, and the ask fires again after three more
consecutive majority iterations, or two more with no logic commit. That is gated on the signal and not on a schedule, so it is not the
periodic check-in the paragraph above forbids.

Presenting a signal is not one of the four forbidden things. It counts no iterations, fires on no
schedule, and stops nothing by itself, and the paragraph above already makes the user the exit for
this case. What is forbidden is ending the run on that signal yourself. A plausible stop nobody
authorized is harder to argue with than a bad one.

The finding count is not the signal, because it falls monotonically the whole time this lane runs,
so a run can look like it is converging while it eats its own output. The lane's only exits are
row 1 once roles 1, 5 and 11 accept what the run wrote, row 2 once an iteration commits nothing,
and the user, by the no-logic ask above, by direction or by interrupt.

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
`db`, `security`, or `error-handling` finding survives at any severity, because those are
the categories where a miss ships rather than costing a pass. A run that reaches row 2b is not a
clean result. See [FINAL-REPORT.md](FINAL-REPORT.md) for how it reports.

Computing the next active set, for every row that goes back to Step 1 (1b, 4, and 5):
- **Row 1b (retry):** the active set is the short kind ONLY. `<ACTIVE_ROLES>` = the roles this
  iteration ran if the `in-depth-review` kind fell short, otherwise empty. `<ACTIVE_GH_STYLE>` =
  true iff the `gh-style-review` kind fell short. The short kind relaunches at full multiplicity,
  and why the reviewers that did report are not relaunched is in [RATIONALE.md](RATIONALE.md).
- **Row 4 (full):** `<ACTIVE_ROLES>` = roles `1..9` and `11`, the iteration-1 set. `<INSTANCE_2_ROLES>` =
  `{}` by default, or the orchestrator's choice per the discretion rule below.
  `<ACTIVE_GH_STYLE>` = **false**. A full rerun is a full run of instance 1. It is not a return to
  the iteration-1 roster, and gh-style is not part of it.
- **Row 5 (pruned):** `<ACTIVE_ROLES>` = the in-depth role numbers in `productive_reviewers`,
  unioned with `{9}` when `any_test_change` is true. `<INSTANCE_2_ROLES>` = the orchestrator's
  choice per the discretion rule below, default `{}`. `<ACTIVE_GH_STYLE>` = **false**.

  **The floor: when `<ACTIVE_ROLES>` would otherwise be empty, set it to `{2}` and run instance 1
  alone.** An empty active set finds nothing, and an empty findings list with `reviewer_unavailable`
  and `roles_missing` both empty is row 1, so the run would stop and report `complete` coverage with
  no reviewer having read the final tree. The bug scan is the cheapest lens that reads the whole diff
  for behaviour, which is what a final-tree check is for. gh-style used to be the floor. It is not
  any more, because it runs in iteration 1 only and a row-5 iteration is never iteration 1.
  **Compute the role-9 union HERE**, so the retry union below can still add a short kind back and the
  `reviewer_unavailable` subtraction below can still remove role 9 along with the rest of an
  unavailable `in-depth-review` kind.

- **Role 10 needs no rule here** because it is not in any set this skill builds.

- **Instance 2's discretion rule, from iteration 2 on.** `<INSTANCE_2_ROLES>` defaults to `{}`,
  so no second instance runs. The orchestrator may replace that with any set, and the only
  constraint is that the set and a one-line reason go in the run log before the launch, as
  `instance2 iter=<N> roles=[...] because <reason>`. A reason names something observed this
  iteration, such as "role 2 raised the only sole-instance logic fix and the new commit rewrites
  that SQL, second read on 2 and 9" for adding roles. Keeping the default needs no reason beyond
  `default`, and the line is still written, so a reader can tell a considered `{}` from a forgotten
  one. "Default" is not a reason for a non-empty set. On nine logged iterations after an
  iteration 1, a `{11, 9, 2}` kept by default added nothing instance 1 had not raised, which is why
  the default moved. The
  discretion is real and the record is mandatory. An unrecorded choice is the same defect as an
  unrecorded stop, and the ask rule above exists because of how that went.

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

**What a pruned `test` iteration gives up.** Roles 1 and 5 are not unioned in, so a pruned
rerun judges new test code through role 9 alone. IMPLEMENTER.md's staged-diff scan is the partial
backstop for the mechanical subset and is not a substitute. The next `logic` commit forces row 4
and they see the accumulated test code then. Cost trade: [PRUNING.md](PRUNING.md).

**The pruned `prose` lane is absorbing.** A `prose` commit usually hands roles 1, 5 and 11 their
own output. Row 4 needs a `logic` commit and a prose-only iteration has none, so no rerun row can
end this lane. Its exits are the ones the signal above names, and the no-logic ask is the one
written for it. Full analysis:
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

Before the Spend block, run the usage filter from Step 3 one last time and append the lines the
hook could not have seen, the final iteration's gh-style and scorer, skipping any `id=` already in
the log. Then delete the marker:

```
rm -f ~/.melvin/config/logs/review-and-fix/.active-<session-id> ~/.melvin/config/logs/review-and-fix/.active-<session-id>.seen
```

Append the report to `run_log_path` too, then tell the user where the log is. That path is the last
line of the run, so it is there whether they want to read the run back or hand several logs to
another agent. A run that aborted on row 0 gets neither, because nothing was reviewed and the log
holds only its header. It still deletes the marker.

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
- **Multiplicity is fixed while a kind is active.** in-depth-review at two instances, the second
  narrow per Step 0, and gh-style-review at one pass in iteration 1 when `--gh-style` turned it on.
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
- **One commit per class group.** Never squash or amend. Prose fixes share one commit, test fixes
  share one, and logic fixes share one per locus, as Step 2 says. A commit that mixes classes has
  one `class`, and one logic hunk among six prose fixes makes the whole commit `logic`, which fires
  row 4 and hides the prose and test work from row 5. One run bundled that way in every iteration
  and row 5 never pruned once in ten. One commit per finding, the opposite extreme, made 22
  commits in an iteration and cost $121 of orchestrator time for it.
- **Never commit broken code.** Lint and tests must pass before committing.
- **Never push.** Only local commits, and the user decides when to push.
- **Ask before acting on ambiguous findings**, with `AskUserQuestion`.
- **Respect project AGENTS.md rules.** Read the mandatory checklists before committing.
