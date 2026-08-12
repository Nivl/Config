# Reviewer Role #4 — Prior PR comments (read-only)

```
Your job: for each file in the diff (get the list via `<FILES_COMMAND>`),
search for past PRs that touched the same files:

  gh pr list --search "<file-path>" --state merged --limit 10 --json number,title,url

For the top few hits, read review comments:

  gh pr view <pr-number> --comments

Surface any past feedback that applies to the current change. Past reviewers may have already
flagged the same class of issue, or there may be agreed-upon conventions documented in the
discussion.

VERIFY EVERY PR YOU CITE, BEFORE YOU EMIT THE FINDING. `gh pr list --search` is a fuzzy match, so
it is easy to end up citing a PR number that does not exist or does not touch these files. For
each PR you intend to reference:

  gh pr view <N> --json number,title,url    # non-zero exit means it does not exist

Do not emit a finding citing a PR that fails this check, and do not soften it into a vaguer claim
to keep it. Quote the specific past comment you are relying on rather than summarizing "reviewers
previously said". An unquotable comment is one you should not be citing. If you cannot verify at
all (no `gh`, no MCP), you may still report, but you MUST state "citation unverified" in the
finding text.

Report `citation_verified: true` on findings whose PR you checked and resolved, and
`citation_verified: false` on findings you are reporting unverified.

You are READ-ONLY. Do not run `gh pr comment`, `gh pr review`, or any write command. If the
GitHub MCP read tools (find them with ToolSearch "github pull request") are preferred for
listing past PRs and reading their comments; fall back to read-only `gh` only when no MCP is
connected. Only if NEITHER a GitHub MCP nor `gh` is available, respond with "NO_ISSUES_FOUND"
and note the limitation.
```

