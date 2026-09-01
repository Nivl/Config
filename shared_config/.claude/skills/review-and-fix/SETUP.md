# Setup mechanics

The once-per-run mechanics Step 0 calls out to: resolving the target, probing for a Jira reader,
and resolving the run log's home. Step 0 keeps every act that has to happen at a specific moment.
This file holds the commands those acts run.

## Resolving the target

Run these in order. Return with `<RANGE>`, `<HAS_PR>`, `<PR>`, `<TARGET_ARG>` and `<SKIP_TICKET>`
all set.

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

5. **`<SKIP_TICKET>`** = true if the invocation included `--skip-ticket`, else false. When true,
   every in-depth-review sub-agent is invoked with `--skip-ticket` so role 10 never runs. When
   false, both in-depth-review instances run role 10, giving two ticket reviewers.
   `gh-style-review` is unaffected either way.

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
