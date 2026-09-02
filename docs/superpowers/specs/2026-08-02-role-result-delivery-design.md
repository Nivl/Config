# Fix role result delivery in in-depth-review

**Date:** 2026-08-02
**Primary file:** `shared_config/.claude/skills/in-depth-review/SKILL.md`
**Callers touched:** `shared_config/.claude/skills/pr-review/SKILL.md`, `shared_config/.claude/skills/review-and-fix/SKILL.md`

## Goal

Stop `in-depth-review` reviewer roles from being silently lost and then fabricated by the parent.
Replace a delivery mechanism that cannot work in this harness with one that is measured to work, and
close the four integrity gaps that let a lost role surface as a clean one.

## Where the source lives

Edit `shared_config/.claude/skills/...` in this repo. `~/.claude/skills/...` is a synced copy and
the next `melvin-config claude sync` overwrites it. The originating brief named the synced path as
primary, which is wrong. The two are currently byte-identical, so the brief's diagnosis was made
against current text.

## Root cause, measured

Two throwaway probes were run before designing. Topology in both matches production: session root
spawns a nested parent, the nested parent spawns children, so a child is a grandchild of the root.

**Probe 1, one child.**
- The Agent tool result contains launch metadata and an agentId. It does NOT contain the child's
  output.
- A `<task-notification>` carrying `<result>BANANA</result>` DID arrive at the nested parent, on the
  turn after launch.
- `ToolSearch select:TaskOutput` from the nested agent returned "No matching deferred tools found".
  The same query resolves fine at session root.
- The notification's `<task-id>` was byte-identical to the child's agentId.

**Probe 2, six children.**
- 6 of 6 delivered. Zero loss. No cross-contamination between task-ids.
- All six arrived batched into the single next turn after launch.
- The probe's own prompt-authoring bug garbled five children's content. It reported that against
  itself and correctly separated it from delivery integrity.

### The actual mechanism

The parent ends its turn while children are outstanding, and a nested agent that stops is finalized.
It stops receiving. Everything else observed follows from that one fact:

- "~6 notifications against 24 launches" is what the parent collected before it stopped.
- "Three of its roles' results arrived in the top-level session instead" is where a finalized
  parent's pending notifications surface. The brief reads this as nesting misrouting the
  notification. The causality is inverted. Notifications reach a nested parent correctly, as probe 1
  and probe 2 both show, and only surface upward once the parent is gone.
- The ~19 minute wait is a parent with no scripted next action.
- Probe 2's clean 6 of 6 is what happens when the parent is ordered to keep taking turns.

### Both failures trace to one section of the current text

`SKILL.md:175-196`, the "role -> parent return contract", added earlier in the same working session
as the bug reports. Its principle is right and is kept. Two sentences are the defect.

1. `:187-188` "**Do not have roles address the parent, and do not have the parent wait on a push.**
   The parent spawns and reads return values. There is no addressing step to get wrong."
   This describes a synchronous Agent tool. This harness launches async, so there is no return value
   at launch. A parent following it literally has nothing to read and no next action, so it stops,
   which finalizes it. A progress note is also stopping.

2. `:183-186` "**Never depend on a message arriving.** If a role also sends results via
   `SendMessage`, agent teams, a shared file, or any other side channel, that is additive only."
   This PERMITS SendMessage. A role that tries it needs an address and the only identifier in scope
   is the parent's agent type, producing `no reachable agent named pr-review-finder-indepth`.
   Measured 256 occurrences in one parent transcript.

`shared_config/.claude/agents/pr-review-finder-indepth.md` needs NO change. It was read in full and
contains no reply-address instruction. The brief lists it as a file to fix; that is incorrect.

## What the brief got right and wrong

Right: the async launch, the absence of a readable return value, zero active collection, the
fabrication, the skipped scorer stage, the progress-note-as-report, the `{6, 10}` both-passes loss,
and that multiplicity does not guarantee every lens ran.

Wrong: that notifications misroute because of nesting. They do not. And its preferred fix,
`TaskOutput` polling, is not merely deprecated but uncallable from a nested agent, so option 1 is
dead rather than needing confirmation.

Its option 3 (parent passes its own agentId, roles SendMessage it) measured 11/12 and 12/12 against
3/12 without. That result is real and is explained by this design's mechanism: every inbound message
forces the parent to take a turn. Option 3 works by accidentally causing turn-taking. This design
causes it directly, without reintroducing a push channel.

## Design

### 1. Replace the mechanism, keep the principle

Rewrite `SKILL.md:175-196` so the section keeps "a role's findings must be in its final text output"
and replaces the mechanism with an explicit collection protocol:

- Launch all roles in ONE message and RECORD EVERY agentId from the launch results. The recorded set
  is the accounting baseline for Step 2.0.
- The Agent tool result carries launch metadata only. There is no result to read at launch. Do not
  try to read one, and do not treat its absence as a failure.
- Results arrive as `<task-notification>` blocks on LATER turns, usually batched. Harvest each
  block's `<result>` body and match it to its `<task-id>`, which is byte-identical to the agentId you
  recorded.
- KEEP TAKING TURNS until every recorded agentId is accounted for. Any tool call is a turn. If you
  have no productive work, re-read the diff for a role you are still waiting on.
- NEVER end your turn while a recorded agentId is unaccounted for, unless you are declaring it in
  `roles_missing` on that same turn. A nested agent that stops is FINALIZED and stops receiving. Its
  pending notifications surface in the session root, where you cannot see them. This is the single
  behaviour that loses roles.
- Bounded give-up: after THREE consecutive turns with zero new arrivals, declare every still
  unaccounted role in `roles_missing` and finalize. Do not wait indefinitely, and do not treat a
  give-up as clean.

### 2. Forbid SendMessage outright

Delete the "additive only" allowance. Replace with a prohibition: roles have no resolvable address
for the parent, an attempt fails 100% of the time with `no reachable agent named <type>`, and the
attempt wastes a role's turn. Roles put findings in returned text and send nothing anywhere.

### 3. Warn about TaskOutput by name

State that `TaskOutput` appears in the deferred-tool listing but does not resolve from a nested
agent, and that the collection protocol above is the mechanism. Without this a future author reads
the tool list and reaches for it, which is exactly what the originating brief did.

### 4. Same protocol for the scoring fan-out

`SKILL.md:751-755` launches one Haiku scorer per finding in a single message. Identical topology,
identical exposure. The scoring step gets the same record-every-id, keep-taking-turns, bounded
give-up protocol, and an unaccounted scorer yields `unscored: true` / `confidence: null` for its
finding rather than a self-assigned score.

### 5. Integrity fixes

a. **Fabrication.** Keep the existing top-level `roles_missing`. Add verbatim: a missing lens is
   SILENT, not clean, and three honest roles beat twelve invented ones. Never emit a finding for a
   role that did not report. Do not reconstruct what a missing role would have found, do not run its
   lens yourself and attribute it to that role, and do not carry a result forward from another pass.

b. **Skipped scorer stage.** `scoring.complete` must be honest. When the scoring stage did not run
   for a finding, that finding carries `unscored: true` and `confidence: null`. A parent must never
   self-assign a confidence number in place of a scorer.

c. **Progress note as final answer.** The parent's final output is the report, never a status
   update. If findings remain unscored at give-up, finalize with them marked unscored rather than
   waiting for another scorer. Waiting is what strands the parent.

d. **Coverage.** In the callers. A role silent in BOTH in-depth instances is at best one unscored
   observation and never a covered lens. Forbid claiming `complete` from the union of two instances'
   roles. Measured: roles 6 and 10 were silent in both passes of one run, so the assumption that 2x
   multiplicity guarantees every lens ran at least once is false.

## Files

- `shared_config/.claude/skills/in-depth-review/SKILL.md` — sections 1, 2, 3, 4, and 5a/5b/5c
- `shared_config/.claude/skills/pr-review/SKILL.md` — section 5d
- `shared_config/.claude/skills/review-and-fix/SKILL.md` — section 5d
- `shared_config/.claude/agents/pr-review-finder-indepth.md` — NO CHANGE, verified clean

## Definition of done

The brief asks for "nonzero TaskOutput calls" as evidence of active collection. That is now
impossible, so it is replaced by evidence appropriate to this mechanism:

1. A real `in-depth-review` run over a small commit range returns `roles_missing: []` and
   `coverage: complete` from both in-depth instances.

   **If `roles_missing` is non-empty, that alone does not fail this criterion.** The fix guarantees
   honest accounting, not a lossless channel, and the channel was only measured to six children wide.
   A non-empty `roles_missing` PASSES if the parent took at least three turns with zero new arrivals
   before declaring it, which is the bounded give-up firing as designed. It FAILS if the parent
   stopped while ids were still unaccounted for and no give-up occurred, because that is the original
   bug. The distinguishing evidence is criterion 2.

2. The parent takes MULTIPLE turns between launching roles and emitting its report, demonstrating
   active collection rather than a single passive wait.
3. No parent emits a finding for a role that did not report.
4. No parent returns a progress note as its final answer.
5. `rg -c 'SendMessage'` over a reviewer transcript shows roles are no longer attempting an
   unresolvable reply address. Use `rg -c` for a COUNT only. Never `Read` or tail these files. They
   are full JSONL conversation transcripts and will overflow the reader's context.

## Verification

- `rg -n -U 'spawns and reads return values|additive only'` over the primary file returns zero hits.
- `rg -n -U 'TaskOutput'` over the primary file returns exactly the warning added by section 3.
- Read the rewritten section end to end and confirm no sentence tells the parent to read a result at
  launch, and no sentence permits a role to send results anywhere.
- Confirm the scoring fan-out and the role fan-out state the same protocol rather than diverging,
  since a reader who fixes one and not the other reproduces the bug in the half that was missed.

## Out of scope

- Flattening the fan-out so roles are spawned by the session root. The probes show nesting is not the
  problem, so this now solves a non-problem, and it would break `in-depth-review`'s composability
  since both orchestrators invoke it as a sub-agent.
- Changing `pr-review` or `review-and-fix` launch mechanics. Both run at session root, where
  notifications already arrive, and neither showed this failure.
- Any change to `pr-review-finder-indepth.md`.
- Probing at the full 24-launch width. The design accounts per-id and gives up on a bound, so it does
  not depend on the channel being lossless at any scale. Loss at higher width would show up as
  `roles_missing` entries rather than as silent fabrication, which is the outcome this spec exists to
  guarantee.
