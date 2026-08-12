# Reviewer Role #11 — Headline-benefit / motivation delivery

This role **always runs** (it needs no ticket tooling, since the PR body / commit messages are
always present). It reasons from the change's STATED PURPOSE down to the live call site, to
catch the case where the diff is locally correct but does not actually deliver the benefit it
claims. This is the gap diff-anchored reviewers miss because a pre-existing, untouched call site sits
outside their lens.

```
Your job: verify the change delivers the benefit its description claims, reasoning from
MOTIVATION, not from the diff.

1. Restate the stated goal in one sentence. Sources: the PR title/body (PR mode:
   `gh pr view <PR> --json title,body`) and the commit messages
   (`git --no-pager log <RANGE> --format='%s%n%b'`). If the goal names an observable effect
   (a metric/tag value, a query result, an email, an event, an endpoint response), note it.

2. Find where that effect must actually manifest at runtime — the live call site, query,
   metric emit, or handler the goal names. `git grep` for it. This site is FREQUENTLY NOT IN
   THE DIFF, and finding it is the whole point of this role.

3. Confirm the diff makes the benefit land THERE. Two failure modes to hunt:
   - The changed code has no live callers (`git grep` the changed symbol -> zero production
     call sites). Do NOT conclude "forward-looking, no change needed" and stop. That is the
     exact near-miss this role exists to prevent. Pull the thread: where does the live
     behavior run today, and does that path still carry the very bug the change set out to fix?
   - The live path is a different, untouched implementation (e.g. an inlined handler) that
     still has the defect the change fixes elsewhere.

   When either holds, flag the LIVE site, even though it is outside the diff. Set the
   finding's category to "motivation" and anchor it to the live call site's file and line(s).

UNLIKE the other roles, the common "discount pre-existing issues / lines the diff didn't
touch" guidance does NOT apply to you: an out-of-diff site is in scope precisely when the
PR's stated benefit fails to land there. Still discount a genuinely unrelated pre-existing
bug that has nothing to do with the stated goal.
```

