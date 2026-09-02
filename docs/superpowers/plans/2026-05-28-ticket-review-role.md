# Ticket-aware Review Role Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a "ticket intent compliance" reviewer (Role #10) to `in-depth-review` that reads a change's linked Jira tickets, optionally investigates Datadog references, and flags where the code does not implement the ticket — then plumb a `--skip-ticket` flag through the orchestrators.

**Architecture:** The check is an ordinary 10th role inside `in-depth-review`, so it runs in every instance and triangulates like roles 1–9 (1 standalone, 3 in `review-and-fix`, 5 in `pr-review`). Findings use the existing schema with a new `ticket` category and `ticket_id` field. Each reviewer aborts on any denied permission, which (via sub-agent isolation) is the "ignore this reviewer" control. `--skip-ticket` turns the role off everywhere.

**Tech Stack:** Markdown skill prompts (`SKILL.md`), `acli jira workitem view` (Jira read), Datadog MCP (read), `gh`/`git` (read). No runtime code — verification is structural (grep + JSON/YAML parse).

**Spec:** `docs/superpowers/specs/2026-05-28-ticket-review-role-design.md`

---

## Files

- Modify: `shared_config/.claude/skills/in-depth-review/SKILL.md` — add Role #10, the `--skip-ticket` flag, scoring note, schema fields, report line, constraints.
- Modify: `shared_config/.claude/skills/pr-review/SKILL.md` — accept `--skip-ticket`, forward to the 5 in-depth sub-agents, render `ticket_id` + "Tickets examined".
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md` — accept `--skip-ticket`, forward to the 3 in-depth sub-agents, route `ticket` findings to the ask-the-user path.
- Untouched: `shared_config/.claude/skills/gh-style-review/SKILL.md`.

After all edits, sync to `~/.claude` with `melvin-config claude sync`.

---

## Task 1: in-depth-review — document the `--skip-ticket` flag

**Files:**
- Modify: `shared_config/.claude/skills/in-depth-review/SKILL.md` (front-matter, "Argument", "Modifier flags", Step 0)

- [ ] **Step 1: Update the front-matter description to mention the optional role**

Replace this exact text in the `description:` block:

```
  Launches NINE specialized parallel reviewer roles (AGENTS.md compliance, shallow bug
  scan, git history context, prior PR comments, in-file code comments, database / data-layer,
  OWASP Top 10 security, error handling, test coverage), then scores each finding 0–100 for
  confidence, filters anything below 70, and deduplicates. Returns the surviving findings.
```

with:

```
  Launches up to TEN specialized parallel reviewer roles (AGENTS.md compliance, shallow bug
  scan, git history context, prior PR comments, in-file code comments, database / data-layer,
  OWASP Top 10 security, error handling, test coverage, and — unless `--skip-ticket` is
  passed — ticket intent compliance), then scores each finding 0–100 for confidence, filters
  anything below 70, and deduplicates. Returns the surviving findings.
```

- [ ] **Step 2: Add `--skip-ticket` to the "Modifier flags" section**

After the `--raw` bullet (which ends with "pass this flag."), insert:

```
- `--skip-ticket` — disable Reviewer Role #10 (ticket intent compliance). By default that
  role runs: it reads the Jira tickets referenced by the change and checks the code against
  them. Pass this to skip all ticket reading (no `acli` / Datadog calls, no related prompts).
  The orchestrators forward this flag from their own `--skip-ticket`.
```

- [ ] **Step 3: Add a `--skip-ticket` example invocation**

In the ```` ``` ```` example block under "Modifier flags", after the `--raw` PR-mode line, add:

```
/in-depth-review 1234 --skip-ticket # PR mode, skip the ticket-intent role
```

- [ ] **Step 4: Parse the flag in Step 0**

In "Step 0: Resolve scope", after the line:

```
   - Matches `^--raw$` → flag (defer until Step 4).
```

insert:

```
   - Matches `^--skip-ticket$` → flag (defer until Step 1; when set, Role #10 is not launched).
```

- [ ] **Step 5: Verify the edits landed**

Run: `grep -nE 'skip-ticket|up to TEN' shared_config/.claude/skills/in-depth-review/SKILL.md`
Expected: at least 4 matching lines (description, flag doc, example, Step 0 parse).

- [ ] **Step 6: Verify front-matter still parses as YAML**

Run:
```bash
python3 -c "import sys,yaml; t=open('shared_config/.claude/skills/in-depth-review/SKILL.md').read(); fm=t.split('---',2)[1]; yaml.safe_load(fm); print('YAML OK')"
```
Expected: `YAML OK`

- [ ] **Step 7: Commit**

```bash
git add shared_config/.claude/skills/in-depth-review/SKILL.md
git commit -m "feat(in-depth-review): document --skip-ticket flag"
```

---

## Task 2: in-depth-review — add Reviewer Role #10

**Files:**
- Modify: `shared_config/.claude/skills/in-depth-review/SKILL.md` ("Common reviewer prompt fragment", Step 1 launch line, after Role #9)

- [ ] **Step 1: Update the launch-count line in Step 1**

Replace this exact text:

```
Spawn **9 sub-agents in a single message** (9 concurrent tool-use blocks). Sequential
launches defeat the purpose of this design — never serialize.
```

with:

```
Spawn the reviewer sub-agents in a single message (concurrent tool-use blocks). Launch
**10** when ticket review is active (the default), or **9** when `--skip-ticket` was passed
(omit Role #10). Sequential launches defeat the purpose of this design — never serialize.
```

- [ ] **Step 2: Generalize the "one of 9 reviewers" line in the common fragment**

In the "Common reviewer prompt fragment" block, replace:

```
You are one of 9 reviewers running concurrently. Do NOT coordinate with the others.
```

with:

```
You are one of the reviewers running concurrently (9, or 10 when ticket review is active).
Do NOT coordinate with the others.
```

- [ ] **Step 3: Insert Role #10 after Reviewer Role #9**

After the entire "### Reviewer Role #9 — Test coverage review" block (it ends with the line about coverage gaps and tests that don't test anything, just before "## Step 2: Confidence scoring"), insert:

````
### Reviewer Role #10 — Ticket intent compliance

**Skip this role entirely if `--skip-ticket` was passed** — do not launch it. Otherwise it is
the 10th concurrent reviewer. Unlike the diff-focused roles, this one reads the change's
tickets and checks the code against them.

```
Your job: check whether the change implements the work its ticket(s) describe. You compare
each ticket's stated intent against what the diff actually does.

1. Collect ticket IDs from the change:
   - Commit messages in scope:
     - Branch mode: git log --no-pager <RANGE> --format='%s%n%b'
     - PR mode:     gh pr view <PR> --json commits
   - PR title and body (PR mode): gh pr view <PR> --json title,body
   Extract Jira-style IDs matching the regex [A-Z][A-Z0-9]+-[0-9]+ and deduplicate them.

2. If you find NO ticket IDs, respond with exactly NO_ISSUES_FOUND and stop. Do NOT call
   acli. Do NOT trigger any permission prompt. The role is silent when there is nothing to
   check.

3. For each ticket ID, read the ticket: acli jira workitem view <ID>
   Pull the title, description, and acceptance criteria.

4. If a ticket references Datadog — a trace ID, a trace or dashboard URL, or a log query —
   investigate it through the Datadog MCP to understand the actual failure and whether the
   diff addresses it. Load the relevant Datadog skill first, per that MCP's own instructions.
   Keep this bounded to what the ticket explicitly references. Do NOT go fishing.

5. Compare intent against implementation. Flag places where the diff does not implement,
   only partially implements, or contradicts the ticket's stated requirements or acceptance
   criteria. For each finding, set category to "ticket" and ticket_id to the relevant ID.
   Anchor the finding to the file and line(s) where the gap shows up (or the file that
   should have changed but did not).

ABORT ON DENIAL: Running acli or Datadog tools may prompt the user for permission. If any
such permission is denied, immediately stop and return exactly:
TICKET_REVIEW_SKIPPED: access denied
Do not retry and do not work around it. A denial is the user's signal to ignore this
reviewer.

CLEAN EXIT: If acli errors, a ticket is not found, or Jira auth is missing, do not crash.
Respond with NO_ISSUES_FOUND and note the limitation in one line.

You are READ-ONLY everywhere: Jira (view only), Datadog (read only), GitHub (read only).
Never comment on, transition, or otherwise write to a ticket.
```
````

- [ ] **Step 4: Verify the role block landed and is well-formed**

Run: `grep -nE 'Reviewer Role #10|TICKET_REVIEW_SKIPPED|acli jira workitem view' shared_config/.claude/skills/in-depth-review/SKILL.md`
Expected: at least 3 matching lines.

Run: `grep -c '^### Reviewer Role #' shared_config/.claude/skills/in-depth-review/SKILL.md`
Expected: `10`

- [ ] **Step 5: Commit**

```bash
git add shared_config/.claude/skills/in-depth-review/SKILL.md
git commit -m "feat(in-depth-review): add Role #10 ticket intent compliance"
```

---

## Task 3: in-depth-review — scoring, return schema, report, constraints

**Files:**
- Modify: `shared_config/.claude/skills/in-depth-review/SKILL.md` (Step 2 scoring, Step 4 JSON + chat report, Constraints)

- [ ] **Step 1: Add a ticket-scoring note in Step 2**

In "## Step 2: Confidence scoring", immediately after the rubric table (the table whose last row is the `100` / "Absolutely certain…" row), insert:

```
**Ticket-category findings** (from Role #10) are scored on the same 0–100 scale. The question
is "how sure are we the code diverges from what the ticket requires?":

- 100 — the ticket explicitly requires X and the diff demonstrably does not-X
- 75 — the code clearly does not do what the ticket explicitly requires
- 25 — could not confirm the ticket actually requires this (likely a misread of the ticket)

Score the divergence, not the importance of the ticket.
```

- [ ] **Step 2: Add `ticket` to the category enum and add `ticket_id` in the Step 4 JSON**

In the "If invoked as a sub-agent" JSON block, replace this exact line:

```
      "category": "<bug | AGENTS.md | history | prior PR | comment guidance | db | security | error-handling | test coverage>",
```

with these two lines:

```
      "category": "<bug | AGENTS.md | history | prior PR | comment guidance | db | security | error-handling | test coverage | ticket>",
      "ticket_id": "<JIRA-ID this gap traces to, or null for non-ticket findings>",
```

- [ ] **Step 3: Add a top-level `tickets_examined` array to the Step 4 JSON**

In the same JSON block, after the closing `]` of the `"findings"` array and before `"skipped_reason"`, insert:

```
  "tickets_examined": [
    { "id": "<JIRA-ID>", "gaps": <count of surviving ticket findings for this id>, "status": "ok | gaps | unread" }
  ],
```

(`status`: `ok` = read, no gaps; `gaps` = read, has ticket findings; `unread` = could not read it. Empty array when no ticket IDs were found or `--skip-ticket` was set.)

- [ ] **Step 4: Verify the JSON example still parses**

Run:
```bash
python3 - <<'PY'
import re,json
t=open('shared_config/.claude/skills/in-depth-review/SKILL.md').read()
# extract the first fenced ```json block
m=re.search(r'```json\n(.*?)\n```', t, re.S)
block=m.group(1)
# the example uses <...> placeholders and comments; just check the braces/brackets balance
assert block.count('{')==block.count('}'), 'brace mismatch'
assert block.count('[')==block.count(']'), 'bracket mismatch'
print('JSON example structure OK')
PY
```
Expected: `JSON example structure OK`

- [ ] **Step 5: Add the "Tickets examined" line to the chat report**

In "### If invoked directly by the user", inside the chat-report fenced block, after the numbered findings list (after the `2. ...` line) add:

```

**Tickets examined:** <ID> ✅ · <ID> ⚠️ <N> gaps · <ID> ❓ unread
```

Then immediately after that fenced block, add this prose line:

```
Omit the **Tickets examined** line when no ticket IDs were found in the change or when
`--skip-ticket` was passed.
```

- [ ] **Step 6: Update the Constraints section**

Replace this exact constraint bullet:

```
- **9 parallel reviewers per pass** — never serialize, never skip a role for speed. The role
  specialization is the point.
```

with:

```
- **9 or 10 parallel reviewers per pass** — 10 by default (the 10th is ticket intent
  compliance), 9 when `--skip-ticket` is passed. Never serialize, never skip a role for
  speed. The role specialization is the point.
- **Role #10 is read-only and abortable.** It may use `acli jira workitem view` (Jira read)
  and read-only Datadog MCP tools — nothing else. On any denied permission it returns
  `TICKET_REVIEW_SKIPPED` and stops; this is the user's "ignore this reviewer" control. It
  never writes to Jira, Datadog, or GitHub.
```

- [ ] **Step 7: Verify**

Run: `grep -nE 'tickets_examined|ticket_id|Tickets examined|9 or 10 parallel' shared_config/.claude/skills/in-depth-review/SKILL.md`
Expected: at least 4 matching lines.

- [ ] **Step 8: Commit**

```bash
git add shared_config/.claude/skills/in-depth-review/SKILL.md
git commit -m "feat(in-depth-review): score, schema, report, constraints for ticket role"
```

---

## Task 4: pr-review — accept and forward `--skip-ticket`, render ticket output

**Files:**
- Modify: `shared_config/.claude/skills/pr-review/SKILL.md` (body intro, Step 0, Sub-agents 1–5 prompt, Step 2, Step 3b, Step 4)

- [ ] **Step 1: Note the optional role in the body intro**

Replace this exact text:

```
five instances of `in-depth-review`** (nine specialized roles each, raw scored findings)
```

with:

```
five instances of `in-depth-review`** (nine or ten roles each — the tenth is ticket intent
compliance, on unless `--skip-ticket` is passed; raw scored findings)
```

- [ ] **Step 2: Detect `--skip-ticket` in Step 0**

At the end of "## Step 0: Resolve the PR" (after the step that saves `<PR>` and `<IS_DRAFT>`), add:

```
5. If the invocation included `--skip-ticket`, set `<SKIP_TICKET> = true` (default `false`).
   When `true`, every `in-depth-review` sub-agent is invoked with `--skip-ticket`, so Role
   #10 never runs. When `false`, all five `in-depth-review` instances run Role #10 (five
   ticket reviewers). `gh-style-review` is unaffected either way — it has no ticket role.
```

- [ ] **Step 3: Forward the flag in the Sub-agents 1–5 prompt**

In "### Sub-agents 1–5 prompt (in-depth-review)", replace this exact line:

```
1. Invoke the `in-depth-review` skill with the arguments: `<PR> --raw`
```

with:

```
1. Invoke the `in-depth-review` skill with the arguments: `<PR> --raw` — and append
   ` --skip-ticket` when the orchestrator's `<SKIP_TICKET>` is true (so the args become
   `<PR> --raw --skip-ticket`). When `<SKIP_TICKET>` is false, pass `<PR> --raw` unchanged
   so Role #10 runs.
```

- [ ] **Step 4: Carry ticket fields through the merge (Step 2)**

In "## Step 2: Merge and deduplicate (findings)", at the end of item 3 ("For each group, produce one merged finding:"), add this sub-bullet:

```
   - `ticket_id`: preserved from `ticket`-category findings (the Jira ID the gap traces to);
     `null` for all other findings. Never merge two findings that name different `ticket_id`s.
```

- [ ] **Step 5: Aggregate tickets and render them in the global body (Step 3b)**

In "### Step 3b: Build the global body", after the `Let:` list of counts (the `K_unaddressed` line), add:

```
- `tickets_examined` = union by `id` of the `tickets_examined` arrays returned by the five
  in-depth-review sub-agents. For each `id`, `status` is `gaps` if any instance reported
  gaps, else `unread` if any reported unread, else `ok`. `gaps` count = number of surviving
  ticket findings for that `id`.
```

Then, inside the markdown body template, immediately after the `### Code review` findings section (before the `### Still unaddressed in this PR` block), add:

```
<<if tickets_examined is non-empty:>>

### Tickets examined

- <id> — ✅ implemented &nbsp;`[no gaps]`
- <id> — ⚠️ <N> gap(s) found (see findings above)
- <id> — ❓ could not read
   <<endif>>
```

And in the inline-finding and global-finding rendering, where a finding has a non-null
`ticket_id`, prepend `[<ticket_id>] ` to its title.

- [ ] **Step 6: Add tickets to the final report (Step 4)**

In "## Step 4: Final report", in BOTH the "review issued" and "clean PR" branches, add this bullet:

```
- Tickets examined and their outcome: `<id> ✅ | ⚠️ N gaps | ❓ unread`. If any sub-agent
  returned `TICKET_REVIEW_SKIPPED`, say so (the user denied ticket access for that reviewer).
```

- [ ] **Step 7: Verify**

Run: `grep -nE 'SKIP_TICKET|Tickets examined|ticket_id|TICKET_REVIEW_SKIPPED' shared_config/.claude/skills/pr-review/SKILL.md`
Expected: at least 5 matching lines.

- [ ] **Step 8: Commit**

```bash
git add shared_config/.claude/skills/pr-review/SKILL.md
git commit -m "feat(pr-review): forward --skip-ticket and surface ticket findings"
```

---

## Task 5: review-and-fix — forward `--skip-ticket`, ask-the-user on ticket findings

**Files:**
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md` (Step 0, Sub-agents 1–3 prompt, Step 2, Step 4)

- [ ] **Step 1: Detect `--skip-ticket` in Step 0**

At the end of "## Step 0: Setup", after the step that defines `<TARGET_ARG>`, add:

```
7. If the invocation included `--skip-ticket`, set `<SKIP_TICKET> = true` (default `false`).
   When `true`, every `in-depth-review` sub-agent is invoked with `--skip-ticket` so Role #10
   never runs. When `false`, all three `in-depth-review` instances run Role #10 (three ticket
   reviewers). `gh-style-review` is unaffected.
```

- [ ] **Step 2: Forward the flag in the Sub-agents 1–3 prompt**

In "### Sub-agents 1–3 prompt (in-depth-review)", replace this exact line:

```
Invoke the `in-depth-review` skill with the arguments: `<TARGET_ARG> --raw`
```

with:

```
Invoke the `in-depth-review` skill with the arguments: `<TARGET_ARG> --raw` — and append
` --skip-ticket` when the orchestrator's `<SKIP_TICKET>` is true (args become
`<TARGET_ARG> --raw --skip-ticket`). When false, pass `<TARGET_ARG> --raw` unchanged so
Role #10 runs.
```

- [ ] **Step 3: Route ticket findings to the ask-the-user path (Step 2)**

In "## Step 2: Fix", in the per-finding "2. Assess confidence:" sub-section, after the
existing ambiguous-fix bullet, add:

```
   - If the finding's `category` is `ticket` → ALWAYS use `ask_user`, regardless of how
     clear the fix looks. Ticket gaps are intent decisions, not mechanical fixes. Present:
     (1) the ticket ID and the requirement it states, (2) the gap — what the diff does vs.
     what the ticket asks. Offer three choices:
       (a) implement the missing intent (then proceed to implement + commit as usual),
       (b) defer — surface only, make no change this run,
       (c) dismiss — the gap is a false positive or out of scope.
     Only implement and commit when the user picks (a). Never auto-commit a ticket finding.
```

- [ ] **Step 4: Add tickets to the Final Report (Step 4)**

In "## Step 4: Final Report", inside the report template, after the "### Changes Made"
section, add:

```
### Tickets examined
- <id>: ✅ implemented | ⚠️ N gap(s) — <user decision: implemented / deferred / dismissed> | ❓ unread
(Omit this section when no ticket IDs were found or `--skip-ticket` was passed. If a reviewer
returned TICKET_REVIEW_SKIPPED, note that ticket access was denied.)
```

- [ ] **Step 5: Note the optional role in the body intro**

In the "# Review and Fix" intro, in item 1 of the numbered "Each iteration:" list, replace:

```
1. Runs **3 `in-depth-review` + 3 `gh-style-review` instances in parallel** (6 total, each
   invoked with `--raw`).
```

with:

```
1. Runs **3 `in-depth-review` + 3 `gh-style-review` instances in parallel** (6 total, each
   invoked with `--raw`). The 3 in-depth instances also run Role #10 (ticket intent
   compliance) unless `--skip-ticket` was passed.
```

- [ ] **Step 6: Verify**

Run: `grep -nE 'SKIP_TICKET|category. is .ticket|Tickets examined|ask_user' shared_config/.claude/skills/review-and-fix/SKILL.md`
Expected: at least 4 matching lines.

- [ ] **Step 7: Commit**

```bash
git add shared_config/.claude/skills/review-and-fix/SKILL.md
git commit -m "feat(review-and-fix): forward --skip-ticket, ask before acting on ticket gaps"
```

---

## Task 6: Sync, cross-file consistency check, and optional smoke test

**Files:** none modified (verification + sync only)

- [ ] **Step 1: Confirm `gh-style-review` was left untouched**

Run: `git diff --name-only origin/main..HEAD -- shared_config/.claude/skills/`
Expected: lists `in-depth-review/SKILL.md`, `pr-review/SKILL.md`, `review-and-fix/SKILL.md` — NOT `gh-style-review/SKILL.md`.

- [ ] **Step 2: Confirm the role count is consistent everywhere**

Run: `grep -rnE 'NINE|nine specialized|9 parallel|9 reviewers' shared_config/.claude/skills/in-depth-review/SKILL.md shared_config/.claude/skills/pr-review/SKILL.md`
Expected: no stale "always 9" claims remain — every mention now allows for 10 (or is the generalized "9 or 10" / "nine or ten" wording).

- [ ] **Step 3: Confirm all three skills name the same flag and category**

Run: `grep -rl 'skip-ticket' shared_config/.claude/skills/`
Expected: the three modified skills are listed.

Run: `grep -rn 'category.*ticket\|category. is .ticket\|"ticket"' shared_config/.claude/skills/`
Expected: `in-depth-review` defines the `ticket` category; `pr-review` and `review-and-fix` reference it. The string is spelled `ticket` everywhere (not `tickets`, not `ticket-intent`).

- [ ] **Step 4: Sync to the live config**

Run: `melvin-config claude sync`
Expected: reports the three skill files synced to `~/.claude/skills/`.

- [ ] **Step 5 (optional, interactive): smoke-test the flag end to end**

On a branch whose commits reference a real Jira ticket, run `/in-depth-review origin/main..HEAD`
and confirm: it launches 10 reviewers, the ticket reviewer reads the ticket (or aborts cleanly
on deny), and a "Tickets examined" line appears. Then run the same with `--skip-ticket` and
confirm it launches 9 reviewers and makes no `acli` call. This step is manual — there is no
automated harness for skill prompts.

- [ ] **Step 6: Final commit (only if Steps 1–3 required any consistency fixes)**

```bash
git add shared_config/.claude/skills/
git commit -m "docs(review-skills): align ticket-role wording across skills"
```

---

## Self-Review (completed by plan author)

- **Spec coverage:** role placement (Tasks 1–2), data sources + abort + clean exit (Task 2), scoring + schema + report + constraints (Task 3), `--skip-ticket` plumbing (Tasks 1/4/5), pr-review surfacing (Task 4), review-and-fix ask-the-user (Task 5), doc consistency + `gh-style-review` untouched (Task 6). All spec sections map to a task.
- **Placeholder scan:** no "TBD/handle-edge-cases" steps — every edit shows exact before/after text. The `<...>` tokens inside skill content are intentional template placeholders that already exist in these prompt files, not plan gaps.
- **Type consistency:** flag is `--skip-ticket`, orchestrator variable is `<SKIP_TICKET>`, finding category is `ticket`, field is `ticket_id`, top-level array is `tickets_examined`, abort sentinel is `TICKET_REVIEW_SKIPPED: access denied` — used identically across all tasks.
