# Loop state variables

Track state explicitly:

- `iteration`: starts at 1, increments before each Step 1 launch
- `batch_clean`: per-iteration flag, true iff the deduplicated findings list was empty AND
  `reviewers_missing` was empty AND `reviewer_unavailable` was empty AND the unioned `roles_missing`
  was empty. An empty findings list with a short kind sets `batch_incomplete`, not `batch_clean`. A
  non-empty `reviewer_unavailable` forces `batch_incomplete` too, because an `unavailable` kind is
  never launched and so never shows up in the per-iteration `reviewers_missing`. A non-empty unioned
  `roles_missing` forces `batch_incomplete` the same way, because a lens no instance reported on
  never shows up in `reviewers_missing` either. Without those conditions `batch_clean` would compute
  true on the very iteration row 1c stops as partial.
- `reviewers_missing`, `roles_missing`: per-iteration; carried into the per-iteration summary and
  Final Report
- `reviewer_unavailable`: per-RUN set of reviewer kinds that will not be relaunched (deterministic
  refusal, or a transient shortfall that already used its one retry)
- `reviewer_retries`: per-RUN count per kind, capped at 1. Increment it on any relaunch of a kind
  that fell short in a prior iteration, whichever row triggered that relaunch. A row-4 full rerun
  that happens to relaunch a previously-short kind consumes that kind's budget too. Never reset it
  between iterations. Resetting it turns a per-run budget into a per-iteration one, and nothing
  caps the iteration count, so the relaunches become unbounded.
- `any_commit`, `any_logic_change`, `productive_reviewers`: per-iteration accumulators from
  Step 2, consumed by Step 3's table.
- `iteration_commits`: per-iteration; the `(short sha, finding title)` pairs Step 2 appended, in
  commit order. The per-iteration commit table is this list.
- `run_commits`: per-RUN list of `(iteration, short sha, finding title)`. Append
  `iteration_commits` to it once that iteration's commit table has been emitted. The Final
  Report's Changes Made table is this list. Never reset it between iterations. Empty
  `iteration_commits` as soon as its pairs are appended here. Step 1 sends a clean batch straight
  to Step 3, so Step 2's reset never runs on that iteration, and a stale list would reprint the
  previous iteration's rows under this iteration's header.
- `<ACTIVE_ROLES>`, `<ACTIVE_GH_STYLE>`: the next iteration's active reviewer set
- `discussion_context_snapshot`: per-iteration snapshot of resolved/unaddressed pools (PR
  mode only) — useful for the per-iteration summary
- `resolved_ticket_findings`: ticket findings the user deferred or dismissed (keyed by
  `ticket_id` + title), so later iterations skip them instead of re-prompting.
- `skipped_findings`: per-RUN set of findings of any category that were examined and then
  skipped because no test was possible (Step 2 sub-step 3's `ask_user`), keyed by the finding's
  `file` plus `title`, the same two-real-field shape `resolved_ticket_findings` uses.
  Later iterations skip them instead of re-prompting. `resolved_ticket_findings` does not
  cover these, because it is keyed by `ticket_id` + title and gated to `category == ticket`.
  Without this set a skipped non-ticket finding is re-attempted on every iteration, and while
  other findings keep committing, `any_commit` stays true and row 2 never fires.
