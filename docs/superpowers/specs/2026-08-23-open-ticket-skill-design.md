# open-ticket, design

Date: 2026-08-23
Status: design approved, ready for an implementation plan
Path note: `docs/superpowers/` is gitignored in this repo, so this file is local scratch. There is nothing to commit here.

## What this is

A new skill at `shared_config/.claude/skills/open-ticket/`. It takes a requirement in prose and files the Jira issues for it, after reading the repo and after checking that nobody has already filed or already shipped the same work.

The skill it most resembles is `work-on`, which drives an existing ticket. This one creates. That difference decides most of the design, because Jira creation has no rollback. You cannot un-notify an assignee, you cannot un-add an issue to a sprint, and deleting an issue leaves a hole in the key sequence. `work-on` can write first and verify after, since the original description is recoverable. This skill cannot, so its one human gate sits before the write.

## Decisions taken during design

Each of these was a real fork. They are recorded so the implementation does not relitigate them.

| Decision | Choice |
|---|---|
| Where the human gate sits | One gate. Draft the whole tree including every description to a local plan file, show it, wait for one yes, then create everything with no further prompts. |
| Duplicate check strength | Multi-angle sweep across all statuses plus git history. A credible match stops the run. |
| Project and sprint discovery | Infer from the user's own assigned tickets. Majority project wins. Sprint comes from the same query. |
| Ambiguous inference | A clean majority proceeds silently. No majority, or no open sprint, and the skill asks with the tally as the options. |
| Tree shape | The points rule, mechanically. Under 5 total is one issue. Any leaf over 5 splits until every leaf is 5 or under. Over 13 total gets an epic. |
| Subtask type | `Bug Subtask`, two calls. It is what GRO actually uses, 2334 issues against 1. |
| Exploration depth | Scaled to the rough size. Small work gets one inline pass. Anything heading for an epic fans out. |
| acli as a fallback | Dropped. No Atlassian MCP means the run aborts. |

## The one place the original request cannot be built as written

The request said to treat Jira as the target when either the Atlassian MCP or `acli` is available. `acli` cannot do this job.

Two separate reasons, and the second is the one that matters.

`acli` is installed at version 1.3.22-stable and is not authenticated in this session. It reads credentials from under `~/.config`, which the sandbox blocks, so it fails even when installed and authenticated. Four sibling skills already carve this out.

Past that, **its request models cap the writable field set.** It cannot set sprint, cannot set story points, cannot set any custom field, and cannot re-parent an existing item. A tree with points and a sprint is not expressible through it. Its `--generate-json` skeleton and an undocumented `additionalAttributes` field on the backend create model were both examined and neither closes the gap.

So Step 0 requires the MCP and aborts without it. The frontmatter says so.

## Files

| File | Holds | Why it is separate |
|---|---|---|
| `SKILL.md` | Frontmatter, pipeline, doctrine, every step, constraints | The pipeline, the doctrine, the decision logic and the constraints stay in SKILL.md regardless of length. That is the house rule. |
| `CREATE-FIELDS.md` | Per-type create screens, exact field ids, which types need two calls, write shapes | Read on one path only. A literal artifact to match exactly. |
| `DEDUP-QUERIES.md` | Q0 through Q5, the quoting recipe, the git commands | Same. Read on one path, matched exactly. |
| `TEMPLATES.md` | The three description templates, plus the epic and subtask shapes the request did not specify | Same. Pasted, not paraphrased. |

Each reference file opens with a back-pointer naming the step that reads it, then the one thing not to get wrong. That follows `JIRA-FORMAT.md` and `VALIDATION.md`.

`JIRA-FORMAT.md` in `work-on/` already carries the markdown-to-ADF problem, the read-back check and the never-a-table rule. `open-ticket` links to it and does not restate it.

## Frontmatter

```yaml
---
name: open-ticket
description: Turns a requirement in prose into the Jira issues for it. Infers the project, the active sprint and the assignee from the user's own tickets, reads the repo so every issue names real files and real patterns, sweeps Jira and git history for work already filed or already shipped, sizes the work against the story-point guidelines, and builds the smallest tree that fits. Drafts every description locally and waits for one approval before creating anything. Use this skill when the user says "open a ticket", "file a ticket for this", "create a Jira issue", "make tickets for this feature", "break this into tickets", or hands over a requirement and expects issues rather than code. Requires the Atlassian MCP. Do not use it to edit an existing ticket (that is work-on) and do not use it to read or summarize one.
---
```

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

## Step 0: Preflight

**Atlassian MCP, required.** `ToolSearch "atlassian jira"`. If the only tool exposed is an `authenticate` tool, it is connected and not authed. Abort with `JIRA_UNAVAILABLE_NO_TOOLING` and say that `acli` cannot substitute and why. Do not offer to proceed.

Get `cloudId` once from `getAccessibleAtlassianResources` and reuse it for every later call.

**Repo, optional but reported.** `git rev-parse --is-inside-work-tree`. This skill files tickets and writes no code, so it does not require a feature branch and does not care whether the tree is dirty. Outside a repo, Step 4 exploration and the git half of Step 5 cannot run. Say that plainly, because it thins the duplicate check.

No `git fetch` and no branch check. Neither has a consumer here.

## Step 1: Read the request

Take the requirement as prose. Accept file paths, ticket keys and URLs as supporting material.

Follow `work-on`'s "Assume nothing" and "Earn every question" doctrine. A question the repo can answer is a question that does not get asked. Sprint and assignee survive that doctrine because they are facts about the user's intent that no amount of reading settles, and Step 2 reduces them to a confirmation rather than an open prompt.

## Step 2: Infer project, sprint, board, assignee

Four calls. The third does most of the work.

```
atlassianUserInfo()                       -> account_id
getAccessibleAtlassianResources()         -> cloudId
searchJiraIssuesUsingJql(
  cloudId: <from above>,
  jql: "assignee = currentUser() AND sprint in openSprints() ORDER BY updated DESC",
  fields: ["customfield_10021"])
```

For each returned issue, filter `fields.customfield_10021` to `state == "active"`. That entry carries the sprint `{id, name, startDate, endDate}` and the `boardId`.

**The array is a sprint history and it is in no meaningful order.** Ten distinct sprints appeared across 69 issues, and one issue carried nine entries with eight of them closed. The active entry was last on 65 of 69 issues and first on 40 of 69, so `[0]` and `[-1]` are both wrong. Filtering on `state == "active"` was unambiguous. All 69 issues had exactly one active entry, none had two, none had zero.

Project is the majority `project.key` over that same result set. It came back 100% GRO. **Report the runner-up rather than assuming one project**, because the all-time tail is real (PSS 24, CORE 8, FIRE 2, SEC 1, COMPLY 1).

Assignee defaults to the current user's `account_id`. It appears in the plan file, so the gate covers it.

**No recency weighting.** All-time, 90 days and 30 days all pick GRO, at 93.7%, 94.3% and 96.1%. A narrower window buys fewer projects to disambiguate, not a different answer, so weighting adds a query and no accuracy.

Two branches that ask:

- **No active sprint.** Between sprints, or a Kanban board. Fall back to `assignee = currentUser() AND updated > -90d` for the project only, report sprint and board as unknown, and ask. This branch was never exercised against live data, so treat it as a real gap and say so in the report.
- **Two or more distinct active `boardId`s.** Ask which board. This user sits on two boards historically (62 and 176), and only the fact that none of board 176's sprints is currently active makes today's query clean. The field is an array and the JQL is a set-membership test, so handle two or more.

**Payload discipline, not optional.** `searchJiraIssuesUsingJql` always returns a mandatory field floor of `assignee, description, issuetype, project, status, summary` plus whatever is requested, and `description` cannot be excluded. Three unguarded assigned-issue queries produced responses of 1,051,178, 760,498 and 925,249 characters. Use `searchResultMode: "count"` for every tally and fetch nodes only for the handful of candidates that survive.

**`"sprint"` as a name in the `fields` array is silently dropped.** No error, just absent. Only `customfield_10021` works.

## Step 3: Rough size, pick the exploration depth

Sketch a rough total from the request. Under 5 points takes one focused inline pass. Five or more fans out.

The same number drives Step 7, so one estimate serves both the reading and the tree.

## Step 4: Explore the codebase

On the fan-out path, launch these concurrently in one message. Each is a leaf reader that spawns nothing.

| Reader | Answers |
|---|---|
| similar-features | What already does something close to this, and where |
| files-per-leaf | Which files each candidate leaf would touch |
| test-patterns | How this area is tested, and what a new test would look like |
| conventions | Naming, structure and error handling in the target directory |
| already-done | Whether git history says some of this shipped |

On the inline path, read the files involved, find the nearest existing pattern, note the test convention, and move on.

Generic issues are the failure this step exists to prevent. Every leaf in the tree carries at least one real path from this step, or the plan file says it could not find one.

## Step 5: Dedup sweep

`DEDUP-QUERIES.md` carries the literal queries. The logic that stays here:

**Q0 is a positive control and it runs first.** An unknown JQL field name returns `totalCount: 0` with no error, while a syntax error is loud. One mistyped field name makes the whole sweep report no duplicates, and the skill then files the duplicate it exists to prevent. If Q0 returns zero for a noun the work is definitely about, the sweep is broken and not clean. Stop and fix it.

Six queries. Q1 finds the ticket that names the same code. Q2 finds the one that describes the same work in prose without naming a file. Q3 un-buries the closed one. Q4 finds the comment where somebody already wrote "superseded by" or "duplicate of". Q5 catches the one sharing neither code nor words.

**No status filter and no date filter.** `statusCategory = Done` was 65% of matches for the tested term, and `created > -180d` cut 29 matches to 8, a 72% recall loss. Rank with `ORDER BY updated DESC` instead.

**Quoting is mandatory.** Every term goes inside escaped inner quotes, `text ~ "\"<TERM>\""`. Bare `~` is an order-independent AND over stemmed tokens, so a single term the ticket does not contain zeroes the query. The value is also a live Lucene query string, where uppercase `OR` and `AND` and a whitespace-preceded `-` change the meaning.

**A bare file path over-matches badly.** `description ~ "runwayer-plans.ts"` gave 31 hits against 1 for the quoted form, and one path-shaped search returned a false positive for a path that does not exist. Never append `:LINE`, because the colon splits and the line number becomes its own AND term.

**The git half uses `-G`, not `-S`.** On the same string `-S` found 4 commits and `-G` found 10, because `-S` only fires when the occurrence count changes. `--all` is not the default and has to be passed. `\s` under `-P` silently returns zero hits on this git build (2.50.1, Apple Git-155), so use POSIX classes.

Every `git` command runs alone with no pipe and no chain, redirected to a file under `/tmp/claude/`, and read back in a separate call. `git` runs outside the sandbox, so a pipe or a chain is denied.

## Step 6: Duplicate gate

A credible match stops the run. "Credible" needs a definition, because left undefined it becomes whatever the run feels like that day, and the gate then either blocks on every loose Q5 hit or blocks on nothing.

A match is credible when either of these holds:

- its summary describes the same change, whatever words it uses, or
- it names a file that a leaf of the planned tree would touch, and its description or its comments discuss that file's behavior rather than mentioning it in passing.

A match is not credible on a Q5 stemmed-concept hit alone, on a shared component or label alone, or on a file mention that turns out to be a different concern in the same file. Q5 exists to widen recall and its hits get read, not trusted.

Borderline goes to the user. A match reported and dismissed costs one question. A duplicate filed costs somebody a week of finding out.

Abort with the key on the same line.

```
DUPLICATE_FOUND: GRO-17812 (status Done)
```

Report every credible match with its status, its close date, its PR if there is one, and what it overlaps. Then offer the options and let the user pick. Follow `work-on` Step 4's shape.

- (a) proceed anyway, filing the ticket as drafted
- (b) narrow the scope to what the match does not cover, then re-run the sweep
- (c) stop

**Write nothing on your own initiative here.** A judgement about whether two pieces of work are the same is exactly the call a human should make, and nothing previews a creation.

## Step 7: Size properly, build the tree

The points rule, applied mechanically so the plan file carries a number to argue with.

| Total | Shape |
|---|---|
| under 5 | one issue, no epic, no subtasks |
| 5 to 13 | flat stories and tasks, no epic |
| over 13 | an epic at the root |

Any leaf estimated over 5 splits until every leaf is 5 or under. A leaf that cannot be split below 8 without producing halves that neither ships alone is a signal the request is not understood yet. Say so rather than splitting badly.

Issue type by intent. `Story` for user-facing work. `Task` for technical work with no direct user impact. `Bug` for a defect. `Epic` for the root when the total clears 13. `Bug Subtask` for a breakdown inside a leaf.

**Epic carries no story points.** `customfield_10028` is not on the Epic create screen. Epic has `customfield_10503`, "Eng Weeks", instead. Derive Eng Weeks from the total points and label it as derived in the plan file, so the number is never presented as though a person estimated it.

**The subtask type is `Bug Subtask`, id 10032.** `Subtask` (10014) is the canonical type and has one issue in all of GRO, against 2334 for `Bug Subtask`. Filing `Subtask` would run against the grain of the project and would likely fall outside board filters written for `Bug Subtask`. The cost is a thin create screen, handled in Step 10.

## Step 8: Draft every description

`writing-work-docs` authors every description, every summary and every comment. Hand it the template rather than letting it invent a shape. Do not restate its rules.

### The publish override, declared once for the whole run

`writing-work-docs` refuses to publish in three separate places, and a run that overrides one and not the others gets blocked by whichever copy survived. All three are overridden here, for every step of this skill:

- its **Publishing** section, which says never publish anything,
- its **Hard rules** bullet `**Draft only.** Produce paste-ready text and stop.`,
- the **enumerated command list** under Publishing, which names `acli jira workitem create` as a distinct entry from `edit`. `open-ticket` performs the create.

Take everything else it says verbatim. This override is about who presses the button, not about its prose rules, and its `TODO(user):` behavior in particular stays exactly as written.

Templates live in `TEMPLATES.md`. The three from the request are the feature story, the bug and security shape with its Stats block and its 1-to-5 likelihood rating, and the technical debt shape. Two more are needed because the request did not specify them.

- **Epic.** Goal, the stories beneath it, what is out of scope, how it is known to be done. No acceptance criteria, since its children carry those.
- **Bug Subtask.** One paragraph and a checklist. A subtask is hours to a day, so a full template on it is noise.

Every template keeps the `ELI5` section. It is the section a reader with no project context uses, and it is the first one a drafter drops.

Hard limits on the drafted text:

- `description` caps at 32000 characters on the string branch. Check the rendered length before the call.
- Never a table. A triage assessment is the natural table and a table is the most common way a ticket arrives mangled. One bullet per row.
- No hard wrapping. One line per paragraph and one line per list item, however long.
- Start headings at `##`. An `h1` renders enormous next to Jira's own page title.
- A number nobody ran is a `TODO(user):` line carrying the query, never prose. Record every such line as it is drafted, with the issue it lands in and what would resolve it. The final report reads that list out.

## Step 9: The plan gate

Write `/tmp/claude/open-ticket-<slug>-plan.md`.

The slug is two or three lowercase words from the requirement, joined by hyphens, matching `^[a-z0-9][a-z0-9-]{0,48}$`. Derive it once at Step 1 and reuse it for every file the run writes. Validate it before it reaches a path, because it becomes part of one.

Keying the file matters. Two runs otherwise share a filename, and the second silently reads the first run's plan and files somebody else's tree.

The file carries everything:

- the tree, with type, summary, points and parent for every node
- the resolved project, sprint, board and assignee, each labeled with how it was inferred
- the runner-up project from Step 2
- every dedup query that ran and what it returned, including Q0's control hit
- the full drafted description for every node
- every `TODO(user):` line
- anything Step 4 could not find a real path for

Show it and wait. **One yes covers the whole run.** Nothing is created before it. After it, creation proceeds with no further prompts.

Offer edit as a response, not just yes or no. A wrong split or a wrong assignee is cheap to fix here and expensive to fix after.

## Step 10: Create

`CREATE-FIELDS.md` carries the per-type recipes. The logic that stays here:

**Parents first, top down.** Epic, then stories and tasks, then subtasks. A child needs its parent's key.

**Markdown works on create.** `createJiraIssue` has `contentFormat`, enum `["markdown", "adf"]`, the same as `editJiraIssue`. No hand-built ADF is needed for `description`. Pass `contentFormat: "markdown"` explicitly on every call, because the shared enum text hedges about defaults and stating it costs nothing.

**Three types need two calls.**

| Type | Calls | Reason |
|---|---|---|
| `Story`, `Task`, `Subtask` | 1 | `description` and `customfield_10028` are on the create screen |
| `Bug`, `Epic`, `Bug Subtask` | 2 | Both are absent from the create screen |

On the two-call types, create with whatever that type's screen actually carries, then `editJiraIssue` with `contentFormat: "markdown"` for the description and the points. The three screens differ, so the first call is not the same shape for all three.

| Type | First call carries | Second call adds |
|---|---|---|
| `Bug` | summary, parent, assignee, labels, priority, sprint | description, points |
| `Epic` | summary, assignee, labels, priority | description, Eng Weeks |
| `Bug Subtask` | summary, parent, labels, priority | description, points, assignee |

`Bug Subtask` has no `assignee` and no `reporter` on its create screen at all, so assignee moves to the second call. Whether it takes there is one of the known unknowns.

`Epic` gets no sprint. `customfield_10021` is not on its create screen, and an epic spanning sprints does not belong to one. `Bug Subtask` gets no sprint either, and it inherits its parent's board placement in practice.

**`parent` for every level**, including Story to Epic and Bug to Epic. GRO is company-managed and still uses the unified `parent` field. **Never write `customfield_10014`** (Epic Link, mirrored from `parent`, on no create screen). **Never write `customfield_10016`** (Story point estimate, the team-managed twin, one non-empty value in all of GRO).

**Never send `components` or `fixVersions`.** Neither is on any GRO create screen and `component is not EMPTY` returns zero. The tool's own docstring advertises a components example, so the wrapper will forward it and Jira will reject it.

**Copy `issueTypeName` verbatim** from `getJiraProjectIssueTypesMetadata`. Case sensitivity is unknown and copying removes the question. The spelling is `Subtask`, one word.

**Field ids are instance-specific.** GRO's ids are the fast path. Re-check them against `getJiraIssueTypeMetaWithFields(requiredFieldsOnly: false)` at run time, and resolve fresh for any project that is not GRO. Do not reuse GRO's ids on another project.

**Partial failure stops the run.** Record every key created so far, report them, and do not retry blind. A half-built tree that a retry duplicates is worse than a half-built tree that is reported.

## Step 11: Read back and verify

Read every created issue back. Confirm the description rendered rather than landing as literal `##` and `**`, and confirm every field that type actually carries landed.

Check against the type, not against one fixed list. An `Epic` has no points and no sprint to check, so reporting them as missing would be a false alarm. A `Bug Subtask` has neither sprint nor a create-screen assignee. The check for each issue is the set from the Step 10 table for its type.

**Three outcomes, never two.** Verified, could not read it back to verify, or failed. Carry whichever one it was into the report in those words. An unverified write is the thing the user most needs told, because nothing previewed the rendered result and a half-converted description looks exactly like a verified one from here.

Do not restore or delete anything on a failed check. Report it and stop. `JIRA-FORMAT.md` bans restoring on a maybe, and the reason is stronger here, because a false-positive read-back would destroy work nobody has seen.

## Step 12: Final report

- every key created, with its type, its summary, its points and its parent
- the browser link for each
- the resolved project, sprint, board and assignee, and how each was inferred
- the verification outcome per issue, in the three words from Step 11
- every `TODO(user):` line that shipped, with the issue it landed in
- the two knowingly unproven writes, and whether they held this run
- the plan file path, since it stays on disk

## Abort codes

Convention is `<SUBJECT>_<CONDITION>_<CAUSE>`, upper snake, with a prose reason after `: ` on the same line.

| Code | When |
|---|---|
| `JIRA_UNAVAILABLE_NO_TOOLING` | No Atlassian MCP. `acli` cannot substitute. |
| `JIRA_WRITE_DENIED` | Tooling is present and the write was refused. Kept separate because the remedy differs. |
| `DUPLICATE_FOUND: <KEY> (status <S>)` | Step 6 found a credible match. |

The literal token is the wire contract. A paraphrase of the reason without the token is not an abort a caller can recognize.

## Known unknowns, each settled by one live write

These are carried as unknowns rather than guessed, because a guessed field shape causes a real failed create.

| Unknown | How the skill handles it |
|---|---|
| Sprint write shape, bare `9511` against `[9511]` | Confirm once on the run's first create, read back, then reuse the proven shape for the rest of the run. |
| Whether assignee can be set by a follow-up edit on `Bug Subtask`, which has no assignee field on its create screen | Attempt it in the edit. Report plainly when it does not stick. |
| Whether `createJiraIssue` carrying `description` hard-fails on Bug, Epic and Bug Subtask or silently drops it | Avoided. The two-call path never sends it. |
| Whether `contentFormat: "markdown"` applies to textarea custom fields such as `customfield_10377` Acceptance Criteria | Avoided. The skill writes acceptance criteria inside `description`, not into the custom field. |
| Whether the top-level `parent` parameter routes correctly for a Story under an Epic, given its docstring says "Parent for subtasks" | Verified on the first epic child of the run by reading the parent back. Reported if it did not take. |

## Out of scope

- Editing an existing ticket. That is `work-on`.
- Transitioning, closing or deleting any issue. The skill creates and reads, nothing else.
- Confluence.
- Writing code for the ticket it just filed. The handoff is the key, and the user decides what happens next.
- Issue links beyond `parent`. `createIssueLink` exists and no decision in this design needs it.
- The four unread issue types, `Spike` (10094), `Documentation` (10127), `QA` (10149) and `Design` (10551). Their create screens were never read, so the skill does not emit them.

## Deliverables beyond the skill itself

Step 4 fans out into subagents, so per `AGENTS.md`:

- add `open-ticket` to `DENIED_SKILLS` in `shared_config/.claude/hooks/deny-review-in-workflow.py`
- add a matching case to `tests/deny_review_in_workflow_test.sh`

The existing `project-management:create-issue` command in the Calm plugin covers the same ground in 52 generic lines with no dedup check, no sprint handling and no templates. `open-ticket` supersedes it. Nothing in this repo needs changing for that, and the overlap is worth noting for whoever wonders which to reach for.

## Appendix A: field facts, project GRO

Site constants. `cloudId` `36dbf4da-8d98-4bfd-a6fe-e80caa66a434` for `calmdotcom.atlassian.net`. Project GRO ("Grow"), numeric id 10057, company-managed, `style: classic`.

`WMP`, the project key used throughout `work-on`'s own examples, does not exist on this site. Do not copy it into this skill's examples.

### Issue types

| Exact `name` | id | subtask | hierarchy | GRO issues |
|---|---|---|---|---|
| `Epic` | 10000 | false | 1 | many |
| `Story` | 10004 | false | 0 | 6216 |
| `Task` | 10013 | false | 0 | 1181 |
| `Subtask` | 10014 | true | -1 | 1 |
| `Bug` | 10015 | false | 0 | 3424 |
| `Bug Subtask` | 10032 | true | -1 | 2334 |

### Create screen availability

| Field | id | Epic | Story | Task | Subtask | Bug | Bug Subtask |
|---|---|---|---|---|---|---|---|
| Description | `description` | no | yes | yes | yes | no | no |
| Story Points | `customfield_10028` | no | yes | yes | yes | no | no |
| Sprint | `customfield_10021` | no | yes | yes | yes | yes | no |
| Assignee | `assignee` | yes | yes | yes | yes | yes | no |
| Parent | `parent` | yes | yes | yes | required | yes | required |
| Labels | `labels` | yes | yes | yes | yes | yes | yes |
| Priority | `priority` | yes | yes | yes | yes | yes | yes |
| Components | `components` | no | no | no | no | no | no |
| Epic Link | `customfield_10014` | no | no | no | no | no | no |

### Custom field ids

| id | Name | Type |
|---|---|---|
| `customfield_10028` | Story Points | number, float |
| `customfield_10021` | Sprint | array of json, greenhopper |
| `customfield_10503` | Eng Weeks | float, Epic only |
| `customfield_10483` | Impact | single select, Epic only. XL 13707, L 13224, M 13223, S 13222, XS 13706 |
| `customfield_10481` | Focus Area | single select, Epic only |
| `customfield_10377` | Acceptance Criteria | textarea, not written by this skill |
| `customfield_10397` | Reproducibility | select, Bug Subtask only |

Nothing custom is required on create on any type. No required QA field, no required team field, no required release field. `reporter` has `hasDefaultValue: true` everywhere it appears, so it never needs passing.

### Required on create

| Type | Required |
|---|---|
| `Epic`, `Story`, `Task`, `Bug` | `issuetype`, `project`, `reporter`, `summary` |
| `Subtask` | the same, plus `parent` |
| `Bug Subtask` | `issuetype`, `project`, `summary`, `parent`. No `reporter` field at all. |

## Appendix B: the dedup query set

Placeholders. `<PROJECTS>` is a comma list. `<PATH_N>` is a repo-relative path or symbol with no `:LINE`. `<NOUN_N>` is a distinctive domain noun.

Run every query with `searchResultMode: "count"` first, then fetch fields only for survivors.

**Q0, mandatory positive control, runs first.**
```
project in (<PROJECTS>) AND summary ~ "\"<NOUN_1>\""
```
Zero here means a mistyped field name or a wrong noun. Without it the sweep's negative verdict is unfalsifiable.

**Q1, identifier and path union. Highest precision.**
```
project in (<PROJECTS>) AND text ~ "\"<PATH_1>\" OR \"<PATH_2>\" OR \"<SYMBOL_1>\"" ORDER BY updated DESC
```
Finds the ticket naming the same code whoever wrote it. Nothing else finds a duplicate worded completely differently.

**Q2, summary noun union. Highest signal per hit.**
```
project in (<PROJECTS>) AND summary ~ "\"<NOUN_1>\" OR \"<NOUN_2>\" OR \"<NOUN_3>\"" ORDER BY updated DESC
```
Finds the duplicate describing the same work without naming a file, which Q1 structurally cannot see. Read these before Q1's hits even when Q1 returns more.

**Q3, the already-done pass.**
```
project in (<PROJECTS>) AND statusCategory = Done AND text ~ "\"<NOUN_1>\" OR \"<PATH_1>\"" ORDER BY updated DESC
```
Redundant by set membership and it still earns its place. Done is the majority of the corpus, so a closed duplicate sits deep in an unsegregated list.

**Q4, comment pointers.**
```
project in (<PROJECTS>) AND comment ~ "\"<NOUN_1>\" OR \"<PATH_1>\" OR \"<KEY_1>\"" ORDER BY updated DESC
```
Adds isolation, not coverage. "Superseded by", "already handled in" and "duplicate of" live in comment threads.

**Q5, loose stemmed net, unscoped by project.**
```
text ~ "<NOUN_1> <NOUN_2>" ORDER BY updated DESC
```
Deliberately unquoted, the only one here that is. Catches a duplicate sharing the concept but no exact string, including one filed outside `<PROJECTS>`. Keep it to two or three nouns. Read its hits with suspicion, since this is the query that produced a false positive for a path that does not exist.

### Search tool limits

`maxResults` maximum is 100, and values below 50 are honored. Pagination is cursor-based, with `nextPageToken` from `issues.pageInfo.endCursor`, and the cursor embeds the JQL and the sort. `ORDER BY` is honored across pages. Rate limit is 300.

### The git half

Runs alone, redirected to `/tmp/claude/`, read back in a separate call.

- `git log -G'<pattern>' --all --oneline` for code that already exists. Use `-G`, not `-S`.
- `git log --grep='<noun>' --all --oneline` for the commit message that already describes it.
- `git log --oneline --all -- <path>` for history on the files a leaf would touch.

POSIX character classes, not `\s`, which returns zero hits under `-P` on git 2.50.1.
