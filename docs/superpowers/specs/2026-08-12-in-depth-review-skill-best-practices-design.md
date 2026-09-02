# in-depth-review: restructure for Skill authoring best practices

Date: 2026-08-12
Target: `shared_config/.claude/skills/in-depth-review/` (currently one file, `SKILL.md`, 1,124 lines)
Source doc: https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices

Revision note: this spec was independently audited against the real files before being shown to
the user. The audit found real arithmetic errors in an earlier draft's line-count table, a genuine
ambiguity in heading levels between two Edits, and two places where "verbatim" text had silently
been re-typed with ASCII punctuation in place of the source's em-dashes. Every number and every
quoted block below was re-derived by extracting the real bytes with `sed` and measuring the actual
replacement text with `wc -l`, not by hand arithmetic. See "Audit corrections" at the end for the
full list of what changed from the pre-audit draft.

## Goal

Bring `in-depth-review` in line with the best-practices doc's authoring guidance -- frontmatter
description length, SKILL.md under 500 body lines, progressive disclosure via reference files one
level deep, table-of-contents on longer reference files, no redundant restatement -- with **zero
behavior change**. Every flag, the role-number-to-category table, and the JSON output shape stay
byte-identical, because `pr-review`, `review-and-fix`, and the wrapper agents
(`pr-review-finder-indepth`, `pr-review-finder-indepth-deep`) invoke this skill by exact name and
parse its output field-for-field.

## Why this is not a trivial reformat

Three things make this more than moving text between files:

1. **The frontmatter `description` is ~1,570 characters against the doc's documented 1,024 max**,
   and unlike everything else in this skill, that field loads into every session's system prompt
   regardless of whether the skill is ever used. It is the single highest-leverage fix in this
   spec, independent of the file-split.
2. **Three roles (#6, #7, #12) are gated on diff content.** The orchestrator must decide whether to
   launch a gated role *before* it has any reason to read that role's prompt text. If a gated
   role's entire section (gate criteria + prompt) moves into `roles/06-database.md`, the
   orchestrator has to read that file just to learn whether to skip it -- which defeats the reason
   for gating it in the first place. The fix: only the prompt block (the text actually sent to the
   launched sub-agent) moves to the role file. The gate criteria -- the mechanical test plus the
   judgment-call guidance for ambiguous cases -- stays in `SKILL.md`'s Step 1, where the
   orchestrator already has to be looking to decide `<ROLE_SET>`.
3. **`SCORING.md` is an addition beyond the file-layout approach as discussed**, worth calling out
   explicitly. Splitting the role prompts, deduplicating the collection protocol, and splitting
   Step 4's output already bring `SKILL.md` to a comfortable ~425 lines on their own (verified line
   budget below) -- so, corrected from an earlier draft of this spec, extracting the scoring rubric
   is not *required* to clear 500 lines. It is still worth doing, for two reasons that hold up
   independent of the line count: (a) it is a genuine content-type split -- the rubric is reference
   material the orchestrator hands to each scorer sub-agent, not orchestration logic the
   orchestrator itself executes, which is exactly the "high-level guide with references" pattern
   the doc describes; (b) it widens the safety margin (425 lines vs. ~489 if the rubric stayed
   inline) against future edits creeping back over 500. Flagged here because it is a real addition
   to the discussed layout, not implied by it, and its original justification (in a pre-audit draft
   of this spec) overstated the case -- see "Audit corrections."

## Decisions taken

| Decision | Choice | Why |
|---|---|---|
| File-layout approach | One file per role (Approach B), plus `COLLECTING.md`, `SCORING.md`, `OUTPUT-JSON.md`, `OUTPUT-CHAT.md` | Approach B was the explicit choice. `SCORING.md` is the one addition -- see point 3 above. |
| Gate criteria for #6 / #7 / #12 | Stays in `SKILL.md` Step 1. Only each role's prompt block moves to its file. | Point 2 above. A gate the orchestrator can't evaluate without opening the gated file isn't a gate. |
| Frontmatter `description` | Rewritten to ~600 characters, what-and-when only | Cuts the ~1,570-character original by more than half. Drops the exact-role-floor detail, which the body already states (Step 1). Also drops the specific counts of how many instances `pr-review` / `review-and-fix` spawn -- that fact is not restated anywhere else in `in-depth-review`'s own file, but it was never this file's job to be the authoritative source for it: `pr-review` and `review-and-fix` each independently document their own composition in their own frontmatter and body (verified: `pr-review/SKILL.md` lines 4-6, 24-25, 182-187; `review-and-fix/SKILL.md` lines 5-13). Losing it from in-depth-review's description costs a reader nothing they couldn't already get from the more-authoritative source. |
| Collection protocol duplication (Step 1 role-collection vs. Step 2.1 scorer-collection) | Merged into one `COLLECTING.md`, parameterized on "agent" | The two blocks are the same turn-taking/polling protocol restated with `s/role/scorer/`. One generalized file, referenced from both call sites, removes the duplication the doc's "concise is key" principle flags. |
| Scoring rubric | Extracted to `SCORING.md` | Point 3 above -- margin and content-type separation, not necessity. |
| Step 4 output (JSON vs. chat report) | Split into `OUTPUT-JSON.md` / `OUTPUT-CHAT.md` | The two branches are mutually exclusive per invocation (sub-agent caller vs. direct user). Splitting them means a caller only ever loads the branch it needs. |
| `Constraints` section (lines 1088-1124) | Deleted. Two clauses relocated, not just one. | Checked bullet-by-bullet, all 10, against Steps 0-4 (see "Constraints section audit"). Nine of ten restate something stated in full elsewhere; the tenth ("gate is a cost guard") is genuinely new and becomes the lead-in to "Conditional role gates." A second clause -- the scorer half of "never let either inherit the session model" -- has no existing home elsewhere and is added to Step 2.1's scorer-launch instruction rather than silently dropped (see Edit 8). |
| New-file heading level | Every new file (`COLLECTING.md`, `SCORING.md`, `OUTPUT-JSON.md`, `OUTPUT-CHAT.md`, every `roles/*.md`) uses a single top-level `#` heading for its own title. | An earlier draft of this spec gave conflicting instructions here (H3 in one Edit, H1 in another, for the same file) -- see "Audit corrections." H1 is correct because each of these is now a standalone file, not a subsection nested under `SKILL.md`'s own headings. |
| Role prompt wording, gate rationale, collection-protocol rationale (the "256 attempts" anecdote, "the shorter version failed in practice," the TTL-cache/CSV-parser example) | Unchanged, verbatim -- including original punctuation (em dashes) and one pre-existing typo | Per your "trim redundancy only, keep load-bearing why" call. These aren't decorative -- each one explains a non-obvious rule or documents a specific past failure mode. An earlier draft of this spec accidentally flattened several of the source's em dashes to `--` and reworded one sentence while reproducing it in the spec document; both are fixed here by extracting the real bytes rather than retyping them (see "Audit corrections"). The pre-existing typo -- a stray unmatched `**` in Role #7's gate text, source line 488-489 -- is left exactly as it is; fixing it is a content change outside "zero behavior change," and is noted under "Known follow-ups" instead. |
| Skill name, role numbers, flags, JSON schema, GitHub read/write policy, model policy | Unchanged | External contract. `pr-review` / `review-and-fix` / the two wrapper agents depend on all of these by exact name or shape. |

## New file layout

```
in-depth-review/
├── SKILL.md              (~425 lines after edits, from 1,124)
├── COLLECTING.md         (shared async agent-collection protocol)
├── SCORING.md            (confidence rubric, citation hard-cap, calibration, ticket/motivation scoring)
├── OUTPUT-JSON.md        (Step 4's sub-agent-caller JSON schema)
├── OUTPUT-CHAT.md        (Step 4's direct-user chat report template)
└── roles/
    ├── _common-fragment.md   (the shared prompt preamble every role prompt starts with)
    ├── 01-agents-md.md       (category: AGENTS.md)
    ├── 02-bug-scan.md        (category: bug)
    ├── 03-history.md         (category: history)
    ├── 04-prior-pr.md        (category: prior PR)
    ├── 05-comment-guidance.md (category: comment guidance)
    ├── 06-database.md        (category: db -- prompt only, gate stays in SKILL.md)
    ├── 07-security.md        (category: security -- prompt only, gate stays in SKILL.md)
    ├── 08-error-handling.md  (category: error-handling)
    ├── 09-test-coverage.md   (category: test coverage)
    ├── 10-ticket.md          (category: ticket)
    ├── 11-motivation.md      (category: motivation)
    └── 12-types.md           (category: types -- prompt only, gate stays in SKILL.md)
```

Every file is referenced directly from `SKILL.md` -- none of the new files reference each other --
so every reference stays one level deep. No new file exceeds 100 lines except possibly
`OUTPUT-JSON.md` (~80 lines, still under 100), so none needs its own table of contents;
`SKILL.md`'s existing Step 0-4 structure plus the role table (with a new `File` column) serves as
the navigation index the doc's "table of contents" framing calls for.

## Line budget

Ranges below tile the real file with no gaps and no overlaps -- each row ends exactly where the
next begins, so blank separator lines and section headers all land in exactly one row. Verified:
the "Old lines" column sums to 1,124, the real, `wc -l`-measured length of the source file. Every
"New lines" figure for a row whose content changes was measured by actually building the
replacement text and running `wc -l` on it (see the scratch files listed against each row), not
computed by hand.

| # | Section | Old range | Old lines | New lines | Disposition |
|---|---|---|---|---|---|
| 1 | Frontmatter | 1-23 | 23 | 12 | Description rewritten (~600 chars); 4 unchanged lines (`---`, `name:`, closing `---`, trailing blank) + 8-line new description block, measured. |
| 2 | Body intro (H1) | 24-34 | 11 | 11 | Unchanged. |
| 3 | Argument + Modifier flags | 35-76 | 42 | 42 | Unchanged. |
| 4 | GitHub side-effect policy | 77-88 | 12 | 12 | Unchanged. |
| 5 | GitHub access | 89-109 | 21 | 21 | Unchanged. |
| 6 | Step 0: Resolve scope | 110-148 | 39 | 39 | Unchanged. |
| 7 | Step 1 preamble + gate table + NEW "Conditional role gates" | 149-179 | 31 | 100 | Preamble/table unchanged (31; only "see Role #N" -> "see below" in 3 cells, no line-count change) + new subsection inserted (69 = 67 measured content lines + 2 blank separators). |
| 8 | "How a role's findings reach the parent" (collection protocol, `--roles` note, role table, model policy) | 180-280 | 101 | 36 | Collection protocol (71 lines) replaced with a `COLLECTING.md` pointer; `--roles` note (4 lines) and model policy (5 lines) kept verbatim; role table gets a `File` column. Measured as one assembled block. |
| 9 | Common reviewer prompt fragment | 281-324 | 44 | 6 | Moved verbatim to `roles/_common-fragment.md`; replaced with a pointer. Measured. |
| 10 | Role #1 (whole block) | 325-337 | 13 | 0 | Moved verbatim to `roles/01-agents-md.md`. |
| 11 | Role #2 (whole block) | 338-351 | 14 | 0 | Moved verbatim to `roles/02-bug-scan.md`. |
| 12 | Role #3 (whole block) | 352-382 | 31 | 0 | Moved verbatim to `roles/03-history.md`. |
| 13 | Role #4 (whole block) | 383-420 | 38 | 0 | Moved verbatim to `roles/04-prior-pr.md`. |
| 14 | Role #5 (whole block) | 421-441 | 21 | 0 | Moved verbatim to `roles/05-comment-guidance.md`. |
| 15 | Role #6 (header + gate + prompt) | 442-485 | 44 | 0 | Header text and gate criteria (443-463) relocate into row 7's new subsection; prompt block (464-484) moves to `roles/06-database.md`. Nothing stays at this location. |
| 16 | Role #7 (header + gate + prompt) | 486-542 | 57 | 0 | Same pattern: header + gate criteria (487-511) into row 7; prompt (512-541) to `roles/07-security.md`. |
| 17 | Role #8 (whole block) | 543-568 | 26 | 0 | Moved verbatim to `roles/08-error-handling.md`. |
| 18 | Role #9 (whole block) | 569-599 | 31 | 0 | Moved verbatim to `roles/09-test-coverage.md`. |
| 19 | Role #10 (whole block) | 600-669 | 70 | 0 | Moved verbatim to `roles/10-ticket.md`. |
| 20 | Role #11 (whole block) | 670-707 | 38 | 0 | Moved verbatim to `roles/11-motivation.md`. |
| 21 | Role #12 (header + gate + prompt) | 708-750 | 43 | 0 | Header + gate criteria (709-719) into row 7; prompt (720-749) to `roles/12-types.md`. |
| 22 | Step 2 heading + Step 2.0 | 751-792 | 42 | 42 | Unchanged -- `roles_missing` semantics are specific to this skill, not generic collection mechanics. |
| 23 | Step 2.1: Pool and score | 793-932 | 140 | 60 | Pooling/dedup (793-813, kept, +1 clause) + scorer-collection duplicate (814-834) replaced with a `COLLECTING.md` pointer + what-to-give-scorer/mandatory rules (835-865, kept) + scoring rubric (866-931) replaced with a `SCORING.md` pointer + trailing blank. Measured as one assembled block. |
| 24 | Step 3: Filter and dedup | 933-961 | 29 | 29 | Unchanged. |
| 25 | Step 4 preamble + sub-agent JSON branch | 962-1046 | 85 | 12 | Ordering preamble (962-969, 8 lines, unchanged) + JSON branch (970-1046, 77 lines) replaced with a 4-line pointer to `OUTPUT-JSON.md`. Measured. |
| 26 | Step 4 direct-user chat branch | 1047-1087 | 41 | 3 | Replaced with a pointer to `OUTPUT-CHAT.md`. Measured. |
| 27 | Constraints | 1088-1124 | 37 | 0 | Deleted. Two clauses relocated (row 7's new subsection; row 23's added scorer-inherit clause), not counted here since they're counted where they land. |
| | **Total** | | **1,124** | **425** | |

**Without the `SCORING.md` split** (rubric kept inline in row 23 instead of a 2-line pointer:
60 - 2 + 66 = 124 for that row), the total would be 425 - 60 + 124 = **489** -- still under 500,
which is the corrected version of point 3 above. The `SCORING.md` split is kept for the reasons
given there, not because 500 is otherwise unreachable.

## Constraints section audit

All 10 bullets (the pre-audit draft of this spec checked only 9 -- see "Audit corrections"),
checked against where each restates existing content, and what happens to the two that don't:

| Constraints bullet | Restates / relocated to |
|---|---|
| "No GitHub writes, ever." (+ the MCP-preference clause) | `## GitHub side-effect policy` (lines 77-88) for the write-command list; the MCP-preference sentence specifically is in the adjacent `## GitHub access` (lines 89-93). Both sections are kept unchanged, so nothing is lost. |
| "9 to 12 parallel reviewers... Eight are unconditional... Three are gated." (+ "Never serialize" + the `--roles`-is-the-only-exception clause) | Step 1's role-count summary (lines 160-172) for the counts; "Never serialize" is in the Step 1 preamble (line 153); the `--roles` clause is in the note at lines 252-256. All three locations are kept unchanged. |
| "The three gated roles are conditional, not optional... never resolve a gate as false to save an agent." | Not stated elsewhere verbatim -- **relocated**, not deleted, to the lead-in of the new "Conditional role gates" subsection (row 7). |
| "#7's gate is deliberately narrower than #6's and #12's... do not widen this gate on the strength of empty results." | Role #7's own gate rationale, which relocates into "Conditional role gates" in full (row 7 / row 16). |
| "Role #10 is read-only and abortable... This is the user's 'ignore this reviewer' control." | Role #10's own prompt (`ABORT ON DENIAL`, `You are READ-ONLY everywhere`), moved unchanged to `roles/10-ticket.md` (row 19). |
| "Single pass per invocation -- no looping... Callers that want iteration loop externally." | Body intro (lines 26-28): "performs ONE complete review pass... does NOT post anywhere, fix anything, or loop." The specific naming of `review-and-fix` is not in that sentence, but is preserved elsewhere in the same unchanged intro: "`review-and-fix` runs 2 of these per iteration, and `pr-review` runs 3" (lines 32-33). Both are row 2, unchanged. |
| "Threshold default is `< 70` discard. `--raw` bypasses..." | Step 3 (lines 933-943), verbatim, unchanged (row 24). |
| "Scoring is per-finding and parallel -- never score serially." | Step 2.1's scorer-launch instruction (line 811), verbatim, unchanged (row 23). |
| "Model policy (cost): reviewers on Sonnet, scorers on Haiku... Never let either inherit the session model." | The reviewer half is at line 275-276 ("Do NOT let them inherit the session model"), unchanged (row 8). The scorer half has **no existing source anywhere else in the file** -- Step 2.1's "Spawn each scorer on Haiku" instruction (line 812-813) never says anything about inheritance. This is added to row 23's replacement text rather than silently dropped: "Do not let it inherit the session model either." (Edit 8). |
| "No fix application. This skill reports; consumers fix." | Body intro (line 28): "does NOT post anywhere, fix anything." Row 2, unchanged. |

Confirmed via `grep -rn "in-depth-review.*Constraints\|Constraints.*in-depth-review"` across
`~/.claude/skills/`: no sibling skill references this section by name, so nothing outside this file
depends on it existing.

## Edits

### Edit 1: Frontmatter description (lines 1-22)

Replace the `description:` block with:

```yaml
description: >
  Performs one in-depth, multi-perspective code review of a pull request or commit range using
  up to twelve specialized parallel reviewer roles (AGENTS.md compliance, bug scan, git history,
  prior PR feedback, in-file comments, database, OWASP security, error handling, test coverage,
  ticket-intent compliance, motivation delivery, TypeScript type safety). Scores findings for
  confidence, filters, and deduplicates. Never writes to GitHub. Use for "in-depth review", "deep
  review", "thorough review", or "code review without fixing" -- standalone or as a building
  block for `pr-review` and `review-and-fix`.
```

(~600 characters, against the doc's 1,024 max; measured at 8 wrapped lines, `wc -l` against the
scratch draft.)

### Edit 2: Step 1 gate table (lines 164-168)

In the `gate detail` column only, change each of the three rows' `see Role #N` to `see below`.
Do not retype the rest of any row -- leave every other character, including the em dash in row 3
("almost always — skip only a provably no-surface diff"), exactly as it is. (An earlier draft of
this spec retyped the whole row and silently flattened that em dash to `--`; see "Audit
corrections.")

### Edit 3: insert "Conditional role gates" subsection (after line 172, before line 174)

Insert the following 67-line block (measured, `wc -l`), then a blank line before and after when
splicing it into `SKILL.md`. Every role-criteria paragraph below is byte-identical to the source
(extracted with `sed`, not retyped), including the em dashes in each role header and Role #7's
pre-existing stray `**` after "makes it run.**" (source lines 488-489) -- that typo is left as-is
per "Known follow-ups." Only the lead sentence (drawn from the relocated Constraints clause) and
the three role headings' level (`###` in the source, `####` here, since this is now a subsection
of a subsection) are not byte-identical to their source location, and both of those are
structural, not content, changes:

```markdown
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
```

### Edit 4: replace "How a role's findings reach the parent" through the role table and model policy (lines 180-280)

Replace the entire tile with the following 36-line block (measured). The `--roles` paragraph and
the model-policy paragraph are byte-identical to the source (extracted with `sed`); the collection
protocol is replaced with a pointer, and the role table's lead sentence is adapted (not
byte-identical) to mention the new `File` column and the relocated gate criteria:

```markdown
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
```

Verify the `#` / `` `category` `` columns above are byte-identical to the source table (lines
260-273 today) except for the new `File` column -- confirmed already, but re-check after
transcribing.

### Edit 5: common fragment extraction (lines 281-324)

Move the entire `### Common reviewer prompt fragment` section's code block (the fenced prompt
preamble, source lines 285-323) verbatim to `roles/_common-fragment.md` under a new H1 title
(`# Common reviewer prompt fragment`). Replace lines 281-324 in `SKILL.md` with this 6-line block
(measured):

```markdown
### Common reviewer prompt fragment

Every reviewer prompt starts with the block in [roles/_common-fragment.md](roles/_common-fragment.md).
Read it once per run and prepend it to each launched role's prompt (from its file below) before
sending.
```

### Edit 6: extract the twelve role prompts

Every new role file uses a single top-level `#` heading as its own title, reusing that role's
original header text verbatim (including its em dash) with the level changed from `###` to `#`.
This applies uniformly to all twelve files; there is no exception for the three gated roles below.

For roles #1-5, #8, #9, #10, #11 (whole-block moves -- no gate to preserve in `SKILL.md`): move
each role's entire section, header through closing code fence, to its file per the table in
Edit 4, changing only that header's level (`###` -> `#`). Nothing else in the moved text changes.
Nothing is left behind in `SKILL.md` for these roles beyond the table row.

For roles #6, #7, #12 (gated): move only the fenced prompt block (the part starting `Your job:
read ONLY the diff itself...` through the closing fence) to the role's file. That file's own `#`
title is the role's original header text (e.g. `# Reviewer Role #6 — Database / data-layer scan
(conditional)`), even though that exact header text also appears at heading level `####` inside
"Conditional role gates" (Edit 3) -- the duplication is intentional and harmless, since one is a
gate-evaluation aid inside `SKILL.md` and the other is the file's own navigational title. The gate
criteria immediately preceding each prompt block in the source were already relocated into
"Conditional role gates" in Edit 3 and do not appear in the role file at all.

After this edit, lines 281-750 of the original file are gone from `SKILL.md` except for the table
(Edit 4) and the gate criteria now living in "Conditional role gates" (Edit 3).

### Edit 7: replace the Step 2.1 scorer-collection duplicate and add the missing model-inheritance clause (lines 793-932)

Replace the entire tile with the following 60-line block (measured). Lines corresponding to source
793-813 and 835-865 are byte-identical to the source except for one added clause (marked below);
the scorer-collection duplicate (814-834) and the scoring rubric (866-931) are replaced with
pointers:

```markdown
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
```

`SCORING.md` gets a new H1 title (`# Scoring rubric`) followed by the rubric block moved verbatim
from source lines 866-931 (the confidence table, the citation hard-cap rule, the calibration
guidance, and the ticket/motivation scoring specifics).

### Edit 8: Step 4 output extraction (lines 962-1087)

Keep lines 962-969 (the shared "order surviving findings by" preamble) in `SKILL.md`, unchanged.
Move lines 970-1046 (the sub-agent JSON branch, header through closing fence and every
explanatory paragraph after it -- including the field-semantics text at old lines 1019-1045 that
`review-and-fix` and `pr-review` actually consume, e.g. "Check `scoring.complete` on every
in-depth-review result") verbatim to `OUTPUT-JSON.md` under a new H1 title (`# Output: sub-agent
JSON shape`). Move lines 1047-1087 (the direct-user chat branch) verbatim to `OUTPUT-CHAT.md`
under a new H1 title (`# Output: chat report`). Replace both branches in `SKILL.md` with this
7-line block (measured: 4 lines + 3 lines):

```markdown
### If invoked as a sub-agent (by `review-and-fix` or `pr-review`)

See [OUTPUT-JSON.md](OUTPUT-JSON.md) for the exact JSON shape and field semantics.

### If invoked directly by the user

See [OUTPUT-CHAT.md](OUTPUT-CHAT.md) for the chat report template and formatting rules.
```

### Edit 9: delete Constraints (lines 1088-1124)

Delete the entire `## Constraints` section. Its two non-redundant clauses were already relocated:
the "gate is a cost guard, not a quality dial" lead-in in Edit 3, and the scorer
model-inheritance clause in Edit 7. No replacement text at this location.

### Edit 10: new file `COLLECTING.md`

Full content (generalizes the original lines 180-250, `s/role/agent/`, with the missing-
bookkeeping structure named generically since it differs by call site). New H1 title, then:

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

## Verification

No executable code changes, so verification is grep-based against the new file set, following the
wrap-tolerant `\s+` convention this repo already uses for its hand-wrapped skill files.

1. `wc -l SKILL.md` reports 425 (not merely "under 500" -- the exact figure this spec derives and
   the implementation should match; investigate any material deviation rather than assuming the
   spec's count was approximate).
2. `rg -c '^\| [0-9]+ \|' SKILL.md` (the role table) returns 12, and every row's last cell matches
   `` `roles/\d\d-[a-z-]+\.md` ``.
3. `rg -n '\]\(.*\.md\)' SKILL.md roles/*.md COLLECTING.md SCORING.md OUTPUT-*.md` -- every link
   target resolves to a file that exists, and no target file itself contains a further `.md` link
   (confirms one-level-deep references throughout).
4. `rg -c '^# ' roles/*.md COLLECTING.md SCORING.md OUTPUT-*.md` -- every file has exactly one
   top-level heading, confirming the H1-everywhere rule from "Decisions taken" was applied
   uniformly (no file left at its original `###`/no-heading state).
5. `rg -c 'roles_missing' SKILL.md` still matches Step 2.0's usage; `rg -c 'unscored'
   SKILL.md SCORING.md` still matches Step 2.1 and the rubric's citation-cap text.
6. `rg 'see\s+Role\s+#' SKILL.md` returns nothing (Edit 2 replaced every "see Role #N" cross-ref).
7. `rg -c 'Constraints' SKILL.md` returns 0.
8. `rg 'inherit the session model' SKILL.md` returns 2 hits (reviewer clause, row 8; the new
   scorer clause added in Edit 7, row 23) -- confirms the previously Constraints-only scorer clause
   survived the deletion.
9. `diff` the role-number-to-category table (old lines 260-273 vs. the new Edit 4 table) column by
   column for `#` and `` `category` `` -- both must be byte-identical to the original; only the
   `File` column is new.
10. `diff` the Step 4 JSON schema block AND its explanatory paragraphs (old lines 970-1046) against
    `OUTPUT-JSON.md`'s content -- byte-identical, not just the fenced JSON object.
11. Re-read `pr-review/SKILL.md`, `review-and-fix/SKILL.md`, `gh-style-review/SKILL.md`, and both
    wrapper agent files in full against the new layout, confirming every reference they make to
    in-depth-review (flags, role numbers, the JSON field names, "Step 1" as the source of the role
    table) still resolves exactly as before.
12. Manually reassemble `roles/_common-fragment.md` + `roles/05-comment-guidance.md` and confirm
    the concatenation is byte-identical (aside from the fragment file's own added H1 title and the
    role file's `###` -> `#` heading change) to the original fragment + Role #5 block (source lines
    285-323 + 421-441) -- a spot check that the split-and-concatenate model actually reproduces
    what a launched sub-agent received today.
13. `grep -c "makes it run\.\*\*" roles/07-security.md` (or the "Conditional role gates" block in
    `SKILL.md`, wherever the check is easiest to run) returns 1 -- confirms the pre-existing stray
    `**` typo survived the move unchanged, per "Known follow-ups," rather than being silently
    fixed as an incidental side effect of retyping.

## Out of scope

- Any change to role numbering, flags (`--raw`, `--skip-ticket`, `--roles`), the JSON output
  shape, the chat report format, the GitHub read/write policy, or the model policy (Sonnet
  reviewers, Haiku scorers).
- Renaming the skill. `in-depth-review` is a noun phrase, which the best-practices doc lists as an
  acceptable alternative to gerund form, and renaming would require updating every caller.
- `pr-review`, `gh-style-review`, `review-and-fix`. None of them get a matching restructure as
  part of this spec; each would need its own pass if desired.
- Adding evaluations or scripts. The doc's advanced-Skills guidance (utility scripts, evaluation
  harnesses, code-execution package lists) doesn't apply -- this skill has no executable code.
- Fixing the pre-existing stray `**` in Role #7's gate text (source lines 488-489). Cosmetic,
  predates this spec, and fixing it is a content change this "zero behavior change" restructure
  deliberately does not make. See "Known follow-ups."

## Known follow-ups

**F1. Pre-existing typo, not fixed.** Role #7's gate rationale has an unmatched closing `**` after
"makes it run.**" (source lines 488-489) -- the opening `**` was already closed one clause earlier
("**This gate is inverted relative to #6 and #12.**"). It renders harmlessly in practice (Markdown
is lenient with unpaired emphasis markers) but is technically malformed. Left as-is per "Out of
scope"; a follow-up could fix it as a one-line, purely cosmetic edit to `SKILL.md`'s new
"Conditional role gates" subsection and to `roles/07-security.md`'s prompt block if any of the
original phrasing carried into that file (it does not -- the typo is in the gate criteria, which
stays in `SKILL.md`, not in the prompt block that moves to the role file).

## Audit corrections

A background audit (five independent checks: line-accounting, contract-preservation,
Constraints-redundancy, gate-mechanics, and a fresh self-review) ran against the first draft of
this spec before it was shown to the user. It found:

- **Arithmetic errors in the Line budget table.** The first draft's "New SKILL.md lines" column
  summed to 402 against a claimed total of ~476; its "Old lines" column summed to 1,163 against
  the real 1,124; a merged "9 roles" row actually listed 8 role numbers and cited 273 lines against
  a real sum of 209. This spec's Line budget table was rebuilt from scratch using gap-free tiled
  ranges (verified to sum to exactly 1,124) and real `wc -l` measurements of every piece of
  replacement text, rather than hand arithmetic.
- **The source file is 1,124 lines, not 1,125.** The first draft (and this task's own framing)
  inherited an off-by-one, likely from a line-numbering display convention. Fixed throughout.
- **A genuine ambiguity**: the first draft's Edit for role-prompt extraction said moved content
  keeps its original `###` header level, while its Edit for new-file content said every new file
  gets a top-level `#` heading -- both illustrated with the identical example file and title text.
  Resolved here: every new file uses `#`, uniformly, stated once in "Decisions taken" rather than
  left implicit in two different Edits.
- **"Verbatim" text that had been silently altered.** Reproducing source content inside the spec
  document by retyping it (rather than extracting the real bytes) flattened several em dashes to
  `--` and reworded one sentence (Role #7's gate lead-in) while merging two clauses. Fixed by
  extracting every quoted "verbatim" block in this spec directly from the source file with `sed`
  and measuring replacement lengths with `wc -l`, and by making Edit 2 surgical (describe the one
  cell that changes) rather than retyping a whole table row.
- **The `SCORING.md` justification was wrong.** The first draft claimed splitting only the role
  prompts left `SKILL.md` at ~536 lines, requiring the `SCORING.md` extraction to clear 500. The
  corrected numbers show ~489 lines even without it. `SCORING.md` is kept anyway, for the reasons
  in "Why this is not a trivial reformat," point 3 -- now stated accurately.
- **The Constraints audit table checked only 9 of 10 bullets**, missing "No fix application."
  Added; it restates the body intro, same as the first nine.
- **A real information-loss risk**: deleting Constraints would have silently dropped the only
  instruction forbidding a scorer sub-agent from inheriting the session model (the reviewer half of
  that instruction lives elsewhere; the scorer half existed only in Constraints). Fixed by adding
  it to Step 2.1's scorer-launch instruction (Edit 7) instead of just deleting it.
- Several smaller citation-precision issues in the Constraints audit table (a bullet's full text
  spanning two source locations, not one) are folded into the corrected table above.
