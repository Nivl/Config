---
name: open-ticket
description: Turns a requirement in prose into the Jira issues for it. Infers the project, the active sprint and the assignee from the user's own tickets, reads the repo so every issue names real files and real patterns, sweeps Jira and git history for work already filed or already shipped, sizes the work against the story-point guidelines, and builds the smallest tree that fits. Drafts every description locally and waits for one approval before creating anything. Use this skill when the user says "open a ticket", "file a ticket for this", "create a Jira issue", "make tickets for this feature", "break this into tickets", or hands over a requirement and expects issues rather than code. Requires the Atlassian MCP. Do not use it to edit an existing ticket (that is work-on) and do not use it to read or summarize one.
---

# Open Ticket

One requirement in prose, from "what is this really asking for" through to the Jira issues for it.

Jira creation has no rollback. You cannot un-notify an assignee, you cannot un-add an issue to a
sprint, and deleting an issue leaves a hole in the key sequence. That one fact decides the shape of
this skill. `work-on` writes the ticket first and verifies afterwards, because it saved the original
description and can put it back. Nothing here is recoverable, so the single human gate sits in front
of the write at Step 9 and every step before it is a read.

## Pipeline

| Step | What happens | Writes to Jira |
|---|---|---|
| 0 | Preflight. Jira tooling, cloudId, repo | no |
| 1 | Read the request. Ask only what investigation cannot settle | no |
| 2 | Infer project, sprint, board, assignee | no |
| 3 | Rough size, pick the exploration depth | no |
| 4 | Explore the codebase | no |
| 5 | Dedup sweep, Jira and git | no |
| 6 | Duplicate gate | no |
| 7 | Size properly, build the tree | no |
| 8 | Draft every description through `writing-work-docs` | no |
| 9 | The plan gate. One approval | no |
| 10 | Create, parents first | yes |
| 11 | Read back and verify | no |
| 12 | Final report | no |

**Steps 0 through 9 read Jira and write local files, and nothing else.** That is what lets the
dedup sweep and the exploration run as hard as they like. A step that filed a placeholder issue to
hang children off would put an unapproved key on a real board, and the key stays even after the
issue is deleted.

## Assume nothing

This is the rule the reading steps serve. A ticket written from the request alone describes the work
in the requester's words and names nothing an assignee can open, so the assignee redoes the search
this skill exists to do before they can start.

- Every leaf in the tree names at least one real path from Step 4, or the plan file says no path was
  found. A leaf with neither is a generic ticket, and a generic ticket comes straight back.
- "The code probably does X" is not a finding. A finding is a path, a symbol, a commit sha, a Jira
  key, or a query with its result.
- Absence of evidence gets written down as absence. When the sweep found nothing, the plan file says
  which queries ran and what each returned, because an empty result and a query nobody ran read
  identically once the run is over.

## Earn every question

Asking is the fallback and not the first move. A question whose answer is sitting in the repo wastes
a round trip and tells the user you did not look, and once that happens they stop reading the
questions that do matter.

**Go find it first.**

| Question shape | Where the answer already is |
|---|---|
| Where does this live, does Y exist, is Z still called that | `rg`, `git --no-pager grep`, read the file |
| How is this area tested, what would a new test look like | The nearest test file, read in Step 4 |
| Has somebody filed this already | Step 5's Jira sweep |
| Has somebody already shipped it | Step 5's git half |
| Which project do my tickets live in | Step 2's majority tally |
| Which sprint is open right now | Step 2's `state == "active"` filter |
| What is my account id | `atlassianUserInfo` in Step 2 |

**Two things survive the doctrine.** The sprint and the assignee are facts about the user's intent,
and no amount of reading the repo settles either one. Step 2 reduces both to a confirmation with its
tally attached, so the user corrects a proposed answer. An open prompt here spends a round trip on a
question Step 2 already has an answer for.

**These are legitimately for the user**, because nothing in the repo answers them.

- A product or scope decision. "Should this cover the admin path too, or is that a second ticket?"
- Which board, when Step 2 finds two active ones.
- Whether a credible duplicate from Step 6 really covers this work.
- The tree itself, at Step 9.

## Step 0: Preflight

**1. Atlassian MCP, required.** Search for it with `ToolSearch "atlassian jira"`. If the only tool
exposed is an `authenticate` tool, the server is connected and not authenticated, which counts here
as absent. Abort with `JIRA_UNAVAILABLE_NO_TOOLING` and stop. Do not offer to proceed.

acli cannot substitute for it. There are two reasons and the second is the one that matters. It is
unauthenticated in this session and it reads its credentials from under `~/.config`, which the
sandbox blocks, so it fails even when installed and authenticated. Past that, its request models cap
the writable field set. It cannot set sprint, cannot set story points, cannot set any custom field,
and cannot re-parent an existing item, so a ticket tree carrying points and a sprint is not
expressible through it at all. Falling back to it would file a tree with no points, no sprint and no
parent links, and nobody reading the result would know what had been dropped.

Get `cloudId` once from `getAccessibleAtlassianResources` and reuse it on every later call. The
value cannot change during a run, so fetching it per call spends a round trip each time for nothing.

**2. The human who approves Step 9.** Check that this run can put the Step 9 plan gate in front of a
person and read their answer back. If it cannot, abort with `GATE_UNREACHABLE_NO_HUMAN`.

**The primary control is the hook, not this check.** `open-ticket` sits in `DENIED_SKILLS` in
`shared_config/.claude/hooks/deny-review-in-workflow.py`, so a `Workflow` whose text names this skill
is denied before it starts.

**This abort is the backstop for what the hook cannot see.** The hook scans a workflow's code and
never its data. So it catches a script that asks for this skill by name, and it misses a workflow
agent that runs the skill without ever naming it. A run reached that way arrives with no denial
behind it and nothing else in front of Step 10.

**There is no probe for this, so the check is weaker than it looks.** `in-depth-review` keys on an
absent `Agent` tool, which is a signal it can test for. Nothing reports whether a human is reachable,
so this one is a judgement the run makes about its own context. If the run cannot tell, it stops.
Guessing wrong creates Jira issues with nobody having approved them, and a creation has no rollback.

**3. Repo, optional and reported either way.** `git rev-parse --is-inside-work-tree`. This skill
files tickets and writes no code, so it does not need a feature branch and does not care whether the
tree is dirty. Outside a repo, Step 4 has nothing to read and the git half of Step 5 cannot run. Say
that plainly in the plan file. A sweep missing its git half is a thinner check than a complete one,
and a reader who is not told treats the two verdicts as the same.

No `git fetch` and no branch check. Neither has a consumer in this pipeline, so both would cost a
call that nothing reads.

## Step 1: Read the request

Take the requirement as prose. Accept file paths, ticket keys and URLs as supporting material.

Read "Assume nothing" and "Earn every question" above before asking anything.

Derive the run's slug here. Step 5's git output files and Step 9's plan file are all keyed by it, so
a slug derived later would leave the earlier files sharing a name with every other run. Step 9
carries the rule and the pattern the slug has to match.

## Step 2: Infer project, sprint, board, assignee

Three calls. The third does most of the work.

```
atlassianUserInfo()                       -> account_id
getAccessibleAtlassianResources()         -> cloudId
searchJiraIssuesUsingJql(
  cloudId: <from above>,
  jql: "assignee = currentUser() AND sprint in openSprints() ORDER BY updated DESC",
  fields: ["customfield_10021"])
```

For each returned issue, filter `fields.customfield_10021` down to the entry whose
`state == "active"`. That entry carries the sprint `{id, name, startDate, endDate}` and the
`boardId`.

**The array is a sprint history and it is in no meaningful order.** Ten distinct sprints appeared
across 69 issues, and one issue carried nine entries with eight of them closed. The active entry was
last on 65 of 69 issues and first on 40 of 69, so `[0]` and `[-1]` are both wrong and each is wrong
on a different issue. Filter on the state. All 69 issues had exactly one active entry, none had two
and none had zero. Take a position in the array and the tree lands in a closed sprint, where nobody
working the board sees it.

**A field named `"sprint"` in the `fields` array is silently dropped.** No error, just an absent
field. Only `customfield_10021` returns the data. Ask for the name and the run reads an empty sprint
field, concludes the user has no active sprint, and asks a question it already had the answer to.

**Project is the majority `project.key` over that same result set.** It came back 100% GRO on the
tested account. Report the runner-up project in the plan file. Presenting one project as the only
possibility hides a real tail (PSS 24, CORE 8, FIRE 2, SEC 1, COMPLY 1), and a tree filed in the
wrong project has to be moved by hand, one issue at a time.

**No recency weighting.** All-time, 90 days and 30 days all pick GRO, at 93.7%, 94.3% and 96.1%. A
narrower window buys fewer projects to disambiguate and not a different answer, so weighting would
add a query and no accuracy.

**Assignee defaults to the current user's `account_id`.** It goes in the plan file, so Step 9 covers
it.

Two branches that ask.

- **No active sprint.** The user is between sprints, or on a Kanban board. Fall back to
  `assignee = currentUser() AND updated > -90d` for the project alone, report sprint and board as
  unknown, and ask. This branch was never exercised against live data, so say so in the plan file
  and treat whatever it produces as unproven.
- **Two or more distinct active `boardId`s.** Ask which board. The tested account sits on two boards
  historically, 62 and 176, and only the fact that none of board 176's sprints is currently active
  makes today's query clean. The field is an array and the JQL is a set-membership test, so handle
  two or more. Pick one silently and half the tree lands on a board its owner does not watch.

**Payload discipline, not optional.** `searchJiraIssuesUsingJql` always returns a mandatory field
floor of `assignee, description, issuetype, project, status, summary` plus whatever is requested, and
`description` cannot be excluded. Three unguarded assigned-issue queries produced responses of
1,051,178, 760,498 and 925,249 characters. Use `searchResultMode: "count"` for every tally and fetch
nodes only for the handful of candidates that survive. A payload that size exhausts the context
before the Step 5 sweep finishes, and a sweep that stops early reports a clean verdict it never
earned.

## Step 3: Rough size, pick the exploration depth

Sketch a rough total from the request. Under 5 points takes one focused inline pass. Five or more
fans out in Step 4.

The same number drives Step 7, so one estimate serves both the reading and the tree. Estimate twice
and the two numbers disagree, so the exploration depth and the tree shape end up sized for different
work.

## Step 4: Explore the codebase

On the fan-out path, launch these concurrently in one message. Each one is a leaf reader that spawns
nothing.

| Reader | Answers |
|---|---|
| similar-features | What already does something close to this, and where |
| files-per-leaf | Which files each candidate leaf would touch |
| test-patterns | How this area is tested, and what a new test would look like |
| conventions | Naming, structure and error handling in the target directory |
| already-done | Whether git history says some of this shipped |

On the inline path, read the files involved, find the nearest existing pattern, note the test
convention, and move on.

**Take the inline path when the `Agent` tool is absent, and say so in the plan file.** Tool
availability is not symmetric across an agent boundary and the loss is never announced. A fan-out
that quietly collapses to one reader produces a plan file in the same shape as five readers would,
so the coverage claim reads as complete when one pass is all that happened.

Generic issues are the failure this whole step exists to prevent. Every leaf in the tree carries at
least one real path from here, or the plan file says it could not find one. A ticket that names no
file hands its assignee this search to do again from scratch.

## Step 5: Dedup sweep

[DEDUP-QUERIES.md](DEDUP-QUERIES.md) carries the six literal queries, the quoting recipe, the
payload limits and the three git commands. Read it before running the sweep, because a query written
from memory is the one that comes back with a silent zero. None of it is restated here.

The logic that stays in this file.

**Q0 is a positive control and it runs first.** An unknown JQL field name returns `totalCount: 0`
with no error, while a syntax error is loud. One mistyped field name therefore makes the whole sweep
report no duplicates, and the skill then files the duplicate it exists to prevent. If Q0 returns
zero for a noun the work is definitely about, the sweep is broken and not clean. Stop and fix it
before reading any other result.

**Six queries counting the control, and each of the five finds something the others structurally
cannot.** Q1 finds the ticket that names the same code. Q2 finds the one that describes the same
work in prose without naming a file. Q3 un-buries the closed one. Q4 finds the comment where
somebody already wrote "superseded by" or "duplicate of". Q5 catches the one sharing neither code nor
words. Drop one and the sweep loses that entire class of duplicate, which is a different loss from
returning fewer hits.

**Every `git` command runs alone.** No pipe and no chain, redirected to a file under `/tmp/claude/`,
and read back in a separate call. `git` runs outside the sandbox, so a hook denies a piped or
chained git command and the git half of the sweep comes back empty while looking like it ran.

## Step 6: Duplicate gate

A credible match stops the run. Credible needs a definition. Left undefined the gate either blocks
on every loose Q5 hit or blocks on nothing, and which one it does on a given day is not predictable.

A match is credible when either of these holds.

- Its summary describes the same change, whatever words it uses.
- It names a file that a leaf of the planned tree would touch, and its description or its comments
  discuss what that file does. A passing mention of the path does not count.

A match is not credible on any one of these alone.

- A Q5 stemmed-concept hit. Q5 is deliberately unquoted and it is the query that produced a false
  positive for a path that does not exist.
- A shared component, or a shared label.
- A file mention that turns out to be a different concern in the same file.

Q5 exists to widen recall, so its hits get read and not trusted.

Borderline goes to the user. A match reported and dismissed costs one question. A duplicate filed
costs somebody a week of finding out.

Abort with the key on the same line.

```
DUPLICATE_FOUND: GRO-17812 (status Done)
```

Report every credible match with its status, its close date, its PR if it has one, and what it
overlaps. Then offer the options and let the user pick.

- (a) proceed anyway, keeping the tree as planned
- (b) narrow the scope to what the match does not cover, then re-run the sweep
- (c) stop

**Write nothing on your own initiative here.** Whether two pieces of work are the same is exactly
the call a human should make, and at this point in the run nothing has previewed a creation.

## Step 7: Size properly, build the tree

The points rule, applied mechanically so the plan file carries a number the user can argue with.

| Total | Shape |
|---|---|
| under 5 | one issue, no epic, no subtasks |
| 5 to 13 | flat stories and tasks, no epic |
| over 13 | an epic at the root |

A total under 5 points gets one issue, and a total over 13 points gets an epic at the root. Both
boundaries are numbers so the shape does not move with the run's mood. Size a tree by feel and the
same amount of work becomes an epic over three stories one day and six flat tasks the next.

**Any leaf estimated over 5 splits until every leaf is 5 or under.** A leaf nobody can finish inside
a sprint sits half done at the end of it, and the board then shows one stalled item where three real
pieces of work are hiding.

**A leaf that cannot be split below 8 without producing halves that neither ships alone is a signal
the request is not understood yet.** Say so in the plan file. A bad split files two tickets that have
to be worked together, which is worse than one honest large ticket, because two people can now pick
up half of it each.

**Issue type by intent.** `Story` for user-facing work. `Task` for technical work with no direct user
impact. `Bug` for a defect. `Epic` for the root once the total clears 13. `Bug Subtask` for a
breakdown inside a leaf. A wrong type puts the work outside the board filters and the reports written
for that type, so it goes missing from the views the team actually reads.

**Epic carries no story points.** `customfield_10028` is not on the Epic create screen. Epic has
`customfield_10503`, Eng Weeks, in its place. Derive Eng Weeks from the total points and label it as
derived in the plan file. An unlabeled derived number reads as an estimate somebody stands behind,
and then a roadmap rests on arithmetic nobody checked.

**The subtask type is `Bug Subtask`, id 10032.** `Subtask`, id 10014, is the canonical type and it
has one issue in all of GRO against 2334 for `Bug Subtask`. Filing `Subtask` would run against the
grain of the project and would likely fall outside board filters written for `Bug Subtask`, so the
subtask would exist and nobody would see it on a board. The cost is a thinner create screen, and
Step 10 handles that.

## Step 8: Draft every description

`writing-work-docs` authors every description, every summary and every comment this run produces. Do
not restate its rules here. Two copies of a voice rule drift apart, and the run then follows
whichever copy it happened to read last.

[TEMPLATES.md](TEMPLATES.md) carries the five templates, the shared rules and the rendered-length
cap. Read it before drafting, since picking the template from the issue type and the intent is the
first decision in this step. Hand the template to `writing-work-docs`. Let it invent a shape and the
tree arrives with five different section orders in it, so no reader can skim two tickets the same
way.

[JIRA-FORMAT.md](../work-on/JIRA-FORMAT.md) holds the two format rules that bind while the text is
being written, the no-hard-wrapping rule and the rule that headings start at `##`. Read it here and
not at Step 11. Both rules govern how the text is composed, so the Step 9 gate locks in whichever
way they were followed, Step 10 sends exactly that, and Step 11 then finds a mangled description it
is forbidden to restore. There is no saved original to put back on this path, because the issue is
new.

Three of the five templates came from the request. Those are the feature story, the bug and security
shape with its Stats block and its 1-to-5 likelihood rating, and the technical debt shape. Two more
exist because the request did not specify them, the Epic shape and the Bug Subtask shape.

**A number nobody ran is a `TODO(user):` line carrying the query.** Record every such line as it is
drafted, with the issue it will land in and what would resolve it. Step 12 reads that list out.
Written as prose the same number reads as a measured claim, and the next reader treats it as
evidence.

### The publish override, declared once for the whole run

`writing-work-docs` refuses to publish in three separate places, and a run that overrides one and
not the others gets blocked by whichever copy survived. This publish override covers all three, for
every step of this skill:

- its **Publishing** section, which says never publish anything,
- its **Hard rules** bullet `**Draft only.** Produce paste-ready text and stop.`,
- the **enumerated command list** under Publishing, which names `acli jira workitem create` as a
  distinct entry from `edit`. `open-ticket` performs the create.

Take everything else it says verbatim. This override is about who presses the button and not about
its prose rules, and its `TODO(user):` behavior in particular stays exactly as written. Read it
wider than those three and the run loses the voice rules that make a ticket readable, which is the
reason `writing-work-docs` is in the pipeline at all.

## Step 9: The plan gate

Write `/tmp/claude/open-ticket-<slug>-plan.md`.

The slug is two or three lowercase words from the requirement, joined by hyphens, matching
`^[a-z0-9][a-z0-9-]{0,48}$`. Derive it once in Step 1 and reuse it for every file the run writes.
Validate it against that pattern before it reaches a path, because it becomes part of one and the
requirement is text somebody else wrote.

Keying the file by slug is what keeps two runs apart. Share a filename and the second run silently
reads the first run's plan, then files somebody else's tree under the current user's name.

The file carries everything.

- the tree, with type, summary, points and parent for every node
- the resolved project, sprint, board and assignee, each labeled with how it was inferred
- the runner-up project from Step 2
- every dedup query that ran and what it returned, including Q0's control hit
- the full drafted description for every node
- every `TODO(user):` line
- anything Step 4 could not find a real path for

Show it and wait. **One yes covers the whole run.** Nothing is created before it, and after it
creation proceeds with no further prompts. A second gate partway through creation would ask a human
to approve a tree that half exists, and the half already filed cannot be taken back, so the approval
would be about nothing.

**Offer edit as a response, not only yes or no.** A wrong split or a wrong assignee is cheap to fix
here and expensive afterwards, because a filed issue keeps its key, its notification and its sprint
placement whatever happens to it next.

## Step 10: Create

[CREATE-FIELDS.md](CREATE-FIELDS.md) carries the complete parameter list, the create-screen table,
the per-type recipes, the exact field ids and the two never-write ids. Read it before the first
create, not after one fails. None of the field shapes is restated here.

The logic that stays in this file.

**Parents first, top down.** Epic, then stories and tasks, then subtasks. A child needs its parent's
key, so a bottom-up order reaches the subtask with no parent to point at and files it unparented,
where the epic's board never counts it.

**Partial failure stops the run.** Record every key created so far, report them, and do not retry
blind. A half-built tree that a retry duplicates is worse than a half-built tree that is reported,
because the duplicates carry real keys and somebody has to work out by hand which half is which.

**A refused write aborts with `JIRA_WRITE_DENIED`**, never with `JIRA_UNAVAILABLE_NO_TOOLING`. The
tooling is present here and the write was refused, so the remedy is a permission or a project
setting. The two codes are kept apart because reporting the wrong one sends the user to
re-authenticate a connection that was working.

## Step 11: Read back and verify

Read every created issue back. Confirm the description rendered, and confirm every field that type
actually carries landed. A conversion that half worked leaves literal `##` and `**` sitting on a
ticket where everyone can see them.

[JIRA-FORMAT.md](../work-on/JIRA-FORMAT.md) carries what survives the markdown-to-ADF conversion and
the read-back check itself. It is the single copy of the conversion table and of the read-back
procedure, and neither is restated here.

**Scope the check to the issue type.** An `Epic` has no points and no sprint to check, so reporting
them as missing would send the user hunting a field that never existed on that screen. A
`Bug Subtask` has neither a sprint nor a create-screen assignee. Read the set for each issue off that
type's column in the create-screen table in [CREATE-FIELDS.md](CREATE-FIELDS.md). The two-call table
in the same file has rows for `Bug`, `Epic` and `Bug Subtask` alone, so a `Story` or a `Task` has no
row there, and looking for one sends the check back to one fixed list for every type, which is the
thing this scoping exists to prevent.

**Three outcomes, never two.** Verified, could not read it back to verify, or failed.
Unverified is its own outcome and it never folds into either of the other two. Carry whichever one
it was into Step 12 in those words. An unverified write is the thing the user most needs told,
because nothing previewed the rendered result and a half-converted description looks exactly like a
verified one from here.

**Do not restore or delete anything on a failed check.** Report it and stop. `JIRA-FORMAT.md` bans
restoring on a maybe, and the reason is stronger on this path, because a false-positive read-back
would destroy work no human has ever seen.

## Step 12: Final report

**The run ends with this report and it is its own message.** Folded into a next-steps offer it gets
skimmed past, and it is the only place a run with one gate accounts for everything it wrote after
that gate.

- every key created, with its type, its summary, its points and its parent
- the browser link for each
- the resolved project, sprint, board and assignee, and how each was inferred
- the verification outcome per issue, in the three words from Step 11
- every `TODO(user):` line that shipped, with the issue it landed in
- the two knowingly unproven writes, and whether each held this run. They are the sprint write shape
  and the `Bug Subtask` assignee. [CREATE-FIELDS.md](CREATE-FIELDS.md) is the single copy of why each
  one is unproven and of what to do about it.
- the plan file path, since it stays on disk

**Zero shipped `TODO(user):` lines is stated and never omitted.** Say there were none. A missing
section reads identically to a forgotten one, and the reader cannot tell which one they have.

## Abort codes

The convention is `<SUBJECT>_<CONDITION>_<CAUSE>`, upper snake, with a prose reason after the colon
on the same line. The literal token is the wire contract. A paraphrase of the reason with no token in
it is not an abort a caller can recognize, so a caller keying on the token reads the run as having
finished normally.

| Code | When |
|---|---|
| `JIRA_UNAVAILABLE_NO_TOOLING` | Step 0 found no Atlassian MCP. acli cannot substitute. |
| `JIRA_WRITE_DENIED` | Tooling is present and the write was refused. Kept separate because the remedy differs. |
| `DUPLICATE_FOUND: <KEY> (status <S>)` | Step 6 found a credible match. |
| `GATE_UNREACHABLE_NO_HUMAN` | Step 0 could not reach a human to answer the Step 9 gate. |

## Constraints

- **Nothing is created before Step 9's one yes.** Every earlier step reads Jira and writes local
  files. A creation cannot be undone, so the gate belongs in front of it and there is nowhere else
  it could go.
- **This skill creates and reads. It never transitions, closes or deletes an issue.** A new GRO
  issue lands in To Do on its own, so a transition has nothing to fix. Deleting leaves a hole in the
  key sequence and throws away the only record of what got filed.
- **Do not use this skill to edit an existing ticket, since that is work-on's job.** A run that
  edits one ticket and creates another carries two different rollback stories, and the gate here
  covers only the creations.
- **This skill files the tree and stops. It does not write the code for what it just filed.** The
  handoff is the keys, and what happens next is the user's call. Step 9's one yes covered a tree of
  tickets, so code written off the back of it is work nobody approved, on a branch nobody asked for.
- **No Confluence.** No step in this pipeline reads or writes a page, so a page written here would
  be an artifact with no step that owns it and no reader expecting it.
- **No issue links beyond `parent`.** `createIssueLink` exists and no decision in this skill needs
  it. A link filed on a guess asserts a relationship between two tickets that no person judged.
- **The skill emits `Epic`, `Story`, `Task`, `Bug` and `Bug Subtask`, and nothing else.** `Spike`
  (10094), `Documentation` (10127), `QA` (10149) and `Design` (10551) all exist in GRO and their
  create screens were never read. Filing one would guess at its required fields, and a guessed field
  shape produces either a failed create or a ticket carrying wrong data.
- **Every `git` command runs alone.** No pipe, no chain, no `&&`. Redirect to a file under
  `/tmp/claude/` and read it back in a separate call. `git` runs outside the sandbox, so a hook
  denies a piped or chained git command and the step that needed it comes back empty.
- **Write the literal `/tmp/claude/` path and never `$TMPDIR`.** That variable resolves to a
  directory the sandbox write-denies, so the command dies at the kernel with an error that does not
  name the cause.
- **Never invent** a Jira key, a sprint id, a board id, a field id, a file path or a point estimate.
  An invented value here becomes a real issue that somebody else has to unpick.
- **One requirement per invocation.** A run that finds the request is really two unrelated pieces of
  work stops and says so. One gate cannot cover two trees the user has not seen separated.
- **`writing-work-docs` writes every human-facing artifact here.** Every summary, every description,
  every comment. Its three refusals to publish are overridden for this run and nothing else it says
  is.
- **No agent branding in a summary, a description or a comment.** Those are human-facing prose and
  the branding reads as noise to whoever picks the ticket up.
