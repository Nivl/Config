# review-and-fix: pre-flight fix discipline

Date: 2026-08-16
Target files:
- `shared_config/.claude/skills/review-and-fix/SKILL.md` (475 lines)
- `shared_config/.claude/skills/review-and-fix/STATE.md` (37 lines)
- `shared_config/.claude/skills/review-and-fix/SUMMARY.md` (63 lines)

## Goal

Reduce how often a `review-and-fix` iteration exists only because the previous iteration's
fixes introduced new findings. Every such iteration costs a full three-sub-agent fan-out plus
another fix phase, so a defect that a fix introduces is the most expensive kind of defect this
skill can produce.

The intervention is prompt-only. No new sub-agents. This was an explicit constraint from the
human, chosen over a one-cheap-agent-per-iteration option and a one-agent-per-fix option.

## The four failure shapes

The human confirmed all four occur:

1. Convention violations on the fix's own lines. Comment punctuation, casts, a log next to a
   throw.
2. Wrong-shaped error handling. A reviewer asks for error handling and gets a swallowing
   `catch`.
3. A genuine behavior regression that lint and the existing suite do not catch.
4. The fix itself has no test.

## Why this is not a "review your work harder" instruction

Step 2's only pre-commit gate today is mechanical and aimed at the wrong target. It runs the
linter and the existing test suite. Almost nothing the reviewers flag is detectable that way.
The existing suite covers behavior that already worked, so it cannot see the finding.

The governing constraint is not token budget, it is attention. Step 2 runs once per finding,
so roughly four to six times per iteration, across an uncapped number of iterations, inside a
476-line file. An instruction added there is executed about twenty times in one run.

Under that pressure the surviving instructions in this file share one property. They produce
something a later step cannot proceed without. `any_logic_change` survives because Step 3's
row selection reads it. `iteration_commits` survives because it is SUMMARY.md's commit table.
An instruction that produces only a self-graded sentence is compressed away within a few
iterations, invisibly, and dilutes its neighbours on the way out.

So the axis that decides whether an item is worth adding is artifact versus assurance, not
before-the-edit versus after-the-edit. An item that forces a tool call or leaves a mark in the
commit does not compete with the post-commit scan, because it covers a shape the scan cannot
reach. An item that asks for a sentence does compete, and loses.

Of the four shapes, only shape 1 is fully decidable from the diff after the fact. Shape 2's
correct form usually needs a wider edit, since propagation touches signatures and callers, and
a wider edit is far cheaper before the commit than after. Shape 4's load-bearing part is
ordering, because a test written after the fix often also passes against the unfixed code, and
ordering is unobservable post-hoc without an artifact. Shape 3 is not greppable at all.

## Decisions taken

| Decision | Choice | Why |
|---|---|---|
| Add pre-implementation guidance at all | Yes, three items | The pre-edit moment is the only one where a non-self-graded check is available without spending a sub-agent. A test that encodes the finding is executable and aimed at the change. Everything else in a prompt-only design is the fixer vouching for itself. |
| "State the minimal change in one line" | Rejected | Two of its three fields are copied out of the finding and the third is self-graded with no unit for "larger than the finding". No step consumes it. Its one operative clause also points the wrong way. Correct error handling and a new test are both enlargements of the edit. |
| Any instruction to prefer the smaller edit | Rejected | Two independent critiques landed here. A swallowing `catch` IS the minimal edit that satisfies "add error handling", and a new test is by definition larger than the finding. |
| "Check whether an earlier iteration's commit touched these lines" | Rejected | Unimplementable against the state the skill keeps. `run_commits` holds `(iteration, short sha, finding title)` with no path or line data. Its answer is near-always yes from iteration 2 onward, so it discriminates nothing. It also targets a fifth shape the human did not report, and SKILL.md:345 forbids an oscillation detector, which `docs/superpowers/plans/2026-07-31-remove-iteration-cap.md` records as explicitly declined once already. |
| "Decide whether the reviewer's suggested fix is the right fix" | Rejected in general form, kept as one narrow conditional | Both reviewer payloads do carry a populated `suggested_fix`, so the rule is writable. Writable is not binding. "The suggestion is fine" is the default self-assessment, so the general form taxes only deviation. The narrow form (a request for error handling does not authorize a swallow) binds because its artifact is in the diff. |
| Placement of the swallow-catch item | A bullet inside the Implement sub-step, directly after the `AGENTS.md` bullet | It is a coding-standards item and belongs with the conventions read, not after the "do not commit" bullet that reads as the closing gate. The synthesis said both "fourth bullet" and "directly after the AGENTS.md bullet"; those conflict and the more specific instruction wins. A new numbered pre-step is also the shape most likely to be compressed away. |
| Placement of the red-test item | Its own numbered sub-step, before Implement | Ordering is the load-bearing half of this item, and only a numbered step before the edit expresses ordering. |
| Placement of the staged-diff scan | Its own numbered sub-step, gating the commit | The gate is the entire point. A bullet inside Implement would be skimmed. |
| Where the swallow-catch check lives post-commit | Reduced to an artifact-presence check | Post-commit it can only report that the fix is already wrong-shaped. The judgment moves before the edit; the residue is "does each added `catch` rethrow or carry a why-comment". |
| Where the test check lives post-commit | Reduced to `Red:` line presence | Presence of a test is the weak half. Post-commit inspection cannot tell a test that encodes the finding from one that also passes against the unfixed code. |
| "State the behavior delta in one line" post-commit | Dropped | An assurance with no artifact, written by the fixer about work it just chose to do. It duplicates the checkable half of the reference-search item, so keeping both pays twice for shape 3 and gets the checkable half once. |
| Contents of the mechanical scan | The closed decidable subset of AGENTS.md only | Bare `as X`, the non-null `!`, a clause-splitting `:`, comment length, log-level correctness, catch-and-continue, log-plus-throw, and metric consumers all need judgment. Log-plus-throw already has two owners. |
| ` - ` as a clause joiner is in scope while a clause-splitting `:` is not, even though AGENTS.md states them as one rule | Split on signal to noise, not on mechanism | Both need the same judgment, so the earlier claim that every check in the scan is judgement-free was wrong and has been removed from the scan's intro. The split still holds for a different reason. Space-hyphen-space almost never appears in prose that is not joining clauses, because the allowed uses (hyphenated words, CLI flags, ranges, ratios) carry no surrounding spaces, so nearly every hit is a real violation. A colon appears constantly in legitimate prose, so scanning for it would be nearly all false positives and would train the reader to ignore the scan. Do not "restore consistency" by adding `:` back or by dropping ` - `. |
| Scope of the mechanical scan | Authored prose, added lines only | AGENTS.md's literal-content carve-out is explicit. A `→` inside a string literal, a fixture, or quoted output is not a violation. Without this the scan generates false findings on test data. |
| `skipped_findings` state entry | Required, not optional | The red-test item's `ask_user` escape opens an infinite re-attempt loop without it. See "The gap this closes" below. |
| Per-iteration finding count | Added to the summary | Nothing here is verifiable at authoring time. The count makes the trend visible so the change can be judged by running it. It is a summary line, not a stop rule, so SKILL.md:345 is untouched. |
| Commit width versus scope check | The scope check exempts call sites the reference search named | The reference-search item mandates that matching call-site changes go in the same commit, and an unexempted scope check would flag exactly those files on every propagation fix. The exemption narrows the check to files neither the finding nor the search named. |

## The gap this closes

`resolved_ticket_findings` is keyed by `ticket_id` plus title and gated to
`category == ticket`. A finding of any other category that is examined and then skipped has
nowhere to live, so it re-surfaces on every iteration.

The red-test item's `ask_user` escape for infrastructure-blocked findings walks straight into
this. The skip is not recorded, the finding is re-attempted every iteration, and meanwhile
other findings keep committing so `any_commit` stays true and row 2 never fires. The run does
not converge and the reason is invisible.

`skipped_findings` is a state variable, not a stop rule. SKILL.md:345 bans a cap, a periodic
check-in, and an oscillation detector. It does not ban state.

## Edits

### Edit 1: SKILL.md, Step 2 lead-in (currently line 242)

The skip list widens to cover the new set.

Replace:

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

### Edit 2: SKILL.md, new sub-step 3 under "For each finding" (insert after current item 2, line 263)

This edit and Edit 4 both insert a numbered sub-step, so state the final structure once and
use it everywhere. After both edits the list reads: 1 Read, 2 Assess confidence, 3 Red test,
4 Implement, 5 Scan the staged diff, 6 Commit, 7 Record, 8 Move to the next finding. Original
items 3, 4, 5, and 6 therefore become 4, 6, 7, and 8. Every sub-step reference in this spec
uses that final numbering. No file references Step 2's sub-item numbers, so the renumber is
safe. See Verification.

Insert:

```
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

Paragraph 2 is not optional. Without the carve-out this item manufactures the ceremony tests
that the test-coverage reviewer then flags, which lengthens the loop instead of shortening it.

### Edit 3: SKILL.md, two bullets inside "Implement the fix" (now sub-step 4, currently line 265)

Insert both directly after the `AGENTS.md` bullet, before the linter bullet:

```
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

### Edit 4: SKILL.md, new sub-step 5, the staged-diff scan (insert before the commit step)

Insert:

```
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

### Edit 5: SKILL.md, the commit template (now sub-step 6)

The template gains the `Red:` line so sub-step 3's artifact has a defined home.

Replace the fenced block:

```
   git add -A
   git commit -m "<type>: <short description of what was fixed>

   <optional body explaining why>
```

with:

```
   git add -A
   git commit -m "<type>: <short description of what was fixed>

   <optional body explaining why>
   Red: <first line of the failing assertion, behavior findings only>
```

### Edit 6: SKILL.md, Step 3's committed-nothing bullet (currently line 318)

The parenthetical widens so an all-skipped iteration does not look like a malfunction.

Replace:

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

### Edit 7: STATE.md, new variable (insert after `resolved_ticket_findings`)

```
- `skipped_findings`: per-RUN set of findings of any category that were examined and then
  skipped because no test was possible (Step 2 sub-step 3's `ask_user`), keyed by finding id.
  Later iterations skip them instead of re-prompting. `resolved_ticket_findings` does not
  cover these, because it is keyed by `ticket_id` + title and gated to `category == ticket`.
  Without this set a skipped non-ticket finding is re-attempted on every iteration, and while
  other findings keep committing, `any_commit` stays true and row 2 never fires.
```

### Edit 8: SUMMARY.md, the non-commit outcomes bullet (currently line 21)

Add the fifth outcome.

Replace:

```
- Every finding kept by the `>=50` filter that did NOT become a commit, and what happened to
  it, meaning deferred, dismissed, abandoned because lint or tests failed, or moot because an
  earlier commit this iteration already fixed it. Committed findings are the table below
  instead.
```

with:

```
- Every finding kept by the `>=50` filter that did NOT become a commit, and what happened to
  it, meaning deferred, dismissed, abandoned because lint or tests failed, skipped because no
  test was possible, or moot because an earlier commit this iteration already fixed it.
  Committed findings are the table below instead.
```

### Edit 9: SUMMARY.md, new finding-count line (insert after the reviewers-reported bullet)

```
- The count of findings kept by the `>=50` filter, so the trend across iterations is visible
  in one place. A run whose count is not falling is the signal to interrupt.
```

## Verification

There is no test framework. These files are prose, and the correctness test for any sentence
is whether two orchestrators reading it would behave the same way. Each edit's check is an
`rg` run before the edit confirming the quoted text still matches, and an `rg` run after
confirming the replacement landed. If a before-check does not match, stop and report drift
rather than guessing at a merge.

- **Always use `rg -U`.** This file has had four separate defects escape line-oriented greps,
  because its prose is hand-wrapped and references straddle line breaks.
- **Write every space in a multi-word pattern as `\s+`.** These are zero-hit checks, so a
  pattern with a literal space cannot fail correctly and passes vacuously.
- **The renumber is safe.** Every cross-reference to Step 2 in these files points at the step
  as a whole, never at a numbered sub-item. Confirmed across SKILL.md, STATE.md, SUMMARY.md,
  and AGGREGATING.md. Re-confirm before editing.
- **Edit 4 puts the banned glyphs into the skill file on purpose.** They are the enumerated
  targets of a scan. A future scan of these files must treat that bullet as literal content,
  which AGENTS.md's carve-out already covers. Do not "fix" them.

## Size cost

Roughly 56 added lines across three files, of which about 46 sit inside the Step 2 loop body
that runs four to six times per iteration.

**This exceeds the estimate the design was approved against.** The synthesis put the three
pre-flight items at 12 to 14 lines and called that near the ceiling. The actual wording needs
about 30 lines for those three items alone, because the red-test item's carve-out paragraph
and its `ask_user` escape are both load-bearing and neither compresses. The scan adds another
15. So the change is roughly three times the size the "near the ceiling" warning was scaled to.

**Decision: ship all three anyway.** The overrun was put to the human against the alternative
of cutting the reference-search bullet in Edit 3, which is 11 lines, ranked last on leverage,
enforced nowhere, and already marked cuttable. Ship-all-three was chosen. The reading taken is
that the 12-to-14-line ceiling was guesswork produced without writing the wording out, so the
wording is the real cost and the ceiling was never a measured limit.

**The standing first cut is the `ask_user` skip escape in Edit 2, not the reference-search bullet.**
An earlier revision nominated the reference search. The final whole-plan review argued the other
way and is right. The reference search is the only item in this change that touches shape 3 at all,
so cutting it surrenders the hardest failure shape entirely. The skip escape costs 4 loop-body
lines plus roughly 13 more lines of cross-file coherence surface, namely the `skipped_findings`
state entry, the Step 2 lead-in widening, the SUMMARY.md outcome, and the FINAL-REPORT.md label,
all for the rarest branch in the step. Replace it with a commit-body trailer (`Red: none,
<reason>`) and that whole chain collapses.

What that cut costs: no way to park an untestable finding, so it is re-surfaced and re-fixed
without a test on every iteration, and the reason moves from an interactive decision to a
greppable trailer.

If a later run shows the per-iteration finding count from Edit 9 is not falling, cut the skip
escape first and re-measure. A fourth pre-flight item proposed later should be treated as a
proposal to remove one of these, not as an addition.

## Honest limits

- **The red test is itself new, unreviewed code, written by the same fixer.** This cuts against
  the claim that it is the one non-self-graded check available. A test with a wrong expected value
  freezes a wrong contract while looking like evidence, and it is the fixer who chose that value.
  The `Red:` line proves a test failed before the fix. It does not prove the test asserted the
  right thing. This is a real limit on the change's central item and it was missed until the final
  whole-plan review.
- **Shape 3 stays largely unaddressed.** It is the shape the human most wants fixed and the
  one prompt-only means reach least. Semantics are not greppable, the existing suite covers
  only behavior that already worked, and the reference search can surface a call site without
  telling you that a return value flipped from `null` to `undefined` in a way that call site
  relied on. The premise that a regression is always a caller nobody thought about is itself a
  mis-model, because plenty of regressions live in a caller that was read and whose contract
  was misread. The existing path, meaning commit, flip `any_logic_change`, full rerun, remains
  the real detector.
- **Nothing here reduces the cost of a rerun, only the number of them.** Every logic fix still
  trips row 4 and a full three-sub-agent fan-out. If a run takes six iterations today, a
  plausible good outcome is four. Not two.
- **Erosion is only detectable for the red-test item.** `git log --grep='Red:'` makes a
  skipped red run greppable. The swallow-catch artifact is visible in the diff. The reference
  search's count line is enforced nowhere, so under a long run it is the first to degrade,
  which is why it is ranked last and marked cuttable.
- **Shape 1 will not reach zero.** The scan covers the closed decidable set. The two most
  commonly violated comment rules, "explain why, not what" and "don't restate the line below",
  are the two least mechanizable rules in AGENTS.md and will keep arriving as findings.
- **None of this is verifiable at authoring time.** The claim is fewer findings per fix, and it
  can only be tested by running the skill. Edit 9 exists so that test is readable.
