# Setup mechanics

The once-per-run mechanics Step 0 calls out to: resolving the target, probing for a Jira reader,
and resolving the run log's home. Step 0 keeps every act that has to happen at a specific moment.
This file holds the commands those acts run.

## Resolving the target

Run these in order. Return with `<RANGE>`, `<HAS_PR>`, `<PR>`, `<TARGET_ARG>`, `<SKIP_TICKET>` and
`<REVIEW_LIMIT>` all set.

1. **Default branch.**

   ```
   git remote show origin | grep 'HEAD branch' | awk '{print $NF}'
   ```

   Fall back to `main` if unavailable.

2. **Commit range.** `<RANGE>` = `origin/<default-branch>..HEAD`.

3. **Open PR for the current branch**, which is what unlocks gh-style-review's Discussion
   Context.

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
