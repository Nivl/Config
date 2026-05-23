---
name: in-depth-review
description: >
  Performs one in-depth multi-perspective code review of either a pull request or a commit
  range. Launches NINE specialized parallel reviewer roles (AGENTS.md compliance, shallow bug
  scan, git history context, prior PR comments, in-file code comments, database / data-layer,
  OWASP Top 10 security, error handling, test coverage), then scores each finding 0–100 for
  confidence, filters anything below 70, and deduplicates. Returns the surviving findings.
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
using 9 specialized reviewer roles, then scores, filters, and deduplicates findings. It
returns the result — it does NOT post anywhere, fix anything, or loop.

The 9-role specialization gives **cross-domain coverage** (style/standards, raw bugs, history,
prior PR feedback, in-file guidance, DB, security, error handling, tests). Within-role
triangulation (running the same role multiple times) is the **caller's** responsibility, not
this skill's — `review-and-fix` runs 3 of these per iteration; `pr-review` runs 5.

## Argument

Single positional arg. Auto-detects mode:

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

Example invocations:

```
/in-depth-review 1234              # PR mode, default filter (≥ 70)
/in-depth-review #1234 --raw       # PR mode, no filter (return all scored)
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

## Step 0: Resolve scope

1. Parse the argument:
   - Matches `^#?[0-9]+$` or a GitHub PR URL → **PR mode**; `<PR>` = the number.
   - Matches `^--raw$` → flag (defer until Step 4).
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
   - `<DIFF_COMMAND>` — `gh pr diff <PR>` (PR mode) or `git diff --no-pager <RANGE>` (branch)
   - `<FILES_COMMAND>` — `gh pr diff <PR> --name-only` or `git diff --name-only <RANGE>`

## Step 1: Launch the 9 specialized reviewers in parallel

Spawn **9 sub-agents in a single message** (9 concurrent tool-use blocks). Sequential
launches defeat the purpose of this design — never serialize.

### Common reviewer prompt fragment

Every reviewer prompt starts with this block:

```
Scope: <SCOPE_DESCRIPTION>

Run `<DIFF_COMMAND>` to see all changes.
Run `<FILES_COMMAND>` if you need just the list of changed files.

[PR mode only] Also run `gh pr view <PR> --json title,body,baseRefName` for PR context.
[Branch mode] Run `git log --no-pager <RANGE> --oneline` for commit context.

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

You are one of 9 reviewers running concurrently. Do NOT coordinate with the others.

IMPORTANT: Do not run `gh pr comment`, `gh pr review`, `gh pr edit`, or any command that
writes to GitHub. Read-only gh commands (gh pr list / view / diff / search) are permitted
only if your role below explicitly requires them.
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
Your job: for the modified lines, run `git blame --no-pager <SCOPE_OR_RANGE> -- <file>` and
`git log --no-pager -p -L <line>,<line>:<file>` to understand WHY the original code was written
that way.

(In PR mode, use the PR's base ref for blame: `gh pr view <PR> --json baseRefName` then
`git fetch origin <base>` and `git blame origin/<base> -- <file>`.)

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

You are READ-ONLY. Do not run `gh pr comment`, `gh pr review`, or any write command. If gh is
not available or unauthenticated, respond with "NO_ISSUES_FOUND" and note the limitation.
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

## Step 2: Confidence scoring

After all 9 reviewers return:

1. Pool every non-clean response. Each response is a list of findings.
2. **Pre-score deduplication** — group findings that look like duplicates (same file +
   overlapping line range + substantially the same problem). For each group, keep one canonical
   entry and record the **agreement count** (how many of the 9 role outputs raised it).
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

## Step 3: Filter and dedup

1. **Filter** — unless invoked with `--raw`, discard findings with `confidence < 70`. The
   70 threshold is slightly more permissive than upstream `code-review`'s default of 80 —
   intentional, because the 9-role rubric here is wider than upstream's 5 roles and the extra
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
      "category": "<bug | AGENTS.md | history | prior PR | comment guidance | db | security | error-handling | test coverage>",
      "description": "<full text>",
      "suggested_fix": "<text or code snippet>",
      "confidence": <0..100>,
      "agreement": <1..9>,
      "permalink": "<github blob URL with full SHA, if available; null otherwise>"
    }
  ],
  "skipped_reason": "<if the skill bailed out early, why; otherwise omit>"
}
```

### If invoked directly by the user

Render a chat report:

```
# In-Depth Review — <SCOPE_DESCRIPTION>

**Findings (confidence ≥ 70):** N

1. <title> &nbsp;`[severity, agreement N/9, confidence X]`
   <file>:<line_range>
   <description>
   *Suggested fix:* <fix>

2. ...
```

If zero findings survive: report `✅ No issues found at confidence ≥ 70.`

If `--raw` was set, the chat report header changes to `**All scored findings (no filter):**`
and the threshold note is dropped.

## Constraints

- **No GitHub writes, ever.** Only read-only `gh` calls are permitted (list, view, diff,
  search). Sub-agents that try to issue a write should be aborted and surfaced to the caller.
- **9 parallel reviewers per pass** — never serialize, never skip a role for speed. The role
  specialization is the point.
- **Single pass per invocation** — no looping inside this skill. Callers that want iteration
  (like `review-and-fix`) loop externally.
- **Threshold default is `< 70` discard.** `--raw` bypasses the filter for callers that apply
  their own threshold.
- **Scoring is per-finding and parallel** — one scorer per unique finding, all launched in a
  single message. Never score serially.
- **No fix application.** This skill reports; consumers fix.
