# open-ticket Skill Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an `open-ticket` skill that turns a requirement in prose into the Jira issues for it, after reading the repo and after checking that the work is not already filed or already shipped.

**Architecture:** Four markdown files under `shared_config/.claude/skills/open-ticket/`, following the house split where `SKILL.md` keeps the pipeline, the doctrine, the decision logic and the constraints, and three reference files hold the literal artifacts. One hook change plus one test change stop the skill running where its human approval gate cannot reach a human. A new prose-contract test asserts the load-bearing rules are actually stated in the files, so a later edit cannot silently drop one.

**Tech Stack:** Markdown skill files. Python 3 for the hook. Bash plus `jq` for tests. GitHub Actions for CI. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-08-23-open-ticket-skill-design.md`

## Global Constraints

Every task's requirements implicitly include this section.

**Prose voice**, from `shared_config/.claude/skills/writing-work-docs/SKILL.md` and `AGENTS.md`. These govern every line of markdown written by this plan.

- No clause joiners. No ` - ` (space-hyphen-space) and no `:` splitting a claim from its elaboration. Write two sentences. Hyphenated words, CLI flags, ranges, a label at the start of a line (`Note:`, `Step 0:`), times, ratios, and anything inside code or a path are untouched.
- Plain ASCII only. `->` not the arrow glyph, `...` not the ellipsis glyph, `>=` and `<=`, `x` not the multiplication sign, straight quotes. Two sentences instead of an em dash.
- Banned words, complete: outright; crucial, pivotal, vital, paramount; comprehensive, holistic; intricate, nuanced; meticulous; granular; utilize; facilitate; foster, cultivate; harness, unlock, empower, elevate; streamline; showcase; surface; socialize, operationalize, ideate; moreover, furthermore, additionally; notably, importantly, crucially; essentially, fundamentally, ultimately, arguably; landscape, realm, testament; myriad, plethora; journey, synergy; game-changer; delve, dive into; unpack; circle back; robust, powerful, seamless, seamlessly; ensure, enhance; cutting-edge. Watch `surface` and `ensure` in particular. Use say, report, confirm, check.
- No rule of three. No two adjacent sentences built the same way with the second inverting the first. No closing sentence that only assigns weight. Nothing hinged on `rather than`, `instead of`, `not X but Y`.
- Every rule is followed by the specific bad outcome it prevents. A rule with no named consequence does not survive review in this repo.

**Reference file convention.** SCREAMING-KEBAB with `.md`, one hyphen at most. Every reference file opens with a back-pointer naming the step that reads it, then the one thing not to get wrong. Model: `Step 5 of [SKILL.md](SKILL.md). Read this before the write, not after the check fails.`

**Site constants**, exact. `cloudId` is `36dbf4da-8d98-4bfd-a6fe-e80caa66a434` for `calmdotcom.atlassian.net`. Project `GRO` ("Grow"), numeric id `10057`, company-managed, `style: classic`. The key `WMP` does not exist on this site and must never appear in an example.

**Field ids**, exact, never paraphrased.

| id | Name |
|---|---|
| `customfield_10028` | Story Points |
| `customfield_10021` | Sprint |
| `customfield_10503` | Eng Weeks, Epic only |
| `customfield_10014` | Epic Link, NEVER WRITE |
| `customfield_10016` | Story point estimate, NEVER WRITE |

**Issue type spellings**, exact. `Epic` 10000, `Story` 10004, `Task` 10013, `Subtask` 10014, `Bug` 10015, `Bug Subtask` 10032. The subtask type this skill emits is `Bug Subtask`. `Subtask` is one word.

**Abort codes.** `JIRA_UNAVAILABLE_NO_TOOLING`, `JIRA_WRITE_DENIED`, `DUPLICATE_FOUND: <KEY> (status <S>)`, `GATE_UNREACHABLE_NO_HUMAN`.

**Bash constraints on every command written into a skill file**, from `AGENTS.md`. `git` and `gh` run outside the sandbox and must run alone with no pipe and no chain, redirected to a file under `/tmp/claude/` and read back in a separate call. No top-level `git -C`. No `cd x && ...` combined with any redirect. No inline scripting. No command substitution. No `$TMPDIR` in any form. Redirects go last. One command per call.

**Scratch paths.** Literal `/tmp/claude/open-ticket-<slug>-<what>.<ext>`. Never `$TMPDIR`.

**Wrap tolerance in every verification grep.** Skill prose is hand-wrapped, so a needle containing a literal space can straddle a newline and a plain `grep` for it returns zero. A zero-hit check that can never hit is a guaranteed pass and tests nothing. Every contract assertion in this plan flattens the file first. The `flatten()` helper in Task 2 is the single copy.

**Commit messages.** Follow the repo's existing subject style, `feat(claude,skill): ...` and `fix(claude,hook): ...`, seen in `git log`. End every commit message with the trailer `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`.

---

## File Structure

| File | Responsibility |
|---|---|
| `shared_config/.claude/skills/open-ticket/SKILL.md` | Frontmatter, pipeline, doctrine, Steps 0 through 12, abort codes, constraints |
| `shared_config/.claude/skills/open-ticket/TEMPLATES.md` | The five description templates, read at Step 8 |
| `shared_config/.claude/skills/open-ticket/CREATE-FIELDS.md` | Per-type create recipes, create-screen table, field ids, write shapes, read at Step 10 |
| `shared_config/.claude/skills/open-ticket/DEDUP-QUERIES.md` | Q0 through Q5, the quoting recipe, the git commands, read at Step 5 |
| `shared_config/.claude/hooks/deny-review-in-workflow.py` | Modify `DENIED_SKILLS` and its comment block |
| `tests/deny_review_in_workflow_test.sh` | Modify, add `open-ticket` cases including both word boundaries |
| `tests/open_ticket_contract_test.sh` | Create, asserts the load-bearing rules are stated in all four skill files |
| `.github/workflows/tests.yml` | Modify, add one job for the new test |

Task 1 is independent of the skill files and can be reviewed alone. Tasks 2 through 5 build the files from leaf to root, so each one's contract assertions land with the file they describe. Task 6 wires CI and installs.

---

## Task 1: Stop open-ticket running where its gate cannot reach a human

**Files:**
- Modify: `shared_config/.claude/hooks/deny-review-in-workflow.py:57` and its comment block above line 57
- Modify: `tests/deny_review_in_workflow_test.sh` (append cases near the existing deny block at lines 45-55)

**Interfaces:**
- Consumes: nothing from other tasks
- Produces: `DENIED_SKILLS` now contains the literal string `"open-ticket"`. No other task reads this.

**Why this task exists, and why the reason differs from every other entry in that tuple.**

The existing four entries (`in-depth-review`, `review-and-fix`, `pr-review`, `work-on`) are denied because a workflow agent has no `Agent` tool, so a skill whose value comes from its own fan-out has nothing to fan out with. That reason does **not** apply cleanly to `open-ticket`. Its Step 4 fan-out has a documented inline fallback for small work, so losing the `Agent` tool degrades it gracefully rather than gutting it.

The real reason is Step 9. `open-ticket` creates Jira issues and gates every creation on one human approval. **A workflow agent cannot present that gate to a human.** Inside a workflow it would either block on approval that can never arrive, or create issues with nobody having approved them, which is the exact failure the whole design exists to prevent. Write that reason into the comment block. The file's existing comments justify each entry individually, and an entry whose stated reason is wrong invites a later reader to remove it.

- [ ] **Step 1: Write the failing test cases**

Append to `tests/deny_review_in_workflow_test.sh`, after the existing `deny_work_on` assertion at line 55:

```bash
# open-ticket is denied for a different reason than the four above. It does not
# lose a fan-out inside a workflow. It loses the human who approves the Jira
# creation, so it would either block forever or create unapproved issues.
assert_eq "deny_open_ticket" "deny" \
  "$(decision script "await agent({ skill: 'open-ticket', prompt: req })")"
assert_eq "deny_open_ticket_in_name" "deny" \
  "$(decision name "open-ticket")"
assert_eq "deny_open_ticket_capitalized" "deny" \
  "$(decision script "run Open-Ticket for this requirement")"

# BOUNDARY_BEFORE excludes letters and digits only, so a leading hyphen still
# matches and a prefixed name denies.
assert_eq "deny_nightly_open_ticket" "deny" \
  "$(decision script "await agent({ skill: 'nightly-open-ticket' })")"

# BOUNDARY_AFTER also excludes a hyphen, so a longer name sharing the prefix
# falls through. This is what lets a future open-ticket-validate workflow run.
assert_eq "silent_open_ticket_validate" "silent" \
  "$(decision script "await agent({ skill: 'open-ticket-validate' })")"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/deny_review_in_workflow_test.sh`

Expected: FAIL on the first new case, printing

```
[deny_open_ticket] expected: deny
             actual:   silent
```

The three boundary cases after it are not reached, because `assert_eq` exits on the first failure.

- [ ] **Step 3: Add the entry to DENIED_SKILLS**

Change line 57 from:

```python
DENIED_SKILLS = ("in-depth-review", "review-and-fix", "pr-review", "work-on")
```

to:

```python
DENIED_SKILLS = ("in-depth-review", "review-and-fix", "pr-review", "work-on", "open-ticket")
```

- [ ] **Step 4: Write the reason into the comment block**

Add this paragraph to the comment block above `DENIED_SKILLS`, after the existing paragraph about `gh-style-review` being deliberately absent. Keep it in the file's existing voice, which is full sentences with no clause joiners.

```python
# open-ticket is here for a different reason than the four fan-out skills. It
# does have a fan-out, and losing it inside a workflow costs coverage rather
# than gutting the run, because Step 4 has a documented inline fallback. What a
# workflow agent cannot do is present Step 9's approval gate to a human. That
# gate is the only control on a Jira creation, which has no rollback. Inside a
# workflow the skill would either block on approval nobody can give, or create
# issues nobody approved. Do not remove this entry on the grounds that the
# fan-out survives. The gate is the reason.
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `bash tests/deny_review_in_workflow_test.sh`

Expected: PASS, no output, exit 0.

- [ ] **Step 6: Commit**

```bash
git add shared_config/.claude/hooks/deny-review-in-workflow.py tests/deny_review_in_workflow_test.sh
```

```bash
git commit -m "fix(claude,hook): deny open-ticket in a workflow, where its approval gate has no human

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: TEMPLATES.md and the contract test harness

**Files:**
- Create: `shared_config/.claude/skills/open-ticket/TEMPLATES.md`
- Create: `tests/open_ticket_contract_test.sh`

**Interfaces:**
- Consumes: nothing from other tasks
- Produces: `tests/open_ticket_contract_test.sh` defining `flatten()`, `SKILL_DIR`, and the four file path variables `SKILL_MD`, `TEMPLATES_MD`, `CREATE_FIELDS_MD`, `DEDUP_MD`. Tasks 3, 4 and 5 append their assertions to this same file and reuse `flatten()`. Do not redefine it.

`TEMPLATES.md` holds five templates. Three come from the request verbatim. Two are additions the request did not specify, and the spec names them.

- [ ] **Step 1: Write the failing test**

Create `tests/open_ticket_contract_test.sh`:

```bash
#!/usr/bin/env bash
# Asserts the load-bearing rules are actually stated in the open-ticket skill
# files. Every rule checked here is one where a silent omission produces a
# wrong Jira write or a duplicate ticket, so a later edit that drops one must
# fail CI rather than pass quietly.
#
# Prose in these files is hand-wrapped, so a needle holding a literal space can
# straddle a newline. A plain grep for it would return zero and the assertion
# would pass without ever being able to fail. flatten() collapses the file to
# one space-normalized line first, which removes the whole class of problem.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SKILL_DIR="$SCRIPT_DIR/shared_config/.claude/skills/open-ticket"

SKILL_MD="$SKILL_DIR/SKILL.md"
TEMPLATES_MD="$SKILL_DIR/TEMPLATES.md"
CREATE_FIELDS_MD="$SKILL_DIR/CREATE-FIELDS.md"
DEDUP_MD="$SKILL_DIR/DEDUP-QUERIES.md"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

for f in "$SKILL_MD" "$TEMPLATES_MD" "$CREATE_FIELDS_MD" "$DEDUP_MD"; do
  if [[ ! -s "$f" ]]; then
    echo "missing or empty: $f" >&2
    exit 1
  fi
done

flatten() { tr '\n' ' ' < "$1" | tr -s ' '; }

TEMPLATES_FLAT="$(flatten "$TEMPLATES_MD")"

# ---- TEMPLATES.md: all five templates present ----
assert_contains "tpl_has_feature_story" "## Feature Story" "$TEMPLATES_FLAT"
assert_contains "tpl_has_bug" "## Bug Fix" "$TEMPLATES_FLAT"
assert_contains "tpl_has_tech_debt" "## Technical Debt" "$TEMPLATES_FLAT"
assert_contains "tpl_has_epic" "## Epic" "$TEMPLATES_FLAT"
assert_contains "tpl_has_bug_subtask" "## Bug Subtask" "$TEMPLATES_FLAT"

# ELI5 is the section a reader with no project context uses, and it is the
# first one a drafter drops. Three templates carry it.
ELI5_COUNT="$(grep -c '^## ELI5$' "$TEMPLATES_MD" || true)"
assert_eq "tpl_eli5_on_three_templates" "3" "$ELI5_COUNT"

# The bug template's Stats block is what makes a bug ticket triageable. The
# 1-to-5 likelihood rating is the part most likely to be quietly dropped.
assert_contains "tpl_bug_has_stats" "### Stats" "$TEMPLATES_FLAT"
assert_contains "tpl_bug_has_rating" "rating from 1 to 5" "$TEMPLATES_FLAT"

# Never a table. A ticket arrives mangled more often from a table than from
# anything else, and a triage assessment is the natural table.
assert_contains "tpl_bans_tables" "Never a table" "$TEMPLATES_FLAT"

# The back-pointer convention every reference file in this repo follows.
assert_contains "tpl_back_pointer" "Step 8 of [SKILL.md](SKILL.md)" "$TEMPLATES_FLAT"

echo "open_ticket_contract_test: ok"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/open_ticket_contract_test.sh`

Expected: FAIL with `missing or empty: /Users/melvin/.melvin/config/shared_config/.claude/skills/open-ticket/SKILL.md`, exit 1. All four files are absent at this point, so the loop catches the first one.

To see the template assertions themselves fail rather than the existence loop, create the four files as empty placeholders is **not** acceptable here, because Task 5 must find `SKILL.md` absent. Accept the existence-loop failure as the red state for this task. It is a real failure of a real precondition.

- [ ] **Step 3: Write TEMPLATES.md**

Create `shared_config/.claude/skills/open-ticket/TEMPLATES.md`.

Opening line, matching the house back-pointer convention:

```markdown
Step 8 of [SKILL.md](SKILL.md). Pick the template from the issue type and the intent, then fill it. Never a table anywhere inside one. A triage assessment is the natural table and a table is the most common way a ticket arrives mangled, so every row is a bullet.
```

Then a short section on what applies to all five. Headings start at `##`, because an `h1` renders enormous next to Jira's own page title. One line per paragraph and one line per list item, however long it runs, because a hard wrap can survive conversion to ADF and arrive as ragged text. A number nobody ran is a `TODO(user):` line carrying the query, never prose.

Then the five templates, each inside a fenced block so it is copied and not paraphrased.

`## Feature Story`, verbatim from the request:

```
## Overview
[What and why in 2-3 sentences, if possible in a "As a user..." type of sentence]

## ELI5

[explain the ticket in a way so someone with no knowledge of the project understands what needs to be done]

## Acceptance Criteria
- [ ] Given [context], when [action], then [outcome]
- [ ] [Additional criteria]

## Technical Notes
- [Architecture decisions]
- [Edge cases to handle]
- [Performance considerations]

## Related Code
- [Similar implementations]
- [Relevant utilities]
```

`## Bug Fix and Security`, verbatim from the request. Keep the `### Stats` block and its `rating from 1 to 5` wording, because the contract test asserts both:

```
## Problem
[Description of the bug and impact, if possible in a "As a user..." type of sentence]

### Stats
[how many people are impacted? Are impacted users in a locked state? does this require a backfill? Is this in a legacy route? Is this in a live codepath? We also want a rating from 1 to 5 about the odds of this to trigger. 1 mean the bugs has a low probability of happening, 5 mean the bug is actively impacting users at a very high rate]

## Root Cause
[What's causing the issue - file and line reference]

## Reproduction Steps
1. [Step 1]
2. [Step 2]
3. [Observe incorrect behavior]

## Expected Behavior
[What should happen instead]

## Proposed Fix

[explain how the issue could be fixed]

## Testing
- [ ] Add unit test for [scenario]
- [ ] Add regression test
- [ ] Verify in [affected areas]

## ELI5

[explain the ticket in a way so someone with no knowledge of the project understands what needs to be done]

## Related Code
- [Where similar issues might exist]
```

`## Technical Debt`, verbatim from the request:

```
## Current State
[What exists now and why it's problematic]

## Desired State
[What the code should look like]

## Benefits
- [Improved maintainability / performance / etc.]

## Implementation Plan
1. [Phase 1 - file paths and changes]
2. [Phase 2 - file paths and changes]

## Migration Path
- [How to transition without breaking changes]

## Acceptance Criteria
- [ ] [Refactoring complete]
- [ ] [Tests still pass]
- [ ] [Performance improved by X]
```

`## Epic`, new. The spec specifies goal, children, out of scope, done. No acceptance criteria, because its children carry those:

```
## Goal
[What this initiative delivers, and why now]

## ELI5

[explain the initiative in a way so someone with no knowledge of the project understands what is being built]

## Stories
- [KEY] [summary]
- [KEY] [summary]

## Out of scope
- [What this epic does not cover]

## Done when
- [The observable condition that closes this epic]
```

`## Bug Subtask`, new. Hours to a day of work, so a full template on it is noise:

```
[One paragraph naming what this piece is and which file it touches.]

- [ ] [step]
- [ ] [step]
```

Close the file with a short section on the two limits that apply to the rendered text. `description` caps at 32000 characters on the string branch, so check the rendered length before the call. Every leaf carries at least one real path from Step 4, or says it could not find one.

- [ ] **Step 4: Run the test to verify the template assertions pass**

Run: `bash tests/open_ticket_contract_test.sh`

Expected: still FAIL at the existence loop, now naming `SKILL.md` as the first missing file, because `TEMPLATES.md` exists and `SKILL.md` does not.

Temporarily comment out the existence-loop entries for the three files not yet written, run again, confirm every `tpl_` assertion passes and the script prints `open_ticket_contract_test: ok`, then restore the loop. Do not commit with the loop commented out.

- [ ] **Step 5: Commit**

```bash
git add shared_config/.claude/skills/open-ticket/TEMPLATES.md tests/open_ticket_contract_test.sh
```

```bash
git commit -m "feat(claude,skill): open-ticket: the five ticket description templates

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: CREATE-FIELDS.md

**Files:**
- Create: `shared_config/.claude/skills/open-ticket/CREATE-FIELDS.md`
- Modify: `tests/open_ticket_contract_test.sh` (append a `CREATE_FIELDS_FLAT` block before the final `echo`)

**Interfaces:**
- Consumes: `flatten()`, `CREATE_FIELDS_MD` from Task 2
- Produces: nothing other tasks read. `SKILL.md` in Task 5 links to this file by name from Step 10.

This file is the one where a wrong value causes a real failed create or a silently wrong ticket. Everything in it is a literal to be copied, never paraphrased.

- [ ] **Step 1: Write the failing test**

Append to `tests/open_ticket_contract_test.sh`, immediately before the final `echo` line:

```bash
CREATE_FIELDS_FLAT="$(flatten "$CREATE_FIELDS_MD")"

assert_contains "cf_back_pointer" "Step 10 of [SKILL.md](SKILL.md)" "$CREATE_FIELDS_FLAT"

# Exact field ids. A paraphrase here becomes a failed create.
assert_contains "cf_story_points_id" "customfield_10028" "$CREATE_FIELDS_FLAT"
assert_contains "cf_sprint_id" "customfield_10021" "$CREATE_FIELDS_FLAT"
assert_contains "cf_eng_weeks_id" "customfield_10503" "$CREATE_FIELDS_FLAT"

# The two never-write ids, each named with its ban. Writing either one produces
# a ticket that looks right and carries the wrong linkage or the wrong estimate.
assert_contains "cf_bans_epic_link" "customfield_10014" "$CREATE_FIELDS_FLAT"
assert_contains "cf_bans_point_estimate" "customfield_10016" "$CREATE_FIELDS_FLAT"
assert_contains "cf_bans_components" "components" "$CREATE_FIELDS_FLAT"
assert_contains "cf_bans_fixversions" "fixVersions" "$CREATE_FIELDS_FLAT"

# The three two-call types. Sending description on any of these three hits a
# field that is not on the create screen.
assert_contains "cf_two_call_bug" "Bug" "$CREATE_FIELDS_FLAT"
assert_contains "cf_two_call_epic" "Epic" "$CREATE_FIELDS_FLAT"
assert_contains "cf_two_call_bug_subtask" "Bug Subtask" "$CREATE_FIELDS_FLAT"
assert_contains "cf_names_two_calls" "two calls" "$CREATE_FIELDS_FLAT"

# markdown works on create. Stating contentFormat explicitly costs nothing and
# the shared enum text hedges about defaults.
assert_contains "cf_content_format" "contentFormat" "$CREATE_FIELDS_FLAT"
assert_contains "cf_markdown_on_create" "markdown" "$CREATE_FIELDS_FLAT"

# parent for every level, including Story to Epic. GRO is company-managed and
# still uses the unified parent field.
assert_contains "cf_parent_every_level" "parent" "$CREATE_FIELDS_FLAT"

# The subtask type this skill emits, spelled exactly.
assert_contains "cf_subtask_one_word" "Subtask" "$CREATE_FIELDS_FLAT"

# The description cap. A longer body fails the call.
assert_contains "cf_description_cap" "32000" "$CREATE_FIELDS_FLAT"

# WMP does not exist on this site. An example using it teaches a wrong key.
WMP_HITS="$(grep -c 'WMP' "$CREATE_FIELDS_MD" || true)"
assert_eq "cf_no_wmp_examples" "0" "$WMP_HITS"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/open_ticket_contract_test.sh`

Expected: FAIL at the existence loop naming `SKILL.md`. Comment out the `SKILL_MD` and `DEDUP_MD` entries in the loop only, run again, and expect FAIL on `cf_back_pointer` with `Expected to find "Step 10 of [SKILL.md](SKILL.md)"` against empty output, because `CREATE-FIELDS.md` does not exist.

- [ ] **Step 3: Write CREATE-FIELDS.md**

Create the file. Opening back-pointer:

```markdown
Step 10 of [SKILL.md](SKILL.md). Read this before the first create, not after one fails. Every id here is a literal. A paraphrased field id produces a failed call, and a wrong one produces a ticket that looks right and carries the wrong data.
```

Sections, in this order.

**Markdown works on create.** `createJiraIssue` has `contentFormat`, enum `["markdown", "adf"]`, the same as `editJiraIssue`. No hand-built ADF is needed for `description`. Pass `contentFormat: "markdown"` explicitly on every call. The shared enum text hedges with "Defaults vary by tool when omitted", and stating it costs nothing. Link to `../work-on/JIRA-FORMAT.md` for what survives conversion and for the read-back check, and do not restate those.

**The complete `createJiraIssue` parameter list.** Required is `cloudId`, `projectKey`, `issueTypeName`, `summary`. `additionalProperties: false`, 11 parameters. There is no `reporter`, `labels`, `components`, `priority`, `sprint`, `storyPoints`, `epicLink` or `issuelinks` parameter. Everything past the named ones goes through `additional_fields`.

| Parameter | Notes |
|---|---|
| `cloudId` | required |
| `projectKey` | required, a key |
| `issueTypeName` | required, a name and not an id. Copy verbatim from `getJiraProjectIssueTypesMetadata`. |
| `summary` | required, plain text with no formatting |
| `description` | markdown string, max 32000, or an ADF doc |
| `contentFormat` | `markdown` or `adf` |
| `assignee_account_id` | an accountId, not an email |
| `parent` | the issue key |
| `additional_fields` | the only route to priority, labels and every `customfield_*` |

**The create-screen table.** Copy the spec's Appendix A table verbatim. It is the whole reason three types need two calls.

**Per-type recipes.** One fenced block per type, with the exact call shape. Story and Task in one call. Subtask in one call. Bug, Epic and Bug Subtask in **two calls**, with the per-type first-call field list from the spec's Step 10 table:

| Type | First call carries | Second call adds |
|---|---|---|
| `Bug` | summary, parent, assignee, labels, priority, sprint | description, points |
| `Epic` | summary, assignee, labels, priority | description, Eng Weeks |
| `Bug Subtask` | summary, parent, labels, priority | description, points, assignee |

State plainly that `Bug Subtask` has no `assignee` and no `reporter` on its create screen at all, which is why assignee moves to the second call, and that whether it takes there is unproven.

State that `Epic` gets no sprint and `Bug Subtask` gets no sprint.

**Never write.** `customfield_10014` (Epic Link, mirrored from `parent`, on no create screen). `customfield_10016` (Story point estimate, the team-managed twin, one non-empty value in all of GRO). `components` and `fixVersions`, neither on any GRO create screen. Name the consequence for the last one, which is that the tool's own docstring advertises a components example, so the wrapper forwards it and Jira rejects it.

**Parent for every level.** Including Story to Epic and Bug to Epic. GRO is company-managed and uses the unified `parent` field.

**Portability.** Every id here is instance-specific. Re-check against `getJiraIssueTypeMetaWithFields(requiredFieldsOnly: false)` at run time, and resolve fresh for any project that is not GRO.

**The two unproven write shapes.** The sprint value, bare `9511` against `[9511]`, confirmed once on the run's first create then reused. Assignee on `Bug Subtask`. Both get reported rather than assumed.

Use `GRO-1234` in every example. Never `WMP`.

- [ ] **Step 4: Run the test to verify the new assertions pass**

Run: `bash tests/open_ticket_contract_test.sh`

Expected: with `SKILL_MD` and `DEDUP_MD` still commented out of the existence loop, every `tpl_` and `cf_` assertion passes and the script prints `open_ticket_contract_test: ok`. Restore both loop entries before committing.

- [ ] **Step 5: Commit**

```bash
git add shared_config/.claude/skills/open-ticket/CREATE-FIELDS.md tests/open_ticket_contract_test.sh
```

```bash
git commit -m "feat(claude,skill): open-ticket: per-type create recipes and the exact field ids

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: DEDUP-QUERIES.md

**Files:**
- Create: `shared_config/.claude/skills/open-ticket/DEDUP-QUERIES.md`
- Modify: `tests/open_ticket_contract_test.sh` (append a `DEDUP_FLAT` block before the final `echo`)

**Interfaces:**
- Consumes: `flatten()`, `DEDUP_MD` from Task 2
- Produces: nothing other tasks read. `SKILL.md` in Task 5 links to this file by name from Step 5.

- [ ] **Step 1: Write the failing test**

Append to `tests/open_ticket_contract_test.sh`, before the final `echo`:

```bash
DEDUP_FLAT="$(flatten "$DEDUP_MD")"

assert_contains "dq_back_pointer" "Step 5 of [SKILL.md](SKILL.md)" "$DEDUP_FLAT"

# Q0 is the positive control and it is the single most important rule in the
# file. An unknown JQL field name returns totalCount 0 with no error, so
# without Q0 a typo makes the sweep report no duplicates and the skill files
# the duplicate it exists to prevent.
assert_contains "dq_has_q0" "Q0" "$DEDUP_FLAT"
assert_contains "dq_q0_mandatory" "positive control" "$DEDUP_FLAT"
assert_contains "dq_silent_zero" "totalCount" "$DEDUP_FLAT"

# All six queries present.
for q in Q1 Q2 Q3 Q4 Q5; do
  assert_contains "dq_has_$q" "$q" "$DEDUP_FLAT"
done

# The quoting recipe. Bare ~ is an order-independent AND over stemmed tokens,
# so one absent term zeroes the query.
assert_contains "dq_escaped_quotes" '\"' "$DEDUP_FLAT"

# No status filter and no date filter. statusCategory = Done was 65% of
# matches and a 180-day window cut 29 hits to 8.
assert_contains "dq_no_date_filter" "no date filter" "$DEDUP_FLAT"

# A bare file path over-matches, and :LINE splits on the colon.
assert_contains "dq_no_line_numbers" ":LINE" "$DEDUP_FLAT"

# The git half uses -G. On the same string -S found 4 commits and -G found 10.
assert_contains "dq_git_uses_dash_g" "log -G" "$DEDUP_FLAT"
assert_contains "dq_git_needs_all" "--all" "$DEDUP_FLAT"

# Payload discipline. Three unguarded queries returned about a million
# characters each, because description is in a mandatory field floor.
assert_contains "dq_count_mode" "searchResultMode" "$DEDUP_FLAT"

# The silent field-name drop, distinct from the silent JQL zero.
assert_contains "dq_sprint_name_dropped" "customfield_10021" "$DEDUP_FLAT"

DEDUP_WMP="$(grep -c 'WMP' "$DEDUP_MD" || true)"
assert_eq "dq_no_wmp_examples" "0" "$DEDUP_WMP"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/open_ticket_contract_test.sh`

Expected: FAIL at the existence loop naming `SKILL.md`. Comment out only the `SKILL_MD` loop entry, run again, and expect FAIL on `dq_back_pointer` against empty output.

- [ ] **Step 3: Write DEDUP-QUERIES.md**

Create the file. Opening back-pointer:

```markdown
Step 5 of [SKILL.md](SKILL.md). Run Q0 first and believe no negative verdict without it. An unknown JQL field name returns totalCount 0 with no error, while a syntax error is loud, so one mistyped field name makes this whole sweep report no duplicates and the skill then files the duplicate it exists to prevent.
```

Sections.

**The quoting recipe.** Two rules cover everything observed. Every term goes inside escaped inner quotes, `text ~ "\"<TERM>\""`. Bare `~` is an order-independent AND over stemmed tokens, so a single term the ticket does not contain zeroes the query. The value is also a live Lucene query string, where uppercase `OR` and `AND` and a whitespace-preceded `-` change the meaning.

**The file-path caveat.** A bare path over-matches badly. `description ~ "runwayer-plans.ts"` gave 31 hits against 1 for the quoted form, and one path-shaped search returned a false positive for a path that does not exist. Never append `:LINE`, because the colon splits and the line number becomes its own AND term.

**No status filter and no date filter.** `statusCategory = Done` was 65% of matches for the tested term, and `created > -180d` cut 29 matches to 8, a 72% recall loss. Rank with `ORDER BY updated DESC` instead.

**Payload discipline.** `searchJiraIssuesUsingJql` always returns a mandatory field floor of `assignee, description, issuetype, project, status, summary` plus whatever is requested, and `description` cannot be excluded. Three unguarded queries produced responses of 1,051,178, 760,498 and 925,249 characters. Use `searchResultMode: "count"` for every tally and fetch nodes only for the candidates that survive. Also state that `"sprint"` as a name in the `fields` array is silently dropped with no error, and only `customfield_10021` works. Name it as a second silent-failure mode distinct from the JQL zero.

**The six queries**, copied verbatim from the spec's Appendix B, each with the one-line statement of what it catches that the others miss. Keep Q5 unquoted and keep the warning to read its hits with suspicion.

**Search tool limits.** `maxResults` maximum 100, values below 50 honored. Cursor pagination via `nextPageToken` from `issues.pageInfo.endCursor`, and the cursor embeds the JQL and the sort. `ORDER BY` honored across pages. Rate limit 300.

**The git half.** Every command runs alone with no pipe and no chain, redirected to a file under `/tmp/claude/`, read back in a separate call, because `git` runs outside the sandbox and a pipe or chain is denied.

```
git log -G'<pattern>' --all --oneline > /tmp/claude/open-ticket-<slug>-git-code.txt
git log --grep='<noun>' --all --oneline > /tmp/claude/open-ticket-<slug>-git-msg.txt
git log --oneline --all -- <path> > /tmp/claude/open-ticket-<slug>-git-path.txt
```

Use `-G`, not `-S`. On the same string `-S` found 4 commits and `-G` found 10, because `-S` only fires when the occurrence count changes. `--all` is not the default and has to be passed. POSIX character classes, not `\s`, which returns zero hits under `-P` on git 2.50.1.

- [ ] **Step 4: Run the test to verify the new assertions pass**

Run: `bash tests/open_ticket_contract_test.sh`

Expected: with only `SKILL_MD` commented out of the existence loop, every `tpl_`, `cf_` and `dq_` assertion passes and the script prints `open_ticket_contract_test: ok`. Restore the loop entry before committing.

- [ ] **Step 5: Commit**

```bash
git add shared_config/.claude/skills/open-ticket/DEDUP-QUERIES.md tests/open_ticket_contract_test.sh
```

```bash
git commit -m "feat(claude,skill): open-ticket: the duplicate sweep, with Q0 as its control

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: SKILL.md

**Files:**
- Create: `shared_config/.claude/skills/open-ticket/SKILL.md`
- Modify: `tests/open_ticket_contract_test.sh` (append a `SKILL_FLAT` block before the final `echo`)

**Interfaces:**
- Consumes: `flatten()`, `SKILL_MD` from Task 2. Links by name to `TEMPLATES.md` (Task 2), `CREATE-FIELDS.md` (Task 3), `DEDUP-QUERIES.md` (Task 4). All three must exist before this task runs.
- Produces: the skill itself. Task 6 installs it.

- [ ] **Step 1: Write the failing test**

Append to `tests/open_ticket_contract_test.sh`, before the final `echo`:

```bash
SKILL_FLAT="$(flatten "$SKILL_MD")"

# Frontmatter. The name must match the directory, or the skill does not load.
assert_contains "sk_frontmatter_name" "name: open-ticket" "$SKILL_FLAT"
assert_contains "sk_frontmatter_desc" "description:" "$SKILL_FLAT"
# The description says what NOT to use it for, following the house convention.
assert_contains "sk_says_not_work_on" "work-on" "$SKILL_FLAT"

# All four abort codes, as literal tokens. The token is the wire contract, and
# a paraphrased reason is not an abort a caller can recognize.
assert_contains "sk_abort_no_tooling" "JIRA_UNAVAILABLE_NO_TOOLING" "$SKILL_FLAT"
assert_contains "sk_abort_write_denied" "JIRA_WRITE_DENIED" "$SKILL_FLAT"
assert_contains "sk_abort_duplicate" "DUPLICATE_FOUND" "$SKILL_FLAT"
assert_contains "sk_abort_no_human" "GATE_UNREACHABLE_NO_HUMAN" "$SKILL_FLAT"

# acli is not a fallback. Even authenticated it cannot set sprint, story
# points, or any custom field, so a tree is not expressible through it.
assert_contains "sk_acli_not_a_fallback" "acli" "$SKILL_FLAT"

# The sprint filter. The array is a history in no meaningful order, and the
# active entry was last on 65 of 69 issues and first on 40 of 69, so both [0]
# and [-1] are wrong.
assert_contains "sk_sprint_active_filter" 'state == "active"' "$SKILL_FLAT"

# The one gate. Nothing is created before it.
assert_contains "sk_plan_gate_path" "/tmp/claude/open-ticket-" "$SKILL_FLAT"

# The points rule, with both boundaries stated as numbers so the plan file
# carries a number to argue with.
assert_contains "sk_points_13" "13" "$SKILL_FLAT"

# Three verification outcomes, never two. An unverified write is the thing the
# user most needs told.
assert_contains "sk_three_outcomes" "Unverified" "$SKILL_FLAT"

# The publish override must name the create command specifically. The
# enumerated list in writing-work-docs names create separately from edit.
assert_contains "sk_publish_override" "publish override" "$SKILL_FLAT"
assert_contains "sk_override_names_create" "workitem create" "$SKILL_FLAT"

# Every reference file is linked from the step that reads it.
assert_contains "sk_links_templates" "TEMPLATES.md" "$SKILL_FLAT"
assert_contains "sk_links_create_fields" "CREATE-FIELDS.md" "$SKILL_FLAT"
assert_contains "sk_links_dedup" "DEDUP-QUERIES.md" "$SKILL_FLAT"

# All thirteen steps present.
for n in 0 1 2 3 4 5 6 7 8 9 10 11 12; do
  assert_contains "sk_has_step_$n" "Step $n" "$SKILL_FLAT"
done

SKILL_WMP="$(grep -c 'WMP' "$SKILL_MD" || true)"
assert_eq "sk_no_wmp_examples" "0" "$SKILL_WMP"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/open_ticket_contract_test.sh`

Expected: FAIL with `missing or empty: .../open-ticket/SKILL.md`, exit 1. The existence loop is the correct red state now, with all three other files present.

- [ ] **Step 3: Write the frontmatter and the pipeline**

Create `shared_config/.claude/skills/open-ticket/SKILL.md`. Frontmatter exactly as approved in the spec:

```yaml
---
name: open-ticket
description: Turns a requirement in prose into the Jira issues for it. Infers the project, the active sprint and the assignee from the user's own tickets, reads the repo so every issue names real files and real patterns, sweeps Jira and git history for work already filed or already shipped, sizes the work against the story-point guidelines, and builds the smallest tree that fits. Drafts every description locally and waits for one approval before creating anything. Use this skill when the user says "open a ticket", "file a ticket for this", "create a Jira issue", "make tickets for this feature", "break this into tickets", or hands over a requirement and expects issues rather than code. Requires the Atlassian MCP. Do not use it to edit an existing ticket (that is work-on) and do not use it to read or summarize one.
---
```

Then `# Open Ticket` as the H1, then the `## Pipeline` table copied from the spec, with its "Writes to Jira" column. Then `## Assume nothing` and `## Earn every question`, adapted from `work-on/SKILL.md`, with the addition that sprint and assignee survive the doctrine because no amount of reading settles them, and Step 2 reduces them to a confirmation.

- [ ] **Step 4: Write Steps 0 through 6**

Copy the spec's Step 0 through Step 6 sections. Every rule keeps the specific bad outcome it prevents, because a rule with no named consequence does not survive review in this repo.

Step 0 states `JIRA_UNAVAILABLE_NO_TOOLING` and says `acli` cannot substitute and why. It also states `GATE_UNREACHABLE_NO_HUMAN`, the skill-level backstop for the case the hook cannot see. The hook denies a `Workflow` whose text names this skill, and it cannot catch a workflow agent that decides to run the skill on its own initiative. So Step 0 checks whether it can present the Step 9 gate at all, and aborts if it cannot. Name the failure this prevents, which is creating Jira issues with nobody having approved them.

Step 2 carries the four-call inference chain with the exact JQL, the `state == "active"` filter, the two traps, the runner-up reporting rule, and both ask-branches.

Step 5 links `DEDUP-QUERIES.md` with a reason to click, and keeps only the logic in `SKILL.md`. Do not restate the queries.

Step 6 carries the credible-match definition from the spec, both positive conditions and all three non-conditions, plus the (a)/(b)/(c) options and the rule that nothing is written on the skill's own initiative here.

- [ ] **Step 5: Write Steps 7 through 12 and the constraints**

Step 7 carries the points table with both boundaries as numbers, the leaf-splitting rule, the issue-type-by-intent mapping, the Epic Eng Weeks derivation labeled as derived, and the `Bug Subtask` choice with its 2334-against-1 justification.

Step 8 links `TEMPLATES.md` and carries the publish override verbatim from the spec, with `acli jira workitem create` named as a distinct entry from `edit`.

Step 9 carries the plan-file path pattern, the slug rule and its regex `^[a-z0-9][a-z0-9-]{0,48}$`, the full list of what the file holds, and the one-yes rule. State that edit is an offered response, not only yes or no.

Step 10 links `CREATE-FIELDS.md` and keeps only the ordering and the failure policy. Parents first, top down. Partial failure stops the run, records every key created, and does not retry blind.

Step 11 carries the three outcomes with `Unverified is its own outcome` stated in those words, the per-type verification scoping, and the ban on restoring or deleting on a failed check.

Step 12 carries the final report contents.

Then `## Abort codes` as a table, and `## Constraints` last, as flat bullets with a bolded lead and the why. The constraints cover never transitioning or deleting an issue, no Confluence, no issue links beyond `parent`, the four unread issue types the skill does not emit, and the Bash rules for every `git` command.

- [ ] **Step 6: Run the test to verify it passes**

Run: `bash tests/open_ticket_contract_test.sh`

Expected: PASS, printing `open_ticket_contract_test: ok`, exit 0. No entries commented out of the existence loop.

- [ ] **Step 7: Check the prose against the voice rules**

Run: `grep -nioE '\b(surface[sd]?|ensure[sd]?|comprehensive|robust|crucial|utilize|leverage|streamline|seamless(ly)?|granular|nuanced|meticulous|holistic|foster|harness|unlock|empower|elevate|showcase|moreover|furthermore|additionally|notably|importantly|essentially|fundamentally|ultimately|arguably|delve|unpack|myriad|plethora|synergy|enhance[sd]?|intricate|pivotal|vital|paramount)\b' shared_config/.claude/skills/open-ticket/SKILL.md`

Expected: no output. Any hit is a banned word and gets rewritten.

Run: `grep -nE ' - |—|–|→|…|≥|≤' shared_config/.claude/skills/open-ticket/SKILL.md`

Expected: no output. Any hit is a clause joiner or a non-ASCII glyph and gets rewritten.

Repeat both on the three reference files.

- [ ] **Step 8: Commit**

```bash
git add shared_config/.claude/skills/open-ticket/SKILL.md tests/open_ticket_contract_test.sh
```

```bash
git commit -m "feat(claude,skill): open-ticket: file the Jira tree for a requirement, gated on one approval

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Wire CI and install

**Files:**
- Modify: `.github/workflows/tests.yml` (add one job after the `deny-review-in-workflow-hook` job)

**Interfaces:**
- Consumes: `tests/open_ticket_contract_test.sh` from Tasks 2 through 5
- Produces: nothing

CI runs one job per test file, so a new test file without a job never runs on a pull request and the contract it asserts decays silently.

- [ ] **Step 1: Add the CI job**

Insert after the `deny-review-in-workflow-hook` job. `ubuntu-latest`, following the `work-on-validation` precedent, because this test is bash plus `tr` and `grep` with nothing macOS-specific and Linux minutes bill at a tenth the rate.

```yaml
  # ubuntu, unlike the hook tests above. This one is bash plus tr and grep with
  # no zsh, no brew and no darwin build tag, so it behaves identically and
  # Linux minutes bill at a tenth the rate.
  open-ticket-contract:
    name: open-ticket contract test
    runs-on: ubuntu-latest

    steps:
      - name: Check out repository
        uses: actions/checkout@v4

      - name: Run open-ticket contract test
        run: bash tests/open_ticket_contract_test.sh
```

- [ ] **Step 2: Run both test files to confirm the suite is green**

Run: `bash tests/open_ticket_contract_test.sh`

Expected: PASS, printing `open_ticket_contract_test: ok`.

Run: `bash tests/deny_review_in_workflow_test.sh`

Expected: PASS, no output, exit 0.

- [ ] **Step 3: Confirm the skill directory is complete**

Run: `ls -la shared_config/.claude/skills/open-ticket/`

Expected: exactly four `.md` files, `SKILL.md`, `TEMPLATES.md`, `CREATE-FIELDS.md`, `DEDUP-QUERIES.md`.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/tests.yml
```

```bash
git commit -m "test(claude,ci): run the open-ticket contract test on pull requests

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

- [ ] **Step 5: Hand the install to the user**

`melvin-config claude sync` writes `~/.claude`, which the sandbox denies, so it cannot run from this session. Do not attempt it and do not work around it.

Ask the user to run it themselves, and tell them the `!` prefix runs it in this session so the output lands in the conversation:

```
! melvin-config claude sync
```

Then ask them to restart Claude Code, because a newly symlinked skill is picked up at startup.

- [ ] **Step 6: Report what is unproven**

The skill ships with two write shapes that no read could settle. Say both plainly in the handoff rather than letting the first real run discover them.

- The sprint value, bare `9511` against `[9511]`. The skill confirms it once on the run's first create and reuses the proven shape.
- Whether assignee takes on a `Bug Subtask` through a follow-up edit, given the type has no assignee field on its create screen at all.

Also say that the no-active-sprint branch of Step 2 was never exercised against live data, because this account had an active sprint throughout the research.

---

## Self-Review

**Spec coverage.** Every spec section maps to a task.

| Spec section | Task |
|---|---|
| Files table | 2, 3, 4, 5 |
| Frontmatter | 5 |
| Pipeline | 5 |
| Step 0 preflight, and the acli decision | 5 |
| Step 1 read the request | 5 |
| Step 2 inference chain | 5 |
| Step 3 rough size | 5 |
| Step 4 explore | 5 |
| Step 5 dedup sweep | 4 for the queries, 5 for the logic |
| Step 6 duplicate gate, credible-match definition | 5 |
| Step 7 tree, points rule, Bug Subtask choice | 5 |
| Step 8 drafting, publish override | 2 for the templates, 5 for the override |
| Step 9 plan gate, slug rule | 5 |
| Step 10 creation, per-type recipes | 3 for the recipes, 5 for the ordering |
| Step 11 verification, three outcomes | 5 |
| Step 12 final report | 5 |
| Abort codes | 1 for the hook, 5 for the skill |
| Known unknowns | 3 and 6 |
| Out of scope | 5, in Constraints |
| Deliverables beyond the skill | 1 and 6 |

**One item the spec left implicit and this plan makes explicit.** The spec's "deliverables beyond the skill" section justified the hook entry by the Step 4 fan-out. That reason does not hold, because Step 4 degrades gracefully to its inline path. Task 1 replaces it with the reason that does hold, which is the Step 9 gate, and adds the `GATE_UNREACHABLE_NO_HUMAN` abort as the skill-level backstop for the case the hook cannot see. Both changes are recorded here rather than left as a silent correction.

**Placeholder scan.** No TBD, no TODO, no "implement later", no "similar to Task N", no "add appropriate error handling". Every `TODO(user):` reference is the deliberate `writing-work-docs` convention, named as such. Every code step carries a real block. Every test step carries the exact command and the exact expected output.

**Type consistency.** `flatten()` is defined once in Task 2 and reused by name in Tasks 3, 4 and 5. The four path variables `SKILL_MD`, `TEMPLATES_MD`, `CREATE_FIELDS_MD`, `DEDUP_MD` are defined once in Task 2 and used consistently. `assert_eq` and `assert_contains` match the real signatures in `tests/test_helpers.sh`, which are `(label, expected, actual)` and `(label, needle, haystack)`. The `decision` helper in Task 1 matches the real one at `tests/deny_review_in_workflow_test.sh:43`. Every field id is identical across Global Constraints, Task 3 and Task 5.

**One known rough edge, stated rather than hidden.** Tasks 2, 3 and 4 each reach a red state at the existence loop rather than at their own assertions, because the loop guards all four files and the files arrive one task at a time. Each of those tasks says to comment out the not-yet-written entries, confirm the real assertions, then restore before committing. That is a manual step and it is the honest cost of a single shared contract test. The alternative, four separate test files, would need four CI jobs for one contract.
