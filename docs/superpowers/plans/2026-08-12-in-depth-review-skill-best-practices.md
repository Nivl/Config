# in-depth-review Skill Restructure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split `shared_config/.claude/skills/in-depth-review/SKILL.md` (1,124 lines) into a lean orchestration file plus 17 reference files, per the best-practices doc's progressive-disclosure guidance, with zero behavior change.

**Architecture:** Extract content that a caller doesn't always need (the 12 reviewer prompts, the async-collection protocol, the scoring rubric, the two output formats) into standalone Markdown files referenced one level deep from `SKILL.md`. `SKILL.md` itself becomes a ~425-line spine: argument parsing, gate evaluation, and pointers to everything else.

**Tech Stack:** Plain Markdown skill files. No executable code, no test framework. "Testing" here means `wc -l`/`grep`/`diff` checks that the resulting files match the spec exactly, plus a full re-read of every sibling skill/agent that depends on this file's contract.

## Global Constraints

These apply to every task below; do not re-derive them per task.

- **Zero behavior change.** Every flag (`--raw`, `--skip-ticket`, `--roles`), the role-number-to-category table, the JSON output shape, the GitHub read/write policy, and the model policy (Sonnet reviewers, Haiku scorers) stay byte-identical to today's behavior.
- **Preserve verbatim content exactly**, including the source's em dashes (`—`) and the one pre-existing stray `**` typo in Role #7's gate text (source lines 488-489). Do not "fix" it. Only NEW authored text (pointer sentences, the new frontmatter description, the relocated Constraints clause) uses plain ASCII per this repo's authored-prose convention.
- **Every new file uses a single top-level `#` heading** as its own title, even when the moved content's original heading was `###` (a role file) or had no heading at all (a gated role's prompt-only file, which gets a freshly authored `#` title using that role's original header text verbatim).
- **Every new file is referenced directly from `SKILL.md`.** No new file references another new file (keeps every reference one level deep).
- **Final `SKILL.md` body must be 425 lines** (`wc -l`). This is not "roughly 425" — if the real count differs after an edit, stop and reconcile against the Line budget table in the spec before continuing, because a mismatch means an edit did something other than what was planned.
- **Prefer `sed` with pattern anchors over hardcoded line numbers** for every `SKILL.md` edit in Tasks 3-4, because earlier edits in the same task shift later line numbers. Every anchor pattern below was verified unique against the live file with `grep -n` immediately before this plan was written.
- **This is macOS/BSD `sed`, not GNU `sed`.** In-place edits need the empty-string argument (`sed -i ''`, not `sed -i`). A range-delete-with-exclusion (`/A/,/B/{/B/!d}`) needs a trailing semicolon before the closing brace (`{/B/!d;}`) or BSD `sed` rejects it with "extra characters at the end of d command" -- every such command below already has it; if you write a new one for a Task 5 fix, don't drop it.
- **Every `sed 'r'` splice below inserts a leading blank line but never a trailing one.** The line immediately after each anchor already has its own separating blank in the untouched original file; a splice block that also ends in a blank produces a double blank line. This was verified by actually running the full sequence against a scratch copy while writing this plan (see the exact expected `wc -l` after each step) -- if you change a splice block's content, re-run that check rather than assuming the count still holds.
- **Source of truth for all "verbatim" content is the live file**, extracted with `sed`, never retyped by hand. The spec this plan implements documents a real incident where hand-retyping "verbatim" text silently flattened em dashes and reworded a sentence — do not repeat that here.
- Commit after each task. This file lives in a git-tracked path (`shared_config/.claude/skills/in-depth-review/`), unlike `docs/superpowers/` (gitignored) where the spec and this plan live.

**Spec:** `docs/superpowers/specs/2026-08-12-in-depth-review-skill-best-practices-design.md` — read it once before starting Task 1. Every exact anchor line and measured line-count below is drawn from it.

---

## File Structure

```
shared_config/.claude/skills/in-depth-review/
├── SKILL.md               (modified: 1,124 -> 425 lines)
├── COLLECTING.md          (new: 77 lines)
├── SCORING.md             (new: ~68 lines)
├── OUTPUT-JSON.md         (new: ~79 lines)
├── OUTPUT-CHAT.md         (new: ~43 lines)
└── roles/
    ├── _common-fragment.md   (new: ~41 lines)
    ├── 01-agents-md.md       (new: 13 lines)
    ├── 02-bug-scan.md        (new: 14 lines)
    ├── 03-history.md         (new: 31 lines)
    ├── 04-prior-pr.md        (new: 38 lines)
    ├── 05-comment-guidance.md (new: 21 lines)
    ├── 06-database.md        (new: ~22 lines)
    ├── 07-security.md        (new: ~31 lines)
    ├── 08-error-handling.md  (new: 26 lines)
    ├── 09-test-coverage.md   (new: 31 lines)
    ├── 10-ticket.md          (new: 70 lines)
    ├── 11-motivation.md      (new: 38 lines)
    └── 12-types.md           (new: ~31 lines)
```

Task 1 creates every file under `roles/`. Task 2 creates the four top-level reference files. Task 3 rewrites `SKILL.md` lines that come before the role prompts (frontmatter through the end of Role #12) into their final form. Task 4 rewrites everything from Step 2.1 onward. Task 5 verifies the finished result end to end. **Run them in this order** — Tasks 1-2 only read `SKILL.md`, never modify it, so `SKILL.md`'s original line numbers stay valid for them regardless of order between the two. Tasks 3-4 modify `SKILL.md` and must run after Tasks 1-2 (they reference the new files by path) and in order relative to each other (Task 4's anchors assume Task 3 already ran, since `SKILL.md`'s content — though not its line numbers — matters for pattern matching).

---

### Task 1: Extract the twelve role prompt files and the common fragment

**Files:**
- Create: `shared_config/.claude/skills/in-depth-review/roles/_common-fragment.md`
- Create: `shared_config/.claude/skills/in-depth-review/roles/01-agents-md.md` through `roles/12-types.md` (12 files)
- Read only: `shared_config/.claude/skills/in-depth-review/SKILL.md` (not modified this task)

**Interfaces:**
- Consumes: nothing (reads the live, still-unmodified `SKILL.md`).
- Produces: 13 files under `roles/`, each starting with a single `#` heading, that Task 3 will reference by path from `SKILL.md`'s role table and that a launched reviewer sub-agent will eventually receive as its prompt text (fragment + role file concatenated).

Work from `~/.melvin/config` (or `cd` there first). All line numbers below are against the **original, unmodified** `SKILL.md` — confirmed via `grep -n` immediately before this plan was written, so they should still be exact when you run this task, since Task 1 doesn't change `SKILL.md`.

- [ ] **Step 1: Extract the common fragment**

```bash
sed -n '285,323p' shared_config/.claude/skills/in-depth-review/SKILL.md > shared_config/.claude/skills/in-depth-review/roles/_common-fragment.md
```

Then prepend a new H1 title before that fence. Use the Edit tool on the new file:
- old_string: the file's current first line, exactly `` ``` `` (the fence-open line the `sed` above copied)
- new_string: three lines -- `# Common reviewer prompt fragment`, a blank line, then `` ``` `` again (the same fence-open line, now with the title above it instead of replaced by it).

- [ ] **Step 2: Verify the common fragment**

```bash
wc -l shared_config/.claude/skills/in-depth-review/roles/_common-fragment.md
```

Expected: `41` (39 lines of extracted content + 2 added: the H1 title line and the blank line after it).

- [ ] **Step 3: Extract the nine whole-block roles**

Each of these roles moves as a complete block (its original `###` header through its closing fence), so extraction is a single `sed` call per role, using the tile ranges below (verified against the live file's header positions):

```bash
sed -n '325,337p' shared_config/.claude/skills/in-depth-review/SKILL.md > shared_config/.claude/skills/in-depth-review/roles/01-agents-md.md
sed -n '338,351p' shared_config/.claude/skills/in-depth-review/SKILL.md > shared_config/.claude/skills/in-depth-review/roles/02-bug-scan.md
sed -n '352,382p' shared_config/.claude/skills/in-depth-review/SKILL.md > shared_config/.claude/skills/in-depth-review/roles/03-history.md
sed -n '383,420p' shared_config/.claude/skills/in-depth-review/SKILL.md > shared_config/.claude/skills/in-depth-review/roles/04-prior-pr.md
sed -n '421,441p' shared_config/.claude/skills/in-depth-review/SKILL.md > shared_config/.claude/skills/in-depth-review/roles/05-comment-guidance.md
sed -n '543,568p' shared_config/.claude/skills/in-depth-review/SKILL.md > shared_config/.claude/skills/in-depth-review/roles/08-error-handling.md
sed -n '569,599p' shared_config/.claude/skills/in-depth-review/SKILL.md > shared_config/.claude/skills/in-depth-review/roles/09-test-coverage.md
sed -n '600,669p' shared_config/.claude/skills/in-depth-review/SKILL.md > shared_config/.claude/skills/in-depth-review/roles/10-ticket.md
sed -n '670,707p' shared_config/.claude/skills/in-depth-review/SKILL.md > shared_config/.claude/skills/in-depth-review/roles/11-motivation.md
```

Run each as its own Bash call (do not chain them with `&&` — the sandbox denies compound `cd`+redirect chains, but sequential plain commands sharing a working directory are fine).

- [ ] **Step 4: Change each whole-block role's header from `###` to `#`**

For each of the 9 files above, use the Edit tool to change just the first line's heading level. The old_string / new_string pairs (the header text is copied exactly from the source, only the leading `###` becomes `#`):

| File | old_string | new_string |
|---|---|---|
| `roles/01-agents-md.md` | `### Reviewer Role #1 — AGENTS.md compliance` | `# Reviewer Role #1 — AGENTS.md compliance` |
| `roles/02-bug-scan.md` | `### Reviewer Role #2 — Shallow bug scan` | `# Reviewer Role #2 — Shallow bug scan` |
| `roles/03-history.md` | `### Reviewer Role #3 — Git history context` | `# Reviewer Role #3 — Git history context` |
| `roles/04-prior-pr.md` | `### Reviewer Role #4 — Prior PR comments (read-only)` | `# Reviewer Role #4 — Prior PR comments (read-only)` |
| `roles/05-comment-guidance.md` | `### Reviewer Role #5 — In-file code comments` | `# Reviewer Role #5 — In-file code comments` |
| `roles/08-error-handling.md` | `### Reviewer Role #8 — Error-handling review` | `# Reviewer Role #8 — Error-handling review` |
| `roles/09-test-coverage.md` | `### Reviewer Role #9 — Test coverage review` | `# Reviewer Role #9 — Test coverage review` |
| `roles/10-ticket.md` | `### Reviewer Role #10 — Ticket intent compliance` | `# Reviewer Role #10 — Ticket intent compliance` |
| `roles/11-motivation.md` | `### Reviewer Role #11 — Headline-benefit / motivation delivery` | `# Reviewer Role #11 — Headline-benefit / motivation delivery` |

- [ ] **Step 5: Extract the three gated roles' prompt blocks only**

These roles' gate criteria go elsewhere (Task 3, into `SKILL.md` itself), so only the fenced prompt block moves here — the range starts at the fence-open, not at the role's original `###` header:

```bash
sed -n '464,484p' shared_config/.claude/skills/in-depth-review/SKILL.md > shared_config/.claude/skills/in-depth-review/roles/06-database.md
sed -n '512,541p' shared_config/.claude/skills/in-depth-review/SKILL.md > shared_config/.claude/skills/in-depth-review/roles/07-security.md
sed -n '720,749p' shared_config/.claude/skills/in-depth-review/SKILL.md > shared_config/.claude/skills/in-depth-review/roles/12-types.md
```

- [ ] **Step 6: Prepend a new H1 title to each gated role's file**

Unlike Step 4 (which changed an existing header's level), these three files currently start directly with a fence (` ``` `), since only the prompt block was extracted. Use the Edit tool to prepend a title before that fence:

For `roles/06-database.md`:
- old_string: `` ``` `` (the file's current first line)
- new_string:
```
# Reviewer Role #6 — Database / data-layer scan (conditional)

```
```

For `roles/07-security.md`:
- old_string: `` ``` `` (the file's current first line)
- new_string:
```
# Reviewer Role #7 — OWASP Top 10 security scan (conditional, but bias hard toward running)

```
```

For `roles/12-types.md`:
- old_string: `` ``` `` (the file's current first line)
- new_string:
```
# Reviewer Role #12 — TypeScript type safety (conditional)

```
```

- [ ] **Step 7: Verify every role file's line count**

```bash
wc -l shared_config/.claude/skills/in-depth-review/roles/*.md
```

Expected (each file gained exactly 2 lines over its extraction range from the header-level change or the prepended title+blank):

```
41 _common-fragment.md
13 01-agents-md.md
14 02-bug-scan.md
31 03-history.md
38 04-prior-pr.md
21 05-comment-guidance.md
23 06-database.md
32 07-security.md
26 08-error-handling.md
31 09-test-coverage.md
70 10-ticket.md
38 11-motivation.md
32 12-types.md
```

If any number is off, re-check that step's `sed` range or Edit before continuing — do not adjust the expected numbers to match a wrong result.

- [ ] **Step 8: Spot-check one file against the source**

Two extractions to temp files, then a plain `diff` -- not `diff <(...) <(...)`, which process substitution denies under this environment's sandbox:

```bash
sed -n '2,13p' shared_config/.claude/skills/in-depth-review/roles/01-agents-md.md > /tmp/spotcheck-a.txt
sed -n '326,337p' shared_config/.claude/skills/in-depth-review/SKILL.md > /tmp/spotcheck-b.txt
diff /tmp/spotcheck-a.txt /tmp/spotcheck-b.txt
```

Expected: no output (the new file's lines 2-13, after its new H1 title, are byte-identical to the source's original lines 326-337).

- [ ] **Step 9: Commit**

```bash
git add shared_config/.claude/skills/in-depth-review/roles/
git commit -m "$(cat <<'EOF'
refactor(claude,skill): in-depth-review: extract reviewer role prompts into roles/

Splits the 12 reviewer prompts and their shared fragment out of SKILL.md
into one file each, per the Skill authoring best-practices doc's
progressive-disclosure guidance. SKILL.md itself is updated in a later
commit; these files aren't referenced by anything yet.
EOF
)"
```

---

### Task 2: Create the four shared reference files

**Files:**
- Create: `shared_config/.claude/skills/in-depth-review/COLLECTING.md`
- Create: `shared_config/.claude/skills/in-depth-review/SCORING.md`
- Create: `shared_config/.claude/skills/in-depth-review/OUTPUT-JSON.md`
- Create: `shared_config/.claude/skills/in-depth-review/OUTPUT-CHAT.md`
- Read only: `shared_config/.claude/skills/in-depth-review/SKILL.md` (not modified this task)

**Interfaces:**
- Consumes: nothing.
- Produces: 4 files that Task 3/4 will reference by path from `SKILL.md`. `COLLECTING.md` is new authored content (generalized from the source); the other three are verbatim extractions with a new title prepended.

- [ ] **Step 1: Write `COLLECTING.md`**

This is new authored content (the source's role-specific and scorer-specific collection-protocol text, generalized to "agent"), not an extraction. Write the complete file:

```markdown
# Collecting async agent results

Shared by Step 1 (collecting reviewer roles) and Step 2.1 (collecting scorers). Wherever this
file says "agent," substitute the caller's term: a launched reviewer role in Step 1, a launched
scorer in Step 2.1. Wherever it says "missing bookkeeping," Step 1 records the agent in
`roles_missing`; Step 2.1 marks the finding `unscored: true` / `confidence: null`.

**An agent's output reaches the parent through the text it returns, and nowhere else.** Every
agent prompt must instruct it to put its COMPLETE output in its FINAL TEXT OUTPUT.

**Agents send nothing anywhere.** Never instruct an agent to use `SendMessage`, agent teams, or a
shared file. An agent has no resolvable address for you. An agent TYPE is not an agent id, so an
attempt fails every time with `no reachable agent named <your agent type>`. One measured run
burned 256 such attempts and delivered nothing. The output goes in the returned text, and that is
the only channel.

**How that returned text actually reaches you.** The Agent tool here launches ASYNCHRONOUSLY. Read
this before deciding what to do after launching, because the intuitive reading is wrong:

- The Agent tool result gives you launch metadata and an `agentId`. It does NOT contain the
  agent's output. There is nothing to read at launch. Do not wait on a launch-time return value,
  and do not treat its absence as a failure.
- An agent's output arrives LATER, inside a `<task-notification>` block, usually several batched
  into one turn. That block's `<result>` body holds the agent's final text, and its `<task-id>` is
  byte-identical to the `agentId` you recorded at launch.
- **You receive notifications only while you keep taking turns.** Any tool call is a turn.
- **A parent that ends its turn is FINALIZED and stops receiving.** Its outstanding agents'
  results then surface in the session root, where you cannot see them. This is the one behaviour
  that loses agents. It is why "wait for the results" is not an instruction anyone can follow, and
  why a progress note is not a safe thing to emit. Emitting one ends your turn.

**The collection protocol.** Follow it exactly.

1. Record every `agentId` the launch returned.
2. Take a turn. Harvest every `<task-notification>` in front of you, match each `<task-id>` to a
   recorded `agentId`, and keep its `<result>` body.
3. If any recorded `agentId` is still unaccounted for, take another turn. If you have no
   productive work, re-read the diff for an agent you are still waiting on. An unproductive turn
   still collects.
4. Repeat until every recorded `agentId` is accounted for, OR until THREE CONSECUTIVE COLLECTING
   TURNS have brought zero new arrivals. A collecting turn is ONE substantive tool call that names
   the artifact it read, so re-read the diff, a changed file, or the finding being scored. Three
   repeats of the same no-op are not three turns.
   Do NOT start the zero-arrival counter until you have taken at least as many collecting turns as
   you launched agents, with a floor of five, whether or not anything has arrived. The three
   zero-arrival turns are counted FRESH from the moment the counter arms, so turns taken before
   arming never count toward them. Agents take minutes, not seconds, so a counter armed at launch
   measures your own polling speed rather than a failure. If you reach the bound and have not yet
   re-armed the counter, re-arm it EXACTLY ONCE and keep collecting. After that, honor the bound.
5. At that bound, once you have already used your single re-arm, record every still-unaccounted
   agent in the caller's missing bookkeeping, close the accounting, and continue. Do not wait
   longer. Do not treat a give-up as clean.

A notification that arrives AFTER the bound is still that agent's report. Fold it into the pool and
clear it from the missing bookkeeping. The accounting is final only at the moment the caller
emits its own final output, not at the moment the bound fires.

**Never end your turn while a recorded `agentId` is unaccounted for**, unless you are declaring it
in the missing bookkeeping on that same turn.

**Your final output is the report, never a status update.** If agents are still unaccounted for
when you hit the bound, emit the report with them marked missing. Never return a progress note
saying you are still waiting, because that ends your turn and finalizes you with less than you
could have collected.

`TaskOutput` appears in the deferred-tool listing and looks like exactly the right tool for this.
It is NOT available to a nested agent. `ToolSearch select:TaskOutput` returns no match from inside
an agent's parent, though the same query resolves at the session root. Do not build on it. The
protocol above is the mechanism.

**An agent that never reports has reported nothing**, whatever it may have produced elsewhere.
Classify it as missing per the caller's bookkeeping. Do not hunt for its output in another channel
and splice it in. That makes delivery depend on luck rather than on the contract.

This section is written this way because the shorter version failed in practice. A parent told
only to read a result that the launch never produced had nothing to read, stopped, and then
invented findings for three agents it never heard from.
```

- [ ] **Step 2: Verify `COLLECTING.md`**

```bash
wc -l shared_config/.claude/skills/in-depth-review/COLLECTING.md
```

Expected: `77`.

- [ ] **Step 3: Extract `SCORING.md`**

```bash
sed -n '866,931p' shared_config/.claude/skills/in-depth-review/SKILL.md > shared_config/.claude/skills/in-depth-review/SCORING.md
```

Then prepend a title. Edit the new file:
- old_string: the current first line, `Each scorer returns a number 0–100 with this rubric:`
- new_string:
```
# Scoring rubric

Each scorer returns a number 0–100 with this rubric:
```

- [ ] **Step 4: Extract `OUTPUT-JSON.md`**

```bash
sed -n '970,1046p' shared_config/.claude/skills/in-depth-review/SKILL.md > shared_config/.claude/skills/in-depth-review/OUTPUT-JSON.md
```

Then prepend a title. Edit the new file:
- old_string: the current first line, `` ### If invoked as a sub-agent (by `review-and-fix` or `pr-review`) ``
- new_string:
```
# Output: sub-agent JSON shape

### If invoked as a sub-agent (by `review-and-fix` or `pr-review`)
```

- [ ] **Step 5: Extract `OUTPUT-CHAT.md`**

```bash
sed -n '1047,1087p' shared_config/.claude/skills/in-depth-review/SKILL.md > shared_config/.claude/skills/in-depth-review/OUTPUT-CHAT.md
```

Then prepend a title. Edit the new file:
- old_string: the current first line, `### If invoked directly by the user`
- new_string:
```
# Output: chat report

### If invoked directly by the user
```

- [ ] **Step 6: Verify line counts**

```bash
wc -l shared_config/.claude/skills/in-depth-review/SCORING.md shared_config/.claude/skills/in-depth-review/OUTPUT-JSON.md shared_config/.claude/skills/in-depth-review/OUTPUT-CHAT.md
```

Expected:

```
68 SCORING.md
79 OUTPUT-JSON.md
43 OUTPUT-CHAT.md
```

- [ ] **Step 7: Spot-check `OUTPUT-JSON.md` against the source**

Two extractions to temp files, then a plain `diff` -- not `diff <(...) <(...)`, which process substitution denies under this environment's sandbox:

```bash
sed -n '3,79p' shared_config/.claude/skills/in-depth-review/OUTPUT-JSON.md > /tmp/spotcheck-a.txt
sed -n '970,1046p' shared_config/.claude/skills/in-depth-review/SKILL.md > /tmp/spotcheck-b.txt
diff /tmp/spotcheck-a.txt /tmp/spotcheck-b.txt
```

Expected: no output. (Line 3 of `OUTPUT-JSON.md` is its own `### If invoked as a sub-agent...` heading, which is source line 970, not 971 -- the file's line 1 is the new H1 title, line 2 is blank, so line 3 lines up with the source's first content line.)

- [ ] **Step 8: Commit**

```bash
git add shared_config/.claude/skills/in-depth-review/COLLECTING.md shared_config/.claude/skills/in-depth-review/SCORING.md shared_config/.claude/skills/in-depth-review/OUTPUT-JSON.md shared_config/.claude/skills/in-depth-review/OUTPUT-CHAT.md
git commit -m "$(cat <<'EOF'
refactor(claude,skill): in-depth-review: extract collection, scoring, and output reference files

COLLECTING.md generalizes the duplicated role/scorer collection protocol.
SCORING.md, OUTPUT-JSON.md, and OUTPUT-CHAT.md are verbatim extractions.
SKILL.md is updated to point at these in a later commit.
EOF
)"
```

---

### Task 3: Rewrite `SKILL.md`'s front half (frontmatter through Role #12)

**Files:**
- Modify: `shared_config/.claude/skills/in-depth-review/SKILL.md`

**Interfaces:**
- Consumes: the file paths created in Task 1 (`roles/*.md`) — this task's new role table references them by path.
- Produces: a `SKILL.md` whose Step 1 is fully modernized; Step 2 onward is still the original text (Task 4 handles that). Intermediate state after this task is coherent and independently reviewable.

Before starting, re-run the anchor checks below and confirm they still match — if `SKILL.md` was touched by anything else since this plan was written, stop and reconcile against the spec instead of proceeding on stale assumptions.

```bash
grep -n '^### How a role' shared_config/.claude/skills/in-depth-review/SKILL.md
grep -n '^## Step 2: Confidence scoring$' shared_config/.claude/skills/in-depth-review/SKILL.md
```

Expected: the first prints `180:### How a role's findings reach the parent`, the second prints `751:## Step 2: Confidence scoring`.

- [ ] **Step 1: Replace the frontmatter description**

Edit `SKILL.md`:
- old_string:
```
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
```
- new_string:
```
description: >
  Performs one in-depth, multi-perspective code review of a pull request or commit range using
  up to twelve specialized parallel reviewer roles (AGENTS.md compliance, bug scan, git history,
  prior PR feedback, in-file comments, database, OWASP security, error handling, test coverage,
  ticket-intent compliance, motivation delivery, TypeScript type safety). Scores findings for
  confidence, filters, and deduplicates. Never writes to GitHub. Use for "in-depth review", "deep
  review", "thorough review", or "code review without fixing" -- standalone or as a building
  block for `pr-review` and `review-and-fix`.
```

- [ ] **Step 2: Verify the description length and the file still parses as valid frontmatter**

```bash
wc -l shared_config/.claude/skills/in-depth-review/SKILL.md
```

Expected: `1113` (1,124 minus the 19-line old description block, plus the 8-line new one: 1124 - 19 + 8 = 1113).

- [ ] **Step 3: Edit the gate table's three "gate detail" cells**

Three separate Edit calls, each touching only the last cell of one row (leave every other character, including the em dash in row 3, untouched):

Edit 1 of 3:
- old_string: `| #6 database / data-layer | the diff touches data-layer code | see Role #6 |`
- new_string: `| #6 database / data-layer | the diff touches data-layer code | see below |`

Edit 2 of 3:
- old_string: `| #7 OWASP security | almost always — skip only a provably no-surface diff | see Role #7 |`
- new_string: `| #7 OWASP security | almost always — skip only a provably no-surface diff | see below |`

Edit 3 of 3:
- old_string: `` | #12 TypeScript type safety | the diff touches `*.ts` / `*.tsx` / `*.mts` / `*.cts` | see Role #12 | ``
- new_string: `` | #12 TypeScript type safety | the diff touches `*.ts` / `*.tsx` / `*.mts` / `*.cts` | see below | ``

- [ ] **Step 4: Verify no "see Role #N" cross-references remain**

```bash
grep -n 'see Role #' shared_config/.claude/skills/in-depth-review/SKILL.md
```

Expected: no output.

- [ ] **Step 5: Write the "Conditional role gates" splice block to a temp file**

```bash
cat > /tmp/conditional-role-gates.md << 'BLOCKEOF'

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
`git --no-pager diff --name-only <RANGE>` (branch mode) before launching it. If the diff has no
TypeScript, skip this role entirely and do not count it against the role total.

It exists because a narrow lens catches what a broad one misses. Role #1 reads AGENTS.md in full
and nominally covers this ground, but casting violations are a specific, mechanical, easily-missed
pattern in a large compliance sweep. The conditional launch is what keeps the extra recall from
costing anything on the many diffs with no TypeScript in them.
BLOCKEOF
```

This is a plain heredoc writing a file, not a script execution — safe to run as-is.

- [ ] **Step 6: Verify the temp file's line count**

```bash
wc -l /tmp/conditional-role-gates.md
```

Expected: `68` (a leading blank line + the 67-line block, no trailing blank -- the original file's own existing blank line right after "Evaluate the gates." provides the separation from "This step owns gate evaluation" that follows, so adding another would double it up).

- [ ] **Step 7: Splice the block in after the "Evaluate the gates." line**

```bash
sed -i '' '/^number\. Evaluate the gates\.$/r /tmp/conditional-role-gates.md' shared_config/.claude/skills/in-depth-review/SKILL.md
```

- [ ] **Step 8: Verify the splice**

```bash
grep -n '^### Conditional role gates$' shared_config/.claude/skills/in-depth-review/SKILL.md
```

Expected: one match.

- [ ] **Step 9: Delete the old "How a role's findings..." through end-of-Role-#12 block, and splice in the replacement**

This single delete removes the collection-protocol section, the `--roles` note, the role table, model policy, the common fragment, and all 12 role blocks — everything Task 1 already safely copied elsewhere, plus the now-relocated gate criteria (already spliced in by Step 7 above). Delete from the "How a role's findings" header through (not including) the "Step 2" header:

```bash
sed -i '' '/^### How a role.s findings reach the parent$/,/^## Step 2: Confidence scoring$/{/^## Step 2: Confidence scoring$/!d;}' shared_config/.claude/skills/in-depth-review/SKILL.md
```

- [ ] **Step 10: Verify the delete**

```bash
grep -n '^### Reviewer Role\|^### Common reviewer prompt fragment$\|^### How a role' shared_config/.claude/skills/in-depth-review/SKILL.md
```

Expected: no output (every one of those headers is gone from `SKILL.md`; they now live only in `roles/*.md` or in the "Conditional role gates" subsection you spliced in above).

```bash
grep -n '^## Step 2: Confidence scoring$' shared_config/.claude/skills/in-depth-review/SKILL.md
```

Expected: one match, now immediately preceded by the end of the Conditional-role-gates splice.

- [ ] **Step 11: Write the replacement content (new tile 8 + tile 9) to a temp file**

```bash
cat > /tmp/how-findings-reach-parent.md << 'BLOCKEOF'

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
column below holds only the text sent to the launched sub-agent:

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

**Model: spawn every reviewer on Sonnet** (Agent-tool `model: sonnet`). Do NOT let them
inherit the session model. Each role is a bounded, tightly-specified recall pass over the
diff; that is exactly the work Sonnet does well and Opus does at ~5x the cost. Confidence is
recovered downstream by the cross-role agreement count and (for the orchestrators) the
triangulation + adversarial converge stage — not by making each finder more expensive.

### Common reviewer prompt fragment

Every reviewer prompt starts with the block in [roles/_common-fragment.md](roles/_common-fragment.md).
Read it once per run and prepend it to each launched role's prompt (from its file below) before
sending.
BLOCKEOF
```

- [ ] **Step 12: Verify the temp file's line count**

```bash
wc -l /tmp/how-findings-reach-parent.md
```

Expected: `42` (a leading blank line + 41 content lines covering both the "How a role's findings" and "Common reviewer prompt fragment" replacements, no trailing blank).

- [ ] **Step 13: Splice the replacement in after the Step 1 preamble's closing paragraph**

**Anchor on the end of the "This step owns gate evaluation" paragraph, not on anything inside the Conditional-role-gates block you just spliced in.** That block (Step 7) also ends with a Role #12 line about TypeScript, which is tempting but wrong to anchor on here -- it would insert this content in the middle of Step 1's gate table area instead of right before "## Step 2: Confidence scoring", which is where it belongs now that the old role blocks are gone:

```bash
sed -i '' '/^this, because the file list does not exist yet when arguments are parsed\.$/r /tmp/how-findings-reach-parent.md' shared_config/.claude/skills/in-depth-review/SKILL.md
```

- [ ] **Step 14: Verify the full front half**

```bash
wc -l shared_config/.claude/skills/in-depth-review/SKILL.md
```

Expected: `652`. This exact figure was confirmed by actually running this task's full sequence against a scratch copy while writing this plan, not derived by hand arithmetic -- treat any deviation as a real defect to find, not a rounding difference to wave off.

```bash
grep -n '^### Conditional role gates$\|^### How a role.s findings reach the parent$\|^### Common reviewer prompt fragment$\|^## Step 2: Confidence scoring$' shared_config/.claude/skills/in-depth-review/SKILL.md
```

Expected: four matches, each exactly once, in that order.

- [ ] **Step 15: Confirm the role table's `#`/`category` columns are unchanged**

```bash
grep -n '| 6 | Database / data-layer | `db` | `roles/06-database.md` |' shared_config/.claude/skills/in-depth-review/SKILL.md
```

Expected: one match.

- [ ] **Step 16: Commit**

```bash
git add shared_config/.claude/skills/in-depth-review/SKILL.md
git commit -m "$(cat <<'EOF'
refactor(claude,skill): in-depth-review: rewrite Step 1 for progressive disclosure

Frontmatter description trimmed under the doc's 1,024-char limit. Gate
criteria for roles #6/#7/#12 consolidated into one "Conditional role
gates" subsection so the orchestrator can evaluate them without opening
a role file. The collection protocol, role prompts, and common fragment
now live in COLLECTING.md and roles/*.md, referenced by a File column
on the existing role table. No behavior change: same flags, same role
numbers, same categories.
EOF
)"
```

---

### Task 4: Rewrite `SKILL.md`'s back half (Step 2.1 through Constraints)

**Files:**
- Modify: `shared_config/.claude/skills/in-depth-review/SKILL.md`

**Interfaces:**
- Consumes: `COLLECTING.md`, `SCORING.md`, `OUTPUT-JSON.md`, `OUTPUT-CHAT.md` from Task 2 (referenced by path).
- Produces: the finished `SKILL.md` at its final 425-line length.

Before starting, confirm these anchors still match (they're independent of Task 3's edits, which only touched lines before Step 2):

```bash
grep -n '^### Step 2\.1: Pool and score$' shared_config/.claude/skills/in-depth-review/SKILL.md
grep -n '^## Step 3: Filter and dedup$' shared_config/.claude/skills/in-depth-review/SKILL.md
grep -n '^### If invoked as a sub-agent' shared_config/.claude/skills/in-depth-review/SKILL.md
grep -n '^## Constraints$' shared_config/.claude/skills/in-depth-review/SKILL.md
```

All four should return exactly one match each.

- [ ] **Step 1: Write the Step 2.1 replacement to a temp file**

```bash
cat > /tmp/step-2-1.md << 'BLOCKEOF'

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
   `model: haiku`). Do not let it inherit the session model either.

**Collect the scorers exactly per [COLLECTING.md](COLLECTING.md)**: here, the "agent" is the
scorer you just launched, and a scorer that never reports leaves its finding `unscored: true`
with `confidence: null` (see below) instead of any `roles_missing`-style list.

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

Score each finding against the rubric in [SCORING.md](SCORING.md), including the citation
hard-cap and the ticket/motivation scoring specifics there.
BLOCKEOF
```

- [ ] **Step 2: Verify the temp file's line count**

```bash
wc -l /tmp/step-2-1.md
```

Expected: `60` (a leading blank line + 59 content lines, no trailing blank — the trailing separation before "## Step 3" comes from the original file's own existing blank line at that point, which the delete in Step 3 below does not touch).

- [ ] **Step 3: Delete the old Step 2.1 section**

```bash
sed -i '' '/^### Step 2\.1: Pool and score$/,/^## Step 3: Filter and dedup$/{/^## Step 3: Filter and dedup$/!d;}' shared_config/.claude/skills/in-depth-review/SKILL.md
```

- [ ] **Step 4: Splice in the replacement**

```bash
sed -i '' '/^`roles_missing` is a required field of the Step 4 output even when empty\.$/r /tmp/step-2-1.md' shared_config/.claude/skills/in-depth-review/SKILL.md
```

- [ ] **Step 5: Verify**

```bash
grep -n '^### Step 2\.1: Pool and score$\|^## Step 3: Filter and dedup$\|Score each finding against the rubric in \[SCORING.md\]' shared_config/.claude/skills/in-depth-review/SKILL.md
```

Expected: all three patterns match, exactly once each.

```bash
grep -n 'Collect the scorers the same way you collected the roles' shared_config/.claude/skills/in-depth-review/SKILL.md
```

Expected: no output (confirms the old duplicated collection-protocol text is gone, not just supplemented).

- [ ] **Step 6: Write the Step 4 replacement to a temp file**

```bash
cat > /tmp/step-4-output.md << 'BLOCKEOF'

### If invoked as a sub-agent (by `review-and-fix` or `pr-review`)

See [OUTPUT-JSON.md](OUTPUT-JSON.md) for the exact JSON shape and field semantics.

### If invoked directly by the user

See [OUTPUT-CHAT.md](OUTPUT-CHAT.md) for the chat report template and formatting rules.
BLOCKEOF
```

- [ ] **Step 7: Delete from the sub-agent branch header through end of file**

This removes both output branches AND the `Constraints` section in one pass, since nothing survives after the sub-agent branch header in the original file's structure except those two things, and neither has any replacement content living below it:

```bash
sed -i '' '/^### If invoked as a sub-agent/,$d' shared_config/.claude/skills/in-depth-review/SKILL.md
```

- [ ] **Step 8: Splice in the Step 4 replacement**

```bash
sed -i '' '/^3\. Confidence descending$/r /tmp/step-4-output.md' shared_config/.claude/skills/in-depth-review/SKILL.md
```

- [ ] **Step 9: Verify Constraints is gone and the new pointers are present**

```bash
grep -c 'Constraints' shared_config/.claude/skills/in-depth-review/SKILL.md
```

Expected: `0`.

```bash
grep -n 'OUTPUT-JSON.md\|OUTPUT-CHAT.md' shared_config/.claude/skills/in-depth-review/SKILL.md
```

Expected: two matches (one per pointer link).

```bash
grep -n 'inherit the session model' shared_config/.claude/skills/in-depth-review/SKILL.md
```

Expected: two matches — the original reviewer-model-policy line (Task 3) and the new scorer clause you added in Step 1 of this task. If you see only one, the scorer clause was lost; re-check Step 1's temp file content.

- [ ] **Step 10: Verify the final line count**

```bash
wc -l shared_config/.claude/skills/in-depth-review/SKILL.md
```

Expected: `425`. This is the number the spec derives from a from-scratch, gap-free tiling of the original 1,124-line file — if you land anywhere else, do not treat it as "close enough." Re-open the spec's Line budget table, find which row's range doesn't match what actually happened, and fix that specific step rather than adjusting content to hit the number.

- [ ] **Step 11: Commit**

```bash
git add shared_config/.claude/skills/in-depth-review/SKILL.md
git commit -m "$(cat <<'EOF'
refactor(claude,skill): in-depth-review: rewrite Step 2.1/Step 4 and drop Constraints

Step 2.1's scorer-collection duplicate now points at COLLECTING.md
(same protocol as Step 1's role collection); the scoring rubric moved
to SCORING.md. Step 4's two output branches point at OUTPUT-JSON.md /
OUTPUT-CHAT.md. Constraints deleted -- every bullet restated content
stated in full elsewhere, except the scorer model-inheritance clause,
which is preserved by adding it to Step 2.1's scorer-launch instruction
rather than dropped. SKILL.md is now 425 lines, down from 1,124.
EOF
)"
```

---

### Task 5: Full verification pass

**Files:**
- Read only: everything under `shared_config/.claude/skills/in-depth-review/`, plus `shared_config/.claude/skills/pr-review/SKILL.md`, `shared_config/.claude/skills/review-and-fix/SKILL.md`, `shared_config/.claude/skills/gh-style-review/SKILL.md`, `shared_config/.claude/agents/pr-review-finder-indepth.md`, `shared_config/.claude/agents/pr-review-finder-indepth-deep.md`.

**Interfaces:**
- Consumes: the finished state of Tasks 1-4.
- Produces: either a clean bill of health, or a fix (in which case, apply it, re-run the specific check that failed, and commit the fix with a message describing what was wrong).

This task runs the spec's full "Verification" section end to end. It should find nothing, since Tasks 1-4 already verified each of their own pieces — this is the integration check that confirms nothing was missed across task boundaries.

- [ ] **Step 1: Line count and role table**

```bash
wc -l shared_config/.claude/skills/in-depth-review/SKILL.md
```

Expected: `425`.

```bash
grep -c '^| [0-9]\{1,2\} | .* | `[a-zA-Z0-9. -]*` | `roles/' shared_config/.claude/skills/in-depth-review/SKILL.md
```

Expected: `12`.

- [ ] **Step 2: Every link resolves, and stays one level deep**

```bash
grep -oE '\[[^]]+\]\(([^)]+\.md)\)' shared_config/.claude/skills/in-depth-review/SKILL.md
```

For every path printed, confirm the file exists:

```bash
ls shared_config/.claude/skills/in-depth-review/COLLECTING.md shared_config/.claude/skills/in-depth-review/SCORING.md shared_config/.claude/skills/in-depth-review/OUTPUT-JSON.md shared_config/.claude/skills/in-depth-review/OUTPUT-CHAT.md shared_config/.claude/skills/in-depth-review/roles/_common-fragment.md
```

Expected: all five listed with no "No such file" errors.

```bash
grep -l '\.md)' shared_config/.claude/skills/in-depth-review/roles/*.md shared_config/.claude/skills/in-depth-review/COLLECTING.md shared_config/.claude/skills/in-depth-review/SCORING.md shared_config/.claude/skills/in-depth-review/OUTPUT-JSON.md shared_config/.claude/skills/in-depth-review/OUTPUT-CHAT.md
```

Expected: no output (none of the new files link to another new file).

- [ ] **Step 3: Every new file has exactly one H1**

```bash
grep -c '^# ' shared_config/.claude/skills/in-depth-review/roles/*.md shared_config/.claude/skills/in-depth-review/COLLECTING.md shared_config/.claude/skills/in-depth-review/SCORING.md shared_config/.claude/skills/in-depth-review/OUTPUT-JSON.md shared_config/.claude/skills/in-depth-review/OUTPUT-CHAT.md
```

Expected: every file reports `1`, **except `OUTPUT-CHAT.md`, which reports `2`.** Its second match is `# In-Depth Review — <SCOPE_DESCRIPTION>` inside the chat-report template's own code fence -- literal example text showing what the rendered report looks like, not a second heading on the file itself. Confirm this by running `grep -n '^# ' OUTPUT-CHAT.md` and checking the second match sits inside a fenced code block, not a real Markdown heading.

- [ ] **Step 4: Field names survived**

```bash
grep -c 'roles_missing' shared_config/.claude/skills/in-depth-review/SKILL.md
```

Expected: at least `1` (Step 2.0's usage).

```bash
grep -c 'unscored' shared_config/.claude/skills/in-depth-review/SKILL.md
```

Expected: at least `1` (Step 2.1's bookkeeping). `SCORING.md` itself never uses this word -- it's purely the confidence rubric handed to a scorer, not the orchestrator-side bookkeeping for a scorer that never reports, which correctly stays in `SKILL.md`. Don't expect it in `SCORING.md`.

- [ ] **Step 5: The pre-existing typo survived unfixed**

```bash
grep -c "makes it run\.\*\*" shared_config/.claude/skills/in-depth-review/SKILL.md
```

Expected: `1`. If this is `0`, the stray `**` in Role #7's gate text was accidentally "fixed" during the splice in Task 3 — restore it exactly, since fixing it is out of scope for this restructure (see the spec's "Known follow-ups").

- [ ] **Step 6: Em dashes survived in verbatim content**

```bash
grep -c "—" shared_config/.claude/skills/in-depth-review/SKILL.md
```

Expected: greater than `0`. If this is `0`, verbatim content was flattened to ASCII somewhere during the splices — find where and restore the original character (check the "Conditional role gates" role headers and the model-policy paragraph first, since those are where the source em dashes live).

- [ ] **Step 7: Re-read every dependent file in full**

Read `shared_config/.claude/skills/pr-review/SKILL.md`, `shared_config/.claude/skills/review-and-fix/SKILL.md`, `shared_config/.claude/skills/gh-style-review/SKILL.md`, `shared_config/.claude/agents/pr-review-finder-indepth.md`, and `shared_config/.claude/agents/pr-review-finder-indepth-deep.md` completely. For each reference any of them makes to `in-depth-review` — its flags, role numbers, category names, or JSON field names — confirm it still resolves correctly against the finished `SKILL.md`. This is a manual read-and-compare, not a scriptable check; there is no expected command output here, only your own confirmation that nothing broke.

- [ ] **Step 8: If everything above passed clean, there is nothing to commit**

No commit for this task unless Step 5 or Step 6 (or the manual read in Step 7) turned up something to fix, in which case: fix it, re-run the specific check that failed, and commit with a message naming what was wrong, e.g. `fix(claude,skill): in-depth-review: restore em dash flattened during Step 1 splice`.
