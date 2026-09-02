# work-on Files Follow-up Tickets Through open-ticket Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let `work-on` file the scope it cuts as a real Jira ticket, by delegating the create to `open-ticket` instead of growing a second create path.

**Architecture:** `open-ticket` gains a delegated entry for a caller that already knows the project, the scope and the files, and skips only the steps that answers. `work-on` gains two places where a follow-up is proposed and filed. Every field id and every create recipe stays in `CREATE-FIELDS.md`, which is the single copy.

**Tech Stack:** Markdown skill files. Bash plus `grep` and `tr` for the contract test. No new dependencies and no new files.

**Spec:** `docs/superpowers/specs/2026-08-23-work-on-follow-up-tickets-design.md`

## Global Constraints

Every task's requirements implicitly include this section.

**Prose voice**, from `shared_config/.claude/skills/writing-work-docs/SKILL.md` and `AGENTS.md`. These govern every line of markdown this plan writes.

- No clause joiners. No ` - ` (space-hyphen-space) and no `:` splitting a claim from its elaboration. Write two sentences. Hyphenated words, CLI flags, ranges, a label at the start of a line (`Note:`, `Step 0:`), times, ratios, and anything inside code or a path are untouched.
- Plain ASCII only. `->` not the arrow glyph, `...` not the ellipsis glyph, `>=` and `<=`, straight quotes. Two sentences instead of an em dash.
- Banned words: outright; crucial, pivotal, vital, paramount; comprehensive, holistic; intricate, nuanced; meticulous; granular; utilize; facilitate; foster, cultivate; harness, unlock, empower, elevate; streamline; showcase; socialize, operationalize, ideate; moreover, furthermore, additionally; notably, importantly, crucially; essentially, fundamentally, ultimately, arguably; landscape, realm, testament; myriad, plethora; journey, synergy; game-changer; delve, dive into; unpack; circle back; robust, powerful, seamless, seamlessly; ensure, enhance; cutting-edge.
- **Calibration, because an earlier project over-applied two rules.** `surface` as a NOUN meaning the scope of a change is correct usage in this repo and `work-on/SKILL.md` uses it that way six times. `rather than` appears 30 times in `work-on/SKILL.md` and 5 times in `AGENTS.md`. Both rules target a sentence whose whole claim is a contrast with a foil, not the bare token. Do not rewrite a sentence merely because it contains one of those phrases.
- Every rule is followed by the specific bad outcome it prevents. This repo rejects a rule with no named consequence.

**Facts that must not be paraphrased.**

| Fact | Value |
|---|---|
| The follow-up's relationship to the originating ticket | Sibling, never child |
| Parent to use | The originating ticket's own `parent`, or none if it has none |
| Sprint in delegated mode | Not supplied and not inferred |
| A fix abandoned because lint or tests failed | Never becomes a follow-up |
| The Trigger B cap | More than three items is reported as a count, not filed |
| Trigger A filing point | After Step 4's agreement, before Step 5's rewrite |

**Contract test mechanics**, established by `tests/open_ticket_contract_test.sh` and binding on any assertion this plan adds.

- A HEADING assertion uses `assert_line_count` with a fully anchored pattern against the raw file. A heading needle absorbs into a deeper heading, so an unanchored `## X` matches `### X` and cannot catch a demotion.
- A PROSE-PHRASE assertion uses `assert_contains` against the flattened text, because prose is hand-wrapped and a needle holding a literal space can straddle a newline.
- A needle meant to prove a RULE is stated must appear in a real declarative sentence and not only in a heading. A heading-only match turns a prose assertion into a heading check.
- No needle may be a bare common word, and no needle may match something that would itself be wrong. Six instances of that defect were found in the `open-ticket` project. For every assertion, name the edit it exists to catch and confirm it would catch it.
- `assert_line_count` runs `grep -c`, which is BRE, so `|`, backticks, `<`, `>` and `:` are literal and need no escaping.
- Helpers are already defined and must not be redefined. `flatten()` at line 22, `require_file()` at line 24, `assert_line_count()` at line 35.
- Every new assertion goes before the final `echo "open_ticket_contract_test: ok"` line, which stays last.

**Bash constraints**, from `AGENTS.md`. `git` runs outside the sandbox and must run ALONE, no pipe and no chain. Redirect to a file under `/tmp/claude/` and read it in a separate call. No top-level `git -C`. No inline scripting. No command substitution in commands you run. No `$TMPDIR` in any form. One command per call. Run the test as exactly `bash tests/open_ticket_contract_test.sh` from the repo root.

**Commit messages** follow the repo's existing subject style, `feat(claude,skill): ...` and `fix(claude,skill): ...`. End every commit message with the trailer `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`. Never `git add -A` or `git add .`, because the tree carries a pre-existing unrelated submodule change to `shared_config/.oh-my-zsh` and an untracked `docs/` directory that must not be committed.

---

## File Structure

| File | Responsibility for this change |
|---|---|
| `shared_config/.claude/skills/open-ticket/SKILL.md` | The delegated entry contract, and the parent-exclusion rule inside the Step 6 gate |
| `shared_config/.claude/skills/work-on/SKILL.md` | Trigger A at Step 4 plus the filing step before Step 5, and Trigger B at the end of Step 8 |
| `tests/open_ticket_contract_test.sh` | Assertions pinning the delegated contract and the sibling-parenting rule |

No new files. No CI change, because the one test file this touches already has a job.

**Known insertion points, verified against the current files.**

- `open-ticket/SKILL.md`: `## Step 0: Preflight` at line 83, `## Step 6: Duplicate gate` at line 254, its "A match is not credible on any one of these alone" list ending around line 269.
- `work-on/SKILL.md`: the Step 4 "Partially valid" paragraph at lines 690 to 698, `## Step 5: Rewrite the ticket` at line 700, `## Step 8: review-and-fix, all the way` at line 931, `### When review-and-fix aborts with REVIEW_UNAVAILABLE_NO_FANOUT` at line 960.
- `tests/open_ticket_contract_test.sh`: the final `echo` at the last line, currently 310.

**Task order is fixed.** Task 1 publishes the contract that Tasks 2 and 3 consume. A reviewer can reject Task 3 while approving Tasks 1 and 2, which is why Trigger B is its own task rather than folded into Task 2.

**A coverage gap this plan does not close, stated once here so no task implies otherwise.** `work-on` has no contract test in the sense `open-ticket` has one.

What it does have is narrow. `tests/work_on_validation_test.sh` extracts the embedded javascript from `VALIDATION.md` and exercises one reducer, and it additionally asserts that six strings are present in `work-on/SKILL.md`. All six belong to the triage feature and none of them touches anything this change adds. So Tasks 2 and 3 run under a test that will not fail on their work and will not vouch for it either.

Their real verification is the voice greps and a read-through against the spec. Both are checks that pass or fail, and neither runs on a pull request. The spec puts a `work-on` contract test deliberately out of scope, so every rule Tasks 2 and 3 add rests on review. Do not read a green suite after Task 3 as coverage of Task 3.

---

## Task 1: open-ticket's delegated entry and the parent exclusion

**Files:**
- Modify: `shared_config/.claude/skills/open-ticket/SKILL.md` (new section after Step 0's numbered checks, and an addition inside `## Step 6: Duplicate gate` at line 254)
- Modify: `tests/open_ticket_contract_test.sh` (append a block before the final `echo`)

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces: the delegated contract that Tasks 2 and 3 both invoke. Its exact terms are the five supplied values in Step 3 below, the heading `## Delegated entry`, and the phrase `sibling of the originating ticket`. Tasks 2 and 3 reference the heading by name and must not restate the contract's rules.

- [ ] **Step 1: Write the failing assertions**

Append this block to `tests/open_ticket_contract_test.sh`, immediately before the final `echo "open_ticket_contract_test: ok"` line.

```bash
# The delegated entry is what work-on calls. Drop the section and work-on's two
# filing paths have no contract to call, while nothing in this suite notices.
assert_line_count "sk_has_delegated_entry" '^## Delegated entry$' 1 "$SKILL_MD"

# Sibling and never child is a Jira validity rule, not a preference. open-ticket
# parents a Story to an Epic, so a follow-up Story filed under a Story either
# fails the create or forces the follow-up to be a subtask of work it is not
# part of. This is the single most expensive sentence in the section to lose.
assert_contains "sk_followup_is_a_sibling" "sibling of the originating ticket" "$SKILL_FLAT"

# The five supplied values. Each one names a step it satisfies, so dropping a row
# silently puts a step back in the delegated path that the caller already answered.
assert_contains "sk_delegated_supplies_project" "project key" "$SKILL_FLAT"
assert_contains "sk_delegated_supplies_files" "the files involved" "$SKILL_FLAT"
assert_contains "sk_delegated_supplies_origin" "originating ticket" "$SKILL_FLAT"

# Preflight, the sweep and the gate always run. A caller can vouch for its own
# scope decision and cannot vouch for Jira access or for whether somebody already
# filed the thing, so skipping either would file a duplicate under a clean verdict.
assert_contains "sk_delegated_still_runs_preflight" "Step 0's preflight always runs" "$SKILL_FLAT"
assert_contains "sk_delegated_still_runs_sweep" "the sweep and the gate always run" "$SKILL_FLAT"

# No sprint in delegated mode. A follow-up lands in the backlog until somebody
# schedules it, and Step 2's open-sprint query is about the caller's current work.
assert_contains "sk_delegated_no_sprint" "no sprint in delegated mode" "$SKILL_FLAT"

# The parent exclusion, inside the Step 6 gate. Without it the sweep finds the
# originating ticket, which is by construction about the same area and often the
# same files, and the gate aborts on the ticket the caller is mid-way through.
assert_contains "sk_origin_not_a_duplicate" "never a credible match" "$SKILL_FLAT"

# A sibling follow-up already carved off the same origin IS a credible match, and
# is the most valuable hit the sweep can return here. Losing this turns the
# exclusion from one key into a blanket amnesty for the whole area.
assert_contains "sk_sibling_still_counts" "a sibling follow-up already filed" "$SKILL_FLAT"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/open_ticket_contract_test.sh`

Expected: FAIL on the first new assertion, printing

```
[sk_has_delegated_entry] expected: 1
             actual:   0
```

The assertions after it are not reached, because `assert_eq` exits on the first failure.

- [ ] **Step 3: Write the delegated entry section**

Add a `## Delegated entry` section to `open-ticket/SKILL.md`, placed after Step 0's numbered checks end and before `## Step 1: Read the request`. It is a narrowing of the existing pipeline and never a second pipeline, so say that.

The section must contain, each with its consequence:

1. **What a caller supplies**, as a table with two columns, the supplied value and the step it satisfies. Five rows, exactly these: `project key` satisfies Step 2's project inference. `the requirement text` satisfies Step 1. `the files involved` satisfies Step 4's exploration. `the originating ticket's key` satisfies Step 5's exclusion and the description's context line. `the originating ticket's own parent, when it has one` satisfies Step 7's parenting.

2. **What always runs regardless.** Use the phrase `Step 0's preflight always runs` and the phrase `the sweep and the gate always run`, both as ordinary prose in real sentences. The reason for preflight is that a caller can vouch for its own scope decision and cannot vouch for Jira access. The reason for the sweep and the gate is that the caller's scope decision says nothing about whether somebody already filed the same work, so skipping them would file a duplicate under a clean verdict. Step 3's sizing and Steps 7 through 12 also run unchanged.

3. **Sibling and never child.** Use the phrase `sibling of the originating ticket`. State the Jira validity reason: this skill parents a `Story`, `Task` or `Bug` to an `Epic` and parents a subtask type to a `Story`, so a follow-up `Story` cannot take a `Story` as its parent. Filing it under the originating ticket either fails the create or forces the follow-up to be a subtask of work it is not part of. Then the rule: read the originating ticket's own `parent`, take the same parent when it has one, and file with no parent when it has none. The relationship to the originating ticket is carried in prose in the follow-up's description, because this skill writes no issue links beyond `parent`.

4. **No sprint.** Use the phrase `no sprint in delegated mode`. The reason is that a follow-up belongs in the backlog until somebody schedules it, and Step 2's open-sprint query is about what the caller is working on now.

5. **The approval.** A delegated caller may supply its own recorded human agreement in place of Step 9's gate, and must say so. This is not a bypass. Step 9 exists so no issue is created that no human approved, and a caller that already has that agreement on record satisfies it. A caller with no such agreement gets the normal Step 9 gate. Name the consequence of getting this wrong, which is an issue created on nobody's authority.

Do not restate anything from `CREATE-FIELDS.md`. Link it where a reader needs the recipes.

- [ ] **Step 4: Write the parent exclusion into Step 6**

Add to `## Step 6: Duplicate gate`, after the "A match is not credible on any one of these alone" list ends around line 269 and before the "Borderline goes to the user" paragraph.

Two rules, each with its consequence.

- The originating ticket supplied by a delegated caller is **`never a credible match`**, using that exact phrase in a real sentence. The reason: the follow-up is by construction about the same area and often the same files, so the sweep finds it every time, and the gate would abort with `DUPLICATE_FOUND` naming the ticket the caller is mid-way through fixing.
- **`a sibling follow-up already filed`** off the same originating ticket stays credible, using that exact phrase. The reason: it is the most valuable hit the sweep can return in a delegated run, and it is the same case `work-on` Step 4 already detects from the other direction. Say plainly that the exclusion covers one key and is not an amnesty for the area.

- [ ] **Step 5: Run the test to verify it passes**

Run: `bash tests/open_ticket_contract_test.sh`

Expected: PASS, printing `open_ticket_contract_test: ok`, exit 0.

- [ ] **Step 6: Prove two assertions can fail**

An assertion nobody has seen fail is an assertion nobody has tested, and six unfalsifiable assertions were found in the `open-ticket` project.

Demote the `## Delegated entry` heading to `### Delegated entry`, run the suite, confirm `sk_has_delegated_entry` fails with expected 1 actual 0, then restore it.

Then delete the sentence containing `never a credible match`, run the suite, confirm `sk_origin_not_a_duplicate` fails, then restore it.

Put both outputs in your report. Confirm with `git diff` that nothing unintended survived either mutation.

- [ ] **Step 7: Check the prose**

Run: `grep -nioE '\b(outright|crucial|pivotal|paramount|comprehensive|holistic|intricate|meticulous|granular|utilize|facilitate|foster|unlock|empower|elevate|streamline|showcase|moreover|furthermore|essentially|fundamentally|arguably|myriad|plethora|synergy|delve|robust|seamless|ensure|enhance|cutting-edge)\b' shared_config/.claude/skills/open-ticket/SKILL.md`

Expected: no output. Any hit is a banned word and gets rewritten. Note `surface` and `rather than` are deliberately absent from this pattern, per the calibration in Global Constraints.

Run: `grep -nE ' - |—|–|→|…|≥|≤' shared_config/.claude/skills/open-ticket/SKILL.md`

Expected: no output.

- [ ] **Step 8: Commit**

```bash
git add shared_config/.claude/skills/open-ticket/SKILL.md tests/open_ticket_contract_test.sh
```

```bash
git commit -m "feat(claude,skill): open-ticket: a delegated entry for a caller that knows the scope

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: work-on Trigger A, the scope cut at Step 4

**Files:**
- Modify: `shared_config/.claude/skills/work-on/SKILL.md` (the Step 4 "Partially valid" paragraph at lines 690 to 698, a new subsection before `## Step 5: Rewrite the ticket` at line 700, and Step 5's description-structure block)

**Interfaces:**
- Consumes: Task 1's `## Delegated entry` section in `open-ticket/SKILL.md`. Reference it by that heading and do not restate its rules, since it is the single copy.
- Produces: the follow-up key that Step 5's "Out of scope" section names. Task 3 reuses the same filing subsection rather than adding a second one.

There is no test step in this task. `work-on` has no contract test, as stated in File Structure. Verification is the voice greps in Step 5 and a read-through against the spec.

- [ ] **Step 1: Extend Step 4's "Partially valid" branch**

That branch already presents the split, the evidence on both sides, and the proposed reduced scope, and already gets explicit agreement before Step 5. Add the follow-up proposal to that same presentation, so no new prompt appears anywhere in the run.

The proposal shows four things and nothing else. The issue type, a one-line summary, the points, and the parent it would take. Keep it to the shape already used elsewhere in the file for a compact proposal.

State why the agreement covers the create, because a reader will otherwise read this as a missing gate. `work-on` already rewrites the parent ticket's whole description with no preview, on its own stated rule that it asks about decisions and never about wording. Whether to file a follow-up for cut scope is a decision and the user makes it here. The follow-up's prose is wording.

State what a decline means. The user can agree the reduced scope and decline the follow-up, in which case Step 5 describes the cut work in prose with no key, exactly as it does today.

- [ ] **Step 2: Add the filing subsection**

Add a `### Filing the follow-up` subsection at the end of Step 4, immediately before `## Step 5: Rewrite the ticket`.

It must carry, each with its consequence:

- **It runs after the agreement and before Step 5.** The reason is that Step 5's "Out of scope" section names the new key, and a key that does not exist yet cannot be named. This is what makes the existing line about the work not being dropped true rather than aspirational.
- **It invokes `open-ticket` through its `## Delegated entry`.** Name the five values to supply, by reference to that section rather than by restating its table. The project key comes from the originating ticket's key. The requirement text is the cut scope as agreed. The files come from what validation already read. The originating ticket's key is this ticket. The parent is the originating ticket's own parent, read from Jira, or none.
- **The recorded Step 4 agreement is what satisfies `open-ticket`'s Step 9.** Say that the agreement is passed as the delegated approval, and that a run which somehow reaches this subsection without one goes through the normal Step 9 gate rather than creating anything.
- **It runs in the main thread.** Never inside a `Workflow`. `open-ticket` sits in `DENIED_SKILLS` in `deny-review-in-workflow.py`, so a workflow naming it is denied, and a workflow agent could not put a gate in front of a human anyway. Step 2 is the contrast rather than the precedent, exactly as Step 8 already argues for `review-and-fix`.
- **A failed create never blocks Step 5.** If the follow-up cannot be filed, Step 5 still runs and describes the cut work in prose with no key, and the final report says the follow-up could not be filed and why. The reason this matters: the parent ticket's rewrite is the run's main deliverable and a follow-up is an addition to it.
- **A follow-up over 5 points is reported, not split.** `open-ticket`'s points rule still applies. A cut scope that sizes above 5 means the split at Step 4 was too coarse, so say so and let the user decide rather than silently filing a tree.

- [ ] **Step 3: Make Step 5's "Out of scope" name the key**

Step 5's description-structure block lists `Out of scope` as "what it does not, especially anything Step 4 cut". Extend that line so it names the follow-up key when one was filed.

Add one sentence stating the consequence of omitting it. A reader who sees cut scope with no key cannot tell whether the work was dropped or moved, and the whole reason Step 4 names an existing follow-up's key is to answer that question.

- [ ] **Step 4: Read the result against the spec**

Read the spec's section 3, `Trigger A, the scope cut at Step 4`, and the diff you just wrote, side by side. Confirm every rule in that section appears in the file and that nothing appears in the file that the section does not sanction.

This is the only correctness check this task has, because `work-on` has no contract test. Report what you compared and anything you could not find a home for.

- [ ] **Step 5: Check the prose**

Run: `grep -nioE '\b(outright|crucial|pivotal|paramount|comprehensive|holistic|intricate|meticulous|granular|utilize|facilitate|foster|unlock|empower|elevate|streamline|showcase|moreover|furthermore|essentially|fundamentally|arguably|myriad|plethora|synergy|delve|robust|seamless|ensure|enhance|cutting-edge)\b' shared_config/.claude/skills/work-on/SKILL.md`

Expected: no output. `surface` and `rather than` are deliberately absent from the pattern, because this file uses both correctly and often.

Run: `grep -nE ' - |—|–|→|…|≥|≤' shared_config/.claude/skills/work-on/SKILL.md`

Expected: no output.

Run: `bash tests/open_ticket_contract_test.sh`

Expected: `open_ticket_contract_test: ok`. This task changes no assertion, so a failure here means Task 1 was disturbed.

Run: `bash tests/work_on_validation_test.sh`

Expected: passes, ending with `work_on_validation_test: ok`.

That test does read `work-on/SKILL.md`, at line 17, and it asserts two things about it. A loop requires the strings `reachability`, `exploit-realism`, `fix-cost`, `TRIAGE_SCHEMA` and `triageRules` each to appear at least once, and a later check requires `all three` to appear at least once. Both are presence checks and not exact counts, so prose you ADD cannot break either one. Run it anyway, because it is the only automated check that reads this file at all, and a failure would mean an edit deleted or reworded something it depends on.

- [ ] **Step 6: Commit**

```bash
git add shared_config/.claude/skills/work-on/SKILL.md
```

```bash
git commit -m "feat(claude,skill): work-on: file the scope Step 4 cuts as a follow-up ticket

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: work-on Trigger B, the leftovers at Step 8

**Files:**
- Modify: `shared_config/.claude/skills/work-on/SKILL.md` (a new subsection inside Step 8, placed after the "It commits each fix and never pushes" line around line 958 and before `### When review-and-fix aborts with REVIEW_UNAVAILABLE_NO_FANOUT` at line 960)

**Interfaces:**
- Consumes: Task 1's `## Delegated entry`, and Task 2's `### Filing the follow-up` subsection in Step 4. Reuse that subsection by reference rather than writing a second filing path, because two filing paths would drift.
- Produces: nothing later tasks read. This is the last task.

There is no test step in this task, for the same reason as Task 2.

This is the riskier of the two triggers and the risk was named during design and accepted. Its mitigations are the two buckets, the required batch approval, and the cap. All three are load-bearing and none of them is optional.

- [ ] **Step 1: Write the classification subsection**

Add a `### Follow-ups from what review-and-fix left` subsection inside Step 8, at the position named above.

Open it with the fact that makes this step necessary, because an implementer who does not know it will look for a field that does not exist. **`review-and-fix` has no out-of-scope category.** Its "Remaining Issues" section covers findings that did not become a commit, and the reasons it names are a row 2 stop, a fix abandoned because lint or tests failed, and a finding skipped for want of a test. Its "Tickets examined" section carries deferred and dismissed ticket findings with a decision. Nothing in either section says a finding belongs to a different ticket, so the classification is `work-on`'s own judgement against the scope agreed at Step 4.

- [ ] **Step 2: Write the two buckets**

Every item in those two sections goes into exactly one of two buckets. Name both, and name what happens to each.

**Unfinished here.** A fix abandoned because lint or tests failed. A finding skipped for want of a test. A row 2 stop. None of these becomes a ticket. State the consequence in full, because this is the rule most likely to be violated by a well-meaning run: Step 8 says run `review-and-fix` all the way, and filing a ticket for a test failure converts a failure into a backlog item that reads like planned work, so the failure stops being visible as a failure.

**Belongs to a different ticket.** A finding about code the scope agreed at Step 4 does not cover. Only these are proposed.

State that an item you cannot confidently sort belongs in the first bucket. The cost of leaving real out-of-scope work unfiled is that somebody finds it later. The cost of filing unfinished work as a ticket is that the branch ships with a known gap wearing a ticket number.

- [ ] **Step 3: Write the batch approval and the cap**

**The batch approval.** The second bucket is proposed as one batch with one yes, at the end of Step 8. State why this trigger needs its own approval when Trigger A does not: the findings did not exist when the user agreed scope at Step 4, so that agreement cannot cover them. Nothing is proposed when the second bucket is empty, and the run says nothing about it rather than reporting an empty list.

**The cap.** More than three items in the second bucket is reported as a count and a signal about the change, not filed as three or more tickets. State the reason: findings from Step 8 are machine-generated and carry no human scope call of their own, so a noisy review round would otherwise turn one `work-on` run into a stack of tickets nobody asked for. Do not file silently past the cap and do not truncate the list without saying so, because a silent truncation reads as "that was all of them".

**Filing.** Reuse Step 4's `### Filing the follow-up` subsection by name. The one difference to state: the originating ticket's key is supplied for the dedup exclusion and the description's context line, and the sibling-parenting rule from `open-ticket`'s `## Delegated entry` applies unchanged. A finding about adjacent code is a sibling of this ticket at best and never a child of it.

- [ ] **Step 4: Read the result against the spec**

Read the spec's sections 4 and 5, `Trigger B, the leftovers at Step 8` and `The cap`, against the diff you just wrote. Confirm every rule appears and that the two buckets' membership matches the spec exactly, item for item. A bucket that has drifted is the failure mode the spec's own "Known risk, accepted" section names.

Report what you compared.

- [ ] **Step 5: Check the prose**

Run: `grep -nioE '\b(outright|crucial|pivotal|paramount|comprehensive|holistic|intricate|meticulous|granular|utilize|facilitate|foster|unlock|empower|elevate|streamline|showcase|moreover|furthermore|essentially|fundamentally|arguably|myriad|plethora|synergy|delve|robust|seamless|ensure|enhance|cutting-edge)\b' shared_config/.claude/skills/work-on/SKILL.md`

Expected: no output.

Run: `grep -nE ' - |—|–|→|…|≥|≤' shared_config/.claude/skills/work-on/SKILL.md`

Expected: no output.

Run: `bash tests/open_ticket_contract_test.sh`

Expected: `open_ticket_contract_test: ok`.

Run: `bash tests/work_on_validation_test.sh`

Expected: passes, ending with `work_on_validation_test: ok`.

That test reads `work-on/SKILL.md` at line 17 and asserts the strings `reachability`, `exploit-realism`, `fix-cost`, `TRIAGE_SCHEMA` and `triageRules` each appear at least once, plus that `all three` appears at least once. Both are presence checks and not exact counts, so prose you ADD cannot break either. It is still the only automated check that reads this file, so a failure means an edit removed or reworded something it depends on.

- [ ] **Step 6: Commit**

```bash
git add shared_config/.claude/skills/work-on/SKILL.md
```

```bash
git commit -m "feat(claude,skill): work-on: propose follow-ups for what review-and-fix leaves out of scope

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage.** Every spec section maps to a task.

| Spec section | Task |
|---|---|
| 1, the delegated entry, all five supplied values | 1 |
| 1, sibling and never child, with the Jira validity reason | 1 |
| 1, no sprint in delegated mode | 1 |
| 1, what always runs regardless | 1 |
| 2, the parent is never a duplicate | 1 |
| 2, a sibling follow-up stays credible | 1 |
| 3, Trigger A proposed inside Step 4's decision | 2 |
| 3, filed between Step 4 and Step 5 | 2 |
| 3, Step 5's Out of scope names the key | 2 |
| 3, a failed create never blocks Step 5 | 2 |
| 4, review-and-fix has no out-of-scope category | 3 |
| 4, the two buckets and their membership | 3 |
| 4, a lint or test failure is never a follow-up | 3 |
| 4, the batch approval and why it is needed | 3 |
| 5, the cap | 3 |
| Out of scope list | none, correctly. Each item is something no task does. |
| Testing gap on work-on | File Structure, and restated in Tasks 2 and 3 |

**Two spec items the plan makes more explicit than the spec did.** The spec's section 1 does not say what happens when a delegated caller has no recorded human agreement, so Task 1 Step 3 item 5 states it: that caller gets the normal Step 9 gate. And the spec does not say where an unsortable Step 8 item goes, so Task 3 Step 2 states it: the first bucket, with the reason. Both are recorded here rather than left as silent additions.

**Placeholder scan.** No TBD, no TODO, no "implement later", no "similar to Task N", no "add appropriate error handling". Every assertion is written out in full. Every prose deliverable is specified by the exact phrases it must contain and the consequence each rule must name, which is what makes the contract assertions in Task 1 able to check it. Every test step carries the exact command and the exact expected output.

**Type consistency.** The heading `## Delegated entry` is used identically in Task 1 (which creates it) and in Tasks 2 and 3 (which reference it). The subsection `### Filing the follow-up` is created in Task 2 Step 2 and referenced by that exact name in Task 3 Step 3. Every phrase an assertion pins in Task 1 Step 1 appears verbatim in the corresponding item of Task 1 Step 3 or Step 4: `sibling of the originating ticket`, `Step 0's preflight always runs`, `the sweep and the gate always run`, `no sprint in delegated mode`, `never a credible match`, `a sibling follow-up already filed`. `assert_line_count`, `assert_contains`, `flatten` and `require_file` match the real helpers at lines 35, the helper on line 22, and the helper on line 24 of `tests/open_ticket_contract_test.sh`, and `assert_eq` and `assert_contains` match the real signatures in `tests/test_helpers.sh`, which are (label, expected, actual) and (label, needle, haystack).

**One thing a reviewer should push on.** Tasks 2 and 3 have no CI coverage and their verification is a read-through. That is stated three times in this plan on purpose, because a green suite after Task 3 proves only that Task 1 still holds.
