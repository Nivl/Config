# Aggregating reviewer results

## Contents
- Collecting the reviewer results (the workflow return for in-depth, the async protocol for gh-style)
- A missing reviewer is not a clean reviewer (retry / unavailable rules)
- Excluding unverified and unscored findings
- Pooling, deduplication, and merge (the `>=50` filter)
- The attribution ledger, and why `unique_actionable` is the counterfactual
- Aggregating `tickets_examined`

## Collecting the reviewer results

**The in-depth roles arrive as the `review-roles` workflow's return value and need no collecting.**
`roles_missing` for instance N is `results.filter(r => r.instance === N && r.findings === null)`,
computed from that return rather than inferred from which notifications happened to arrive. Union
the two instances' `roles_missing` for the coverage rule below. Never reason that running two
instances means every lens ran at least once. A role that came back `null` in BOTH instances is a
hole, and the union of what the instances DID return covered neither. Measured before the workflow
existed: two roles were silent in both instances of one run. The barrier makes that visible as two
nulls rather than as silence, and the rule is the same.

**The gh-style instance is the one Agent-tool sub-agent left, and it is what the async protocol
below is for.** Classify it as *reported* or *missing* (nothing returned, errored, unparseable output
that is not the no-fanout sentinel, or never notified before the give-up bound), and record a miss
in `reviewers_missing` under the gh-style kind. The in-depth kind never appears in
`reviewers_missing`, because the barrier cannot fail to return. It reports its holes per role instead.

The gh-style result arrives in the sub-agent's own final text, on a later turn than the launch,
never in the Agent tool's launch result. Record its `agentId` at launch, then take turns and harvest
each `<task-notification>`, matching its `<task-id>` to that `agentId`. Keep taking turns until it
is accounted for, OR until THREE CONSECUTIVE
COLLECTING TURNS have brought zero new arrivals. A collecting turn is ONE substantive tool call
naming the sub-agent it checked on, so re-read the diff for a sub-agent you are still waiting on.
Three repeats of the same no-op are not three turns.
Do NOT start the zero-arrival counter until you have taken at least as many collecting turns as you
launched sub-agents, with a floor of five, whether or not anything has arrived. The three zero-arrival
turns are counted FRESH from the moment the counter arms, so turns taken before arming never count
toward them. Sub-agents take minutes, not seconds, so a counter armed at launch measures your own
polling speed rather than a failure. If you reach the bound and have not yet re-armed the counter,
re-arm it EXACTLY ONCE and keep collecting. After that, honor the bound. If you have no other work,
re-read the diff for a sub-agent you are still waiting on. Never end your turn with a recorded
`agentId` unaccounted for unless the give-up bound's single re-arm has already been used and you are
recording it in `reviewers_missing` on that same turn. If you stop while one is outstanding you will
not receive it at all. A notification that arrives after the bound is still
that sub-agent's report. Fold it into the pool and remove it from `reviewers_missing`. The
accounting is final only when you emit your report. A sub-agent that reported, but whose returned
text is empty, has reported nothing, whatever it may have sent over any other channel.

**Never fabricate a missing reviewer's findings**, and never fix on an inferred finding. Do not
write what a missing reviewer would have found, do not run its lens in the parent and attribute it,
and do not reuse a previous iteration's output for it. This skill commits code, so an invented
finding becomes an invented commit.

**Unavailability is keyed by reviewer KIND, and only the gh-style kind can become unavailable.**
Two kinds exist, the in-depth unit and the `gh-style-review` unit. The gh-style kind **fell short**
in an iteration when its instance did not report. A shortfall is retried by relaunching it once per
run. On a second shortfall the kind is marked `unavailable` for the rest of the run. `unavailable`
affects only future launches. It never discards a report already received. `reviewers_missing`,
`reviewer_unavailable`, and `reviewer_retries` are keyed by kind, and in practice hold only gh-style.

**The in-depth kind has no retry here, because it cannot fall short as a kind any more.** The
`review-roles` workflow returns for every role, with `null` where a role returned nothing after its
own one retry inside the barrier. A null role is a missing ROLE, recorded in that instance's
`roles_missing`, and it is not a kind that failed to report. Relaunching the whole workflow for one
null role would rerun 23 roles that already answered, and the barrier already spent the retry that
was worth spending. The old kind-level retry existed because a wrapper sub-agent could go silent as
a unit, and the roles inside it went with it. There is no wrapper now.

The 2x in-depth multiplicity is still triangulation, not two different lenses, and `<ACTIVE_ROLES>`
plus `<ACTIVE_GH_STYLE>` still express every launch decision. Do not add reduced-multiplicity
support. The two instances are two runs of each role inside one workflow call, and the call takes
`instances: 2` or it takes nothing.

## A missing reviewer is not a clean reviewer

The stakes are higher here than in `pr-review` because this skill acts on findings rather than
just posting them:

- **Separate a deterministic refusal, a transient failure, and a context that cannot host a
  review.** A reviewer that returned a
  `skipped_reason` bailed out on purpose (an empty `--roles` set, PR mode with no `gh`, a closed
  PR). Re-running it cannot change the outcome. A reviewer that returned nothing at all, or
  unparseable output, may simply have flaked. A reviewer that came back `impossible` reviewed
  nothing at all.

  | reviewer state | meaning | action |
  |---|---|---|
  | returned `coverage: "impossible"`, OR the `REVIEW_UNAVAILABLE_NO_FANOUT` line instead of parseable JSON | no fan-out, so nothing was reviewed | abort the run; never mark the kind `unavailable` and never retry it |
  | missing WITH `skipped_reason` | deterministic refusal | mark `unavailable` immediately; never relaunch it this run |
  | missing WITHOUT `skipped_reason` | possibly transient | relaunch **at most once per run**; on the second miss mark `unavailable` |

  `impossible` is not the `unavailable` path, whether the instance returned the JSON `coverage`
  value or the text-form `REVIEW_UNAVAILABLE_NO_FANOUT` line. `unavailable` means a reviewer that
  could have worked and did not, so the run can honestly stop with partial coverage. `impossible`
  means no reviewer can ever work in this context, so every further iteration fails identically,
  and stopping with `partial` would relabel a broken harness as a finished review.

  The retry budget is **one per reviewer per RUN, not per iteration.** A per-iteration budget would
  grant a fresh relaunch on every pass, and nothing caps the iteration count, so it would permit
  unbounded relaunches. That is the same bug with extra bookkeeping.
- **An `impossible` instance is not a missing instance either.** Missing feeds `reviewers_missing`
  and the retry budget. `impossible` bypasses both and aborts the run.
- **An `unavailable` reviewer does not have to report for the loop to stop, and the loop MAY stop
  with one.** It has to be able to, or the loop never terminates. But stopping that way is not a
  clean result. An `unavailable` kind forces `batch_clean` false rather than being excluded from
  that test, Coverage stays `partial`, and the run ends on the "Stopped with incomplete coverage"
  outcome, naming the reviewer and what went unreviewed. Both properties hold at once. A lost
  reviewer can never block the loop from stopping, and the loop never claims a clean result it did
  not earn.
- Never count a non-response as "found nothing".
- **A short kind must not satisfy the loop-exit condition.** A batch is only clean when every kind
  launched this iteration reported, reported nothing, `reviewer_unavailable` is empty, AND the
  unioned `roles_missing` is empty. All four are required. Step 3's table is the authority on which
  row fires. Whether to relaunch a short
  kind follows the retry rule above rather than being automatic. Relaunch it only while it still
  has retry budget, never relaunch a kind already in `reviewer_unavailable`, and let row 1c
  terminate the run once a kind is `unavailable`.
- Surface `reviewers_missing` and the unioned `roles_missing` in the per-iteration summary. The
  unioned `roles_missing` also feeds the Final Report's Coverage section. A shortfall that a later
  relaunch cleared stays in the per-iteration summary and goes no further, since it left nothing
  unreviewed. A run that fixed everything the reviewers that *did* report found is not a run that
  fixed everything.

## Excluding unverified and unscored findings

**In-depth findings arrive unscored by design, and that is not an exclusion.** The `review-roles`
workflow returns raw per-role findings with `confidence: null` and no `scoring` block, because this
orchestrator scores the merged set itself in the step below. There is no `scoring.complete` to check
on an in-depth result and no `unscored` flag on its findings. A null confidence from the workflow
means "not yet scored", never "a scorer was owed and did not deliver".

- **`citation_verified: false` excludes, per finding, BEFORE the pool.** Verification happens at
  role emit time and has nothing to do with scoring, so this applies to every in-depth finding
  exactly as it did before. Excluding first is what keeps this skill's merge safe without needing a
  downward-merge rule for the field. `pr-review` merges first and so needs one.
- **The gh-style instance still scores its own findings**, so its result carries the old shape.
  Check `scoring.complete` on it. A `false` means its numbers are self-assessments by the model that
  proposed them, so report its findings as unfiltered leads in the per-iteration summary, name the
  instance, and leave them unfixed. Any gh-style finding carrying `unscored: true` is treated the
  same way.
- **Fixing on an unscored finding is how a fabricated finding becomes a commit.** This skill edits
  and commits code, so the cost of acting on a self-graded finding is a commit that encodes an
  invented problem. When in doubt, leave it and report it. The in-depth findings avoid this by
  construction, because the only scores they ever carry come from the `review-scorer` step below.

## Pooling, deduplication, and merge

Once collection has ended, whether every active sub-agent reported or the give-up bound above was
reached (up to 3 active; fewer when the iteration is pruned):

1. **Pool every finding** from every non-null `results[*].findings` array and from the gh-style
   result into one flat pool. Each in-depth finding carries the `instance` and `role` of the result
   it came from, plus `file`, `line_range`, `category`, and `confidence: null`. Each gh-style
   finding carries `source: "gh-style-review"`, its own scored `confidence`, and no instance. Don't
   pre-segregate by source. Cross-prompt triangulation is the point.

2. **One dedup pass, across roles and instances together.** Two findings are duplicates if they
   refer to the **same file**, have **overlapping line ranges**, AND describe substantially the same
   problem (paraphrases count). Cross-source duplicates (one from in-depth, one from gh-style) count
   as duplicates, so merge them.

   This used to be two layers. Each in-depth instance deduped across its own roles before returning,
   and this step deduped across instances. The workflow returns raw per-role findings, so the layers
   collapse into this one pass, and both agreement counts fall out of the tags rather than being
   carried in from an upstream merge:
   - `role_agreement`: the number of distinct `role` values among the group's members from any one
     instance, taking the larger instance's count when they differ.
   - `raised_by`: the SET of distinct `instance` values among the group's members.
   - `cross_instance_agreement`: the size of `raised_by`.

3. **For each duplicate group, produce one merged finding:**
   - `confidence`: not set here. It stays `null` until the scoring step below, which assigns one
     score per unique finding after this merge. The rule here used to be the `max` of the group's
     scores, and that was a bias rather than a tie-break. Each instance scored its own findings
     independently, so a finding two instances raised had two noisy draws and the larger was kept.
     Corroboration then counted twice, once through that max and once deliberately through
     `cross_instance_agreement` below, and only the second was anybody's intent. There is no group
     of scores left to reduce.
   - `raised_by`, `cross_instance_agreement`, `role_agreement`: as computed in step 2 from the
     `instance` and `role` tags. Keep `raised_by` as the set, not just its size. A one-member set is
     a finding only that instance found, and per-instance marginal value cannot be computed from a
     count. `cross_instance_agreement` is derived from the set so the two cannot disagree.
   - `roles`: the set of `role` values among the group's members, kept per contributing instance so
     a one-member `raised_by` can still say which role found it. gh-style has no roles, so its
     contribution is recorded as `gh-style` rather than a number.
   - `sources`: set of distinct source skills (one of `{in-depth-review}`, `{gh-style-review}`,
     or both). Used as a tiebreaker. A both-source finding is stronger signal.
   - `title`, `description`, `suggested_fix`: pick the clearest from the group; if suggested
     fixes differ meaningfully, mention the alternatives in the description.
   - `category`: union of categories.
   - `ticket_id`: preserved from `ticket`-category findings (the Jira ID the gap traces to);
     `null` for all other findings. Never merge two findings with different `ticket_id`s.

4. **Score the merged set, once.** Every in-depth finding in the pool carries `confidence: null` at
   this point, because the `review-roles` workflow returns them unscored. This is the step that was worth
   moving here. Scoring inside each instance meant a finding two instances found was scored twice
   by two different agents and one number was discarded. Measured on iteration 3 of
   `20260829-004744-api-ml-stripe-webhooks-GRO-18050`, the instances returned 38 and 42 findings, so
   80 were scored to produce about 46 unique ones.

   Spawn `subagent_type: review-scorer` and pass no `model` override, so its agent file stays the
   source of truth for its tier. Give it, per finding: the finding's identifier, `title`,
   `description`, `suggested_fix`, `severity`, `role_agreement`, `cross_instance_agreement`,
   **`citation_verified`**, the diff for its lines, and the paths of any AGENTS.md or CLAUDE.md a
   reviewer cited for it. Name the rubric's path in the prompt, which is
   `~/.claude/skills/in-depth-review/SCORING.md`.

   `citation_verified` is not optional. The rubric caps an unverified-citation finding at 60, the
   scorer refuses to score rather than guess when the field is missing, and `in-depth-review` no
   longer applies the cap itself on a deferred run. This skill excludes `citation_verified: false`
   before the merge, so in practice the scorer sees `true` throughout, and passing it anyway is what
   keeps that a fact the scorer can check rather than an assumption it has to make.

   One agent for up to about 60 findings. Above that, batches of about 20, so a large iteration
   degrades into a few agents rather than one holding everything.

   **The ladder. After every rung, check whether any finding is still unscored, and stop as soon as
   none is.**

   1. Spawn the scorer with every unique finding.
   2. Nothing came back at all. Nudge it once with `SendMessage`, asking it to finalize with
      whatever it genuinely has.
   3. Nothing came back, or only part did. Spawn ONE fresh scorer with only the findings still
      unscored. Exactly one relaunch, because unbounded relaunches are the same bug with extra
      bookkeeping, and one matches the retry per reviewer kind per run that row 1b already allows.
      This rung sits before the inline one because a stall is usually transient. Three roles that
      never reported in one measured iteration each finished in under a minute when the next
      iteration reran them.
   4. Every finding has a score. Continue to the threshold below.
   5. Findings are still unscored. Score those inline, and mark them.

   **Rung 5 is not the self-scoring this repo bans.** That ban exists because a proposer graded its
   own invention and a fabricated finding scored 100. The proposers here are role agents inside
   `in-depth-review`, and this orchestrator merged their output without proposing anything.

   It does carry a different bias, which is why it is the last rung and why it is marked. This
   orchestrator is the model that will be asked to FIX these findings, so it has an interest in
   their scores that a role agent does not have. A number from rung 5 is worth having, and it is not
   worth the same as a number from rung 1. Reaching rung 5 beats the alternative, which is
   discarding a whole review's worth of correct findings for want of a number.

   **Record `scored_by` on every finding**, one of `scorer`, `scorer-retry`, or `inline-fallback`.
   Per finding rather than per iteration, because the ladder produces mixed provenance inside a
   single iteration and one label would hide which numbers to distrust. Record
   `inline_fallback_count` for the iteration too, so a run that reached rung 5 says so without a
   reader checking every finding.

5. **Apply the orchestrator's confidence threshold: discard everything with `confidence < 50`.**
   This is the review-and-fix-specific threshold, lower than each sub-skill's default of 70
   because cross-instance triangulation across the active passes raises our confidence in
   50–69 findings.

6. **If the post-filter list is empty** (every finding scored < 50), mark the **findings**
   batch clean. The iteration is **fully clean** only if Step 1.5's Discussion Context
   aggregation is also empty (PR mode). See Step 1.5 in SKILL.md.

7. **Otherwise** proceed to Step 2 with the filtered + deduplicated list, ordered by:
   1. Severity descending (critical -> major -> minor -> suggestion)
   2. `cross_instance_agreement` descending (more instances agreeing wins when severity ties)
   3. Both-sources first (a both-source finding beats a same-confidence single-source one)
   4. `confidence` descending

## The attribution ledger

Built here, from `raised_by`, once the merge and the threshold have both run. It exists to answer
one question with measurements rather than argument: what would a one-instance run have missed. Per
active instance, keyed by `sub_agent`:

- `raw`: how many findings it returned.
- `pooled`: how many of those entered the pool. Lower than `raw` when something was excluded before
  the merge, such as an instance-level `scoring.complete: false` taking its unscored findings out as
  unfiltered leads. Name the reason next to the number.
- `unique`: merged findings whose `raised_by` is exactly this instance.
- `shared`: merged findings this instance raised alongside at least one other. `unique` is only
  readable next to this. Three unique out of forty is a different claim from three out of five.
- `unique_kept`: of its `unique`, how many cleared the `>= 50` threshold.
- `roles`: for its `unique_kept`, which roles produced them. In-depth only, so gh-style records
  `n/a` rather than an empty value, which would read as a role list nobody filled in.
- `state`: reported, partial, or failed, naming what was lost. An instance that returned nothing
  still gets this field, and it is the one that separates a quiet instance from a dead one.

**Append these to the run log the moment you have them, one line per instance, before Step 2
starts.** Not later as an assembled table. This file's own record is that a shape specified for
later assembly does not get written: the duration summary was specified three ways and produced
zero rows across seven runs, and the commit table was emitted for iteration 1 only of a measured
six-iteration run. A rule stated at the moment of the act gets followed. So the line goes down
here, where the numbers exist, in whatever form is legible:

    attribution iter=3 sub_agent=1 kind=in-depth raw=38 pooled=38 unique=9 unique_kept=7 shared=29 roles=2,6,9 state=reported
    attribution iter=3 sub_agent=2 kind=in-depth raw=42 pooled=42 unique=13 unique_kept=11 shared=29 roles=1,5,8,11 state=reported
    attribution iter=3 sub_agent=3 kind=gh-style raw=2 pooled=0 unique=0 unique_kept=0 shared=0 roles=n/a state=below-threshold

Every field above appears on every line, so the example is not a shorter form of the list. A line
that drops one is the omission this section exists to prevent, and a reader copying the example is
the likeliest way it happens.

`unique_kept` and `unique_actionable` are not known yet. The threshold has run so `unique_kept` can
go on that line, but whether a commit fixed one is not known until Step 2 has. So carry the
`unique_kept` findings forward by title, and let sub-step 7 append the actionable mark as each
commit lands. [SUMMARY.md](SUMMARY.md)'s table is a rollup of these lines rather than the place they
are first written, the same way the commit table is a rollup of the classes sub-step 7 appends.

**Read `unique_actionable` as the counterfactual.** For each instance it is what a run without that
instance would have lost. Recording it for every instance gives every counterfactual from a single
run, so two or three runs settle whether the second in-depth pass earns its cost, rather than
another argument from finding counts.

**Do not read `unique` as value on its own.** An instance can be unique because it alone was right
or because it alone was wrong, and the threshold plus Step 2's dismissals are what separate those.
That is why the ledger carries `unique_kept` and `unique_actionable` beside it rather than stopping
at `unique`.

**An instance that failed contributed nothing, and that is a measurement.** Record its row with
zeros and its state rather than omitting it. A missing row reads as an instance that was never
launched, and the difference between "ran and found nothing new" and "died" is most of what the
ledger is for.

## Aggregating tickets_examined

(in-depth-review only). Union the `tickets_examined` arrays from the active in-depth-review
sub-agents (gh-style-review has none; there are none this iteration if in-depth-review was not
active or role #10 was not in `<ACTIVE_ROLES>`). Union by `id`; for each `id`, `status` is `gaps`
if any instance reported gaps, else `unread` if any reported unread, else `ok`. The `gaps` count
is the number of surviving ticket findings for that `id` in the merged pool after the >=50 filter.
Also collect each instance's `ticket_review.status`: if any returned `denied` or `unavailable`,
record it for the Final Report so the user knows ticket review did not fully run.
