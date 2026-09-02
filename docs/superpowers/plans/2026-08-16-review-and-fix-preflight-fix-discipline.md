# review-and-fix Pre-Flight Fix Discipline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add three pre-implementation items and one reduced post-commit scan to the `review-and-fix` fix phase, plus the state and summary plumbing they need, so fewer iterations exist only because the previous iteration's fixes introduced new findings.

**Architecture:** Nine prose edits across three markdown files in one skill directory. These files are Claude Code skills, meaning the prose IS the program that an LLM orchestrator reads top to bottom and follows. There is no build and no test suite. The correctness test for any sentence is whether two orchestrators reading it would behave the same way, so each task's verification is a wrap-tolerant `rg -U` search plus a read of the surrounding section.

**Tech Stack:** Markdown. `rg` for verification. `git` for staging.

**Spec:** `docs/superpowers/specs/2026-08-16-review-and-fix-preflight-fix-discipline-design.md`

## Global Constraints

- **Do not commit.** The human explicitly said not to commit anything. Nothing was committed while authoring the spec or this plan. Each task ends at a verification step and leaves the change unstaged. If a later human instruction authorizes commits, the commit message for each task is given at the end of that task, and one commit per task is the rule.
- **Always use `rg -U`.** These files are hand-wrapped prose and references straddle line breaks. This skill has had four separate defects escape line-oriented greps.
- **Write every space in a search pattern as `\s+`.** These are presence checks. A pattern with a literal space cannot fail correctly across a line wrap, so it passes vacuously and verifies nothing.
- **No backticks inside `rg` patterns.** Anchor on the bare identifier instead, or use `.` to stand in for the backtick. This keeps the patterns clear of the shell's quote-state rules.
- **Plain ASCII in authored prose.** Do not join two independent clauses with an em-dash, an ASCII space-hyphen-space, or a clause-splitting colon. Split into sentences. Those forms stay allowed as label prefixes, definition dashes at the start of a list item, list introductions, ratios, ranges, CLI flags, and inside literal templates. The existing files use a definition em-dash after a bullet label; match that.
- **Task 4 deliberately puts banned glyphs into `SKILL.md`.** They are the enumerated targets of a scan, which AGENTS.md's literal-content carve-out covers. Do not "fix" them, and do not let a later cleanup sweep them.
- **Do not renumber Step 3's loop-control rows.** Rows 1, 1b, 1c, 2, 4, and 5 are referenced by number throughout `SKILL.md`. No task in this plan touches them.
- **No new cap, no periodic check-in, no oscillation detector.** All three were offered to the human previously and explicitly declined. `skipped_findings` is a state variable, not a stop rule, so it does not violate this.
- **Repo rules.** Run plain `git` from inside `/Users/melvin/.melvin/config`. Never top-level `git -C`. Never pipe or chain a git command. No inline scripting, no command substitution, no `$TMPDIR`, redirects last.

## There is no test framework

Each task's "failing test" is an `rg -U` run BEFORE editing that confirms the current text matches what this plan quotes. **If a before-check does not match, STOP and report drift rather than guessing at a merge.** Each task's "passing test" is an `rg -U` run after editing that confirms the replacement landed, plus a `Read` of the edited section to confirm it still reads coherently.

## File Structure

- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md` (475 lines before the change)
- Modify: `shared_config/.claude/skills/review-and-fix/STATE.md` (37 lines before the change)
- Modify: `shared_config/.claude/skills/review-and-fix/SUMMARY.md` (63 lines before the change)

Counts are `wc -l`. An earlier revision of this plan said 476, 38, and 64, taken from the last
line number the Read tool displays, which counts one past the final newline.

No files are created. No other file changes. `pr-review`, `in-depth-review`, `gh-style-review`, and `AGGREGATING.md` are NOT touched. The change is local to `review-and-fix`'s fix phase and its two supporting files.

## The sub-step renumbering, stated once

Step 2's "For each finding" list is an explicitly numbered markdown list. Two tasks insert into it, so renumbering is a real edit, not a rendering detail.

| State | List |
|---|---|
| Before this plan | 1 Read, 2 Assess, 3 Implement, 4 Commit, 5 Record, 6 Move on |
| After Task 2 | 1 Read, 2 Assess, **3 Red test**, 4 Implement, 5 Commit, 6 Record, 7 Move on |
| After Task 4 (final) | 1 Read, 2 Assess, 3 Red test, 4 Implement, **5 Scan**, 6 Commit, 7 Record, 8 Move on |

**Every sub-step reference written by this plan uses the FINAL numbering.** Task 2's inserted text says "sub-step 4" for the existing suite, and Implement is 4 in both the after-Task-2 and final states, so that reference is stable and needs no later fixup. Task 4's inserted text references sub-step 3 and sub-step 4, both correct in the final state.

**No file references Step 2's sub-item numbers.** Verified across `SKILL.md`, `STATE.md`, `SUMMARY.md`, and `AGGREGATING.md`: every cross-reference points at "Step 2" as a whole. Task 1 Step 1 re-confirms this before any edit lands.

## Task ordering is forced

- Task 1 must precede Task 2, because Task 2's inserted text writes a skip into `skipped_findings` and that variable must already be defined.
- Task 2 must precede Task 3, because Task 3 edits the Implement sub-step and needs to know it is item 4.
- Task 3 must precede Task 4, because Task 4's scan references the catch rule that Task 3 adds.
- Task 5 may run any time after Task 1, but is placed last so `SUMMARY.md` is edited once.

---

### Task 1: The `skipped_findings` mechanism

Closes the gap that Task 2's `ask_user` escape would otherwise open. `resolved_ticket_findings` is keyed by `ticket_id` plus title and gated to `category == ticket`, so a skipped non-ticket finding has nowhere to live and re-surfaces every iteration while `any_commit` stays true and row 2 never fires.

**Files:**
- Modify: `shared_config/.claude/skills/review-and-fix/STATE.md:36-37`
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md:242-245`
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md:318-319`

**Interfaces:**
- Consumes: nothing.
- Produces: the state variable name `skipped_findings`, keyed by finding id, per-RUN. Task 2 writes to it. Task 5 reports its outcome in the per-iteration summary.

- [ ] **Step 1: Confirm no file references Step 2's sub-item numbers**

Run:
```
rg -n 'Step 2' shared_config/.claude/skills/review-and-fix/
```
Expected: 11 hits, every one of them referring to "Step 2" as a whole (the fix step, the diff-based classifier, `git add -A`, the accumulator reset). Zero hits of the form "Step 2's item N", "Step 2.3", or "sub-step" followed by a digit. If any hit names a sub-item number, STOP and report it, because the renumbering in Tasks 2 and 4 would silently break that reference.

- [ ] **Step 2: Before-check all three sites**

Run:
```
rg -U -n 'resolved_ticket_findings.:\s+ticket\s+findings\s+the\s+user\s+deferred\s+or\s+dismissed' shared_config/.claude/skills/review-and-fix/STATE.md
```
Expected: 1 hit at line 36.

Run:
```
rg -U -n 'dismissed\s+in\s+a\s+prior\s+iteration\)\.\s+Do\s+not\s+re-prompt\.\s+Deferred\s+ones\s+are\s+carried' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit at line 243.

Run:
```
rg -U -n 'every\s+finding\s+was\s+deferred\s+or\s+dismissed\)\s+->\s+\*\*stop\*\*' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit at line 318.

If any of the three returns 0 hits, STOP and report drift.

- [ ] **Step 3: Add the state variable to `STATE.md`**

Append after the `resolved_ticket_findings` entry (currently the last entry, ending line 37):

```markdown
- `skipped_findings`: per-RUN set of findings of any category that were examined and then
  skipped because no test was possible (Step 2 sub-step 3's `ask_user`), keyed by finding id.
  Later iterations skip them instead of re-prompting. `resolved_ticket_findings` does not
  cover these, because it is keyed by `ticket_id` + title and gated to `category == ticket`.
  Without this set a skipped non-ticket finding is re-attempted on every iteration, and while
  other findings keep committing, `any_commit` stays true and row 2 never fires.
```

- [ ] **Step 4: Widen the Step 2 skip list in `SKILL.md`**

Replace lines 242-245:

```
Process each finding from the ordered work list (Step 1) one at a time. Skip any
`ticket`-category finding already recorded in `resolved_ticket_findings` (deferred or
dismissed in a prior iteration). Do not re-prompt. Deferred ones are carried to the Final
Report.
```

with:

```
Process each finding from the ordered work list (Step 1) one at a time. Skip any
`ticket`-category finding already recorded in `resolved_ticket_findings` (deferred or
dismissed in a prior iteration), and any finding of any category recorded in
`skipped_findings` (examined, but no test was possible, see sub-step 3). Do not re-prompt for
either. Both are carried to the Final Report.
```

- [ ] **Step 5: Widen Step 3's committed-nothing bullet in `SKILL.md`**

Replace lines 318-319:

```
- **An iteration committed nothing** (every finding was deferred or dismissed) -> **stop**.
  The diff is unchanged, so a rerun would resurface the identical findings.
```

with:

```
- **An iteration committed nothing** (every finding was deferred, dismissed, or skipped for
  want of a test) -> **stop**. The diff is unchanged, so a rerun would resurface the
  identical findings. An iteration that skips every finding is the intended outcome of that
  rule, not a malfunction.
```

- [ ] **Step 6: After-check all three sites**

Run:
```
rg -U -n 'skipped_findings.:\s+per-RUN\s+set\s+of\s+findings\s+of\s+any\s+category' shared_config/.claude/skills/review-and-fix/STATE.md
```
Expected: 1 hit.

Run:
```
rg -U -n 'and\s+any\s+finding\s+of\s+any\s+category\s+recorded\s+in\s+.skipped_findings.' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit.

Run:
```
rg -U -n 'deferred,\s+dismissed,\s+or\s+skipped\s+for\s+want\s+of\s+a\s+test' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit.

Run:
```
rg -U -n 'every\s+finding\s+was\s+deferred\s+or\s+dismissed' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 0 hits. A hit here means the old bullet survived alongside the new one.

- [ ] **Step 7: Read both edited sections**

Read `STATE.md` in full and `SKILL.md` lines 228-250 and 310-325. Confirm the new state entry sits with its siblings, the skip sentence still reads as one thought per sentence, and the Step 3 bullet's three-item list is grammatical.

Commit message, for an executor who has been authorized to commit:
```
feat(claude,skill): review-and-fix: add skipped_findings state
```

---

### Task 2: The red-test pre-flight sub-step

The load-bearing item. Ordering is what makes it work, because a test written after the fix often also passes against the unfixed code, and that ordering is unobservable post-hoc without an artifact.

**Files:**
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md` (insert after line 263, renumber items 3-6, edit the commit template at lines 274-279)

**Interfaces:**
- Consumes: `skipped_findings` from Task 1.
- Produces: sub-step 3 exists and Implement is now sub-step 4. The `Red: <line>` commit-body convention, which Task 4's scan checks for presence.

- [ ] **Step 1: Before-check the insert point and the current numbering**

Run:
```
rg -U -n 'record\s+the\s+finding\s+in\s+.resolved_ticket_findings.\s+\(keyed' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit at line 262. This is the last line of sub-step 2, so the insert goes after line 263.

Run:
```
rg -U -n '^3\.\s+\*\*Implement\s+the\s+fix\*\*' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit at line 265. Confirms Implement is currently item 3.

Run:
```
rg -U -n 'optional\s+body\s+explaining\s+why' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit at line 278.

If any returns 0 hits, STOP and report drift.

- [ ] **Step 2: Insert the new sub-step 3**

Insert after line 263 (the blank line closing sub-step 2), before `3. **Implement the fix**`:

```markdown
3. **Behavior findings get a red test before the edit.** A behavior finding is one where you
   can name an input and the wrong output the current code gives for it. Write that test
   first, run it, and confirm it fails on the assertion the finding names rather than on an
   import error or a missing fixture. Then fix until it passes. Record the failing
   assertion's first line in the commit body as `Red: <line>`. The red run happens before the
   commit, so the never-commit-broken-code rule is untouched. You still run the existing
   suite in sub-step 4. That run is not evidence the finding is fixed, because it only covers
   behavior that already worked.

   Everything else gets no new test. Comment punctuation, a cast turned into a type guard, a
   log removed from beside a throw, a dropped metric, and doc wording are verified by the
   linter, the type checker, or by reading the diff. Never invent an assertion to satisfy
   this rule. A test that also passes against the unfixed code is worse than none, and the
   test-coverage reviewer flags it as ceremony next iteration.

   If the test needs infrastructure the repo lacks (a live DB, a new mock harness, a running
   server), use `ask_user` and offer to fix without a test or to skip the finding. Record a
   skip in `skipped_findings`, keyed by finding id, so later iterations do not re-prompt. An
   untestable finding never blocks the loop.
```

**Paragraph 2 is not optional.** Without the carve-out this item manufactures the ceremony tests that the test-coverage reviewer then flags, which lengthens the loop instead of shortening it. Do not trim it for length.

- [ ] **Step 3: Renumber the four following sub-steps**

Change each list marker, leaving the bold titles and bodies untouched:

| Was | Becomes | Title |
|---|---|---|
| `3.` | `4.` | `**Implement the fix**` |
| `4.` | `5.` | `**Commit the fix:**` |
| `5.` | `6.` | `**Record what this commit was**` |
| `6.` | `7.` | `After the bookkeeping, move to the next finding.` |

- [ ] **Step 4: Add the `Red:` line to the commit template**

The fenced block at lines 274-279 currently reads:

```
   git add -A
   git commit -m "<type>: <short description of what was fixed>

   <optional body explaining why>
```

Add one line so the artifact has a defined home:

```
   git add -A
   git commit -m "<type>: <short description of what was fixed>

   <optional body explaining why>
   Red: <first line of the failing assertion, behavior findings only>
```

Note: the `git commit -m` string in this template has no closing quote. That is pre-existing and out of scope. Do not "fix" it in this task.

- [ ] **Step 5: After-check the insert, the renumber, and the template**

Run:
```
rg -U -n '^3\.\s+\*\*Behavior\s+findings\s+get\s+a\s+red\s+test\s+before\s+the\s+edit\.\*\*' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit.

Run:
```
rg -U -n '^4\.\s+\*\*Implement\s+the\s+fix\*\*' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit.

Run:
```
rg -U -n '^3\.\s+\*\*Implement' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 0 hits. A hit means the renumber did not land and there are now two item 3s.

Run:
```
rg -U -n '^5\.\s+\*\*Commit\s+the\s+fix' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit.

Run:
```
rg -U -n '^6\.\s+\*\*Record\s+what\s+this\s+commit\s+was' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit.

Run:
```
rg -U -n '^7\.\s+After\s+the\s+bookkeeping' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit.

Run:
```
rg -U -n 'Red:\s+<first\s+line\s+of\s+the\s+failing\s+assertion' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit.

Run:
```
rg -U -n 'Record\s+a\s+skip\s+in\s+.skipped_findings.,\s+keyed\s+by\s+finding\s+id' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit. This confirms Task 2's write to Task 1's variable is present.

- [ ] **Step 6: Read the whole "For each finding" list**

Read `SKILL.md` from `### For each finding:` through the end of sub-step 7. Confirm the list numbers run 1 through 7 with no gap and no duplicate, the new sub-step 3's three paragraphs are all indented to align under the list marker, and the reference to "sub-step 4" points at Implement.

Commit message, for an executor who has been authorized to commit:
```
feat(claude,skill): review-and-fix: require a red test before behavior fixes
```

---

### Task 3: The two Implement-step bullets

The swallow-catch item is the highest leverage-to-cost ratio in the change, because it is conditional and fires only when the fix is about to add a `catch`. The reference-search item is the standing first cut if the change proves too heavy.

**Files:**
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md` (the Implement sub-step, now item 4)

**Interfaces:**
- Consumes: sub-step numbering from Task 2.
- Produces: the catch-shape rule that Task 4's scan checks as an artifact-presence test, and the reference-search call-site list that Task 4's scope check exempts.

- [ ] **Step 1: Before-check the bullet list**

Run:
```
rg -U -n 'Read\s+the\s+relevant\s+.AGENTS\.md.\s+\(root\s+and\s+sub-project\)\s+for\s+mandatory\s+conventions' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit. This is the first bullet of the Implement sub-step and the anchor for the insert.

Run:
```
rg -U -n 'Run\s+the\s+project.s\s+linter/formatter\s+if\s+one\s+exists' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit, on the line directly after the `AGENTS.md` bullet. Both new bullets go between these two.

If either returns 0 hits, STOP and report drift.

- [ ] **Step 2: Insert both bullets directly after the `AGENTS.md` bullet**

Insert before the linter bullet, at the same three-space indent as its siblings:

```markdown
   - A finding asking for error handling does not authorize a `catch` that swallows. If your
     fix adds a `catch`, it must either rethrow (bare, or wrapped with `cause`) or carry a
     comment naming why continuing is correct. AGENTS.md requires that comment and a
     reviewer's request does not waive it. If neither shape fits the finding, use `ask_user`
     rather than guessing at the reviewer's intent.
   - Decide whether the fix changes a signature, a return value, what the code throws, or
     anything else a caller can observe. When it does, list the call sites first with a
     reference search (`mcp__serena__find_referencing_symbols`, or `rg` on the symbol name),
     report the count in one line, and read every call site the change reaches. Any call site
     that needs a matching change goes in the same commit. Fixes confined to comments,
     formatting, or doc files skip this entirely.
```

Placement rationale, so a reviewer does not "correct" it: these are coding-standards items and belong with the `AGENTS.md` conventions read, not after the "Do not commit if lint or tests fail" bullet, which reads as the sub-step's closing gate.

- [ ] **Step 3: After-check both bullets**

Run:
```
rg -U -n 'does\s+not\s+authorize\s+a\s+.catch.\s+that\s+swallows' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit.

Run:
```
rg -U -n 'list\s+the\s+call\s+sites\s+first\s+with\s+a\s+reference\s+search' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit.

Run:
```
rg -U -n 'Any\s+call\s+site\s+that\s+needs\s+a\s+matching\s+change\s+goes\s+in\s+the\s+same\s+commit' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit. Task 4's scope-check exemption depends on this sentence existing.

- [ ] **Step 4: Read the Implement sub-step**

Read `SKILL.md` sub-step 4 in full. Confirm it now has six bullets in this order: `AGENTS.md` conventions, catch shape, reference search, linter, tests, do-not-commit. Confirm every bullet is at the same indent.

Commit message, for an executor who has been authorized to commit:
```
feat(claude,skill): review-and-fix: bound fix shape before the edit
```

---

### Task 4: The staged-diff scan sub-step

What remains of the post-commit checklist after the judgment items moved before the edit. Four checks, all mechanical, none a judgment call. This is what gates the commit.

**Files:**
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md` (insert a new item 5 before the Commit sub-step, renumber items 5-7)

**Interfaces:**
- Consumes: the catch rule and the reference-search call-site list from Task 3. The `Red:` convention from Task 2.
- Produces: final sub-step numbering, 1 through 8.

- [ ] **Step 1: Before-check the insert point**

Run:
```
rg -U -n '^5\.\s+\*\*Commit\s+the\s+fix' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit. Confirms Task 2's renumber landed and Commit is currently item 5. The new sub-step goes immediately before this line. If this returns 0 hits, Task 2 did not complete. STOP.

- [ ] **Step 2: Insert the new sub-step 5**

Insert immediately before `5. **Commit the fix:**`:

```markdown
5. **Scan the staged diff before committing.** Run `git diff --staged` and read the added
   lines only. Four checks, all mechanical, none of them a judgement call. Fix whatever a
   check catches, re-stage, and rerun the scan. Never commit with a note to fix it later. A
   noted violation is next iteration's finding, which is the cost this scan exists to remove.
   - **Pattern scan, authored prose and added lines only.** No `→ ← … ≥ ≤ × — –` and no curly
     quotes. No ` - ` joining two independent clauses. No ticket key matching
     `\b[A-Z][A-Z0-9]{1,9}-\d{1,6}\b`, excluding protocol names such as UTF-8, SHA-256,
     RFC-7231, ISO-8601, and CVE-2024-1234. None of the changelog literals `added this`,
     `changed from`, `new logic`, `was previously`, `remove old impl`. No added `as any` and
     no added `as unknown as`. Literal content is exempt, so one of these glyphs inside a
     string, a fixture, or quoted output is not a violation.
   - **Catch artifact.** Every added `catch` rethrows or carries a why-comment (sub-step 4).
   - **`Red:` presence.** The commit body carries a `Red:` line when this was a behavior
     finding (sub-step 3).
   - **Scope.** `git diff --staged --stat` lists only files the finding named or the
     reference search in sub-step 4 turned up. For any other file, state why in one line.
```

**The glyphs in bullet 1 are intentional literal content.** They are the scan's targets. Do not replace them with ASCII, and do not let a later prose-cleanup pass sweep them.

- [ ] **Step 3: Renumber the three following sub-steps**

| Was | Becomes | Title |
|---|---|---|
| `5.` | `6.` | `**Commit the fix:**` |
| `6.` | `7.` | `**Record what this commit was**` |
| `7.` | `8.` | `After the bookkeeping, move to the next finding.` |

- [ ] **Step 4: After-check the insert and the renumber**

Run:
```
rg -U -n '^5\.\s+\*\*Scan\s+the\s+staged\s+diff\s+before\s+committing\.\*\*' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit.

Run:
```
rg -U -n '^6\.\s+\*\*Commit\s+the\s+fix' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit.

Run:
```
rg -U -n '^7\.\s+\*\*Record\s+what\s+this\s+commit\s+was' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit.

Run:
```
rg -U -n '^8\.\s+After\s+the\s+bookkeeping' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit.

Run:
```
rg -U -n '^5\.\s+\*\*Commit' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 0 hits.

Run:
```
rg -U -n 'Never\s+commit\s+with\s+a\s+note\s+to\s+fix\s+it\s+later' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit.

Run:
```
rg -U -n 'the\s+reference\s+search\s+in\s+sub-step\s+4\s+turned\s+up' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: 1 hit. This is the exemption that stops the scope check fighting Task 3's same-commit rule on every propagation fix.

- [ ] **Step 5: Read the whole list one final time**

Read `SKILL.md` from `### For each finding:` through the end of sub-step 8. Confirm:
- The numbers run 1 through 8 with no gap and no duplicate.
- Sub-step 5's four bullets are present and the intro says "Four checks".
- Sub-step 5's references to sub-step 3 and sub-step 4 point at the red test and Implement respectively.
- Sub-step 3's reference to "sub-step 4" still points at Implement.

Commit message, for an executor who has been authorized to commit:
```
feat(claude,skill): review-and-fix: gate the commit on a mechanical diff scan
```

---

### Task 5: The per-iteration summary lines

Adds the fifth non-commit outcome so a skipped finding is reportable, and the finding count so the change can be judged by running it. Nothing here is verifiable at authoring time, and the count is the only measurement.

**Files:**
- Modify: `shared_config/.claude/skills/review-and-fix/SUMMARY.md:18-23`

**Interfaces:**
- Consumes: `skipped_findings` from Task 1.
- Produces: nothing. This is the last task.

- [ ] **Step 1: Before-check both sites**

Run:
```
rg -U -n 'abandoned\s+because\s+lint\s+or\s+tests\s+failed,\s+or\s+moot\s+because\s+an' shared_config/.claude/skills/review-and-fix/SUMMARY.md
```
Expected: 1 hit at line 22.

Run:
```
rg -U -n 'A\s+shortfall\s+that\s+a\s+later\s+relaunch\s+cleared\s+is\s+recorded\s+here\s+and\s+nowhere\s+else' shared_config/.claude/skills/review-and-fix/SUMMARY.md
```
Expected: 1 hit at line 20. This is the end of the reviewers bullet and the anchor for the count line.

If either returns 0 hits, STOP and report drift.

- [ ] **Step 2: Add the fifth non-commit outcome**

Replace lines 21-23:

```
- Every finding kept by the `>=50` filter that did NOT become a commit, and what happened to
  it, meaning deferred, dismissed, abandoned because lint or tests failed, or moot because an
  earlier commit this iteration already fixed it. Committed findings are the table below instead.
```

with:

```
- Every finding kept by the `>=50` filter that did NOT become a commit, and what happened to
  it, meaning deferred, dismissed, abandoned because lint or tests failed, skipped because no
  test was possible, or moot because an earlier commit this iteration already fixed it.
  Committed findings are the table below instead.
```

- [ ] **Step 3: Add the finding-count line**

Insert as a new bullet directly after the reviewers bullet that ends at line 20, before the non-commit-outcomes bullet:

```markdown
- The count of findings kept by the `>=50` filter, so the trend across iterations is visible
  in one place. A run whose count is not falling is the signal to interrupt.
```

- [ ] **Step 4: After-check both sites**

Run:
```
rg -U -n 'skipped\s+because\s+no\s+test\s+was\s+possible,\s+or\s+moot' shared_config/.claude/skills/review-and-fix/SUMMARY.md
```
Expected: 1 hit.

Run:
```
rg -U -n 'The\s+count\s+of\s+findings\s+kept\s+by\s+the\s+.>=50.\s+filter,\s+so\s+the\s+trend' shared_config/.claude/skills/review-and-fix/SUMMARY.md
```
Expected: 1 hit.

Run:
```
rg -U -n 'lint\s+or\s+tests\s+failed,\s+or\s+moot' shared_config/.claude/skills/review-and-fix/SUMMARY.md
```
Expected: 0 hits. A hit means the old wording survived.

- [ ] **Step 5: Read the bullet list**

Read `SUMMARY.md` lines 15-35. Confirm the "Cover these in this order" list still reads as an ordered sequence, the new count bullet sits between the reviewers bullet and the non-commit-outcomes bullet, and the summary is still described as terse.

Commit message, for an executor who has been authorized to commit:
```
feat(claude,skill): review-and-fix: report skipped findings and the finding count
```

---

## Final verification, after all five tasks

- [ ] **Sub-step integrity**

Run:
```
rg -U -n '^[0-9]\.\s+\*\*' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: the "For each finding" list contributes exactly items 1 through 7 with bold titles, plus item 8 which has no bold title. Confirm no duplicate numbers anywhere in the file.

- [ ] **No dangling references**

Run:
```
rg -U -n 'sub-step\s+[0-9]' shared_config/.claude/skills/review-and-fix/
```
Expected: every hit names a sub-step that exists in the final 1-through-8 list. Specifically, sub-step 3 is the red test and sub-step 4 is Implement. No reference to a sub-step above 8.

- [ ] **The deliberate glyphs survived**

Run:
```
rg -n '→\s+←\s+…\s+≥\s+≤\s+×\s+—\s+–' shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: exactly 1 hit, the enumerated bullet inside sub-step 5. A 0 means something normalized
the scan's own targets to ASCII, which silently guts the check that bullet describes.

Match the exact glyph sequence, not a count of glyph-bearing lines. A count is the wrong shape
here: the file legitimately carries about 25 other lines using an em-dash as a definition dash
after a bullet label, which AGENTS.md allows, so any expected count has to be re-derived every
time the file grows. During this plan's execution a count-based version of this check was given
to an implementer with an expected value of 1 and returned 26, which cost a round trip to
diagnose as a bad check rather than a bad file.

- [ ] **`skipped_findings` is defined and used**

Run:
```
rg -U -n 'skipped_findings' shared_config/.claude/skills/review-and-fix/
```
Expected: exactly 3 hits on the identifier. Defined once in `STATE.md`, written once in `SKILL.md` sub-step 3, and read once in `SKILL.md`'s Step 2 skip list. `SUMMARY.md` reflects the same state in prose ("skipped because no test was possible") and deliberately does not name the identifier, so it contributes no hit here. Fewer than 3 means a task did not land. More than 3 means a duplicate definition.

- [ ] **The skill still parses as one readable program**

Read `SKILL.md` Step 2 in full, start to finish, as an orchestrator would. Ask the plan's real correctness question: would two orchestrators reading this behave the same way? Specifically, is it unambiguous when the red test is required versus skipped, and is it unambiguous what blocks the commit?

- [ ] **Do not commit**

Confirm `git status --porcelain` shows the three modified files as unstaged or staged but uncommitted, per the human's instruction. Report the diff summary and stop.

## Self-review of this plan against the spec

Checked, with results recorded rather than asserted clean:

**Spec coverage.** All nine spec edits map to a task. Edit 1 to Task 1 Step 4. Edit 2 to Task 2 Step 2. Edit 3 to Task 3 Step 2. Edit 4 to Task 4 Step 2. Edit 5 to Task 2 Step 4. Edit 6 to Task 1 Step 5. Edit 7 to Task 1 Step 3. Edit 8 to Task 5 Step 2. Edit 9 to Task 5 Step 3. No gaps.

**Placeholder scan.** No TBD, no TODO, no "similar to Task N", no "add appropriate handling". Every edit carries its verbatim replacement text. The `<type>`, `<line>`, and `<first line of the failing assertion>` placeholders inside quoted skill text are template slots in the skill's own commit template, which is correct and intentional.

**Consistency.** Sub-step numbers were the live risk, since two tasks insert into one numbered list. The renumbering table above states the final state once and every task's after-check asserts it positively (the new number exists) and negatively (the old number is gone). Task 2 Step 5 and Task 4 Step 4 both include the zero-hit check that catches a half-applied renumber.

**One deliberate deviation from the writing-plans template.** The template's Step 5 is "Commit". The human said not to commit anything, so each task ends at verification and the commit message is recorded as a labelled artifact for a later authorized executor rather than as a checkbox step. The Global Constraints state this.

**One known ordering hazard.** Task 2's inserted text references `skipped_findings`, and Task 1 defines it. Running Task 2 first leaves a dangling reference. The ordering section states this as forced, and Task 4 Step 1 has a hard STOP if Task 2 has not completed. Task 2 has no equivalent guard against Task 1 being skipped, so its Step 5 includes the `Record a skip in skipped_findings` after-check as the closest available signal.
