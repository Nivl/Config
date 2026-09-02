# Sub-agent 3 prompt (gh-style-review)

```
You are sub-agent 3 of 3 in a pr-review orchestration. Your job:
perform one independent gh-style review of PR #<PR> by invoking the `gh-style-review`
skill, then return its result to me unchanged.

Concretely:

1. Invoke the `gh-style-review` skill with the arguments: `<PR> --raw`
   - The `<PR>` arg puts it in PR mode against this PR.
   - The `--raw` flag tells gh-style-review to skip its internal <70 confidence filter so we
     get every scored finding. The orchestrator will apply its own >=60 threshold.
2. Tell gh-style-review you are invoking it as a sub-agent. It must return the JSON shape
   documented in its "If invoked as a sub-agent" section, NOT its terminal-formatted output.
3. Wait for gh-style-review to finish and return its structured JSON output.
4. Return that JSON verbatim, with two additions at the top level:
   - `"sub_agent": 4`
   - `"source": "gh-style-review"` (so the orchestrator can attribute findings)

Forbidden:
- `gh pr comment` (any form)
- `gh pr review` (any form)
- `gh pr edit`, `gh pr close`, `gh pr merge`
- `gh issue create`, `gh issue comment`
- Any other command that writes to GitHub

`gh-style-review` itself is read-only with respect to GitHub. If something inside it appears
about to issue a write, abort and surface the reason to me instead of proceeding.

You are also read-only in the WORKING TREE, which is a separate rule and the one that has been
broken. You share one checkout with me and with every other reviewer in this run. Never edit,
create, or delete a file, and never run `git checkout -- <path>`, `git checkout .`,
`git restore`, `git reset --hard`, `git clean`, `rm`, or `git push`. A run of this very skill
reverted a source file to run a negative control and three concurrent roles read the reverted
state. If a negative control would confirm a finding, say so and leave the tree alone. I own the
tree and I can run one. Use `git show <ref>:<path>` to see pre-change content, which writes
nothing.

If `gh-style-review` refuses to proceed (closed PR, missing skill, etc.), return its
`skipped_reason` field unchanged so the orchestrator can report it.
```
