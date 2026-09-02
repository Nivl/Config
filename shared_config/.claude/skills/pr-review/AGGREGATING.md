# Merging and deduplicating findings (Step 2)

## Contents
- Collecting the reviewer results (the workflow return for in-depth, the async protocol for gh-style)
- A missing reviewer is not a clean reviewer
- Excluding unverified and unscored findings
- Pooling, deduplication, and merge
- Ordering the surviving findings

## Collecting the reviewer results

**The in-depth roles arrive as the `review-roles` workflow's return value and need no collecting.**
`roles_missing` for instance N is `results.filter(r => r.instance === N && r.findings === null)`,
computed from that return rather than inferred from which notifications arrived. Union the two
instances' `roles_missing`, and treat a non-empty union as partial coverage. Never reason that
running two instances means every lens ran at least once. A role that came back `null` in BOTH
instances is a hole, and the union of what the instances DID return covered neither. Measured before
the workflow existed: two roles were silent in both instances of one run. The barrier makes that two
nulls rather than silence, and the rule is the same. A lens that no instance reported on is a hole,
not a covered lens, and the union of findings can never be used to claim `complete`.

If the `Workflow` tool was absent so the roles could not be dispatched at all, that is the no-fan-out
abort in the next section. It is neither reported nor missing. Stop accounting and stop there.

**The gh-style instance is the one Agent-tool sub-agent, and it is what the async protocol below is
for.** Classify it as *reported* (returned parseable JSON) or *missing* (returned nothing, errored,
returned output you cannot parse, or never notified before the give-up bound), and record a miss in
`reviewers_missing`. The in-depth kind never appears there, because the barrier cannot fail to
return. It reports its holes per role instead.

**The gh-style result arrives in the sub-agent's own final text, on a later turn than the launch.**
The Agent tool launches asynchronously. Its result carries launch metadata and an `agentId`, and never
the sub-agent's findings, so there is nothing to read at launch. Record its `agentId` when you launch
it. Then take a turn, harvest every `<task-notification>` in front of you, match its `<task-id>` to
that `agentId`, and keep its `<result>` body. Keep taking turns until it is accounted for, OR until
THREE CONSECUTIVE COLLECTING TURNS have brought zero new arrivals. A collecting turn is ONE substantive tool call that names the sub-agent it checked on, so
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

- **No `Workflow` tool means no in-depth roles ran, and that aborts the run.** The roles are
  dispatched from this thread through the `review-roles` workflow. If the tool is absent from your
  tool list the dispatch never happened, no role read the diff, and there is no review to compute
  coverage for. Do not post, do not carry the run forward as a review of any kind, and do not fall
  back to Agent-tool spawns of `in-depth-review`, because that nesting is the defect the workflow
  replaced. Say the fan-out was impossible and stop. This is the only way the in-depth kind can
  fail as a whole now. It cannot come back `impossible` from inside a sub-agent, because it no longer
  runs inside one.
- **Impossible is not partial, and it is not missing either.** A partial run had the barrier return
  with some roles `null`, which is a real review with holes in it. An impossible run had no barrier
  at all. A retry is futile rather than merely expensive. The `Workflow` tool that was absent on the
  first attempt is absent on the second.
- **Every role `null` at once is not twelve independent deaths.** If the workflow returns and every
  `findings` is `null`, the plausible reading is that `agentType: 'in-depth-review-role'` did not
  resolve, because the agent files were not synced or were renamed. Say so in the Step 4 report
  rather than listing twelve missing roles as though each failed on its own. Coverage is `partial`,
  and the cause named is what lets someone fix it.
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
- Proceed with the review anyway when coverage is merely partial. A partial review that says it
  is partial is useful; one that claims completeness it does not have is worse than none. An
  impossible instance is not this case. That run aborted above and there is no review to proceed
  with.

## Excluding unverified and unscored findings

**In-depth findings arrive unscored by design, and that is not an exclusion.** The `review-roles`
workflow returns raw per-role findings with `confidence: null` and no `scoring` block, because this
orchestrator scores the merged set itself in the step below. There is no `scoring.complete` to check
on an in-depth result and no `unscored` flag on its findings. A null confidence from the workflow
means "not yet scored", never "a scorer was owed and did not deliver". Carry those findings into the
pool unchanged and let the scoring step give them numbers.

**The gh-style instance still scores its own findings**, so its result carries the old shape. Check
`scoring.complete` on it. A `false` means its numbers are self-assessments by the model that
proposed them, not scores.

- **Do not feed gh-style's findings into the >=60 filter as though they were scored** when its
  `scoring.complete` is `false`. Treat them as unscored leads: exclude them from the posted review,
  list them separately in the Step 4 report as unfiltered, and name the instance.
- Any gh-style finding carrying `unscored: true` is likewise never posted.
- A finding arriving with `citation_verified: false`, from either kind, is **never posted**,
  whatever its score. The rubric caps it at 60, but do not rely on that cap to keep it out. This
  skill's filter is `confidence >= 60`, which a capped finding satisfies exactly, so the cap alone
  would post it. Exclude it the same way `review-and-fix` excludes it from fixing. List it in the
  Step 4 report as an unverified-citation lead instead.

An unfiltered instance is not a free extra reviewer. Posting its findings puts a single model's
self-graded output into a PR review, which is how a fabricated finding gets published. The in-depth
findings avoid this by construction, because the only scores they ever carry come from the
`review-scorer` step below.

## Pooling, deduplication, and merge

Once the workflow has returned and the gh-style instance has reported or hit the give-up bound:

1. **Pool every finding** from every non-null `results[*].findings` array and from the gh-style
   result into one flat pool. Each in-depth finding carries the `instance` and `role` of the result
   it came from, plus `file`, `line_range`, `category`, and `confidence: null`. Each gh-style
   finding carries `source: "gh-style-review"`, its own scored `confidence`, and no instance. Don't
   pre-segregate by source. The cross-prompt triangulation is the point.

2. **One dedup pass, across roles and instances together.** Two findings are duplicates if they
   refer to the **same file**, have **overlapping line ranges**, AND describe substantially the same
   problem (paraphrases count). Findings from different sources (one in-depth, one gh-style) that
   describe the same problem are duplicates, so merge them.

   This used to be two layers. Each in-depth instance deduped across its own roles before returning,
   and this step deduped across instances. The workflow returns raw per-role findings, so the layers
   collapse into this one pass, and both agreement counts fall out of the tags:
   - `role_agreement`: the number of distinct `role` values among the group's members from any one
     instance, taking the larger instance's count when they differ.
   - `raised_by`: the SET of distinct `instance` values among the group's members.
   - `cross_instance_agreement`: the size of `raised_by`.

3. **For each group, produce one merged finding:**
   - `confidence`: not set here. It stays `null` until the scoring step below, which assigns one
     score per unique finding after this merge. The rule was the `max` of the group's scores,
     described as intentionally non-conservative, and with each instance scoring independently it
     kept the largest of several noisy draws. That is a bias rather than a tie-break, and
     corroboration was already counted on purpose by `cross_instance_agreement` below. There is no
     group of scores left to reduce.
   - `cross_instance_agreement`, `raised_by`, `role_agreement`: as computed in step 2 from the
     `instance` and `role` tags. `cross_instance_agreement` is the name `review-and-fix` already
     uses. Do not invent a third. The denominators differ and that is the point. `role_agreement` is
     how many of ONE instance's 8-12 role lenses raised the finding, and `cross_instance_agreement`
     is how many independent instances did. Roles share a model and a context window and are prompted
     to look at different things, so several converging is correlated evidence. Separate instances
     converging is the independent signal, and it is the one that should drive ordering.
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
   - **There is nothing to re-cap here any more, and the hazard moved rather than went away.** The
     old rule re-applied the 60 cap after the merge, because confidence merged upward while this
     field merged downward, so a verified member scoring 85 would otherwise carry an unverified
     finding to 85 and through the filter. No confidence exists at this point now, so the cap is the
     scorer's job in the step below and it is applied once, to the merged `citation_verified` value
     this step just resolved. That ordering is what makes it correct rather than a second attempt.
     Pass the merged value to the scorer, never a member's own.

4. **Score the merged set, once.** Every finding in the pool carries `confidence: null` at this
   point, because the `review-roles` workflow returns them unscored. Scoring used to happen
   inside each instance, before anything had merged, so a finding several instances raised was
   scored once per instance and all but one number was discarded. With three instances the waste and
   the max bias were both larger here than in `review-and-fix`.

   This step comes BEFORE the `merged_all` snapshot below, and the order is load-bearing. Step 2.7
   reads `merged_all` to check whether a reviewer already proposed the same thing with
   confidence > 50, so a snapshot taken before scoring would hand that check a set of nulls and
   every approach finding would read as novel.

   Spawn `subagent_type: review-scorer` and pass no `model` override, so its agent file stays the
   source of truth for its tier. Begin its prompt with the stamp
   `<!-- review-scorer batch=<B> tag=pr<PR> target=<PR> -->`, one line, so the usage accounting in
   [USAGE.md](../review-and-fix/USAGE.md) can find its transcript. Then give it, per finding: the
   identifier, `title`, `description`,
   `suggested_fix`, `severity`, `role_agreement`, `cross_instance_agreement`, the merged
   **`citation_verified`** from step 3, the diff for its lines, and the paths of any AGENTS.md or
   CLAUDE.md a reviewer cited for it. Name the rubric's path in the prompt.

   **`citation_verified` matters more here than in `review-and-fix`.** That skill excludes unverified
   findings before its merge, so its scorer never sees one. This skill merges first, so the scorer
   does see them and is the thing applying the rubric's 60 cap. The cap value equals this skill's own
   threshold exactly, so a capped finding satisfies `confidence >= 60` and the cap alone would post
   it. What keeps it out is the separate `citation_verified: false` drop in the filter below, which
   is why both exist.

   One agent for up to about 60 findings. Above that, batches of about 20, so a large review degrades
   into a few agents rather than one holding everything.

   **The ladder. After every rung, check whether any finding is still unscored, and stop as soon as
   none is.**

   1. Spawn the scorer with every unique finding.
   2. Nothing came back at all. Nudge it once with `SendMessage`, asking it to finalize with
      whatever it genuinely has.
   3. Nothing came back, or only part did. Spawn ONE fresh scorer with only the findings still
      unscored. Exactly one relaunch, because unbounded relaunches are the same bug with extra
      bookkeeping. This rung sits before the inline one because a stall is usually transient.
   4. Every finding has a score. Continue to the snapshot and filter below.
   5. Findings are still unscored. Score those inline, and mark them `inline-fallback`.

   **Rung 5 is not the self-scoring this repo bans.** That ban exists because a proposer graded its
   own invention and a fabricated finding scored 100. The proposers here are role agents inside
   `in-depth-review`, and this orchestrator merged their output without proposing anything.

   It carries a different bias, which is why it is last and why it is marked. This orchestrator is
   the model that POSTS these findings to a pull request, so it has an interest in what it will be
   seen to have posted that a role agent does not have. A number from rung 5 is worth having and is
   not worth the same as a number from rung 1. Reaching it still beats posting nothing because a
   scorer died.

   **Record `scored_by` on every finding**, one of `scorer`, `scorer-retry`, or `inline-fallback`,
   and `inline_fallback_count` for the run. Per finding rather than per run, because the ladder
   mixes provenance and one label would hide which numbers to distrust. Surface a nonzero
   `inline_fallback_count` in the Step 4 report.

5. **Snapshot the full merged set as `merged_all` BEFORE filtering.** `merged_all` is the
   complete list of merged findings with their max `confidence`, including the 51–59 ones that
   the next step discards. Step 2.7 (approach dedup) needs it, because once you apply the >=60 cut
   you can no longer tell whether a 51–59 finding "was already proposed by another reviewer."
   `merged_all` is used only by Step 2.7; it never affects posting on its own.

6. **Filter:** keep only findings with `confidence >= 60`. Then drop every finding with
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

7. **Order the surviving findings:**
   1. `confidence` descending
   2. `cross_instance_agreement` descending (3/3 > 2/3 > 1/3 when scores tie)
   3. Both-sources first (a finding raised by both in-depth-review and gh-style-review beats
      a same-confidence-and-agreement finding from a single source)
   4. `role_agreement` descending. A finding several role lenses independently raised beats one
      a single lens raised, once the stronger cross-instance signal has already tied
   5. `category` priority: bug > types > security > db > error-handling > AGENTS.md > history >
      prior PR > test coverage > motivation > comment guidance > ticket > approach. This chain
      covers every category the finding schema allows, so two tied findings always have a defined
      order. Any category not named here ranks last, immediately above nothing.

   **The finder wrappers are all on the same tier, so no instance gets an ordering exemption.**
   An earlier rule promoted a solo finding from a third in-depth instance whose wrapper ran on
   Opus, on the grounds that catching what the others miss was its job. That instance is gone and
   the exemption went with it. Tiebreaker 2 applies to every finding now. Do not reintroduce a
   per-instance override without a tier difference to justify it.
