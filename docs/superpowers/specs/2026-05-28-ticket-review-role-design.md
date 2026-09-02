# Ticket-aware review role — design

**Date:** 2026-05-28
**Status:** Approved design, pending spec review

## Goal

Add a reviewer that checks whether the code under review actually implements the
work its tickets describe. Today the review skills look at the diff, history, prior
comments, security, tests, and so on — but nothing reads the linked ticket and asks
"does this change do what the ticket asked for?" This adds that perspective.

## Scope

This is one new reviewer role plus the plumbing to skip it. It lives inside the
existing `in-depth-review` skill as a 10th role and flows through the orchestrators
(`pr-review`, `review-and-fix`) like every other role. `gh-style-review` is not
touched.

## Where it lives

The ticket check is **Role #10 in `in-depth-review`**: "Ticket intent compliance."

It is an ordinary role. It runs in every `in-depth-review` instance, exactly like
roles 1 through 9. The reviewer count therefore follows the existing fan-out:

| Skill | in-depth-review instances | ticket reviewers |
| --- | --- | --- |
| `in-depth-review` (standalone) | 1 | 1 |
| `review-and-fix` | 3 | 3 |
| `pr-review` | 5 | 5 |

Because it runs in every instance, ticket findings triangulate like any other role —
they can reach `agreement: 5/5`. There is no special "run on only one instance"
gating.

## What the role does

1. **Collect ticket IDs** from the change under review:
   - Commit messages in scope.
     - Branch mode: `git log --no-pager <RANGE> --format='%s%n%b'`
     - PR mode: `gh pr view <PR> --json commits`
   - PR title and body (PR mode): `gh pr view <PR> --json title,body`
   - Extract Jira-style IDs with the regex `[A-Z][A-Z0-9]+-[0-9]+`, then dedup.
2. **If no ticket IDs are found, return `NO_ISSUES_FOUND` immediately.** Make no Jira
   calls and trigger no permission prompts. The role is silent when it has nothing
   to check.
3. **Read each ticket, with a tooling fallback:** prefer `acli jira workitem view <ID>`
   (read-only, already in the allowlist). If acli is not installed or not authenticated,
   fall back to a Jira/Atlassian MCP (discovered via ToolSearch, e.g. `atlassian jira`).
   Pull the title, description, and acceptance criteria.
   - **If neither acli nor a Jira MCP is available and authenticated**, the role cannot run.
     It returns `TICKET_REVIEW_UNAVAILABLE: <reason>` — a distinct signal from
     `NO_ISSUES_FOUND` (no tickets) and `TICKET_REVIEW_SKIPPED` (user denied). in-depth-review
     surfaces this to the main agent and the user; the check did not happen.
   - **If one specific ticket is unreadable** while the tooling works (bad ID, no access),
     mark just that ticket `unread` and continue with the rest.
4. **Follow Datadog references:** when a ticket mentions Datadog — trace IDs, trace
   or dashboard URLs, log queries — investigate through the Datadog MCP. Load the
   relevant Datadog skill first, per that MCP's own instructions. Keep this bounded
   to what the ticket explicitly references. Do not go fishing.
5. **Compare intent against implementation.** Flag places where the diff does not
   implement, only partially implements, or contradicts the ticket's stated
   requirements or acceptance criteria.

## Findings, scoring, and reports

- Ticket findings use the existing finding shape with two changes:
  - a new `category: "ticket"`
  - a new optional `ticket_id` field naming the ticket the gap traces to (`null`
    for the other nine roles)
- They flow through `in-depth-review`'s normal per-finding scoring (0–100) and the
  same confidence threshold as everything else. The existing rubric adapts cleanly:
  - `100` — the ticket explicitly requires X and the diff demonstrably does not-X
  - `75` — the code clearly does not do what the ticket explicitly requires
  - `25` — could not confirm the ticket actually requires this (guards against
    misreading the ticket)
- Reports gain a one-line **"Tickets examined"** coverage note, for example
  `TICKET-001 ✅ · TICKET-002 ⚠️ 2 gaps`. This shows the check ran even when no
  finding crosses the threshold.

## The skip flag

- **`in-depth-review`** gains `--skip-ticket`. The role is ON by default. The flag
  turns it off. It composes with the existing `--raw` flag.
- **`pr-review`** accepts `--skip-ticket`. When set, it passes `--skip-ticket` to
  all five in-depth-review instances. When not set, all five run the role.
- **`review-and-fix`** accepts `--skip-ticket` with the same all-or-nothing
  behavior across its three instances.
- **`gh-style-review`** is unchanged. It has no ticket role.

## "Ignore this reviewer" — abort behavior

Claude Code's permission prompt has fixed buttons. A skill cannot add a literal
"ignore this reviewer" option. The behavior is achieved through sub-agent isolation
instead.

The role's prompt ends with: *"You will run `acli jira workitem view` and possibly
Datadog MCP tools, which may prompt for permission. If any such permission is
denied, immediately stop and return exactly `TICKET_REVIEW_SKIPPED: access denied`.
Do not retry or work around it."*

Each ticket reviewer is its own sub-agent, so:

- **Allow path:** the first `acli` or Datadog prompt — pick "Yes, allow all" once and
  all reviewers proceed. One click.
- **Ignore path:** pressing "No" aborts that one reviewer instantly. With N copies
  you may be prompted up to N times, but each copy aborts on its first denial, so it
  is bounded and quick. This is the accepted trade-off for keeping the role a plain
  role with no extra machinery (no sentinel files, no wrapper commands, no hooks).

Distinct outcomes are propagated, not silently swallowed. in-depth-review carries a
top-level `ticket_review: { status, note }` field:

- `ran` — the role executed (with or without findings, including the no-tickets case).
- `skipped` — `--skip-ticket` was passed.
- `denied` — the user denied a permission (the "ignore this reviewer" path).
- `unavailable` — tickets exist but no Jira tooling is available/authed; the check did
  not run. The chat report shows a prominent ⚠️ line so the user knows to install/auth.

## Tooling preflight (orchestrators)

Before launching any reviewers, `pr-review` and `review-and-fix` run a Jira-tooling
preflight (skipped only when `--skip-ticket` was passed). They confirm that a Jira reader
is available **and authenticated** — either `acli` (installed + authed) or a connected +
authed Jira/Atlassian MCP. If neither is ready, the orchestrator asks the user to choose:

1. install/authenticate acli or the Atlassian MCP, then continue (re-check after);
2. proceed now with `--skip-ticket` (run the other reviewers, skip the ticket check);
3. abort.

The review does not start until this is resolved. This front-loads the failure so the
user is not surprised by a mid-run `TICKET_REVIEW_UNAVAILABLE`.

## Orchestrator integration

- **`pr-review`:** ticket findings merge into the flat pool, get classified INLINE or
  GLOBAL, and post like any other finding, with their `ticket_id` shown. The "Tickets
  examined" note goes in the review body. If any sub-agent still reports `denied` or
  `unavailable` at runtime, the final report says so.
- **`review-and-fix`:** ticket findings are always routed to the existing
  ask-the-user path. They are never auto-fixed or auto-committed. For each one the
  loop presents the gap and the ticket requirement, then asks the user to choose:
  **implement the missing intent, defer (surface only), or dismiss.** The user's
  choice drives what happens.

## Documentation consistency

The "NINE roles" wording in `in-depth-review`'s description, and the role-count
references in `pr-review` and `review-and-fix`, are updated to reflect the optional
10th role.

## Side-effect policy

The role only reads. It uses `acli jira workitem view` (read-only) and read-only
Datadog MCP tools. It writes nothing to Jira, Datadog, or GitHub. This matches the
existing no-writes policy of the review skills.

## Decisions defaulted (open to change during spec review)

- **Threshold:** ticket findings are subject to the same confidence threshold as
  every other finding. A moderate-confidence ticket gap can therefore drop below
  `pr-review`'s 60. The "Tickets examined" coverage note compensates by always
  showing what was checked. Alternative considered: give ticket findings a floor so
  they always surface.
- **Ticket ID format:** assumed Jira-style `ABC-123` (`[A-Z][A-Z0-9]+-[0-9]+`). If
  project keys can be lowercase or numeric-only, the regex widens.

## Out of scope

- Adding the role to `gh-style-review`.
- A literal custom button in the permission prompt (not possible).
- Sentinel/hook machinery for one-click ignore across all copies (declined in favor
  of per-copy deny).
- Writing back to Jira (commenting on tickets, transitioning status, etc.).
