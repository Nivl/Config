---
name: in-depth-review
description: >
  Performs one in-depth multi-perspective code review of either a pull request or a commit
  range. Launches up to TEN specialized parallel reviewer roles (AGENTS.md compliance, shallow bug
  scan, git history context, prior PR comments, in-file code comments, database / data-layer,
  OWASP Top 10 security, error handling, test coverage, and — unless `--skip-ticket` is
  passed — ticket intent compliance), then scores each finding 0–100 for confidence, filters
  anything below 70, and deduplicates. Returns the surviving findings.
  Never writes to GitHub.
  Used as one of two parallel review primitives (the other being `gh-style-review`) by
  `review-and-fix` (which spawns 3 of each per iteration and adds a fix/commit loop) and
  `pr-review` (which spawns 5 of each, merges them into one flat pool, and posts a single
  PR review with inline + global comments).
  Use this skill when the user asks for "in-depth review", "deep review", "thorough review",
  "code review without fixing", or invokes it directly to get a one-shot review report.
---

# In-Depth Review

This skill performs ONE complete review pass over a target scope (a PR or a commit range)
using up to ten specialized reviewer roles, then scores, filters, and deduplicates findings. It
returns the result — it does NOT post anywhere, fix anything, or loop.

The multi-role specialization gives **cross-domain coverage** (style/standards, raw bugs, history,
prior PR feedback, in-file guidance, DB, security, error handling, tests, ticket intent). Within-role
triangulation (running the same role multiple times) is the **caller's** responsibility, not
this skill's — `review-and-fix` runs 3 of these per iteration; `pr-review` runs 5.

## Argument

Accepts a positional arg plus optional modifier flags (`--raw`, `--skip-ticket`), in any order. Auto-detects mode from the positional arg:

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

Example invocations:

```
/in-depth-review 1234              # PR mode, default filter (≥ 70)
/in-depth-review #1234 --raw       # PR mode, no filter (return all scored)
/in-depth-review 1234 --skip-ticket # PR mode, skip the ticket-intent role
/in-depth-review origin/main..HEAD # branch mode, default filter
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

## GitHub access (`gh` with GitHub-MCP fallback)

Every GitHub call below is written as a `gh` command. **If the `gh` binary is unavailable or
unauthenticated in this environment, fall back to the GitHub MCP server** — the same way Role
#10 falls back from `acli` to the Atlassian MCP. Discover the MCP tools with
`ToolSearch "github pull request"` and call the operation matching the `gh` call:

| `gh` call used here | GitHub MCP equivalent (confirm exact name via ToolSearch) |
|---|---|
| `gh pr view <N> --json …` | get pull request (metadata, title/body/baseRefName/commits) |
| `gh pr diff <N>` | get pull request diff |
| `gh pr diff <N> --name-only` | get pull request files (changed files) |
| `gh pr list --search …` | list / search pull requests |
| `gh pr view <N> --comments` | get pull request review comments + issue comments |

Prefer `gh` when present; only fall back when the binary is missing or auth fails. This is a
**read-only** fallback — read calls map to read tools, so the "never writes to GitHub"
constraint is unchanged. If NEITHER `gh` nor a GitHub MCP is available, PR mode cannot proceed:
abort Step 0 with that reason. Local `git` calls (`git diff`, `git log`, `git blame`,
`git rev-list`) need no `gh` and are unaffected — branch mode works without GitHub entirely.

## Step 0: Resolve scope

1. Parse the argument. First split it on whitespace into tokens; classify each token, then apply mode detection to the lone non-flag token:
   - Matches `^#?[0-9]+$` or a GitHub PR URL → **PR mode**; `<PR>` = the number.
   - Matches `^--raw$` → flag (defer until Step 4).
   - Matches `^--skip-ticket$` → flag; when set, Role #10 is omitted in Step 1.
   - Anything else → **branch mode**; `<RANGE>` = the arg.
   - If no positional arg: branch mode with `<RANGE>` = `origin/<default-branch>..HEAD`.

2. **PR mode preflight:**

   ```
   gh pr view <PR> --json number,state,isDraft,headRefOid,url
   ```

   If exit non-zero or `state != "OPEN"`, abort with a clear reason. **Draft PRs are
   accepted** — reviewing early is a valid workflow. Store `<PR_HEAD_SHA>` for downstream
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
**10** when ticket review is active (the default), or **9** when `--skip-ticket` was passed
(omit Role #10). Sequential launches defeat the purpose of this design — never serialize.

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
- Issues a linter / typechecker / compiler would catch (assume CI runs these — don't run them)
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

You are one of the reviewers running concurrently (9, or 10 when ticket review is active).
Do NOT coordinate with the others.

IMPORTANT: Do not run `gh pr comment`, `gh pr review`, `gh pr edit`, or any command that
writes to GitHub. Read-only gh commands (gh pr list / view / diff / search) are permitted
only if your role below explicitly requires them. If the `gh` binary is unavailable or
unauthenticated, use the GitHub MCP read tools instead (find them with
ToolSearch "github pull request") — they are equally read-only; never use a write tool.
```

### Reviewer Role #1 — AGENTS.md compliance

```
Your job: audit the changes for compliance with the relevant AGENTS.md / CLAUDE.md files
(root + any sub-project AGENTS.md whose directory the diff touches). Read each one in full.

Note that AGENTS.md is guidance for Claude as it WRITES code, so not every rule is a review
criterion — ignore any that clearly only apply at authoring time (e.g. "use TodoWrite for tasks").
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

You are READ-ONLY. Do not run `gh pr comment`, `gh pr review`, or any write command. If the
`gh` binary is not available or unauthenticated, fall back to the GitHub MCP read tools (find
them with ToolSearch "github pull request") to list past PRs and read their comments. Only if
NEITHER `gh` nor a GitHub MCP is available, respond with "NO_ISSUES_FOUND" and note the
limitation.
```

### Reviewer Role #5 — In-file code comments

```
Your job: read the inline code comments and docstrings in the modified files (not just the diff —
the whole file is fair game). Surface any place where the change contradicts guidance written in
those comments.

Typical signals:
- A "// IMPORTANT:" or "// WARNING:" comment that the change ignores
- A function-level docstring whose invariant the change violates
- A TODO whose resolution the change implicitly makes urgent
```

### Reviewer Role #6 — Database / data-layer scan

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

### Reviewer Role #7 — OWASP Top 10 security scan

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

**Skip this role entirely if `--skip-ticket` was passed** — do not launch it. Otherwise it is
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
   Bash inside a sandbox (sandboxed acli fails even when installed and authenticated — its
   credentials are unreadable there, so don't misread the failure as an auth problem), fall
   back to a Jira/Atlassian MCP — search the available tools (e.g. ToolSearch
   "atlassian jira") for an issue-read tool and use it. Pull the title, description, and
   acceptance criteria.

   If NEITHER acli (installed + authenticated) NOR a Jira/Atlassian MCP (connected +
   authenticated) is available, you cannot perform this review. Stop and return exactly:
   TICKET_REVIEW_UNAVAILABLE: <one-line reason>
   This is NOT "no issues" — it tells the caller the ticket check did not run, so the caller
   can warn the user.

   If one specific ticket cannot be read while the tooling works (bad ID, no access), mark
   just that ticket as unread and continue with the rest.

4. If a ticket references Datadog — a trace ID, a trace or dashboard URL, or a log query —
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

## Step 2: Confidence scoring

After all reviewers return:

1. Before pooling, scan every reviewer response: if a response begins with
   `TICKET_REVIEW_UNAVAILABLE:` or `TICKET_REVIEW_SKIPPED:`, set it aside to populate
   `ticket_review` (Step 4) — it is NOT a finding, so do not pool it or send it to a scorer.
   Then pool every remaining non-clean response (each is a list of findings). `NO_ISSUES_FOUND`
   from any role is an empty (clean) response.
2. **Pre-score deduplication** — group findings that look like duplicates (same file +
   overlapping line range + substantially the same problem). For each group, keep one canonical
   entry and record the **agreement count** (how many of the role outputs raised it).
   Highest severity in the group wins.
3. **Launch a scoring sub-agent for each unique finding in parallel** (one Skill call per
   finding, all in a single message). Give each scorer:
   - The finding (file, line, severity, description, suggested fix)
   - The path of every AGENTS.md / CLAUDE.md file referenced by any reviewer that raised it
   - The diff for the relevant lines
   - The agreement count

Each scorer returns a number 0–100 with this rubric:

| Score | Meaning                                                                                                                                              |
| ----- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0     | False positive that doesn't survive light scrutiny, or a pre-existing issue                                                                          |
| 25    | Somewhat confident — might be real, might be false; couldn't verify either way. For AGENTS.md issues: the cited rule doesn't actually call this out. |
| 50    | Moderately confident — verified real, but might be a nitpick or rarely hits in practice                                                              |
| 75    | Highly confident — verified, will likely be hit in practice; OR an explicit AGENTS.md violation                                                      |
| 100   | Absolutely certain — evidence directly confirms it, happens frequently                                                                               |

**Ticket-category findings** (from Role #10) are scored on the same 0–100 scale. The question
is "how sure are we the code diverges from what the ticket requires?":

- 100 — the ticket explicitly requires X and the diff demonstrably does not-X
- 75 — the code clearly does not do what the ticket explicitly requires
- 50 — a gap is plausible but the ticket's requirement is ambiguous, or the divergence may be minor
- 25 — could not confirm the ticket actually requires this (likely a misread of the ticket)
- 0 — false positive: the ticket does not require this, or the diff already satisfies it

Score the divergence, not the importance of the ticket.

## Step 3: Filter and dedup

1. **Filter** — unless invoked with `--raw`, discard findings with `confidence < 70`. The
   70 threshold is slightly more permissive than upstream `code-review`'s default of 80 —
   intentional, because the multi-role rubric here is wider than upstream's 5 roles and the extra
   roles (DB, security, error-handling, test coverage) tend to score in the 70-79 band even
   when they're genuinely useful.

   When `--raw` is in effect, **skip this step** and pass all findings (0–100) through to
   Step 4. Callers that apply their own threshold (`pr-review` uses 60, `review-and-fix`
   uses 50) use this mode.

2. **Post-score dedup** — re-check for duplicates one more time, in case scoring revealed two
   findings that scored identically and pointed at the same locus. Keep the highest severity,
   max confidence, union of categories, and max agreement.

## Step 4: Return / report

Order surviving findings by:

1. Severity descending (critical → major → minor → suggestion)
2. Agreement count descending (more roles raised it → higher priority)
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
      "category": "<bug | AGENTS.md | history | prior PR | comment guidance | db | security | error-handling | test coverage | ticket>",
      "ticket_id": "<JIRA-ID this gap traces to, or null for non-ticket findings>",
      "description": "<full text>",
      "suggested_fix": "<text or code snippet>",
      "confidence": <0..100>,
      "agreement": <1..10>,
      "permalink": "<github blob URL with full SHA, if available; null otherwise>"
    }
  ],
  "tickets_examined": [
    { "id": "<JIRA-ID>", "gaps": <count of surviving ticket findings for this id>, "status": "ok | gaps | unread" }
  ],
  "ticket_review": { "status": "ran | skipped | denied | unavailable", "note": "<reason when denied/unavailable, else null>" },
  "skipped_reason": "<if the skill bailed out early, why; otherwise omit>"
}
```

Populate `ticket_review` from Role #10's response: a findings list or `NO_ISSUES_FOUND` →
`{ "status": "ran", "note": null }`; `TICKET_REVIEW_SKIPPED: access denied` →
`{ "status": "denied", "note": "access denied" }`; `TICKET_REVIEW_UNAVAILABLE: <r>` →
`{ "status": "unavailable", "note": "<r>" }`; `--skip-ticket` passed (role not launched) →
`{ "status": "skipped", "note": null }`.

Populate `tickets_examined` from Role #10's `TICKETS_EXAMINED:` line: one entry per `<id>=<status>`,
with `gaps` = number of that ticket's surviving findings. When `--skip-ticket` was passed, Role #10
returned `NO_ISSUES_FOUND` with no `TICKETS_EXAMINED:` line, or `ticket_review.status` is `denied`/
`unavailable`, set `tickets_examined` to `[]`.

### If invoked directly by the user

Render a chat report:

```
# In-Depth Review — <SCOPE_DESCRIPTION>

**Findings (confidence ≥ 70):** N

1. <title> &nbsp;`[severity, agreement N, confidence X]`
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

If zero findings survive: report `✅ No issues found at confidence ≥ 70.`

If `--raw` was set, the chat report header changes to `**All scored findings (no filter):**`
and the threshold note is dropped.

## Constraints

- **No GitHub writes, ever.** Only read-only `gh` calls are permitted (list, view, diff,
  search) — or, when `gh` is unavailable, the equally read-only GitHub MCP PR-read tools.
  Sub-agents that try to issue a write should be aborted and surfaced to the caller.
- **9 or 10 parallel reviewers per pass** — 10 by default (the 10th is ticket intent
  compliance), 9 when `--skip-ticket` is passed. Never serialize, never skip a role for
  speed. The role specialization is the point.
- **Role #10 is read-only and abortable.** It may use `acli jira workitem view` (Jira read)
  and read-only Datadog MCP tools — nothing else. On any denied permission it returns
  `TICKET_REVIEW_SKIPPED: access denied` and stops. This is the user's "ignore this reviewer"
  control — Role #10 never writes to Jira, Datadog, or GitHub.
- **Single pass per invocation** — no looping inside this skill. Callers that want iteration
  (like `review-and-fix`) loop externally.
- **Threshold default is `< 70` discard.** `--raw` bypasses the filter for callers that apply
  their own threshold.
- **Scoring is per-finding and parallel** — one scorer per unique finding, all launched in a
  single message. Never score serially.
- **No fix application.** This skill reports; consumers fix.
