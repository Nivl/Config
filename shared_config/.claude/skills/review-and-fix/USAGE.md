# Token usage per agent

Shared by `review-and-fix` (one block per iteration) and `pr-review` (one block per run). Every
agent the review spawns leaves a transcript, and every assistant turn in that transcript carries a
`usage` object. Summing it per agent is the whole method. Nothing is estimated except the dollar
figure, and that is labelled.

## Where the data is

Under the session directory, which is
`~/.claude/projects/<project-hash>/<session-id>/`. The project hash is the working directory with
`/` replaced by `-`, and the session id is the directory name in the scratchpad path the harness
hands every session.

- Agent-tool spawns, meaning gh-style, the scorer, and the approach pair:
  `subagents/agent-<agentId>.jsonl`. The orchestrator recorded each `agentId` at launch.
- Workflow agents, meaning every role the `review-roles` workflow ran:
  `subagents/workflows/wf_*/agent-<agentId>.jsonl`. The orchestrator never sees these ids, which is
  why the join goes through the stamp below rather than through an id list.

## The stamp

The harness stores no label for an agent. `agent-<id>.meta.json` holds only the agent type, and the
workflow journal holds ids and cache keys. The transcript's first record is the prompt verbatim, so
the prompt is where identity has to live. Every prompt this skill or its workflow sends begins with
one line of the form:

```
<!-- review-roles inst=1 role=7 attempt=1 tag=iter3 target=15892 -->
<!-- gh-style tag=iter3 target=15892 -->
<!-- review-scorer batch=1 tag=iter3 target=15892 -->
<!-- approach-proposer round=1 tag=pr15892 -->
<!-- nuanced-judge round=1 tag=pr15892 -->
```

`tag` is what tells this iteration's agents apart from the last iteration's on the same target.
`review-and-fix` passes `iter<N>`. `pr-review` passes `pr<N>`. `attempt=2` on a role is the retry
the workflow launched after `attempt=1` returned nothing, so both costs show and the retry's price is
visible rather than folded into the role's.

## The recipe

The filter lives in [usage.jq](usage.jq) beside this file, so it is versioned and tested rather
than pasted. Run it once, after the fan-out has returned and before the per-iteration summary:

```
jq -r -n -f ~/.claude/skills/review-and-fix/usage.jq \
  <session>/subagents/agent-*.jsonl \
  <session>/subagents/workflows/*/agent-*.jsonl
```

It groups every record by `input_filename`, so one call over every transcript yields one line per
agent. `jq` is allow-listed, `-f` reads a filter rather than executing a script, and a shell loop is
not needed, which matters because the shell-wrapper hook denies one.

The filter reads each transcript's first record for the stamp, sums the `usage` object over every
assistant turn, and prices the sum from the table below. An unstamped transcript still gets a line,
with `kind=unstamped`, so an agent this skill did not launch is visible rather than silently absent.
Filter by the `tag=` you passed to keep to this iteration's agents.

Verified on the session `0304256d` run before this file was written. 30 transcripts in, 30 lines
out, and the summed estimate matched an independent hand calculation to the cent.

## The line to append

One per agent, appended to the run log the moment the recipe runs, before any rollup:

```
usage kind=review-roles inst=1 role=7 attempt=1 tag=iter3 target=15892 id=a0d9cfe8 model=opus-5 turns=71 cache_read=8.7M cache_write=0.3M in=0K out=19K est_usd=6.52
usage kind=review-roles inst=2 role=7 attempt=1 tag=iter3 target=15892 id=a4efde94 model=opus-5 turns=68 cache_read=8.1M cache_write=0.3M in=0K out=17K est_usd=6.36
usage kind=review-roles inst=2 role=7 attempt=2 tag=iter3 target=15892 id=ab12c034 model=opus-5 turns=12 cache_read=1.2M cache_write=0.1M in=0K out=3K est_usd=1.30
usage kind=gh-style tag=iter3 target=15892 id=af7b3fcc model=opus-5 turns=53 cache_read=8.4M cache_write=0.4M in=0K out=10K est_usd=6.95
usage kind=review-scorer batch=1 tag=iter3 target=15892 id=a7c31560 model=sonnet-5 turns=9 cache_read=0.6M cache_write=0.1M in=0K out=4K est_usd=0.42
```

That is the filter's exact output, appended verbatim. The stamp's own fields come first because the
filter copies them through, then the transcript id, then the sums. The per-iteration rollup in
[SUMMARY.md](SUMMARY.md) and the run total in the Final Report are derived from these lines, so a
skipped rollup loses nothing, and `grep 'usage.*role=7.*tag=iter3'` across a run log gets every cent
role 7 spent in that iteration.

## The estimate

`est_usd` is list price times the standard cache multipliers, and it is an estimate rather than a
bill. Rates as of 2026-09-02, per million tokens:

| model | input | cache read (0.1x) | cache write (1.25x) | output |
|---|---|---|---|---|
| `claude-opus-5` | 5.00 | 0.50 | 6.25 | 25.00 |
| `claude-sonnet-5` | 2.00 | 0.20 | 2.50 | 10.00 |
| `claude-haiku-4-5` | 1.00 | 0.10 | 1.25 | 5.00 |

`est_usd = input*in + cache_read*cr + cache_write*cw + output*out`, each in millions. A model not in
the table gets `est_usd=?` rather than a guess, and the summary says which model was unpriced. This
table rots when prices change. The date is here so a reader can tell how stale it is, and updating
it is the fix rather than trusting it.

## What this deliberately leaves out

**The orchestrator's own cost.** That is the session transcript, and the skill would be measuring
itself mid-run, from a file that is still being appended to. Leave it out and say so in the summary.
The number a reader wants for the orchestrator is the run total minus the sum of these lines, read
from the session's own usage after the run, and that is a different tool.

**Thinking tokens as a separate line.** `usage.output_tokens_details.thinking_tokens` exists on each
turn and is already inside `output_tokens`, so it is billed in the output figure above. Break it out
only if effort tuning becomes the question, since it is the number that effort moves.

## Why cache reads dominate, so nobody is surprised by the ratio

Measured on one `pr-review` run: 24 role agents at Opus read 104M cached tokens and produced 272K
output tokens, a ratio near 380 to 1. Every turn re-reads the whole context, so an agent's cost is
close to linear in its turn count and nearly independent of what it says. That is why `turns` is on
the line beside the token counts. An agent that ran `bash true` 111 times to poll for results read
42.5M tokens doing it, more than any two roles combined, and that number is what made the polling
visible as a cost rather than a curiosity.
