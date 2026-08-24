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
---

# Work On

One Jira ticket, from "is this still true?" through to a draft PR.

The ticket is the least trustworthy input in the whole run. It was written at some past moment,
against a tree that has since moved, by someone who may have been guessing. Everything before
Step 5 exists to find out what is still true. Everything after Step 5 is ordinary development on
a premise that has been checked.

## Pipeline

| Step | What happens | Writes anything? |
|---|---|---|
| 0 | Preflight. Ticket key, branch, tooling. | no |
| 1 | Gather ground truth once, in the main thread. | no |
| 2 | Validate. Inline when the surface is small, seven parallel lenses plus the telemetry probes and the triage agents when it is not. | no |
| 3 | Ask only what investigation could not settle. | no |
| 4 | Verdict gate. Valid, invalid, superseded, or partial. | only if the user picks option (a) |
| 5 | Rewrite the ticket. Description in place, plus a validation comment. | **Jira** |
| 6 | Do the work via brainstorming or systematic-debugging. | local files |
| 7 | Commit everything, push. No PR yet. | **remote** |
| 8 | `review-and-fix`, every iteration, no early stop. | local commits |
| 9 | Push, then `open-pr --draft`. Always a draft, never a question. No PR opens at all when Step 8 could not run. | **remote** |
| end | "Final report". PR URL, write verification, one line per shipped `TODO(user):`. | no |

**Nothing writes anything a human reads before Step 5.** No Jira edit, no commit, no push, no PR. That is
what lets the validation phase be paranoid, and Step 4's option (a) is the single exception, a comment the
user explicitly asks for after seeing the evidence.

**From Step 5 on, writes do not stop to ask.** The ticket, the commits, and the PR all go out without a
preview, because the validation phase in front of them is what earns that. The rule is that this skill
**asks about decisions and never about wording**. Step 3's questions and Step 4's a/b/c pick are
decisions and both stay. A rendered description, a comment, a commit message, and a PR body are
wording, and each is written, verified after the fact, and accounted for in "Final report".

The "no" column above is about those artifacts, not about every remote call. Step 2 reads Datadog and
Amplitude when the ticket claims something about production, which is a read against live telemetry.
The warehouse depends on what is installed. This skill has no connection of its own, so it looks for a
skill that runs SQL and asks the user when there is none. See [DB-QUERIES.md](DB-QUERIES.md). A query
that does run is a read, and it is bounded and `SELECT`-only either way.

## Assume nothing

This is the rule the rest of the skill serves. A validation phase that accepts the ticket's
framing has done nothing, because the framing is the thing most likely to be stale.

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

**1. Resolve the ticket key.** Accept a bare key (`WMP-837`), a Jira URL
(`.../browse/WMP-837`), or the key embedded in the current branch name (`wmp-837-invoice-tax`).
Uppercase it. If the invocation carries no key and the branch name has none, ask for it and
stop until you have it. Never invent a key and never guess the project prefix.

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

**The ticket.** Read it twice, in two formats, for two different reasons.

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

## Step 2: Validate

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

Two writes. The description gets replaced in place. A comment gets appended.

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

Invoke `review-and-fix`. Let it run every iteration it wants.

That skill stops on its own terms, and it has no iteration cap by design. Its loop continues
only while each pass commits at least one real fix, so a long run means it is still finding
things. Do not interrupt it, do not summarize partway and call it finished, and do not pass any
flag that shortens it.

Three things to get right when invoking it:

- **Do not pass `--skip-ticket`.** Step 0 confirmed Jira access, so the ticket-compliance
  reviewer can run. Checking the diff against the ticket is worth more here than anywhere else,
  because Step 5 just rewrote that ticket and this is what catches a rewrite that drifted from
  what got built. There is no flag for passing the key. That reviewer reads it out of the commit
  messages, which is why Step 7 puts it there. Confirm it did before trusting a clean result.
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

Put an item you cannot confidently place into "Unfinished here." The cost of leaving real
out-of-scope work unfiled is that somebody finds it later. The cost of filing unfinished work as a
ticket is that the branch ships with a known gap wearing a ticket number.

**The batch approval.** Propose the second bucket as one batch with one yes, at the end of Step 8,
once the loop has stopped. The Step 4 cut needs no separate approval, because the user's agreement
to the reduced scope there already covers it. This bucket needs its own, because these findings
did not exist when that agreement was made, so it cannot cover them. Present each candidate in the
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

## Step 9: Push and open a draft PR

Push the review-and-fix commits. Then invoke `open-pr` with `--draft`.

**The PR is not a question.** It always opens, and it always opens as a draft. Do not ask whether to
open it, do not ask whether draft is right, and do not offer ready-for-review as an alternative. A
draft is the correct end state for this pipeline: the work is reviewed by Step 8 and unreviewed by a
human, which is exactly what draft means.

**The one exception is a Step 8 that could not run at all.** That justification rests on Step 8
having reviewed the work, so a `REVIEW_UNAVAILABLE_NO_FANOUT` abort that survives its one
re-invocation leaves nothing for a draft to mean. The run stops in Step 8 on that path. Its abort
section has the detail.

`open-pr` carries the title rules, the template detection, and the `writing-work-docs` routing.
It owns all of that. What it does not get to own here is its own gates.

### Overriding `open-pr`, by name

`open-pr` was written to be invoked by a human, so it asks a human several things. Four of those are
suspended for this run. Name all four, because it states each rule in more than one place and a run
that overrides one copy obeys the copy still standing:

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
  describe what got built, and the PR should describe what got built.

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

```
PR            <url>  (draft)
Ticket        <url>  (description: rewritten | write failed; comment: posted | not posted)
Follow-ups (<n>)
  - <key>  filed from the Step 4 scope cut  <one-line summary>
  - <key>  filed from a Step 8 leftover  <one-line summary>
  - could not file  <Step 4 scope cut | Step 8 leftover>  <why>
Branch        <name>
Jira write    verified | could not read it back to verify | failed
TODOs (<n>)
  - <artifact> <locator>  <the line's text>  -> <what would resolve it>
Validation    <one line on what it changed about the ticket>
Iterations    <n> review-and-fix passes
Stash         restored | still stashed at <sha> | none
```

**Every `TODO(user):` line that shipped gets its own line, wherever it landed.** The Jira
description, the validation comment, the option (a) comment, the PR body. Read them off the running
list Step 5 has been keeping. Never summarize them as a count, never fold two into one line, and
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

## Constraints

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
  one thing the wording rule must never be read to allow.
- **Every write is verified after the fact, since none is approved before it.** Read the Jira ticket
  back, fetch the PR body back. Report the Jira read-back's outcome in "Final report" using its three
  exact words. Verification replaces the preview, so skipping it leaves a write with no control on it
  at all.
- **Nothing that shipped unverified stays unmentioned.** Every `TODO(user):` line that reached Jira or
  a PR body gets its own line in "Final report", on every ending the run can have. These artifacts
  outlive the conversation and the user did not read them before they went out, so the report is their
  only view of what went out.
- **An invalid or superseded ticket stops the run.** Step 4 writes nothing on its own initiative. It
  never closes the ticket, and it does not comment or start a branch until the user picks one of
  Step 4's three options. Executing option (a) afterwards is the user's decision being carried out,
  not this rule being broken.
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
  Step 8's reviewers.
- **Never invent** a ticket key, a PR number, a dashboard link, a date, a metric, or a test
  result. Unknown becomes a `TODO(user):` line, and every one of those reaches "Final report".
- **A triage recommendation never gets published.** The facts go into the Jira comment with their
  locators. The recommendation stays in the conversation at Step 4. Step 5 explains why.
- **`worth_fixing: 'no'` requires evidence that reaches.** A query with a result, a flag state at a
  `file:line`, or a precondition chain read out of the code. An unmeasured impact is `unclear`, never
  `no`. "No telemetry covers this path" and "nobody is affected" are opposite conclusions and only one
  of them stops work.
- **Verify the Jira formatting by reading it back.** Do not trust that the markdown converted.
- **Execution mode is not a question.** When `writing-plans` offers its Execution Handoff choice,
  take Subagent-Driven (`superpowers:subagent-driven-development`) and say so in one line. Every other
  question that skill asks still goes to the user. Honor an explicit request for inline execution.
- **The PR is not a question either.** It always opens, always as a draft, unless Step 8 could not
  run at all. Step 9 suspends four of `open-pr`'s gates by name and lets its `TODO(user):` lines
  ship. Every other question `open-pr` asks, including which template to use, still goes to the
  user.
- **`writing-work-docs` writes every human-facing artifact here.** Ticket description, validation
  comment, option (a) comment, commit messages, PR title and body. Its three refusals to publish are
  overridden for this whole run, and nothing else it says is.
- **One ticket per invocation.** A run that discovers the work spans two tickets stops and says
  so.
- **Never force-push and never rewrite pushed history.** Step 8 adds commits on top.
- **No agent branding in the ticket, the comment, or the PR body.** No generated-with footer, no
  `Co-Authored-By` there. Those are human-facing prose and the branding reads as noise in them.
  **Commits are the exception**: follow whatever trailer convention the repo already uses, which in this
  one means keeping the `Co-Authored-By` trailer. A commit trailer is metadata rather than prose, git
  has a standard field for it, and a skill that told a run to drop it would be instructing it to break
  the convention every other commit in the history follows.
