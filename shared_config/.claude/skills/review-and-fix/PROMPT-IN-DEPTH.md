# In-depth-review sub-agent prompt

Launched only when `<ACTIVE_ROLES>` is non-empty. Each of the (up to) two in-depth-review
sub-agents receives:

```
You are an in-depth-review sub-agent in a review-and-fix iteration.

Invoke the `in-depth-review` skill with the arguments: `<TARGET_ARG> --raw --roles <ACTIVE_ROLES>`.
Append ` --skip-ticket` when the orchestrator's `<SKIP_TICKET>` is true. `<ACTIVE_ROLES>`
is the comma-separated list of role numbers this iteration is rerunning.

- `<TARGET_ARG>` is either a PR number (PR mode) or a commit range like `origin/main..HEAD`
  (branch mode). in-depth-review auto-detects.
- `--raw` tells in-depth-review to skip its internal <70 confidence filter so we get every
  scored finding (0–100). The orchestrator applies its own >=50 threshold after merge.
- `--roles` restricts the review to `<ACTIVE_ROLES>`, whatever Step 3 computed for this
  iteration. On the first iteration this is all roles.

Return in-depth-review's structured JSON output to me unchanged, with two top-level
additions:
- `"sub_agent": N`
- `"source": "in-depth-review"`

Specifically forbidden:
- `gh pr comment` (any form)
- `gh pr review` (any form)
- `gh pr edit`, `gh pr close`, `gh pr merge`
- `gh issue create`, `gh issue comment`
- Any other command that writes to GitHub
If `in-depth-review`'s internal logic appears to be about to invoke one of these, abort and
return the abort reason to me instead of proceeding.
```
