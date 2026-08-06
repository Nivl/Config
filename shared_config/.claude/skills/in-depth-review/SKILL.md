---
name: in-depth-review
description: >
  Performs one in-depth multi-perspective code review of either a pull request or a commit
  range. Launches EIGHT TO TWELVE specialized parallel reviewer roles. Eight are unconditional
  (AGENTS.md compliance, shallow bug scan, git history context, prior PR comments, in-file code
  comments, error handling, test coverage, headline-benefit / motivation delivery). A ninth, ticket
  intent compliance, runs unless `--skip-ticket` is passed, which is why the floor is eight rather
  than nine. Three are gated on what the diff actually
  contains: database / data-layer (only when the diff touches data-layer code), OWASP Top 10
  security (skipped only for a diff with no executable code, no dependency change, and no config
  change), and TypeScript type safety (only when the diff touches TypeScript). It then scores each finding
  0–100 for confidence, filters anything below 70, and deduplicates. Returns the surviving findings.
  Never writes to GitHub.
  Used as one of two parallel review primitives (the other being `gh-style-review`) by
  `review-and-fix` (which spawns 2 of these plus 1 `gh-style-review` per iteration, may rerun only
  a subset of roles via `--roles` on later iterations, and adds a fix/commit loop) and `pr-review`
  (which spawns 3 of these — 2 on Sonnet, 1 on Opus — plus 1 `gh-style-review`, merges them into
  one flat pool, and posts a single PR review with inline + global comments).
  Use this skill when the user asks for "in-depth review", "deep review", "thorough review",
  "code review without fixing", or invokes it directly to get a one-shot review report.
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
  Diff source = `git diff <RANGE>`. No PR required.

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

4. Save:
   - `<SCOPE_DESCRIPTION>` — human-readable scope (e.g. `PR #1234` or `origin/main..HEAD`)
   - `<DIFF_COMMAND>` — `gh pr diff <PR>` (PR mode) or `git --no-pager diff <RANGE>` (branch)
   - `<FILES_COMMAND>` — `gh pr diff <PR> --name-only` or `git --no-pager diff --name-only <RANGE>`

## Step 1: Launch the specialized reviewers in parallel

Spawn the reviewer sub-agents in a single message (concurrent tool-use blocks). Launch
exactly the roles in `<ROLE_SET>` (Step 0). Sequential launches defeat the purpose of this
design. Never serialize.

**Record the `agentId` of every role you launch.** That set is what you collect against, and it is
the accounting baseline for Step 2.0. The launch result does NOT contain a role's findings. Read
"How a role's findings reach the parent" below before you decide what to do after launching, because the
obvious next action is the one that loses roles.

**Nine roles always run: #1, #2, #3, #4, #5, #8, #9, #11**, plus #10 unless `--skip-ticket`.
Three more are **conditional.** Evaluate each gate against the diff before launching, and skip
the role entirely (not counted in the total) when its gate is false:

| role | runs when | gate detail |
|---|---|---|
| #6 database / data-layer | the diff touches data-layer code | see Role #6 |
| #7 OWASP security | almost always — skip only a provably no-surface diff | see Role #7 |
| #12 TypeScript type safety | the diff touches `*.ts` / `*.tsx` / `*.mts` / `*.cts` | see Role #12 |

So the launch count is **9 to 12** normally, and **8 to 11 with `--skip-ticket`**. Nine always run,
plus up to three gated, minus #10 when `--skip-ticket` drops it. Do not try to memorize a single
number. Evaluate the gates.

**This step owns gate evaluation and the abort.** Get the file list once
(`gh pr diff <PR> --name-only` in PR mode, `git --no-pager diff --name-only <RANGE>` in branch mode)
and reuse it for all three gates rather than re-running it per role. Then narrow `<ROLE_SET>` by the
gate results, and if that leaves it empty, abort with "no roles selected". Step 0 cannot do any of
this, because the file list does not exist yet when arguments are parsed.

### How a role's findings reach the parent

**A role's findings reach the parent through the text that role returns, and nowhere else.**
Every role prompt must instruct the role to put its COMPLETE findings list in its FINAL
TEXT OUTPUT.

**Roles send nothing anywhere.** Never instruct a role to use `SendMessage`, agent teams, or a shared
file. A role has no resolvable address for you. An agent TYPE is not an agent id, so an attempt fails
every time with `no reachable agent named <your agent type>`. One measured run burned 256 such
attempts and delivered nothing. The findings go in the returned text, and that is the only channel.

**How that returned text actually reaches you.** The Agent tool here launches ASYNCHRONOUSLY. Read
this before deciding what to do after launching, because the intuitive reading is wrong:

- The Agent tool result gives you launch metadata and an `agentId`. It does NOT contain the role's
  output. There is nothing to read at launch. Do not wait on a launch-time return value, and do not
  treat its absence as a failure.
- A role's output arrives LATER, inside a `<task-notification>` block, usually several batched into
  one turn. That block's `<result>` body holds the role's final text, and its `<task-id>` is
  byte-identical to the `agentId` you recorded at launch.
- **You receive notifications only while you keep taking turns.** Any tool call is a turn.
- **A parent that ends its turn is FINALIZED and stops receiving.** Its outstanding roles' results
  then surface in the session root, where you cannot see them. This is the one behaviour that loses
  roles. It is why "wait for the results" is not an instruction anyone can follow, and why a progress
  note is not a safe thing to emit. Emitting one ends your turn.

**The collection protocol.** Follow it exactly.

1. Record every `agentId` the launch returned.
2. Take a turn. Harvest every `<task-notification>` in front of you, match each `<task-id>` to a
   recorded `agentId`, and keep its `<result>` body.
3. If any recorded `agentId` is still unaccounted for, take another turn. If you have no productive
   work, re-read the diff for a role you are still waiting on. An unproductive turn still collects.
4. Repeat until every recorded `agentId` is accounted for, OR until THREE CONSECUTIVE COLLECTING
   TURNS have brought zero new arrivals. A collecting turn is ONE substantive tool call that names
   the artifact it read, so re-read the diff or a changed file. Three repeats of the same no-op are
   not three turns.
   Do NOT start the zero-arrival counter until you have taken at least as many collecting turns as you
   launched roles, with a floor of five, whether or not anything has arrived. The three zero-arrival
   turns are counted FRESH from the moment the counter arms, so turns taken before arming never count
   toward them. Roles take minutes, not seconds, so a counter armed at launch measures your own
   polling speed rather than a failure. If you reach the bound and have not yet re-armed the counter,
   re-arm it EXACTLY ONCE and keep collecting. After that, honor the bound.
5. At that bound, once you have already used your single re-arm, record every still-unaccounted role
   in `roles_missing` per Step 2.0, close the accounting, and continue to Step 2.0. Do not wait
   longer. Do not treat a give-up as clean.

A notification that arrives AFTER the bound is still that role's report. Fold it into the pool and
remove that role from `roles_missing`. The accounting is final only at the moment you emit your Step 4
output, not at the moment the bound fires.

**Never end your turn while a recorded `agentId` is unaccounted for**, unless you are declaring it in
`roles_missing` on that same turn.

**Your final output is the report, never a status update.** If roles are still unaccounted for when
you hit the bound, emit the report with them in `roles_missing`. Never return a progress note saying
you are still waiting, because that ends your turn and finalizes you with less than you could have
collected.

`TaskOutput` appears in the deferred-tool listing and looks like exactly the right tool for this. It
is NOT available to a nested agent. `ToolSearch select:TaskOutput` returns no match from inside a
role parent, though the same query resolves at the session root. Do not build on it. The protocol
above is the mechanism.

**A role that never reports has reported nothing**, whatever it may have produced elsewhere.
Classify it as missing per Step 2.0. Do not hunt for its output in another channel and splice it in.
That makes delivery depend on luck rather than on the contract.

This section is written this way because the shorter version failed in practice. A parent told only
to read a result that the launch never produced had nothing to read, stopped, and then invented
findings for three roles it never heard from.

When a caller passed `--roles`, launch only that subset (e.g. two sub-agents for `--roles 1,5`).
**A gate still wins over explicit selection.** `--roles 6` on a diff with no data-layer code skips
Role #6, and if that empties `<ROLE_SET>` the run aborts per Step 0. Naming a role does not
override its gate, because a gated-off role has nothing to review.

The role number ↔ category mapping used by `--roles` and by the `category` field of every
finding:

| # | Role | `category` |
|---|------|------------|
| 1 | AGENTS.md compliance | `AGENTS.md` |
| 2 | Shallow bug scan | `bug` |
| 3 | Git history context | `history` |
| 4 | Prior PR comments | `prior PR` |
| 5 | In-file code comments | `comment guidance` |
| 6 | Database / data-layer | `db` |
| 7 | OWASP Top 10 security | `security` |
| 8 | Error handling | `error-handling` |
| 9 | Test coverage | `test coverage` |
| 10 | Ticket intent compliance | `ticket` |
| 11 | Headline-benefit / motivation | `motivation` |
| 12 | TypeScript type safety (conditional) | `types` |

**Model: spawn every reviewer on Sonnet** (Agent-tool `model: sonnet`). Do NOT let them
inherit the session model. Each role is a bounded, tightly-specified recall pass over the
diff; that is exactly the work Sonnet does well and Opus does at ~5x the cost. Confidence is
recovered downstream by the cross-role agreement count and (for the orchestrators) the
triangulation + adversarial converge stage — not by making each finder more expensive.

### Common reviewer prompt fragment

Every reviewer prompt starts with this block:

```
Scope: <SCOPE_DESCRIPTION>

Run `<DIFF_COMMAND>` to see all changes.
Run `<FILES_COMMAND>` if you need just the list of changed files.

[PR mode only] Also run `gh pr view <PR> --json title,body,baseRefName` for PR context.
[Branch mode] Run `git --no-pager log <RANGE> --oneline` for commit context.

Also read the project AGENTS.md and CLAUDE.md files (root + sub-project) relevant to the
changed files. They contain mandatory quality standards and coding conventions.

Discount or downscore findings that look like:
- Pre-existing issues (on lines the diff didn't touch)
- Things that look like bugs but aren't on closer inspection
- Pedantic nitpicks a senior engineer wouldn't raise
- Issues a linter / typechecker / compiler would catch (assume CI runs these, so don't run them)
- General code-quality complaints (test coverage, generic security advice, doc) unless explicitly
  required by AGENTS.md (these have dedicated roles below; don't generalize from them)
- Issues called out in AGENTS.md but explicitly silenced in code (e.g. a lint-ignore comment)
- Changes in functionality that are obviously intentional or directly related to the broader change

Return a structured list of findings. For each finding include:
- File path and line number(s)
- Severity: critical | major | minor | suggestion
- Clear description of the issue or improvement
- Suggested fix (code snippet or approach)

If you find NO issues, respond with exactly: "NO_ISSUES_FOUND"

You are one of the reviewers running concurrently (up to 12; fewer when the caller
restricted the set via `--roles`). Do NOT coordinate with the others.

IMPORTANT: Do not run `gh pr comment`, `gh pr review`, `gh pr edit`, or any command that
writes to GitHub. Read-only gh commands (gh pr list / view / diff / search) are permitted
only if your role below explicitly requires them. Prefer the GitHub MCP read tools (find them with
ToolSearch "github pull request"); fall back to those read-only `gh` commands only when no MCP
is connected. Both are equally read-only. Never use a write tool.
```

### Reviewer Role #1 — AGENTS.md compliance

```
Your job: audit the changes for compliance with the relevant AGENTS.md / CLAUDE.md files
(root + any sub-project AGENTS.md whose directory the diff touches). Read each one in full.

Note that AGENTS.md is guidance for Claude as it WRITES code, so not every rule is a review
criterion. Ignore any that clearly only apply at authoring time (e.g. "use TodoWrite for tasks").
Focus on rules about what the resulting code must look like, contain, or avoid.

When flagging, cite the specific AGENTS.md file and the rule text.
```

### Reviewer Role #2 — Shallow bug scan

```
Your job: read ONLY the diff itself. Do NOT read surrounding context unless you absolutely must.
Look for obvious, big-impact bugs:
- Logic errors, off-by-one, wrong operator, inverted condition
- Null/undefined/None handling
- Resource leaks, missing cleanup
- Race conditions visible from the diff alone

Skip nitpicks. Skip anything a linter would catch. Skip "could be cleaner" complaints. If a bug
isn't a real production risk, don't flag it.
```

### Reviewer Role #3 — Git history context

```
Your job: for the modified lines, run `git --no-pager blame <SCOPE_OR_RANGE> -- <file>` and
`git --no-pager log -p -L <line>,<line>:<file>` to understand WHY the original code was written
that way.

(In PR mode, use the PR's base ref for blame: `gh pr view <PR> --json baseRefName` then
`git fetch origin <base>` and `git --no-pager blame origin/<base> -- <file>`.)

Flag bugs that are visible only in light of that history. Common patterns:
- A fix is being reverted (search the log for the commit that introduced the line being deleted)
- A change reintroduces a previously fixed bug
- A change contradicts a documented invariant from a past commit message

VERIFY EVERY COMMIT YOU CITE, BEFORE YOU EMIT THE FINDING. You are reasoning about artifacts
outside the diff, which makes you the role most able to produce a confident-looking finding that
rests on a commit that does not exist. For each SHA you intend to cite:

  git cat-file -e <sha>^{commit}     # non-zero exit means the SHA does not exist
  git branch -r --contains <sha>     # empty output means it is on no remote branch

Do not emit a finding whose commit fails the existence check. Do not paraphrase it into a vaguer
claim to keep it. A finding whose stated evidence does not exist is not a finding. If you cannot
run verification at all (shallow clone, SHA below fetch depth), you may still report, but you MUST
state "citation unverified" in the finding text so the parent can cap its score.

Report `citation_verified: true` on findings whose SHA you checked and resolved, and
`citation_verified: false` on findings you are reporting unverified.
```

### Reviewer Role #4 — Prior PR comments (read-only)

```
Your job: for each file in the diff (get the list via `<FILES_COMMAND>`),
search for past PRs that touched the same files:

  gh pr list --search "<file-path>" --state merged --limit 10 --json number,title,url

For the top few hits, read review comments:

  gh pr view <pr-number> --comments

Surface any past feedback that applies to the current change. Past reviewers may have already
flagged the same class of issue, or there may be agreed-upon conventions documented in the
discussion.

VERIFY EVERY PR YOU CITE, BEFORE YOU EMIT THE FINDING. `gh pr list --search` is a fuzzy match, so
it is easy to end up citing a PR number that does not exist or does not touch these files. For
each PR you intend to reference:

  gh pr view <N> --json number,title,url    # non-zero exit means it does not exist

Do not emit a finding citing a PR that fails this check, and do not soften it into a vaguer claim
to keep it. Quote the specific past comment you are relying on rather than summarizing "reviewers
previously said". An unquotable comment is one you should not be citing. If you cannot verify at
all (no `gh`, no MCP), you may still report, but you MUST state "citation unverified" in the
finding text.

Report `citation_verified: true` on findings whose PR you checked and resolved, and
`citation_verified: false` on findings you are reporting unverified.

You are READ-ONLY. Do not run `gh pr comment`, `gh pr review`, or any write command. If the
GitHub MCP read tools (find them with ToolSearch "github pull request") are preferred for
listing past PRs and reading their comments; fall back to read-only `gh` only when no MCP is
connected. Only if NEITHER a GitHub MCP nor `gh` is available, respond with "NO_ISSUES_FOUND"
and note the limitation.
```

### Reviewer Role #5 — In-file code comments

```
Your job: read the inline code comments and docstrings in the modified files (not just the diff,
since the whole file is fair game). Surface any place where the change contradicts guidance written in
those comments.

Typical signals:
- A "// IMPORTANT:" or "// WARNING:" comment that the change ignores
- A function-level docstring whose invariant the change violates
- A TODO whose resolution the change implicitly makes urgent

Also flag comment-punctuation violations of AGENTS.md on comments the diff ADDS or edits (low
severity, category "comment guidance"): a comment that glues two independent clauses with ` - `
(space-hyphen-space, used as a stand-in for an em-dash) or with a `:` that splits a claim from
its elaboration. The fix is to split it into two sentences. Flag ONLY the clause-joiner use.
Do NOT flag hyphenated words (`read-only`), CLI flags (`-c`), ranges (`1-10`), label prefixes
(`TODO:`, `NOTE:`, `IMPORTANT:`), ratios/times (`3:1`), or code/paths/URLs. Keep these as
minor suggestions; never let them crowd out a real bug.
```

### Reviewer Role #6 — Database / data-layer scan (conditional)

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

```
Your job: read ONLY the diff itself. Do NOT read surrounding context unless you absolutely must.
Look for obvious, big-impact database / data-layer bugs:

- N+1 query patterns (queries executed inside a loop, including hidden loops via map/filter or
  ORM lazy-loading)
- Queries that filter, join, or sort on columns that are unlikely to be indexed (or whose
  composite-index column order doesn't match the WHERE clause)
- Large or unbounded SELECTs that lack pagination, LIMIT, or streaming
- Repeated identical queries within a single request that should be batched or cached
- Pagination that won't scale to thousands+ of rows: deep OFFSET, missing stable sort key,
  page-size that grows with input, no cycle detection on cursor pagination
- Backfill / batch scripts that aren't properly chunked, throttled, or checkpointed and could
  hammer the database or get stuck halfway through
- Transaction issues that can cause data corruption or inconsistent state: wrong isolation
  level, partial commits, long-held transactions, foreign keys touched without locking,
  deadlock-prone update orderings, missing transaction wrapping multi-write operations

Skip nitpicks. Skip anything a linter would catch. Skip "could use a more idiomatic query
builder" complaints. If a pattern isn't a real production risk, don't flag it.
```

### Reviewer Role #7 — OWASP Top 10 security scan (conditional, but bias hard toward running)

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

```
Your job: read ONLY the diff itself. Do NOT read surrounding context unless you absolutely must.
Look for obvious, big-impact security bugs, scanning against the OWASP Top 10. For each item,
flag concrete instances tied to specific lines in the diff — not generic advice.

1. Injection — unsanitized user input passed to SQL, NoSQL, OS shell, LDAP, XPath, or template
   engines; string concatenation or template literals used to build queries / commands
2. Broken authentication — hard-coded credentials, missing auth checks on protected endpoints,
   session-fixation patterns, predictable tokens, weak password handling
3. Sensitive data exposure — secrets, API tokens, PII, or credentials appearing in logs,
   responses, error messages, URLs, or commits; missing TLS where required
4. XXE / unsafe parser input — XML, YAML, or JSON parsers that load external entities or
   instantiate arbitrary types from untrusted input
5. Broken access control — missing authorization check on a resource, IDOR (user can access
   another user's resource by changing an identifier), privilege escalation paths
6. Security misconfiguration — debug flags left on, permissive CORS, open redirects, admin
   endpoints exposed without auth, dangerous defaults
7. XSS — user-controlled input rendered without escaping in HTML, JS, attribute, or CSS
   contexts; raw-HTML injection sinks (React's unsafe-html prop, Vue's v-html, DOM innerHTML)
   used on untrusted strings
8. Insecure deserialization — unsafe deserializers (Python binary-object loaders, non-safe
   YAML loaders, Ruby `Marshal`, Java `ObjectInputStream`, etc.) invoked on user-controlled data
9. Known-vulnerable dependencies — version downgrades, pinning to CVE-known versions, removing
   security patches
10. Insufficient logging / monitoring — sensitive operations (auth, payments, data writes)
    performed without an audit trail; logs that themselves leak sensitive data

Skip "you should also add 2FA" type recommendations. Skip generic hardening advice. Only flag
concrete vulnerabilities anchored to a specific change.
```

### Reviewer Role #8 — Error-handling review

```
Your job: read ONLY the diff itself. Do NOT read surrounding context unless you absolutely must.
Review error-handling patterns introduced or modified by the diff:

1. Specificity — try/catch blocks should catch specific exception types, not blanket
   `Error` / `Exception` / `catch (e)` / `except:` that obscure the failure mode.
2. Swallowed errors — empty catch blocks; catches that only log-and-continue when the caller
   needs to know; catches that return a default value silently.
3. Propagation — errors that should bubble up to the caller are being absorbed; errors that
   should be handled locally are being thrown all the way out of the layer that owns them.
4. User-facing messages — error responses returned to end users that leak stack traces,
   internal file paths, raw SQL, database schema details, or implementation details.
5. Atomicity — critical operations (payments, state changes, multi-write workflows) that lack
   a rollback / compensation / retry path on partial failure.
6. Context preservation — errors that lose the underlying cause (re-throwing a new error
   without wrapping the original; using `throw new Error(e.message)` instead of `cause: e`).
7. Log level. A failure that needs human attention logged below error level. Error is what
   monitors are keyed to, so a bug logged at `warn` is a bug nobody is paged for.

Skip "you could define a custom error class". Skip pedantic typing-only nits. Only flag
patterns that could cause real bugs, data corruption, security leaks, or unmaintainable
debugging.
```

### Reviewer Role #9 — Test coverage review

```
Your job: review the test coverage for the changes in the diff.

UNLIKE OTHER ROLES, you MAY read surrounding context for this review — specifically, the
existing test files corresponding to the modified production files. Coverage assessment
fundamentally requires it. Use `<FILES_COMMAND>` to get the changed files, then look in the
conventional test-file location for each one (`__tests__/`, `*.test.ts`, `*_test.go`,
`tests/`, etc.) and read what's there.

For each piece of newly-added or modified behavior, evaluate:

1. **Existence** — is there at least one test that exercises this code path? If a non-trivial
   new function or branch has no test, flag it.
2. **Usefulness** — does the test actually verify the behavior, or is it ceremony? Flag tests
   that:
   - Mock the very thing they're claiming to test
   - Only check "doesn't throw" when the function's return value is what matters
   - Assert only on shape (typeof, keys present) when semantic correctness is what's at stake
   - Cover the happy path only when the change explicitly introduces error / edge-case handling
3. **Coverage gaps** — when the diff visibly handles null / empty / boundary / concurrent
   inputs, are those branches tested? Untested defensive code is a smell.
4. **Test quality** — duplicated test bodies that should be parameterized; brittle snapshot
   tests where any unrelated change will churn the snapshot; hardcoded paths/dates/random
   seeds that will go stale.

Skip "add a test for getter X". Skip "100% coverage" goals. Only flag genuine coverage gaps
for non-trivial new behavior, or tests that exist but don't actually test anything.
```

### Reviewer Role #10 — Ticket intent compliance

**Skip this role entirely if `--skip-ticket` was passed.** Do not launch it. Otherwise it is
the 10th concurrent reviewer. Unlike the diff-focused roles, this one reads the change's
tickets and checks the code against them.

```
Your job: check whether the change implements the work its ticket(s) describe. You compare
each ticket's stated intent against what the diff actually does.

1. Collect ticket IDs from the change:
   - Commit messages in scope:
     - Branch mode: git --no-pager log <RANGE> --format='%s%n%b'
     - PR mode:     gh pr view <PR> --json commits
   - PR title and body (PR mode): gh pr view <PR> --json title,body
   Extract Jira-style IDs matching the regex [A-Z][A-Z0-9]+-[0-9]+ and deduplicate them.
   Discard obvious non-ticket matches such as encoding or version strings (e.g. UTF-8).

2. If you find NO ticket IDs, respond with exactly "NO_ISSUES_FOUND" and stop. Do NOT call
   acli. Do NOT trigger any permission prompt. The role is silent when there is nothing to
   check.

3. Read each ticket, preferring acli: acli jira workitem view <ID>
   If acli is not installed, errors because it is not authenticated, or the session runs
   Bash inside a sandbox (sandboxed acli fails even when installed and authenticated, because its
   credentials are unreadable there, so don't misread the failure as an auth problem), fall
   back to a Jira/Atlassian MCP. Search the available tools (e.g. ToolSearch
   "atlassian jira") for an issue-read tool and use it. Pull the title, description, and
   acceptance criteria.

   If NEITHER acli (installed + authenticated) NOR a Jira/Atlassian MCP (connected +
   authenticated) is available, you cannot perform this review. Stop and return exactly:
   TICKET_REVIEW_UNAVAILABLE: <one-line reason>
   This is NOT "no issues". It tells the caller the ticket check did not run, so the caller
   can warn the user.

   If one specific ticket cannot be read while the tooling works (bad ID, no access), mark
   just that ticket as unread and continue with the rest.

4. If a ticket references Datadog (a trace ID, a trace or dashboard URL, or a log query),
   investigate it through the Datadog MCP to understand the actual failure and whether the
   diff addresses it. Load the relevant Datadog skill first, per that MCP's own instructions.
   Keep this bounded to what the ticket explicitly references. Do NOT go fishing.

5. Compare intent against implementation. Flag places where the diff does not implement,
   only partially implements, or contradicts the ticket's stated requirements or acceptance
   criteria. For each finding, set category to "ticket" and ticket_id to the relevant ID.
   Anchor the finding to the file and line(s) where the gap shows up (or the file that
   should have changed but did not).

6. Regardless of whether you found gaps, end your response with one line listing every ticket
   you examined and its status:
   TICKETS_EXAMINED: <ID>=ok, <ID>=gaps, <ID>=unread
   (ok = read, no gaps; gaps = you raised at least one finding for it; unread = tooling worked
   but this ticket could not be read.) Omit this line only in the no-ticket-IDs case (step 2).

ABORT ON DENIAL: Running acli, MCP, or Datadog tools may prompt the user for permission. If
any such permission is denied, immediately stop and return exactly:
TICKET_REVIEW_SKIPPED: access denied
Do not retry and do not work around it. A denial is the user's signal to ignore this
reviewer.

DO NOT MASK FAILURE AS SUCCESS: never return NO_ISSUES_FOUND because you could not read the
tickets. NO_ISSUES_FOUND means "tickets read, code matches" or "no ticket IDs to check".
Inability to read tickets at all is TICKET_REVIEW_UNAVAILABLE (see step 3).

You are READ-ONLY everywhere: Jira (view only), Datadog (read only), GitHub (read only).
Never comment on, transition, or otherwise write to a ticket.
```

### Reviewer Role #11 — Headline-benefit / motivation delivery

This role **always runs** (it needs no ticket tooling, since the PR body / commit messages are
always present). It reasons from the change's STATED PURPOSE down to the live call site, to
catch the case where the diff is locally correct but does not actually deliver the benefit it
claims. This is the gap diff-anchored reviewers miss because a pre-existing, untouched call site sits
outside their lens.

```
Your job: verify the change delivers the benefit its description claims, reasoning from
MOTIVATION, not from the diff.

1. Restate the stated goal in one sentence. Sources: the PR title/body (PR mode:
   `gh pr view <PR> --json title,body`) and the commit messages
   (`git --no-pager log <RANGE> --format='%s%n%b'`). If the goal names an observable effect
   (a metric/tag value, a query result, an email, an event, an endpoint response), note it.

2. Find where that effect must actually manifest at runtime — the live call site, query,
   metric emit, or handler the goal names. `git grep` for it. This site is FREQUENTLY NOT IN
   THE DIFF, and finding it is the whole point of this role.

3. Confirm the diff makes the benefit land THERE. Two failure modes to hunt:
   - The changed code has no live callers (`git grep` the changed symbol -> zero production
     call sites). Do NOT conclude "forward-looking, no change needed" and stop. That is the
     exact near-miss this role exists to prevent. Pull the thread: where does the live
     behavior run today, and does that path still carry the very bug the change set out to fix?
   - The live path is a different, untouched implementation (e.g. an inlined handler) that
     still has the defect the change fixes elsewhere.

   When either holds, flag the LIVE site, even though it is outside the diff. Set the
   finding's category to "motivation" and anchor it to the live call site's file and line(s).

UNLIKE the other roles, the common "discount pre-existing issues / lines the diff didn't
touch" guidance does NOT apply to you: an out-of-diff site is in scope precisely when the
PR's stated benefit fails to land there. Still discount a genuinely unrelated pre-existing
bug that has nothing to do with the stated goal.
```

### Reviewer Role #12 — TypeScript type safety (conditional)

**This role runs ONLY when the diff touches a TypeScript file** (`*.ts`, `*.tsx`, `*.mts`,
`*.cts`). Check with `gh pr diff <PR> --name-only` (PR mode) or
`git --no-pager diff --name-only <RANGE>` (branch mode) before launching it. If the diff has no
TypeScript, skip this role entirely and do not count it against the role total.

It exists because a narrow lens catches what a broad one misses. Role #1 reads AGENTS.md in full
and nominally covers this ground, but casting violations are a specific, mechanical, easily-missed
pattern in a large compliance sweep. The conditional launch is what keeps the extra recall from
costing anything on the many diffs with no TypeScript in them.

```
Your job: audit the TypeScript in this diff for type assertions used in place of narrowing, per
the "TypeScript: narrow with type guards, don't cast" rule in AGENTS.md. Review ONLY TypeScript
files. Ignore every other language in the diff.

Flag these, in the diff's added or modified lines:
- `value as SomeType` used to force a shape the compiler cannot verify.
- `value as unknown as SomeType` — the double cast. Treat this as the highest-severity form. It
  means the compiler actively disagreed and was overruled twice.
- `as any`, including in a type parameter or a return position.
- The non-null assertion `!` on a value that can genuinely be null or undefined at runtime.
  `arr.find(...)!` is the classic case, since `find` returns `T | undefined`.

For each finding, say what the correct narrowing would be, whether a `typeof` / `instanceof` / `in`
check, a user-defined type predicate (`function isX(v: unknown): v is X`), a discriminated union
plus `switch`, or validation at the trust boundary that makes the value arrive already typed.
A finding without a concrete alternative is not actionable; either supply one or drop it.

Do NOT flag these. They are explicitly allowed by the rule:
- `as const`.
- `satisfies`.
- The definite-assignment declaration `let x!: T` — a declaration, not an expression assertion,
  and normal with dependency injection or late initialization.
- Deliberately partial fixtures in test files, where the missing fields are the point.
- A `!` where the value was genuinely validated but the compiler lost the narrowing (across a
  closure or an `await`). Restructuring is better, but this is not the defect the rule targets.

Discount pre-existing casts on lines the diff did not touch. A cast the diff moved or reindented
without otherwise changing is pre-existing, not new. Set every finding's category to "types".
```

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
the role may still be running.

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
3. **Launch a scoring sub-agent for each unique finding in parallel** (one sub-agent per
   finding, all in a single message). **Spawn each scorer on Haiku** (Agent-tool
   `model: haiku`).
   **Collect the scorers the same way you collected the roles.** Record every scorer's `agentId`
   at launch. The launch result carries no score. Take turns and harvest each
   `<task-notification>`, matching its `<task-id>` to a recorded `agentId`, and keep the score
   from its `<result>` body. Keep taking turns until every scorer is accounted for, OR until THREE
   CONSECUTIVE COLLECTING TURNS have brought zero new arrivals. A collecting turn is ONE substantive
   tool call that names the finding it checked on, so re-read the finding and the diff lines it
   cites. Three repeats of the same no-op are not three turns.
   Do NOT start the zero-arrival counter until you have taken at least as many collecting turns as you
   launched scorers, with a floor of five, whether or not anything has arrived. The three zero-arrival
   turns are counted FRESH from the moment the counter arms, so turns taken before arming never count
   toward them. Scorers take minutes, not seconds, so a counter armed at launch measures your own
   polling speed rather than a failure. If you reach the bound and have not yet re-armed the counter,
   re-arm it EXACTLY ONCE and keep collecting. After that, honor the bound.
   A scorer you never hear from does NOT get a self-assigned number. Its finding carries
   `unscored: true` and `confidence: null`, exactly as it would if no scorer had been spawned. If
   you end your turn with scorers outstanding you are finalized and they are lost, unless you are
   finalizing at the bound, once you have already used your single re-arm, with those findings
   marked `unscored`. Do not stop to report progress otherwise.
   A notification that arrives AFTER the bound is still that scorer's score. Fold it into its
   finding and clear that finding's `unscored` mark. The scoring accounting is final only when you
   emit your output.
   Scoring one finding against the rubric is a small, structured judgment
   with the diff and AGENTS.md handed in, not open-ended reasoning; Haiku is ~15–20x cheaper
   than Opus for it. Give each scorer:
   - The finding (file, line, severity, description, suggested fix)
   - The path of every AGENTS.md / CLAUDE.md file referenced by any reviewer that raised it
   - The diff for the relevant lines
   - The `role_agreement` count

   **The scoring stage is MANDATORY and is not yours to perform.** Confidence is a second-stage
   judgment by a different model than the one that proposed the finding. That two-stage split is
   the whole reason a confidence number means anything here.

   - **Never self-assign a confidence value.** If you find yourself writing a score without
     having spawned a scorer for that finding, you have collapsed the two stages into one model
     grading its own work, and the number is worthless.
   - **A reviewer-authored confidence value is not a score.** Roles emit `severity`, which is
     theirs to judge. If a role also emits a confidence number, DISCARD it and score the finding
     properly. Never pass a role's own number through as the score.
   - **Count what you spawned.** Record `scorers_spawned` and compare it to the number of unique
     findings after dedup. They must be equal.
   - **A finding with no scorer is `unscored`, not confident.** Set its confidence to `null`,
     mark it `unscored`, and treat it as BELOW every caller threshold. It must never be posted
     or reported as a finding. List it separately as unscored so the gap is visible.
   - If `scorers_spawned` is 0 while unique findings exist, the run is **degraded, not clean**:
     emit `scoring.complete: false`, report every finding as unscored, and say plainly that no
     confidence filtering happened.

   Skipping this stage is not a shortcut, it is a correctness failure. It has already produced a
   fabricated finding that scored 100 and survived the filter, because the model that invented the
   precedent was also the model that graded it.

Each scorer returns a number 0–100 with this rubric:

| Score | Meaning                                                                                                                                              |
| ----- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0     | False positive that doesn't survive light scrutiny, or a pre-existing issue                                                                          |
| 25    | Somewhat confident — might be real, might be false; couldn't verify either way. For AGENTS.md issues: the cited rule doesn't actually call this out. |
| 50    | Moderately confident — the mechanism is real, but residual uncertainty remains about whether it truly applies                                        |
| 75    | Highly confident — verified the code definitively does this; OR a provable convention / AGENTS.md violation                                          |
| 100   | Absolutely certain — evidence directly confirms it                                                                                                   |

**Hard cap: a finding whose citation is unverified cannot score above 60.** Roles #3 and #4 verify
their own commit and PR citations at emit time and report `citation_verified` (see their prompts),
so this cap holds even if that verification could not run. It is a backstop, not the primary
enforcement. The roles drop fabricated citations before they ever reach you.

- `citation_verified: true` — score normally against the rubric above.
- `citation_verified: false` — cap at **60** and preserve it as a lead. Never resolve an unverified
  citation upward. The cap is not what keeps such a finding out of a caller's output. Callers
  exclude `citation_verified: false` outright, `pr-review` from posting and `review-and-fix` from
  fixing, whatever the score. Do not describe the cap as sitting below a caller's threshold. It does
  not. `pr-review` filters at `confidence >= 60`, which 60 satisfies.
- A finding that cites nothing has nothing to verify; score it normally.

If a finding cites a commit or PR and carries no `citation_verified` field at all, treat it as
`false` and cap it. An absent field means the role did not verify.

**Calibration — confidence is the TRUTH axis, not current impact.** Confidence answers "how
sure are we this finding is real and valid," NOT "how big is the blast radius today." Two
consequences:
- A finding whose truth is *provable and binary* — a convention or safety violation (e.g. a
  non-`CONCURRENTLY` index build on a pre-existing table, checkable against repo convention) —
  is scored by provability ALONE. Do NOT discount it because the current blast radius is
  small: an empty or feature-gated table, low live traffic, or a cheap fix. That low impact
  belongs in the **severity** field (`minor` / `suggestion`), not in the confidence number.
  Deflating a provably-true finding by today's table size is the calibration error to avoid.
  It buries real, cheap-to-fix findings below the caller's threshold.
- This is NOT a blanket "score every real-ish finding high." For a latent *bug*, confidence
  still reflects whether it is genuinely a defect and whether its path is reachable at all. A
  rare, marginal conjunction that may not even constitute a real defect legitimately sits near
  or below the line. Reachability (can the path EVER execute) is a truth question and bounds
  confidence; frequency (how OFTEN, how big the blast radius) is impact and does not.

When scoring, `role_agreement` and the diff are inputs to *truth*, not impact. More roles
raising the same provable violation supports a higher confidence, never a lower one.

**Ticket-category findings** (from Role #10) are scored on the same 0–100 scale. The question
is "how sure are we the code diverges from what the ticket requires?":

- 100 — the ticket explicitly requires X and the diff demonstrably does not-X
- 75 — the code clearly does not do what the ticket explicitly requires
- 50 — a gap is plausible but the ticket's requirement is ambiguous, or the divergence may be minor
- 25 — could not confirm the ticket actually requires this (likely a misread of the ticket)
- 0 — false positive: the ticket does not require this, or the diff already satisfies it

Score the divergence, not the importance of the ticket.

**Motivation-category findings** (from Role #11) are scored on "how sure are we the PR's
stated benefit fails to land at the live call site?" Do NOT score one of these as 0 merely
because the cited line is outside the diff. The PR's stated purpose puts that site in scope:

- 100 — the stated goal demonstrably does not occur (the live call site still has the bug, or
  the changed code has zero live callers and the real path is untouched)
- 75 — strong evidence the benefit does not land where the goal says it should
- 50 — plausible the benefit is undelivered, but the goal or the live call site is ambiguous
- 25 — could not confirm the live call site; may be a misread of the goal
- 0 — the benefit does land (the diff reaches the real call site), or the "goal" was misread

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

Return this exact JSON shape:

```json
{
  "scope": "<SCOPE_DESCRIPTION>",
  "mode": "pr" | "branch",
  "pr_head_sha": "<sha or null>",
  "raw": <true if --raw was set, else false>,
  "summary": "<short summary of what changed>",
  "findings": [
    {
      "id": "<file:line-range>",
      "title": "<one-line description>",
      "file": "<path>",
      "line_range": "<L<start>-L<end>>",
      "category": "<bug | AGENTS.md | history | prior PR | comment guidance | db | security | error-handling | test coverage | motivation | ticket | types>",
      "ticket_id": "<JIRA-ID this gap traces to, or null for non-ticket findings>",
      "description": "<full text>",
      "suggested_fix": "<text or code snippet>",
      "confidence": <0..100, or null when unscored>,
      "role_agreement": <1..12>,
      "citation_verified": <true | false | null>,
      "unscored": <true when no scorer produced this finding's confidence; omit or false otherwise>,
      "permalink": "<github blob URL with full SHA, if available; null otherwise>"
    }
  ],
  "roles_launched": [<role numbers actually spawned, after gates and any --roles subset>],
  "roles_missing": [
    {
      "role": <number>,
      "reason": "<empty response | unparseable | errored | skipped by harness | no notification received>"
    }
  ],
  "coverage": "complete | partial",
  "scoring": {
    "unique_findings": <count after pre-score dedup>,
    "scorers_spawned": <count of scoring sub-agents actually spawned>,
    "complete": <true when scorers_spawned == unique_findings, else false>
  },
  "tickets_examined": [
    { "id": "<JIRA-ID>", "gaps": <count of surviving ticket findings for this id>, "status": "ok | gaps | unread" }
  ],
  "ticket_review": { "status": "ran | skipped | denied | unavailable", "note": "<reason when denied/unavailable, else null>" },
  "skipped_reason": "<if the skill bailed out early, why; otherwise omit>"
}
```

`roles_launched`, `roles_missing`, and `coverage` are **required.** Emit them even when
`roles_missing` is empty and `coverage` is `"complete"`. `coverage` is `"partial"` whenever
`roles_missing` is non-empty. A caller that sees `"partial"` knows not to read a short findings
list as a clean bill of health. Callers aggregating several instances (`pr-review`,
`review-and-fix`) rely on these fields to avoid asserting coverage nobody delivered.

`scoring` is **required**, and it exists so a caller can tell a filtered result from an unfiltered
one. `scoring.complete: false` means the confidence numbers in `findings` did not all come from the
two-stage process and must not be trusted as a filter. A caller seeing it should treat the run as
leads, not conclusions. Never omit the block to make a run look clean. Any finding carrying
`unscored: true` has `confidence: null` and sits below every threshold by construction.

`citation_verified` is `true` when the finding cites a commit / PR / branch that was checked and
resolved, `false` when it cites one that could not be verified, and `null` when there is nothing to
verify. Roles #3 and #4 set it at emit time, so it survives a skipped scoring stage. A `false`
caps the finding at 60 regardless of any score.

Populate `ticket_review` from Role #10's response: a findings list or `NO_ISSUES_FOUND` ->
`{ "status": "ran", "note": null }`; `TICKET_REVIEW_SKIPPED: access denied` ->
`{ "status": "denied", "note": "access denied" }`; `TICKET_REVIEW_UNAVAILABLE: <r>` ->
`{ "status": "unavailable", "note": "<r>" }`; `--skip-ticket` passed (role not launched) ->
`{ "status": "skipped", "note": null }`.

Populate `tickets_examined` from Role #10's `TICKETS_EXAMINED:` line: one entry per `<id>=<status>`,
with `gaps` = number of that ticket's surviving findings. When `--skip-ticket` was passed, Role #10
returned `NO_ISSUES_FOUND` with no `TICKETS_EXAMINED:` line, or `ticket_review.status` is `denied`/
`unavailable`, set `tickets_examined` to `[]`.

### If invoked directly by the user

Render a chat report:

```
# In-Depth Review — <SCOPE_DESCRIPTION>

**Coverage:** complete | partial (when partial, list every missing role number and the lenses not
applied). Never omit this line. Its absence reads as complete coverage.
**Unscored:** N findings had no scorer (omit this line when N is 0)

**Findings (confidence >= 70):** N

1. <title> &nbsp;`[severity, roles N, confidence X]`
   <file>:<line_range>
   <description>
   *Suggested fix:* <fix>

2. ...

**Tickets examined:** <ID> ✅ · <ID> ⚠️ <N> gaps · <ID> ❓ unread
```

Omit the **Tickets examined** line when no ticket IDs were found in the change or when
`--skip-ticket` was passed. When `ticket_review.status` is `unavailable`, replace it with a
prominent warning: `⚠️ **Ticket review NOT performed** — <note>. Install/authenticate acli or
the Atlassian MCP, or re-run with --skip-ticket.` When the status is `denied`, show:
`ℹ️ Ticket review skipped — access denied.`

If zero findings survive, coverage is complete, AND no finding was left `unscored`: report
`✅ No issues found at confidence >= 70.`
If zero findings survive but any role is missing, you MUST NOT report a bare green check. Replace
the ✅ line with `⚠️ No issues found, but coverage was PARTIAL.` The Coverage line above already
names every missing role and the lenses not applied; do not repeat that list here.
If zero findings survive, coverage is complete, but any finding was left `unscored`, you MUST NOT
report a bare green check either. Replace the ✅ line with `⚠️ No issues found, but N finding(s)
were never scored.` The Unscored line above already states the count; do not repeat it here.

If `--raw` was set, the chat report header changes to `**All scored findings (no filter):**`
and the threshold note is dropped.

## Constraints

- **No GitHub writes, ever.** Prefer the equally read-only GitHub MCP PR-read tools; read-only
  `gh` calls (list, view, diff, search) are the fallback when no MCP is connected.
  Sub-agents that try to issue a write should be aborted and surfaced to the caller.
- **9 to 12 parallel reviewers per pass, or 8 to 11 with `--skip-ticket`.** Eight are
  unconditional (#1, #2, #3, #4, #5, #8, #9, #11). #10 runs unless `--skip-ticket` drops it, which
  is what lowers the floor to eight. Three are gated on the diff: #6 data-layer, #7 security, #12
  TypeScript. See the Step 1 gate table. A caller may run a smaller subset via
  `--roles` (Step 0/1) for iterative reruns, and that is the ONLY reason to drop a role beyond the
  gates. Never serialize, and never drop an ungated role for speed on a standalone or first-pass
  review. The role specialization is the point.
- **The three gated roles are conditional, not optional.** A gate answers "is there anything in
  this diff for this lens to read", and it is a cost guard, not a quality dial. Skipping a
  language- or domain-specific lens on a diff without that language or domain is free; skipping
  it on a diff that HAS one is a missed finding. Never resolve a gate as false to save an agent.
  When a gate is genuinely ambiguous, run the role.
- **#7's gate is deliberately narrower than #6's and #12's.** Data-layer code and TypeScript files
  are mechanically identifiable; security surface is not. Skip #7 only for a diff with no
  executable code, no dependency or lockfile change, and no config/secret/CI change. An empty
  result from #7 means "looked, found no vulnerability" — a successful review, NOT evidence that
  the role should have been gated off. Do not widen this gate on the strength of empty results.
- **Role #10 is read-only and abortable.** It may use `acli jira workitem view` (Jira read)
  and read-only Datadog MCP tools — nothing else. On any denied permission it returns
  `TICKET_REVIEW_SKIPPED: access denied` and stops. This is the user's "ignore this reviewer"
  control. Role #10 never writes to Jira, Datadog, or GitHub.
- **Single pass per invocation** — no looping inside this skill. Callers that want iteration
  (like `review-and-fix`) loop externally.
- **Threshold default is `< 70` discard.** `--raw` bypasses the filter for callers that apply
  their own threshold.
- **Scoring is per-finding and parallel** — one scorer per unique finding, all launched in a
  single message. Never score serially.
- **Model policy (cost):** reviewers run on **Sonnet** (`model: sonnet`), scorers on **Haiku**
  (`model: haiku`). Never let either inherit the session model (it may be Opus / a `[1m]`
  variant — far pricier and unnecessary for these bounded tasks). Recall + provability come
  from role specialization and the agreement count, not from a bigger model per finder.
- **No fix application.** This skill reports; consumers fix.
