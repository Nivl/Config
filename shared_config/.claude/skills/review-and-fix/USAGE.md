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
<!-- fix-precommit tag=iter3 batch=1 findings=B,C,D target=15892 -->
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
bill. Rates as of 2026-10-07, per million tokens, from platform.claude.com's pricing page. Its table still listed Sonnet 5.5's cache read at $0.20 while its prompt-caching section gave 0.05x input, $0.10, and the section is the one used here. Haiku 5.5 is priced per response by that response's prompt size (input plus cache read plus cache write):

| model | input | cache read | cache write (1.25x) | output |
|---|---|---|---|---|
| `claude-fable-5-1`, `claude-fable-5` | 10.00 | 0.25 | 12.50 | 50.00 |
| `claude-opus-5-5` | 4.00 | 0.20 | 5.00 | 20.00 |
| `claude-sonnet-5-5` | 2.00 | 0.10 | 2.50 | 10.00 |
| `claude-haiku-5-5`, prompt up to 100K tokens | 0.10 | 0.01 | 0.125 | 0.50 |
| `claude-haiku-5-5`, prompt over 100K tokens | 0.50 | 0.05 | 0.625 | 2.50 |
| `claude-opus-5` | 5.00 | 0.50 | 6.25 | 25.00 |
| `claude-sonnet-5` | 2.00 | 0.20 | 2.50 | 10.00 |
| `claude-haiku-4-5` | 1.00 | 0.10 | 1.25 | 5.00 |

Fable's rates were given by the user on 2026-09-14 (cache read is 0.025x input there, not 0.1x),
and its cache-write rate is assumed at the 1.25x multiplier. Fable is the orchestrator's model, so
this row prices the session transcript when the filter is pointed at it, which is how the
orchestrator's per-phase cost in the run-log notes was computed.

Opus 5.5's cache read is 0.05x input, not 0.1x, so it is 60 percent below Opus 5's while input and
output are 20 percent below. A role agent's spend is mostly cache reads, so the same role run
prices at roughly half on 5.5. Compare runs across the switch by the `model=` field, since a
`$/iteration` figure from before 2026-09-22 is an Opus 5 figure. The agents pin the `opus` alias,
so which Opus ran is read from `model=`, never from the pin.

**Every figure computed before 2026-09-25 is too high, by about 1.8x to 3.3x.** A transcript writes
one API response as several assistant records, one per content block, and each record carries the
same `usage`. The filter summed every record, so a response with thinking, text and a tool call was
priced and counted as three turns. It now counts each message id once. Measured on the fix: one
orchestrator transcript held 1,244 records for 380 responses, and older Fable and Opus 5 sessions
split 1.8x to 3.0x. Comparisons between runs from before the fix mostly hold, since the ratio
varies within that band, but the dollar amounts and turn counts in older run logs, in the Final
Reports, and in the rationales quoted across these skills and AGENTS.md are the inflated ones.

**A one-hour cache write bills at 2x input, not 1.25x.** Claude Code writes one-hour cache entries,
and the usage record splits `cache_creation` into `ephemeral_5m_input_tokens` and
`ephemeral_1h_input_tokens`. The filter prices the five-minute part at the table's cache-write rate
and the one-hour part at twice the input rate. The line prints the one-hour part as
`cache_write_1h=`, inside the `cache_write=` total, so the rate each part got is readable from the
line. Before 2026-09-26 it priced both at the five-minute
rate, which put one Opus 5.5 orchestrator session about 7 percent low.

`est_usd = input*in + cache_read*cr + cache_write_5m*cw + cache_write_1h*2*in + output*out`, each in millions. A model not in
the table gets `est_usd=?` rather than a guess, and the summary says which model was unpriced.

**A dated model id prices at its undated key.** A transcript can record either spelling, and one
measured run logged `claude-haiku-4-5-20251001` where the table has `claude-haiku-4-5`. The filter
strips a trailing `-YYYYMMDD` before the lookup, so the dated id prices at the same row instead of
falling through to `?`. The printed `model=` keeps the id as recorded, so which snapshot ran is
still readable. A bare tier alias is a different case and stays unpriced. `haiku` and `sonnet` both
appear in this project's transcripts, and neither names a version, so pricing one would mean
picking a version on the reader's behalf and printing a number for it. That is the guess the
sentence above refuses.

This
table rots when prices change. The date is here so a reader can tell how stale it is, and updating
it is the fix rather than trusting it.

## What this deliberately leaves out

**Calls with no transcript record.** About 8M Opus cache-read tokens in the GRO-19075 session, around
$1.60, appear in Claude Code's own tally and in no transcript. Which calls they are was not found.

**Thinking tokens as a separate line.** On the main thread, `output_tokens` already includes
thinking, so it is billed in the output figure above. Break it out only if effort tuning becomes the
question, since it is the number that effort moves.

## Sub-agent output is a floor

A sub-agent's transcript records each response's `output_tokens` from the start of the stream, not
the final count. One `fix-precommit` response wrote about 600 tokens of tool input and recorded 4.
The main thread's transcript records the final count. Measured on the GRO-19075 session against
Claude Code's own tally in `~/.claude.json` (`lastModelUsage`), sub-agent output was 18K against
57K to 103K for Opus 5.5, 2.9K against 27.8K for Sonnet 5.5, and 0.6K against 9.4K for Fable 5.1.
The range for Opus depends on whether that tally counts thinking inside `outputTokens`, which it
does not say. Cache reads and writes are most of an agent's cost, so the dollar effect is smaller
than those ratios: the sub-agent lines ran about 10 to 25 percent low, and the whole session priced
at $25.70 against the tally's $29.56. The thinking text is blanked in the transcript, so the real
count cannot be recovered from it. Read every sub-agent `est_usd` as a floor.

**The orchestrator** is priced from its own transcript with `--arg since`, which drops the
responses before the run started (a `work-on` session holds the implementation phase before them).
FINAL-REPORT.md's Spend section has the command. On GRO-19075 the whole main thread was $15.12 and
the review run's part of it $3.92.

## Why cache reads dominate, so nobody is surprised by the ratio

Measured on one `pr-review` run: 24 role agents at Opus read 104M cached tokens and produced 272K
output tokens, a ratio near 380 to 1. Every turn re-reads the whole context, so an agent's cost is
close to linear in its turn count and nearly independent of what it says. That is why `turns` is on
the line beside the token counts. An agent that ran `bash true` 111 times to poll for results read
42.5M tokens doing it, more than any two roles combined, and that number is what made the polling
visible as a cost rather than a curiosity.
