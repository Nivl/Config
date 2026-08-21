# Jira formatting

Step 5 of [SKILL.md](SKILL.md). Read this before the write, not after the check fails.

## The problem

Jira Cloud does not store markdown. The description and comment fields hold **ADF** (Atlassian
Document Format), a JSON tree of typed nodes. Markdown that reaches a field without being
converted is stored as literal characters, so the ticket renders with `##` and `**` visible on
the page. That is the exact failure this file exists to prevent, and it is fully public the
moment it happens.

Neither write path takes markdown natively:

| Path | What it accepts |
|---|---|
| Atlassian MCP `editJiraIssue` | markdown **or** ADF, selected by the `contentFormat` parameter |
| `acli jira workitem edit` | plain text **or** ADF. There is no markdown mode. |

So the MCP converts for you and `acli` does not. That is why Step 0 prefers the MCP.

## Path A: Atlassian MCP, markdown

The default. Pass `contentFormat: "markdown"` and let the server build the ADF.

```
editJiraIssue({
  cloudId: <from getAccessibleAtlassianResources>,
  issueIdOrKey: "WMP-837",
  contentFormat: "markdown",
  fields: { description: "<the drafted markdown>" }
})
```

The comment is the same shape:

```
addCommentToJiraIssue({
  cloudId: ...,
  issueIdOrKey: "WMP-837",
  contentFormat: "markdown",
  commentBody: "<the drafted markdown>"
})
```

Two things to keep out of the markdown regardless of path:

- **The `summary` field is plain text.** It has no ADF and no formatting. Never send markdown to
  it. Backticks and asterisks in a summary render as backticks and asterisks forever.
- **No hard wrapping.** One line per paragraph and one line per list item, however long it runs.
  `writing-work-docs` already requires this. It matters more here than anywhere, because a hard
  wrap inside a paragraph can survive conversion as a real line break and arrive as ragged text.

### What to keep the markdown to

Stay inside the constructs that convert cleanly and there is nothing to debug:

- `##` and `###` headings. Start at `##`. An `h1` renders enormous next to Jira's own page title.
- Paragraphs, `**strong**`, `*em*`, `` `code` ``.
- Flat `-` bullet lists and `1.` ordered lists.
- Fenced code blocks with a language tag.
- Inline links as `[text](url)`.

Avoid these, and restructure rather than send them. Whether each survives depends on the
converter version, which means it is not something to rely on:

- **Markdown tables.** The highest-risk construct. A dropped or flattened table is the most
  common way a ticket arrives mangled. Use a bullet list per row, or build the table in ADF.
- **Nested lists.** One level is usually fine, deeper is not. Flatten, or make the sub-points
  their own sentence.
- **Task lists** (`- [ ]`), footnotes, reference-style links, raw HTML, and anything else outside
  the list above.

Restructuring is cheap here. A Jira ticket needs almost none of this to read well.

## Path B: acli, hand-built ADF

When only `acli` is available, build the ADF yourself and write it from a file. There is no
markdown mode to fall back on.

```
acli jira workitem edit --key WMP-837 --description-file /tmp/claude/work-on-WMP-837-new.adf.json --yes
acli jira workitem comment create --key WMP-837 --body-file /tmp/claude/work-on-WMP-837-comment.adf.json
```

`acli jira workitem edit --generate-json` prints a skeleton you can fill and pass back with
`--from-json`, which is worth using for a multi-field edit.

### ADF shape

The document wrapper:

```json
{ "version": 1, "type": "doc", "content": [ ... ] }
```

The nodes a ticket needs:

```json
{ "type": "heading", "attrs": { "level": 2 }, "content": [{ "type": "text", "text": "Problem" }] }

{ "type": "paragraph", "content": [
  { "type": "text", "text": "Tax is added twice in " },
  { "type": "text", "text": "applyTax()", "marks": [{ "type": "code" }] },
  { "type": "text", "text": "." }
] }

{ "type": "bulletList", "content": [
  { "type": "listItem", "content": [
    { "type": "paragraph", "content": [{ "type": "text", "text": "First point" }] }
  ] }
] }

{ "type": "codeBlock", "attrs": { "language": "typescript" },
  "content": [{ "type": "text", "text": "const total = net + tax + tax;" }] }

{ "type": "panel", "attrs": { "panelType": "info" }, "content": [
  { "type": "paragraph", "content": [{ "type": "text", "text": "Validated against a1b2c3d." }] }
] }
```

Marks go in a `marks` array on a `text` node. The useful ones are `strong`, `em`, `code`,
`strike`, and `link` (`{"type":"link","attrs":{"href":"..."}}`).

Four rules that account for most rejected documents:

- Top-level `content` holds **block** nodes only. A bare `text` node at the top level is invalid.
- `listItem` content is block nodes, normally a `paragraph`. Never raw `text`.
- `tableCell` and `tableHeader` content is block nodes too, again normally a `paragraph`.
- `codeBlock` holds one plain `text` node. Marks inside it are invalid.

A nested list goes inside the parent `listItem`, as a sibling after its `paragraph`.

## Verify by reading it back

Do this after every write, on both paths. The converter is not the authority on whether the
conversion worked, and neither is anything written above. The stored document is.

```
getJiraIssue({ cloudId: ..., issueIdOrKey: "WMP-837",
               fields: ["description", "comment"], responseContentFormat: "adf" })
```

`acli` equivalent: `acli jira workitem view WMP-837 --fields '*all' --json`.

**Check 1: no markdown syntax survived as text.** Walk every `text` node in the returned tree
and look for these. A hit means that construct did not convert and is now visible on the ticket.

**Skip the contents of `codeBlock` nodes entirely, and skip any text carrying the `code` mark.** Inside
a code block these patterns are the payload, not a failure. A shell comment starts with `#`, a diff
line starts with `- `, and a quoted markdown table is all pipes, and every one of them is exactly what
a correctly-converted code block should contain. Checking them guarantees a false positive on the
constructs this same file recommends using, and the remediation below opens with restoring the
description, so a false positive here destroys a good rewrite. Nobody previewed either version, so
there is no human who would recognize what went missing. Scan the prose nodes, never the code.

| Pattern in a text node | What failed |
|---|---|
| `#` at the start | heading |
| `**` or `__` | bold |
| Triple backtick | code fence |
| A line starting `- `, `* `, or `1. ` | list |
| A line starting and ending with `\|` | table |
| `[...](...)` | link |
| `- [ ]` or `- [x]` | task list |

**Check 2: the structure is actually there.** Count nodes by type and compare against what you
sent. Headings sent versus `heading` nodes stored. Code fences sent versus `codeBlock` nodes.
Lists sent versus `bulletList` and `orderedList`. Tables sent versus `table`.

Check 1 catches the loud failure. Check 2 catches the quiet one, where a construct is silently
dropped rather than flattened into text, so nothing looks wrong and content is simply gone.

**Check 3: read the rendered text.** Extract the text content in order and read it as prose.
Confirm the sections are in the order you sent and no paragraph was truncated or merged into
its neighbor. Compare against the markdown you handed the write call, which is the only reference
point there is.

### If the read-back call itself fails

Separate this from a failed check before doing anything. A check that ran and found mangled content
tells you the write landed badly. A read that errored, timed out, or was denied tells you nothing at
all about the write, and the two need opposite responses.

1. Retry the read once. A transient error is the common case.
2. If it fails again, **stop and say the write is unverified.** Report it in those words, with
   whether the write call itself returned success. "Wrote the description, could not read it back to
   verify" is the honest sentence.
3. **Do not restore, and do not rewrite with hand-built ADF.** Both remediations assume the content
   is known bad. Applying them to a ticket that may be perfectly fine can replace good content with
   a second guess, and the restore would silently discard the rewrite. Nobody has read either
   version, since this skill previews neither, so a wrong restore here is unrecoverable in practice
   even though the artifact still exists. That makes the ban stronger, not weaker.
4. Leave the rollback artifact in place and tell the user where it is, so they can compare or revert
   by hand.

Unverified is its own outcome. Never fold it into either success or failure, which is the same rule
this skill applies to an unreachable Datadog, Amplitude, or warehouse.

### If a check fails

The read succeeded and the content is wrong. Both writes reported success, so nothing failed as a
call. Which write produced the bad content decides the remediation, so check that first.

**The description.**

0. Confirm the hit is real before touching anything. Re-read the node the check flagged and satisfy
   yourself it is prose rather than code-block content or `code`-marked text. Restoring is destructive
   and it discards the rewrite, so it is not the right response to a maybe. Weigh it knowing no human
   has seen the rewrite it would discard. If you cannot tell, treat it the same way as a read-back
   that could not run: report it unverified and stop.
1. Restore it from `/tmp/claude/work-on-<KEY>-original.adf.json`, which Step 1 saved before any
   write. Do this first. Leaving a mangled description live while you debug is worse than reverting
   and retrying.
2. Rewrite using hand-built ADF (Path B) for the constructs that failed. Do not re-send the same
   markdown expecting a different result.
3. Verify again.
4. If the second attempt also fails, stop and show the user the stored ADF alongside the intended
   markdown. Do not keep retrying against a converter that is not doing what you expect.

**The comment.** There is no saved original, because a new comment had no prior state. So the
description's restore-first sequence does not apply and following it would revert an untouched
description over a comment problem.

1. Edit the comment in place instead. `editJiraIssue`'s sibling takes a `commentId`
   (`addCommentToJiraIssue` with the existing `commentId` updates rather than adds), and `acli` has
   `jira workitem comment update`. Rewrite it with hand-built ADF.
2. If the edit will not take, delete the comment and post a clean one. `acli jira workitem comment
   delete` does this. A garbled comment left in place is worse than a missing one, because the next
   reader treats it as content.
3. Never leave a half-converted comment live while you move on.

Report the verification outcome in one line either way. "Wrote and verified: 5 headings, 2 code
blocks, 3 lists, no literal markdown" is enough, and its absence is how a silent formatting
failure reaches the user.
