---
name: writing-work-docs
description: Use whenever writing or rewriting prose that a human will read. Covers Jira tickets, Confluence pages, PR descriptions, commit messages, Slack and email updates, design docs, RFCs, runbooks, postmortems, READMEs, and release notes. Triggers on "write", "draft", "write up", "fill out", "summarize this", "turn this into a ticket", "rewrite", and "this reads badly". Does not apply to code or code comments.
---

# Writing Work Docs

Everything below governs prose a colleague will read, whatever it lands in.

## Voice

No antithesis. No corrective negation. No paragraph pinning. No parataxis. No summary beats. No rhetorical crutches. No negative parallelisms. No negative anaphoras. No contrasting pairs. No rule of three. No em dashes.
Plain ASCII punctuation only. Write `->` not `→`, `<-` not `←`, `...` not `…`, `>=` and `<=` not `≥` and `≤`, `x` not `×`, and straight quotes `'` `"` not the curly ones. Most people cannot type the fancy glyphs, so they read as machine-written. This governs prose you author, not literal content you are reproducing. Quoted command output, an identifier from the code, a path, a URL, and a name that genuinely contains one all stay verbatim.
**Do not join two clauses with punctuation at all.** Banning the em dash and then reaching for its ASCII stand-in ` - ` (space-hyphen-space) changes the glyph and keeps the shape. A `:` that splits a claim from its elaboration is the same move again. Write two sentences.
- Bad: `The read-only guard is a floor - it stops a mistake, not a determined author.`
- Bad: `The read-only guard is a floor: it stops a mistake, not a determined author.`
- Good: `The read-only guard is a floor. It stops a mistake, not a determined author.`
This bans `-` and `:` as clause JOINERS only. Hyphenated words (`read-only`), CLI flags (`-c`), ranges (`1-10`), a label at the start of a line (`Note:`, `Impact:`), times and ratios (`12:00`, `3:1`), and anything inside code, a path, or a URL are all untouched.
No throat-clearing openers. No landing sentences.
No setup/payoff constructions. No parallel sentence structures within a paragraph. Vary sentence length unpredictably. No stacked noun phrases. No filler intensifiers (genuinely, really, truly, actually). No corporate-register verbs (leverage, underscore, reflect). No nominalization.
No hedging qualifiers. Write for the spoken voice.
No performed enthusiasm.

### Catching yourself

The labels above are easy to agree with and easy to violate anyway. These are
the shapes that slip through:

- Two adjacent sentences built the same way, the second inverting the first.
  `A visible gap is useful. A confident fabrication is a landmine.`
  Delete the second one. It carries no fact.
- A closing sentence that adds no information and only assigns weight. If a
  sentence can go without losing a fact, it goes.
- Anything hinged on `rather than`, `instead of`, `not X but Y`. State what you
  want and drop the foil.
- Three examples where one would do.

Mechanical check on the finished draft: read only the last sentence of each
paragraph, ignoring the rest. If it restates what came before it, cut it.

## Banned words

The user does not talk this way. Using one of these is a tell that the draft came from a model, so it fails the doc no matter how accurate the sentence is. Find a plainer word or restructure the sentence.

- outright
- crucial, pivotal, vital, paramount
- comprehensive, holistic
- intricate, nuanced
- meticulous
- granular
- utilize
- facilitate
- foster, cultivate
- harness, unlock, empower, elevate
- streamline
- showcase
- surface
- socialize, operationalize, ideate
- moreover, furthermore, additionally
- notably, importantly, crucially
- essentially, fundamentally, ultimately, arguably
- landscape, realm, testament
- myriad, plethora
- journey, synergy
- game-changer
- delve, dive into
- unpack
- circle back
- robust, powerful, seamless, seamlessly
- ensure, enhance
- cutting-edge

Add to this list whenever the user flags a word. Do not treat it as exhaustive: any word you would not hear the user say out loud belongs on it.

## Hard rules

**Invent nothing.** Ticket IDs, dates, owners, metrics, dashboard links, test
results, root causes. Anything you cannot verify becomes a `TODO(user):` line
in the draft.

**Follow the artifact's own template.** Read it from the repo or the existing
page. Use its headings and add none. Do not reorder, rename, merge, or drop one either, including a
section the work does not touch. "Add none" is only half the rule, and dropping an inapplicable
section is the half people get wrong: a repo's reviewers key on seeing `Risk` or `Rollback` present
and answered, so a missing heading reads as an oversight rather than as a considered `N/A`. Fill every
section the template has, in the order it has them. Leave its HTML comment blocks in place.
`N/A` is the right answer for a section the work does not touch. Never write a
paragraph to fill a heading. Leave checkboxes unchecked when the work is not done.

**Match length to substance.** A rollout percentage bump needs two sentences.
Spend a full paragraph only where the reader needs the mechanism explained.

**Every claim is checkable.** Name the env, the numbered steps, the dashboard
with its link, the test file. "Tested thoroughly" says nothing.

**Identifiers are exact.** Real paths, function names, flag names, resource
names, in backticks. Copy them from the source.

**No hard wrapping in the artifact you produce.** One line per paragraph, one line per list item, however long that runs. Never wrap prose to 80 columns or any other width. This governs the draft you hand over, not this file: the rules below are wrapped for reading and that is not the shape to copy. Hard wraps make a one-word edit touch four lines of diff, they fight every editor's own soft wrap, and they survive the paste into Jira, Confluence, and Slack as ragged line breaks. Code blocks and tables keep their own line structure.

**Do not narrate the diff in prose** ("adds a function `foo` that loops over the
items and ..."). The diff already shows that. Explain why the change was needed
and what breaks without it.

**Draft only.** Produce paste-ready text and stop. See Publishing.

## Process

1. Gather ground truth before writing a word. Sources below. If the user pasted
   rough notes, those notes outrank everything you find on your own. Sharpen them.
2. List what you could not find. Each becomes a `TODO(user):`.
3. Draft.
4. Reread the draft against the Voice section and cut. This is a separate pass.
   Do it every time. Report in one line what you cut.
5. Deliver as a fenced block ready to paste, with the `TODO(user):` lines
   called out separately.

## Ground truth

Look these up instead of asking.

Two things the commands below get right and are easy to get wrong. Resolve the default branch rather
than hardcoding `main`, since a repo on `master` or `develop` gives `fatal: ambiguous argument`. And
compare against `origin/<default>`, not the local ref, because a local branch can be weeks behind and
the diff would then include work that already merged upstream.

| Need | Source |
|---|---|
| Default branch | `git remote show origin`, read the `HEAD branch` line. Do not assume `main`. |
| What changed | `git diff origin/<default>...HEAD` (three dots), `git log origin/<default>..HEAD --format='%s%n%b'` (two dots) |
| PR template | `.github/PULL_REQUEST_TEMPLATE.md`, differs per repo, read it every time |
| Repo conventions | `CLAUDE.md`, `CONTRIBUTING.md`, `docs/` |
| Existing ticket | `acli jira workitem view KEY-123`, add `--json` to inspect fields |
| Local convention | `git log --format='%s%n%b' -20`, or `gh pr list --state merged --limit 10` |
| Ticket ID | Branch name (`wmp-837-...`) or the user. Never invent one. |

## Rewriting existing text

Preserve every fact and every link, then cut. Say what you removed. Keep facts
that read awkwardly. Flag anything that looks wrong so the user can decide.

If the existing text is hard-wrapped, unwrap it. When reflowing a whole file mechanically, verify the word stream is unchanged before and after, so the reflow cannot have eaten a word. Normalize each version to one word per line and compare:

```
tr -s '[:space:]' '\n' < before.md > /tmp/claude/words-before.txt
tr -s '[:space:]' '\n' < after.md > /tmp/claude/words-after.txt
diff /tmp/claude/words-before.txt /tmp/claude/words-after.txt
```

Empty `diff` output means no word was added, dropped, or altered. Plain commands and redirects on purpose. An inline `python -c` or `node -e` one-liner is the obvious way to write this check, and this environment denies both, so the check would go unrun.

Check tables, fenced code blocks, and headings came through untouched. `diff` above will not catch those, since reordering words within a table still preserves the word stream.

## Publishing

Never publish anything. Produce the text and the user will be responsible for publishing.

This covers writes only: `gh pr create`, `gh pr edit`, `acli jira workitem create`,
`acli jira workitem edit`, Confluence write APIs. Reading is expected. Every
command in Ground truth is a read.