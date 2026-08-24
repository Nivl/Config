# Ticket templates

Step 8 of [SKILL.md](SKILL.md). Pick the template from the issue type and the intent, then fill it. Never a table anywhere inside one. A triage assessment is the natural table and a table is the most common way a ticket arrives mangled, so every row is a bullet.

## Shared rules

All five templates follow these rules.

Headings start at `##`, because an `h1` renders enormous next to Jira's own page title.

Write one line per paragraph and one line per list item, however long it runs, because a hard wrap can survive conversion to ADF and arrive as ragged text.

A number nobody ran becomes a `TODO(user):` line carrying the query, never prose. Prose reads as a measured claim, and a later reader treats a number nobody ran as evidence.

## Feature Story

Verbatim from the request.

```
## Overview
[What and why in 2-3 sentences, if possible in a "As a user..." type of sentence]

## ELI5

[explain the ticket in a way so someone with no knowledge of the project understands what needs to be done]

## Acceptance Criteria
- [ ] Given [context], when [action], then [outcome]
- [ ] [Additional criteria]

## Technical Notes
- [Architecture decisions]
- [Edge cases to handle]
- [Performance considerations]

## Related Code
- [Similar implementations]
- [Relevant utilities]
```

## Bug Fix and Security

From the request, plus the rubric under the template, which exists so Step 4 has one scale to pick from. The `## Stats` block is load-bearing. It is what makes a bug ticket triageable without a follow-up question. Its six questions are the request's own and so is the `Odds of triggering` rating, but the request asked for all of it in one paragraph and this block answers it one bullet each. Do not put that paragraph back. A stakeholder reads this block to decide whether the bug gets picked up, and a paragraph makes them hunt for each answer.

```
## Problem
[Description of the bug and impact, if possible in a "As a user..." type of sentence]

## Stats
- **How many people are impacted**: [number]
- **Are impacted users in a locked state**: [yes/no]
- **Odds of triggering**: [1-5]/5 - [why, landed on a path with a line number or on the query that measured it]
- **Is a backfill required to unblock users**: [yes/no]
- **Is this in a legacy route**: [yes/no]
- **Is this in a live codepath**: [yes/no]

## Reproduction Steps
1. [Step 1]
2. [Step 2]
3. [Observe incorrect behavior]

## Expected Behavior
[What should happen instead]

## ELI5

[explain the ticket in a way so someone with no knowledge of the project understands what needs to be done]

## Root Cause
[What's causing the issue - file and line reference]

## Proposed Fix

[explain how the issue could be fixed]

## Testing
- [ ] Add unit test for [scenario]
- [ ] Add regression test
- [ ] Verify in [affected areas]

## Related Code
- [Where similar issues might exist]
```

**Every Stats answer is short.** A number, a yes or a no, or the rating with its rationale. Nothing in that block is a paragraph and no bullet there argues a case. The rating's rationale is the one exception, because a bare digit is not checkable. The impact count is a number too, so one nobody measured is a `TODO(user):` line and never a guess sitting in the `[number]` slot.

### The odds rating

The request's own wording sets the ends of the scale at 1 and 5. These five rungs fill it in, and Step 4 picks from this scale and no other.

- **1.** Needs a combination of conditions no known user hits.
- **2.** Needs a non-default config or an unusual input.
- **3.** On a normal path, but behind a condition only some users meet.
- **4.** Fires on the default path for anyone who takes it.
- **5.** Measured, and actively firing in production now.

Rungs 1 through 4 measure preconditions, so a code read alone places a defect anywhere in that range and the probes only sharpen the number. Rung 5 is the deliberate exception. It needs a measurement, so a read of the guards tops out at 4 however severe the defect looks, and only a probe moves it past that. A 4 argued from the guards and a 4 Datadog measured are both legal, and the rationale is what tells them apart.

Population share never enters the scale. An Amplitude count of how many users reach the path belongs in the Stats block's own impact question and cannot raise a rating by itself.

A defect that cannot fire at HEAD has no rung here. A dead path goes to the Step 9 gate as a question rather than onto the scale, because filing it as a 1 reads as work worth doing.

**A bare digit is not a rating.** The rationale carries either a path with a line number or the query that produced the number. `4/5` gives a triager nothing they can check. `4/5 - default branch in checkout submit, no flag guard (src/checkout/submit.ts:142)` tells them where to look and lets them disagree.

**When nothing derived the number, the bullet carries a `TODO(user):` where the digit would go.** The Shared rules above already demand that shape for any number nobody ran, and a rating is a number. Write `- **Odds of triggering**: TODO(user): run <the query> to place this on the scale.` and never a digit standing in for one. The bullet keeps its label either way, so the block still reads as six questions with one of them open.

## Technical Debt

Verbatim from the request.

```
## Overview
[What and why in 2-3 sentences, if possible in a "As a user..." type of sentence]

## ELI5
[explain the ticket in a way so someone with no knowledge of the project understands what needs to be done]

## Benefits
- [Improved maintainability / performance / etc.]

## Current State
[What exists now and why it's problematic]

## Desired State
[What the code should look like]

## Implementation Plan
1. [Phase 1 - file paths and changes]
2. [Phase 2 - file paths and changes]

## Migration Path
- [How to transition without breaking changes]

## Acceptance Criteria
- [ ] [Refactoring complete]
- [ ] [Tests still pass]
- [ ] [Performance improved by X]
```

## Epic

New. An epic's children carry the acceptance criteria, so this template has none of its own.

```
## Goal
[What this initiative delivers, and why now]

## ELI5

[explain the initiative in a way so someone with no knowledge of the project understands what is being built]

## Stories
- [KEY] [summary]
- [KEY] [summary]

## Out of scope
- [What this epic does not cover]

## Done when
- [The observable condition that closes this epic]
```

## Bug Subtask

New. This piece of work runs hours to a day, so a full template on it is noise.

```
[One paragraph naming what this piece is and which file it touches.]

- [ ] [step]
- [ ] [step]
```

## Limits on the rendered text

`description` caps at 32000 characters on the string branch. Check the rendered length before the call. Skip the check, and the create call fails after the tree is already half built.

Every leaf carries at least one real path from Step 4. When Step 4 found none, the leaf says so. Skip both, and the assignee has to redo the search Step 4 exists to do, the reason this skill reads the repo before it writes a ticket.
