---
name: take-over
description: >
  Takes a Jira ticket key whose implementation already exists as someone else's open PR, and
  finishes that PR as its new owner. Validates the ticket the way work-on does, rebases the
  teammate's branch and force-pushes it, checks their implementation strictly against the
  ticket, patches or rewrites it, runs review-and-fix, files follow-ups, and updates the
  existing PR's title and description.
  Use it when the user says "/take-over GRO-123", "take over this PR", "finish <teammate>'s PR
  for GRO-123", or "this ticket has a PR, pick it up".
  Flags are work-on's: `--no-assess`, `--fast`, `--assess`, `--review-limit X`, `--full`,
  `--ab-model`. `--assess` also judges the existing implementation and changes no code.
  Do not use it for a ticket with no open PR (that is work-on), and do not use it only to answer
  review comments on a PR (that is fix-pr).
---

# Take Over

One Jira ticket whose implementation already exists as somebody else's open PR, from "is this
still true?" through to that PR, now the user's, reviewed and up to date.

**This skill is a set of deltas on `work-on`.** Before Step 0, read these files by path, in full:
`../work-on/SKILL.md`, `../work-on/VALIDATION.md`, `../work-on/JIRA-FORMAT.md` and
`../work-on/DB-QUERIES.md`. Never invoke `work-on` through the Skill tool, since that runs it
end to end and opens a new PR. A step this file does not name runs exactly as `work-on` writes
it. Where this file and `work-on` disagree, this file wins.

**The failure this skill is built around is scope creep through the review loop.** A run that keeps
the teammate's code mostly as it is hands `review-and-fix` a pile of implementation problems, and
each iteration fixes some and surfaces more, at real cost. So Phase B's check is strict and runs
before the loop, and everything it finds is fixed in one build. The loop is for polish.

## Argument

One positional argument, the ticket key, plus `work-on`'s flags. Strip and validate them exactly
as `work-on`'s "Argument" says, including every error combination it names. Each flag means what
it means there, with one difference:

- **`--assess` also judges the implementation.** It runs Phase A in full, then Phase B's check
  against the branch as it is, and posts one Jira comment carrying both verdicts. It rebases
  nothing, pushes nothing, edits no PR, and makes no board write.

`--no-assess` and `--fast` skip `work-on`'s Steps 2 to 4 as there. Phase B's check still runs on
those paths, against the ticket as filed, because it is the guard against the failure above.

The review is uncapped by default, as on a plain `work-on` run. `--no-assess` still defaults the
cap to 2, and the user's `--review-limit` wins either way.

## Step 0: Preflight and PR lookup

Run `work-on`'s Step 0 with these deltas.

**Find the PR after item 6, the GitHub access check, and before item 6b.** Item 2 still decides
whether the current branch matches the key, and the lookup below uses that answer. Run this alone:

```
gh pr list --state open --search <KEY> --json number,title,url,headRefName,author,isDraft,isCrossRepository > /tmp/claude/take-over-<KEY>-prs.json
```

`--search` does not match branch names, so filter the result locally. Keep an entry when its
`headRefName` or its `title` matches `(?i)<KEY>(?![0-9])`, the same match `work-on`'s item 2
uses. Then:

- **Zero entries.** Stop. Say no open PR carries the key and suggest `/work-on <KEY>`.
- **One entry.** That is the takeover PR.
- **Several entries.** List them with number, author, branch and title, and ask which one with
  `AskUserQuestion`. This is a decision, so it is asked on a flag run too.
- **The current branch matches the key.** The takeover PR is the entry whose `headRefName` is the
  current branch. When none is, treat it as zero entries. When other entries also match, still
  name them in one line, so the user knows more than one PR is open for this ticket.
- **`isCrossRepository` is true on the chosen PR.** Stop. A fork's branch is not one this run can
  push to, and that case is out of scope.

Record `<PR>` (its number), `<HEAD>` (its `headRefName`), `<AUTHOR>` (its author's login), and
`<LEASE_SHA>`, which is `git rev-parse origin/<HEAD>` right after the fetch. `<LEASE_SHA>` is what
Phase B's force-push compares against, so a later fetch cannot move it.

**The branch.** When the current branch matches the key, it must be `<HEAD>`, per the rule above.
When it does not match, run `work-on`'s "Worktree preflight" with `<HEAD>` as a tracking target,
in place of `work-on`'s item 6b. Every rule in that subsection applies, the extra-commits stop
included.

## Phase A: validate the ticket

`work-on`'s Steps 1 to 5, with these deltas.

**Step 1.** Drop `<PR>` from the open-PR sweep before it becomes `open_prs`, and add a
`coverage_notes` entry saying `PR #<PR> excluded as the takeover PR`. Other PRs that match the key
stay in, since they are what the Step 0 warning is about. Set `args.takeover` to
`{ "pr": <PR>, "head": "<HEAD>", "author": "<AUTHOR>", "baseline": "origin/<default>" }`.
`VALIDATION.md` describes what it does to the lenses.

**Step 2.** The inline path, when the surface is small enough that `work-on` skips the workflow,
follows the same rule the `TAKEOVER` block gives the lenses. Judge the ticket against
`origin/<default>` with `git show origin/<default>:<path>`, and never treat the branch's commits
as evidence the ticket is satisfied.

**Step 4.** A dead verdict, for any reason, touches no part of the PR. No comment, no close, no
edit. The ending names `<PR>` and `<AUTHOR>` and recommends what to do with the PR, and the user
decides.

**Step 5.**

- **Assignee.** Assign the ticket to the user, as `work-on` does. It usually moves the ticket off
  `<AUTHOR>`'s name, which is what a takeover means.
- **Status.** No status move. The ticket keeps whatever status it has, In Review included.
- **Sprint.** Add the ticket to the user's active sprint only when it sits in no active sprint.
  A ticket on any active sprint, the teammate's included, stays where it is.
- The description rewrite and the validation comment run as in `work-on`.
- Under `--assess` there is no board write, as in `work-on`, and the comment waits for Phase B's
  check so it carries both verdicts.
