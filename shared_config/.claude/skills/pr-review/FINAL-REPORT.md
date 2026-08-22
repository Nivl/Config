# Step 4: Final report (to the user, not GitHub)

Summarise to the user in chat. This report happens whether or not a review was posted.

If `<IS_DRAFT>` was true, prepend a short note: `ℹ️ PR #<PR> is still a draft.` so the user
remembers their PR isn't ready-for-review yet.

If an in-progress comment was posted (Step 0.7), the run deleted it in Step 3e; if that
deletion failed, say so and include the comment URL so the user can remove it manually.

**Review file (Step 3c.7).** Whenever the gate ran, the assembled comments were written to
`/tmp/claude/pr-review-<PR>-comments.md`. Always report this path. If the user kept only a
subset, name which findings they dropped. If the user **declined** (or dropped everything),
lead with a clear line that the review was written to that file but NOT posted to GitHub at
the user's request, then still give the tallies below.

## If at least one finding or unaddressed prior concern was posted (review issued)

- The PR URL.
- How many findings each sub-agent originally returned (pre-merge counts), broken down by
  source and tier: `2 x in-depth-review (sonnet): [N1, N2]`,
  `1 x in-depth-review (opus): [N3]`, `1 x gh-style-review: [N4]`. Call out separately how many
  findings the Opus finder contributed that NO Sonnet finder raised. That number is the direct
  measure of whether the mixed tier is earning its cost on this PR.
- How many unique findings survived the >= 60 filter (post-merge count), broken down as
  GLOBAL vs INLINE, and how many were both-source vs single-source.
- How many unaddressed prior concerns surfaced: `unaddressed: K`.
- **Approach stage:** how many findings the approach reviewer raised, how many the pair
  *converged* on as needing a change, how many of those were dropped as `> 50` duplicates of an
  existing finding, and how many were posted (and the round count). If it surfaced or posted
  nothing, say so.
- Sub-agents that returned `skipped_reason` (if any) and why. Distinguish these from sub-agents
  that returned nothing. A `skipped_reason` is a deliberate refusal the run cannot recover from,
  whereas an empty response may just have flaked. Say which kind each one was.
- **Coverage:** `complete` or `partial`. Partial when `reviewers_missing` is non-empty, when the
  unioned `roles_missing` is non-empty, or when `approach_stage_missing` is set. When partial, name
  every missing reviewer, every missing role, and the approach role if it was the one missing, and
  state which lenses the diff was therefore NOT reviewed against. Never omit this line. Its
  absence reads as complete coverage.
- The PR review's HTML URL (the `html_url` returned by the `gh api .../reviews` response).
- Any inline comments that had to be demoted to GLOBAL because GitHub rejected them
  (line not in diff), with a brief explanation.
- Tickets examined and their outcome: `<id> ✅ | ⚠️ N gaps | ❓ unread`. If any in-depth-review
  sub-agent reported `ticket_review.status` of `denied` (user denied access) or `unavailable`
  (no Jira tooling), say so explicitly.

## If the run aborted for no fan-out

- **Lead with the abort.** Name every instance that came back `coverage: "impossible"` and quote
  the `skipped_reason` it carried. Say plainly that no reviewer read the diff. An instance that
  returned the `REVIEW_UNAVAILABLE_NO_FANOUT` line instead of JSON is the same abort. Name it by
  its sub-agent number and quote the line itself, since a text-form return carries no
  `skipped_reason` field.
- Tell the user to re-run the review from the main thread. A workflow agent is the usual context
  that lacks the `Agent` tool.
- **Emit no Coverage line.** There is no coverage to report. `partial` here would relabel a broken
  harness as a review with holes in it.
- This is NOT the clean-PR section below. Do not emit the all-clear line, and do not report finding
  counts, threshold counts, or an approach-stage outcome as though a review ran.

## If no review was posted (clean PR)

- **Only when coverage is complete AND the approach stage ran**, lead with a clear "all clear"
  line, e.g. ✅ `PR #<PR> looks good — four independent reviewers
(2 x in-depth on Sonnet, 1 x in-depth on Opus, 1 x gh-style) raised no findings at confidence >= 60, the approach pair
converged on nothing, and there are no unaddressed discussion items. Nothing posted to GitHub.`
  That line asserts the pair converged on nothing, which is a claim about a stage that ran and
  agreed. It is false when the stage never ran, so `approach_stage_missing` blocks this line the
  same way a missing finder does.
- **When coverage is partial, do NOT emit that line.** It asserts a clean result from N reviewers,
  which is false if fewer than N reported. Lead instead with what actually happened, e.g.
  ⚠️ `PR #<PR>: no findings from the 3 of 4 reviewers that reported. Sub-agent 3 (in-depth, Opus)
  returned nothing, so the subtle-bug pass did not run. Roles missing across instances: #7
  security. This is NOT an all-clear. The diff has not been reviewed for security. Nothing posted
  to GitHub.` Name every missing reviewer and every missing role, and state plainly which lenses
  the PR therefore has NOT been checked against.
- The PR URL.
- How many findings each sub-agent originally returned (pre-merge counts), broken down by
  source as above.
- How many were filtered out by the >= 60 threshold (so the user can see whether reviewers
  flagged anything below the bar).
- **Approach stage:** how many findings the approach reviewer raised, how many the pair
  converged on as needing a change, and how many were dropped as `> 50` duplicates. If the
  approach stage is the reason there is anything at all, note that nothing else was posted
  only because the pair converged on nothing (or every survivor was a duplicate). **If
  `approach_stage_missing` is set, write `approach stage did not run: <reason>` here instead**,
  naming which role was missing, e.g. `approach stage did not run: judge returned nothing`. Never
  report a missing stage as a converged-on-nothing stage.
- Sub-agents that returned `skipped_reason` (if any) and why. Distinguish these from sub-agents
  that returned nothing. A `skipped_reason` is a deliberate refusal the run cannot recover from,
  whereas an empty response may just have flaked. Say which kind each one was.
- **Coverage:** `complete` or `partial`. Partial when `reviewers_missing` is non-empty, when the
  unioned `roles_missing` is non-empty, or when `approach_stage_missing` is set. When partial, name
  every missing reviewer, every missing role, and the approach role if it was the one missing, and
  state which lenses the diff was therefore NOT reviewed against. Never omit this line. Its
  absence reads as complete coverage.
- Tickets examined and their outcome: `<id> ✅ | ⚠️ N gaps | ❓ unread`. If any in-depth-review
  sub-agent reported `ticket_review.status` of `denied` (user denied access) or `unavailable`
  (no Jira tooling), say so explicitly.
