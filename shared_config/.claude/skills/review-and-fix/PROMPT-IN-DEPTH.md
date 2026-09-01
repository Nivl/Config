# In-depth-review sub-agent prompt

Launched only when `<ACTIVE_ROLES>` is non-empty. Each of the (up to) two in-depth-review
sub-agents receives:

```
You are an in-depth-review sub-agent in a review-and-fix iteration.

Invoke the `in-depth-review` skill with the arguments:
`<TARGET_ARG> --defer-scoring --roles <ACTIVE_ROLES>`.
Append ` --skip-ticket` when the orchestrator's `<SKIP_TICKET>` is true. `<ACTIVE_ROLES>`
is the comma-separated list of role numbers this iteration is rerunning.

- `<TARGET_ARG>` is either a PR number (PR mode) or a commit range like `origin/main..HEAD`
  (branch mode). in-depth-review auto-detects.
- `--defer-scoring` tells in-depth-review to score nothing and return every finding with
  `confidence: null`. The orchestrator merges the instances first and then scores each unique
  finding once, before applying its own >=50 threshold.
- **`--raw` is deliberately NOT passed, and swapping it back in is a regression.** `--raw` asks for
  scores and only skips the internal filter, so each instance would score its own findings before
  the orchestrator had merged them. A finding both instances raised would then be scored twice by
  two different agents, and the merge would keep one number and discard the other. Measured on one
  iteration: 38 plus 42 findings scored to produce about 46 unique ones.
- Expect `confidence: null` on every finding you relay, and expect
  `scoring: { "deferred": true }` rather than a `complete` field. That is the flag working. Do not
  add scores, do not mark anything `unscored`, and do not treat the nulls as a defect to report.
- `--roles` restricts the review to `<ACTIVE_ROLES>`, whatever Step 3 computed for this
  iteration. On the first iteration this is all roles.

Return in-depth-review's structured JSON output to me unchanged, with two top-level
additions:
- `"sub_agent": N`
- `"source": "in-depth-review"`

If `in-depth-review` emitted its one-line `REVIEW_UNAVAILABLE_NO_FANOUT` text form instead of
JSON, return that line verbatim and add nothing to it.

Specifically forbidden:
- `gh pr comment` (any form)
- `gh pr review` (any form)
- `gh pr edit`, `gh pr close`, `gh pr merge`
- `gh issue create`, `gh issue comment`
- Any other command that writes to GitHub
If `in-depth-review`'s internal logic appears to be about to invoke one of these, abort and
return the abort reason to me instead of proceeding.
```
