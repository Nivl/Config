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
  true on the very iteration row 1c stops as partial. An instance that came back `impossible` aborts
  the run before `batch_clean` is computed.
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
- `any_test_change`: per-iteration; true iff at least one commit this iteration classified as
  `test` (Step 2 sub-step 7). Independent of `any_logic_change`, and both are true in the same
  iteration when one commit was `test` and another was `logic`. Row 4 is evaluated first, so
  `any_logic_change` wins that case and the whole set reruns anyway. Row 5 is the only reader,
  and it unions role 9 into the pruned set. It never sets `<ACTIVE_GH_STYLE>`.
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
- `run_log_path`: per-RUN. The absolute path Step 0 resolved for the run log, under
  `~/.melvin/config/logs/review-and-fix/` when that `mkdir` succeeded and
  `/tmp/claude-skills-logs/review-and-fix/` when it did not. Set once and never changed. Step 3
  appends each per-iteration block to it and Step 4 appends the Final Report. The Final Report
  names it, so a reader can tell which of the two homes a run used rather than guessing whether it
  fell back.
- `t0`, `arrivals`, `t2`: per-iteration UTC timestamps from `date -u +%FT%TZ`, each appended to the
  run log on its own line the moment it is taken. `t0` goes before the Step 1 launch, one arrival
  stamp goes beside each instance's result as it is read, and `t2` goes when the Step 2 fix phase
  ends. `t1` is derived rather than stamped. It is the last arrival. `t1` minus `t0` is waiting time
  and `t2` minus `t1` is fixing time. Nothing reads them but the summary block and the Final Report.
  A block whose stamps fail `t0 < t1 < t2` is refused rather than emitted.
  Arrivals are per instance because there is no observable moment at which collection ends.
  Results land asynchronously on later turns and the give-up bound is noticed minutes after the
  last one, so a single collection-end stamp is a guess. Measured: across the first two real logs a
  stamped `t1` was usable in one iteration out of four, while every `t2` was exact.
- `self_inflicted_count`: per-iteration count of findings whose target line an earlier commit of
  this same run wrote, from the `git blame` check in Step 2 sub-step 1. No stop rule reads it and it
  never changes how a finding is handled. It distinguishes a run making progress from a run
  correcting its own output, which the finding count alone cannot do, because that count falls
  either way.
- `resolved_ticket_findings`: ticket findings the user deferred or dismissed (keyed by
  `ticket_id` + title), so later iterations skip them instead of re-prompting.
- `skipped_findings`: per-RUN set of findings of any category that were examined and then
  skipped because no test was possible (Step 2 sub-step 3's `ask_user`), keyed by the finding's
  `file` plus `title`, the same two-real-field shape `resolved_ticket_findings` uses.
  Later iterations skip them instead of re-prompting. `resolved_ticket_findings` does not
  cover these, because it is keyed by `ticket_id` + title and gated to `category == ticket`.
  Without this set a skipped non-ticket finding is re-attempted on every iteration, and while
  other findings keep committing, `any_commit` stays true and row 2 never fires.
