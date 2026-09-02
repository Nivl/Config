# work-on files follow-up tickets through open-ticket, design

Date: 2026-08-23
Status: design approved, ready for an implementation plan
Path note: `docs/superpowers/` is gitignored in this repo, so this file is local scratch. There is nothing to commit here.

## What this is

`work-on` cuts scope and writes what it cut into the ticket's "Out of scope" section. It creates nothing. This change lets it file the cut work as a real Jira ticket, by delegating to `open-ticket` rather than growing a second create path.

Two skills change. `open-ticket` gains a delegated entry for a caller that already knows the project, the scope and the files. `work-on` gains two places where a follow-up gets proposed and filed.

## What exists today, for the reader who has not read work-on

- Step 3 can ask the user a scope question, in the shape "The batch path has the same bug. In scope, or a separate ticket?"
- Step 4's "Partially valid" verdict presents a split and gets explicit agreement on a reduced scope before Step 5 runs.
- Step 4 already handles the case where somebody else's follow-up carved part of this ticket out, and names that key so the user can see the work is not being dropped.
- Step 5 writes an "Out of scope" section into the ticket description.
- Step 8 runs `review-and-fix` to completion.

Nothing in that list creates a ticket. The cut work lands as prose and stops there.

## Decisions taken during design

| Decision | Choice |
|---|---|
| How the two skills compose | Delegated mode. `work-on` hands over what it knows and `open-ticket` skips the steps that answers. |
| Which approval governs the create | Step 4's existing decision absorbs it. No new prompt for Trigger A. |
| What triggers a follow-up | Both the Step 4 scope cut and the out-of-scope leftovers from Step 8. |
| When Trigger A files | Between Step 4 and Step 5, so Step 5's "Out of scope" can name the new key. |
| The parent ticket in the dedup sweep | Excluded from the credible-match set. A sibling follow-up is not. |

## One discovery that changed the second trigger

`review-and-fix` has no out-of-scope category. Its "Remaining Issues" section covers findings that did not become a commit, and the stated reasons are a row 2 stop, a fix abandoned because lint or tests failed, and a finding skipped for want of a test. Every one of those is unfinished work on this change. Its "Tickets examined" section carries deferred and dismissed ticket findings with a decision. Nothing in either section says a finding belongs to a different ticket.

So Trigger B has no ready-made input. `work-on` classifies the leftovers itself, against the scope agreed at Step 4, and proposes only the ones that belong elsewhere. That keeps the change to two skills. Teaching `review-and-fix` to classify scope would be a third skill and a wider change than this earns.

## The design

### 1. open-ticket gains a delegated entry

A section near Step 0 stating what a caller may supply and which steps that satisfies.

| Supplied by the caller | Satisfies |
|---|---|
| project key | Step 2's project inference |
| the requirement text | Step 1 |
| the files involved | Step 4's exploration |
| the originating ticket's key | Step 5's exclusion below, and the description's context line |
| the originating ticket's own parent, when it has one | Step 7's parenting |

**The follow-up is a sibling of the originating ticket and never its child.** This is a Jira validity rule and not a preference. `open-ticket` parents a `Story`, `Task` or `Bug` to an `Epic`, and parents a subtask type to a `Story`. So a follow-up `Story` cannot take a `Story` as its parent. Filing it under the originating ticket would either fail the create or force the follow-up to be a subtask of work it is not part of.

The rule that follows. Read the originating ticket's own `parent`. If it has one, the follow-up takes the same parent and lands beside it. If it has none, the follow-up is filed with no parent. Either way the relationship to the originating ticket is carried in prose, in the follow-up's description and in the originating ticket's "Out of scope" section, because `open-ticket` writes no issue links beyond `parent`.

Still runs, always. Step 0's preflight, because a caller cannot vouch for Jira access. Step 3's sizing. Step 5's dedup sweep and Step 6's gate, because the caller's scope decision says nothing about whether somebody already filed it. Steps 7 through 12 unchanged.

Sprint is not supplied and is not inferred in delegated mode. A follow-up belongs in the backlog until somebody schedules it, and Step 2's open-sprint query is about what the caller is working on now.

The delegated entry is a narrowing of the existing pipeline and never a second pipeline. `CREATE-FIELDS.md` stays the single copy of every field id and every recipe.

### 2. The parent is never a duplicate

In delegated mode, the supplied parent key is excluded from Step 6's credible-match set.

Without this, Step 5's sweep finds the parent, since the follow-up is by construction about the same area and often the same files, and Step 6 aborts with `DUPLICATE_FOUND` naming the ticket the caller is mid-way through fixing. That would stop `work-on` on its own work.

A sibling follow-up already carved off the same parent stays a credible match, and is the most valuable hit the sweep can return here. It is the case `work-on` Step 4 already detects from the other direction.

### 3. Trigger A, the scope cut at Step 4

Step 4's "Partially valid" branch already presents the split and the proposed reduced scope. It gains the follow-up proposal in that same presentation. Type, one-line summary, points, and the parent key.

The user's agreement to the reduced scope is the approval for the create. No separate gate, and `open-ticket`'s Step 9 is satisfied by that recorded agreement rather than re-asked.

The reason this is principled and not a shortcut: `work-on` already rewrites the parent ticket's whole description with no preview, on the stated rule that it asks about decisions and never about wording. Whether to file a follow-up for cut scope is a decision, and the user makes it at Step 4. The follow-up's prose is wording.

Filing happens after Step 4's agreement and before Step 5's rewrite, so Step 5's "Out of scope" section names the key.

If the create fails, Step 5 still runs. The "Out of scope" section then describes the cut work in prose with no key, and the final report says the follow-up could not be filed. A failed follow-up never blocks the parent ticket's rewrite.

### 4. Trigger B, the leftovers at Step 8

After `review-and-fix` returns, read its "Remaining Issues" and "Tickets examined" sections and sort each item into one of two buckets.

**Unfinished here.** A fix abandoned because lint or tests failed. A finding skipped for want of a test. A row 2 stop. None of these becomes a ticket. Step 8 says run `review-and-fix` all the way, and filing a ticket for a test failure converts a failure into a backlog item that reads like planned work.

**Belongs to a different ticket.** A finding about code the agreed scope does not cover. These get proposed.

The proposal is one batch with one yes, at the end of Step 8. This trigger needs its own approval because the findings did not exist when the user decided scope at Step 4, so Step 4's agreement cannot cover them.

Nothing is proposed when the second bucket is empty, and the run says nothing about it.

Trigger B files through the same delegated entry as Trigger A, with one difference. The originating ticket's key is supplied for the dedup exclusion and for the description's context line, and the parenting rule from section 1 applies unchanged. A finding about adjacent code is a sibling of this ticket at best, so it never becomes a child of it.

### 5. The cap

More than three items in the second bucket is a signal about the change rather than three tickets. Report the count, propose the batch, and let the user decide. Do not file silently past that point and do not truncate the list without saying so.

This cap exists because the risk was named during design and accepted. Findings from Step 8 are machine-generated and carry no human scope call of their own, so a noisy review round could otherwise turn one `work-on` run into a stack of tickets nobody asked for.

## Out of scope

- Teaching `review-and-fix` to classify a finding's scope. That is a third skill.
- Filing follow-ups for `TODO(user):` lines. Those lines exist because something could not be settled, which is thin ground for a ticket, and a run with four of them would file four.
- Filing anything from PR review comments at Step 9.
- Trees. A follow-up is one issue. `open-ticket`'s points rule still applies, so a follow-up estimated over 5 gets reported as too large to file as one ticket rather than silently split.
- Changing `work-on`'s existing behavior for a follow-up somebody else already filed. Step 4 keeps naming that key as it does now.

## Files

| File | Change |
|---|---|
| `shared_config/.claude/skills/open-ticket/SKILL.md` | The delegated entry section. The parent-exclusion rule in Step 5 and Step 6. |
| `shared_config/.claude/skills/work-on/SKILL.md` | Step 4 gains the proposal. A filing step between Step 4 and Step 5. Step 8 gains the classification and the batch proposal. |
| `tests/open_ticket_contract_test.sh` | Assertions for the delegated section and the parent-exclusion rule. |

No new files.

## Testing, and where it does not reach

`tests/open_ticket_contract_test.sh` already guards `open-ticket`'s load-bearing rules, so the delegated entry and the parent-exclusion rule get assertions there. Follow its established mechanics. A heading assertion uses `assert_line_count` with a fully anchored pattern against the raw file, because a heading needle absorbs into a deeper heading. A prose-phrase assertion uses `assert_contains` against the flattened text, because prose is hand-wrapped. Any needle meant to test that a rule is stated must appear in a real sentence and not only in a heading.

**`work-on` has no contract test.** Its only test, `tests/work_on_validation_test.sh`, extracts the embedded javascript from `VALIDATION.md` and exercises one reducer. Prose changes to `work-on/SKILL.md` have no CI coverage today and get none from this change. Every rule this design adds to `work-on` rests on review, not on a test. Say so rather than implying the suite covers it.

Adding a contract test for `work-on` would be a reasonable follow-up and is deliberately not in this change.

## Known risk, accepted

Trigger B files tickets from machine-generated findings with no human scope decision behind each one. The cap and the required batch approval are the mitigations. The failure mode to watch on the first real runs is a `work-on` run that proposes follow-ups for work that was really unfinished on the branch, which would mean the two buckets in section 4 are not being sorted correctly.
