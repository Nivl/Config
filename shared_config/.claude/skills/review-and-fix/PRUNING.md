# Why the adaptive rerun is sound

The soundness arguments behind Step 3's rows 4 and 5. Every rule these arguments justify is stated
at its act site in SKILL.md. This file is why those rules are safe, for a reader deciding whether
to change them.

## Contents

- The retry union, and the coverage hole without it
- Why pruning is safe without a final full sweep
- Why a `test` commit is safe to prune on
- What a pruned `test` iteration gives up
- What a pruned `prose` iteration gives up, and why that lane is absorbing
- Role 11 is the seam

## The retry union, and the coverage hole without it

Row 5 is why the union is needed. It builds its set from `productive_reviewers`, and a kind that
reported nothing contributed no findings, so it cannot be in that set on its own merit.

SKILL.md states the keying rule and the guarantee it buys. What that rule looks like in practice is
this. Row 5's role-9 union can put role 9 in `<ACTIVE_ROLES>` while the `in-depth-review` kind
reported nothing at all, which happens when gh-style raised the finding whose fix classified `test`.
The kind is still owed its retry there, and adding it back at full multiplicity supersedes the lone
role 9.

Without the union, a kind that fell short while OTHER reviewers had findings that got fixed is
dropped from the next launch, never retried, and never reaches the second shortfall that would
mark it `unavailable`. The run could then stop on row 1 and report `complete` coverage with a
reviewer silently gone. That is the coverage hole, and it is why the union runs on every path back
to Step 1 rather than only on the row that noticed the shortfall.

## Why pruning is safe without a final full sweep

The loop only reaches a pruned rerun (row 5) after an iteration whose fixes left program logic
identical to what the logic reviewers last cleared. The diff-based classifier in sub-step 7 flips
`any_logic_change` and forces a full rerun (row 4) the instant any fix touches logic.

So a logic reviewer is only ever skipped while the logic it already approved is unchanged. By the
time the loop stops on row 1, row 1c or row 2, every logic reviewer that was still available has
validated the final logic.

Two cases break that guarantee, and both are reported rather than hidden. An interrupted run
carries no such guarantee, because an interrupt can land right after a logic-change commit. And a
kind in `reviewer_unavailable` validated nothing, so a run that stops with one reports `partial`
coverage instead.

A fix that sneaks a logic change past a "comment" finding is caught by the diff classifier, not by
re-running everyone.

## Why a `test` commit is safe to prune on

A `prose` commit is safe because it adds nothing executable. A `test` commit does add executable
code, so it earns its place on the non-logic side for a different reason.

Nothing in production imports a test file, so production behavior after the commit is
byte-identical to what the logic reviewers cleared. That is why the classifier's `test` bucket
excludes the one shape where that claim fails, a shared helper under a test path that non-test code
may import. That shape is `logic`.

A test-runner config file is on the never-logic list instead. Nothing in production imports one of
those either, so the byte-identical claim holds for it. What it can break is which tests run
rather than what the app does. The cost that concedes is real. A change that breaks which tests run
no longer triggers a full rerun, so a suite that stopped executing can reach a clean iteration. Row
4 exists to revalidate application logic, and test wiring is not that.

What a `test` commit CAN introduce is a bad test, which is exactly role 9's lens. That is why row 5
unions role 9 in rather than trusting the productive set alone.

## What a pruned `test` iteration gives up

Roles 1, 5 and 12 are not unioned in, so a pruned rerun judges the new test code through role 9
alone.

Sub-step 5's staged-diff scan is the partial backstop. It runs before the commit exists and catches
the mechanical subset. It is NOT a substitute for role 1's full AGENTS.md read or role 12's type
analysis.

This is a deliberate cost trade, since the loop is uncapped and those three roles would be paid on
every test iteration. The next `logic` commit forces a full rerun (row 4), and they see the
accumulated test code then.

## What a pruned `prose` iteration gives up, and why that lane is absorbing

A `prose` commit usually hands roles 1, 5 and 11 their own output, because `productive_reviewers`
is built from the findings that got fixed, and fixing one of their findings often authors prose.

Role 11 belongs in that list and is not interchangeable with the other two. Roles 1 and 5 check
authored prose against rule text. Role 11 checks a claim in a commit message or a PR body against
the runtime site it names, which is the shape of error this lane actually produces.

Row 4 needs a `logic` commit and a prose-only iteration has none, so there is no route from this
lane back to a full rerun. The only exits are row 1 once roles 1, 5 and 11 accept what the run
wrote, row 2 once an iteration commits nothing, and the user, either by direction or by interrupt.
This lane is the case the user-directed stop exists for, and its signal is the one to put in front
of them.

Sub-step 4's claim sweep and sub-step 5's scan are what shorten the lane. They review the authored
prose before it lands rather than an iteration later, and they do not close the lane.

`self_inflicted_count` is the direct signal, because it counts findings whose target line the run
itself wrote. The two booleans corroborate it.

## Role 11 is the seam

"Fixing their findings authors prose" does not hold for role 11. A role 11 finding names two
things, a claim and the runtime site it describes, and either one can be the thing that is wrong.
Correcting the claim is a `prose` fix. Making the site match the claim is a `logic` fix, and it is
frequently the right one.

So an iteration whose findings came from roles 1, 5 and 11 can legitimately commit `logic` and sit
on row 4, and it is not in this lane at all. The measured run where that was misreported is in
`~/.melvin/config/docs/research/review-and-fix-run-log/NOTES.md`.
