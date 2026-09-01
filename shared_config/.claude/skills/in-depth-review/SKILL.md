---
name: in-depth-review
description: >
  Performs one in-depth, multi-perspective code review of a pull request or commit range using
  up to twelve specialized parallel reviewer roles (AGENTS.md compliance, bug scan, git history,
  prior PR feedback, in-file comments, database, OWASP security, error handling, test coverage,
  ticket-intent compliance, motivation delivery, TypeScript type safety). Scores findings for
  confidence, filters, and deduplicates. Never writes to GitHub. Use for "in-depth review", "deep
  review", "thorough review", or "code review without fixing" -- standalone or as a building
  block for `pr-review` and `review-and-fix`.
---

# In-Depth Review

This skill performs ONE complete review pass over a target scope (a PR or a commit range)
using eight to twelve specialized reviewer roles, then scores, filters, and deduplicates findings. It
returns the result. It does NOT post anywhere, fix anything, or loop.

The multi-role specialization gives **cross-domain coverage** (style/standards, raw bugs, history,
prior PR feedback, in-file guidance, DB, security, error handling, tests, ticket intent). Within-role
triangulation (running the same role multiple times) is the **caller's** responsibility, not
this skill's. `review-and-fix` runs 2 of these per iteration, and `pr-review` runs 3.

## Argument

Accepts a positional arg plus optional modifier flags (`--raw`, `--skip-ticket`, `--roles`), in any order. Auto-detects mode from the positional arg:

- **PR mode** — arg looks like `123`, `#123`, or a GitHub PR URL. Diff source = `gh pr diff`.
  Prerequisites: open PR (draft PRs are accepted) + `gh` authenticated.
- **Branch mode** — arg is a git revision range (e.g. `origin/main..HEAD`, `HEAD~5..HEAD`).
  Diff source = `git diff` from the merge base (see Step 0 note on two-dot vs three-dot). No PR
  required.

If no arg is supplied: default to branch mode with range `origin/<default-branch>..HEAD`
(default branch detected via `git remote show origin | grep 'HEAD branch' | awk '{print $NF}'`,
fallback `main`).

### Modifier flags

- `--raw` — skip the internal `< 70` confidence filter. Return ALL scored findings (0–100).
  Callers that want to apply their own threshold (e.g. `pr-review` uses 60, `review-and-fix`
  uses 50) pass this flag.
- `--skip-ticket` — disable Reviewer Role #10 (ticket intent compliance). By default that
  role runs: it reads the Jira tickets referenced by the change and checks the code against
  them. Pass this to skip all ticket reading (no `acli` / Datadog calls, no related prompts).
  The orchestrators forward this flag from their own `--skip-ticket`.
- `--roles <csv>` — run ONLY the listed roles instead of all of them. Accepts role numbers
  (`1`..`12`) and/or their category names, comma-separated, e.g. `--roles 1,5` or
  `--roles "AGENTS.md,comment guidance"`. The number/name mapping is the table in Step 1.
  When the flag is **absent, all roles run** (the normal, complete review), so this flag is
  purely additive and existing callers are unaffected. It exists for iterative callers
  (`review-and-fix`) that rerun only the reviewers that were productive in the previous
  iteration. `--skip-ticket` still wins: it removes Role #10 from whatever set `--roles`
  selects. If `--roles` resolves to an empty set, abort with a clear reason (a caller bug).

Example invocations:

```
/in-depth-review 1234              # PR mode, default filter (>= 70)
/in-depth-review #1234 --raw       # PR mode, no filter (return all scored)
/in-depth-review 1234 --skip-ticket # PR mode, skip the ticket-intent role
/in-depth-review origin/main..HEAD # branch mode, default filter
/in-depth-review origin/main..HEAD --raw --roles 1,5  # only AGENTS.md + comment roles
/in-depth-review                   # branch mode, range = origin/<default-branch>..HEAD
```

## GitHub side-effect policy

**This skill never writes to GitHub.** Forbidden: `gh pr comment`, `gh pr review`,
`gh pr edit`, `gh pr close`, `gh pr merge`, `gh issue create`, `gh issue comment`, or any
other write command — even transitively through other skills.

Permitted read-only `gh` calls: `gh pr list`, `gh pr view`, `gh pr diff`, `gh search pulls`,
`gh search issues`.

If a sub-agent appears about to issue any write command, abort and surface the attempt to the
caller.

## Working-tree side-effect policy

This is a SECOND policy, not a restatement of the one above, and it is the one that has been
broken. "Read-only" has been read as "read-only with respect to GitHub" by the reviewer agents,
and the local tree went unnamed.

**This skill and every role it spawns leave the working tree alone.** Forbidden:
`git checkout -- <path>`, `git checkout .`, `git restore`, `git reset --hard`, `git clean`, `rm`,
`git push`, and any edit, creation, or deletion of a file. This holds however the skill was
invoked, and it holds hardest when it runs as a sub-agent, because then one checkout is shared
with the orchestrator and with every other reviewer in that run.

Two runs lost work before this policy was written down. One discarded a user's uncommitted edits
with no stash entry to recover from. Another reverted a file mid-review, and concurrent roles read
the reverted state and reported on it.

**A negative control is not this skill's to run.** Reverting a fix to watch its test fail is a
good check, and it needs the tree, which a reviewer does not own. Report the finding and say a
negative control would confirm it. The caller owns the tree and can run one.

To read content at another revision, use `git show <ref>:<path>`. It writes nothing, so it is
always the right way to see a pre-change state.

If a sub-agent appears about to change tracked content, abort and surface the attempt to the
caller, the same way you would for a GitHub write.

## GitHub access (GitHub MCP with `gh` fallback)

Every GitHub call below is written as a `gh` command for reference. **Prefer the GitHub MCP
server when it is connected; use the `gh` command only as a fallback when no GitHub MCP is
available (or its tools don't cover the call).** Discover the MCP tools with
`ToolSearch "github pull request"` and call the operation matching the `gh` call:

| `gh` call used here | GitHub MCP equivalent (confirm exact name via ToolSearch) |
|---|---|
| `gh pr view <N> --json …` | get pull request (metadata, title/body/baseRefName/commits) |
| `gh pr diff <N>` | get pull request diff |
| `gh pr diff <N> --name-only` | get pull request files (changed files) |
| `gh pr list --search …` | list / search pull requests |
| `gh pr view <N> --comments` | get pull request review comments + issue comments |

Prefer the GitHub MCP when connected; fall back to `gh` only when no MCP is available. Both
paths are **read-only** here. Read calls map to read tools, so the "never writes to GitHub"
constraint is unchanged. If NEITHER a GitHub MCP nor `gh` is available, PR mode cannot proceed:
abort Step 0 with that reason. Local `git` calls (`git diff`, `git log`, `git blame`,
`git rev-list`) need no `gh` and are unaffected. Branch mode works without GitHub entirely.

## Fan-out prerequisite

**This skill requires the `Agent` tool.** Every reviewer role is a sub-agent, so a context without
that tool cannot run a single one of them. Check for the tool before Step 1. When it is missing,
launch nothing and emit Step 1's `REVIEW_UNAVAILABLE_NO_FANOUT` abort instead.

A workflow agent (one spawned by the `Workflow` tool's `agent()` call) carries no `Agent` tool, and
that is the usual cause. The fix is to re-run this skill from the main thread, where the tool is
present. This is a fact about the context, not about the target scope, so the re-run needs no change
to the argument.

## Step 0: Resolve scope

1. Parse the argument. First split it on whitespace into tokens; classify each token, then apply mode detection to the lone non-flag token:
   - Matches `^#?[0-9]+$` or a GitHub PR URL -> **PR mode**; `<PR>` = the number.
   - Matches `^--raw$` -> flag (defer until Step 4).
   - Matches `^--skip-ticket$` -> flag; when set, Role #10 is omitted in Step 1.
   - Matches `^--roles$` (followed by its value) or `^--roles=...$` -> flag; parse the
     comma-separated value into `<ROLE_SET>` (role numbers 1..12 and/or category names via
     the Step 1 table). When the flag is absent, `<ROLE_SET>` = all roles. `--skip-ticket`
     removes Role #10 from `<ROLE_SET>` here, since that needs no diff data.
     **Do NOT evaluate the conditional gates (#6, #7, #12) in Step 0, and do not run the
     empty-`<ROLE_SET>` abort check here.** Those gates read the changed-file list, which Step 0
     only records as `<FILES_COMMAND>` without running. Step 1 fetches the file list once,
     evaluates all three gates, narrows `<ROLE_SET>` further, and owns the abort. Step 0 records
     the caller's selection. Step 1 finalizes it.
   - Anything else -> **branch mode**; `<RANGE>` = the arg.
   - If no positional arg: branch mode with `<RANGE>` = `origin/<default-branch>..HEAD`.

2. **PR mode preflight:**

   ```
   gh pr view <PR> --json number,state,isDraft,headRefOid,url
   ```

   If exit non-zero or `state != "OPEN"`, abort with a clear reason. **Draft PRs are
   accepted.** Reviewing early is a valid workflow. Store `<PR_HEAD_SHA>` for downstream
   tooling (e.g. permalink construction); also note `isDraft` so callers that care (e.g.
   `pr-review`) can mention it in their output.

3. **Branch mode preflight:**
   - `git rev-list --count <RANGE>` — if 0, abort: "no commits in <RANGE>".
   - `<PR_HEAD_SHA>` is unset in branch mode; permalinks for findings are produced
     against `git rev-parse HEAD` as a best-effort fallback.
   - Convert `<RANGE>`'s two dots to three (`<BASE>..<HEAD>` becomes `<BASE>...<HEAD>`) before
     using it in a `git diff` command — do not pass the literal `<RANGE>` string through to
     `git diff`. Unlike `git log` / `git rev-list`, where two dots already means "commits in
     HEAD not in BASE" (merge-base aware), `git diff`'s two-dot form is a literal comparison of
     the two commits' trees with no merge-base logic at all. If `<BASE>` has advanced
     independently since the branch diverged, a two-dot `git diff` shows every one of those
     independent `<BASE>`-side commits reverted, on top of the branch's real changes. Three-dot
     diffs from the merge-base instead, which is what "what did this branch change" means, and
     is a no-op change when there's no divergence to begin with. The `git rev-list --count
     <RANGE>` call above keeps its two-dot form; it is unaffected.

4. Save:
   - `<SCOPE_DESCRIPTION>` — human-readable scope (e.g. `PR #1234` or `origin/main..HEAD`)
   - `<DIFF_COMMAND>` — `gh pr diff <PR>` (PR mode) or `git --no-pager diff <BASE>...<HEAD>` (branch)
   - `<FILES_COMMAND>` — `gh pr diff <PR> --name-only` or
     `git --no-pager diff --name-only <BASE>...<HEAD>` (branch)

## Step 1: Launch the specialized reviewers in parallel

Spawn the reviewer sub-agents in a single message (concurrent tool-use blocks). Launch
exactly the roles in `<ROLE_SET>` (Step 0). Sequential launches defeat the purpose of this
design. Never serialize.

**Record the `agentId` of every role you launch.** That set is what you collect against, and it is
the accounting baseline for Step 2.0. The launch result does NOT contain a role's findings. Read
"How a role's findings reach the parent" below before you decide what to do after launching, because the
obvious next action is the one that loses roles.

**No fan-out, no review.** Two triggers, and either one aborts the run. Before launching, if the
`Agent` tool is not among your available tools, abort and do not attempt the launch at all. If you do
attempt it and every launch call fails because the tool is unavailable, so that zero roles launched,
abort as well. On either trigger, emit the output below and STOP. Do not continue to Step 2, and do
not read the diff yourself.

Invoked directly by the user, the entire output is this one line:

```
REVIEW_UNAVAILABLE_NO_FANOUT: this skill launches its reviewer roles as sub-agents, and this context has no Agent tool, so no role could run. Re-run it from the main thread. A workflow agent is the usual cause.
```

Invoked as a sub-agent, return the no-fanout payload in [OUTPUT-JSON.md](OUTPUT-JSON.md), which
carries `coverage: "impossible"`, an empty `roles_launched`, `roles_missing`, and `findings`, plus the
exact `skipped_reason` documented there.

**This is not the empty-`<ROLE_SET>` abort.** That one fires when the gates and a caller's `--roles`
leave no role selected, which is a caller bug in a context that could have hosted the review. This
one fires when the context cannot host the skill at all. Say which one happened. A caller that reads
a no-fanout abort as a caller bug will go and fix the wrong thing.

**Eight roles always run: #1, #2, #3, #4, #5, #8, #9, #11**, plus #10 unless `--skip-ticket`.
Three more are **conditional.** Evaluate each gate against the diff before launching, and skip
the role entirely (not counted in the total) when its gate is false:

| role | runs when | gate detail |
|---|---|---|
| #6 database / data-layer | the diff touches data-layer code | see below |
| #7 OWASP security | almost always — skip only a provably no-surface diff | see below |
| #12 TypeScript type safety | the diff touches `*.ts` / `*.tsx` / `*.mts` / `*.cts` | see below |

So the launch count is **9 to 12** normally, and **8 to 11 with `--skip-ticket`**. Nine always run,
plus up to three gated, minus #10 when `--skip-ticket` drops it. Do not try to memorize a single
number. Evaluate the gates.

**This step owns gate evaluation and the abort.** Get the file list once
(`gh pr diff <PR> --name-only` in PR mode, `git --no-pager diff --name-only <BASE>...<HEAD>` in
branch mode — three dots, per the Step 0 note) and reuse it for all three gates rather than
re-running it per role. Then narrow `<ROLE_SET>` by the
gate results, and if that leaves it empty, abort with "no roles selected". Step 0 cannot do any of
this, because the file list does not exist yet when arguments are parsed.

### Conditional role gates

**The three gated roles are conditional, not optional.** A gate answers "is there anything in
this diff for this lens to read", and it is a cost guard, not a quality dial. Skipping a
language- or domain-specific lens on a diff without that language or domain is free; skipping
it on a diff that HAS one is a missed finding. Never resolve a gate as false to save an agent.
When a gate is genuinely ambiguous, run the role.

#### Reviewer Role #6 — Database / data-layer scan (conditional)

**Runs only when the diff touches data-layer code.** Launch it when ANY of these hold:

- A changed file is a migration or schema file (`migrations/`, `db/migrate/`, `schema.rb`,
  `schema.prisma`, `*.sql`).
- A changed file sits in a data-layer path (`models/`, `repositories/`, `repos/`, `dao/`,
  `entities/`, `queries/`, `persistence/`).
- The diff's added or modified lines contain SQL (`SELECT`, `INSERT`, `UPDATE`, `DELETE`,
  `JOIN`, `CREATE TABLE`, `ALTER TABLE`) in a string, heredoc, or query builder.
- The diff touches an ORM or query builder (Prisma, Sequelize, TypeORM, Knex, Drizzle, Mongoose,
  ActiveRecord, SQLAlchemy, Django ORM, GORM, Ecto, Diesel) or imports the repo's own db module.
- The diff changes transaction handling, connection pooling, or a caching layer that fronts a
  database.

Skip it when none hold. A data-layer lens on a diff with no data layer has nothing to read.
This gate is mechanical. Data-layer code is identifiable from the diff without judgement calls,
which is why this role is gated and #7 is not gated the same way.

**When in doubt, run it.** A missed N+1, unbounded SELECT, or transaction bug is a production
incident; a wasted agent is not. Prefer a false positive on the gate over a false negative.

#### Reviewer Role #7 — OWASP Top 10 security scan (conditional, but bias hard toward running)

**This gate is inverted relative to #6 and #12.** It is defined by what lets you SKIP, not by what
makes it run.** Default to running this role. Skip it only when the diff is provably free of
security surface, which means ALL of these hold:

- No changed file is executable code. The diff touches only documentation, comments, markdown,
  changelogs, or a formatting-only reflow.
- No dependency manifest or lockfile changed (`package.json`, `requirements.txt`, `go.mod`,
  `Gemfile`, `Cargo.toml`, any `*.lock`). A version change is a supply-chain surface.
- No configuration, environment, secret, IAM, or CI/CD file changed.

If you find yourself constructing an argument for why some code change is safe, stop and run the
role. The gate is a formality for docs-only diffs, not a judgement call about code.

**Why this gate is narrow, unlike #6's.** Security surface is not mechanically identifiable the
way data-layer code is. Almost anything touching input, I/O, rendering, deserialization, paths,
or dependencies has one. Measurement backs this up: in the role-subsetting experiment the security
lens returned empty on a TTL cache and a CSV parser, and both of those DO have surface. A cache
with unbounded user-controllable keys is a DoS vector, and a CSV parser consumes untrusted input.
Those empty results meant "looked, found no vulnerability", which is a successful review, not a
wasted agent. Do not mistake one for the other and widen this gate.

A false negative here is the most expensive miss in the whole role set. Asymmetric cost gets an
asymmetric gate.

#### Reviewer Role #12 — TypeScript type safety (conditional)

**This role runs ONLY when the diff touches a TypeScript file** (`*.ts`, `*.tsx`, `*.mts`,
`*.cts`). Check with `gh pr diff <PR> --name-only` (PR mode) or
`git --no-pager diff --name-only <BASE>...<HEAD>` (branch mode, three dots) before launching it.
If the diff has no TypeScript, skip this role entirely and do not count it against the role total.

It exists because a narrow lens catches what a broad one misses. Role #1 reads AGENTS.md in full
and nominally covers this ground, but casting violations are a specific, mechanical, easily-missed
pattern in a large compliance sweep. The conditional launch is what keeps the extra recall from
costing anything on the many diffs with no TypeScript in them.

### How a role's findings reach the parent

Follow the collection protocol in [COLLECTING.md](COLLECTING.md) exactly: here, the "agent" is
the reviewer role you just launched, and the missing-bookkeeping structure is `roles_missing`
(Step 2.0).

When a caller passed `--roles`, launch only that subset (e.g. two sub-agents for `--roles 1,5`).
**A gate still wins over explicit selection.** `--roles 6` on a diff with no data-layer code skips
Role #6, and if that empties `<ROLE_SET>` the run aborts per Step 0. Naming a role does not
override its gate, because a gated-off role has nothing to review.

The role number ↔ category mapping used by `--roles` and by the `category` field of every
finding. Gate criteria for #6, #7, and #12 are in Conditional role gates above; the `File`
column below holds the role's prompt text (the fenced block):

| # | Role | `category` | File |
|---|------|------------|------|
| 1 | AGENTS.md compliance | `AGENTS.md` | `roles/01-agents-md.md` |
| 2 | Shallow bug scan | `bug` | `roles/02-bug-scan.md` |
| 3 | Git history context | `history` | `roles/03-history.md` |
| 4 | Prior PR comments | `prior PR` | `roles/04-prior-pr.md` |
| 5 | In-file code comments | `comment guidance` | `roles/05-comment-guidance.md` |
| 6 | Database / data-layer | `db` | `roles/06-database.md` |
| 7 | OWASP Top 10 security | `security` | `roles/07-security.md` |
| 8 | Error handling | `error-handling` | `roles/08-error-handling.md` |
| 9 | Test coverage | `test coverage` | `roles/09-test-coverage.md` |
| 10 | Ticket intent compliance | `ticket` | `roles/10-ticket.md` |
| 11 | Headline-benefit / motivation | `motivation` | `roles/11-motivation.md` |
| 12 | TypeScript type safety (conditional) | `types` | `roles/12-types.md` |

**Model and effort: spawn every reviewer by `subagent_type: in-depth-review-role`, and pass no
`model` override.** That file under `.claude/agents/` pins Opus at effort `low`. Never let a
reviewer inherit the session model or the session effort.

The Agent tool has no `effort` parameter, so an unset effort silently tracks whatever the user
last set with `/effort`. Pinning the tier in the agent definition is the only way to fix both
knobs together, which is why the role is addressed by `subagent_type` rather than by a `model`
argument here.

**If `subagent_type: in-depth-review-role` does not resolve** (the agent files have not been
synced to `~/.claude/agents/` yet, or were renamed), do not abort the run. Fall back to a plain
Agent call with `model: opus`, and say in the report that effort could not be pinned and therefore
inherited the session value. An absent `Agent` tool is a different failure. That one aborts under the
no-fanout rule above rather than falling back.

Why this tier. A replicated A/B measured Opus at `low` against Sonnet at `xhigh` over ten roles
and three passes per arm, on a diff about Postgres transaction semantics. Opus at `low` found four
keyed defects that Sonnet never found in any pass, Sonnet found none that Opus missed, and it cost
56% as much per pass. Turn count is the reason, not token price. Sonnet at `xhigh` took 2.1x the
assistant turns, and every turn re-reads the cached context, so cache reads dominate the bill.
Per-token price favors Sonnet. Per-task cost does not. That was one diff in one domain, so
re-measure before assuming it holds for a very different shape of change.

Confidence is recovered downstream by the cross-role agreement count and, for the orchestrators,
the triangulation and adversarial converge stage. It is not recovered by making each finder more
expensive.

### Common reviewer prompt fragment

Every reviewer prompt starts with the block in [roles/_common-fragment.md](roles/_common-fragment.md).
Read it once per run and prepend it to each launched role's prompt (from the File column in the
table above) before sending.

## Step 2: Confidence scoring

### Step 2.0: Account for every launched role BEFORE pooling anything

Build `roles_launched` = the role numbers you actually spawned in Step 1 (after the gates and any
`--roles` subset). Then, for each one, classify its response:

- **Reported** — returned a parseable findings list, `NO_ISSUES_FOUND`, or one of the documented
  `TICKET_REVIEW_*` sentinels.
- **Missing** — returned nothing, errored, was skipped by the harness, returned output you
  cannot parse as any of the above, or never reported before Step 1's give-up bound.

Record every missing one in `roles_missing` (role number + why, e.g. `4=empty response`). When a role
was launched but no `<task-notification>` for it arrived before Step 1's give-up bound, the reason is
`no notification received`. That is a distinct cause from an empty or unparseable response, because
the role may still be running. `roles_missing` also holds every role whose launch call failed. Such
a role never spawned, so it stays out of `roles_launched`, and its reason is `launch failed`.

**NEVER FABRICATE A MISSING ROLE'S OUTPUT.** This is the sharpest rule in this step. When a role
does not report, the correct output is a hole, explicitly labelled. Do not:

- write findings you think that role *would* have found;
- infer its verdict from the other roles' results;
- run its lens yourself in the parent and present the result as that role's report;
- reuse a previous iteration's output for it.

A hole is a fact about the run. Filling it in converts "we did not check" into "we checked and it
was fine", which is the single most damaging thing this skill can emit. If you catch yourself
reasoning about what the missing role would have said, stop and record it as missing.

**A missing role is NOT a clean role.** Never treat a non-response as `NO_ISSUES_FOUND`, and never
describe coverage you did not get. Concretely:

- If `roles_missing` is non-empty, the review is **partial**. Say so in the output, name the roles,
  and never emit a bare "no issues found" that implies the full set ran.
- Do not assert or imply a conclusion that depends on a role that did not report. If Role #7 never
  returned, the review has NOT cleared the diff on security. It is silent on security.
- Report the partial result anyway. A review missing one role is still useful. A review that
  silently claims completeness it does not have is worse than no review. Three honest roles beat
  twelve invented ones.

**There is a floor under partial.** Some roles missing is **partial**, a degraded review that is
still a real one, and it belongs in the output exactly as above. Zero roles launchable because the
`Agent` tool is absent is not **partial**. It is **impossible**, it aborts per Step 1, and it never
reports through this channel, because a run that launched nothing has no coverage to be honest
about. Every role launched and every one missing is still **partial**, because the launch itself
worked. Zero roles launched for any other reason is not **complete** either. Record every role you
could not launch in `roles_missing` with the launch failure as its reason, which makes the run
**partial**.

`roles_missing` is a required field of the Step 4 output even when empty.

### Step 2.1: Pool and score

Once collection has ended, whether every role reported or the Step 1 give-up bound was reached:

1. Before pooling, scan every reviewer response: if a response begins with
   `TICKET_REVIEW_UNAVAILABLE:` or `TICKET_REVIEW_SKIPPED:`, set it aside to populate
   `ticket_review` (Step 4). It is NOT a finding, so do not pool it or send it to a scorer.
   Then pool every remaining non-clean response (each is a list of findings). `NO_ISSUES_FOUND`
   from any role is an empty (clean) response.
2. **Pre-score deduplication** — group findings that look like duplicates (same file +
   overlapping line range + substantially the same problem). For each group, keep one canonical
   entry and record **`role_agreement`** (how many of the role outputs raised it).
   Highest severity in the group wins. **`citation_verified` merges downward**: if ANY member of
   the group has `citation_verified: false`, the canonical entry carries `false`. Never let a
   verified member's `true` overwrite an unverified member's `false`. Merging is not verification.
   The pessimistic direction is the only safe one here, because callers exclude
   `citation_verified: false` outright and an upward merge would silently smuggle an unverified
   citation past that exclusion.
3. **Launch the scoring sub-agents in parallel, in BATCHES of about ten findings each**, all in
   a single message. **Spawn each scorer on Haiku** (Agent-tool `model: haiku`). Do not let it
   inherit the session model either.

   **Batch, rather than one agent per finding.** A measured instance raised 54 unique findings,
   which under the old one-to-one shape meant 54 concurrent scorers, and 53 of them never
   reported. That instance's findings were all `unscored`, which is below every caller threshold,
   so a complete review was thrown away by its own scoring stage. [COLLECTING.md](COLLECTING.md)
   also owes at least as many collecting turns as agents launched before its give-up counter may
   arm, so 54 scorers bought 54 turns of polling in the orchestrator that then had to aggregate.
   Ten per batch turns both numbers into single digits.

   **Group a batch by file where the findings allow it**, so one diff excerpt serves several
   findings rather than being pasted once per finding.

   What batching must not change is that a different model scores than proposed. That is the
   invariant below, and the batch size has nothing to do with it. Nothing here or in
   [SCORING.md](SCORING.md) ever asked for one agent per finding.

**Collect the scorers exactly per [COLLECTING.md](COLLECTING.md)**: here, the "agent" is the
scorer you just launched, and a scorer that never reports leaves its finding `unscored: true`
with `confidence: null` (see below) instead of any `roles_missing`-style list.

   Scoring one finding against the rubric is a small, structured judgment
   with the diff and AGENTS.md handed in, not open-ended reasoning; Haiku is ~15–20x cheaper
   than Opus for it. Give each scorer, per finding in its batch:
   - The finding (file, line, severity, description, suggested fix)
   - The path of every AGENTS.md / CLAUDE.md file referenced by any reviewer that raised it
   - The diff for the relevant lines
   - The `role_agreement` count

   **Tell the batch to score each finding against the rubric alone, never against its
   batch-mates.** This is the one risk batching adds and it does not announce itself. A scorer
   holding ten findings can rank them, scoring the worst of the ten low and the best high, and
   return ten numbers that are internally sensible and wrong against
   [SCORING.md](SCORING.md)'s bands. The bands are absolute. A batch of ten weak findings scores
   ten low numbers, and a batch of ten strong ones scores ten high numbers, and neither outcome
   is evidence the scorer erred. Say so in the prompt, and require one score per finding with the
   finding's own identifier beside it rather than a ranked list.

   **The scoring stage is MANDATORY and is not yours to perform.** Confidence is a second-stage
   judgment by a different model than the one that proposed the finding. That two-stage split is
   the whole reason a confidence number means anything here.

   - **Never self-assign a confidence value.** If you find yourself writing a score without
     having spawned a scorer for that finding, you have collapsed the two stages into one model
     grading its own work, and the number is worthless.
   - **A reviewer-authored confidence value is not a score.** Roles emit `severity`, which is
     theirs to judge. If a role also emits a confidence number, DISCARD it and score the finding
     properly. Never pass a role's own number through as the score.
   - **Count findings scored, not agents spawned.** Record `findings_scored` and compare it to the
     number of unique findings after dedup. They must be equal. Under batching `scorers_spawned`
     is roughly a tenth of that and says nothing about coverage, because one batch that returns
     eight scores for the ten findings it was given leaves two findings unscored while the agent
     itself reported. Keep recording `scorers_spawned` for the run log, and never compare it to
     the finding count.
   - **A finding with no score is `unscored`, not confident.** Set its confidence to `null`,
     mark it `unscored`, and treat it as BELOW every caller threshold. It must never be posted
     or reported as a finding. List it separately as unscored so the gap is visible. This is
     per finding rather than per agent, so a batch that comes back short leaves exactly the
     findings it omitted unscored and the rest of its batch keeps its scores.
   - If `findings_scored` is 0 while unique findings exist, the run is **degraded, not clean**:
     emit `scoring.complete: false`, report every finding as unscored, and say plainly that no
     confidence filtering happened. A scoring stage that cannot spawn anything at all looks like
     Step 1's no-fanout abort. It is not one. Step 1 launched its roles, so the `Agent` tool is
     present and the real reason lies elsewhere.

   Skipping this stage is not a shortcut, it is a correctness failure. It has already produced a
   fabricated finding that scored 100 and survived the filter, because the model that invented the
   precedent was also the model that graded it.

Score each finding against the rubric in [SCORING.md](SCORING.md), including the citation
hard-cap and the ticket/motivation scoring specifics there.

## Step 3: Filter and dedup

1. **Filter** — unless invoked with `--raw`, discard findings with `confidence < 70`. The
   70 threshold is slightly more permissive than upstream `code-review`'s default of 80. That is
   intentional, because the multi-role rubric here is wider than upstream's 5 roles and the extra
   roles (DB, security, error-handling, test coverage) tend to score in the 70-79 band even
   when they're genuinely useful.

   When `--raw` is in effect, **skip this step** and pass all findings (0–100) through to
   Step 4. Callers that apply their own threshold (`pr-review` uses 60, `review-and-fix`
   uses 50) use this mode.

2. **Post-score dedup** — re-check for duplicates one more time, in case scoring revealed two
   findings that scored identically and pointed at the same locus. Keep the highest severity,
   union of categories, and max `role_agreement`. `citation_verified` merges downward here too.
   Any `false` in the group makes the merged entry `false`.

   **Confidence is max, then re-capped.** Take the max confidence in the group, but if the merged
   entry ends up `citation_verified: false`, re-apply the 60 cap afterwards. Taking a verified
   member's higher score and keeping it would launder an unverified finding above its cap, which
   contradicts "never resolve an unverified citation upward". The cap applies to the merged entry,
   not just to the members.

   The order matters, so here it is concretely. A group holds member A scoring 60 with
   `citation_verified: false`, and member B scoring 85 with `citation_verified: true`. The field
   merges downward to `false`. Confidence takes the max, 85. The re-cap then applies because the
   merged entry is unverified, giving a final **60**, not 85. Do it in the other order and the
   finding ships at 85 with an unverified citation.

## Step 4: Return / report

Order surviving findings by:

1. Severity descending (critical -> major -> minor -> suggestion)
2. `role_agreement` descending (more roles raised it, higher priority)
3. Confidence descending

### If invoked as a sub-agent (by `review-and-fix` or `pr-review`)

See [OUTPUT-JSON.md](OUTPUT-JSON.md) for the exact JSON shape and field semantics.

### If invoked directly by the user

See [OUTPUT-CHAT.md](OUTPUT-CHAT.md) for the chat report template and formatting rules.

