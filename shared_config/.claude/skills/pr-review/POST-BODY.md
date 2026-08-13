# Building and posting the review (Step 3b/3c/3d)

## Contents
- Step 3b: Build the global body — the confidence label and rule-attribution rules
- The global body template
- Formatting rules (pluralization, the bold-marker rule, section order, tags)
- Step 3c: Build each inline comment
- Step 3d: Post the review (one API call)

## Step 3b: Build the global body

The global body **must start** with the disclosure line below, verbatim, as the first
paragraph, on its own line. Do not change its wording. Do not append any branding footer,
"Generated with Claude Code" line, or ensemble-stats `<sub>` tag at the end.

**Never emit a section that has no content, and never report what passed.** Every
`###`/`####` block below is conditional on its count. When the count is zero, omit the
header and the block entirely. No "all clear", no "no gaps", no roll-call of green tickets.
The review surfaces only what needs attention.

Let:

- `K_global` = count of GLOBAL findings
- `K_inline` = count of INLINE findings (each is also posted as an inline diff comment in Step 3c)
- `K_unaddressed` = count of entries in `unaddressed_pool`

**Confidence label (what the reader sees).** Internally a finding has a numeric `confidence`
(0-100), used ONLY for filtering and ordering. The POSTED comment never shows the number, the
agreement count, or the source skill. Those are internal mechanics that confuse readers. It
shows one word, mapped from the score:

| score | label |
|---|---|
| 90-100 | `Critical` |
| 75-89 | `High` |
| 60-74 | `Medium` |
| below 60 | `Low` |

(Normal findings cleared the >=60 bar, so they read `Medium` or higher; only an approach
finding can bypass that bar and read `Low` -- e.g. a score of 42 shows as `Low`.) The whole tag
is just `` `[confidence: <Low|Medium|High|Critical>]` `` (plus the `approach` marker for
those findings, and a `[<ticket_id>]` title prefix for ticket findings).

**Rule attribution (what the reader sees).** A finding may arrive citing the convention file it
came from, `AGENTS.md` or `CLAUDE.md`, together with the rule text. Role #1 of `in-depth-review`
is told to cite both, and `gh-style-review` carries its own convention-compliance directive, so
either sub-skill can produce a doc-attributed finding. That citation is what lets the scorer
check the rule says what the finding claims. It is internal mechanics, the same as the confidence
number. **Never post the file name, whichever file it is, and never post a phrase like "AGENTS.md
says" or "CLAUDE.md says".** The reviewer's convention files are usually not in the repo under
review, so a reader goes looking for a rule that is not there and concludes the finding was
invented. This holds even when the file IS in the repo under review. A reader cannot tell which
copy you mean, and the repo's copy may differ from yours, so a citation that looks checkable can
still mislead.

State the rule directly instead, as a norm with no source attached, and keep whatever the rule
permits, since the permitted list is what tells the author where the boundary is. For the comment
that prompted this rule, that reads:

> One thought per sentence, and don't glue two clauses with a colon or a dash. The colon after
> "DB I/O" splits a claim from its elaboration. Hyphenated words, CLI flags, ranges, label
> prefixes, ratios and times, and code, paths, or URLs are fine, but this is not one of those.

This governs the title as well as the description, and inline comments as well as global ones.

Two ways to get this wrong. Do not swap the citation for a vaguer authority such as "the project
requires" or "the team convention is". That is worse than naming the file, because it asserts
something about this repo that is not true. And do not soften the rule into a preference, because
the finding still has to read as actionable rather than as one reviewer's taste.

## The global body template

```markdown
I used an AI agent with a custom prompt to generate this review.

<<if K_global + K_inline > 0:>>

### Code review

Found <K_global + K_inline> issue(s):

<<if K_global > 0:>>

#### <K_global> global issue(s)

**1.** <title> &nbsp;`[confidence: <Low|Medium|High|Critical>]`

<description, including any suggested-fix alternatives>

<permalink>

**2.** ...
<<endif>>

<<if K_inline > 0:>>

#### <K_inline> local issue(s)

- <title>
- <title>
- ...

I left <K_inline> comment(s) directly in the diff for those.
<<endif>>
<<endif>>

<<if ticket_notes_present (Step 3a.5):>>

### Tickets examined

<One short paragraph. Surface ONLY: sub-threshold ticket observations — name the ticket and
the AC, and say it did not clear the >=60 bar and why — plus any unread/unverified ticket. Do
NOT list tickets that passed. Do NOT repeat above-threshold ticket gaps; those already appear
as Code review findings above, prefixed [<ticket_id>].>
<<endif>>

<<if K_unaddressed > 0:>>

### Still unaddressed in this PR

Concerns raised by reviewers earlier that this PR does not appear to address:

- > <quote> — ([link](url))
  >
  > ⚠️ <gap>
- ...
  <<endif>>
```

A global body is produced whenever Step 3 reaches here (at least one finding or one
unaddressed concern). The disclosure line is the only unconditional content. The `### Code
review` section (and its `#### global` / `#### local` subsections), "Tickets examined", and
"Still unaddressed" are each emitted only when their count is non-zero. A body carrying just
the disclosure line is impossible: Step 3 gates entry on having something to say.

## Formatting rules

**Pluralize the headers.** In the `#### ... issue(s)` headers, render the real word: "issue"
when the count is 1, "issues" otherwise, e.g. `#### 1 global issue`, `#### 3 local issues`.
The `Found <N> issue(s):` count is `K_global + K_inline` (the (s) shorthand is fine there).

**Global issues use a bold `**N.**` marker, NOT a real Markdown ordered list.** Each global
finding spans several paragraphs (title, description, permalink), and a multi-paragraph ("loose")
`1.` list item gets its marker re-printed before every paragraph by some renderers, so the
whole block shows up as `1.` on every line. A bold `**1.**` marker with flush-left paragraphs
renders identically on GitHub and elsewhere (a line starting with `**` is never parsed as a list
item, so nothing re-numbers). Number them `**1.**`, `**2.**`, … by hand. Do not convert this
back to a `1.`/`2.` ordered list.

**Local issues are names only.** Under `#### local issue(s)`, list each INLINE finding's
`<title>` as a bullet — no description, no permalink (single-line bullets render cleanly, so an
ordinary `-` list is fine here). The full text rides on the inline diff comment (Step 3c); the
trailing "I left <K_inline> comment(s) directly in the diff for those." line points the reader
at them.

**Section order is fixed:** code review (global then local) -> tickets examined -> still
unaddressed.

**Ticket findings:** when a finding has a non-null `ticket_id`, prepend `[<ticket_id>] ` to
its `<title>` in both the global body (global list and local list) and any inline comment, so
the ticket is visible.

**Approach findings (`source = "approach"`):** these carry the `approach` marker.
Render their annotation as `` `[approach, confidence: <Low|Medium|High|Critical>]` `` everywhere
the other findings show `[confidence: ...]`: in the global `**N.**` list and the inline comment.
Bucket the approach finding's own score with the same mapping above (a sub-60 approach finding
reads `Low`). Do NOT show the round count; it is an internal debate mechanic, not reader signal.
The names-only local list shows just the `<title>`; the `[approach, ...]` tag rides on the
global entry and the inline comment. If an approach finding also has a `ticket_id`, prepend
`[<ticket_id>] ` to the title as above; that is independent of the approach marker.

## Step 3c: Build each inline comment

Each inline comment carries a tighter body (no disclosure repetition, no permalink — GitHub
anchors the comment to the line for you):

```markdown
**<title>** `[confidence: <Low|Medium|High|Critical>]`

<description, including any suggested-fix alternatives>
```

**Approach findings:** for a finding with `source = "approach"`, use the
`` `[approach, confidence: <Low|Medium|High|Critical>]` `` tag here instead. See Step 3b.

**Rule attribution:** the rule above applies here unchanged. An inline comment reuses the same
`<description>`, so strip the convention file name and any phrasing that attributes the rule to a
document here too.

GitHub line-range encoding:

- Single line: `{ "path": "<file>", "line": <N>, "side": "RIGHT", "body": "<body>" }`
- Multi-line: `{ "path": "<file>", "start_line": <S>, "start_side": "RIGHT", "line": <E>, "side": "RIGHT", "body": "<body>" }`

`side` is always `RIGHT` (the new revision). Comments on the LEFT side (deleted lines) are
seldom useful for a forward-looking review and are out of scope here.

## Step 3d: Post the review (one API call)

Reached only after the Step 3c.7 human-review gate has approved (all findings, or the kept
subset). Post exactly the surviving set.

Fetch the PR head SHA and the owner/repo:

```sh
PR_HEAD_SHA=$(gh pr view <PR> --json headRefOid --jq .headRefOid)
OWNER_REPO=$(gh repo view --json owner,name --jq '.owner.login + "/" + .name')
```

Build the JSON payload (single review with body + inline comments):

```json
{
  "event": "COMMENT",
  "commit_id": "<PR_HEAD_SHA>",
  "body": "<global body from Step 3b>",
  "comments": [
    { "path": "...", "line": ..., "side": "RIGHT", "body": "..." },
    { "path": "...", "start_line": ..., "start_side": "RIGHT", "line": ..., "side": "RIGHT", "body": "..." }
  ]
}
```

`event=COMMENT` is important. It leaves the review as comments, not as approval or
change-requested. This skill never approves a PR or requests changes; it only comments.

Post it:

```sh
gh api -X POST "/repos/$OWNER_REPO/pulls/<PR>/reviews" --input - <<EOF
<rendered JSON payload>
EOF
```

**When a GitHub MCP is connected (the preferred path), post the review through it** (see
**GitHub access**): use a create-and-submit-pull-request-review tool with the same `body` and
`event=COMMENT`; when there are inline comments, use the pending-review trio (create a pending
review -> add one review comment per INLINE finding at its `path`/`line`/`side` -> submit the
pending review as `COMMENT`). This is still ONE logical review. The 422 handling below applies
either way.

If the API responds 422 because one or more inline comments target lines not in the diff,
demote those specific comments to GLOBAL (append them to the global body in a "Couldn't anchor
inline" subsection) and re-issue the call. Do not silently drop findings.

This is the only PR review this skill writes, and the only other writes it may make are the
opt-in in-progress comment (Step 0.7) and its deletion (Step 3e) — see the orchestrator
SKILL.md's Constraints section for the exact posting conditions.

## Step 3c.7 file format: the human-review gate

SKILL.md states the hard gate itself (dump to a file, pause, post only what's approved). This
section is the exact file format and response handling.

1. **Write** `/tmp/claude/pr-review-<PR>-comments.md`. Emit **one block per finding**, numbered
   `#1` through `#N` in the Step 2 order (GLOBAL and INLINE findings), followed by the
   unaddressed concerns. Each of these is its own block:
   - each GLOBAL finding,
   - each INLINE comment,
   - each entry in `unaddressed_pool`.

   Each block is:
   - a header line: `#<n> [GLOBAL]`, or `#<n> [INLINE] <file>:<line_range>`, or
     `#<n> [UNADDRESSED]`. Append the confidence label and any `approach` /
     `[<ticket_id>]` tag the finding carries, so the header reads the way the posted comment
     will (e.g. `#3 [INLINE] src/auth.ts:40..52  [confidence: High]`).
   - the rendered comment body below it: title + description for a finding, or the
     `quote` + `gap` for an unaddressed concern.
   - for a finding that arrived with a convention-file citation, that citation on its own line
     beneath the body, prefixed `source:`. This file never reaches GitHub, and the user owns the
     convention file, so the citation is what lets them check a finding against their own rule
     before approving it.

   Separate every block from the next with a divider line that is exactly thirteen `=`
   characters on its own line, nothing else:

   ```
   =============
   ```

   The divider makes each finding visually distinct so the user can scan and judge them one at
   a time.

2. **Pause.** Tell the user the file path and a one-line tally: `<K_global> global,
   <K_inline> inline, <K_unaddressed> unaddressed`. Ask them to review the file and confirm
   before you post. Then wait.

3. **Act on the user's response:**
   - **Approve all** -> continue to Step 3d and post everything.
   - **Keep a subset** (e.g. "drop #2 and #5", "only post #1 and #4") -> remove the dropped
     findings from the pools, re-run the Step 3b body assembly and Step 3c inline set from the
     survivors (so counts, numbering, and the global/local lists all reflect the reduced set),
     rewrite the file, then continue to Step 3d with the remainder. If dropping leaves nothing
     to post, treat it as a decline.
   - **Decline all** -> post nothing. Skip Step 3d entirely, still run Step 3e cleanup, and
     report in Step 4 that the comments were written to the file but not posted at the user's
     request.
