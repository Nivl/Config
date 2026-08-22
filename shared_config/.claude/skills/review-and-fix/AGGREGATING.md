# Aggregating reviewer results

## Contents
- Collecting sub-agent results (the async protocol, and reviewer-kind bookkeeping)
- A missing reviewer is not a clean reviewer (retry / unavailable rules)
- Excluding unscored and unverified findings
- Pooling, deduplication, and merge (the `>=50` filter)
- Aggregating `tickets_examined`

## Collecting sub-agent results

**First, account for every sub-agent this iteration launched.** Classify each as *reported* or
*missing* (nothing returned, errored, unparseable output that is not the no-fanout sentinel, or
never notified before the give-up bound). Then roll the per-instance verdict up
to the reviewer kind, since that is what the bookkeeping below is keyed on. Record every kind that
fell short in `reviewers_missing`, keep the per-instance detail for the per-iteration summary, and
union the `roles_missing` arrays the in-depth-review instances report. Never reason that running several
in-depth instances means every lens ran at least once. Measured: in one run two roles were silent in BOTH
instances, so the union of what the instances DID return covered neither. A lens that no instance reported on
is a hole, not a covered lens, and the union of findings can never be used to claim `complete`.
Results arrive in each sub-agent's own final text, on a later turn than the launch, never in the
Agent tool's launch result. Record every sub-agent's `agentId` at launch, then take turns and
harvest each `<task-notification>`, matching its `<task-id>` to a recorded `agentId`. Keep taking
turns until every sub-agent this iteration launched is accounted for, OR until THREE CONSECUTIVE
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

**Unavailability is keyed by reviewer KIND, not by instance.** Two kinds exist, the
`in-depth-review` unit and the `gh-style-review` unit. A kind **fell short** in an iteration when
fewer of its instances reported than were launched. A shortfall is retried by relaunching the kind
at FULL multiplicity, once per run. On a second shortfall the kind is marked `unavailable` for the
rest of the run. `unavailable` affects only future launches. It never discards a report already
received. `reviewers_missing`, `reviewer_unavailable`, and `reviewer_retries` are all keyed by kind.

**Why kind and not instance.** A deterministic `skipped_reason` about the invocation is derived
from the invocation arguments, and both in-depth instances receive identical arguments, so both
refuse identically. The no-fanout `skipped_reason` is about the context instead, and it aborts
rather than marking anything `unavailable`. Deterministic unavailability is inherently per-kind.
The 2x in-depth multiplicity exists for
triangulation, not to supply two different lenses, so a shortfall is a coverage question about the
kind rather than the loss of a distinct lens. Keying by kind also keeps `<ACTIVE_ROLES>` plus
`<ACTIVE_GH_STYLE>` sufficient to express every launch decision. Keying by instance would need a
new launch variable and an exception to the multiplicity rule in the Constraints. The cost is that
one flaky instance triggers a full 2x relaunch of its kind. That is mildly wasteful and it is the
accepted tradeoff, and the path is rare. Do not add reduced-multiplicity support.

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

## Excluding unscored and unverified findings

**Check `scoring.complete` on every in-depth-review result.** An instance reporting `false` did not
run its two-stage confidence filter, so its numbers are self-assessments by the same model that
proposed the findings, not scores.

- **Do not run them through the >=50 filter, and do not fix on them.** Report them in the
  per-iteration summary as unfiltered leads, name the instance, and leave them unfixed.
- Findings carrying `unscored: true`, and any finding with `citation_verified: false`, are treated
  the same way. **This applies to any such finding from any instance**, not only to instances whose
  `scoring.complete` came back `false`. Exclude them per instance, BEFORE the cross-instance pool
  and merge below. Excluding first is what keeps this skill's merge safe without needing a
  downward-merge rule for the field. `pr-review` merges first and so needs one.
- **Fixing on an unscored finding is how a fabricated finding becomes a commit.** This skill edits
  and commits code, so the cost of acting on a self-graded finding is a commit that encodes an
  invented problem. When in doubt, leave it and report it.

## Pooling, deduplication, and merge

Once collection has ended, whether every active sub-agent reported or the give-up bound above was
reached (up to 3 active; fewer when the iteration is pruned):

1. **Pool every finding** from the active result sets into one flat pool. Each finding carries
   its raw `confidence` (0–100), `file`, `line_range`, `category`, originating `sub_agent`,
   and `source` (`"in-depth-review"` or `"gh-style-review"`). Don't pre-segregate
   by source. Cross-prompt triangulation is the point.

2. **Cross-instance dedup.** Two findings are duplicates if they refer to the **same file**
   and have **overlapping line ranges** AND describe substantially the same problem
   (paraphrases count). Cross-source duplicates (one from in-depth, one from gh-style)
   count as duplicates, so merge them.

3. **For each duplicate group, produce one merged finding:**
   - `confidence`: **max** of the group's scores.
   - `cross_instance_agreement`: count of distinct active instances that raised this finding.
   - `sources`: set of distinct source skills (one of `{in-depth-review}`, `{gh-style-review}`,
     or both). Used as a tiebreaker. A both-source finding is stronger signal.
   - `title`, `description`, `suggested_fix`: pick the clearest from the group; if suggested
     fixes differ meaningfully, mention the alternatives in the description.
   - `category`: union of categories.
   - `ticket_id`: preserved from `ticket`-category findings (the Jira ID the gap traces to);
     `null` for all other findings. Never merge two findings with different `ticket_id`s.

4. **Apply the orchestrator's confidence threshold: discard everything with `confidence < 50`.**
   This is the review-and-fix-specific threshold, lower than each sub-skill's default of 70
   because cross-instance triangulation across the active passes raises our confidence in
   50–69 findings.

5. **If the post-filter list is empty** (every finding scored < 50), mark the **findings**
   batch clean. The iteration is **fully clean** only if Step 1.5's Discussion Context
   aggregation is also empty (PR mode). See Step 1.5 in SKILL.md.

6. **Otherwise** proceed to Step 2 with the filtered + deduplicated list, ordered by:
   1. Severity descending (critical -> major -> minor -> suggestion)
   2. `cross_instance_agreement` descending (more instances agreeing wins when severity ties)
   3. Both-sources first (a both-source finding beats a same-confidence single-source one)
   4. `confidence` descending

## Aggregating tickets_examined

(in-depth-review only). Union the `tickets_examined` arrays from the active in-depth-review
sub-agents (gh-style-review has none; there are none this iteration if in-depth-review was not
active or role #10 was not in `<ACTIVE_ROLES>`). Union by `id`; for each `id`, `status` is `gaps`
if any instance reported gaps, else `unread` if any reported unread, else `ok`. The `gaps` count
is the number of surviving ticket findings for that `id` in the merged pool after the >=50 filter.
Also collect each instance's `ticket_review.status`: if any returned `denied` or `unavailable`,
record it for the Final Report so the user knows ticket review did not fully run.
