# take-over: finish a teammate's open PR for a ticket

## Intent

`/take-over GRO-123` is `work-on` for a ticket whose implementation already exists. A teammate
opened a PR for it, and the user is taking that PR over completely. The PR is now the user's. The
skill validates the ticket the way `work-on` does, then judges the existing implementation against
it, fixes or rewrites what needs it, runs `review-and-fix`, files follow-ups, and updates the
existing PR.

The typical PR being taken over is not mergeable. It has not been reviewed, it is often behind
the default branch, and its implementation may not match the ticket.

## The failure this design is built around

The user named one outcome as the worst: **scope creep through the review loop.** The run keeps
the teammate's implementation mostly as is, then `review-and-fix` finds many implementation
problems, each iteration fixes some and surfaces more, and the run burns many iterations and a lot
of money circling a problem a single up-front decision would have handled.

So the implementation check (Phase B below) is strict and happens before the review loop, and
everything it finds is fixed in one build step. The review loop is meant for polish, not for
rebuilding.

## Goals

- `/take-over <KEY>` with the same flags as `work-on`: `--no-assess`, `--fast`, `--assess`,
  `--review-limit`, `--full`, `--ab-model`. Same meanings. `--assess` also judges the existing
  implementation (see Flags).
- Reuse `work-on`'s validation, ticket rewrite, review, follow-up and report steps rather than
  copying them.
- Give `work-on` the same worktree preflight `take-over` uses, as part of this work.

## Non-goals

- Splitting `work-on` into several separately invocable skills. See "How the reuse works".
- Handling human review comments as a separate step. `review-and-fix`'s roles cover the PR's code,
  and the user did not ask for a `fix-pr`-style pass.
- CI failures. Not discussed, so not in scope.
- Notifying the teammate. Not asked for. The Jira validation comment and the reassignment are the
  visible trail.

## Anti-goals

- Patching a structurally wrong implementation and leaving the review loop to discover it (the
  failure above).
- Overwriting work the teammate pushed after the run fetched. Force-pushes use
  `--force-with-lease`.
- Validation concluding the ticket is dead because the PR being taken over already does the work.

## Decisions the user made

- **Input:** `/take-over <jira-key>`. No PR number argument.
- **Branch:** if the current branch name contains the ticket key anywhere (case-insensitive), work
  there. Otherwise create a worktree with `wt`. The user wants to keep using `wt`.
- **`work-on` gets the same worktree step** in this same piece of work. Today `work-on` Step 0
  aborts on the default branch and never creates a branch.
- **Ownership:** a full takeover. Push to the teammate's branch and PR directly.
- **Multiple open PRs** for the ticket: warn, so the run is sure it is on the right one.
- **Their code:** do whichever is easier. Update it when that is cheaper. Rewrite it when starting
  over makes more sense.
- **Behind the default branch:** rebase and force-push ("it's my PR now"). If merging the default
  branch in is easier than the rebase, merge instead. Otherwise the agent does its best to resolve
  conflicts itself.
- **Review:** always branch-level, since the whole PR diff is local. Forcing `work-on` into branch
  mode too is fine.
- **Review cap:** uncapped by default, like plain `work-on`. `--review-limit` and `--no-assess`
  behave as they do in `work-on`.
- **PR at the end:** draft or ready state does not change. Title and description are updated where
  they do not follow the repo's conventions.
- **Jira:** assign the ticket to the user. Do not change its status.
- **Flags:** same as `work-on`.
- **Code beyond the ticket's scope:** keep it if it makes sense, otherwise split it into a
  follow-up.

## The pipeline

Step numbers refer to `work-on/SKILL.md` as it stands today.

### Phase A. Preflight and ticket validation (work-on Steps 0 to 5, with deltas)

**Step 0 deltas, shared with work-on (the new worktree preflight).** Replaces "Abort if the current
branch is the default branch" in `work-on` Step 0 item 2.

1. If the current branch name contains the key, continue on it. This is the existing branch-key
   check, loosened from "embedded" to "anywhere in the name, case-insensitive". My call: the match
   respects digit boundaries, so `GRO-12` does not match `gro-123-foo`.
2. Otherwise, whatever the current branch is, open a worktree. This follows the user's rule ("if
   the current branch doesn't have the jira ticket in its name") and replaces two current `work-on`
   behaviors: stopping to ask when the branch carries a different key, and continuing on any
   non-default branch. Resolve the target branch:
   - `work-on`: `<key-lower>-<summary-slug>`, created from `origin/<default>`.
   - `take-over`: the head branch of the PR found in the PR lookup below.
3. Create the local branch before calling `wt`, so `wt` reuses it instead of creating one from
   whatever HEAD is (`shared_config/config.zshrc:398-403`):
   - `take-over`: `git fetch origin <branch>`, then `git branch --track <branch> origin/<branch>`.
   - `work-on`: `git fetch origin`, then `git branch <branch> origin/<default>`.
   - If the local branch already exists, check it. For `take-over`, it must track
     `origin/<branch>`. Fix the upstream if it does not, and stop if local has commits the remote
     lacks, since those are not the teammate's and not this run's.
4. `wt <branch>`, then move the whole session into the worktree `wt` created with Claude Code's
   `EnterWorktree` tool (my call). A `cd` in the Bash tool moves only the main thread, and
   subagents (the Step 2 lenses, the B2 checker, the review roles) would otherwise read the
   original checkout. `wt` still creates the worktree. `EnterWorktree` only enters it. If
   `EnterWorktree` cannot enter an existing worktree, stop and tell the user to relaunch Claude
   Code there. Then confirm with `git rev-parse --show-toplevel` and, for `take-over`, that `HEAD`
   equals `origin/<branch>`.

The user's own manual flow is `wt <name>`, re-point the upstream, `git pull`. The design pre-creates
the branch instead because `wt` from a non-default branch would base the new branch on that
branch, and the pull would then merge those commits into the teammate's branch. Builder's call to
keep the user's flow if pre-creating fails for a reason this doc did not foresee, but say so.

**take-over's PR lookup (new, in Step 0).** Find open PRs whose head branch name or title contains
the key (`gh pr list --state open --search <KEY> --json number,title,url,headRefName,author,isDraft,headRepositoryOwner,isCrossRepository`).

- Zero: stop. Say no open PR was found and suggest `/work-on <KEY>`.
- One: that is the takeover PR. Name its number, author and branch.
- More than one: list them and ask which one with `AskUserQuestion`. That is a decision, so it is
  asked even on a flag run.
- On a key-named current branch, the takeover PR is the one whose head is that branch. If none is,
  treat as zero. If other PRs also match the key, still warn.
- A PR from a fork (`isCrossRepository`): stop and say so. Pushing to a fork is a different
  permission problem and is not in scope.

**Step 1 and Step 2 deltas (take-over only).** Two traps:

1. The takeover PR is an open PR doing the ticket's work, which Step 1's open-PR sweep and Step 4's
   superseded path read as "the ticket is dead". Remove the takeover PR from `open_prs` before the
   handoff, and record that in `coverage_notes` so the lenses know the exclusion was deliberate.
   Other matching PRs stay in, since they are exactly what the warning is about.
2. The lenses are pinned to "the tree at HEAD", and HEAD now contains the implementation. A lens
   would find the work already done and call the ticket satisfied. The ticket must be validated
   against `origin/<default>`, with the branch's own commits described in `args` as the
   implementation under takeover rather than as evidence. How to pin the lenses there (a
   `git worktree` of `origin/<default>` for the read, `git show origin/<default>:<path>`, or an
   instruction in the lens brief) is the builder's call. It must be checkable that a lens did not
   read the branch's files as the ticket's baseline.

**Step 4 delta (take-over only, my call).** When the verdict is dead for any reason, the run does
not touch the teammate's PR. No comment, no close. The report names the PR and recommends what
to do with it, and the user decides.

**Step 5 deltas (take-over only).** Assign to the user, as `work-on` does. Do **not** move the
status. Sprint (my call): add to the user's active sprint only when the ticket is in no active
sprint. Description rewrite and validation comment are unchanged. Under `--assess`, `work-on` makes
no board write, and `take-over` keeps that.

### Phase B. Rebase, then check the implementation (take-over only, replaces work-on Step 6's start)

**B1. Bring the branch up to date.** Done before the check so it reads the code as it will merge.

- `git rebase origin/<default>`. On conflicts, resolve them and continue.
- If conflicts keep recurring commit after commit, so one merge is clearly easier, `git rebase
  --abort` and `git merge origin/<default>` instead.
- Push: `git push --force-with-lease` after a rebase, plain `git push` after a merge. A lease
  rejection means the teammate pushed after the fetch. Stop and say so. Do not refetch and retry
  on your own.
- The Final report says which one happened and how many conflicts were resolved.
- Under `--assess`, B1 does not run, since that mode writes no code and pushes nothing. The check
  in B2 then reads the branch as it is, and its report says the branch was not rebased.

**B2. The implementation check.** One fresh leaf subagent (my call: a fresh reader resists
anchoring on the validation context and keeps the full diff out of the main thread). Input: the
ticket as validated (or as filed on a flag run, said plainly), the agreed scope from Step 4 when
there is one, the repo conventions, and `origin/<default>...HEAD`. It is strict. The question is
not "does it roughly do the thing" but:

- **done and correct:** parts of the ticket the diff implements correctly.
- **wrong:** behavior that does not match the ticket, bugs, missing edge cases inside the scope.
- **missing:** parts of the ticket the diff does not implement.
- **approach:** whether the overall approach is sound, or structurally wrong such that patching
  it will keep producing findings.
- **beyond scope:** code the ticket does not ask for, or code outside the scope Step 4 agreed.
  The user's rule: keep it if it makes sense, otherwise split it into a follow-up. B2 proposes
  keep or split per item, with a one-line reason. Kept items are listed in the PR description so
  later readers and the review loop see them as deliberate. Split items are removed from this
  branch in the build step and filed through `work-on`'s "Filing the follow-up" path. The B3
  question below is the approval for each split, which is why splitting is allowed even on a
  `--no-assess` run where `work-on` files no follow-ups (my call).

Output is structured so B3 can decide from it. Exact shape is the builder's call.

**B3. Patch or rewrite.** Pick whichever is cheaper, where cost includes the review iterations a
patch would leave behind, not just today's lines. A structurally wrong approach leans to rewrite.
State the pick and a one-line reason.

**If B2 finds nothing wrong and nothing missing,** skip to Phase C. Say so.

### Phase B continued. Build (work-on Steps 6 and 7, with deltas)

- My call: whenever B2 finds anything, the pick and B2's lists go to the user in one
  `AskUserQuestion` (approve or change) before the first edit, whichever process skill Step 6
  chose. `work-on`'s existing gate sits only on the `brainstorming` path, and a bug ticket routed
  to `systematic-debugging` would otherwise rewrite a teammate's work unseen. On the
  `brainstorming` path this is the same single question, not a second one.
- Everything B2 found is fixed in this build, in one go, before the review loop.
- A rewrite is done as new commits on the same branch, removing or replacing their code, not by
  resetting the branch. Builder's call if a reset is clearly cleaner, but the teammate's commits
  then disappear from the PR history and the report must say so.
- Step 7 unchanged: ticket key in the commit body, push. Stash handling unchanged.

### Phase C. Review, follow-ups, PR, report (work-on Steps 8, 9 and Final report, with deltas)

- **Step 8:** invoke `review-and-fix --branch` (new flag, below). Uncapped unless the user gave
  `--review-limit` or `--no-assess` set its default of 2. Follow-ups filed exactly as `work-on`
  does, against the Step 4 scope.
- **Step 9 delta (take-over only):** no `open-pr`. The PR exists. Push the review commits. Then
  compare the PR's title and description to the repo's conventions (the PR template when there is
  one, `open-pr`'s title rules otherwise) and the branch diff. Rewrite either one only where it
  breaks convention or no longer describes the diff. Use `writing-work-docs`. Apply with
  `gh pr edit`. Do not change draft or ready state. Read the PR back after the edit, as `work-on`
  does after `open-pr`.
- **Final report:** `work-on`'s, plus: takeover PR number and original author, rebase or merge and
  conflict count, B2's verdict, patch or rewrite and why, whether the title or description changed.

## The new `--branch` flag on review-and-fix

`review-and-fix` always reviews `origin/<default>..HEAD`. A PR only changes `<TARGET_ARG>` and
turns on the gh-style Discussion Context (`review-and-fix/SETUP.md:22-41`). `--branch` sets
`<HAS_PR> = false` regardless of what `gh pr view` returns. `work-on` and `take-over` both pass
it. For `work-on` it is a no-op today, since no PR exists at Step 8, and it keeps the two
invocations identical. `--branch` with `--gh-style` is a contradiction (gh-style's product is the
PR context). Stop and say so, the same way the skill treats its other contradictory flags.

## Flags

As in `work-on` ("Argument" in `work-on/SKILL.md`), with one difference in `--assess`:

- `--assess` runs Phase A in full, then B2 against the branch as it is (no rebase, no push), and
  posts one Jira comment carrying both the ticket verdict and the implementation verdict. No board
  write, no code, no push, no PR edit.
- `--no-assess` and `--fast` skip Steps 2 to 4 as in `work-on`. B2 still runs, against the ticket
  as filed, since it is the scope-creep guard. Its report says the ticket was not validated.

## How the reuse works (my call)

The user offered to split `work-on` into reusable skills. This design does not, for now. `work-on`
is 2,282 lines whose steps cross-reference each other (the flag rules, the stash ownership rule,
the running `TODO(user):` list, the follow-up filing path reused by name). Cutting it into skills
would mean passing that state across skill boundaries, which is a larger and riskier change than
`take-over` needs.

Instead:

- `take-over/SKILL.md` is a delta document. It tells the run to read `work-on/SKILL.md` and its
  helper files and follow Steps 0 to 5, 7, 8 and the Final report, applying the deltas above at
  named points. It owns Phase B and the Step 9 replacement outright.
- `work-on` gets the shared worktree preflight and the `--branch` pass-through directly in its own
  text, so both skills share one copy.
- Loading: `take-over` must not invoke `work-on` through the Skill tool, since that runs `work-on`
  end to end. It reads the file.

If the delta approach proves fragile in use, splitting comes next, starting with the tail (Steps 8,
9 and the report).

## Other changes this needs

- **`hooks/deny-review-in-workflow.py`:** add `take-over` to `DENIED_SKILLS`. It fans out through
  `review-and-fix` and has an approval gate, the same reasons `work-on` is there. Add matching cases
  to `tests/deny_review_in_workflow_test.sh`, including that a `take-over-*` name falls through per
  the hook's word-boundary rule.
- **AGENTS.md:** the "Fan-out skills must run in the main thread" section names the denied skills.
  Add `take-over` there.

## Risks the builder must check before relying on them

- **`wt` from the Bash tool.** `wt` is a zsh function that `cd`s and runs `code .`. Test it from
  the Bash tool first. The user says the sandbox is rarely on now, so a sandbox block is not the
  expected case. If it fails anyway, ask the user to run `! wt <branch>`. Do not reimplement
  `wt`, since the user wants to keep using it.
- **cwd after `wt`.** The run must continue in the new worktree. Confirm with
  `git rev-parse --show-toplevel` before Step 1.
- **The commit-time hooks** (`ask-prose-claims.py`, `check-staged-code.py`) apply to commits in
  this repo. Per the user's memory, commits in `~/.melvin/config` use
  `PROSE_CLAIMS_OK=1 CASE_LIST_OK=1 CODE_CHECKS_OK=1`.

## Left to the builder

- B2's output schema and the exact subagent prompt.
- How the Phase A lenses are pinned to `origin/<default>`.
- The slug rule for `work-on`'s new branch names (the existing Step 0 suggestion rule is a fine
  start).
- Wording and placement of the deltas in `take-over/SKILL.md`.
