# Duplicate queries

Step 5 of [SKILL.md](SKILL.md). Run Q0 first and believe no negative verdict without it. An unknown JQL field name returns totalCount 0 with no error, while a syntax error is loud, so one mistyped field name makes this whole sweep report no duplicates and the skill then files the duplicate it exists to prevent.

## The quoting recipe

Every term goes inside escaped inner quotes, `text ~ "\"<TERM>\""`. Bare `~` is an order-independent AND over stemmed tokens, so a single term the ticket does not contain zeroes the whole query. The value is also a live Lucene query string, where uppercase `OR` and `AND` and a whitespace-preceded `-` change what the query means.

## The file-path caveat

A bare path over-matches badly. `description ~ "runwayer-plans.ts"` gave 31 hits against 1 for the quoted form, and the unquoted search returned a false positive for a path that does not exist. Never append `:LINE` to a path. The colon splits the term into two clauses, and the line number becomes its own AND condition that narrows the match away from the real ticket.

## Filters

No status filter and no date filter. `statusCategory = Done` was 65% of matches for the tested term, and `created > -180d` cut 29 matches to 8, a 72% recall loss. Rank with `ORDER BY updated DESC`. A status or date filter drops the exact tickets this sweep exists to find.

## Payload discipline

`searchJiraIssuesUsingJql` always returns a mandatory field floor of `assignee, description, issuetype, project, status, summary` plus whatever is requested, and `description` cannot be excluded from that floor. Three unguarded queries returned responses of 1,051,178, 760,498 and 925,249 characters. A payload that size stalls the run or exhausts the context before the sweep finishes. Use `searchResultMode: "count"` for every tally, and fetch fields only for the candidates that survive the count.

A field named `"sprint"` in the `fields` array is silently dropped, with no error. Only `customfield_10021` returns data. This is a second silent-failure mode, distinct from the silent JQL zero that Q0 exists to catch, and it reports an empty field instead of a missing one.

## The placeholders

Every placeholder in the queries below is filled from an earlier step. Leave one unfilled and the query searches for the literal text of the placeholder, which returns zero and reads exactly like a clean sweep.

| Placeholder | What goes in it |
|---|---|
| `<PROJECTS>` | A comma list of project keys. Step 2's majority project plus the runner-up it reports. |
| `<PATH_N>` | A repo-relative path from Step 4. Never with `:LINE` appended. |
| `<SYMBOL_1>` | A distinctive symbol from Step 4, a function or type name. |
| `<NOUN_N>` | A distinctive domain noun from the request. |
| `<KEY_1>` | An epic, parent or sibling key already known from the request or from an earlier query's hits. |

`<PROJECTS>` is a comma list and not a single key. Scope it to the majority project alone and a duplicate filed in the runner-up project goes unseen, and Q5 is the only query that would still catch it.

No step before Step 5 produces a candidate key, so `<KEY_1>` is often empty. Skip Q4 when no key is in hand. Run it with the placeholder still in it and it searches comments for that literal string, returns zero, and the zero then reads as Q4 having cleared.

## The six queries

Copied verbatim from the design spec's Appendix B. Run every query with `searchResultMode: "count"` first, then fetch fields only for the survivors.

**Q0, mandatory positive control, runs first.**

```
project in (<PROJECTS>) AND summary ~ "\"<NOUN_1>\""
```

Zero here means a mistyped field name or a wrong noun. Without it, the sweep's negative verdict cannot be trusted.

**Q1, identifier and path union. Highest precision.**

```
project in (<PROJECTS>) AND text ~ "\"<PATH_1>\" OR \"<PATH_2>\" OR \"<SYMBOL_1>\"" ORDER BY updated DESC
```

Finds the ticket naming the same code, whoever wrote it. Nothing else finds a duplicate worded completely differently.

**Q2, summary noun union. Highest signal per hit.**

```
project in (<PROJECTS>) AND summary ~ "\"<NOUN_1>\" OR \"<NOUN_2>\" OR \"<NOUN_3>\"" ORDER BY updated DESC
```

Finds the duplicate describing the same work without naming a file, which Q1 cannot see. Read these hits before Q1's, even when Q1 returns more of them.

**Q3, the already-done pass.**

```
project in (<PROJECTS>) AND statusCategory = Done AND text ~ "\"<NOUN_1>\" OR \"<PATH_1>\"" ORDER BY updated DESC
```

Redundant by set membership, and it still earns its place. Done is the majority of the corpus, so a closed duplicate sits deep in an unsegregated list.

**Q4, comment pointers.**

```
project in (<PROJECTS>) AND comment ~ "\"<NOUN_1>\" OR \"<PATH_1>\" OR \"<KEY_1>\"" ORDER BY updated DESC
```

Adds isolation, not coverage. "Superseded by", "already handled in" and "duplicate of" live in comment threads.

**Q5, loose stemmed net, unscoped by project.**

```
text ~ "<NOUN_1> <NOUN_2>" ORDER BY updated DESC
```

Deliberately unquoted, the only query here that is. Catches a duplicate sharing the concept but no exact string, including one filed outside `<PROJECTS>`. Keep it to two or three nouns, and read its hits with suspicion. This is the query that produced the false positive for a path that does not exist.

## Search tool limits

`maxResults` maximum is 100, and values below 50 are honored. Pagination is cursor-based, with `nextPageToken` from `issues.pageInfo.endCursor`, and the cursor embeds the JQL and the sort. `ORDER BY` is honored across pages. Rate limit is 300.

## The git half

Every command runs alone with no pipe and no chain, redirected to a file under `/tmp/claude/`, and read back in a separate call. `git` runs outside the sandbox, so a piped or chained git command is denied by a hook.

```
git log -G'<pattern>' --all --oneline > /tmp/claude/open-ticket-<slug>-git-code.txt
git log --grep='<noun>' --all --oneline > /tmp/claude/open-ticket-<slug>-git-msg.txt
git log --oneline --all -- <path> > /tmp/claude/open-ticket-<slug>-git-path.txt
```

Use `-G`, not `-S`. On the same string, `-S` found 4 commits and `-G` found 10. `-S` only fires when the occurrence count changes, so relying on it would have reported fewer than half the real hits. `--all` is not the default and has to be passed, or a commit on another branch goes unseen. `\s` under `-P` returns zero hits on git 2.50.1. Use POSIX character classes like `[[:space:]]`. A pattern written with `\s` reports a clean empty result when real matches exist.

Use `GRO-1234` in every example. GRO is not the only project key on this site, and `<PROJECTS>` is a list for exactly that reason. Do not copy an example key out of another skill's documentation. A key from a project this site cannot resolve scopes the sweep to a project that does not exist, so it reports a clean verdict it never earned.
