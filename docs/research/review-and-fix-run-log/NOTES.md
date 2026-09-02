# The review-and-fix run log, and when to build phase 2

Written 2026-08-25, when phase 1 shipped. Phase 2 is deferred on purpose. This note records the
trigger for starting it and the checking already done, so nobody redoes either.

## Contents

- Why the log exists
- What phase 1 records
- Why phase 2 is deferred
- What was checked, so it does not get rechecked
- The trigger
- The first read, 2026-08-26
- The third log, and the finding that outranks the rest
- What the first read established about long runs
- The number to watch first
- The measured run census, fifteen findings that each justify a rule in SKILL.md. Read the
  charter subsection before editing SKILL.md.

## Why the log exists

Runs took 7 and 17 iterations. Three separate retrospectives explained why, and each was written
from recall after the fact. Each got something wrong. One asserted a grep that passes before any
edit. One asserted a mechanism the prompt files do not contain. One computed a comment-line share
that no longer exists to check. A fourth claim, made while reviewing those three, put four commits
where `git log -L` names ten.

The pattern is not carelessness. Nothing was measuring, so every account was a reconstruction, and
a reconstruction of a long run is a story.

## What phase 1 records

`review-and-fix` Step 0 opens one markdown file per run and appends to it as the run goes. Step 3
appends each per-iteration block as it emits it, byte for byte. Step 4 appends the Final Report.

Home is `~/.melvin/config/logs/review-and-fix/` when Step 0's first `mkdir` returns 0, and
`/tmp/claude-skills-logs/review-and-fix/` otherwise. The Final Report names the path, so which home
a run used is readable rather than inferred.

Appending live rather than assembling at the end is the load-bearing part. The loop has no
iteration cap, so a run that is not converging ends when the user interrupts it, and an interrupted
run never reaches Step 4. Before the log, that run's only trace was chat scrollback plus `git log`,
and it is the run most worth reading.

Three fields are new rather than copied from the existing chat blocks. Three timestamps per
iteration, giving waiting time and fixing time separately. A `git blame` per finding against
`run_commits`, giving `self_inflicted_count`. A rollup table in the Final Report.

## Why phase 2 is deferred

Phase 2 is a structured format for cross-run analysis. It is deferred because no logs exist yet, so
any schema would be fitted to a guess about what the questions turn out to be.

Structure can be added to markdown later. Data that was never recorded cannot be added to runs that
already happened. So the decision that could not wait was whether phase 1 is losing something
unrecoverable.

## What was checked, so it does not get rechecked

The headline phase-2 question is which reviewer lens generates the most churn. That needs a finding
category cross-referenced against the churn count, and `SUMMARY.md` records no per-finding
category. Titles only, in the commit table.

It is still answerable. Every block's last bullet records the row that fired and the next
iteration's active set, and on a row 5 pruned rerun that set is `productive_reviewers`, meaning the
roles whose findings were fixed. Read against `self_inflicted_count` on the same block, that gives
churn by lens at iteration granularity, which is the granularity the question needs.

So nothing was added to the commit table. Widening it would also have fought the rule in
`SUMMARY.md` that keeps every cell either git output or a string a reviewer wrote, which is what
stops the table describing work that did not happen.

These were checked the same way and are already recorded: where the wall clock went, whether
coverage degraded, how one repo or branch compares to another, and which finding titles recur
across runs on the same branch.

## The trigger

Read the log after the next `review-and-fix` or `work-on` on a branch of real size. One test.

**Does it answer what took so long without reconstructing anything?**

If it does, phase 2 may never need to exist. A small analysis prompt pointed at the log directory
covers cross-run work, reporting churn share per run, the waiting-versus-fixing split, and which
productive sets sit beside high churn. An agent reads markdown.

If it does not, whatever was missing is the phase-2 spec. Write that down before designing a
format. If markdown parsing is what gets in the way at that point, JSONL earns its place then, and
by then the fields that matter are known rather than guessed.

Three or four logs is enough to tell, and they have to be three or four USEFUL logs. See below.

## The first read, 2026-08-26

Two logs arrived. The instrument failed its own trigger test, and the failures are enforcement
rather than design.

`Where the run went`, the section written to answer the trigger question, appeared in neither log.
A stamped `t1` was usable in one iteration out of four, while every `t2` was exact, because `t2`
falls where the orchestrator is acting and `t1` fell where it was waiting with no observable moment
to stamp. `self_inflicted_count` produced one informative value across four iterations, twice
reading zero by construction on branches a prior run had already churned. Each of those has a fix
now in `SKILL.md`, `SUMMARY.md`, `STATE.md`, and `FINAL-REPORT.md`.

The logs are also more trustworthy than the four hand-written retrospectives that preceded them.
Their emphatic claims were spot-checked and held. The failure was that a number was replaced by an
adjective wherever the number was unavailable, which is a fixable thing.

**The sample is weaker than two logs.** Same repo, same operator, same spec version, same session,
started four seconds apart, with a spend limit already live. Both were second invocations on
branches a prior loop had churned, one with 12 of 25 in-range commits from that prior loop and one
with all 7. Neither reached a stop the protocol defines, and each stopped on a criterion its own
orchestrator invented mid-run. Both
report roughly 21 minutes of iteration-1 waiting over the same wall-clock window, so the two
numbers are not additive and neither is attributable.

So waiting needs three specific conditions rather than a count, because more of the same sample
answers nothing:

1. **One first invocation on a branch no loop has touched.** Every churn reading in the sample is
   confounded by being a second pass over prior-loop output.
2. **One run that reaches a protocol-defined stop**, meaning row 1, row 1c, or row 2. Zero of three
   runs did. Whether those rules work is the whole question, and the sample never observes them.
3. **One run with no concurrent sibling**, so waiting time is attributable to a run.

## The third log, and the finding that outranks the rest

A third log arrived the same day, from a run that started after the enforcement fixes landed, so it
is the first log written under the corrected spec. Branch `ml/stripe/webhooks/GRO-18015`, seven
iterations, one commit in range at the start.

It satisfies two of the three conditions the trigger section asks for. A first invocation on a
branch no loop had touched, and no concurrent sibling. It does not satisfy the third.

**The respecified stamps worked for iteration 1.** Its three instance arrivals each carry an exact
time and `t1` is derived as the last of them, which is what the respec asked for. `commits in range
at t0` landed for the whole run and measured range growth for the first time instead of inferring
it, at 1, 7, 10, 13, 15, 16, 18 commits across the seven iterations.

**Then every stamped field decayed together, from iteration 2 onward.** Of eight arrival lines, 3
carry a timestamp, all of them in iteration 1. Every `t2` after iteration 1 landed on `:00` seconds,
and five consecutive iterations started before the previous one ended.

That is one cause rather than several. The instruction being broken is not any stamp's definition. It
is "append each stamp on its own line the moment you take it". Iteration 1's block was written live
and the rest were written in arrears, where a time has to be remembered instead of read. So the
lever is whatever makes falling behind impossible or visible, not further precision about what each
stamp means. The cross-iteration ordering check is a step toward the visible half, because a
reconstructed pair usually fails it.

**The prose lane fired, was recognised by name, and could not be acted on.** Iterations 5 and 6 both
carried `any_logic_change` and `any_test_change` false, and the run cites "the prose lane the skill
names" and later "gives row 2 a real chance rather than letting the prose lane run indefinitely".
The diagnosis this note was written to enable now works. The loop still has no exit to take once it
reads.

### Every observed run stops on a criterion it invented

Four runs, and rows 1, 1c and 2 have never fired in any of them. This is the finding to act on.

The stops were not external interruptions. Each orchestrator decided to stop and did. The third log
says `STOPPED HERE, on judgement, not on a rule` and records "Row 2 never fired. Every one of seven
iterations found something real, so the loop had no natural exit." The first log deviated earlier and
labelled it, `Step 3: DEVIATING FROM ROW 4, deliberately`. The `GRO-17927` retrospective is plainest,
"What ended the run was not the stop condition. It was a rule I set and then held to."

So `SKILL.md` names three stops and forbids a cap, an iteration count, a periodic check-in and an
oscillation detector, and a fifth stop is being added at runtime anyway, four times out of four.

An invented stop is worse than a specified one on three counts. Its criterion varies per run and is
recorded in no field. No Outcome line fits it, which is why the third log carries no Final Report at
all, since the template offers clean batch, incomplete coverage, or converged and none of them
describes "the residue was polish". And the prohibition is being broken silently rather than argued
with.

Step 3's prohibited list does not name a severity gate. A severity gate plus a narrowed scope is
what two of these four runs actually used. That is the shape worth putting to a human, and it is a
decision rather than a fix.

## What the first read established about long runs

Recorded because it corrects the diagnosis this note was written under.

The absorbing prose lane did not fire on the `GRO-17927` run. Its finding count did not fall, and
the run appears to have taken row 4 on every iteration, so the churn counter, the two booleans and
the three-consecutive-false test would each have read normal throughout.

Read that as inference rather than as measurement. That run predates the log, so no field recorded
which row fired. The row sequence was reconstructed from commit file lists cross-checked against
launch counts, and the finding series exists only in a hand-written retrospective whose numbers
were not all reproducible. The claim worth relying on is the narrower one, that nothing in the
sample shows the prose lane firing.

The mechanism with the most support is fix quality under review pressure. A repair introduces the
next iteration's findings, and the record is in the commit bodies rather than in anyone's recall.
On that branch 2258 of 2451 insertions are repair, so the growing review range is largely the same
variable counted twice rather than an independent cause.

The narrow-scope phase is the strongest single data point and it points the opposite way to how it
was first read. Cutting scope 12x did not reduce churn. Self-infliction ran near 100 percent inside
that phase. What it reduced was detection surface, until the residue cleared the operator's
severity gate and the run was declared done.

One recommendation was considered and rejected. Re-scoping the blame check from the run's commits
to the whole branch saturates. On these branches it would read near 100 on every run that follows
any prior loop run. Git cannot separate loop-authored from human-authored text here either, because
every commit on that branch carries one author and one identical `Co-Authored-By` trailer, the
feature commit included.

## The number to watch first

`self_inflicted_count` against the finding count, per iteration.

Findings falling while churn climbs is the prose lane, described under "What a pruned `prose`
iteration gives up" in `SKILL.md` Step 3. Nothing shipped so far cuts that lane. The claim sweep
and the staged-diff scan shorten it by reviewing authored prose before it lands, and the lane stays
absorbing because a `prose` commit sets no flag and row 4 needs one.

The candidate that would cut it is freezing a lens's review range to Step 0's `HEAD` instead of a
re-resolving one. As designed it was narrow. Record `BASE_TIP` at Step 0, and pass the frozen range
only to a row 5 pruned rerun whose in-depth role set is a subset of roles 1 and 5, while every other
lens and every full rerun keeps the live range. That gate is written here because an earlier version
of this note recorded the candidate in one sentence with no gate at all, and a reader picking it up
from that sentence would have built the broad version.

It stays declined, and the reason is the coverage vocabulary rather than the mechanism.
`FINAL-REPORT.md` computes Coverage from three inputs, a non-empty `reviewer_unavailable`, an
outstanding kind shortfall, and a non-empty unioned `roles_missing`. None can express "role 1 ran,
but against a frozen base". So a frozen-range rerun reports `complete` while a lens never read part
of the shipped tree, and the section whose own rule is to name every role involved has no way to
name the gap. That is a decision about the honesty contract.

Two later findings weakened the candidate further, and both are recorded so it is not re-proposed
from the original argument. Freezing a range narrows which hunks are nominated and not which bytes
a lens may read, and roles 1 and 5 open files, so a clause-joiner authored in an earlier iteration
inside a file already in range is still found. And a stronger version of the same idea was run by
hand on the `GRO-17927` branch, where scope was cut to the newest repair commit alone. Three
consecutive narrow passes each found a real defect introduced by the pass before it.

## The measured run census

Fifteen findings that each justify a rule now stated at its act site in
`shared_config/.claude/skills/review-and-fix/SKILL.md`. They live here rather than in the skill so
the skill body stays near the authoring standard's 500-line guidance, and because an executing
agent needs the rule while only a maintainer needs the evidence.

### The charter: why an act-time field is specified inline

The `stamps:` line replaced a summary table that was specified three separate ways and produced
zero rows across seven runs. Every field that landed in those runs was an inline imperative in
SKILL.md at the moment of the act. Every field that did not was specified elsewhere for later
assembly.

Treat that as a constraint on editing SKILL.md. Moving an act-time imperative into a reference file
is the one edit this record forbids.

### Two working-tree losses

One reviewer reverted a source file mid-review and concurrent roles read the reverted state.
Another ran `git checkout`-style cleanup, reported "Worktree is now clean", and discarded the
user's own uncommitted edits with no stash entry to recover from.

Both agents were trying to run a negative control, which is a good check that needs the tree. That
is why negative controls belong to the orchestrator, in the fix phase.

### Where the run log lives, and why it is outside the repo

The home is deliberately outside the repo under review. Step 2 commits with `git add -A`, so a log
inside the working tree would land in a fix commit and the next iteration would review it.

### Why `t1` is not a duration boundary

A straggler's report can arrive after fixing has already begun. That put two measured iterations at
a fixing time of `0m00s` while `git` shows two commits landing inside each. `t1` stays as the
coverage record of when collection finished.

### A fix applied while a reviewer was still reading

One iteration applied a fix between the gh-style arrival and both in-depth arrivals, and the Final
Report had to caveat that an instance read a tree mutated mid-review. Coverage was `complete` on
every other axis, so the caveat was the only trace.

### Nine runs that could not price the second in-depth pass

None recorded which instance a surviving finding came from, so whether the second in-depth pass
earns its cost was unanswerable from the logs.

### Three blind spots in a per-role-yield proxy

Inferring per-role yield from which roles the next pruned set contains is blind to a role whose
finding was dismissed, blind to a role that did its whole job on iteration 1, and blind to a
category that maps to no role at all. All three have happened.

### A six-iteration run's log gaps

It emitted summaries for iteration 1 only, wrote per-sha classes inline for iteration 3, nothing at
all for iteration 4, and counts for iterations 5 and 6. Iteration 4's nothing is what the rule
bans, because the next iteration's stop decision reads the class.

### Why the loop is not a termination proof

One iteration's fixes can introduce a defect the next iteration then finds, and that cycle can
sustain itself. Row 2b bounds how long it can run on polish. Nothing bounds it on substance.

### Nine logged runs, four invented stops

Two runs ended by user direction and wrote an outcome by hand. Four stopped on a criterion their
orchestrator invented and no row defines, and one of those four named it `converged-on-self`.

The rows were never the problem. The graceful human exit had nowhere to land, so each run built its
own. FINAL-REPORT.md now carries that Outcome line.

### Every invented stop was a defensible reading

Each of the four was a defensible reading of real evidence, which is what makes them worth banning.
A plausible stop nobody authorized is harder to argue with than a bad one.

### Seven runs in which no defined stop ever fired

Across seven runs no stop the Step 3 table defines ever fired, and every one of those runs stopped
anyway on a criterion its own orchestrator invented and recorded nowhere. One announced row 1c and
denied that row's own precondition in the next clause. That is why row 2b was added. An unwritten
stop varies per run, has no Outcome line, and cannot be audited.

### The stop recommendation the class gate would have caught

An orchestrator recommended stopping on the premise that the two unreviewed commits were prose
corrections. Neither had a derived class, both were `logic`, and one had restructured error
propagation across a function boundary.

The base rate pointed the other way the whole time. Each of the three prior iterations had found
defects in its predecessor's own logic, ten of them in one case.

### Role 11 and the `Promise<void>` reconstruction

One iteration fixed two role-11-shaped findings by changing a function's return type from
`Promise<void>` to an error union, converting two throws into returns, and moving a control-flow
gate that decides who receives a one-shot email.

Both commits were `logic`. Both were reported to the user as corrections to prose, because the
findings had been about claims, and the stop recommendation that followed was wrong.

### The wrap that defeated a line-oriented claim sweep

The claim sweep in Step 2 sub-step 4 searches by a distinctive fragment with `rg -U`, or with `\s+`
standing in for every space. That is not fussiness. Prose in these files is hand-wrapped near 100
columns, so a phrase that reads as one sentence is frequently split across two lines, and a
line-oriented search for the literal phrase returns zero hits.

Zero hits reads exactly like a clean sweep. That is how the miss this rule exists to prevent
actually happened. Anyone tempted to simplify the rule back to a plain `grep` for the phrase should
note that the simplification is silent in the direction of passing.
