# Logging levels, metric emission, and the error-handling reviewer

Date: 2026-08-03

Target files:
- `shared_config/.claude/AGENTS.md` (syncs to `~/.claude/AGENTS.md`, loads for every repo)
- `shared_config/.claude/skills/in-depth-review/SKILL.md` (Reviewer Role #8 only)

## Goal

Claude logs and emits metrics too readily. Two new AGENTS.md sections curb it. One fixes log level
choice, so a bug reaches the level monitors actually watch. The other requires a named consumer
before a metric exists.

A third change makes the log level rule enforceable at review time. The error-handling reviewer
role gains one rubric item for it. That role carries a self-contained rubric and never defers to
AGENTS.md, so a new AGENTS.md rule does not reach it on its own.

## Why this file and not the repo root

`/Users/melvin/.melvin/config/AGENTS.md` is about this repo specifically. Its sections are
Bootstrap flow, Shell config, Layout, and similar. A general engineering rule does not belong
there. `shared_config/.claude/AGENTS.md` is the global file that already carries the other
code-quality rules, namely "Code comments" and "TypeScript: narrow with type guards".

Neither file has any existing logging, metrics, or Datadog guidance. Verified by grep for
datadog, log level, logger, metric, telemetry, and counter. The only hit is an unrelated
`// increment the counter` example inside the Code comments section.

## Decisions taken

| Decision | Choice | Why |
|---|---|---|
| Genericity | Provider-neutral prose with one stack note | The file loads into Go repos as well as Calm TypeScript repos. A global file that asserts `calmLogger` everywhere trains an agent to invent that import where it does not exist. One parenthetical keeps the concrete anchor without the cost. |
| Which logging tics to govern | Level choice, plus never log-and-swallow | The two the user reported. "One failure, one log" and "no narration logging" were considered and dropped. Neither was a reported problem, and a rule nobody needs is exactly what the metrics section bans. |
| Level ladder depth | Error vs warn only, with a checkable test for warn | That one boundary is what gets confused. Info and debug are left alone. The test matters because "needs visibility" is the judgement being got wrong today, so the section has to give the agent something new to check itself against. |
| Name Datadog | No | The rule turns on whether a consumer exists, which is backend-independent. One stack note is enough concreteness. |
| Heading level | `##` | Every section in this file is `##`. The source text used `###`. |
| Section order | Logging levels first | The metrics section refers to "**Logging levels** above". |
| Line wrapping | None in AGENTS.md. One long line per paragraph and per bullet. | This file does not hard-wrap. 43 of its lines exceed 200 characters. This is the opposite of the `SKILL.md` files, which wrap near 98. The Role #8 item DOES wrap, matching its sibling rubric items. |
| Em-dash in new text | None, anywhere, including the Role #8 rubric item | The em-dash is banned outright. The carve-out list in "Code comments" enumerates allowed non-joiner uses of the ASCII hyphen `-` and the colon `:`, meaning hyphenated words, CLI flags, ranges, label prefixes, ratios. None of those involves an em-dash, so `—` never had a blessed use. Commit `4a0b06d` invented a "definition dash" exemption the rule does not grant and left 216 lines standing on it. This spec adds none. |
| Reviewer coverage | Role #8 gets the log level item. Nothing gets a metrics item. | "Can you name the consumer?" is not answerable from a diff, and Role #8 is instructed to read only the diff. A metrics rubric item would generate findings that die in the confidence filter. Metrics stay with Role #1, which reads AGENTS.md in full. |
| Reviewer item direction | Under-leveling only, meaning a bug logged below error | Over-leveling is covered by a prohibitive sentence in the warn paragraph, "A condition nobody must act on does not belong at error", NOT by the definitional sentence. This distinction is load-bearing. Role #1 must cite actual rule text, and in-depth-review's scoring rubric assigns 25 when "the cited rule doesn't actually call this out", which dies at every caller threshold. An earlier draft of this spec claimed the definitional sentence sufficed. It did not, and the final review caught it. Do not remove that prohibitive sentence believing the definition covers it. |

## Placement

Insert both sections between the end of the TypeScript section and the Plain ASCII heading. At
time of writing that is after line 118 and before line 120, but locate by text rather than number.

Anchor to find:

```
## Plain ASCII in authored prose
```

Insert the two sections immediately above it, keeping one blank line between each section and the
next heading. That lands them in the file's code-quality cluster and leaves the prose-style rule
last.

## Section 1, exact text

Applies to Sections 1 and 2 only, NOT Section 3. Every paragraph and every bullet here is a single
unwrapped line, and the blocks below are already written that way. Copy each one as one line. Do
not introduce a line break inside a paragraph or a bullet. The only newlines are the blank lines
between blocks. Section 3 edits a different file that DOES wrap, and it carries its own rule.

```markdown
## Logging levels

Error level is what monitors are keyed to. Anything a human needs to see and act on emits at error, even when the code handled it and recovered. A bug logged at warn is a bug nobody is paged for.

Warn is for a condition you expected, handled, and nobody needs to act on. If you can name who must act on it and what they would do about it, it was not a condition you expected, so it is an error. A condition nobody must act on does not belong at error, since every false error dilutes the level monitors page on.

A `catch` block that logs and continues must say why continuing is correct. Swallowing a failure converts it into silent wrong behavior, which is harder to diagnose than a crash. If the fallback path is genuinely fine, a comment naming why belongs there. If it is not fine, the log is an error and the failure propagates.
```

## Section 2, exact text

```markdown
## Emitting metrics

Emit a metric only when something will consume it. Be able to name the consumer before adding the metric, meaning a dashboard, a monitor, or an experiment readout. A cron is the clear yes case. Nobody watches it run, so the monitor keyed to its success or its absence is the consumer. Unread telemetry is not free. It persists indefinitely, and it advertises a monitoring story that does not exist, so a later reader trusts a signal no alert is keyed to.

- A counter next to an error-level log for the same failure usually fails this test (`calmLogger.error` in Calm repos). The error log is already the loud, alertable signal, so the counter earns its place only if you specifically need the aggregate volume as well. See "Logging levels" above.
- Prefer deriving a number from data already kept, such as a table you can count or an event already emitted, over a new counter whose only purpose is to be counted.
- Splitting one condition across several counters needs a reason a reader can check. If two counters exist so a dashboard can tell two causes apart, say which dashboard. Otherwise emit one, or none.
```

## Section 3, the error-handling reviewer item

Different file: `shared_config/.claude/skills/in-depth-review/SKILL.md`. Reviewer Role #8 lives at
lines 543-565 at time of writing. Its rubric is six numbered items inside a fenced prompt block,
followed by a closing paragraph that begins "Skip".

Append a seventh item after item 6 and before that closing paragraph. Appending means items 1
through 6 keep their numbers, so nothing else in the file needs touching.

Anchor to find, which spans the boundary so the insert lands in the right place:

```
   without wrapping the original; using `throw new Error(e.message)` instead of `cause: e`).

Skip "you could define a custom error class".
```

Insert between those two, giving:

```
   without wrapping the original; using `throw new Error(e.message)` instead of `cause: e`).
7. Log level. A failure that needs human attention logged below error level. Error is what
   monitors are keyed to, so a bug logged at `warn` is a bug nobody is paged for.

Skip "you could define a custom error class".
```

Three constraints on this item.

**No em-dash.** Items 1 through 6 each use a ` — ` definition dash, and item 7 deliberately does
not. It uses `Log level.` with a period instead, which keeps the scannable label without the glyph.
Item 7 will therefore look inconsistent beside its siblings until the follow-up spec sweeps them.
That is expected and correct. Do not "fix" the inconsistency by giving item 7 a dash, and do not
sweep items 1 through 6 as part of this spec. See **Follow-up spec** below.

Wrap to match the sibling items, which break near 97 columns with a three-space continuation
indent. Do not match a file-wide width. The block is what matters.

Do not touch the role count anywhere. The frontmatter description says "EIGHT TO TWELVE
specialized parallel reviewer roles" and the role table near line 269 lists role 8 as
`error-handling`. Adding an item to a role changes neither.

## Why Role #8 and not only Role #1

Role #1 reads the relevant AGENTS.md files in full and audits against them, so it would pick up
the new Logging levels section generically. Role #8 is still the right home for an explicit item
for three reasons.

It is the specialist where a log level defect naturally surfaces, and its rubric is self-contained
and never mentions AGENTS.md. Role #1's attention is spread across a long file whose harness rules
it is explicitly told to ignore, so a single level rule is diluted there. And `review-and-fix` maps
findings to role numbers for its adaptive rerun, so a level finding attributed to role 8 reruns the
error-handling specialist rather than the whole set.

Item 2 already covers "catches that only log-and-continue when the caller needs to know". That is
the swallow case, and item 7 is the level case. They are independent, since code can log and
continue at the right level, or propagate correctly at the wrong one. One defect may be reported
under both, which the existing dedup step handles.

## What changed from the source text

The source is a section from another repo's AGENTS.md. It cannot be pasted in as-is, because it
violates three rules this very file states. Every transformation below is required, not stylistic
preference.

| Source | Adapted | Rule that forces it |
|---|---|---|
| `the consumer — a dashboard, a monitor, an experiment readout —` | `the consumer before adding the counter, meaning a dashboard, a monitor, or an experiment readout` | Plain ASCII bans the em-dash as a clause joiner |
| `not free: it persists indefinitely` | `not free. It persists indefinitely` | Bans a clause-splitting colon |
| `data already kept — a table you can count, an event already emitted — over` | `data already kept, such as a table you can count or an event already emitted, over` | Em-dash pair |
| `say which dashboard; otherwise emit one, or none.` | `say which dashboard. Otherwise emit one, or none.` | Code comments says "Avoid semicolons" |
| a calmLogger.error | an error-level log, with calmLogger named once in a parenthetical | Genericity decision above |
| `### Emitting metrics` | `## Emitting metrics` | Every heading in this file is `##` |

Two additions beyond punctuation. The cron sentence, which is the user's own canonical yes-case.
And the log-and-swallow paragraph, which the source does not have because its companion Logging
levels section was not quoted.

All three of the source's bullets are kept. The user endorsed them.

## Verification

No executable code, so verification is grep-based. Patterns use `\s+` for spaces as a habit,
though this file does not wrap, so a literal space would also work here.

1. `rg -c '^## Logging levels' shared_config/.claude/AGENTS.md` returns 1.
2. `rg -c '^## Emitting metrics' shared_config/.claude/AGENTS.md` returns 1.
3. `rg -n '^## Logging levels|^## Emitting metrics|^## Plain ASCII' shared_config/.claude/AGENTS.md`
   returns three lines whose numbers ascend in that order. This is the ordering requirement, since
   the metrics section refers to Logging levels as being above it.
4. `rg -c 'calmLogger' shared_config/.claude/AGENTS.md` returns 1. Exactly one stack note.
5. `grep -c '—' shared_config/.claude/AGENTS.md` returns 15, unchanged from before the edit. No
   em-dash was introduced. An en-dash check is unnecessary since the file has none.
6. `rg -c '\x3B' shared_config/.claude/AGENTS.md` returns 8, unchanged. No semicolon introduced.
   Note the pattern uses a hex escape because a literal semicolon in a Bash command trips the
   multi-command hook.
7. `rg -c 'Datadog' shared_config/.claude/AGENTS.md` returns 0, confirming the decision not to
   name the backend held.
8. Read both new sections once and confirm no sentence joins two independent clauses with a dash
   or a splitting colon.

For `shared_config/.claude/skills/in-depth-review/SKILL.md`:

9. `rg -c '^7\. Log level' shared_config/.claude/skills/in-depth-review/SKILL.md` returns 1.
10. `rg -c '^6\. Context preservation' shared_config/.claude/skills/in-depth-review/SKILL.md`
    returns 1. Confirms item 6 survived and was not overwritten by the insert.
11. `rg -n '^7\. Log level|^Skip "you could define' shared_config/.claude/skills/in-depth-review/SKILL.md`
    returns two lines, with the item 7 line number lower. Confirms the item landed inside the
    rubric rather than after the closing paragraph.
12. `rg -c 'EIGHT TO TWELVE' shared_config/.claude/skills/in-depth-review/SKILL.md` returns 1.
    Confirms the role count in the frontmatter was not touched.
13. Em-dash line count stays at exactly 82, unchanged. Item 7 introduces none. Record the baseline
    with `grep -c '—' <file>` before editing in case the file has moved on. Any rise means a dash
    slipped into item 7, which is the single most likely error here, since every sibling item has
    one and the shape invites copying.
14. Read item 7 beside items 1 through 6 and confirm the wrap and the three-space continuation
    indent match.

## Follow-up spec: the absolute em-dash ban

Deliberately NOT in this spec. It is its own spec and plan, to be written next. Recorded here
because this spec's item 7 leaves a visible inconsistency that only makes sense with the follow-up
in view.

Scope of that follow-up:

- Rewrite the AGENTS.md **Plain ASCII in authored prose** and **Code comments** rules so the
  em-dash ban is stated as absolute, with no joiner-versus-non-joiner distinction. The carve-out
  list stays, but must say explicitly that it covers the ASCII hyphen and the colon only.
- Sweep the existing em-dashes. Measured counts: AGENTS.md 15 lines, in-depth-review 82,
  pr-review 41, review-and-fix 40, gh-style-review 38. That is 216 lines across five files.
- Standardize a replacement shape for `Term — definition` list items. This spec uses
  `Term. Definition.` for item 7, so adopting that shape leaves item 7 untouched by the sweep.
- Keep, verbatim, the lines that quote the glyph in order to ban it. The rule text itself contains
  `—` as data, not as punctuation, and the same is true of any quoted command output. That is the
  one carve-out a sweep must honour, and it is about literal content rather than style.

Why it is separate: rewriting a rule and editing 216 sites across five files is not a footnote to a
logging change. Bundling them would bury the two rules actually asked for inside a mechanical diff.

Why it should not be deferred indefinitely: `4a0b06d`'s own commit message diagnosed the problem it
then recreated. A style rule its own files violate cannot be enforced, which is why reviewers keep
flagging these files and keep being correct against the text.

## Known follow-ups from the reviews (parked, not applied)

Recorded here because the SDD workspace holding the review ledger has been deleted and these
rulings are not in git history. None is load-bearing. All were adjudicated rather than fixed,
since the process allows no second fix wave.

**R1. Role #8's item 7 could now carry the tightened warn test.** Item 7 says only "A failure
that needs human attention", which is less concrete than the rule it enforces. The final review
declined to import the test at the time, because the version then in the file carried the
capability-versus-necessity defect and would have imported it straight into the rubric. That
defect is now fixed, so importing the tightened test became a cheap improvement. It fits inside
the 97-column wrap. This is the most actionable of these follow-ups.

**R2. Sentence 2's rationale is false for one case, deliberately left.** It reads "if you can
name who must act on it ... it was not a condition you expected, so it is an error". For a rate
limit being hit you genuinely did expect it and someone must still act, so the stated reason is
false there even though the conclusion is right. Leave it. The conclusion cannot be wrong, since
a genuine must-actor means the section's first paragraph forces error independently. And that
same false-rationale check is the mechanism that keeps a deprecation notice out of error, so
making the rationale universally true would re-open the defect it was written to fix.

**R3. info and debug are undefined.** A cache-miss fallback and an optional config default both
route to warn, with no lower level offered, though both are arguably debug. This was a deliberate
decision, see the Decisions table. Revisit only if warn starts collecting noise.

**R4. Metrics bullets 2 and 3 still say "counter".** The reasoning generalizes to any metric type,
since splitting one condition across several gauges has the same problem. The approved scope of
that fix was "where the rule is stated", meaning the paragraph, so the bullets were left as
examples. Widening them is optional.

**R5. If these commits are ever squashed, take the corrected wording.** Commit `09dff3f`'s body
still asserts the pre-fix inverted test, "If you cannot name who acts on it, it is an error".
Commit `b0fd6e0` documents the flip and `7fb78e2` the tightening, so the history is honest read as
a sequence. A naive squash that keeps the first message would ship a claim the rule now
contradicts.

## Out of scope

- The repo-root `AGENTS.md`. Wrong home for a general rule.
- Log volume rules. "One failure, one log" and "no narration logging" were offered and declined.
- Info and debug levels.
- Naming Datadog, or any metrics client API.
- Rewriting the em-dash rule, and sweeping the 216 existing em-dashes. See **Follow-up spec**.
- `pr-review`, `review-and-fix`, and `gh-style-review`. The first two delegate their reviewer
  roles to `in-depth-review`, and the third mirrors the GitHub Action prompt, so the Role #8 change
  reaches all of them with no edit of their own. The follow-up sweep does touch all three, but
  nothing in THIS spec does.
- Every reviewer role in `in-depth-review` except #8. Roles are not renumbered and no role is
  added, so the role count, the role table, and the frontmatter description all stay as they are.
- A metrics rubric item on any role, for the reason in the decisions table.
- An earlier draft of this spec claimed the review skills needed no change because they defer to
  AGENTS.md. That was wrong for Role #8, whose rubric is self-contained. Recorded here so the
  claim is not reintroduced.
