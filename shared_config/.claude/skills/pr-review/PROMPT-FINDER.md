# Sub-agents 1-3 prompt (in-depth-review)

Identical prompt for all three; only the `subagent_type` differs (1 and 2 use
`pr-review-finder-indepth`, 3 uses `pr-review-finder-indepth-deep`). Do not tell the sub-agent
which tier it is on. It should review the same way either way, and the orchestrator attributes
findings by `sub_agent` number.

```
You are sub-agent N of 4 in a pr-review orchestration (N is 1, 2, or 3). Your job:
perform one independent in-depth review of PR #<PR> by invoking the `in-depth-review`
skill, then return its result to me unchanged.

Concretely:

1. Invoke the `in-depth-review` skill with the arguments: `<PR> --raw`. Append
   ` --skip-ticket` when the orchestrator's `<SKIP_TICKET>` is true (so the args become
   `<PR> --raw --skip-ticket`). When `<SKIP_TICKET>` is false, pass `<PR> --raw` unchanged
   so Role #10 runs.
   - The `<PR>` arg puts it in PR mode against this PR.
   - The `--raw` flag tells in-depth-review to skip its internal <70 confidence filter so we
     get every scored finding. The orchestrator will apply its own >=60 threshold.
2. Wait for `in-depth-review` to finish and return its structured JSON output.
3. Return that JSON verbatim, with two additions at the top level:
   - `"sub_agent": N` (which of the 4 instances you are)
   - `"source": "in-depth-review"` (so the orchestrator can attribute findings)

Forbidden:
- `gh pr comment` (any form)
- `gh pr review` (any form)
- `gh pr edit`, `gh pr close`, `gh pr merge`
- `gh issue create`, `gh issue comment`
- Any other command that writes to GitHub

`in-depth-review` itself is read-only with respect to GitHub. If something inside it appears
about to issue a write, abort and surface the reason to me instead of proceeding.

If `in-depth-review` refuses to proceed (closed/merged PR, or other ineligibility), return its
`skipped_reason` field unchanged so the orchestrator can report it. A no-fanout abort is not this
case. Return the whole payload including `coverage`, or, if `in-depth-review` emitted its one-line
text form, return that line verbatim.
```
