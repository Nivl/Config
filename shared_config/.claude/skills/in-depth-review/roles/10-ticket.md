# Reviewer Role #10 — Ticket intent compliance

**Skip this role entirely if `--skip-ticket` was passed.** Do not launch it. Otherwise it is
the 10th concurrent reviewer. Unlike the diff-focused roles, this one reads the change's
tickets and checks the code against them.

```
Your job: check whether the change implements the work its ticket(s) describe. You compare
each ticket's stated intent against what the diff actually does.

1. Collect ticket IDs from the change:
   - Commit messages in scope:
     - Branch mode: git --no-pager log <RANGE> --format='%s%n%b'
     - PR mode:     gh pr view <PR> --json commits
   - PR title and body (PR mode): gh pr view <PR> --json title,body
   Extract Jira-style IDs matching the regex [A-Z][A-Z0-9]+-[0-9]+ and deduplicate them.
   Discard obvious non-ticket matches such as encoding or version strings (e.g. UTF-8).

2. If you find NO ticket IDs, respond with exactly "NO_ISSUES_FOUND" and stop. Do NOT call
   acli. Do NOT trigger any permission prompt. The role is silent when there is nothing to
   check.

3. Read each ticket, preferring acli: acli jira workitem view <ID>
   If acli is not installed, errors because it is not authenticated, or the session runs
   Bash inside a sandbox (sandboxed acli fails even when installed and authenticated, because its
   credentials are unreadable there, so don't misread the failure as an auth problem), fall
   back to a Jira/Atlassian MCP. Search the available tools (e.g. ToolSearch
   "atlassian jira") for an issue-read tool and use it. Pull the title, description, and
   acceptance criteria.

   If NEITHER acli (installed + authenticated) NOR a Jira/Atlassian MCP (connected +
   authenticated) is available, you cannot perform this review. Stop and return exactly:
   TICKET_REVIEW_UNAVAILABLE: <one-line reason>
   This is NOT "no issues". It tells the caller the ticket check did not run, so the caller
   can warn the user.

   If one specific ticket cannot be read while the tooling works (bad ID, no access), mark
   just that ticket as unread and continue with the rest.

4. If a ticket references Datadog (a trace ID, a trace or dashboard URL, or a log query),
   investigate it through the Datadog MCP to understand the actual failure and whether the
   diff addresses it. Load the relevant Datadog skill first, per that MCP's own instructions.
   Keep this bounded to what the ticket explicitly references. Do NOT go fishing.

5. Compare intent against implementation. Flag places where the diff does not implement,
   only partially implements, or contradicts the ticket's stated requirements or acceptance
   criteria. For each finding, set category to "ticket" and ticket_id to the relevant ID.
   Anchor the finding to the file and line(s) where the gap shows up (or the file that
   should have changed but did not).

6. Regardless of whether you found gaps, end your response with one line listing every ticket
   you examined and its status:
   TICKETS_EXAMINED: <ID>=ok, <ID>=gaps, <ID>=unread
   (ok = read, no gaps; gaps = you raised at least one finding for it; unread = tooling worked
   but this ticket could not be read.) Omit this line only in the no-ticket-IDs case (step 2).

ABORT ON DENIAL: Running acli, MCP, or Datadog tools may prompt the user for permission. If
any such permission is denied, immediately stop and return exactly:
TICKET_REVIEW_SKIPPED: access denied
Do not retry and do not work around it. A denial is the user's signal to ignore this
reviewer.

DO NOT MASK FAILURE AS SUCCESS: never return NO_ISSUES_FOUND because you could not read the
tickets. NO_ISSUES_FOUND means "tickets read, code matches" or "no ticket IDs to check".
Inability to read tickets at all is TICKET_REVIEW_UNAVAILABLE (see step 3).

You are READ-ONLY everywhere: Jira (view only), Datadog (read only), GitHub (read only).
Never comment on, transition, or otherwise write to a ticket.
```

