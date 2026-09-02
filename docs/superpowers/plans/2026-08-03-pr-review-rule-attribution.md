# pr-review Rule Attribution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop `pr-review` posting AGENTS.md as the source of a finding, so a teammate does not open the project's AGENTS.md, find nothing, and conclude the finding was invented.

**Architecture:** Three prose edits to one file. Task 1 adds a posting-boundary rule that rewrites rule attribution, plus a pointer so inline comments get the same treatment. Task 2 closes a second, independent leak where the global template prints the finding's category, which for these findings is literally the string `AGENTS.md`. `in-depth-review` is deliberately untouched.

**Tech Stack:** Markdown only. No executable code, so no test runner. The red/green cycle is a `grep` that must miss before an edit and hit after it.

**Spec:** `docs/superpowers/specs/2026-08-03-pr-review-rule-attribution-design.md`

## Global Constraints

- **One target file, both tasks:** `shared_config/.claude/skills/pr-review/SKILL.md`. Nothing else.
- **`in-depth-review/SKILL.md` must NOT be edited.** Role #1 keeps its instruction to "cite the specific AGENTS.md file and the rule text". That citation is what lets the scorer check the rule actually says what the finding claims, and `review-and-fix` shares the role but never posts to GitHub, so there the citation helps rather than confuses. A verification step asserts it is unchanged.
- **Repo root:** `/Users/melvin/.melvin/config`. Run `git` from inside it. Never use top-level `git -C` (a hook denies it).
- **This file hard-wraps.** Wrap new prose at or below 98 columns. Never rewrap a line you did not otherwise change.
- **No em-dash (`—`) or en-dash (`–`) in any added text.** The ban is absolute. A verification step asserts the count is unchanged.
- **No semicolons in added prose.** To grep for one use `rg -c '\x3B'`, because a literal semicolon in a Bash command trips a hook here.
- **No ` - ` (space-hyphen-space) and no clause-splitting `:` as clause joiners.** Split into two sentences.
- **Commit subject prefix:** `pr-review: ` (six commits in recent history use it).
- **Commit footer, every commit:**
  ```
  Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
  ```
- **Stage only the one file.** Never `git add -A`. The repo has an untracked `docs/` tree and two untracked `.patch` files at the root. Leave them alone.
- **Run each `git` command ALONE.** No pipes, no `&&`, no `;`.
- **Do NOT use command substitution** such as `git commit -m "$(cat <<'EOF' ...)"`. A hook denies it. Use a plain multi-line `-m "..."`, or write the message to a file and use `git commit -F <file>`.

---

### Task 1: Add the Rule attribution rule and the inline pointer

Both edits in one task and one commit. The Step 3c pointer refers to the Step 3b block by name, so
shipping the pointer alone leaves a dangling reference.

**Files:**
- Modify: `shared_config/.claude/skills/pr-review/SKILL.md` (insert in Step 3b after the confidence-label block, currently ending line 788, and append a pointer in Step 3c after line 900)

**Interfaces:**
- Consumes: nothing.
- Produces: a Step 3b block whose bolded lead-in is exactly `**Rule attribution (what the reader sees).**`, referred to by Task 1's own Step 3c pointer. Task 2 does not depend on it.

- [ ] **Step 1: Record the baselines (the "failing test")**

Run each from `/Users/melvin/.melvin/config`.

```
rg -c 'Rule attribution' shared_config/.claude/skills/pr-review/SKILL.md
```
Expected: no output, exit 1. The block does not exist yet.

```
rg -c 'AGENTS.md says' shared_config/.claude/skills/pr-review/SKILL.md
```
Expected: no output, exit 1. This phrase appears nowhere today, which is what makes the post-edit
count of exactly 2 meaningful.

```
grep -c '—' shared_config/.claude/skills/pr-review/SKILL.md
```
Expected: `41`. Write it down.

```
grep -c '.\{99,\}' shared_config/.claude/skills/pr-review/SKILL.md
```
Expected: `47`. Write it down.

If any baseline differs, stop and report. The file has moved on from the spec.

- [ ] **Step 2: Insert the Rule attribution block in Step 3b**

The anchor spans the end of the confidence-label block and the start of the fenced global-body
template, which pins the insertion position. **The anchor contains a triple-backtick fence, so the
blocks below are fenced with FOUR backticks to hold it.** Do not copy the outer four-backtick lines
into the file.

Find this exact text:

````
those findings, and a `[<ticket_id>]` title prefix for ticket findings).

```markdown
I used an AI agent with a custom prompt to generate this review.
````

Replace it with:

````
those findings, and a `[<ticket_id>]` title prefix for ticket findings).

**Rule attribution (what the reader sees).** A finding may arrive citing the AGENTS.md file and
rule text it came from. Role #1 of `in-depth-review` is told to cite both, and that citation is
what lets the scorer check the rule says what the finding claims. It is internal mechanics, the
same as the confidence number. **Never post the file name, and never post a phrase like
"AGENTS.md says".** The reviewer's AGENTS.md is usually not in the repo under review, so a reader
goes looking for a rule that is not there and concludes the finding was invented.

State the rule directly instead, as a norm with no source attached, and keep whatever the rule
permits, since the permitted list is what tells the author where the boundary is:

> One thought per sentence, and don't glue two clauses with a colon or a dash. The colon after
> "DB I/O" splits a claim from its elaboration. Hyphenated words, CLI flags, ranges, label
> prefixes and ratios are fine, but this is not one of those.

This governs the title as well as the description, and inline comments as well as global ones.

Two ways to get this wrong. Do not swap the citation for a vaguer authority such as "the project
requires" or "the team convention is". That is worse than naming the file, because it asserts
something about this repo that is not true. And do not soften the rule into a preference, because
the finding still has to read as actionable rather than as one reviewer's taste.

```markdown
I used an AI agent with a custom prompt to generate this review.
````

- [ ] **Step 3: Append the pointer in Step 3c**

Step 3c builds each inline comment from the same `<description>`, so it needs the Step 3b rule
applied. It already carries an `**Approach findings:**` pointer to Step 3b, so this follows that
shape.

Find this exact text:

```
**Approach findings:** for a finding with `source = "approach"`, use the
`` `[approach, confidence: <Low|Medium|High|Critical>]` `` tag here instead. See Step 3b.
```

Replace it with:

```
**Approach findings:** for a finding with `source = "approach"`, use the
`` `[approach, confidence: <Low|Medium|High|Critical>]` `` tag here instead. See Step 3b.

**Rule attribution:** Step 3b's rule applies here unchanged. An inline comment reuses the same
`<description>`, so strip the AGENTS.md file name and any "AGENTS.md says" phrasing here too.
```

- [ ] **Step 4: Verify the content landed**

```
rg -c 'Rule attribution' shared_config/.claude/skills/pr-review/SKILL.md
```
Expected: `2`. One for Step 3b's bolded lead-in, one for Step 3c's pointer.

```
rg -c 'never post a phrase like' shared_config/.claude/skills/pr-review/SKILL.md
```
Expected: `1`.

```
rg -c 'AGENTS.md says' shared_config/.claude/skills/pr-review/SKILL.md
```
Expected: `2`. Both are inside the text you just added, as the quoted banned example in Step 3b and
in Step 3c's pointer. Any other number means either the new text is incomplete or the phrase exists
somewhere it should not.

- [ ] **Step 5: Verify placement and that nothing else moved**

```
rg -n 'Rule attribution \(what the reader sees\)|I used an AI agent with a custom prompt' shared_config/.claude/skills/pr-review/SKILL.md
```
Expected: two lines, with the `Rule attribution` line number LOWER. Confirms the block landed
between the confidence-label block and the global-body template, not after the template.

```
grep -c '—' shared_config/.claude/skills/pr-review/SKILL.md
```
Expected: `41`, unchanged from Step 1.

```
grep -c '.\{99,\}' shared_config/.claude/skills/pr-review/SKILL.md
```
Expected: `47`, unchanged from Step 1. A rise means a line you added exceeded 98 columns.

```
rg -c 'cite the specific AGENTS.md file' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: `1`. Confirms Role #1 still cites the file, which the scorer depends on. This file must
not be edited by this plan.

```
git status --porcelain shared_config/
```
Expected: exactly one line, ` M shared_config/.claude/skills/pr-review/SKILL.md`.

Then read the new Step 3b block and the Step 3c pointer once, confirming no sentence joins two
independent clauses with a dash or a splitting colon, and that the block reads coherently where it
sits.

- [ ] **Step 6: Commit**

**Use `git commit -F <file>` for this one.** The message contains both single and double quotes, and
escaping them inside a `-m "..."` is where implementers waste turns. Write the message to a file
under `/tmp/claude/` first, then commit from it. Do not reach for command substitution.

```
git add shared_config/.claude/skills/pr-review/SKILL.md
```

Message content:

```
pr-review: stop posting AGENTS.md as the source of a finding

A posted comment read 'AGENTS.md "Code comments" says: ...'. The rule comes
from the reviewer's own global AGENTS.md, which is usually not in the repo
under review, so a teammate opened the project's AGENTS.md, found nothing,
and concluded the finding was invented.

The citation stays internal. Role #1 of in-depth-review still cites the file
and the rule text, because the scorer checks whether the cited rule actually
says what the finding claims. review-and-fix keeps it too, since it never
posts to GitHub. Only the posted wording changes.

The rule is now stated as a norm with no source attached, keeping whatever
the rule permits so the author still learns where the boundary is.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
```

---

### Task 2: Stop printing the category in the global body

A second, independent leak. The global template says the description includes the category, and for
these findings the category value IS the string `AGENTS.md`, so it reprints what Task 1 just removed
from the description. This is its own task because dropping the category is broader than the stated
goal, so a reviewer could reasonably accept Task 1 and reject this.

**Files:**
- Modify: `shared_config/.claude/skills/pr-review/SKILL.md` (the global-body template's description line, currently line 805 before Task 1's insert shifts it)

**Interfaces:**
- Consumes: nothing. Independent of Task 1's block.
- Produces: nothing downstream. This is the last task.

- [ ] **Step 1: Record the baselines**

```
rg -c 'including category' shared_config/.claude/skills/pr-review/SKILL.md
```
Expected: `1`.

```
rg -c 'description, including any suggested-fix alternatives' shared_config/.claude/skills/pr-review/SKILL.md
```
Expected: `1`. Only the inline template reads this way today. After this task there will be two,
because the global template will match it.

- [ ] **Step 2: Drop the category from the template**

Locate by text, not line number. Task 1's insert shifted everything below it down.

Find:

```
<description, including category and any suggested-fix alternatives>
```

Replace with:

```
<description, including any suggested-fix alternatives>
```

- [ ] **Step 3: Verify**

```
rg -c 'including category' shared_config/.claude/skills/pr-review/SKILL.md
```
Expected: no output, exit 1. Confirms the category instruction is gone.

```
rg -c 'description, including any suggested-fix alternatives' shared_config/.claude/skills/pr-review/SKILL.md
```
Expected: `2`. The global template and the inline template now read identically, which was the
second half of the point.

```
grep -c '—' shared_config/.claude/skills/pr-review/SKILL.md
```
Expected: `41`, unchanged.

```
grep -c '.\{99,\}' shared_config/.claude/skills/pr-review/SKILL.md
```
Expected: `47`, unchanged.

```
git status --porcelain shared_config/
```
Expected: exactly one line, ` M shared_config/.claude/skills/pr-review/SKILL.md`. Task 1 is already
committed, so its change must not appear here.

- [ ] **Step 4: Commit**

```
git add shared_config/.claude/skills/pr-review/SKILL.md
```

```
git commit -m "pr-review: stop printing the category in the global body

The global template said the description includes the category, and for an
AGENTS.md-category finding that printed the words the previous commit had
just removed from the description. The inline template never printed it, so
the two templates disagreed.

Dropping it makes them match. Informative categories like bug and security
stop printing too, which is accepted. Keeping those would need a special
case for one category value inside a template, and would leave the two
templates inconsistent.

The category field itself is untouched, including the category-priority
tiebreaker. Only the line that PRINTS it changes.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Spec coverage

| Spec item | Task |
|---|---|
| Change 1, the Rule attribution block, exact text | Task 1, Step 2 |
| Change 2, close the category leak | Task 2, Step 2 |
| Change 3, pointer from Step 3c | Task 1, Step 3 |
| Leak 1 closed (description text) | Task 1 |
| Leak 2 closed (category printed) | Task 2 |
| Decision: unattributed-norm register | Task 1 Step 2's literal text |
| Decision: register generalizes to non-style rules | Task 1 Step 2. The text contains no word like "style", so it carries type-safety and error-handling rules too. |
| Decision: keep the rule's permitted list | Task 1 Step 2's block quote retains hyphenated words, CLI flags, ranges, label prefixes and ratios |
| Decision: pr-review only, in-depth-review untouched | Global Constraints, plus Task 1 Step 5's assertion that Role #1 still cites |
| Decision: drop category entirely rather than special-case | Task 2, and its commit message records the tradeoff |
| Decision: rule covers titles as well as descriptions | Task 1 Step 2, the sentence "This governs the title as well as the description" |
| Verification 1-10 | Task 1 Steps 1, 4, 5 and Task 2 Steps 1, 3 |
| Worked example (before/after comment) | No task. It is spec illustration, not file content. |
| Out of scope: in-depth-review, review-and-fix, gh-style-review, the category field, the filters | No task. Global Constraints limit the plan to one file, and Task 1 Step 5 asserts in-depth-review is unchanged. |

## Note on a trap in Task 1

Task 1's find-and-replace text contains a ` ```markdown ` fence, because the anchor deliberately
spans into the global-body template to pin the insertion point. In this plan those blocks are fenced
with four backticks so the inner fence survives. An implementer must copy only the inner content and
must not paste the outer four-backtick lines into `SKILL.md`. A stray four-backtick line anywhere in
`SKILL.md` afterwards means exactly that mistake, and Task 1 Step 5's read-through should catch it.
