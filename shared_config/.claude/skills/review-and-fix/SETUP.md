# Setup mechanics

The once-per-run mechanics Step 0 calls out to: resolving the target, probing for a Jira reader,
and resolving the run log's home. Step 0 keeps every act that has to happen at a specific moment.
This file holds the commands those acts run.

## Resolving the target

Run these in order. Return with `<RANGE>`, `<RANGE_BASE>`, `<FORCE_BRANCH>`, `<HAS_PR>`, `<PR>`, `<TARGET_ARG>`,
`<SKIP_TICKET>`, `<REVIEW_LIMIT>`, `<FULL>` and `<AB_MODEL>` all set.

1. **Default branch.**

   ```
   git remote show origin | grep 'HEAD branch' | awk '{print $NF}'
   ```

   Fall back to `main` if unavailable.

2. **Commit range.** `<RANGE>` = the value of `--range` when the invocation included it, else
   `origin/<default-branch>..HEAD`. Both `--range <base>..HEAD` and `--range=<base>..HEAD` work.
   `<RANGE_BASE>` = the part before `..`, which is `origin/<default-branch>` by default.

   **The value must be `<base>..HEAD`.** Fix commits land on `HEAD`, so a range ending anywhere
   else would hide them from the next iteration. Check the base resolves with
   `git rev-parse --verify <base>^{commit}` and that `git merge-base --is-ancestor <base> HEAD`
   exits zero. A `--range` with no value, a value that is not `<base>..HEAD` with two dots (a
   three-dot `<base>...HEAD` is malformed too), or a base that fails either check is a malformed
   invocation. Say which and stop. When the base is not an ancestor of `HEAD`, such as
   `origin/<default-branch>` after it moved past the branch point, also say to drop `--range` to
   review the whole branch, or to pass the sha `git merge-base <base> HEAD` prints as the base.

   `--range` sets `<FORCE_BRANCH>` below to true. A PR's diff is the whole branch, so a narrower
   range only means anything in branch mode.

2b. **`<FORCE_BRANCH>`** = true if the invocation included `--branch` or `--range`, else false. When true, skip
   item 3 entirely and set `<HAS_PR> = false`, so sub-agents run in branch mode even when the
   branch has an open PR. Callers that already hold the whole PR diff locally, such as `work-on`
   and `take-over`, want the same branch-range review whether or not a PR exists. `--branch` or
   `--range` with `--gh-style` is a malformed invocation, because gh-style's product is the PR
   context that both flags turn off. Say so, naming both flags, and stop.

3. **Open PR for the current branch**, which is what unlocks gh-style-review's Discussion
   Context. Skip this item when `<FORCE_BRANCH>` is true.

   ```
   gh pr view --json number,state,isDraft,url 2>/dev/null
   ```

   - Exit non-zero, or `state != "OPEN"` -> `<HAS_PR> = false`. Sub-agents receive `<RANGE>` and
     run in branch mode.
   - Exit zero and `state == "OPEN"` -> `<HAS_PR> = true`, and save `<PR>`. Draft PRs are
     accepted.

   Prefer the GitHub MCP server when it is connected, and use `gh` only as a fallback when no
   GitHub MCP is available or its tools do not cover the call. Discover its tools with
   `ToolSearch "github pull request"` and use a get-pull-request or list-pull-requests tool. If
   neither a GitHub MCP nor `gh` is available, set `<HAS_PR> = false` and continue. The loop still
   works, and the reviewer sub-skills carry the same fallback for their own reads. Local `git`
   calls need no `gh`.

4. **`<TARGET_ARG>`** = `<PR>` when `<HAS_PR>`, otherwise `<RANGE>`.

5. **`<SKIP_TICKET>`** = true if the invocation included `--skip-ticket`, else false. Role 10 does
   not run in this skill either way, per Step 0 item 6, so the flag gates the Jira preflight and
   the ticket-category decisions in Step 2 and removes nothing from the role set.

5c. **`<AB_MODEL>`** = the value of `--ab-model` when the invocation included it, else empty. Both
   `--ab-model sonnet` and `--ab-model=claude-sonnet-5-5` work, and the value is passed to the
   workflow as given. A `--ab-model` with no value is a malformed invocation, so say so and stop.
   When set, Step 1 runs iteration 1 as an A/B of that model against the role agent's pin.

5b. **`<GH_STYLE>`** = true if the invocation included `--gh-style`, else false. When true,
   `<ACTIVE_GH_STYLE>` is true for iteration 1 and the Discussion Context step runs in PR mode.
   When false, no gh-style pass runs and Step 1.5 is skipped. Off by default because across the
   priced runs it was the sole raiser of one committed fix for $178, and every logged run was in
   branch mode, where its distinct product does not exist.

6. **`<REVIEW_LIMIT>`** = the integer value of `--review-limit` when the invocation included it,
   else 0. Both `--review-limit 3` and `--review-limit=3` set it to 3. A value of 0 or below means
   no cap, which is the unflagged behavior. A value of 1 or more is the maximum number of
   iterations Step 3 will let the loop reach, and that step is the single place the value is read.

   A `--review-limit` with no value after it, or with a value that is not an integer, is a
   malformed invocation. Say which and stop. Do not substitute 0, and do not substitute a number
   of your own. Both of those turn a typo into a silently different run, and the 0 case is the
   worse of the two because it reads as the cap having been honored.

   The flag is also how a caller passes a cap down. `work-on` passes the user's value, or the
   default its `--no-assess` mode supplies, and nothing about this skill's own behavior changes
   based on which of those it was. A caller-supplied cap and a user-typed one are the same value
   here. What the caller owes its own user is disclosure, and that is the caller's report to write.

7. **`<FULL>`** = true if the invocation included `--full`, else false. It takes no value. When
   true, Step 3's two asks do not fire and the loop ends only on a stop row, the `<REVIEW_LIMIT>`
   cap, or an interrupt. Step 3 says what replaces each ask. `work-on` passes it down when its own
   user typed it.

## Probing for a Jira reader

A reader counts only if it is both available AND authenticated. Either of these satisfies it:

- **acli**: installed (`command -v acli`) and able to read Jira. Run one lightweight
  authenticated call. If it fails with an auth or login error, treat acli as unauthenticated. In a
  sandboxed session, skip the acli probe entirely and treat acli as unavailable, because sandboxed
  acli fails even when installed and authenticated. Rely on the MCP check instead.
- **A Jira or Atlassian MCP**: connected and authenticated. Search available tools, for example
  `ToolSearch "atlassian jira"`. If the only exposed tool is an `authenticate` tool, the server is
  connected but not yet authed, which does not count.

## Resolving the run log's home

Run these two in order and read the exit code of the first:

```
mkdir -p ~/.melvin/config/logs/review-and-fix
mkdir -p /tmp/claude-skills-logs/review-and-fix
```

Exit 0 on the first makes that directory the home. A non-zero exit falls back to the second,
which is under `/tmp` and therefore works on any machine. Run the fallback only after a non-zero
exit.

Then set `run_log_path` to `<home>/<YYYYMMDD-HHMMSS>-<repo>-<branch>.md`, taking the stamp from
`date -u +%Y%m%d-%H%M%S`, the repo from the remote's basename, and the branch from
`git rev-parse --abbrev-ref HEAD`. The stamp leads so a directory listing sorts by run.
