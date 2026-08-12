# Scoring rubric

Each scorer returns a number 0–100 with this rubric:

| Score | Meaning                                                                                                                                              |
| ----- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0     | False positive that doesn't survive light scrutiny, or a pre-existing issue                                                                          |
| 25    | Somewhat confident — might be real, might be false; couldn't verify either way. For AGENTS.md issues: the cited rule doesn't actually call this out. |
| 50    | Moderately confident — the mechanism is real, but residual uncertainty remains about whether it truly applies                                        |
| 75    | Highly confident — verified the code definitively does this; OR a provable convention / AGENTS.md violation                                          |
| 100   | Absolutely certain — evidence directly confirms it                                                                                                   |

**Hard cap: a finding whose citation is unverified cannot score above 60.** Roles #3 and #4 verify
their own commit and PR citations at emit time and report `citation_verified` (see their prompts),
so this cap holds even if that verification could not run. It is a backstop, not the primary
enforcement. The roles drop fabricated citations before they ever reach you.

- `citation_verified: true` — score normally against the rubric above.
- `citation_verified: false` — cap at **60** and preserve it as a lead. Never resolve an unverified
  citation upward. The cap is not what keeps such a finding out of a caller's output. Callers
  exclude `citation_verified: false` outright, `pr-review` from posting and `review-and-fix` from
  fixing, whatever the score. Do not describe the cap as sitting below a caller's threshold. It does
  not. `pr-review` filters at `confidence >= 60`, which 60 satisfies.
- A finding that cites nothing has nothing to verify; score it normally.

If a finding cites a commit or PR and carries no `citation_verified` field at all, treat it as
`false` and cap it. An absent field means the role did not verify.

**Calibration — confidence is the TRUTH axis, not current impact.** Confidence answers "how
sure are we this finding is real and valid," NOT "how big is the blast radius today." Two
consequences:
- A finding whose truth is *provable and binary* — a convention or safety violation (e.g. a
  non-`CONCURRENTLY` index build on a pre-existing table, checkable against repo convention) —
  is scored by provability ALONE. Do NOT discount it because the current blast radius is
  small: an empty or feature-gated table, low live traffic, or a cheap fix. That low impact
  belongs in the **severity** field (`minor` / `suggestion`), not in the confidence number.
  Deflating a provably-true finding by today's table size is the calibration error to avoid.
  It buries real, cheap-to-fix findings below the caller's threshold.
- This is NOT a blanket "score every real-ish finding high." For a latent *bug*, confidence
  still reflects whether it is genuinely a defect and whether its path is reachable at all. A
  rare, marginal conjunction that may not even constitute a real defect legitimately sits near
  or below the line. Reachability (can the path EVER execute) is a truth question and bounds
  confidence; frequency (how OFTEN, how big the blast radius) is impact and does not.

When scoring, `role_agreement` and the diff are inputs to *truth*, not impact. More roles
raising the same provable violation supports a higher confidence, never a lower one.

**Ticket-category findings** (from Role #10) are scored on the same 0–100 scale. The question
is "how sure are we the code diverges from what the ticket requires?":

- 100 — the ticket explicitly requires X and the diff demonstrably does not-X
- 75 — the code clearly does not do what the ticket explicitly requires
- 50 — a gap is plausible but the ticket's requirement is ambiguous, or the divergence may be minor
- 25 — could not confirm the ticket actually requires this (likely a misread of the ticket)
- 0 — false positive: the ticket does not require this, or the diff already satisfies it

Score the divergence, not the importance of the ticket.

**Motivation-category findings** (from Role #11) are scored on "how sure are we the PR's
stated benefit fails to land at the live call site?" Do NOT score one of these as 0 merely
because the cited line is outside the diff. The PR's stated purpose puts that site in scope:

- 100 — the stated goal demonstrably does not occur (the live call site still has the bug, or
  the changed code has zero live callers and the real path is untouched)
- 75 — strong evidence the benefit does not land where the goal says it should
- 50 — plausible the benefit is undelivered, but the goal or the live call site is ambiguous
- 25 — could not confirm the live call site; may be a misread of the goal
- 0 — the benefit does land (the diff reaches the real call site), or the "goal" was misread
