# Merging and deduplicating findings (Step 2)

## Contents
- Collecting sub-agent results (the async protocol)
- A missing reviewer is not a clean reviewer
- Excluding unscored and unverified findings
- Pooling, deduplication, and merge
- Ordering the surviving findings

## Collecting sub-agent results

**First, account for every sub-agent you launched.** Classify each of the four as *reported*
(returned parseable JSON) or *missing* (returned nothing, errored, returned output you cannot
parse, or never notified before the give-up bound). Record the missing ones in
`reviewers_missing`. Then union the `roles_missing` arrays that the in-depth-review instances
report, and treat any instance whose `coverage` is `"partial"` as
partial here too. Never reason that running several in-depth instances means every lens ran at least once.
Measured: in one run two roles were silent in BOTH instances, so the union of what the instances DID return
covered neither. A lens that no instance reported on is a hole, not a covered lens, and the union
of findings can never be used to claim `complete`.

**Reviewer results arrive in each sub-agent's own final text, on a later turn than the launch.** The
Agent tool launches asynchronously. Its result carries launch metadata and an `agentId`, and never the
sub-agent's findings, so there is nothing to read at launch. Record the `agentId` of all four
sub-agents when you launch them. Then take a turn, harvest every `<task-notification>` in front of
you, match each `<task-id>` to a recorded `agentId`, and keep its `<result>` body. Keep taking turns
until all four are accounted for, OR until THREE CONSECUTIVE COLLECTING TURNS have brought zero new
arrivals. A collecting turn is ONE substantive tool call that names the sub-agent it checked on, so
re-read the diff for a sub-agent you are still waiting on. Three repeats of the same no-op are not
three turns.
Do NOT start the zero-arrival counter until you have taken at least as many collecting turns as you
launched sub-agents, with a floor of five, whether or not anything has arrived. The three zero-arrival
turns are counted FRESH from the moment the counter arms, so turns taken before arming never count
toward them. Sub-agents take minutes, not seconds, so a counter armed at launch measures your own
polling speed rather than a failure. If you reach the bound and have not yet re-armed the counter,
re-arm it EXACTLY ONCE and keep collecting. After that, honor the bound. Never end your turn with a
recorded `agentId` unaccounted for unless you have already used your single re-arm and are recording it
in `reviewers_missing` on that same turn. If you stop while one is outstanding you will not receive it
at all. A notification that arrives after the bound is still that sub-agent's
report. Fold it into the pool and remove it from `reviewers_missing`. The accounting is final only
when you emit your report. Do not go hunting for a result in any other channel, and do not treat the
absence of a launch value as a failure. A sub-agent that reported, but whose returned text is empty,
has reported nothing and belongs in `reviewers_missing`.

## A missing reviewer is not a clean reviewer

**A missing reviewer is not a clean reviewer, and must never be filled in.** Do not write findings
on behalf of a reviewer that did not report, do not infer what it would have found, do not run its
lens yourself and attribute it, and do not carry a result forward from elsewhere. Then:

- If `reviewers_missing` is non-empty, or any instance came back `"partial"`, this run's coverage
  is **partial**. Carry that flag through to Step 4 and to the clean-PR path.
- **Coverage computed here is provisional.** The approach stage (Step 2.7) has not run yet, and it
  is a reviewer too. If the proposer or the judge does not report, set `approach_stage_missing` and
  make coverage **partial** at that point. Coverage is final only after Step 2.7. Do not treat the
  Step 2 value as settled, or a run whose finders were all clean will report `complete` while the
  approach stage never ran.
- **A miss is reported immediately, not retried.** `review-and-fix` gives a missing reviewer kind
  one retry per run before marking it unavailable, and this skill deliberately does not. That skill
  loops, so it has a next iteration to relaunch into and a cheap way to spend one. This skill is
  one-shot, so there is no next pass, and a relaunch here would mean holding the whole review open
  on one sub-agent. Reporting partial coverage is the honest and cheaper answer. The asymmetry is
  intentional. Do not add a retry here to match the other skill.
- The "all clear" line in Step 4 asserts that N independent reviewers found nothing. **Do not emit
  it when fewer than N reported.** Say how many reported and which did not.
- Proceed with the review anyway. A partial review that says it is partial is useful; one that
  claims completeness it does not have is worse than none.

## Excluding unscored and unverified findings

**Check `scoring.complete` on every in-depth-review result.** An instance reporting `false` did not
run its two-stage confidence filter, so its numbers are self-assessments by the same model that
proposed the findings, not scores.

- **Do not feed its findings into the >=60 filter as though they were scored.** Treat them as
  unscored leads: exclude them from the posted review, list them separately in the Step 4 report as
  unfiltered, and name the instance that produced them.
- Any finding carrying `unscored: true` is likewise never posted.
- A finding arriving with `citation_verified: false` is **never posted**, whatever its score. It is
  capped at 60 upstream, but do not rely on that cap to keep it out. This skill's filter is
  `confidence >= 60`, which a capped finding satisfies exactly, so the cap alone would post it.
  Exclude it the same way `review-and-fix` excludes it from fixing. List it in the Step 4 report as
  an unverified-citation lead instead.

An unfiltered instance is not a free extra reviewer. Posting its findings puts a single model's
self-graded output into a PR review, which is how a fabricated finding gets published.

## Pooling, deduplication, and merge

Once collection has ended, whether all four reported or the give-up bound above was reached:

1. **Pool every finding** across the four result sets into one flat pool. Each finding carries
   its `confidence`, `file`, `line_range`, originating `sub_agent` (1..4), and `source`
   (`"in-depth-review"` or `"gh-style-review"`). Don't pre-segregate by source. The cross-
   prompt triangulation is the point.

2. **Group duplicates.** Two findings are duplicates if they refer to the **same file** and have
   **overlapping line ranges** AND describe substantially the same problem (paraphrases count).
   Findings from different sources (one from in-depth-review, one from gh-style-review) that
   describe the same problem are duplicates, so merge them.

3. **For each group, produce one merged finding:**
   - `confidence`: **max** of the group's scores (any one reviewer with high confidence is strong
     evidence; merging by max is intentionally non-conservative).
   - `cross_instance_agreement`: count of distinct sub-agents (1..4) that raised this finding.
     This is the name `review-and-fix` already uses. Do not invent a third.
     **Compute it yourself. Never reuse the `role_agreement` value on an incoming finding as this
     count.** The denominators differ. `role_agreement` is how many of ONE instance's
     8-12 role lenses raised the finding, and this is how many of the four independent instances
     did. Roles share a model and a context window and are prompted to look at different things,
     so several converging is correlated evidence. Separate instances converging is the
     independent signal, and it is the one that should drive ordering.
   - `role_agreement`: carry through the MAX across the group, for use as a lower tiebreaker.
     Keeping it preserves the role-level signal rather than discarding it.
   - `sources`: set of distinct sources (`{"in-depth-review"}`, `{"gh-style-review"}`, or both).
     A finding raised by both sources is stronger signal than a finding raised by only one;
     used as a tiebreaker in step 6.
   - `title`, `description`, `suggested_fix`: pick the clearest from the group; if suggested
     fixes differ meaningfully, mention the alternatives in the description.
   - `category`: union of categories.
   - `permalink`: take any one valid permalink from the group.
   - `ticket_id`: preserved from `ticket`-category findings (the Jira ID the gap traces to);
     `null` for all other findings. Never merge two findings that name different `ticket_id`s.
   - `citation_verified`: **merges DOWNWARD, unlike confidence.** If ANY member of the group carries
     `citation_verified: false`, the merged finding carries `false`. Never let a verified member's
     `true` overwrite an unverified member's `false`, and never drop the field during the merge. An
     absent field on the merged finding reads as "nothing to verify", which is how an unverified
     citation slips past the Step 5 exclusion.
   - `unscored`: merges downward the same way. Any `unscored: true` in the group makes the merged
     finding `unscored: true`.
   - **Then re-cap.** If the merged finding came out `citation_verified: false`, re-apply the 60 cap
     to the max confidence computed above. Confidence merges upward and this field merges downward,
     so without the re-cap a verified member scoring 85 would carry an unverified finding to 85 and
     straight through the filter. `in-depth-review` applies the same merge-down-then-recap rule
     inside its own dedup. This is the identical hazard one layer up, across instances rather than
     across roles.

4. **Snapshot the full merged set as `merged_all` BEFORE filtering.** `merged_all` is the
   complete list of merged findings with their max `confidence`, including the 51–59 ones that
   the next step discards. Step 2.7 (approach dedup) needs it, because once you apply the >=60 cut
   you can no longer tell whether a 51–59 finding "was already proposed by another reviewer."
   `merged_all` is used only by Step 2.7; it never affects posting on its own.

5. **Filter:** keep only findings with `confidence >= 60`. Then drop every finding with
   `citation_verified: false` or `unscored: true` regardless of score. The threshold does not
   remove them. An unverified citation is capped at 60 upstream, and this filter is inclusive, so
   60 passes it. These two exclusions are what keep an unverified or self-graded finding out of the
   posted review, so apply them here and not only where unfiltered instances are discussed. Report
   the dropped ones in Step 4 as leads. Discard everything else below the bar — with one
   exception: retain any discarded `ticket`-category finding (confidence < 60) in a separate
   `sub_threshold_ticket_notes` list. Step 3a.5 uses it for the "Tickets examined" section.
   These never enter the findings pool, the INLINE/GLOBAL classification, or the ordering
   below. They are notes, not findings. (Threshold is 60.)

   Note the two exclusions only ever fire on `in-depth-review`-sourced findings, because
   `gh-style-review` does not emit `citation_verified` or `unscored` at all. That is expected, not a
   gap to close. A gh-style finding has no commit citation to verify and is scored by its own
   skill. Do not read the exclusions as gating gh-style findings, and do not add the fields to
   gh-style's schema to make the filter symmetric.

## Ordering the surviving findings

6. **Order the surviving findings:**
   1. `confidence` descending
   2. `cross_instance_agreement` descending (4/4 > 2/4 > 1/4 when scores tie)
   3. Both-sources first (a finding raised by both in-depth-review and gh-style-review beats
      a same-confidence-and-agreement finding from a single source)
   4. `role_agreement` descending. A finding several role lenses independently raised beats one
      a single lens raised, once the stronger cross-instance signal has already tied
   5. `category` priority: bug > types > security > db > error-handling > AGENTS.md > history >
      prior PR > test coverage > motivation > comment guidance > ticket > approach. This chain
      covers every category the finding schema allows, so two tied findings always have a defined
      order. Any category not named here ranks last, immediately above nothing.

   **Do not deprioritize a solo finding from sub-agent 3 (the Opus finder) on agreement alone.**
   Catching what the Sonnet reviewers miss is precisely its job, so `cross_instance_agreement: 1` from that
   instance is expected rather than weak evidence. When an Opus-only finding ties on
   `confidence` with a multi-reviewer Sonnet finding, rank the Opus-only one first. This
   overrides tiebreaker 2 for that case and nothing else.
