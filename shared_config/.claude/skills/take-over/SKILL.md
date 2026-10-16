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
whether the current branch matches the key, and the lookup below uses that answer. Run these two,
each alone:

```
gh pr list --state open --search <KEY> --limit 100 --json number,title,url,headRefName,author,isDraft,isCrossRepository > /tmp/claude/take-over-<KEY>-prs.json
gh pr list --state open --limit 300 --json number,title,url,headRefName,author,isDraft,isCrossRepository > /tmp/claude/take-over-<KEY>-open.json
```

`--search` does not match branch names, so the second call is what finds a PR whose key sits in
its branch name alone. Union the two by `number`. When either returns its limit, say the sweep may
be truncated, since the takeover PR could sit past it. Filter the union locally. Keep an entry when its
`headRefName` or its `title` matches `(?i)(?<![a-z0-9])<KEY>(?![0-9])`, the same match `work-on`'s item 2
uses. Then:

- **Zero entries.** Stop. Say no open PR carries the key and suggest `/work-on <KEY>`.
- **One entry.** That is the takeover PR.
- **Several entries.** List them with number, author, branch and title, and ask which one with
  `AskUserQuestion`. This is a decision, so it is asked on a flag run too.
- **The current branch matches the key.** The takeover PR is the entry whose `headRefName` is the
  current branch. When none is and other entries matched, stop. List those entries with number,
  author and branch, say the current branch is not the head of any of them, and tell the user to
  switch to that PR's branch, or to run from a checkout whose branch does not carry the key so the
  worktree preflight opens it. Do not suggest `/work-on <KEY>`, since a PR already carries the
  key. When no entry matched at all, treat it as zero entries. When other entries match beside the
  current branch's own PR, still name them in one line, so the user knows more than one PR is open
  for this ticket.
- **`isCrossRepository` is true on the chosen PR.** Stop. A fork's branch is not one this run can
  push to, and that case is out of scope.

Record `<PR>` (its number), `<HEAD>` (its `headRefName`) and `<AUTHOR>` (its author's login).

**The branch.** When the current branch matches the key, it must be `<HEAD>`, per the rule above.
When it does not match, run `work-on`'s "Worktree preflight" with `<HEAD>` as a tracking target,
in place of `work-on`'s item 6b. Then run "Branch sync" below on both paths.

**Under `--assess`, open no worktree and sync nothing.** That mode writes no code, so it reads the
teammate's branch from the remote ref instead. Phase A judges against `origin/<default>` already,
and B2 reads `origin/<default>...origin/<HEAD>`. The run stays in the current checkout, and the
report says no branch or worktree was created.

### Branch sync

Both paths end here, so one invariant holds before Phase A. Local `<HEAD>` tracks
`origin/<HEAD>`, has no commits the remote lacks, and is at the remote's tip. The lease in B1 only
protects against pushes after the fetch. It does not protect against a local branch that was
never at the tip, so this check is what keeps a force-push from erasing the teammate's newest
commits.

1. `git fetch origin <HEAD>`. This is the last fetch before B1.
2. `git rev-parse --abbrev-ref <HEAD>@{upstream}`. When it is not `origin/<HEAD>`, fix it with
   `git branch --set-upstream-to=origin/<HEAD> <HEAD>`.
3. `git rev-list origin/<HEAD>..<HEAD>`. A non-empty list is commits that are neither the
   teammate's nor this run's. Stop, list them, and let the user decide.
4. `git rev-list <HEAD>..origin/<HEAD>`. A non-empty list means the local branch is behind. Run
   `git merge --ff-only origin/<HEAD>` and say how many commits it brought in.
5. Record `<LEASE_SHA>` as `git rev-parse origin/<HEAD>`, and confirm `git rev-parse HEAD` prints
   the same sha. Phase B's force-push compares against it, so a later fetch cannot move it.

## Phase A: validate the ticket

`work-on`'s Steps 1 to 5, with these deltas.

**Step 1.** Drop `<PR>` from the open-PR sweep before it becomes `open_prs`, and add a
`coverage_notes` entry saying `PR #<PR> excluded as the takeover PR`. Other PRs that match the key
stay in, since they are what the Step 0 warning is about. Set `args.takeover` to
`{ "pr": <PR>, "head": "<HEAD>", "author": "<AUTHOR>", "baseline": "origin/<default>" }`.
`VALIDATION.md` describes what it does to the lenses. Under `--assess` the run is still in the
original checkout, so every range Step 1 builds from `HEAD` uses `origin/<HEAD>` in its place.

**Step 2.** The inline path, when the surface is small enough that `work-on` skips the workflow,
follows the same rule the `TAKEOVER` block gives the lenses. Judge the ticket against
`origin/<default>` with `git show origin/<default>:<path>`, and never treat the branch's commits
as evidence the ticket is satisfied.

**Step 4.** A dead verdict, for any reason, touches no part of the PR. No comment, no close, no
edit. The ending names `<PR>` and `<AUTHOR>` and recommends what to do with the PR, and the user
decides. Phase B does not run on a dead verdict, since there is no work left to check. Under
`--assess` the Jira comment then posts here, as `work-on` posts it, carrying the ticket verdict and
naming `<PR>`. It does not wait for Phase B.

**Step 5.**

- **Assignee.** Assign the ticket to the user, as `work-on` does. It usually moves the ticket off
  `<AUTHOR>`'s name, which is what a takeover means.
- **Status.** No status move. The ticket keeps whatever status it has, In Review included.
- **Sprint.** As `work-on` does.
- The description rewrite and the validation comment run as in `work-on`.
- Under `--assess` there is no board write, as in `work-on`. On a live verdict the comment waits
  for Phase B's check so it carries both verdicts.

## Phase B: bring the branch up to date, check it, build

This phase replaces the start of `work-on`'s Step 6. `work-on`'s Constraints say never force-push
and never rewrite pushed history. **B1 overrides that rule, here and nowhere else.** The PR is the
user's now, and they asked for a rebase.

### B1. Rebase onto the default branch

Done before the check, so the check reads the code as it will merge.

1. `git rebase origin/<default>`. Resolve each conflict and continue. Read both sides before
   resolving, since the default branch's side is code somebody already reviewed and merged.
2. **When the same conflicts keep coming back commit after commit**, so one merge is plainly
   easier than resolving each replayed commit, run `git rebase --abort`, then
   `git merge origin/<default>`, and resolve once.
3. **Push.** After a rebase, `git push --force-with-lease=<HEAD>:<LEASE_SHA> origin <HEAD>`. After
   a merge, `git push origin <HEAD>`.
4. **A rejected push stops the run**, a lease rejection after a rebase or a non-fast-forward
   rejection after a merge alike. Both mean `<AUTHOR>` or someone else pushed after the fetch. Say
   so, name `<LEASE_SHA>` and the sha the remote now has, and stop. Do not refetch and retry on
   your own, and never fall back to a plain `--force`, since either would overwrite whatever they
   pushed.

Record which one happened, `rebase` or `merge`, and how many conflicts were resolved. Phase C's
report reads both.

**Under `--assess`, B1 does not run.** That mode writes nothing to the branch. Record
`not rebased`, and B2 reads `origin/<default>...origin/<HEAD>`, the branch as it is on the remote.

### B2. Check the implementation, strictly

Launch one fresh `general-purpose` subagent, from the main thread. A fresh reader does not anchor on
the validation context, and the full diff stays out of the main thread. Its brief carries:

- the ticket as validated, or on `--no-assess` and `--fast` the ticket as filed, said plainly as
  unvalidated,
- the scope Step 4 agreed, when there is one,
- the repo's `AGENTS.md` and `CLAUDE.md` paths,
- the range `origin/<default>...HEAD`, or `origin/<default>...origin/<HEAD>` under `--assess`,
  to read with `git diff` and `git show`,
- the instruction to be strict. The question is whether the code does what the ticket asks,
  correctly and completely, not whether it roughly does the thing.

It returns JSON with five buckets:

| Bucket | Holds |
|---|---|
| `done` | parts of the ticket the diff implements correctly, each with a locator |
| `wrong` | behavior that does not match the ticket, bugs, and missing edge cases inside the scope |
| `missing` | parts of the ticket the diff does not implement |
| `approach` | `sound` or `structural`, with a reason. `structural` means patching it will keep producing findings |
| `beyond_scope` | code the ticket does not ask for, or code outside the Step 4 scope, each marked `keep` or `split` with a reason |

For `beyond_scope` the user's rule is to keep it when it makes sense and split it into a follow-up
otherwise.

### B3. Patch or rewrite, and the gate

When `wrong`, `missing` and `beyond_scope` are all empty and `approach` is `sound`, the pick is
`none`. Say so and go to Phase C.

Otherwise pick `patch` or `rewrite`, whichever is cheaper, with a one-line reason. **Cost includes
the review iterations a patch would leave behind**, not just the lines changed today. A
`structural` approach leans to `rewrite`, since patching one is the scope-creep path this skill
exists to avoid.

**Before the first edit, put the pick to the user in one `AskUserQuestion`**, with two options,
approve and change. The question carries the pick, its reason, and the `wrong`, `missing` and
`beyond_scope` lists. Ask it whichever process skill `work-on`'s Step 6 picks. On the brainstorming
path it is that skill's design gate, not a second question. On the systematic-debugging path,
`work-on` has no gate before the first edit, and a rewrite would otherwise discard a teammate's
work unseen.

That answer also approves each `split` item for filing. That holds on `--no-assess` too, where
`work-on`'s Step 8 files nothing because no scope was agreed, since this question is a human
agreeing to each item by name.

Under `--assess`, no question is asked and nothing is built. The pick and the five buckets go into
the Jira comment.

### B4. Build

Run `work-on`'s Steps 6 and 7 with the approved pick.

- **Everything in `wrong` and `missing` is fixed in this build**, in one go, before the review loop.
- **`split` items are removed from the branch here** and filed through `work-on`'s "Filing the
  follow-up", one call per item, with this run's ticket key as the originating ticket.
- **`keep` items stay**, and Phase C lists them in the PR description.
- **A rewrite lands as new commits on `<HEAD>`**, removing or replacing the teammate's code.
  Never reset the branch. B1's push is the run's one force-push, and Step 7 pushes plainly, so a
  reset would leave a branch that cannot be pushed without a second, unguarded force-push.

## Phase C: review, the PR, the report

### Step 8

Run `work-on`'s Step 8 as written. That invokes `review-and-fix --branch`, plus `--review-limit`,
`--full` and `--ab-model` when they apply. `--branch` matters more here than in `work-on`, since a
PR is open and the review must still cover the branch range. Follow-ups from what the loop left
are sorted and filed as `work-on` does, against the Step 4 scope.

### Step 9, replaced. Update the existing PR

Do not invoke `open-pr`. The PR exists.

1. Push the review commits with `git push origin <HEAD>`.
2. Read the PR with `gh pr view <PR> --json title,body,isDraft > /tmp/claude/take-over-<KEY>-pr.json`.
3. Compare the title and body with the repo's PR template when one exists, `open-pr`'s title
   rules otherwise, and the branch diff. Rewrite either one only where it breaks those conventions
   or no longer describes the diff. Use `writing-work-docs` for the rewrite. The body links the
   ticket and lists each `keep` item from B2, so a reviewer reads them as deliberate.
4. When something changed, write the body to `/tmp/claude/take-over-<KEY>-pr-body.md` and run
   `gh pr edit <PR> --title "<title>" --body-file /tmp/claude/take-over-<KEY>-pr-body.md`. When
   nothing changed, make no edit.
5. **Never change the draft or ready state.** No `gh pr ready`, and no `--draft` on any command.
6. Read the PR back and confirm the edit landed, as `work-on` does after `open-pr`.

If the edit fails, the branch is already pushed. Say so, give the exact error and the drafted
title and body, and still emit the report.

### The `--assess` ending

Post one Jira comment carrying both verdicts: the ticket verdict from Phase A, then B2's buckets
and B3's pick. Edit no PR and push nothing. Settle the stash as `work-on` does on that ending.

### Final report

`work-on`'s "Final report", with its `PR` line naming the existing PR and saying `edited` or
`unchanged` instead of `(draft)`, plus these lines:

```
Takeover      PR #<PR> by <AUTHOR>, branch <HEAD>
Sync          rebase, <n> conflicts | merge, <n> conflicts | not rebased (--assess) | stopped on lease rejection
Check         done <n>, wrong <n>, missing <n>, approach <sound|structural>, beyond scope <n> (keep <n>, split <n>)
Pick          none | patch | rewrite: <reason>
PR text       title <changed|unchanged>, body <changed|unchanged>
```

