# Worked examples

## Worked example

- **Iter 1** (full: all roles + gh-style). Findings from Role #2 (`bug`) and Role #5
  (`comment guidance`). Fixing #2 edits executable code, so that commit is `logic`. Fixing #5
  edits a comment, so that commit is `prose`. `any_logic_change = true` -> **Iter 2 is a full
  rerun**.
- **Iter 2** (full). Only Role #5 fires now. Its fix is comment-only, so the commit is `prose`.
  `any_commit = true`, `any_logic_change = false`, `any_test_change = false`,
  `productive_reviewers = {5}` -> **Iter 3 is pruned** to `--roles 5`, gh-style skipped.
- **Iter 3** (pruned: 2 x in-depth-review `--roles 5`). Clean batch -> **Stop** (row 1). Logic
  reviewers last ran in Iter 2 on logic identical to the final tree, so nothing was missed.

## Worked example, a test-only iteration

Traces the `test` commit class and role 9's union into the pruned set.

- **Iter 1** (full). Two findings survive the filter. Role #9 (`test coverage`) wants an
  uncovered branch tested, and Role #5 (`comment guidance`) wants a ` - ` clause joiner split.
  Fixing #9 adds a case to `parse.test.ts`, and every changed file in that commit is a test
  file, so it classifies `test`. Fixing #5 edits a comment, so it classifies `prose`. Neither
  is `logic`, so `any_logic_change = false`, `any_test_change = true`, and
  `productive_reviewers = {9, 5}`. **Row 5** fires. Iter 2 is pruned to `--roles 5,9`, gh-style
  skipped. Role 9 was already productive here, so the union changes nothing yet.
- **Iter 2** (pruned: 2 x in-depth-review `--roles 5,9`). Clean batch -> **Stop** (row 1). The
  logic reviewers last ran in Iter 1 on production behavior identical to the final tree, since
  nothing imports `parse.test.ts`. Role 1 and role 12 never read the added test case, which is
  the cost trade Step 3's "What a pruned `test` iteration gives up" names.

Two variants on Iter 1, neither continuing the run above:

- **The variant the union exists for.** Say the only finding fixed had been Role #2's (`bug`),
  reporting that `parse.test.ts` asserts the wrong expected value, and the fix stayed inside
  that one test file. The commit still classifies `test`, so `any_logic_change` stays false and
  row 5 still fires. But `productive_reviewers = {2}`, and role 9 is not in it. The union makes
  the set `--roles 2,9` so role 9 judges the rewritten assertion. Without it, new test code
  would ship having been read only by the reviewer that asked for it.
- **The boundary.** Had the Role #9 fix also needed a new `setupFiles` entry in
  `vitest.config.ts`, that commit would classify `logic`, not `test`. A runner config decides
  which tests run at all, so it can change what the suite proves about production code. Row 4
  fires and Iter 2 is a full rerun.

## Worked example, a reviewer that flakes

Traces the retry union, row 1b, and row 1c. All three are absent from the example above.

- **Iter 1** (full). The gh-style instance returns nothing, no `skipped_reason`, so the
  `gh-style-review` kind **fell short**. in-depth reports one `comment guidance` finding, which is
  fixed and committed. `any_commit = true`, `any_logic_change = false`, `any_test_change = false`,
  so row 5 fires and computes `productive_reviewers = {5}` with no role-9 union. Row 5 alone
  would drop gh-style, because a kind that reported nothing produced no committed fix and so
  cannot be in that set. **The retry union adds it back**,
  since it fell short and still has budget. `reviewer_retries[gh-style] = 1`. Iter 2 launches
  in-depth `--roles 5` plus gh-style at full multiplicity.
- **Iter 2.** gh-style reports this time, and everything is clean. The shortfall is **cleared**, so
  it is history in the per-iteration summary and NOT a coverage gap. Row 1 fires. Coverage is
  `complete` and the Outcome is "✅ Clean batch". This is the case that must not latch `partial`,
  because every lens did get applied.
- **Iter 2, alternative.** gh-style falls short a second time. Its budget is gone, so it is marked
  `unavailable` and the subtract step drops it for the rest of the run. With the findings list empty
  and every launched kind either reported or `unavailable`, row 1 fails on the non-empty
  `reviewer_unavailable` and **row 1c** fires. The run stops. Coverage is `partial`, naming
  gh-style, and the Outcome is "⚠️ Stopped with incomplete coverage". No green check, because
  Coverage is not `complete`.
- **Row 1b, for contrast.** Had Iter 1's findings list been empty when gh-style fell short, row 1b
  would have fired instead of row 5, and the active set would have been the short kind ONLY. Same
  retry, reached by a different row. That is the path the union generalizes to every other row.
