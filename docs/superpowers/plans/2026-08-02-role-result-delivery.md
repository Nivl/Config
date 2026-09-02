# Role Result Delivery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the unimplementable role-to-parent delivery mechanism in `in-depth-review` with a bounded active-collection protocol, so reviewer roles stop being silently lost and then fabricated.

**Architecture:** Three markdown skill files. Their prose IS the program, because an LLM orchestrator reads them top to bottom and follows them. The fix is a mechanism rewrite in one section of `in-depth-review`, the same protocol applied to its scoring fan-out, and three one-sentence integrity additions. No new tool, no new channel, no addressing step.

**Tech Stack:** Markdown. `rg` for verification. `git` for commits.

**Spec:** `docs/superpowers/specs/2026-08-02-role-result-delivery-design.md`

## Global Constraints

- Plain ASCII in authored prose. Do not join two independent clauses with an em-dash, an ASCII space-hyphen-space, or a clause-splitting colon. Split into sentences. Those forms stay allowed as label prefixes, definition dashes at the start of a list item, list introductions, ratios, ranges, CLI flags, and inside literal templates.
- Edit `shared_config/.claude/skills/...` in this repo. NEVER edit `~/.claude/skills/...`. That is a synced copy and the next `melvin-config claude sync` overwrites it.
- `shared_config/.claude/agents/pr-review-finder-indepth.md` gets NO CHANGE. It was read in full and contains no reply-address instruction.
- Use `rg -U` for every verification search. Line-break-straddling text has caused several escaped defects in these files. A pattern with a literal space between two words will NOT match across a line break, so use `\s+` between words that might wrap.
- Match sites on TEXT, never on line number. The spec's line numbers are pre-change and shift as tasks land.
- Repo rules: plain `git` from inside `/Users/melvin/.melvin/config`, never top-level `git -C`, never pipe or chain a git command, no inline scripting, no command substitution, no `$TMPDIR`, redirects last.

## There is no test framework

These are prose files. Each task's "failing test" is an `rg -U` run BEFORE editing that confirms the current text matches what this plan quotes. If it does not match, STOP and report drift rather than guessing at a merge. Each task's "passing test" is an `rg -U` after editing.

## File Structure

- Modify: `shared_config/.claude/skills/in-depth-review/SKILL.md` — the delivery mechanism, the scoring fan-out, and one integrity sentence
- Modify: `shared_config/.claude/skills/pr-review/SKILL.md` — one integrity sentence
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md` — one integrity sentence
- NO CHANGE: `shared_config/.claude/agents/pr-review-finder-indepth.md`

## Scope reductions from the spec, and why

The spec listed four integrity items. On inspection of the current files, two are already implemented and a third is mostly implemented. This plan does NOT redo them. Churning correct text invites new defects for no gain.

- **Spec item 5b, honest `scoring.complete` and `unscored` / `confidence: null`: ALREADY DONE.** `in-depth-review` lines 773 to 777 already require a finding with no scorer to carry `confidence: null` and `unscored`, and already require `scoring.complete: false` with every finding reported unscored. The JSON contract at lines 909 to 912 and the explanation at 941 to 944 already match. NO TASK.
- **Spec item 5a, fabrication: MOSTLY DONE.** Step 2.0 at lines 709 to 729 already carries "NEVER FABRICATE A MISSING ROLE'S OUTPUT", the four "do not" bullets, "A missing role is NOT a clean role", the partial-coverage rule, and "It is silent on security". Only the brief's second verbatim phrase is absent. Task 3 adds one sentence.
- **Spec item 5d, coverage: MOSTLY DONE.** Both callers already union `roles_missing` across instances and force `partial` when that union is non-empty, which is stricter than the spec asked for. What is genuinely absent is a prohibition on the specific fallacy the brief measured. Task 3 adds one sentence to each caller.
- **Spec item 5c, no progress note as the final answer:** folded into Task 1, because the bounded give-up is what makes finalizing possible. A rule to finalize without a mechanism to finalize on is unactionable.

---

### Task 1: Replace the return contract with the collection protocol

The core fix. The current section states a correct principle and an unimplementable mechanism, and separately permits a channel that cannot resolve. Both halves are replaced.

**Files:**
- Modify: `shared_config/.claude/skills/in-depth-review/SKILL.md` (the `### The role -> parent return contract` section, and the Step 1 launch paragraph above it)

**Interfaces:**
- Consumes: nothing.
- Produces: the collection protocol, which Task 2 applies to the scoring fan-out. Task 2 must state the protocol rather than cross-reference it, so a reader of either fan-out sees the whole rule.

- [ ] **Step 1: Confirm the current text of both sites**

Run:
```
rg -n -U 'Spawn the reviewer sub-agents in a single message|spawns and reads return values|additive only' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: three hits. The Step 1 launch sentence, and two inside the return-contract section. If any reads differently from what this plan quotes, STOP and report drift.

- [ ] **Step 2: Add the agentId-recording instruction to the Step 1 launch paragraph**

Replace:
```
Spawn the reviewer sub-agents in a single message (concurrent tool-use blocks). Launch
exactly the roles in `<ROLE_SET>` (Step 0). Sequential launches defeat the purpose of this
design. Never serialize.
```
With:
```
Spawn the reviewer sub-agents in a single message (concurrent tool-use blocks). Launch
exactly the roles in `<ROLE_SET>` (Step 0). Sequential launches defeat the purpose of this
design. Never serialize.

**Record the `agentId` of every role you launch.** That set is what you collect against, and it is
the accounting baseline for Step 2.0. The launch result does NOT contain a role's findings. Read
"The role -> parent return contract" below before you decide what to do after launching, because the
obvious next action is the one that loses roles.
```

- [ ] **Step 3: Replace the entire return-contract section**

Replace the whole section, from the `### The role -> parent return contract` heading through the paragraph ending `an accident it can hope for.`, with:

```
### The role -> parent return contract

**A role's findings reach the parent through the Agent tool's return value, and nowhere else.**
Every role prompt must end by instructing the role to put its COMPLETE findings list in its FINAL
TEXT OUTPUT.

**Roles send nothing anywhere.** Never instruct a role to use `SendMessage`, agent teams, or a shared
file. A role has no resolvable address for you. An agent TYPE is not an agent id, so an attempt fails
every time with `no reachable agent named <your agent type>`. One measured run burned 256 such
attempts and delivered nothing. The findings go in the returned text, and that is the only channel.

**How that returned text actually reaches you.** The Agent tool here launches ASYNCHRONOUSLY. Read
this before deciding what to do after launching, because the intuitive reading is wrong:

- The Agent tool result gives you launch metadata and an `agentId`. It does NOT contain the role's
  output. There is nothing to read at launch. Do not wait on a return value, and do not treat its
  absence as a failure.
- A role's output arrives LATER, inside a `<task-notification>` block, usually several batched into
  one turn. That block's `<result>` body holds the role's final text, and its `<task-id>` is
  byte-identical to the `agentId` you recorded at launch.
- **You receive notifications only while you keep taking turns.** Any tool call is a turn.
- **A parent that ends its turn is FINALIZED and stops receiving.** Its outstanding roles' results
  then surface in the session root, where you cannot see them. This is the one behaviour that loses
  roles. It is why "wait for the results" is not an instruction anyone can follow, and why a progress
  note is not a safe thing to emit. Emitting one ends your turn.

**The collection protocol.** Follow it exactly.

1. Record every `agentId` the launch returned.
2. Take a turn. Harvest every `<task-notification>` in front of you, match each `<task-id>` to a
   recorded `agentId`, and keep its `<result>` body.
3. If any recorded `agentId` is still unaccounted for, take another turn. If you have no productive
   work, re-read the diff for a role you are still waiting on. An unproductive turn still collects.
4. Repeat until every recorded `agentId` is accounted for, OR until THREE CONSECUTIVE turns have
   brought zero new arrivals.
5. At that bound, record every still-unaccounted role in `roles_missing` per Step 2.0 and finalize.
   Do not wait longer. Do not treat a give-up as clean.

**Never end your turn while a recorded `agentId` is unaccounted for**, unless you are declaring it in
`roles_missing` on that same turn.

**Your final output is the report, never a status update.** If roles are still unaccounted for when
you hit the bound, emit the report with them in `roles_missing`. Never return a progress note saying
you are still waiting, because that ends your turn and finalizes you with less than you could have
collected.

`TaskOutput` appears in the deferred-tool listing and looks like exactly the right tool for this. It
is NOT available to a nested agent. `ToolSearch select:TaskOutput` returns no match from inside a
role parent, though the same query resolves at the session root. Do not build on it. The protocol
above is the mechanism.

**A role that never reports has reported nothing**, whatever it may have produced elsewhere.
Classify it as missing per Step 2.0. Do not hunt for its output in another channel and splice it in.
That makes delivery depend on luck rather than on the contract.

This section is written this way because the shorter version failed in practice. A parent told only
to read a result that the launch never produced had nothing to read, stopped, and then invented
findings for three roles it never heard from.
```

Note the closing sentence deliberately PARAPHRASES the old instruction instead of quoting it. Quoting
it verbatim would leave the banned phrase in the file, and Step 4's search for that phrase is how a
future editor checks the bad mechanism is gone. A documented past failure must not create a permanent
false positive in the check that guards against it.

- [ ] **Step 4: Verify**

Run:
```
rg -n -U 'spawns and reads return values|additive only' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: zero hits.

Run:
```
rg -n -U 'TaskOutput' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: exactly the hits inside the new warning paragraph, and nothing that recommends using it.

Run:
```
rg -n -U 'Record every `agentId`|Record the `agentId`|THREE CONSECUTIVE turns' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: three hits, one per phrase.

Then READ the new section end to end and confirm two things by eye, because no grep can check them. No sentence tells the parent to read a result at launch. No sentence permits a role to send results anywhere.

- [ ] **Step 5: Check line lengths**

Run:
```
rg -n '^.{110,}$' shared_config/.claude/skills/in-depth-review/SKILL.md
```
The file's prose convention is roughly 88 to 99 characters, and it has a pre-existing tail of markdown table rows and template bullets over 110. Confirm no line YOU added appears in the output. Re-wrap any that do, changing zero words.

- [ ] **Step 6: Commit**

```
git add shared_config/.claude/skills/in-depth-review/SKILL.md
git commit -F /private/tmp/claude/rrd-t1.txt
```
Message body to write to that file first:
```
in-depth-review: replace the role delivery mechanism with active collection

The return contract stated a correct principle and an unimplementable
mechanism. "The parent spawns and reads return values" describes a
synchronous Agent tool. This harness launches asynchronously, so there is
no return value at launch. A parent following that instruction has nothing
to read and no next action, so it stops. A nested agent that stops is
finalized and stops receiving, and its outstanding roles' results surface
in the session root where the parent cannot see them.

Measured: one parent collected about 6 notifications against 24 launches,
waited roughly 19 minutes, then invented findings for three roles it never
heard from. Two probes established the real behaviour. Notifications DO
reach a nested parent, reliably and batched, and a task-id is byte-identical
to the agentId. Six of six children were delivered with zero loss when the
parent kept taking turns.

So the fix is not a new channel. It is an explicit protocol: record every
agentId, keep taking turns, harvest notifications as they batch in, and give
up after three consecutive quiet turns by recording the rest in
roles_missing. Never end a turn with an unaccounted id unless declaring it
missing, and never emit a progress note, because that ends the turn too.

Also removes the "additive only" SendMessage allowance. It invited roles to
attempt a reply address that cannot resolve, since an agent type is not an
agent id. One run burned 256 failed attempts on it.

Also warns that TaskOutput, which looks like the right tool and is listed as
deferred, does not resolve from a nested agent.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
```

---

### Task 2: Give the scoring fan-out the same protocol

The scorer launch has identical topology and identical exposure. The spec warns that fixing one fan-out and not the other reproduces the bug in the half that was missed.

**Files:**
- Modify: `shared_config/.claude/skills/in-depth-review/SKILL.md` (the scoring launch step, item 3 of the filter-and-dedup sequence)

**Interfaces:**
- Consumes: Task 1's collection protocol. State it here rather than cross-referencing, so a reader of this step alone still gets the whole rule.
- Produces: nothing.

- [ ] **Step 1: Confirm the current text**

Run:
```
rg -n -U 'Launch a scoring sub-agent for each unique finding in parallel' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: one hit. If it reads differently from the quote below, STOP and report drift.

- [ ] **Step 2: Add the protocol to the scoring launch**

Find the passage beginning:
```
3. **Launch a scoring sub-agent for each unique finding in parallel** (one sub-agent per
   finding, all in a single message). **Spawn each scorer on Haiku** (Agent-tool
   `model: haiku`).
```
Immediately after the sentence ending `` `model: haiku`). ``, insert this on a new line, indented to match the surrounding list item at three spaces:
```
   **Collect the scorers the same way you collected the roles.** Record every scorer's `agentId`
   at launch. The launch result carries no score. Take turns and harvest each
   `<task-notification>`, matching its `<task-id>` to a recorded `agentId`. Keep taking turns
   until every scorer is accounted for, or until three consecutive turns bring zero new arrivals.
   A scorer you never hear from does NOT get a self-assigned number. Its finding carries
   `unscored: true` and `confidence: null`, exactly as it would if no scorer had been spawned. If
   you end your turn with scorers outstanding you are finalized and they are lost, so do not stop
   to report progress.
```

- [ ] **Step 3: Verify both fan-outs state the rule**

Run:
```
rg -n -U 'THREE CONSECUTIVE turns|three consecutive turns' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: two hits, one in the role protocol and one in the scoring step. If only one, the second fan-out was missed, which is the specific failure this task exists to prevent.

Run:
```
rg -n -U 'unscored: true` and `confidence: null|confidence: null' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: at least the pre-existing hits plus the new one. Confirm the new sentence does not contradict the existing rule at the "A finding with no scorer is `unscored`, not confident" bullet.

- [ ] **Step 4: Check line lengths**

Run:
```
rg -n '^.{110,}$' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Confirm no line you added appears. Re-wrap any that do, changing zero words.

- [ ] **Step 5: Commit**

```
git add shared_config/.claude/skills/in-depth-review/SKILL.md
git commit -F /private/tmp/claude/rrd-t2.txt
```
Message body:
```
in-depth-review: collect the scorer fan-out the same way as the roles

The scoring step launches one Haiku scorer per finding in a single message,
which is the same topology and the same exposure as the role fan-out. A
parent that stops while scorers are outstanding loses them and then has an
unscored finding with no scorer to blame, which is how a self-assigned
confidence gets emitted as if a scorer produced it.

States the protocol here rather than cross-referencing the role section, so
a reader of this step alone still gets the whole rule. Fixing one fan-out
and not the other would reproduce the bug in the half that was missed.
```

---

### Task 3: Three one-sentence integrity additions

Everything else in the spec's integrity list is already implemented. These three sentences are what is genuinely absent.

**Files:**
- Modify: `shared_config/.claude/skills/in-depth-review/SKILL.md` (Step 2.0)
- Modify: `shared_config/.claude/skills/pr-review/SKILL.md` (the roles_missing union)
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md` (the roles_missing union)

**Interfaces:**
- Consumes: nothing.
- Produces: nothing.

- [ ] **Step 1: Confirm all three sites**

Run:
```
rg -n -U 'Report the partial result anyway' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: one hit.

Run:
```
rg -n -U 'union the `roles_missing` arrays' shared_config/.claude/skills/pr-review/SKILL.md shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: one hit in each file.

- [ ] **Step 2: Add the honest-roles sentence to in-depth-review Step 2.0**

Replace:
```
- Report the partial result anyway. A review missing one role is still useful; a review that
  silently claims completeness it does not have is worse than no review.
```
With:
```
- Report the partial result anyway. A review missing one role is still useful. A review that
  silently claims completeness it does not have is worse than no review. Three honest roles beat
  twelve invented ones.
```
Note this also splits a semicolon that was joining two independent clauses, per the prose rule.

- [ ] **Step 3: Add the multiplicity-fallacy prohibition to pr-review**

Find the sentence containing `union the `roles_missing` arrays` and append this immediately after the sentence it belongs to, as a new sentence in the same paragraph:
```
Never reason that running several in-depth instances means every lens ran at least once. Measured:
in one run two roles were silent in BOTH instances, so the union of what the instances DID return
covered neither. A lens that no instance reported on is a hole, not a covered lens, and the union
of findings can never be used to claim `complete`.
```

- [ ] **Step 4: Add the same prohibition to review-and-fix**

Find the sentence containing `union the `roles_missing` arrays` and append the identical text as a new sentence in the same paragraph:
```
Never reason that running several in-depth instances means every lens ran at least once. Measured:
in one run two roles were silent in BOTH instances, so the union of what the instances DID return
covered neither. A lens that no instance reported on is a hole, not a covered lens, and the union
of findings can never be used to claim `complete`.
```
The text is repeated verbatim rather than cross-referenced because each skill is read on its own by a different orchestrator.

- [ ] **Step 5: Verify**

Every space in these patterns is `\s+`, because the mandated text is wrapped and a literal space can
never match across a line break. A plain-space version of the first pattern returned zero hits against
text that was present and correct, which would have been read as a missing sentence.

Run:
```
rg -n -U 'Three\s+honest\s+roles\s+beat\s+twelve\s+invented\s+ones' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: one hit. It spans two lines, breaking between `beat` and `twelve`.

Run:
```
rg -n -U 'every\s+lens\s+ran\s+at\s+least\s+once' shared_config/.claude/skills/pr-review/SKILL.md shared_config/.claude/skills/review-and-fix/SKILL.md
```
Expected: one hit in each file.

Run:
```
rg -n '^.{110,}$' shared_config/.claude/skills/in-depth-review/SKILL.md shared_config/.claude/skills/pr-review/SKILL.md shared_config/.claude/skills/review-and-fix/SKILL.md
```
Confirm no line you added appears. Re-wrap any that do, changing zero words.

- [ ] **Step 6: Commit**

```
git add shared_config/.claude/skills/in-depth-review/SKILL.md shared_config/.claude/skills/pr-review/SKILL.md shared_config/.claude/skills/review-and-fix/SKILL.md
git commit -F /private/tmp/claude/rrd-t3.txt
```
Message body:
```
reviewers: forbid the two coverage fallacies that survived

Two sentences the integrity rules were missing.

in-depth-review's Step 2.0 already forbids fabricating a missing role and
already says a missing role is not a clean role. It now also states that
three honest roles beat twelve invented ones, which is the framing parents
were measured to self-catch and retract against.

Both orchestrators already union roles_missing across instances and force
partial when that union is non-empty. Neither forbade the specific fallacy
that running two instances means every lens ran at least once. Measured: in
one run roles 6 and 10 were silent in BOTH passes, so the union of what the
instances returned covered neither.
```

---

## Final verification, after all three tasks

- [ ] **Step 1: The mechanism is gone and the protocol is present**

Spaces are `\s+` here for the same reason as Step 5 above, and the first check is the one where it
matters most. It expects ZERO hits, so a literal-space pattern cannot fail correctly: it returns zero
whether the banned phrase is absent or merely wrapped across a line. A wrap-blind zero-hit check is a
guaranteed pass, not a verification.

```
rg -n -U 'spawns\s+and\s+reads\s+return\s+values|additive\s+only' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: zero hits.

```
rg -n -U -i 'three\s+consecutive\s+turns' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: two hits, one per fan-out. `-i` replaces the case alternation, so a copy written in a third
casing still counts instead of silently dropping out of the total.

- [ ] **Step 2: No sentence invites passive waiting**

Read the return-contract section and the scoring launch step end to end. Confirm neither tells the parent to read a result at launch, to wait for one, or to report progress instead of finalizing. A grep cannot check this. The original defect contained no keyword that would have flagged it.

- [ ] **Step 3: Frontmatter still valid**

```
rg -n '^---$' shared_config/.claude/skills/in-depth-review/SKILL.md
```
Expected: the opening and closing fences. Read lines 1 to the closing fence and confirm every continuation line under `description: >` sits at exactly 2 spaces. A 1 or 3 space line silently converts the folded scalar to a literal block and breaks skill selection.

- [ ] **Step 4: The agent definition is untouched**

```
git log --oneline -3 --name-only
```
Expected: exactly the three commits from this plan, and `shared_config/.claude/agents/pr-review-finder-indepth.md` does NOT appear among their files. It was verified clean and needs no change. If a fix round added a commit, widen `-3` to cover this plan's commits only, and never past the plan's base.

## Self-review of this plan

**Spec coverage.** Spec sections 1, 2 and 3 are Task 1. Section 4 is Task 2. Section 5a's missing sentence and 5d's missing sentences are Task 3. Section 5c is folded into Task 1, because the bounded give-up is what makes finalizing actionable. Section 5b has NO task, because it is already fully implemented, and that reduction is stated in "Scope reductions from the spec" with the line numbers that prove it.

**Placeholder scan.** Every step carries the exact text to write, delete, or run. No "TBD", no "add appropriate handling", no "similar to Task N". Task 3 repeats its caller sentence verbatim in two files rather than cross-referencing, which is deliberate and stated.

**Consistency.** Task 2 states the protocol rather than cross-referencing Task 1, so the phrase "three consecutive turns" appears twice by design, and the final verification asserts exactly two hits. If a future edit collapses them into a cross-reference, that check fails loudly, which is the intent.

**Known omission.** The spec's definition of done includes a live `in-depth-review` run. That is not a task here because it is a verification activity rather than an edit, and it needs a real commit range to review. It belongs to whoever runs the plan's final review, and the spec already states the pass and fail conditions, including that a non-empty `roles_missing` passes when the bounded give-up fired and fails when the parent stopped without one.
