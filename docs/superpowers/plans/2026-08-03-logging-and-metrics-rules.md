# Logging Levels and Metric Emission Rules Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two AGENTS.md rules that stop Claude logging bugs below error level and emitting metrics nobody consumes, then make the level rule enforceable by the error-handling reviewer.

**Architecture:** Two prose edits to two files. Task 1 adds two sections to the global AGENTS.md, which must land together because the second refers to the first. Task 2 adds one rubric item to Reviewer Role #8 in `in-depth-review`, which is the specialist role whose rubric is self-contained and so does not inherit AGENTS.md rules.

**Tech Stack:** Markdown only. No executable code, so no test runner. The red/green cycle is a `grep` that must miss before an edit and hit after it.

**Spec:** `docs/superpowers/specs/2026-08-03-logging-and-metrics-agents-rules-design.md`

## Global Constraints

- **Two target files, nothing else:**
  - `shared_config/.claude/AGENTS.md` (Task 1)
  - `shared_config/.claude/skills/in-depth-review/SKILL.md` (Task 2)
- **Repo root:** `/Users/melvin/.melvin/config`. Run `git` from inside it. Never use top-level `git -C` (a hook denies it).
- **NO em-dash (`—`) or en-dash (`–`) in any text this plan adds, in either file.** This is absolute. The em-dash has no permitted use. The carve-out list in AGENTS.md's "Code comments" covers the ASCII hyphen `-` and the colon `:` only, meaning hyphenated words, CLI flags, ranges, label prefixes, and ratios. Commit `4a0b06d` invented a "definition dash" exemption the rule never granted.
- **No semicolons in added prose.** "Code comments" says "Avoid semicolons."
- **No ` - ` (space-hyphen-space) and no clause-splitting `:` as clause joiners.** Split into two sentences.
- **Wrapping differs per file, and getting it backwards is the likeliest error:**
  - `AGENTS.md` does **NOT** wrap. One long line per paragraph and per bullet. 43 of its lines already exceed 200 characters.
  - `in-depth-review/SKILL.md` **DOES** wrap. Match the sibling rubric items, which break near 97 columns with a three-space continuation indent.
- **Never rewrap a line you did not otherwise change.**
- **Heading level in AGENTS.md is `##`.** Every section uses it. The source material this came from used `###`.
- **Stage only the one file each task names.** Never `git add -A`.
- **Run each `git` command ALONE.** No pipes, no `&&`, no `;`. A hook denies chained git commands here.
- **Do NOT use command substitution** such as `git commit -m "$(cat <<'EOF' ...)"`. A hook denies it. Write the message to a file and use `git commit -F <file>`, or use a plain multi-line `-m "..."`.
- **A literal semicolon in a Bash command trips the multi-command hook.** To grep for one, use `rg -c '\x3B'`.

---

### Task 1: Add the Logging levels and Emitting metrics sections

Both sections in one task and one commit. The metrics section ends a bullet with "See **Logging levels** above", so shipping either alone leaves a dangling reference.

**Files:**
- Modify: `shared_config/.claude/AGENTS.md` (insert between the TypeScript section and the Plain ASCII heading, currently lines 118-120)

**Interfaces:**
- Consumes: nothing.
- Produces: two `##` sections named exactly `Logging levels` and `Emitting metrics`, in that order, both above `## Plain ASCII in authored prose`. Task 2's rubric item restates the level rule in its own words and does not quote these, so it depends on the wording only in spirit.

- [ ] **Step 1: Record the baselines (the "failing test")**

Run each from `/Users/melvin/.melvin/config`.

```
rg -c '^## Logging levels' shared_config/.claude/AGENTS.md
```
Expected: no output, exit 1. The section does not exist yet.

```
rg -c '^## Emitting metrics' shared_config/.claude/AGENTS.md
```
Expected: no output, exit 1.

```
grep -c '—' shared_config/.claude/AGENTS.md
```
Expected: `15`. Write this number down. Step 4 asserts it is unchanged.

```
rg -c '\x3B' shared_config/.claude/AGENTS.md
```
Expected: `8`. Write it down.

```
rg -c 'calmLogger' shared_config/.claude/AGENTS.md
```
Expected: no output, exit 1.

If any baseline differs, stop and report. The file has moved on from the spec.

- [ ] **Step 2: Insert both sections**

Find this exact text. It spans the end of the TypeScript section and the next heading, which pins the insertion position.

```
When a cast looks unavoidable, that is usually a signal that a type is wrong further upstream or that a boundary is missing its validation step. Fix the upstream type or add the guard. Do not paper over it at the use site.

## Plain ASCII in authored prose
```

Replace it with the following. **Every paragraph and every bullet below is ONE long line.** Do not wrap them. The only newlines are the blank lines between blocks.

```
When a cast looks unavoidable, that is usually a signal that a type is wrong further upstream or that a boundary is missing its validation step. Fix the upstream type or add the guard. Do not paper over it at the use site.

## Logging levels

Error level is what monitors are keyed to. Anything a human needs to see and act on emits at error, even when the code handled it and recovered. A bug logged at warn is a bug nobody is paged for.

Warn is for a condition you expected, handled, and nobody needs to act on. If you can name who acts on a warn and what they would do about it, it is an error.

A `catch` block that logs and continues must say why continuing is correct. Swallowing a failure converts it into silent wrong behavior, which is harder to diagnose than a crash. If the fallback path is genuinely fine, a comment naming why belongs there. If it is not fine, the log is an error and the failure propagates.

## Emitting metrics

Emit a metric only when something will consume it. Be able to name the consumer before adding the counter, meaning a dashboard, a monitor, or an experiment readout. A cron is the clear yes case, since its whole point is that nobody watches it run. Unread telemetry is not free. It persists indefinitely, and it advertises a monitoring story that does not exist, so a later reader trusts a signal no alert is keyed to.

- A counter next to an error-level log for the same failure usually fails this test (`calmLogger.error` in Calm repos). The error log is already the loud, alertable signal, so the counter earns its place only if you specifically need the aggregate volume as well. See **Logging levels** above.
- Prefer deriving a number from data already kept, such as a table you can count or an event already emitted, over a new counter whose only purpose is to be counted.
- Splitting one condition across several counters needs a reason a reader can check. If two counters exist so a dashboard can tell two causes apart, say which dashboard. Otherwise emit one, or none.

## Plain ASCII in authored prose
```

- [ ] **Step 3: Verify the sections landed in the right place**

```
rg -c '^## Logging levels' shared_config/.claude/AGENTS.md
```
Expected: `1`.

```
rg -c '^## Emitting metrics' shared_config/.claude/AGENTS.md
```
Expected: `1`.

```
rg -n '^## Logging levels|^## Emitting metrics|^## Plain ASCII' shared_config/.claude/AGENTS.md
```
Expected: three lines whose numbers ascend in exactly that order. Ordering is a requirement, because the metrics bullet says Logging levels is above it.

```
rg -c 'calmLogger' shared_config/.claude/AGENTS.md
```
Expected: `1`. Exactly one stack note, in the first metrics bullet.

```
rg -c 'When a cast looks unavoidable' shared_config/.claude/AGENTS.md
```
Expected: `1`. The find-text spanned the TypeScript section's closing paragraph, so this confirms
that paragraph survived rather than being consumed by the replacement.

- [ ] **Step 4: Verify you introduced no banned punctuation**

```
grep -c '—' shared_config/.claude/AGENTS.md
```
Expected: `15`, unchanged from Step 1. Any rise means an em-dash slipped in.

```
rg -c '\x3B' shared_config/.claude/AGENTS.md
```
Expected: `8`, unchanged from Step 1.

```
rg -c 'Datadog' shared_config/.claude/AGENTS.md
```
Expected: no output, exit 1. The spec deliberately does not name the backend.

```
git status --porcelain shared_config/
```
Expected: exactly one line, ` M shared_config/.claude/AGENTS.md`. Confirms nothing else under
`shared_config/` was touched. Scope the path so the untracked `docs/` tree, which is unrelated to
this plan, does not appear.

Then read the two new sections once and confirm no sentence joins two independent clauses with a dash or a splitting colon, and that no paragraph or bullet got wrapped.

- [ ] **Step 5: Commit**

```
git add shared_config/.claude/AGENTS.md
```

```
git commit -m "observability: ban unread metrics and bugs logged below error

Two rules Claude keeps breaking. It reaches for warn on real failures, and
warn is not what the monitors watch, so the bug is never paged. And it adds
counters nobody reads, which persist forever and advertise a monitoring
story that does not exist.

Logging levels defines both sides of the error/warn boundary and gives a
test for warn. If you can name who acts on it, it is an error. It also
covers log-and-continue, which has to justify continuing.

Emitting metrics requires a named consumer before the counter exists. A
cron is the clear yes case.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Add the log level item to Reviewer Role #8

**Files:**
- Modify: `shared_config/.claude/skills/in-depth-review/SKILL.md` (Role #8 rubric, currently lines 543-565)

**Interfaces:**
- Consumes: Task 1's `Logging levels` section in spirit. The rubric item restates the rule in its own words rather than quoting it, so there is no textual dependency to keep in sync.
- Produces: nothing downstream. This is the last task.

- [ ] **Step 1: Record the baselines**

```
rg -c '^7\. Log level' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: no output, exit 1.

```
rg -c '^6\. Context preservation' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: `1`. This is the item your insert goes after.

```
grep -c '—' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: `82`. Write it down. Step 4 asserts it is **unchanged**, not incremented.

```
rg -c 'EIGHT TO TWELVE' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: `1`. The frontmatter role count, which must not change.

- [ ] **Step 2: Append item 7 to the rubric**

Find this exact text. It spans item 6's last line, the blank line, and the closing paragraph, so the insert cannot land outside the rubric.

```
   without wrapping the original; using `throw new Error(e.message)` instead of `cause: e`).

Skip "you could define a custom error class". Skip pedantic typing-only nits. Only flag
```

Replace it with:

```
   without wrapping the original; using `throw new Error(e.message)` instead of `cause: e`).
7. Log level. A failure that needs human attention logged below error level. Error is what
   monitors are keyed to, so a bug logged at `warn` is a bug nobody is paged for.

Skip "you could define a custom error class". Skip pedantic typing-only nits. Only flag
```

Note there is no blank line between item 6 and item 7. Items 1 through 6 are consecutive lines, and item 7 joins them. The blank line stays where it is, between item 7 and the "Skip" paragraph.

- [ ] **Step 3: Do NOT give item 7 an em-dash, and do NOT sweep its siblings**

This step is a decision to honor, not a command to run. Read it before Step 4.

Items 1 through 6 each read `Term — definition`. Item 7 deliberately reads `Log level.` with a period instead. It **will** look inconsistent beside them. That is expected and correct for this plan.

- Do not "fix" the inconsistency by giving item 7 a dash.
- Do not sweep items 1 through 6 to match item 7. A separate spec covers the 216 existing em-dashes across five files.

- [ ] **Step 4: Verify**

```
rg -c '^7\. Log level' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: `1`.

```
rg -c '^6\. Context preservation' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: `1`. Confirms item 6 survived and was not overwritten.

```
rg -n '^7\. Log level|^Skip "you could define' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: two lines, with the item 7 line number LOWER. Confirms the item landed inside the rubric rather than after the closing paragraph.

```
grep -c '—' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: `82`, unchanged from Step 1. **This is the check most likely to fail.** Every sibling item has a dash, so the shape invites copying one into item 7. Any rise means you added one.

```
rg -c 'EIGHT TO TWELVE' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: `1`. Confirms the frontmatter role count is untouched. Adding an item to a role does not add a role.

```
git status --porcelain shared_config/
```
Expected: exactly one line, ` M shared_config/.claude/skills/in-depth-review/SKILL.md`. Task 1's change is already committed, so it must not appear.

Then read item 7 beside items 1 through 6 and confirm the wrap and the three-space continuation indent match.

- [ ] **Step 5: Commit**

```
git add shared_config/.claude/skills/in-depth-review/SKILL.md
```

```
git commit -m "in-depth-review: flag failures logged below error level

Role #8 carries a self-contained rubric and never reads AGENTS.md, so the
new Logging levels rule does not reach it on its own. Item 7 adds the check.

Only the under-leveling direction is here. A routine handled condition
logged at error contradicts the AGENTS.md rule directly, so Role #1 catches
that side.

Item 7 uses a period where its siblings use an em-dash. The dash is banned
and the siblings are a known violation, swept separately.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## After both tasks: a sync is required, and the USER runs it

Neither edit takes effect globally on commit. Verified: `~/.claude/AGENTS.md` is a regular 25 KB
file, not a symlink, and `~/.claude/skills/*` are real directories rather than links. Both are
copies that `melvin-config claude sync` produces. So committing to `shared_config/` changes the
source of truth and nothing else.

**No task in this plan runs the sync, and no implementer should try.** That command writes
`~/.claude`, which the sandbox denies, and `allowUnsandboxedCommands` is off, so retrying with
`dangerouslyDisableSandbox` fails identically. Per AGENTS.md, ask rather than retry.

When both tasks are committed, tell the user to run this themselves in a regular terminal:

```
melvin-config claude sync
```

Then restart Claude Code so the new AGENTS.md sections and the Role #8 item load. Until then the
rules are committed but inert, and a review run would not apply them.

Note that this repo's own `shared_config/.claude/skills/` is also picked up as directory-scoped
skills while working inside this repo, which is why the scoped `shared_config:in-depth-review`
variant appears alongside the global one. That path does not need a sync, but it only applies to
work done inside this repo, so it is not a substitute.

## Spec coverage

| Spec item | Task |
|---|---|
| Section 1, Logging levels, exact text | Task 1, Step 2 |
| Section 2, Emitting metrics, exact text | Task 1, Step 2 |
| Section 3, Role #8 item 7, exact text | Task 2, Step 2 |
| Placement: both sections above Plain ASCII, in order | Task 1, Steps 2 and 3 |
| Placement: item 7 after item 6, before the Skip paragraph | Task 2, Steps 2 and 4 |
| Decision: no wrapping in AGENTS.md | Task 1, Step 2 and Global Constraints |
| Decision: item 7 wraps, matching siblings | Task 2, Step 2 and Step 4 read-through |
| Decision: no em-dash anywhere | Global Constraints, Task 1 Step 4, Task 2 Steps 3 and 4 |
| Decision: `##` heading level | Task 1, Step 2 (literal text) and Step 3 |
| Decision: exactly one `calmLogger` stack note | Task 1, Steps 1 and 3 |
| Decision: Datadog not named | Task 1, Step 4 |
| Decision: role count untouched | Task 2, Steps 1 and 4 |
| Verification 1-8 (AGENTS.md) | Task 1, Steps 1, 3, 4 |
| Verification 9-14 (in-depth-review) | Task 2, Steps 1, 4 |
| Follow-up spec, the 216-em-dash sweep | No task. Task 2 Step 3 forbids doing it here. |
| Out of scope items | No task. Global Constraints limit the plan to two files. |
| Making the rules actually load | No task, by necessity. See **After both tasks**. The sync escapes the sandbox, so the user runs it. |

## Note on commit shape

Precedent `0c2e514` ("typescript: ban casts in favour of type guards, add a reviewer role for it")
did this same shape, an AGENTS.md section plus a reviewer role, as ONE commit. This plan uses two
so each half gets its own review gate, since a reviewer could reasonably accept the rules while
rejecting the rubric wording. Squashing the two at the end reproduces `0c2e514`'s shape exactly and
is a reasonable finish.
