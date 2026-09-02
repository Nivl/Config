# Collecting scorer results

Used by Step 2 to collect the batched scoring agents. Wherever this file says "agent," read it as a
launched scorer. Wherever it says "missing bookkeeping," read it as marking every finding in that
scorer's batch that came back without a score `unscored: true` with `confidence: null`.

The reviewer roles are not collected this way any more. They run inside the `review-roles`
workflow, whose `parallel()` is a barrier in code that resolves every role to output or `null`
before the call returns, and nothing in this file applies to them. Scorers are still Agent-tool
spawns from this skill's own context, and the protocol below is how their results arrive.

This works for scorers because the skill runs standalone, in the session root, which is where an
agent's completion notification is delivered. It stopped working for roles the moment the skill was
nested inside another agent, because the notifications kept going to the root while the skill sat one
level down waiting for them. That is why the roles moved and the scorers did not.

**An agent's output reaches the parent through the text it returns, and nowhere else.** Every
agent prompt must instruct it to put its COMPLETE output in its FINAL TEXT OUTPUT.

**Agents send nothing anywhere.** Never instruct an agent to use `SendMessage`, agent teams, or a
shared file. An agent has no resolvable address for you. An agent TYPE is not an agent id, so an
attempt fails every time with `no reachable agent named <your agent type>`. One measured run
burned 256 such attempts and delivered nothing. The output goes in the returned text, and that is
the only channel.

**How that returned text actually reaches you.** The Agent tool here launches ASYNCHRONOUSLY. Read
this before deciding what to do after launching, because the intuitive reading is wrong:

- The Agent tool result gives you launch metadata and an `agentId`. It does NOT contain the
  agent's output. There is nothing to read at launch. Do not wait on a launch-time return value,
  and do not treat its absence as a failure.
- An agent's output arrives LATER, inside a `<task-notification>` block, usually several batched
  into one turn. That block's `<result>` body holds the agent's final text, and its `<task-id>` is
  byte-identical to the `agentId` you recorded at launch.
- **You receive notifications only while you keep taking turns.** Any tool call is a turn.
- **A parent that ends its turn is FINALIZED and stops receiving.** Its outstanding agents'
  results then surface in the session root, where you cannot see them. This is the one behaviour
  that loses agents. It is why "wait for the results" is not an instruction anyone can follow, and
  why a progress note is not a safe thing to emit. Emitting one ends your turn.

**The collection protocol.** Follow it exactly.

1. Record every `agentId` the launch returned.
2. Take a turn. Harvest every `<task-notification>` in front of you, match each `<task-id>` to a
   recorded `agentId`, and keep its `<result>` body.
3. If any recorded `agentId` is still unaccounted for, take another turn. If you have no
   productive work, re-read the diff for an agent you are still waiting on. An unproductive turn
   still collects.
4. Repeat until every recorded `agentId` is accounted for, OR until THREE CONSECUTIVE COLLECTING
   TURNS have brought zero new arrivals. A collecting turn is ONE substantive tool call that names
   the artifact it read, so re-read the diff, a changed file, or the finding being scored. Three
   repeats of the same no-op are not three turns.
   Do NOT start the zero-arrival counter until you have taken at least as many collecting turns as
   you launched agents, with a floor of five, whether or not anything has arrived. The three
   zero-arrival turns are counted FRESH from the moment the counter arms, so turns taken before
   arming never count toward them. Agents take minutes, not seconds, so a counter armed at launch
   measures your own polling speed rather than a failure. If you reach the bound and have not yet
   re-armed the counter, re-arm it EXACTLY ONCE and keep collecting. After that, honor the bound.
5. At that bound, once you have already used your single re-arm, record every still-unaccounted
   agent in the caller's missing bookkeeping, close the accounting, and continue. Do not wait
   longer. Do not treat a give-up as clean.

A notification that arrives AFTER the bound is still that agent's report. Fold it into the pool and
clear it from the missing bookkeeping. The accounting is final only at the moment the caller
emits its own final output, not at the moment the bound fires.

**Never end your turn while a recorded `agentId` is unaccounted for**, unless you are declaring it
in the missing bookkeeping on that same turn.

**Your final output is the report, never a status update.** If agents are still unaccounted for
when you hit the bound, emit the report with them marked missing. Never return a progress note
saying you are still waiting, because that ends your turn and finalizes you with less than you
could have collected.

`TaskOutput` appears in the deferred-tool listing and looks like exactly the right tool for this.
It is NOT available to a nested agent. `ToolSearch select:TaskOutput` returns no match from inside
an agent's parent, though the same query resolves at the session root. Do not build on it. The
protocol above is the mechanism.

**An agent that never reports has reported nothing**, whatever it may have produced elsewhere.
Classify it as missing per the caller's bookkeeping. Do not hunt for its output in another channel
and splice it in. That makes delivery depend on luck rather than on the contract.

This section is written this way because the shorter version failed in practice. A parent told
only to read a result that the launch never produced had nothing to read, stopped, and then
invented findings for three agents it never heard from.
