# Output: chat report

### If invoked directly by the user

Render a chat report:

```
# In-Depth Review — <SCOPE_DESCRIPTION>

**Coverage:** complete | partial (when partial, list every missing role number and the lenses not
applied). Never omit this line. Its absence reads as complete coverage.
**Unscored:** N findings had no scorer (omit this line when N is 0)

**Findings (confidence >= 70):** N

1. <title> &nbsp;`[severity, roles N, confidence X]`
   <file>:<line_range>
   <description>
   *Suggested fix:* <fix>

2. ...

**Tickets examined:** <ID> ✅ · <ID> ⚠️ <N> gaps · <ID> ❓ unread
```

The **Coverage** line never reads `impossible`. That value lives only in the sub-agent JSON. When no
reviewer role could be launched at all, Step 1's no-fanout abort replaces this entire report with its
single `REVIEW_UNAVAILABLE_NO_FANOUT` line, so there is no header, no coverage, and no findings list
to render.

Omit the **Tickets examined** line when no ticket IDs were found in the change or when
`--skip-ticket` was passed. When `ticket_review.status` is `unavailable`, replace it with a
prominent warning: `⚠️ **Ticket review NOT performed** — <note>. Install/authenticate acli or
the Atlassian MCP, or re-run with --skip-ticket.` When the status is `denied`, show:
`ℹ️ Ticket review skipped — access denied.`

If zero findings survive, coverage is complete, AND no finding was left `unscored`: report
`✅ No issues found at confidence >= 70.`
If zero findings survive but any role is missing, you MUST NOT report a bare green check. Replace
the ✅ line with `⚠️ No issues found, but coverage was PARTIAL.` The Coverage line above already
names every missing role and the lenses not applied; do not repeat that list here.
If zero findings survive, coverage is complete, but any finding was left `unscored`, you MUST NOT
report a bare green check either. Replace the ✅ line with `⚠️ No issues found, but N finding(s)
were never scored.` The Unscored line above already states the count; do not repeat it here.

If `--raw` was set, the chat report header changes to `**All scored findings (no filter):**`
and the threshold note is dropped.

