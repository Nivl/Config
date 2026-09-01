---
name: work-on
description: >
  Takes a Jira ticket key and drives it end to end. Validates the ticket is still real against
  the code at HEAD, the merged PRs the branch does not contain, the open PRs that overlap, and the
  related Jira tickets that may already duplicate it or have carved scope out of it.
  On a bug or a security ticket it also establishes the real impact and the effort to fix, so a
  fix that is not worth doing gets caught before any code is written.
  Rewrites the ticket from what it found. Then hands off to brainstorming or
  systematic-debugging, commits, pushes, runs review-and-fix to completion, and opens a draft PR.
  Use this skill when the user says "work on WMP-837", "/work-on ABC-123", "pick up this
  ticket", "start on this ticket", "take this Jira ticket end to end", or hands over a Jira key
  or Jira URL and expects the whole loop rather than one step of it. Also use it when the user
  wants a ticket checked for staleness, or for whether it is worth doing, before any code gets
  written ("is this ticket still valid", "is this bug worth fixing", "how bad is this security
  issue really", "does this ticket still reproduce"). Do not use it for a ticket the user only
  wants read or summarized, and do not use it to fix an existing PR (that is fix-pr).
  Accepts two modifier flags. `--lite` skips the validation phase and `--fast` skips that plus
  review-and-fix. Every staleness and worth-fixing use above needs the full run, because neither
  flag validates anything, so route such a request to the unflagged pipeline even when the user
  asks for speed.
---

# Work On

One Jira ticket, from "is this still true?" through to a draft PR.

The ticket is the least trustworthy input in the whole run. It was written at some past moment,
against a tree that has since moved, by someone who may have been guessing. Everything before
Step 5 exists to find out what is still true. Everything after Step 5 is ordinary development on
a premise that has been checked.

**Both flags in "Argument" below cut into that first half, so on a flag run the premise is not
checked.** The second half proceeds anyway, on the ticket exactly as filed. That is the trade the
flags exist to make, and every sentence in this file that justifies a later step by pointing at the
validation phase is qualified by it.

## Pipeline

| Step | What happens | Writes anything? | Cut by |
|---|---|---|---|
| 0 | Preflight. Ticket key, branch, tooling. | no | never |
| 1 | Gather ground truth once, in the main thread. | no | reduced by `--lite` |
| 2 | Validate. Inline when the surface is small, seven parallel lenses plus the telemetry probes and the triage agents when it is not. | no | `--lite` |
| 3 | Ask only what investigation could not settle. | no | `--lite` |
| 4 | Verdict gate. Valid, invalid, superseded, or partial. | only if the user picks option (a) | `--lite` |
| 5 | Rewrite the ticket. Status to In Progress, description in place, plus a validation comment. | **Jira** | reduced by `--lite` |
| 6 | Do the work via brainstorming or systematic-debugging. | local files | never |
| 7 | Commit everything, push. No PR yet. | **remote** | never |
| 8 | `review-and-fix`, every iteration, no early stop. | local commits | `--fast` |
| 9 | Push, then `open-pr --draft`. Always a draft, never a question. No PR opens at all when Step 8 could not run, which is a Step 8 that broke and never a Step 8 the user skipped. | **remote** | never |
| end | "Final report". PR URL, write verification, one line per shipped `TODO(user):`. | no | never |

**`--fast` cuts everything `--lite` cuts, and Step 8 as well.** The column names the narrower flag
where both apply, so a row reading `--lite` is cut by both. This column is the per-step map, and it
is what wins if anything else in the file describes a step differently. What each flag means, why,
and what it costs is "Argument" below.

**Nothing writes anything a human reads before Step 5.** No Jira edit, no commit, no push, no PR. That is
what lets the validation phase be paranoid, and Step 4's option (a) is the single exception, a comment the
user explicitly asks for after seeing the evidence. On a flag run the property holds trivially, since
the phase it protects does not run and option (a) can never be offered.

**From Step 5 on, writes do not stop to ask.** The ticket, the commits, and the PR all go out without a
preview, because the validation phase in front of them is what earns that. The rule is that this skill
**asks about decisions and never about wording**. Step 3's questions and Step 4's a/b/c pick are
decisions and both stay. A rendered description, a comment, a commit message, and a PR body are
wording, and each is written, verified after the fact, and accounted for in "Final report".

**On a flag run the thing that earns those ungated writes is the flag itself, and nothing else.** The
paragraph above points at the validation phase, and a `--lite` run does not have one, so the
authorization has to come from somewhere. It comes from the user having asked for this mode on this
ticket, which is a narrower warrant than a validation phase and covers exactly the same writes.
Two consequences follow and neither is optional. Every one of those writes still happens without a
preview, because a flag that skipped the checking does not get to also start asking. And "Final
report" becomes the run's only account of what went out, so the reporting rules there tighten rather
than relax. A `--lite` run also asks the user nothing at all between Step 0 and Step 6, because both
surviving decisions named above live in steps it skips.

The "no" column above is about those artifacts, not about every remote call. Step 2 reads Datadog and
Amplitude when the ticket claims something about production, which is a read against live telemetry.
The warehouse depends on what is installed. This skill has no connection of its own, so it looks for a
skill that runs SQL and asks the user when there is none. See [DB-QUERIES.md](DB-QUERIES.md). A query
that does run is a read, and it is bounded and `SELECT`-only either way. All of that belongs to Step 2
and Step 3, so a flag run makes no telemetry read and no warehouse request of any kind.

## Argument

One positional argument, the ticket key, plus optional modifier flags in any order.

**Strip the flags before resolving the key.** Step 0 validates what is left against
`^[A-Z][A-Z0-9]+-[0-9]+$` and stops when it does not match, so a flag still sitting in the argument
either fails that check or gets mistaken for a missing key. Order does not matter once they are
stripped, and `work-on --lite WMP-837` is the same invocation as `work-on WMP-837 --lite`.

**The run never picks a flag on its own.** Both flags remove controls, and which controls a ticket
can afford to lose is the user's judgement rather than this skill's. A long validation phase is not
a reason to reach for one. Neither is a ticket that looks obviously true, because looking obviously
true is exactly what a stale ticket does.

### Modifier flags

- **`--lite` skips the validation phase.** Steps 2, 3, and 4 do not run at all. Step 1 reduces to
  the ticket read and the repo conventions. Step 5 reduces to the status move. Steps 6 through 9 run,
  `review-and-fix` included, with the two carve-outs named under "What still runs" below.
- **`--fast` skips everything `--lite` skips, and Step 8 as well.** `review-and-fix` does not run.
  Step 9 still pushes and still opens the draft PR.

`--fast` implies `--lite`. Passing both is not an error and gets no warning, because `--fast` is a
superset of it and the wider skip wins.

**This section is the single copy of what the flags mean, and the Pipeline table's `Cut by` column is
the per-step map.** The two bullets above summarize that map in prose, and where the summary and the
table ever disagree the table wins, since it is the one that goes step by step. Every step below
carries a one-line guard pointing back here rather than restating its own skip condition, since a
second copy is what this file says gets half-overridden. The one thing a guard may add is a carve-out
belonging to that step alone, and there are exactly two, both in "What still runs".

**Announce the mode before Step 0 does anything.** Name the flag, the steps it cuts, and the two
costs under "What the flags cost" below. Keep it short, but do not compress it into one line at the
expense of the costs, since those are the whole point of announcing. The user is about to watch a run
that asks nothing and writes to Jira anyway, and this is what tells them that is the flag working
rather than the validation phase failing silently.

**The duplicate-link note under "What Step 1 keeps" is a second, later disclosure, not part of this
one.** Its input does not exist yet here. Step 1 has not read the ticket when the announcement goes
out, so that note goes out when the read lands.

#### Step 0 is never reduced

Nothing in Step 0 is validation. The branch check, the dirty-tree stash, the stash ownership rule,
`git fetch`, and the Jira and GitHub access checks are damage control, and a flag run needs them more
rather than less. `--fast` commits, pushes, and opens a PR with nothing having reviewed the code, so
the checks that stop it doing that on the default branch or over somebody else's uncommitted work are
the last ones standing.

Two things inside Step 0 differ on a flag run, and both are marked at their own line. Item 1 strips
the flags, per the rule above. And the GitHub-access fallback question is not asked at all, because
everything it offers to trade away is Step 2 evidence a flag run was never going to gather. That is
the one question Step 0 drops. Every check in it still runs.

#### What Step 1 keeps, and why those two

Two things survive, and each has a consumer that still runs.

- **The markdown ticket read**, description and comments both. It is the specification for Step 6.
  Read the comment thread even here, because a ticket narrowed or abandoned in its comments while the
  description still says the original thing is a difference Step 6 has to build against.
- **The repo conventions**, the root and nearest sub-project `AGENTS.md` and `CLAUDE.md`. Step 6
  writes code against them, and under `--lite` Step 8 reviews it against them too.

Everything else in Step 1 exists to feed a step that no longer runs, so it is dropped rather than
gathered and discarded:

| Dropped | Its only consumer |
|---|---|
| the `adf` read and the `-original.adf.json` rollback save | Step 5's description rewrite, which no longer happens |
| the branch-commit list | Step 2 |
| the missing-commit list, the stat, the commit-to-PR mapping | Step 2 |
| the open-PR sweep | Step 2, and Step 4's superseded path |
| the related tickets, both the linked half and the JQL half | Step 2, and Step 4's duplicate path |
| `{ kind, kind_because }` | Step 2's triage agents |
| `{ datadog, amplitude }` | Step 2's telemetry probes |

The rollback artifact is the one worth naming out loud. It exists so a botched description rewrite on
someone else's ticket can be put back, and under both flags there is no description rewrite to botch.
The status move needs no rollback, since the transition either took or it did not and the transition
list says which.

**The dropped rows are dropped as work, not as facts.** Nothing here licenses claiming that no open
PR overlaps or that no ticket duplicates this one. Nobody looked.

**One exception, because it costs nothing.** The surviving ticket read already uses
`fields: ["*all"]`, so `issuelinks` arrives in the payload whether or not anything asks for it. If a
link of type `duplicates` or `is duplicated by` is sitting in it, **say so in one line the moment that
read lands**, naming the key. Not in the mode announcement, which has already gone out by then and
could not have known this. That is a disclosure and not a gate. It does not stop the run, it does not
become a question, and nothing downstream reads it. It exists because working a ticket somebody has
already marked a duplicate is the worst thing a `--lite` run can do, and the evidence for it is
already in hand.

#### What still runs, with two carve-outs

Steps 6, 7, and 9 run unchanged. Step 8 runs unchanged under `--lite` and not at all under `--fast`.
Two things that normally happen inside a surviving step do not, and each one is a control that lost
its producer rather than work the flag meant to skip.

**Step 8 does not file follow-up tickets under `--lite`.** Its "Follow-ups from what review-and-fix
left" subsection classifies a leftover against the scope agreed at Step 4, and under `--lite` no scope
was ever agreed, so the predicate that decides what is out of scope has no value at all. Its filing
path is also a by-name reuse of a subsection that lives inside Step 4, which carries the points gate
and the approval. A Jira create has no rollback, so an undefined predicate reaching it would produce
an unapproved write rather than merely stale prose. That is one of two places these flags could make
a run less safe instead of less thorough. The other is the `TODO(user):` list under "The flags skip
work, never disclosure" below.
Report every leftover in "Final report" instead, unfiled, with the reason. Do not substitute your own
scope judgement for the agreement that never happened.

**Nothing under `--fast` may claim the code was reviewed.** Step 8 is the only reviewer in this
pipeline and it did not run. Per an explicit decision the PR body carries no disclosure of that, so
the `Mode` and `Iterations` lines in "Final report" are the entire disclosure the run produces. That
makes those two lines a control and not a formatting nicety, which is why the reporting rules name
them as mandatory on every ending.

#### What the flags cost

Say this in the announcement, and do not soften it later.

- **`--lite` takes every claim in the ticket on trust.** Nothing checked whether the bug still
  reproduces, whether a merged commit already fixed it, whether an open PR is already doing the work,
  or whether another ticket has carved the scope out from under it. Those are the four ways a ticket
  is dead and a `--lite` run detects none of them. It will work a dead ticket all the way to a PR.
- **`--fast` also ships code that nothing reviewed.** Not `review-and-fix`, not a human.

**Never describe a flag run's output as validated.** No verdict was reached, so there is nothing to
report as `valid`, and `invalid` and `superseded` are equally unavailable. The `Validation` line in
"Final report" says the phase was skipped and names the flag, which is the only honest value it can
carry.

#### The flags skip work, never disclosure

Every step a flag cut is named in "Final report", and its `Mode` line names the flag. A run that
quietly did less has to look different on the way out from a run that did everything, because the
Jira ticket and the PR carry no record of which mode wrote them and the conversation is the only
place that record exists.

**Keep the running `TODO(user):` list from Step 0, in every mode.** Step 5 is where the full run
establishes it, and a flag run skips the write that paragraph sits in, so start it empty here
instead. A flag run can still ship such a line from Step 6, from a `--lite` Step 8's leftovers, or
into the PR body at Step 9, and Step 9 suspends four of `open-pr`'s human gates with this report
named as the only thing standing in their place. Dropping the list would remove that control while
leaving the four suspensions in force. That is the second of the two places these flags could make a
run less safe instead of less thorough, the first being the Jira create named under "What still
runs" above.

```
/work-on WMP-837                  # full pipeline
/work-on WMP-837 --lite           # no validation phase, review-and-fix still runs
/work-on WMP-837 --fast           # no validation phase and no review-and-fix
/work-on --lite WMP-837           # flags in any position
/work-on WMP-837 --lite --fast    # --fast wins, no error
```

## Assume nothing

This is the rule the rest of the skill serves. A validation phase that accepts the ticket's
framing has done nothing, because the framing is the thing most likely to be stale.

**This whole section governs Steps 1 through 4, so a `--lite` run has nothing for it to govern.**
The three rules below are not relaxed on that path, they are unreachable, and the difference matters
for how the run talks afterwards. A `--lite` run accepts the ticket's framing wholesale, which is the
thing the paragraph above calls doing nothing. So it may never report a claim as checked, may never
say a bug reproduces, and may never say one does not. Every claim in the ticket is unverified for the
whole run, and "Argument" requires the announcement to say so.

- Every factual claim in the ticket gets checked one at a time against the tree at HEAD. A
  claim you cannot land on a `file:line` is unverified, and unverified is reported as
  unverified, never as true.
- "The code looks like it would do X" is not evidence. Evidence is a path, a line number, a
  test that fails, a commit sha, a PR number, a Datadog query with its result.
- Absence of evidence gets stated as absence. If nobody could find the reported behavior, the
  finding is "could not reproduce, here is where I looked", not "the ticket is wrong".

## Earn every question

Asking is the fallback, not the first move. A question whose answer is sitting in the repo wastes
a round trip and tells the user you did not look, and once that happens they stop trusting that
the questions you do ask are worth their attention.

**Go find it first.** In rough order of how often it works:

| Question shape | Where the answer already is |
|---|---|
| Where is X, does Y exist, is Z still called | `rg`, `git --no-pager grep`, read the file |
| Why is it written this way, when did it change | `git blame`, `git log -S '<symbol>'`, `git log -p <path>` |
| What did the author actually mean | The ticket's own comment thread. Read all of it. |
| Has someone already done this | `gh pr list --search`, the merged and open PRs from Step 1 |
| What does this default to | Read the config, the schema, the flag definition |
| Does this really happen in production, how often | A Datadog probe subagent |
| Do users actually hit this path, how many | An Amplitude probe subagent |
| How many rows, accounts, or records are affected | A query skill if one works, else the user, per [DB-QUERIES.md](DB-QUERIES.md) |
| Is this bug or security issue worth fixing at all | The triage agents in Step 2, then a priority call to the user |

**On a flag run the last four rows have no producer, and the ban below still holds.** The Datadog
probe, the Amplitude probe, the warehouse handoff, and the triage agents all live in Steps 2 and 3.
That does not promote their questions to askable. It means the question does not get asked and does
not get answered either, so whatever it would have settled stays unsettled for the whole run and gets
reported that way. The first five rows are unaffected, since `rg`, `git`, the ticket's own comment
thread, and the config are all still right there, and Step 6 leans on them harder than usual because
nothing else looked at the code first. Row 4 is the mixed one. Its Step 1 half is gone, so
"has someone already done this" falls back to running `gh pr list --search` yourself, and that keeps
the question off the user where the ban below already puts it.

**These are never questions for the user.** Each one is a search you skipped:

- "Where is the tax calculation?" / "Does `applyTax` still exist?" / "Is this function used?"
- "What does `BILLING_RETRY_LIMIT` default to?"
- "Why does this branch check for null twice?"
- "Is anyone working on this already?"
- "Does this error actually fire in prod?" / "How many users are affected?"

**These are legitimately for the user**, because nothing in the repo can answer them:

- A product or intent decision. "Should this apply to trial accounts too?"
- A scope or priority call. "The batch path has the same bug. In scope, or a separate ticket?"
- Context that only exists in someone's head or a meeting.
- A judgement between two approaches where both work and the trade-off is a preference.
- A contested finding where the skeptics split and the evidence genuinely does not settle it.

**A warehouse number is never a question in prose.** Try a query skill first, and if none works it
becomes a file of numbered queries and one absolute path. Never a paragraph asking the user to go look
something up. [DB-QUERIES.md](DB-QUERIES.md) has the skill check, the two rules every query obeys, the
file format, and the follow-up protocol that keeps the request from getting lost.

### Datadog and Amplitude go to subagents

**Never query either one from the main thread, and never from a code lens.** Dispatch a probe per
source and run them concurrently, in one message.

**A flag run dispatches neither probe, and the prohibition outlives them.** The dispatch instruction
belongs to Step 2 and the flags that gate it belong to Step 1, so under `--lite` there is nothing to
launch. The first sentence above is not conditional on that. It bans querying Datadog or Amplitude
from the main thread, and a run with no probes must read that as "no telemetry read happens" rather
than as "the main thread may do it itself now". A production claim in the ticket therefore stays
unmeasured for the whole run.

Three reasons this is not optional:

- **Discovery is per-source, not per-caller.** Both servers want a discovery sequence before the
  first query, Datadog for its skill guides and Amplitude to find real event names. Anything that
  queries directly pays that again. Seven lenses each doing it is seven redundant sequences for one
  answer.
- **Raw telemetry does not belong in a reasoning context.** Log lines, event rows, and chart
  payloads are large and almost entirely noise. A probe reads them and returns numbers, so the
  volume stays inside the subagent that needed it.
- **The two sources are independent, so they overlap for free.** Datadog answers "does this happen"
  while Amplitude answers "to how many". Neither needs the other's output, and neither needs the
  code lenses.

**Use the briefs in [VALIDATION.md](VALIDATION.md)'s `TELEMETRY` array.** They are the single copy.
Both paths dispatch the same text, the workflow as two more entries in its parallel Investigate
batch, the inline path as two concurrent `Agent` calls.

A brief on its own is not the whole prompt. The workflow pairs each one with three more things, and
the inline path has to supply all three itself or the probe comes back unusable:

- **`telemetryRules()`**, which carries the never-paste-raw-rows rule, the ground-every-number rule,
  the `verdict_critical` calibration, and the rule that a warehouse question goes back as SQL rather
  than being queried in place.
- **The context block** you gathered in Step 1, since the briefs reference `repo.missing_commits`,
  and `merged_prs` by name.
- **`TELEMETRY_SCHEMA`** as the output shape. This is the one people drop, and dropping it is the
  worst of the three, because `available` and `unavailable_reason` live there. Without them a probe
  has no way to say it could not reach its source, and an unreachable source silently reads as
  "found nothing".

**Only launch a probe with something to ask.** Datadog when the ticket asserts something about
production, such as an error occurring, a rate, or a regression. Amplitude when it asserts something
about users, such as how many are affected, a segment, or a funnel. Both off for a refactor or an
internal cleanup, which is the common case. A probe with nothing to ask burns a discovery sequence
and returns nothing. **A `bug` or `security` kind, decided in Step 1, turns Datadog on regardless
of what the ticket asserts**, and turns Amplitude on too when the path is user-facing. This is the
one case where the flags are not set from the ticket's own claims, because the triage pass makes a
production claim the ticket never made. It asks whether anybody is actually affected, and that
question has exactly one honest source. Without the flags the triage block comes back unmeasured
on the tickets it exists for, and an unmeasured impact reads as a small one.

**A probe that cannot reach its source reports that**, and every production claim in the ticket
stays unverified. Carry that into Step 4 and into the rewritten ticket. Unchecked is not the same as
false, and the usual cause is an unauthenticated MCP, which the user can fix.

One thing a probe finds that nothing else can: **the shape of the signal over time.** If the error
rate dropped to zero on the day a PR in `HEAD..origin/<default>` merged, the ticket is superseded
and no amount of reading the code at HEAD would have told you, because the code at HEAD is already
fixed. The Datadog brief asks for that correlation explicitly.

## Step 0: Preflight

**1. Resolve the ticket key.** **Strip the modifier flags first**, per "Argument", and record the
mode. `--lite` and `--fast` are the only two, they may sit anywhere in the argument, and what is
left after removing them is the key. A flag left in place either fails the regex below or reads as
a missing key, and both failures look like a malformed invocation rather than a stripping bug.

Then accept a bare key (`WMP-837`), a Jira URL
(`.../browse/WMP-837`), or the key embedded in the current branch name (`wmp-837-invoice-tax`).
Uppercase it. If the invocation carries no key and the branch name has none, ask for it and
stop until you have it. Never invent a key and never guess the project prefix.

**Announce the mode once, here, before anything else runs.** "Argument" says what the announcement
carries.

**Then validate it against `^[A-Z][A-Z0-9]+-[0-9]+$` and stop if it does not match.** This is the
same pattern `in-depth-review`'s ticket reviewer uses. It matters beyond catching a typo, because the
key becomes part of file paths (`/tmp/claude/work-on-<KEY>-queries.sql`) and of branch and commit
references on real commands. A branch name is set by whoever created the branch, not necessarily by
the person running this, so anything derived from it gets checked before it reaches a command line.
Ask for the key directly rather than trying to sanitize a branch name into one.

If the argument key and the branch-name key disagree, stop and ask which one is right. Working
a ticket on a branch named for a different ticket is almost always a mistake, and it poisons
the commit trail and the PR.

**2. Verify the repo and the branch.**

```
git rev-parse --is-inside-work-tree
git rev-parse --abbrev-ref HEAD
```

Determine the default branch (`git remote show origin` and read the `HEAD branch` line; fall
back to `main`).

**Abort if the current branch is the default branch, or if HEAD is detached.** Tell the user to
cut a feature branch and re-run, and suggest a name from the ticket key and summary
(`wmp-837-invoice-tax-double-add`). Do not create the branch. Which branch the work lands on is
the user's call, and this skill pushes to the remote later in the run, so guessing wrong is
expensive.

**3. Working tree.** `git status --porcelain`. If it is dirty, ask whether to stash or to commit
first. Do not decide this silently, and do not offer to validate against a dirty tree. Every lens is
pinned to "the tree at HEAD", and nothing in the workflow's `args` describes uncommitted changes, so
a lens that happened to read one would be contradicting its own brief rather than honoring the
choice. Stash or commit makes the tree and HEAD agree, which is what the whole validation phase
assumes.

**Stash with `-u`, and record the stash's commit sha, not `stash@{0}`.**

```
git rev-parse -q --verify stash@{0}        # before: what was already on top, if anything
git stash push -u -m "work-on <KEY> preflight"
git rev-parse -q --verify stash@{0}        # after: must differ from before
```

**Compare the two shas and only record yours if they differ.** `git stash push` can create no entry at
all: it refuses on unmerged paths, and it prints "No local changes to save" when there is nothing it will
take. `git rev-parse stash@{0}` still resolves in that case, to whatever stash the user already had. Record
that and Step 7 will apply and then drop the user's unrelated work. If the shas match, no stash was
created: say so and treat the tree as still dirty rather than proceeding as if it were clean.

`-u` because `git status --porcelain` counts untracked files (`??`) as dirty, and a stash without `-u`
leaves them sitting in the tree. It can even create no stash entry at all when untracked files are the
only difference, so the run would believe it had cleaned the tree, every lens pinned to "the tree at
HEAD" could read a file nothing in `args` describes, and Step 7 would sweep those files into the ticket's
commits and push them.

The sha because `stash@{0}` is positional, not a handle. It means "top of the stack", so it is the same
thing a bare `git stash pop` takes, and any stash created later by the user, by Step 6, or by Step 8
shifts your entry to `stash@{1}` while `stash@{0}` now names someone else's. The sha survives any
reordering.

**If you stash, you own restoring it.** Tell the user in Step 9's report either that you popped it or that
it is still there and how to get it back. A run that stashes and never pops ends by handing the user a PR
while their own uncommitted work sits invisible in the stash stack, missing from the diff they are about
to review.

Pop it once the work is committed, which is after Step 7. Do not pop before then, since a conflict
against freshly written code is worse than a late restore. If the pop conflicts, stop and say so
rather than resolving it yourself. Those are the user's changes and they know what is in them.

**Pop it on every early exit too, not just the happy path.** Step 4 stops the run outright on an
invalid or superseded verdict, and that is the most common early exit there is. Steps 7 and 9 never
execute on that path, so the pop and the disclosure both have to happen wherever the run actually
ends. The rule is: no turn ends with work stashed and unmentioned. If you stashed and you are about
to stop for any reason, restore it first, or say plainly that it is still stashed and how to get it
back. Handing someone a dead-ticket verdict while their own changes sit invisible in the stash stack
is exactly the failure this paragraph exists to prevent.

**A flag run loses that most common early exit and keeps the rule.** Step 4 does not run, so the
dead-ticket ending cannot happen, and Step 4's own settlement block is unreachable. The endings that
remain are a Step 0 abort, a Step 6 that stops or hands back, a `--lite` run whose Step 8 aborts
twice, and an `open-pr` failure. Every one of them is a turn that ends, so every one of them settles
the stash and says so. Read "no turn ends with work stashed and unmentioned" as the whole rule and
the Step 4 sentence above as one example of it.

Step 8 (`review-and-fix`) will ask the same question again if it finds a dirty tree, so resolving it
now avoids that interruption later. Step 7 commits everything anyway, so by Step 8 the tree is
normally clean regardless.

**4. `git fetch origin`.** Every "what changed since the ticket was filed" question in Step 1
reads remote refs. A stale `origin/<default>` makes the merged-PR analysis quietly wrong, which
is the single worst failure mode of this skill.

**5. Jira access.** Confirm a Jira reader AND writer is available before anything else, because
Step 5 is a write and discovering the block after validation wastes the whole phase.

- **Atlassian MCP (preferred).** Search for it (`ToolSearch "atlassian jira"`). If the only
  tool exposed is an `authenticate` tool, it is connected but not authed. Get the `cloudId`
  once via `getAccessibleAtlassianResources` and reuse it for every later call. This path is
  preferred because it converts markdown for you. See [JIRA-FORMAT.md](JIRA-FORMAT.md).
- **`acli` (fallback).** `command -v acli`, then one lightweight authenticated read. **In a
  sandboxed session, skip the probe and treat `acli` as unavailable.** It reads its credentials
  from under `~/.config`, which the sandbox blocks, so it fails even when installed and
  authenticated. This is the same carve-out `review-and-fix` makes.

If neither is ready, ask the user to authenticate one, then re-check. Do not offer to proceed
without Jira. A run that cannot read the ticket has nothing to validate, and a run that cannot
write it silently drops Step 5.

**6. GitHub access.** Prefer a GitHub MCP when connected (`ToolSearch "github pull request"`),
fall back to `gh`. If neither is available, say so and ask whether to continue with git-only
evidence. You can still diff `HEAD...origin/<default>` (three dots, as in Step 1), so merged-commit
analysis survives, but you lose PR numbers, PR discussion, and every open PR. That is a real gap in
the validation and the user should get to weigh it, not find out afterward.

**On a flag run, do not ask that question.** Everything it offers to trade away is Step 2 evidence a
`--lite` run was never going to gather, so putting it to the user asks them to weigh a loss the flag
already took. State that GitHub is unavailable in one line and carry on. Step 9 still needs `gh` to
open the PR, so say that too, since that consequence is real on this path and the validation gap is
not.

**7. Warehouse access. Nothing to do here, deliberately.** This skill has no database connection of
its own. When a warehouse number is actually wanted, Step 3 checks whether a skill that runs SQL is
installed and falls back to asking the user. That check is deferred rather than done here because most
tickets need no warehouse query at all, so probing in Step 0 spends effort on the common case that
never uses it.

Note the scratch-file key now, though. Every file this run writes is keyed by ticket, and the query
handoff is `/tmp/claude/work-on-<KEY>-queries.sql`. See [DB-QUERIES.md](DB-QUERIES.md).

## Step 1: Ground truth

Collect this once here, not inside the workflow. Seven agents each re-fetching the same ticket is
six chances to read a different version of it, and the workflow's job is to reason, not to
gather.

**On a flag run this step reduces to two things**, the markdown ticket read and the repo
conventions. "Argument" has the full list of what is dropped and which step was its only consumer.
Read that before reading on, because most of this step is addressed to a Step 2 that will not run,
including the reason above. The command-shape note at the end of the step is the exception and
applies to every run.

**The ticket.** Read it twice, in two formats, for two different reasons. **On a flag run read it
once, in markdown only.** The `adf` half exists solely to make the rollback artifact for a
description rewrite that no longer happens.

- `responseContentFormat: "markdown"` with `fields: ["*all"]` plus `"comment"`. This is what
  you reason over. Comments matter as much as the description. A ticket is often corrected,
  narrowed, or quietly abandoned in its comment thread while the description still says the
  original thing.
- `responseContentFormat: "adf"`. Save the raw ADF to
  `/tmp/claude/work-on-<KEY>-original.adf.json` before any write happens. This is the rollback
  artifact for Step 5. Jira's history is not a convenient restore path, and a botched
  description rewrite on someone else's ticket is the kind of damage that is hard to undo from
  memory.

**The branch.**

**Key every scratch file by ticket.** `/tmp/claude/work-on-<KEY>-*` throughout, never a bare
`work-on-*`. Two runs in two repos, or one run after another, otherwise share these filenames, and the
second run silently reads the first run's merged-commit list into `args`. Every lens then reasons
about merged work from the wrong repository, which is the failure Step 0's `git fetch` note calls the
single worst one this skill has.

```
git rev-list --count origin/<default>..HEAD
git --no-pager log origin/<default>..HEAD --oneline > /tmp/claude/work-on-<KEY>-branch-commits.txt
```

**The merged work this branch does not have.** This is the reverse range and it is the one
people get backwards. `origin/<default>..HEAD` is your commits. `HEAD..origin/<default>` is
everything merged that you are missing, which is where a ticket goes stale.

```
git --no-pager log HEAD..origin/<default> --oneline > /tmp/claude/work-on-<KEY>-missing-commits.txt
git --no-pager diff --stat HEAD...origin/<default> > /tmp/claude/work-on-<KEY>-missing-stat.txt
```

Then map those commits to PRs so the workflow can read intent rather than infer it. Per commit
sha, `gh api repos/{owner}/{repo}/commits/<sha>/pulls`. If the list is long, resolve PRs only
for commits that touch files the ticket implicates, and **say in the Step 2 handoff how many
commits you skipped and why.** A cap nobody mentions reads as full coverage.

**The open PRs.**

```
gh pr list --state open --limit 100 --json number,title,url,headRefName,author,updatedAt,files > /tmp/claude/work-on-<KEY>-open-prs.json
```

Keep the ones whose `files` intersect the paths the ticket implicates, plus any whose title or
branch name carries the ticket key. Carry `author` through to `args`, because Step 4's superseded path
has to name it. An open PR that already does the work is the second most common reason a ticket is
dead.

**If the list comes back at the limit, say so in a `coverage_notes` entry.** A truncated sweep and a
repo with no overlapping PR look identical from the result, and the wrong reading produces a `valid`
verdict on work somebody else is already doing. Count the returned PRs, and when the count equals the
limit, either raise it and re-run or record what you could not see. This is the same rule the
merged-commit cap above follows, and it applies here for the same reason.

**The related tickets.** The tracker is the one source the code cannot show you. A duplicate somebody
filed last month, a follow-up already carved out of this ticket, or a sibling that shipped the fix under
another key are all invisible to every other lens here.

Two halves, and the first is free.

**Linked tickets cost nothing.** The ticket fetch above already uses `fields: ["*all"]`, so
`issuelinks` and `parent` are in the payload you have. Read them. Each link carries a type
(`duplicates`, `blocks`, `relates to`) and that type is somebody's deliberate judgement, which makes it
the strongest signal in this whole block. `parent` gives you the epic, whose other children are the
most likely place for an overlapping ticket to be sitting.

**Then search, because most related tickets were never linked.** One JQL, scoped to the project:

```
project = <PROJ> AND key != <KEY>
  AND (statusCategory != Done OR resolutiondate >= -90d)
  AND text ~ "<terms>"
ORDER BY updated DESC
```

Take `<terms>` from the ticket's own summary and the paths it implicates, not from its whole
description. Cap at 50. Closed tickets are included within 90 days because "already shipped under
another key" is a real and common cause of a dead ticket, and excluding Done would hide exactly that.
Older than that and a resolved ticket is history rather than a duplicate.

Screen the results here, in the main thread, against each summary. Drop the obvious misses rather than
passing 50 rows into `args` for seven lenses to each re-read.

Write the survivors to `/tmp/claude/work-on-<KEY>-related-tickets.json` and pass them as
`related_tickets`. Every entry carries `key`, `summary`, `status`, `resolution`, `resolutiondate`,
`assignee`, `url`, and **`relation`**: the Jira link type when a person linked it, or `"search"` when
only the query found it. **Do not flatten those two into one field.** A `duplicates` link and a shared
word are wildly different evidence, and the `ticket-overlap` lens is told to weight them apart.

**If the search hits the cap or cannot run, say so in `coverage_notes`.** This is the same rule the
merged-commit and open-PR blocks above follow. An empty `related_tickets` has to mean "looked, found
nothing", because a lens reading an empty array otherwise concludes the tracker is clear when nobody
actually asked it.

**Repo conventions.** Read the root `AGENTS.md` / `CLAUDE.md` and the nearest sub-project one.
Step 6 writes code and Step 8 reviews it against these, so load them before either.

**Decide what kind of ticket this is.** You have read it and nothing downstream has, so this call is
yours, exactly like the telemetry flags below. The ticket fetch already used `fields: ["*all"]`, so
`issuetype`, `labels`, `components`, and `priority` are in the payload you have. Read those and the
description, then record `{ kind, kind_because }` where `kind` is `bug`, `security`, or `other`.
Step 2 passes it to the workflow as `args.kind` or dispatches the triage agents directly on the
inline path.

Judge it, do not read it off one field. Bugs get filed as Tasks and Stories constantly, and a scanner
finding pasted into a Story with no label is exactly the ticket where this matters most. A `security`
kind is anything about authorization, authentication, tokens, secrets, tenant isolation, or data
exposure, however it was filed.

**A ticket that qualifies as both takes `security`.** The two mistakes are not symmetric.
`exploit-realism` is the one assessment nothing else supplies. A security issue labelled `bug`
loses it silently, and the `triage` block still looks complete, because `fix-cost` runs either
way. A bug labelled `security` gets `exploit-realism` instead of `reachability`, and it reports
that no attacker model applies. That is a cheap and visible wrong answer, not a missing one. The
gate pass weighs findings by this same asymmetry when deciding which ones get skeptics, so this
rule is consistent with the rest of the design rather than a new principle.

**In doubt between a `bug` and `other`, record `bug`.** Two extra agents is cheap. A skipped
assessment is silent, and silence here reads as "nobody thought this was worth checking".

**Say which kind you chose in the Step 2 handoff**, with the reason. An `other` on an obvious bug is
the failure mode, and it is invisible unless the choice is stated.

**Decide which telemetry probes to launch.** You have read the ticket and nothing downstream has,
so this call is yours. Datadog on when the ticket asserts something about production behavior.
Amplitude on when it asserts something about users. Both off for a refactor or internal cleanup.
A `bug` or `security` kind from the kind decision above overrides that. Datadog goes on
regardless of what the ticket asserts. Amplitude goes on too when the path is user-facing. The
triage pass makes a production claim the ticket never made, so the flags cannot come from the
ticket's own claims alone. See "Datadog and Amplitude go to subagents" above for the full
reasoning. Record it as
`{ datadog: bool, amplitude: bool }`. Step 2 passes it to the workflow as `args.telemetry` or
dispatches the probes directly on the inline path.

Note on command shape, because this repo's hooks enforce it: `git` and `gh` run outside the
sandbox and must run **alone**, with no pipe and no chaining. Redirect to a file under
`/tmp/claude/`, then read the file in a separate call. Do not write `git log ... | head`.

**This note is a hook rule, not a Step 1 rule, and it survives every reduction.** It happens to sit
at the end of the step a flag guts, and the commands it governs are spread across the run: Step 0's
`git fetch`, `git status`, and `git rev-parse`, Step 7's commit, push, and three stash commands, and
Step 9's `gh`. A flag run runs all of those, so read this note even when you skip everything above
it.

## Step 2: Validate

**Skipped entirely by `--lite` and `--fast`.** Not reduced, not run inline, not collapsed to a single
reader. Nothing in this step happens and nothing downstream receives its output. Go to Step 5. See
"Argument".

The one thing a workflow buys here is resistance to anchoring. A single reader reads the ticket,
absorbs the ticket's view of the problem, and then goes looking for support, and it finds some,
because it stopped looking where things fit. Seven lenses each pinned to one question and blind to
each other cannot converge that way. When six say the problem is real and the seventh finds the
open PR that already fixed it, that disagreement is the finding.

That is worth real money on a wide ticket and worth nothing on a narrow one.

**Do it inline, in the main thread, when the surface is small.** All of:

- the branch is behind by a handful of commits, not hundreds
- no open PR touches the paths the ticket implicates
- no related ticket looks like a duplicate or a carve-out
- the ticket makes a few checkable claims rather than a page of them
- one or two files are implicated

Then investigate directly. Read the code, read the missing commits, read the ticket's comments, read
the related tickets Step 1 found. Ask as you go. This is the common case for a small bug and the
workflow would be ceremony.

**Telemetry still goes to subagents on this path.** Skipping the workflow does not mean querying
Datadog or Amplitude yourself. If the Step 1 flags turned either on, dispatch those probes as
`Agent` calls in a single message so they run concurrently with your own reading, using the briefs
from [VALIDATION.md](VALIDATION.md)'s `TELEMETRY` array. Their digests come back while you are still
in the code, which is the point.

**Triage goes to subagents on this path too.** If Step 1 recorded a `bug` or `security` kind,
dispatch the triage agents in one message, alongside the telemetry probes, using the brief from each
entry in [VALIDATION.md](VALIDATION.md)'s `TRIAGE` array whose `enabled` matches that kind. A brief
is not the whole prompt. Pair each one with `triageRules()`, the context block including its
`<<<UNTRUSTED_INPUT_BEGIN>>>` and `<<<UNTRUSTED_INPUT_END>>>` markers, and `TRIAGE_SCHEMA`. Dropping
the schema is the worst of the three, because `gate_critical` lives there and without it nothing
separates an argument against the work from an argument for it.

**Keep the cost agent blind to the impact answer, and the impact agent blind to the cost.** On this
path you are holding both, which makes it your job not to leak either. Dispatching them in one message
is what enforces it.

**You have no skeptics here, so say so.** The workflow path attacks every dismissal before it reaches
a verdict. Inline, nothing does. A `worth_fixing` of `no` from this path is a dismissal nobody tested,
and it goes to Step 4 marked that way, as a coverage gap. Presenting it as settled would be the silent
degradation this skill refuses everywhere else.

**Run the workflow when the surface is real.** Any of: the branch is far behind, several open PRs
overlap, the ticket makes many claims, the blast radius spans services, several related tickets
suggest scope has already moved out of this one, or the inline pass has already turned up two findings
that contradict each other.

[VALIDATION.md](VALIDATION.md) has the script, the seven lens prompts, the two telemetry probe
briefs, the three triage briefs, and the schemas. It runs once, not in a loop. Seven lenses,
whichever telemetry probes Step 1 enabled, and the triage agents on a bug or security ticket all go
out in one parallel batch, then skeptics attack any finding that could flip the verdict, then one
synthesizer produces the verdict. Pass the Step 1 artifacts via `args` so no agent re-fetches,
including `kind` and `kind_because` from the classification above, `telemetry` with the probe flags,
and `related_tickets` with the tracker results.

**Check the `Workflow` tool is actually available before routing a wide ticket to it.** It is a
harness capability, not something this repo ships, so it is present in some contexts and absent in
others, and a subagent typically does not have it at all. If it is missing, fan out with plain
concurrent `Agent` calls instead: one per lens, each getting its lens `ask` plus `rules()`, the context
block, and `FINDING_SCHEMA`, then do the refute and synthesize passes the same way. That is what the
sibling review skills do, and it costs the run only the script's bookkeeping, not its structure. A
denied call takes the same fallback. A hook denies the `Workflow` when the script body names a
fan-out review skill, so a script that mentions one by name never runs. Fan out with plain
concurrent `Agent` calls instead. Never edit the ticket text down to get a call through, because
the lenses reason over exactly that text.
The loss runs the other way too. A workflow agent has no `Agent` tool, so nothing that fans out into
sub-agents can go inside one, and Step 2's lenses are safe there precisely because each is a leaf
that spawns nothing.

What it must not collapse to is the inline single-reader pass. The inline path is chosen for a
*small* surface, so falling back to it on a wide ticket is the anchored single-reader read that this
whole phase exists to prevent, and it would be silent about having done so. Say which path you took
and why.

Either way the output is the same shape and Step 4 reads it identically: a verdict, evidence with
locators, the questions investigation could not settle, and on a bug or security ticket the triage
block, with whatever the path could not establish named as a coverage gap.

## Step 3: Ask what investigation could not settle

**Skipped entirely by `--lite` and `--fast`.** No investigation ran, so there is nothing this step
could be asking about, and its questions are not reassigned to Step 6. See "Argument". A genuine
product decision that surfaces later in Step 6 still goes to the user, through whichever process
skill Step 6 picked and under that skill's own rules, never as a revival of this step.

Read "Earn every question" above before asking anything. Every question here has already survived
a search of the codebase, the git history, the ticket comments, the PRs, and Datadog or Amplitude
where the claim was about production.

Ask conversationally. If a couple of genuine decisions came out of validation, put them in a
message. Use `AskUserQuestion` when the choices are discrete enough to be worth options, at most
four per call, each with its real consequence and your recommendation first. Batch related ones so
the user answers a theme rather than a trickle.

**A triage question is a priority call, not a request for a number.** "How many users are
affected?" is a search somebody else owns and this skill forbids asking it. What reaches the user
is the decision. Here is what is reachable, here is what it costs, do we fix it, defer it, or run
a query first. If it carries SQL, it goes down the warehouse path below like any other query, and
the priority call is what remains after the query comes back. **Suppress the question entirely when
the answer could not change the decision**, such as an S-sized fix that gets done regardless of who
is affected, and treat that as a rule for whoever composes the question, on the workflow path and
the inline one alike.

**The defer option must say what it does.** Offer it as posting a comment carrying the impact and
cost evidence, so the ticket can be deprioritized on the tracker itself. Stating that up front is
what makes the write authorized once the user picks it, the same way option (a) is authorized by
its own wording at Step 4.

**Warehouse queries get run before they get asked.** If any probe returned a `sql` field, pool those
queries and check whether a skill that runs SQL is installed and working. Judge a candidate by what it
declares it does, not its name, and try it once. See
[DB-QUERIES.md](DB-QUERIES.md#try-to-run-them-first). Anything it answers never reaches the user, and
one failure on its environment sends everything remaining to the handoff without a retry.

**A SQL handoff is the one thing that never gets batched.** For whatever is left, pool every query into
one file and ask for it in its own message, with nothing else in it. Then re-ask on every turn until it
resolves, because a request bundled with three other questions is the request that gets skipped, and a
skipped query either stalls the run or turns into a number nobody actually has. Assume it was missed,
not refused, unless the user says so outright.
[DB-QUERIES.md](DB-QUERIES.md#keep-asking) has the exact protocol, including what counts as a decline
and what does not.

**When an answer opens a new line of inquiry, go investigate it.** Do not re-run the workflow.
Seven lenses re-reading the same tree to absorb one answer is waste, and the answer usually points
at one specific thing to go read. Re-run a single lens only when an answer invalidates that lens's
premise outright, and pass the answer in.

Questions that turn out not to matter get dropped rather than asked for completeness. Anything
still open but not blocking rides to Step 5 as a `TODO(user):` line in the ticket, and onto the
running list that "Final report" reads out.

## Step 4: Verdict gate

**Skipped entirely by `--lite` and `--fast`.** No verdict value exists on that path, so none of the
four below is available and neither is the a/b/c option list. The run cannot conclude the ticket is
dead, and it cannot conclude the ticket is valid either. It proceeds on the ticket as filed. See
"Argument".

**Two things inside this step are needed elsewhere, so find them before skipping it.** The stash
settlement rule under "Valid" applies to every early exit the run can have, including ones a flag
run reaches, and it is restated at its own line below. The "### Filing the follow-up" subsection is
the machinery Step 8 reuses by name, and `--lite` disables that reuse rather than borrowing it.

Validation returns one of four verdict values, `valid`, `invalid`, `superseded`, or `partial`, which
fall into the three response paths below. Present the evidence for whichever it is, in chat, before
doing anything. On a bug or security ticket the triage block goes out with that evidence, in the
same message, ahead of any option list.

**Valid.** The problem reproduces at HEAD, no merged commit fixed it, no open PR covers it. State
where it reproduces, present the triage block below alongside that evidence on a bug or security
ticket, then go to Step 5.

**The triage block, on a bug or security ticket.** It rides with the verdict evidence, in the same
message, whatever the verdict is, and always ahead of the (a)/(b)/(c) list that follows Invalid and
Superseded below. Four lines: whether anybody is affected now and how that was measured, whether the
path is live and at which `path:line`, what a fix costs and why, and the recommendation with its
confidence. On a security ticket the second line is the precondition chain instead, including how
long each precondition stays valid, because a value that expires in a minute and a sequential id
are different tickets.

Name what was not established. An unattacked dismissal, an unreachable telemetry source, or a cost
estimate that never named a file all bound what this block can claim. On the workflow path, every
one of them is in the workflow's `gate_coverage`. On the inline path, where that field does not
exist, name the gap directly, the same way "You have no skeptics here" above already requires. A
confident recommendation over thin evidence is the worst thing this pass can produce, and it is
worse than no recommendation because it looks like one.

**This is not a verdict and it does not stop the run on its own.** A `worth_fixing` of `no` or
`unclear` reaches the user earlier, as one blocking question at Step 3, so by the time this gate
runs the answer is already in hand. Present the triage block together with that answer, and act on
it here. Do not ask it again. A defect that reproduces is still a defect that reproduces, and
whether it is worth the money is the user's call and not this skill's.

**Map the Step 3 answer to what happens next.** Fixing it continues to Step 5 on the ticket as it
stands. Deferring it ends the run the way option (a) below does for a dead ticket, so post the
comment carrying the impact and cost evidence, and let the stash rule's (a) branch apply. A query
that had to run first resolves through the warehouse handoff, and its answer then decides between
the other two. A query the user declined or waived leaves the call unanswered. Treat that the same
as the case below where Step 3 asked nothing. Present the block and continue, because an unmeasured
impact is never evidence of no impact.

**On an `invalid` or `superseded` verdict, this mapping does not apply.** There is nothing to fix,
so the priority call is moot. Present the triage block for context, then take the Invalid or
Superseded path below as usual. That path's own comment carries the evidence, and the two endings
never merge or double up.

**When Step 3 asked nothing, there is nothing to decide here either.** Step 3 suppresses the
priority question when the answer could not change the decision, such as an S-sized fix that gets
done regardless of who is affected. Present the block and continue.

**If `worth_fixing` is `yes`, say so in one line and move on.** The block still gets presented,
because the cost is worth knowing before Step 6, but a recommendation to proceed does not need a
decision from anybody.

**Superseded.** Something already handles it. The two causes need different evidence, and asking for
the wrong one produces a report that cannot be assembled.

- **A merged fix.** Name the PR or commit, its merge date, and the `file:line` at HEAD showing the
  behavior no longer holds. The problem is genuinely gone from the tree.
- **An open PR.** Name the PR, its author, and the files it touches, and say whether it covers the
  whole ticket or only part. **The behavior still reproduces at HEAD here**, because nothing has
  merged, so there is no `file:line` proving otherwise and you should not go looking for one. Saying
  the work is already in flight elsewhere is the whole finding. Duplicated effort and a merge
  conflict are what this catches, not a fixed bug.
- **Another ticket.** Name the key, its status, its assignee, and whether it covers the whole thing or
  part. Say how it was found, because a `duplicates` link somebody created and a text-search hit are
  not the same claim, and the user is about to close a ticket on this. If that ticket is resolved,
  treat it exactly like a merged fix and go find the `file:line` at HEAD. A Done status is not evidence
  the code changed. If it is still open, the behavior does still reproduce here, so do not go hunting
  a `file:line`, and the finding is the duplicated effort.

**Invalid.** It never reproduced, or the ticket's premise is refuted. The evidence here has a
different shape, and asking for a commit that fixed it is the wrong question, because there may be
no such commit and nothing may ever have been broken. Show instead where you looked, at
`file:line`, what the code does there rather than what the ticket claims, and which of the ticket's
specific claims the claim-audit refuted. An existing passing test over that path is strong evidence
and worth naming.

**Both stop the run. Write nothing on your own initiative.** No Jira write, no code, no commit.

Present the evidence, then offer:

- (a) post a comment carrying this evidence so the user can close the ticket themselves,
- (b) narrow the ticket to whatever still reproduces and continue from Step 5 on that,
- (c) proceed anyway, because the user disagrees with the finding.

This (a)/(b)/(c) list belongs to a dead ticket. A triage priority call is a different question,
asked once at Step 3 and never re-asked here, per the triage block above. Only the a/b/c choice
itself does not carry over, since that choice already happened at Step 3. Everything else option
(a) requires of its comment still applies to a deferral, including the drafting and read-back path,
the authorization rule, the stash rule's (a) branch just below, and the TODO roll-call.

Closing or rewriting someone else's ticket on an automated verdict is not this skill's call to
make. Option (c) is a real option. Take it at face value, note the disagreement in one line,
and continue.

If the user picks (a), post that comment. Writing nothing is the default, not a prohibition on
carrying out what the user just chose.

**Settle a Step 0 stash only once the user has chosen, and only if the run is actually ending.**
Options (b) and (c) continue into Step 5 and beyond, so restoring before they choose would put the
run back on a dirty tree, which Step 0 refused to validate against, and Step 7 would then commit the
user's unrelated changes into the ticket's commits and push them. So:

- (a), or the user stopping here: the run is over. Restore the stash, or say it is still stashed and
  where. Do not end on a dead-ticket verdict with the user's own work unmentioned.
- (b) or (c): the run continues. Leave the stash alone and let Step 7 handle it as usual.

**That comment takes the same path as Step 5's, all of it.** It is the only Jira write this skill
makes outside Step 5, which makes it the easiest one to post raw. Draft it with `writing-work-docs`,
write it per [JIRA-FORMAT.md](JIRA-FORMAT.md), and read it back to verify. Skipping any of that is how
a comment lands with literal `##` and `**` in it on the `acli` path, which is exactly the failure
JIRA-FORMAT.md exists to prevent.

**The (a) pick is the authorization. Do not ask again about the wording.** The user chose to post a
comment carrying this evidence, having read the evidence. A second confirmation on the rendered text
would be asking them to approve prose they already asked for, which is the one kind of question this
skill does not ask. Post it.

**If that comment ships a `TODO(user):` line, this ending owes the roll-call.** Option (a) ends the
run before Step 9, so the report at Step 9 never happens on this path. Emit the TODO roll-call from
"Final report" here instead, naming the comment as where each line landed. No run ends with a
published `TODO(user):` line unmentioned.

**Partially valid.** Some of it reproduces. Present the split, the evidence on both sides, and
the scope you propose to keep. Get explicit agreement on the reduced scope before Step 5,
because that scope becomes the ticket, the branch, and the PR.

**A follow-up ticket that already carved part of this one out lands here, not in `superseded`.** The
ticket is still real, it is just narrower than it reads, and the half that moved belongs to whoever owns
the follow-up. Name that key alongside the scope you propose to keep, so the user can see the work is
not being dropped. This case is easy to miss because the description still describes the whole
original problem.

**Propose filing the cut scope as its own follow-up, in this same presentation.** Show four
things and nothing else, in the same compact block shape "Final report" below uses.

```
Type      <Story, Task, or Bug>
Summary   <one line>
Points    <n>
Parent    <key, or none>
```

Showing more than these four turns the agreement into a review of drafted prose, and that is the
one wording gate this skill does not have anywhere else in the run.

**The agreement to the reduced scope is the agreement to file this.** State that plainly, or a
reader who does not see it stated will read the create as missing its own approval. Step 5 below
already rewrites the parent ticket's whole description with no preview, on this skill's stated
rule that it asks about decisions and never about wording. Whether to file a follow-up for cut
scope is a decision, and the user is making it right here, in the same answer that sets the
reduced scope. The follow-up's prose, once drafted, is wording, and a second prompt just for the
create would ask about a decision already made, which is the redundant round trip that teaches a
user their answers do not stick.

**Agreeing to the reduced scope and declining the follow-up are two different answers, and both
stay available.** Say so, or a reader cannot tell whether declining the create also reopens the
scope question. When the user takes the smaller scope and declines the follow-up, Step 5
describes the cut work in prose with no key, exactly as it does today.

### Filing the follow-up

This runs only on the agreement described above. A decline skips it, and Step 5 proceeds as it
does today.

**It runs after the agreement and before Step 5.** Step 5's "Out of scope" section names the new
key, and it cannot name a key that does not exist yet. This is what makes the line about the work
not being dropped true here, the same as it already is for a follow-up somebody else filed.

**Check the proposal's points before invoking anything.** If the `Points` value from the
proposal above is over 5, do not invoke `open-ticket` at all. Say instead that the split proposed
at Step 4 was too coarse, and ask the user to narrow the cut further until the follow-up sizes at
5 or under. This is the cheap check and it is not the only one. `open-ticket` sizes a delegated
requirement itself at its own Step 3, and a delegated call whose sizing lands over 5 aborts with
`DELEGATED_TOO_LARGE` rather than splitting into a tree. So skipping this check costs a round trip
and may come back with a number somebody still has to act on. The check sits here because this is
where the user can still narrow the cut, and `open-ticket` only has the abort.

**It invokes `open-ticket` through its Delegated entry section.** Supply the six values that
section names, mapped from what this run already has rather than restated here. The project key
comes from the originating ticket's own key, so a cut carved out of `GRO-1234` files under project
`GRO`. The requirement text is the cut scope as just agreed. The issue type is the `Type` line from
the proposal above, and it is the type that gets filed, because a delegated call takes the type from
its caller rather than deriving its own. Send it, or the user approves one type and a different one
lands. The files are what Step 2's validation already read. The originating ticket's key is
`GRO-1234` itself, this run's own ticket. The parent is that ticket's own `parent`, read from Jira,
or none when it has none.

**The recorded agreement from above is what satisfies `open-ticket`'s Step 9.** Pass it as the
delegated approval, and say plainly that this run is doing so. A run that somehow reaches this
subsection with no such agreement on record goes through `open-ticket`'s normal Step 9 gate
instead, because an issue created on nobody's authority is the one outcome that gate exists to
prevent.

**It runs in the main thread, never inside a `Workflow`.** `open-ticket` sits in `DENIED_SKILLS`
in `deny-review-in-workflow.py`, so a workflow naming it is denied before it can run. A workflow
agent could not host this gate even if the deny were lifted, because putting Step 9's approval in
front of a human is exactly what a workflow agent cannot do. Step 2's validation lenses are a
contrast here too, but not for Step 8's reason. Step 8's reason is fan-out loss, because
`review-and-fix` inside a workflow has no `Agent` tool to launch its reviewers with. This one is
the gate. `AGENTS.md` names it as the one exception to the fan-out reason the rest of
`DENIED_SKILLS` carries.

**A failed create never blocks Step 5.** If `open-ticket` cannot file the follow-up, Step 5 still
runs and describes the cut work in prose with no key, and the final report says the follow-up
could not be filed and why. The parent ticket's rewrite is this run's main deliverable, and a
follow-up is an addition to it, not a precondition for it.

## Step 5: Rewrite the ticket

Three writes. The status moves to In Progress, the description gets replaced in place, and a comment
gets appended. The status move goes first, because it is mechanical and the other two need drafting.

**On a flag run there is one write, the status move.** The description rewrite and the validation
comment are both dropped, because both exist to record what validation found and nothing found
anything. There is no "other two" and no "rest of Step 5" on that path. See "Argument".

**Four things in this step are declared for the whole run and are not dropped with those two
writes.** Read them even on a flag run, because Step 7 and Step 9 both depend on them and neither
step restates them:

- **`writing-work-docs` authors every human-facing artifact**, which on a flag run means the commit
  messages and the PR title and body.
- **"### The publish override, declared once for the whole run"**, which is what permits this skill to
  publish at all. Drop it and `writing-work-docs` refuses to publish, so Step 7 cannot author a commit
  message and Step 9 cannot author a PR body. Its own heading says it is declared for the whole run.
- **"No confirmation on wording, at any step"**, which governs the commits and the PR body too.
- **The running `TODO(user):` list**, whose own paragraph says the list belongs to the run rather than
  to this write.

What a flag run does skip is the drafting spec for each dropped write, the rendered-description
rules, the comment's Evidence trail, and the write-then-verify read-back, since there is no authored
content to read back.

### Move the ticket to In Progress

**Reaching this step means work is going to happen, and that is the whole condition.** No verdict
check. A `valid` or `partial` verdict arrives here directly, and a dead-ticket verdict arrives here
only when the user chose Step 4's option (b) to narrow the ticket or option (c) to proceed anyway,
which are the user saying continue in plainer terms than a verdict can. Option (a) stops the run
before this step, so a ticket this run concluded should be closed never gets parked on the active
board.

**A flag run reaches this step by a fourth route and the whole condition is the same one.** No
verdict arrives, because Step 4 did not run, and the sentence above already says no verdict check
happens. What makes the move correct is unchanged: the user invoked this skill on this key, so work
is going to happen. The move is also the reason a flag run cannot skip Step 5 outright. A ticket
somebody is building against belongs on the active board whether or not anything validated it.

Fetch the ticket's available transitions rather than writing a status directly. Then:

- **Already in a work-started status.** No-op. Say so in one line.
- **Match on a transition's TARGET status, not on its name.** A transition named `Start Progress`
  leads to the status `In Progress`, and the target is the field carrying the meaning. When the tool
  exposes only transition names, match those instead.
- **Match case-insensitively against `In Progress`, `In Development`, and `Doing`**, preferring an
  exact `In Progress`. Do not hardcode one string. Projects rename this status, and a hardcoded
  match turns a renamed workflow into a silent no-op that reads like a broken skill.
- **No match, or the transition needs fields this run does not hold.** No-op. Say so in one line and
  carry on.

**A failed transition never blocks the rest of Step 5 and never ends the run.** It is a board
update. Letting it abort would give the cheapest write in this skill the power to kill the work the
run exists to do. Report the outcome in the final report either way, so a reader can tell a no-op
from a move.

This write carries no gate of its own. It rides on Step 4, and invoking this skill on a key is
already the intent to work on that key. It also carries no authored prose, so there is nothing to
mangle in conversion and nothing to read back, which is why it skips the rollback artifact and the
read-back the description write gets below. The transition either took or it did not, and the
transition list you matched against says which.

**On a flag run only the second half of that first sentence is available, and it is enough alone.**
Step 4 did not run, so nothing rides on it. The invocation is the authorization, which is the same
warrant "Argument" gives every other ungated write on that path. Everything else in the paragraph
above holds unchanged, and the no-prose half of it is why this write needs no read-back in any mode.

**Draft with `writing-work-docs`.** Invoke it for both pieces. It carries the voice rules, the
banned words, the invent-nothing rule, and the no-hard-wrapping rule that keeps text from
arriving in Jira with ragged line breaks. Do not restate its rules here.

**`writing-work-docs` authors every human-facing artifact this run produces.** Not just the ticket.
The Jira description, the validation comment, the Step 4 option (a) comment, the commit messages, and
the PR title and body all go through it. There is no write path in this skill that drafts prose
without it. When an artifact has a template, hand it the template so it fills that rather than
inventing a shape.

### The publish override, declared once for the whole run

`writing-work-docs` refuses to publish, in three separate places, and a run that overrides one and
not the others gets blocked by whichever copy survived. All three are overridden here, for every step
of this skill, not just this one:

- its **Publishing** section, which says never publish anything,
- its **Hard rules** bullet `**Draft only.** Produce paste-ready text and stop.`,
- the **enumerated command list** under Publishing, which names `acli jira workitem edit` and
  `gh pr create` as writes it does not perform. `work-on` performs both.

Take everything else it says verbatim. This override is about who presses the button, not about its
prose rules, and its `TODO(user):` behavior in particular stays exactly as written.

**No confirmation on wording, at any step.** This skill does not show drafted prose and wait. The
distinction to hold: **`work-on` asks about decisions and never about wording.** Step 3's questions
and Step 4's a/b/c pick are decisions and they stay. A rendered description, a comment, a commit
message, and a PR body are wording, and none of them gets a gate.

The risk that confirmation used to carry is real and does not disappear: the ticket may not be the
user's, and the description outlives the conversation. Three things cover it now, and all three are
load-bearing rather than decorative. The Step 1 rollback artifact means the original description is
recoverable. The read-back below means a mangled write is detected rather than assumed. The "Final
report" means the user learns what was written, where, and whether it verified, without having to
ask.

**Description structure.** Use the ticket's own template if the project has one. Otherwise:

```
Problem            what is wrong now, in the reader's terms
Evidence           file:line, commit shas, PR links. one line each
Scope              what this ticket covers
Out of scope       what it does not, especially anything Step 4 cut, plus the follow-up's key
                   when one was filed
Proposed solution  the approach, and why not the alternatives
Open questions     only when non-blocking questions survived Step 3
```

Cut scope with no key reads as dropped instead of moved, and the key is what tells a reader
otherwise.

Drop any section the work does not touch rather than padding it. A rollout-percentage bump does
not need six headings.

**Comment structure.** The validation trail, so the next reader does not redo it.

```
Validated <date> against <sha> on branch <branch>.

- still reproduces at src/billing/tax.ts:88
- PR #812 (merged 2026-08-04) is not on this branch and changed adjacent code, not this path
- PR #830 (open) overlaps on the same file, so scope narrowed to the tax path only
- could not verify the claim about the nightly job. No such job exists in this repo.
```

On a bug or security ticket the trail gains the triage facts, in the same shape as every other line
here, each with its locator:

```
- path is live at src/billing/retry.ts:88, behind FLAG_RETRY_V2, and the flag is on in prod since 2026-07-02
- the error fires 340 times a day, from the Datadog query in this run
- a fix is size M, in src/billing/retry.ts, src/billing/retryQueue.ts, and src/billing/retryWorker.ts, plus a backfill over billing_invoice, which has no index on the filter column
```

**The recommendation itself is not published.** It has no locator of any accepted kind, and nothing
previews this write, so a judgement about somebody else's ticket would ship with no human having read
it. The facts belong in the trail because the next reader would otherwise redo them. The conclusion
belongs in the conversation at Step 4, where a human is present to disagree with it.

**A triage number nobody ran is a `TODO(user):` line**, exactly like any other unrun query. Do not
soften it into prose instead. `writing-work-docs` forbids hedging qualifiers, so there is no register
between a locator and a TODO line, and reaching for one produces a claim that reads as measured.

**Never a table.** A triage assessment is the natural table and a table is the single most common way
a ticket arrives mangled. One bullet per row, per [JIRA-FORMAT.md](JIRA-FORMAT.md).

Preserve the reporter's facts and links even when they read awkwardly. Say in one line what you
cut. Flag anything that looks wrong rather than silently fixing it.

**A number nobody ran is not evidence.** If a warehouse query was declined, skipped, or never
answered, the claim resting on it goes in as a `TODO(user):` line, not into Evidence. Paste the
query into that line so the next reader can run it. This is the step where an unresolved SQL handoff
would quietly become a fact on a public ticket, so check the handoff status before drafting Evidence.

**Record every `TODO(user):` line as you draft it.** Keep a running list from here to the end of the
run, each entry carrying the artifact it landed in, its locator, its text, and what would resolve it.
"Final report" reads that list out. Build it as you go rather than reconstructing it at Step 9 from
memory, because the ticket write and the report are four steps apart and a line drafted here is
exactly the kind of thing that goes missing in between. With no preview in front of this write, the
report is the only place a shipped `TODO(user):` line becomes visible to a human.

**The list belongs to the run, not to this write.** A flag run drops this write and still keeps the
list, because a `TODO(user):` line can be drafted later by Step 6, by a `--lite` Step 8's leftovers,
or into the PR body at Step 9, and Step 9's override of `open-pr` names the report as the single
compensating control for shipping one there unpreviewed. So a flag run starts the list empty at
Step 0 and appends to it from wherever its first entry actually comes. What the flag removes is this
step's contribution to the list, never the list.

**Write, then verify.** Write it. Then read the ticket back and confirm the formatting actually
landed, because a markdown-to-ADF conversion that half-worked leaves literal `##` and `**` sitting in
the ticket where everyone can see them. [JIRA-FORMAT.md](JIRA-FORMAT.md) has the write paths,
what survives conversion, the read-back check, the unverified-read case, and the hand-built ADF
fallback. Read it before the write, not after the check fails.

**The read-back is now the only control on this write, so its outcome is a reported fact.** It has
three outcomes and they are not two. Verified, could not read it back to verify, or failed. Carry
whichever one it was to "Final report" and put it there in those words. An unverified write is the
single thing the user most needs told, because nobody saw this text before it went out and a
half-converted description sitting on a public ticket looks exactly like a verified one from here.

**Because nothing was previewed, an automated restore is more dangerous than it used to be, not
less.** [JIRA-FORMAT.md](JIRA-FORMAT.md) bans restoring and bans hand-built ADF on a maybe. Its
reason is that a restore discards a rewrite. Do not read the absence of a preview as making that
cheap. No human has seen either version now, so a false-positive read-back that triggers a restore
destroys the new description while nobody is in a position to notice what was lost. The ban is
stronger here, not weaker. Leave the rollback artifact in place, report the check as unverified, and
stop.

**Description first, comment second, and treat them as two writes rather than one step.** The
description is the risky one, since it replaces content that already existed and is the only one
with a rollback artifact. Write it, verify it, and only then post the comment. That ordering means a
failure on the comment cannot leave you holding an unverified description at the same time.

If the description write succeeds and the comment write then errors outright, the description is not
at risk. Retry the comment on its own. Do not restore the description and do not redo the whole step,
which would revert a description that already passed its own read-back over a failure that never
touched it. That read-back is what makes the description trustworthy here, and the comment failing
says nothing about it. If the comment cannot be posted at all, say so and hand the user the drafted
text. The validation trail living in chat instead of on the ticket is a small loss. A reverted
description is not.

## Step 6: Do the work

Pick one process skill and announce which and why.

**`superpowers:systematic-debugging`** when behavior is already wrong and the cause is not yet
known. Signals: a stack trace, an error message, "returns X instead of Y", "stopped working
after", a failing test, a regression, a metric that dropped.

**`superpowers:brainstorming`** when the work is new behavior, or when the cause is understood
and the open question is what to build. Signals: "add", "support", "allow users to", a feature
request, several defensible approaches, or a bug whose root cause Step 2 already pinned down so
there is nothing left to diagnose and only a fix to choose.

When both fit, run `systematic-debugging` first to confirm the cause, then `brainstorming` for
the shape of the fix. When neither clearly fits, run `brainstorming`. A wrong pick here is cheap
to correct and stalling to decide is not.

Step 2 already produced the reproduction, the blast radius, and the constraints. Hand those to
the process skill rather than making it rediscover them.

**On a flag run none of those three exists, so the process skill does the discovering.** Hand it the
ticket and the repo conventions, which is what Step 1 kept, and say plainly that no reproduction was
established and no blast radius was measured. Do not synthesize either one to fill the handoff. This
is also why the pick above matters more on this path than on the full one: `systematic-debugging`
starts by establishing the behavior, and on a flag run nothing has, so a bug ticket that would have
gone to `brainstorming` after Step 2 pinned the cause has no pinned cause to build on.

### Take Subagent-Driven execution without asking

The brainstorming path runs on into `writing-plans`, and that skill ends with an "Execution Handoff"
section offering two ways to implement the plan it just wrote:

1. **Subagent-Driven** (which it already marks recommended), a fresh subagent per task with review
   between tasks, via `superpowers:subagent-driven-development`.
2. **Inline Execution**, batched in the current session with checkpoints, via
   `superpowers:executing-plans`.

**Pick Subagent-Driven. Do not put the question to the user.** Say in one line that you are taking it
and keep going.

The prompt comes from `writing-plans`, not from `brainstorming`, so it arrives two skills downstream
and after the plan is already written and saved. That is the moment to recognize, and it is easy to
miss because by then several skills have handed off in sequence.

Two reasons the choice is already made here. A `work-on` run is long and mostly unattended, and its
whole design is to spend the user's attention on the validation questions in Step 3, where only a
human can answer. Asking them to pick an execution mode they picked the same way last time spends
that attention on nothing. And a fresh subagent per task keeps one task's context from bleeding into
the next, which matters more than usual on this path, because Step 2 has already filled the main
thread with validation findings, telemetry digests, and merged-PR diffs.

**A flag run loses both of those reasons and keeps the default anyway, on a third one.** Step 3 does
not run, so there are no validation questions the user's attention was being saved for, and Step 2
does not run, so the main thread arrives here nearly empty. Neither reason survives. What replaces
them is the flag itself: a user who asked for `--lite` or `--fast` asked for fewer interruptions, so
an extra question about execution mode is the last thing that path should produce. Take
Subagent-Driven, say so in one line, and keep going. The rule holds in every mode. Only its
justification changes.

If the user asks for inline execution, in the invocation or at any point during the run, take it. This
is a default, not a policy. Anything else `writing-plans` asks for still goes to the user as normal.
This overrides only the execution-mode question.

## Step 7: Commit and push

Commit everything the work produced.

**`writing-work-docs` writes the commit messages too, and it gets handed the repo's convention.** A
commit message is prose a human reads, so it belongs to that skill like every other artifact here.
It already looks up local convention with `git log --format='%s%n%b' -20` and it already follows an
artifact's own template. Give it the three things it cannot infer from this repo on its own:

- the conventional-commit types from `.github/semantic.yml` when that file exists,
- the ticket key, which goes in the body,
- the trailer convention below, which is the one place its no-branding stance does not apply.

**The `Co-Authored-By` trailer survives.** `writing-work-docs` has no commit-trailer rule, so its
general no-agent-branding stance would otherwise arrive by default and strip it. It does not apply to
a commit trailer. See the constraint at the end of this file for why: a trailer is metadata rather
than prose, git has a standard field for it, and every other commit in this repo's history carries
it. Tell the skill to keep it rather than letting it decide.

**Put the ticket key in the commit body.** Never in a code comment. This is not bookkeeping.
Step 8's ticket-compliance reviewer finds its ticket by regexing `[A-Z][A-Z0-9]+-[0-9]+` out of
the commit messages in range, so a commit trail with no key means that reviewer silently finds
nothing to check and reports no issues. The one reviewer positioned to catch a build that drifted
from the ticket you just rewrote is the one you disable by forgetting this.

**The rule holds in every mode, and its reason changes in both.** Under `--lite` Step 5 rewrote
nothing, so that reviewer checks the build against the ticket as filed, which Step 8 says is worth
more on that path rather than less. Under `--fast` the reviewer does not run at all, and the key
still goes in, because the commit trail outlives this run. Whoever reviews this branch by hand, and
`review-and-fix` if it is invoked on the branch later, both find the ticket the same way.

Push the branch. **Do not open a PR.** Step 8 runs against the branch range, and Step 9 opens
the PR once the review loop has stopped finding things. A PR opened here collects review noise
on commits that are about to be rewritten.

**Pop the Step 0 stash now, if you made one and it is still there.** The work is committed, so a
restore can no longer be mistaken for part of it.

Apply the **sha** you recorded in Step 0, not `git stash pop` and not `stash@{0}`. Those two are the same
operation, and both take whatever is on top of the stack, which by now may be an entry Step 6 or Step 8
created, or one the user made in another terminal. Find your entry by sha and apply that one:

```
git stash list --format='%H %gd %s'
git stash apply <recorded-sha>
git stash drop <the stash@{N} whose sha matched>
```

`apply` then `drop` rather than `pop`, so a conflict leaves the entry in the stack instead of consuming
it. Check the sha is still listed first: Step 4 settles the stash on the paths where the run ends there,
so on those paths there is nothing left to apply and a blind attempt errors.

Step 8 needs a clean tree, so if the pop leaves changes in the tree, tell the user and settle it with
them before starting the review loop rather than sweeping their work into a review commit.

## Step 8: review-and-fix, all the way

**Skipped entirely by `--fast`. Runs in full under `--lite`, with one carve-out named below.** On
`--fast` no reviewer runs, `Iterations` reports the flag rather than a count, and Step 9 still opens
the PR. See "Argument".

Invoke `review-and-fix`. Let it run every iteration it wants.

That skill stops on its own terms, and it has no iteration cap by design. Its loop continues
only while each pass commits at least one real fix, so a long run means it is still finding
things. Do not interrupt it, do not summarize partway and call it finished, and do not pass any
flag that shortens it.

**"Any flag that shortens it" means a flag you pass down to `review-and-fix`, not `--fast` arriving
from above.** The two are opposite directions. `--fast` is the user removing this step, decided
before the run started and disclosed in the report. Shortening the review from inside is this run
quietly getting less than it asked for. So `--fast` skips the step whole and never invokes the skill
with something that truncates it, and there is no middle setting where the review runs briefly.

Three things to get right when invoking it:

- **Do not pass `--skip-ticket`.** Step 0 confirmed Jira access, so the ticket-compliance
  reviewer can run. Checking the diff against the ticket is worth more here than anywhere else,
  because Step 5 just rewrote that ticket and this is what catches a rewrite that drifted from
  what got built. There is no flag for passing the key. That reviewer reads it out of the commit
  messages, which is why Step 7 puts it there. Confirm it did before trusting a clean result.
  **Under `--lite` the reason inverts and the instruction stands.** Step 5 rewrote nothing, so that
  reviewer checks the build against the ticket exactly as somebody else filed it, unvalidated. That
  is the only check in a `--lite` run comparing the code to the ticket's own words, which makes it
  worth more on this path than on the full one rather than less.
- **It runs in branch mode**, since no PR exists yet. That is expected. Discussion Context comes
  back empty and nothing is wrong.
- **Step 8 runs in the main thread.** Never wrap `review-and-fix`, or any reviewer, in a `Workflow`.
  A workflow agent has no `Agent` tool, so a skill that fans out into sub-agents has nothing to fan
  out with inside one. `review-and-fix` cannot launch its reviewers, and `in-depth-review` cannot
  launch its eight to twelve roles. What comes back is an abort with no coverage, not a review.
  Step 2 is what makes reaching for a workflow here feel natural, and it is the contrast rather
  than the precedent. Its seven lenses, its telemetry probes, and its triage agents are all leaf
  readers that spawn nothing, so they lose nothing inside a workflow agent. A reviewer is itself
  a fan-out, so it loses everything.

It commits each fix and never pushes. Step 9 pushes.

### Follow-ups from what review-and-fix left

`review-and-fix` has no out-of-scope category. Its "Remaining Issues" section covers findings
that did not become a commit, and the reasons it names are a row 2 stop, a fix abandoned because
lint or tests failed, and a finding skipped for want of a test. Its "Tickets examined" section
carries deferred and dismissed ticket findings with a decision. Nothing in either section says a
finding belongs to a different ticket. The classification below is `work-on`'s own judgement
against the scope agreed at Step 4, not something the review reports for you.

Once `review-and-fix` returns, read both sections and sort every item in them into exactly one of
two buckets.

**Unfinished here.** A fix abandoned because lint or tests failed. A finding skipped for want of a
test. A row 2 stop. None of these becomes a ticket. Step 8 says run `review-and-fix` all the way.
Filing a ticket for a test failure converts that failure into a backlog item that reads like
planned work, so the failure stops being visible as a failure. This is the rule a well-meaning run
is most likely to break, so check every item against it before proposing anything.

**Belongs to a different ticket.** A finding about code that the scope agreed at Step 4 does not
cover. Only items in this bucket get proposed.

**Under `--lite` this bucket is empty and nothing here files anything.** Its membership test is
"outside the scope agreed at Step 4", and on that path no scope was ever agreed, so the test has no
value to evaluate rather than a value of false. "Argument" has the reasoning under "What still runs".
Two things it does not say, which belong to this bucket alone. Do not substitute your own reading of
the ticket for the agreement. And do not fall back to the ticket as filed either, since nothing
validated it and the whole reason a scope gets agreed at Step 4 is that the filed one is
untrustworthy. Every leftover goes to "Final report" unfiled and named, with the flag as the reason.

Put an item you cannot confidently place into "Unfinished here." The cost of leaving real
out-of-scope work unfiled is that somebody finds it later. The cost of filing unfinished work as a
ticket is that the branch ships with a known gap wearing a ticket number.

**The batch approval.** Propose the second bucket as one batch with one yes, at the end of Step 8,
once the loop has stopped. The Step 4 cut needs no separate approval, because the user's agreement
to the reduced scope there already covers it. This bucket needs its own, because these findings
did not exist when that agreement was made, so it cannot cover them. **Under `--lite` there is no
Step 4 agreement to reason from and no batch to approve**, since the bucket above is empty. Never
read the sentence about the Step 4 cut as a standing approval on that path. No approval of any kind
was taken before Step 6, so nothing here inherits one. Present each candidate in the
same compact block Step 4 uses for its own proposal.

```
Type      <Story, Task, or Bug>
Summary   <one line>
Points    <n>
Parent    <key, or none>
```

Say nothing when the second bucket is empty. A report with nothing to decide trains the user to
skim exactly the messages that do carry a decision.

**The cap.** The batch approval above does not change when the second bucket holds more than three
items. One batch, one yes, and nothing gets filed without that yes, whatever the count. Findings
from Step 8 are machine-generated and carry no human scope call of their own, so a noisy review
round could otherwise turn one `work-on` run into a stack of tickets nobody asked for. A user who
reads every candidate and says yes has just supplied that human scope call directly, so the cap
does not sit between that yes and the create. A cap that overrode an explicit yes would not be a
safety rail. It would only block work somebody already approved.

What the cap adds above three is a reporting step, not a block. Report the count, and say plainly
that a count this size is a signal about the change itself and not just a list of tickets. Then
propose the batch the same way as below the cap, one compact block per candidate, and let the same
single yes decide whether to file all of them.

Do not file past the cap without first reporting the count and the signal. Filing everything past
three with no word about either is the exact noisy-review-round outcome the cap exists to surface.
Do not truncate the list without saying so either. A silently truncated list reads as "that was all
of them" when it was not, and a yes given on a truncated list is a yes to a batch the user never
actually saw.

**Filing.** Reuse Step 4's "### Filing the follow-up" subsection by name, one call per approved
item, rather than writing a second filing path here. Two copies of the same filing rule drift
apart over time, and a follow-up filed from a Step 8 leftover would then silently follow
different rules than one filed from the Step 4 cut, for no reason anyone decided on purpose.

It already carries the points gate, the delegated invocation, the Step 9 approval substitution,
and the main-thread-only rule. The single yes from the batch approval above is the recorded
agreement each of those calls passes in place of Step 9's gate.

Timing does not carry over the same way. This call fires at the end of Step 8 once the loop has
stopped, the timing already stated above, not after Step 4's agreement the way Step 4's own filing
runs.

The points gate's remedy does not carry over unchanged either. The threshold stays the same. An
item scored over 5 still does not get invoked. Step 4's remedy tells the user its proposed split
was too coarse and asks them to narrow a cut that is theirs to narrow, but there is no such split
at Step 8. The candidate here is a review finding, so say instead that the finding is too large to
file as one follow-up, and let the user decide what to do with it, rather than pointing them at a
narrowing that was never theirs to do.

One more thing changes beyond the mechanics above. This run's own ticket key, not a parent's, is
what gets supplied for the dedup exclusion and the description's context line. The
sibling-parenting rule from `open-ticket`'s "Delegated entry" applies the same way it always does.
A finding about adjacent code is a sibling of this ticket at best, and never its child.

One call per approved item can leave a partial batch, and this is the uncommon ending rather than
what a batch normally does. `open-ticket`'s Delegated entry runs its dedup sweep and gate on every
call, and a sibling follow-up already filed off the same originating ticket is judged on the same
two credibility tests as any other match. Two candidates out of one review round are usually about
different code, so the earlier call's freshly created sibling does not describe the same change and
does not stop the later call. When it does describe the same change, the later call aborts with
`DUPLICATE_FOUND` and the batch ends partway through, and what that abort has found is two
candidates that were the same work seen twice. Report a partial batch as what was created and what
was not, each with its key when it has one. Do not suppress the sweep and do not special-case it
here, since the sweep finding a real sibling is correct behavior. The reporting is what keeps a
partial result visible instead of silent.

A create that fails here gets the same treatment Step 4's subsection already gives a failed
create. Say in "Final report" that the follow-up could not be filed and why. No later step in this
run consumes the new key the way Step 5's "Out of scope" section consumes the key the Step 4 cut
creates, so a failed create here blocks nothing downstream from running.

### When `review-and-fix` aborts with `REVIEW_UNAVAILABLE_NO_FANOUT`

**This subsection is about a Step 8 that broke, never a Step 8 the user skipped.** `--fast` produces
the same physical state, code that no reviewer looked at, and must not be routed through anything
below. **An unavailable reviewer is a failure. A skipped reviewer is an instruction.** Only the
failure stops the run, because only the failure means the run got less than the user asked for. On
`--fast` the run has exactly what was asked for, so it continues to Step 9, opens the draft, and
discloses the skip in "Final report". Nothing here applies to it, including the stop, the one
re-invocation, and the sentinel.

That sentinel means the `Agent` tool was absent where the skill was invoked, so not one reviewer
could be launched. No review happened.

**The literal token is the wire contract.** `review-and-fix` emits `REVIEW_UNAVAILABLE_NO_FANOUT`
verbatim, and everything below keys on that exact string. A paraphrase of the reason without the
token is not an abort this step can recognize. So a `review-and-fix` run that comes back with
neither the token nor its Final Report is an abort too. Treat it as one.

Two moves, in order.

**Re-invoke it once, directly, from the main thread.** If Step 8 somehow ran inside an agent that
has no `Agent` tool, such as a workflow agent, this is the whole fix and it costs one invocation.

**If it aborts a second time, stop the run.** Do not proceed to Step 9 and do not open a PR. Step 9
already says why a draft is the correct end state, which is that the work is reviewed by Step 8 and
unreviewed by a human, and that is exactly what draft means. A second abort makes that sentence
false. The PR would then claim a review it never got. A draft cannot disclose the gap either,
because a draft already reads as reviewed by the pipeline and not by a person.

Emit "Final report" before you stop, the same as the `open-pr` failure path does. Step 5's Jira
writes already happened, so any `TODO(user):` line that shipped has to be reported whether or not a
review ran. A follow-up filed from the Step 4 scope cut, if the run filed one, gets reported the
same way, since the run never reached the point in Step 8 where a leftover could be proposed. Put
the sentinel where the `PR` URL would go, write `0` on the `Iterations` line, and
name the branch so the user can invoke `review-and-fix` from a context that has the `Agent` tool and
pick the pipeline back up at Step 9.

**`0` on the `Iterations` line belongs to this ending and to no other.** It reads as "the review was
supposed to run and could not", which is what the sentinel in the `PR` field beside it confirms. A
`--fast` run also performed zero passes but has a real PR URL, so writing `0` there would describe
this failure on a run that had none. `--fast` writes `skipped (--fast)` instead, and the two are
never interchangeable. Two zero-pass runs that ended differently must not report the same number.

## Step 9: Push and open a draft PR

Push the review-and-fix commits. Then invoke `open-pr` with `--draft`.

**The PR is not a question.** It always opens, and it always opens as a draft. Do not ask whether to
open it, do not ask whether draft is right, and do not offer ready-for-review as an alternative. A
draft is the correct end state for this pipeline: the work is reviewed by Step 8 and unreviewed by a
human, which is exactly what draft means.

**Under `--fast` the draft means less and still opens.** Step 8 did not run, so the sentence above
does not hold and a draft here means only that no human has reviewed it. That is a weaker claim, not
a false one, and it is still the right end state. The PR is not a question on this path either. Do
not ask whether to open it, and do not offer ready-for-review, since the code has had less review
than usual rather than more.

**Per an explicit decision the PR body carries no note that no review ran.** So do not add one, and
do not let `open-pr` infer one. The disclosure lives on the `Mode` and `Iterations` lines of "Final
report", which makes those two lines the entire record that this branch was never reviewed. A reader
who sees only GitHub cannot tell a `--fast` PR from a fully reviewed one, and that is the accepted
consequence of the decision rather than an oversight to correct here.

**The one exception is a Step 8 that could not run at all.** That justification rests on Step 8
having reviewed the work, so a `REVIEW_UNAVAILABLE_NO_FANOUT` abort that survives its one
re-invocation leaves nothing for a draft to mean. The run stops in Step 8 on that path. Its abort
section has the detail. **"Could not run at all" means broke, not skipped.** `--fast` also produces a
Step 8 that did not run, and it is not this exception. The abort section's opening draws the line.

`open-pr` carries the title rules, the template detection, and the `writing-work-docs` routing.
It owns all of that. What it does not get to own here is its own gates.

### Overriding `open-pr`, by name

`open-pr` was written to be invoked by a human, so it asks a human several things. Four of those are
suspended for this run. Name all four, because it states each rule in more than one place and a run
that overrides one copy obeys the copy still standing:

**All four stay suspended on a flag run, and the report they lean on is what makes that safe.** This
is the densest set of suspended human gates in the file and it sits on a remote write, so it is worth
being explicit that a flag does not touch it. What holds it up is unchanged: the invocation is the
authorization, exactly as "Argument" says for every other ungated write on that path. What does
change is that "Final report" is now the only control left anywhere in the run, since the validation
phase that normally sits in front of all of this did not happen. That is why the running
`TODO(user):` list is kept from Step 0 in every mode, and why the roll-call below is not optional on
a flag run. Suspending four gates while dropping the one thing that stands in their place is the
failure this paragraph exists to prevent.

- **Its Step 5 "Preview and confirm" gate.** No `Ready to open this PR?`. The invocation is the
  authorization.
- **Its Constraints restatement** of that same gate, `Always preview before posting. User must
  confirm.` This one is short, absolute, and sitting in Constraints, which makes it the copy most
  likely to win a conflict. Overriding Step 5 and leaving this is the most probable way this
  override fails.
- **Its `gh pr create` step being "gated on the Step 5 preview".** The gate is gone, so the write
  is ungated. Read that phrase as describing a gate that no longer exists rather than as a
  precondition to go satisfy.
- **Its push confirmation**, `ok to push?`. Step 7 and this step already pushed without asking, so
  a confirmation here would be asking permission for a push that already happened. Normally
  `open-pr` finds nothing to push and skips it. Do not rely on that. A partially failed push or a
  commit landing between the two puts it back.

**`TODO(user):` lines ship in this draft body.** `open-pr` says a `TODO(user):` line never ships, in
four separate places, and resolves them at the preview that no longer runs. Overridden, narrowly: a
`--draft` PR opened by `work-on` keeps its `TODO(user):` lines in the body. Do not strip them and do
not ask about them. Stripping would delete a known gap, which contradicts this skill's rule that
unverified stays unverified. The compensating control is that every shipped line is named in "Final
report", so the gap is disclosed rather than hidden.

**That control is fed by the running list Step 5 normally starts, and a flag run starts it at Step 0
instead.** Step 5's drafting block is where the full run establishes the list, and both flags drop
that block, so a run reading only the surviving steps would ship these lines with nothing recording
them. "Argument" moves the list's start to Step 0 for exactly this reason. Under `--lite` and
`--fast` the entries come from Step 6, from a `--lite` Step 8's leftovers, and from this body.

One wording trap in `open-pr` to not lean on. Where it says such lines are "correct in a draft and
wrong in a live PR", "draft" there means a draft of the prose, not a GitHub draft PR. It does not
already permit this. It has to be overridden.

**Direction matters here, and confusing it reverts the whole override.** `open-pr` declares "three
overrides, and only three". That closed list is about `open-pr` overriding `writing-work-docs`. This
section is `work-on` overriding `open-pr`, which is a different relation between different skills.
The closed list does not forbid it. Read "only three" as a limit on that other relation, or the run
concludes the caller may not override anything and honors the preview gate after all.

Two additions from this run:

- Give it the ticket key and URL so `Goal` can link the ticket instead of paraphrasing it.
- Give it the validated framing from Step 4. The ticket as originally filed may no longer
  describe what got built, and the PR should describe what got built. **On a flag run there is no
  validated framing, so give it the diff and the commit messages instead.** Those describe what got
  built and they are the only things on that path that do. Do not pass the ticket's own framing as
  though it had been validated, and do not synthesize a framing to fill the slot. The rule behind
  this bullet still holds and is what decides the substitute: the PR describes what got built.

After it returns the URL, fetch the created PR body back and read it. Confirm the template
sections came through, the ticket link resolves, and no section is sitting empty. GitHub renders
markdown natively so there is no conversion to distrust here, but a template field silently left
blank is still worth catching before a reviewer finds it.

**If `open-pr` fails, the push already happened.** Do not report the run as finished and do not retry
blindly. Say the branch is pushed and safe, name it, surface the exact error `open-pr` gave, and hand
the user the drafted title and body so they can open the PR themselves. Common causes are a protected
base branch, a template that could not be resolved, and a plain API error, and none of them are worth
guessing at. The work is committed and pushed either way. Only the PR is missing, so say exactly that
rather than letting a success-shaped summary imply otherwise.

**Still emit "Final report" on that path.** The Jira writes already happened, so the ticket is live
and may be carrying `TODO(user):` lines with no PR to report them against. It may also be carrying
a follow-up filed from either trigger, with no PR to report that against either. Emit the report
with `PR` as the error instead of a URL, and every other field filled in from what actually
happened, the `Ticket` parenthetical included.

## Final report

**The run ends with this report, and it is its own message.** Nothing else in it. Not a preamble, not
a next-steps offer, and not an outstanding SQL request, which gets its own separate message per
[DB-QUERIES.md](DB-QUERIES.md). The report is where a run with no preview gates accounts for
everything it wrote while nobody was looking, so it is the one output that must not be skimmable
past.

**The `Mode` line is part of the report, not something else in it.** "Nothing else in it" bans a
preamble and an unrelated request. It does not ban a field. On a flag run the `Mode` line is the only
place the run discloses that it did less, and under `--fast` it is the only place in the world that
records that nothing reviewed the code, since the PR body carries no such note. Read the rule as
protecting the report from padding rather than as fixing its field list.

```
Mode          full | --lite | --fast   (steps cut: <list>)
PR            <url>  (draft)
Ticket        <url>  (description: rewritten | write failed | skipped (<flag>); comment: posted | not posted | skipped (<flag>))
Status        moved to <name> | already <name> | no matching transition | write failed
Follow-ups (<n>)
  - <key>  filed from the Step 4 scope cut  <one-line summary>
  - <key>  filed from a Step 8 leftover  <one-line summary>
  - could not file  <Step 4 scope cut | Step 8 leftover>  <why>
  - not filed  Step 8 leftover  no Step 4 scope was agreed (--lite)  <one-line summary>
Branch        <name>
Jira write    verified | could not read it back to verify | failed | nothing to verify (<flag>)
TODOs (<n>)
  - <artifact> <locator>  <the line's text>  -> <what would resolve it>
Validation    <one line on what it changed about the ticket> | skipped (<flag>), every claim in the ticket is unverified
Iterations    <n> review-and-fix passes | 0 | skipped (--fast)
Stash         restored | still stashed at <sha> | none
```

**`Mode` goes first, because it governs how every line under it reads.** A `Validation` line saying
`skipped` is a flag working as asked when `Mode` says `--lite`, and a bug when `Mode` says `full`.
Name the steps cut explicitly rather than leaving the reader to look up what the flag does: `2, 3, 4`
for `--lite` and `2, 3, 4, 8` for `--fast`. Write `Mode full` on an unflagged run rather than omitting
the line, for the same reason zero TODOs is stated below. A missing `Mode` line and a `full` one are
not distinguishable to a reader, and the whole point of the field is that a reduced run looks
different from a complete one.

**`Iterations` distinguishes three states and never conflates them.** A count is a review that ran.
`0` is the `REVIEW_UNAVAILABLE_NO_FANOUT` abort, where the review was supposed to run and could not,
and it always appears with the sentinel in the `PR` field. `skipped (--fast)` is a review the user
removed, and it appears with a real PR URL. The last two are both zero passes and they mean opposite
things about whether the run did what was asked.

**`Jira write` has four values and each is exact.** `nothing to verify (<flag>)` is the one for a flag
run, where the status move is the only write and the transition list already confirmed it, so there is
no authored content to read back. Do not print `verified` there. That word reports a read-back that
did not happen, and the other three values all assert one did.

**Every `TODO(user):` line that shipped gets its own line, wherever it landed.** The Jira
description, the validation comment, the option (a) comment, the PR body. Read them off the running
list, which Step 5 starts on a full run and Step 0 starts on a flag run, so it exists either way. On
a flag run only the PR body is among those four landing places, and the roll-call matters at least as
much there, since it is the only control left on that write. Never summarize them as a count, never
fold two into one line, and
never write "some open questions remain". The whole point is that a human who never saw these
artifacts before they went out can see, in one place, exactly what went out unverified and where to
find it.

**Zero TODOs is stated, not omitted.** Write `TODOs (0)  none`. A missing section reads identically
to a forgotten one, and the reader cannot tell which they are looking at.

**Every follow-up this run created or attempted gets its own line, naming its key and which case
created it, or naming why a create failed.** The Step 4 scope cut and a Step 8 leftover are the
two cases, and each filed key says which one filed it. `<n>` counts every line in the list, filed
keys and failed creates together, never successful creates alone. A run that creates a Jira ticket
and never says so hands the user a ticket they do not know exists, and the whole point of
reporting a create is that nothing previewed it before it went out.

**A follow-up that could not be filed gets its own line too, naming why.** Step 4's "### Filing the
follow-up" subsection, and the "### Follow-ups from what review-and-fix left" subsection that
reuses it, both say a failed create never blocks the rest of the run. This line is where that
failure becomes visible to a human instead of disappearing into a step that just moved on. A run
with one failed create and no successful ones still shows that one line, because the count above
already includes it.

**Zero follow-ups is stated, not omitted, for the same reason as zero TODOs above.** Write
`Follow-ups (0)  none` only when neither trigger produced any candidate at all, filed or failed. A
missing section reads identically to a forgotten one here too.

**On a flag run neither trigger exists, and that is a different state from producing nothing.** The
Step 4 cut cannot happen because Step 4 did not run, and under `--lite` the Step 8 bucket is empty by
rule rather than by outcome. Write `Follow-ups (0)  no trigger ran (<flag>)` rather than a bare
`none`, so a reader can tell a run that looked and found nothing from a run that never looked. A
`--lite` Step 8 that did produce leftovers reports each one on a `not filed` line, and those count
toward `<n>` exactly as failed creates do.

**`Jira write` uses those exact words.** `could not read it back to verify` is the phrase
[JIRA-FORMAT.md](JIRA-FORMAT.md) requires for that outcome. Unverified is its own result. Do not fold
it into either verified or failed, and do not soften it, because it is the outcome the reader most
needs to act on.

**Every field is filled from what happened, never printed as written.** The skeleton above is a
shape, not a sentence to copy. This matters most on the `Ticket` line, because Step 5 is two
independent writes and either can fail on its own. Fill each half separately. `comment: not posted`
is the right value whenever the comment errored out per Step 5, or was deleted rather than left
half-converted per [JIRA-FORMAT.md](JIRA-FORMAT.md), and on that value say again where the drafted
comment text was handed over, since the validation trail then exists only in chat. Never print
`posted` for a comment that is not on the ticket. `Jira write` does not cover this: it reports the
read-back on content that did land, so it can honestly read `verified` on a run whose comment never
posted at all.

**`skipped` and `not posted` are different values and a flag run uses the first.** `not posted` means
the write was attempted or drafted and did not land, which is why it carries the extra duty of saying
where the drafted text went. On a flag run nothing was drafted, so there is no handover to point at
and nothing was attempted. Print `skipped (<flag>)` on both halves and add no handover note. Reading
`not posted` there would send the user looking in the conversation for a comment that was never
written.

**No run ends with a published `TODO(user):` line unmentioned.** This holds on every ending, not just
the one that reaches this step. The Step 4 option (a) exit, the Step 8 no-fanout abort, and the
`open-pr` failure path all emit the roll-call too. Same rule as the stash disclosure, for the same
reason: it is the user's problem now, and they cannot see it from where they are sitting.

**No run ends with a filed follow-up unmentioned either.** A follow-up can reach Jira well before
this step runs, since a Step 4 cut files before Step 5 and a Step 8 leftover files before Step 9.
The Step 8 no-fanout abort and the `open-pr` failure path both name every follow-up filed by that
point, the same way each already names its `TODO(user):` lines. The Step 4 option (a) exit does not
carry this risk. That exit belongs to a dead ticket and a Step 4 follow-up belongs to a partially
valid one, so the two never happen in the same run.

**No run ends with its mode unmentioned.** This is the same rule as the two above and the stash
disclosure, applied to the thing the flags introduce, and it binds on every ending rather than only
the one that reaches this step. A Step 0 abort, a Step 6 that hands back, a `--lite` Step 8 that
aborts twice, and an `open-pr` failure all state the mode. The reason is sharper here than for the
others. A `TODO(user):` line and a filed follow-up both leave a trace somebody can find later, on
the ticket or the board. A skipped validation phase leaves no trace at all, and under `--fast`
neither does a skipped review, since the PR body is silent by decision. So the report is not the
best record of what the flags did. It is the only one, and a run that omits it has made the skip
undetectable rather than merely undocumented.

## Constraints

- **The two flags cut work and never cut disclosure.** What each one cuts is "Argument" and the
  Pipeline table's `Cut by` column, and this bullet deliberately does not restate it. What belongs
  here is the reading rule: every bullet below that names a validation product is qualified by those
  two, and where a bullet justifies itself by pointing at the validation phase, a flag run keeps the
  rule and loses the justification. The invocation is what authorizes the write instead. No flag adds
  a gate back and none removes one beyond what "Argument" names.
- **No run ends with its mode unmentioned.** On every ending, including the ones that never reach
  "Final report" in full. The skip leaves no trace on the ticket, the board, or the PR, so the report
  is the only place it is visible at all.
- **Nothing writes before Step 5 unless the user asks for it.** Steps 0 through 4 are read-only
  against Jira, GitHub, and the working tree on their own initiative, which is what makes the
  validation phase safe to run aggressively. There are two exceptions and neither one moves without
  an answer from the user first. Step 4's option (a) posts a comment, and it happens only after the
  user has seen the evidence and chosen it. "Filing the follow-up" creates a Jira issue between
  Step 4 and Step 5, and it happens only on the agreement to the reduced scope. Leave that second
  one off this list and a later reader treats the Step 4 filing as a bug, which is a bad way to
  discover a create that has no rollback.
- **Decisions get asked about. Wording never does.** Step 3's questions, Step 4's a/b/c pick, the
  follow-up proposal Step 4 presents alongside the reduced scope, and the one yes over Step 8's
  batch of leftover follow-ups are decisions and they stay. The Jira description, the Jira comment,
  the commit messages, the PR body, and every follow-up's drafted prose are wording. None of them
  gets a preview, a confirmation, or an "does this look right?". Both follow-up decisions belong on
  this list because each one authorizes a Jira create, and a create nobody was asked about is the
  one thing the wording rule must never be read to allow. **On a flag run all four named decisions
  are gone**, the first three with the steps that hold them and the fourth by rule, so a `--lite` run
  asks the user nothing from Step 0 to Step 6. That does not promote wording to askable. It means the
  run makes no decisions, which is what the flag traded for.
- **Every write is verified after the fact, since none is approved before it.** Read the Jira ticket
  back, fetch the PR body back. Report the Jira read-back's outcome in "Final report" using its three
  exact words. Verification replaces the preview, so skipping it leaves a write with no control on it
  at all. **A flag run has no authored Jira content to read back**, since the status move is its only
  Jira write and the transition list already confirmed it, so `Jira write` takes its fourth value and
  the PR body fetch still happens.
- **Nothing that shipped unverified stays unmentioned.** Every `TODO(user):` line that reached Jira or
  a PR body gets its own line in "Final report", on every ending the run can have. These artifacts
  outlive the conversation and the user did not read them before they went out, so the report is their
  only view of what went out.
- **An invalid or superseded ticket stops the run.** Step 4 writes nothing on its own initiative. It
  never closes the ticket, and it does not comment or start a branch until the user picks one of
  Step 4's three options. Executing option (a) afterwards is the user's decision being carried out,
  not this rule being broken. **A flag run cannot reach this rule and does not get to claim its
  protection.** Nothing produced a verdict, so no ticket is found invalid or superseded and none
  stops the run. This is the flags' central cost rather than a gap to patch: a `--lite` run works a
  dead ticket through to a PR and never notices. Say so in the announcement, and never report a flag
  run's ticket as valid on the grounds that nothing stopped it.
- **Every claim carries a locator.** `file:line`, a commit sha, a PR number, a Jira key, or a Datadog
  or Amplitude query with its result. A claim without one is reported as unverified.
- **A related ticket found by text search is not a duplicate until somebody reads it.** A Jira link
  type is a person's judgement and carries weight. A `"search"` hit is two tickets sharing a word.
  Never close, narrow, or supersede a ticket on the strength of a search hit alone.
- **Search before you ask.** A question answerable by `rg`, `git log`, `git blame`, the ticket's
  comment thread, an existing PR, a related Jira ticket, Datadog, or Amplitude is not a question. See
  "Earn every question".
- **Try a query skill before asking, then never improvise past it.** An installed skill that declares
  it runs SQL is the one sanctioned way to run a query here. No database client, no connection string,
  no credentials from a file or the environment. See [DB-QUERIES.md](DB-QUERIES.md).
- **A warehouse ask is a file, not a paragraph.** Numbered queries plus an absolute path plus a
  follow-up on every turn until it resolves. Silence is never a decline.
- **Every warehouse query is `SELECT` only, and always bounded.** No statement that mutates data or
  schema. A date filter, an id range, or a `LIMIT` on every query, aggregates included. Assume nothing
  checks this for you, and note the bound matters most when a skill runs it, because then no human
  reads it first. See [DB-QUERIES.md](DB-QUERIES.md).
- **Never guess a number.** Not from a query that could not run, not from one the user declined,
  not from reasoning about table sizes. An unrun query leaves an unverified claim, and unverified is
  what the ticket says.
- **Never assume to avoid asking either.** The two failure modes are asking what you could have
  looked up and guessing what you could only have asked. Investigation separates them.
- **The workflow is optional.** Small surface, investigate inline. Wide surface, fan out. It
  runs once either way, never in a loop, and it is for Step 2's validation lenses only, never for
  Step 8's reviewers. **A flag run takes neither path.** "Optional" here is a choice between two ways
  of validating, and a flag run does not validate, so it is not the inline path with the ceremony
  removed. The inline path is a real pass over the code that reports what it found.
- **Never invent** a ticket key, a PR number, a dashboard link, a date, a metric, or a test
  result. Unknown becomes a `TODO(user):` line, and every one of those reaches "Final report".
- **A triage recommendation never gets published.** The facts go into the Jira comment with their
  locators. The recommendation stays in the conversation at Step 4. Step 5 explains why.
- **`worth_fixing: 'no'` requires evidence that reaches.** A query with a result, a flag state at a
  `file:line`, or a precondition chain read out of the code. An unmeasured impact is `unclear`, never
  `no`. "No telemetry covers this path" and "nobody is affected" are opposite conclusions and only one
  of them stops work.
- **A flag run produces no triage at all, so the two bullets above have no subject.** No `kind` was
  recorded, no triage agent ran, and no `worth_fixing` value exists in any state, including `unclear`.
  A flag run therefore never reports whether the work was worth doing, in the conversation or on the
  ticket. It is not that the impact came back unmeasured. Nobody asked.
- **Verify the Jira formatting by reading it back.** Do not trust that the markdown converted. A
  flag run writes no markdown to Jira, so there is nothing to convert and nothing to distrust.
- **Execution mode is not a question.** When `writing-plans` offers its Execution Handoff choice,
  take Subagent-Driven (`superpowers:subagent-driven-development`) and say so in one line. Every other
  question that skill asks still goes to the user. Honor an explicit request for inline execution.
- **The PR is not a question either.** It always opens, always as a draft, unless Step 8 could not
  run at all. **"Could not run at all" means Step 8 broke, never `--fast`.** A `--fast` run has no
  Step 8 by instruction and still opens the draft, so this exception does not reach it. That
  distinction has to be here rather than only in Step 9, because a short absolute in Constraints is
  the copy most likely to win a conflict, and a run that read only this bullet would refuse to open
  the PR the flag requires. Step 9 suspends four of `open-pr`'s gates by name, on every mode, and lets
  its `TODO(user):` lines ship. Every other question `open-pr` asks, including which template to use,
  still goes to the user.
- **`writing-work-docs` writes every human-facing artifact here.** Ticket description, validation
  comment, option (a) comment, commit messages, PR title and body. Its three refusals to publish are
  overridden for this whole run, and nothing else it says is. **A flag run has two of those five**,
  the commit messages and the PR title and body, and it routes both through that skill exactly as
  usual. The other three belong to writes the flag dropped.
- **One ticket per invocation.** A run that discovers the work spans two tickets stops and says
  so.
- **Never force-push and never rewrite pushed history.** Step 8 adds commits on top.
- **No agent branding in the ticket, the comment, or the PR body.** No generated-with footer, no
  `Co-Authored-By` there. Those are human-facing prose and the branding reads as noise in them.
  **Commits are the exception**: follow whatever trailer convention the repo already uses, which in this
  one means keeping the `Co-Authored-By` trailer. A commit trailer is metadata rather than prose, git
  has a standard field for it, and a skill that told a run to drop it would be instructing it to break
  the convention every other commit in the history follows.
