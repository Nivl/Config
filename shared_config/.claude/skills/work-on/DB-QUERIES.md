# Warehouse queries

How a skill gets a number out of the data warehouse. `work-on` reads it from SKILL.md's "Earn every
question" and from the validation probes. `open-ticket` reads it from Step 4, for the Stats block's
impact count. Both follow it as written, and the only thing that differs between them is the name of
the handoff file.

**Neither skill has a warehouse connection of its own.** Each writes SQL and then needs someone or
something else to run it. Two paths, in this order:

1. **A skill that runs SQL**, if this environment has one that works. Check before asking.
2. **The user**, otherwise. Pool the queries into one file, hand over the path, and keep asking until
   it resolves.

Path 2 is the reliable one, so it gets most of this document. Do not treat it as a failure state. An
agent composing SQL against production tables it has never seen, with no view of their size, is a
good thing to have a human read first.

- [Try to run them first](#try-to-run-them-first). Check for a query skill before asking.
- [Every query obeys these two rules](#every-query-obeys-these-two-rules). Read-only, and bounded.
- [Handing the work over](#handing-the-work-over). Pool the queries, give one path.
- [The handoff file](#the-handoff-file). Exact format.
- [Keep asking](#keep-asking). The part that actually matters.
- [Never](#never)

## Try to run them first

**Only the main thread does this, and only once**, after the phase that needs the numbers has pooled
what it wants. A subagent never runs SQL, whatever it finds available. A wide run dispatches twenty
or so agents, and each one probing the same skill produces the same answer that many times. A subagent
that hits a failure also cannot ask the user what to do about it, so the decision has to sit where the
user is reachable.

**Read the skills already listed in your context.** Judge each one by what it declares it does, not by
its name. A skill that runs a query against a database or warehouse and returns rows is a candidate. A
skill that reads dashboards, charts, saved reports, or metrics is not, because those answer a
pre-shaped question and cannot run arbitrary SQL.

This costs one read of a list you already have. Do not search the filesystem for skills, do not go
hunting for one that is not listed, and do not install anything.

**If a candidate exists, try the first query through it.** Then:

- **Rows come back.** Run the rest through it too. Record each result and cite the query as the
  locator for any claim built on it.
- **It fails on its environment.** Missing credentials, a connection it cannot open, a blocked host, a
  missing dependency. Stop. Hand every query over per [Handing the work over](#handing-the-work-over).
  Do not diagnose it, do not retry with different arguments, and do not try to fix its configuration.
  It will fail the same way for every remaining query, so more attempts only delay the handoff the
  user is already waiting on.
- **It refuses your SQL.** A guard naming your query, such as a read-only check or a statement it will
  not accept, is yours to fix. Fix it once and resubmit. If it refuses again, hand the queries over and
  say what the guard objected to. Never route a refused query into the handoff file unchanged, because
  that hands a human a query a tool already rejected.

**A partial run is normal.** If three of five queries return rows and the fourth fails on its
environment, keep the three results and hand over the remaining two. Never re-ask for a number you
already have.

**Say which path you took**, in one line, when you report results. A number's provenance is part of
the number, and "the warehouse says 4,812" reads very differently depending on whether a human ran it.

## Every query obeys these two rules

Both apply on both paths, and assume nothing checks them for you. A query skill may enforce
read-only, or it may not, and no skill knows how big your tables are.

The bound matters **more** on the skill path, not less. When a human runs the file, they read the query
first and that is a real control. A skill just runs it. So the one time an unbounded scan is cheapest
to catch is also the one time nobody is looking.

This warehouse is shared production infrastructure and the query you write was composed by an agent
with no view of table sizes.

**Read-only. `SELECT` and nothing else.** No `INSERT`, `UPDATE`, `DELETE`, `MERGE`, `TRUNCATE`,
`DROP`, `ALTER`, `CREATE`, or `GRANT`. This connection exists to answer a question about a ticket. If
a ticket seems to need a data change, that is the ticket's own work, done through the product's normal
path with review, never through a validation probe.

**Bound the range you scan.** Every query carries a date filter, an id range, or a `LIMIT` before it
runs. An aggregate is not exempt, and it is the easiest place to forget: `max()`, `count()`, and
`min()` over an unfiltered table read every row. Pick the bound from the question. A ticket filed
three months ago rarely needs more than six months of history.

When you genuinely cannot bound a query and still answer the question, say so in the handoff file and
let the user decide whether to run it. An unbounded scan a human chose to run is a different thing
from one an agent issued on its own.

## Handing the work over

Once no query skill is available or one has failed on its environment, this is the path. It is also
where you start if the skills list has no candidate at all, which is the common case.

**A skill is the only sanctioned alternative to asking.** Do not substitute anything else for it. No
database client, no `psql` on the path, no warehouse CLI, no connection string, no credentials out of a
file or an environment variable, and never a request that the user paste a token. Above all do not
guess the number the query would have returned, and do not reason your way to a figure and then present
it as measured.

Hand the work over instead:

1. Pool **every** query the run needs into one file. Not one file per query, and not a query at a
   time. The user is going to run these in one sitting and switching context per query wastes their
   time.
2. Write it to `/tmp/claude/work-on-<KEY>-queries.sql`, or to
   `/tmp/claude/open-ticket-<slug>-queries.sql` when `open-ticket` is the caller. There is no Jira key
   on that path, because the ticket does not exist yet, so the file takes the run's slug the same way
   its plan file does.
3. Give them the **absolute path**, on its own line, in its own message.
4. Follow [Keep asking](#keep-asking) until it resolves.

**Collect before you ask.** One file at one moment beats three asks across three turns, so let the
phase that needs the numbers finish and pool what it wants. A question that arrives after the user
already ran the first batch costs them a second sitting.

**Only the queries that did not run.** If a query skill answered some of them, those results are done
and their queries stay out of the file. Keep the numbering contiguous in whatever you do write, so
`Q1` and `Q2` mean the first and second query in the file the user is looking at.

## The handoff file

The file has to be runnable as-is and readable by someone who has not been in this conversation.
Every query says what it asks and what its answer changes, because a result the user cannot
interpret is a result they will not bother pasting back.

```sql
-- work-on WMP-837: 2 queries. Paste each result back and I will finish validating the ticket.
-- Generated against HEAD a1b2c3d on branch wmp-837-invoice-tax.

-- ============================================================
-- Q1. How many invoices actually hit the double-tax path?
-- Why it matters: under ~50 this is a data fix, not a code fix, and the ticket's scope changes.
-- ============================================================
SELECT count(*) AS affected
FROM invoices
WHERE tax_applied_count > 1
  AND created_at >= '2026-05-01';

-- ============================================================
-- Q2. Did it stop when PR #812 merged on 2026-08-04?
-- Why it matters: if the last row predates that date, the ticket is already fixed and I stop here.
-- ============================================================
SELECT max(created_at) AS last_occurrence
FROM invoices
WHERE tax_applied_count > 1
  AND created_at >= '2026-05-01';
```

One block per query. Numbered, so the user can answer "Q2 returned null" without pasting
everything. Keep the stated count in the header in step with the number of blocks. No placeholders
and no `<fill this in>`. If a query needs a value you do not have, that is a question for the user,
not a hole in the file.

Note that Q2 carries the same date bound as Q1 even though the question is "when did it last
happen". Dropping the bound there would turn a targeted lookup into a scan of every invoice ever
written. The bound has to be wide enough to answer the question and no wider. If the answer comes
back at exactly the boundary, widen it and ask again rather than removing it.

## Keep asking

**This is the part that goes wrong.** A query request bundled with three other questions gets
skipped, and the run then either stalls or quietly proceeds on a number nobody has. Neither is
acceptable.

**Ask alone.** The file path goes in its own message with nothing else in it. No other questions, no
status update, no summary riding along. Bundling is the single reason these get missed.

**Re-ask every turn until it resolves.** At the top of each turn, check whether the last user
message delivered results or explicitly declined. If neither, ask again in one short line naming the
path. One line, not a repeat of the file.

> Still need those queries when you get a chance: `/tmp/claude/work-on-WMP-837-queries.sql`

**Assume they missed it, not that they refused.** Anything that is not a clear decline means it did
not land. All of these mean ask again:

- No mention of the queries at all
- "ok", "thanks", "sounds good", "got it"
- "later", "in a bit", "hold on"
- A reply that answers your other questions but not this one
- A new topic, a new instruction, a correction to something else

**Only three things end the loop:**

- **Results arrive.** Record them, and cite the query as the locator for any claim built on them.
- **An explicit decline.** "Don't run queries", "skip the SQL", "I don't have warehouse access", "no
  DB for this one". Stop asking, and mark every dependent claim unverified.
- **An explicit proceed-without.** Continue, and carry each dependent claim into the ticket as a
  `TODO(user):` line rather than as a fact. Add each one to the running list the calling skill's final
  report reads out, with its query, so the user ends the run holding the exact SQL nobody ran.

These two endings are the only things that turn a number into a `TODO(user):` line. Reaching for that
line before the ask happened ships a claim nobody tried to check, wearing the shape of one somebody
tried and could not.

Silence is never a decline. Neither is impatience with a different part of the run.

**Never block on it.** Do everything that does not need the numbers while you wait. Read the code,
diff the merged commits, check the open PRs, draft the parts of the ticket that do not depend on a
count. The nag rides along on turns you are already taking, which is what keeps it from being
annoying. A run that sits idle waiting for SQL is a worse failure than a run that asks twice.

**Do not soften it into nothing.** "Let me know if you get a chance to run those" invites being
ignored. Say what is blocked and what it is blocking.

## Never

- Never write anything but a `SELECT`. No statement that mutates data or schema, ever.
- Never write a query with no bound on the range it scans, aggregates included.
- Never guess, estimate, or infer a number a query would have returned, and never present a
  reasoned-out figure as if it came from the warehouse.
- Never reach for raw database access. No database client, no connection string, no credentials out of
  a file or the environment. An installed skill that declares it runs SQL is the one sanctioned way to
  run a query yourself. Where there is none, asking is the only other way.
- Never retry a query skill that failed on its environment, and never try to repair its configuration.
  Hand the queries over instead.
- Never ask the user to paste a credential, token, or connection string into the chat. If one
  appears anyway, tell them to rotate it and do not reuse or repeat it.
- Never treat an empty result set as a missing result, or a missing result as an empty set. "Nobody
  is affected" and "nobody ran the query" support opposite conclusions.
- Never let a query the user declined silently become a confirmed claim in the ticket.
- Never let a number reach a ticket as a `TODO(user):` line without the ask having happened. The line
  records that somebody was asked and did not run it, so writing it unasked makes a false statement
  about what the run did.
