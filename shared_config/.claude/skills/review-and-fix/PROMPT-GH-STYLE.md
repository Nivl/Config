# gh-style-review sub-agent prompt

Launched only when `<ACTIVE_GH_STYLE>` is true. gh-style-review has no roles, so it is
rerun as a whole unit (or skipped entirely). The single sub-agent receives:

```
You are a gh-style-review sub-agent in a review-and-fix iteration.

Invoke the `gh-style-review` skill with the arguments: `<TARGET_ARG> --raw`

- `<TARGET_ARG>` is either a PR number (PR mode — unlocks Discussion Context) or a commit
  range (branch mode — Discussion Context arrays will be empty). gh-style-review auto-detects.
- `--raw` tells gh-style-review to skip its internal <70 confidence filter so we get every
  scored finding. The orchestrator applies its own >=50 threshold after merge.

Tell gh-style-review you are invoking it as a sub-agent. It must return the JSON shape
documented in its "If invoked as a sub-agent" section, NOT its terminal-formatted output.

Return that JSON to me unchanged, with two top-level additions:
- `"sub_agent": N`
- `"source": "gh-style-review"`

Specifically forbidden:
- `gh pr comment` (any form)
- `gh pr review` (any form)
- `gh pr edit`, `gh pr close`, `gh pr merge`
- `gh issue create`, `gh issue comment`
- Any other command that writes to GitHub
If `gh-style-review`'s internal logic appears to be about to invoke one of these, abort and
return the abort reason to me instead of proceeding.
```
