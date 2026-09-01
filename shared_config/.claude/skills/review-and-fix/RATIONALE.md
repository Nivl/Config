# Why the rules are drawn where they are

Derivations behind rules that SKILL.md states as imperatives. Nothing here is needed to execute a
run. It is here for a reader deciding whether a rule can be relaxed, because each of these rules
looks arbitrary until you have the argument for it.

Measured run history is not in this file. That lives in
`~/.melvin/config/docs/research/review-and-fix-run-log/NOTES.md`.

## Contents

- Why the Agent-tool abort precedes the Jira preflight
- Why the run-log home is tried rather than assumed
- Why appending live is load-bearing
- Why recording a shortfall does not discharge resolving the agent
- Why the claim sweep is bounded to already-modified files
- Why test-runner config sits on the never-logic list
- Why classify-up is the safe tie-break
- What relitigating an approach actually costs
- Why narrowing a mechanism claim fails again
- What an empty active set would do to row 1
- Why row 1b relaunches only the short kind
- What the staged-diff scan and the red-test ban each cost when skipped
- Why the class vocabulary collides with the words the roles use
- Why the two glosses on the wide-versus-deep counts were dropped

## Why the Agent-tool abort precedes the Jira preflight

The preflight asks the user a question that a context with no `Agent` tool cannot usefully answer.
Offering someone the choice to authenticate a Jira reader, when nothing downstream can run either
way, spends their attention for nothing. So the tool check goes first, and the abort ends the run
before the question is asked.

## Why the run-log home is tried rather than assumed

A preference with no check behind it becomes the fallback on every run. Worse, a log that silently
landed in `/tmp` looks exactly like one that was meant to be there, so the degradation is invisible
in the artifact it degrades.

That is why the rule is to run the preferred `mkdir` and read its exit code, rather than reasoning
about whether the directory is likely to exist.

## Why appending live is load-bearing

The loop has no iteration cap, so a run that is not converging ends when the user interrupts it,
and an interrupted run never reaches Step 4. Appending as the run goes is what leaves that run a
record at all, and those are the runs most worth reading afterwards.

## Why recording a shortfall does not discharge resolving the agent

An entry in `reviewers_missing` says an instance's findings are not in the pool. It says nothing
about whether that process is still reading files. A written-off reviewer goes on running until
something stops it, and while it runs the fix phase is editing the tree underneath it.

Two separate obligations, then. Record the shortfall for coverage, and resolve the agent for
safety. Doing the first is not doing the second.

## Why the claim sweep is bounded to already-modified files

Every lens that raises a prose finding is scoped to the branch's own changes. So an occurrence in
an unmodified file could never have become a finding in the first place.

Editing it anyway pulls that file into the modified set, where role 5 then reads all of its
pre-existing comments. The bound is what keeps a rule meant to remove findings from generating
them.

## Why test-runner config sits on the never-logic list

The cost is real. A change that breaks which tests run no longer triggers a full rerun, so a suite
that stopped executing can reach a clean iteration.

It is on the list anyway because row 4 exists to revalidate application logic, and test wiring is
not that. Nothing in production imports a test-runner config, so what such a change can break is
which tests run rather than what the app does.

## Why classify-up is the safe tie-break

The cost of classifying up is only rerunning more reviewers next iteration, never a missed defect.
The cost of classifying down is pruning the logic reviewers on a commit that changed logic, which
reaches a clean stop with a green check and nothing prompting anyone to look.

The rule breaks ties in your knowledge, not ties in the order. A commit that cleanly matches an
earlier class is not a tie.

## What relitigating an approach actually costs

The failure mode looks like this. Implement approach A, decide mid-edit that B is better, implement
B, then conclude A was right after all. Every lap costs a full implementation, and the tree is left
in a third state that is neither A nor B.

That third state is the real damage, because the fix phase stages with `git add -A`. A working A
beats an unbuilt B, and the loop reviews A next iteration either way.

## Why narrowing a mechanism claim fails again

AGENTS.md's "Claims in authored prose" carries the general rule. It matters most during a fix
because a fix authored under review pressure reaches for the smallest edit that answers the
finding, and for a mechanism sentence the smallest edit is a rescope.

A rescoped mechanism sentence is still a mechanism sentence. So it fails again next iteration on a
different reader, in a new way each time.

## What an empty active set would do to row 1

An empty active set finds nothing. An empty findings list with `reviewer_unavailable` and
`roles_missing` both empty is row 1, so the run would stop and report `complete` coverage with no
reviewer having read the final tree.

The category aliases in sub-step 7 narrow this to one case, a `discussion` finding as an
iteration's only commit, since every other gh-style category attributes to an in-depth role too.
`any_commit` was true and every committed fix attributes to a reviewer, but the reviewer it
attributes to can be the gh-style unit alone.

## Why row 1b relaunches only the short kind

The diff is unchanged since the other reviewers cleared it, so relaunching them buys nothing. Row
1b fires on an empty findings list, which means the reviewers that did report found nothing left to
find in that same tree.

## What the staged-diff scan and the red-test ban each cost when skipped

Both rules exist to move a cost earlier, so the argument for each is the cost of paying it later.

A violation noted and left for later is next iteration's finding. That is a whole reviewer fan-out
and a whole fix pass spent on something the scan catches in the seconds before the commit lands.
The same holds for prose written during a fix, which is unreviewed prose. A path that no longer
resolves becomes a finding the next iteration raises against text this run just wrote.

An invented assertion costs the other direction. A test that also passes against the unfixed code
proves nothing, and role 9 flags it as ceremony on the next pass, so the run pays for writing it and
then pays again for removing it.

## Why the class vocabulary collides with the words the roles use

`prose`, `test` and `logic` are commit classes derived from a diff in sub-step 7. The same three
words are also used loosely for authored text, for test code, and for what roles 1, 5, 11 and 9
review. That collision is why the rule against naming a class you did not derive is stated
separately.

A finding can be about prose while its fix is `logic`. So the lane a finding came from never names
the class of the commit that fixed it, and describing a commit from its finding's subject matter is
how a `logic` commit gets reported as a prose correction.

## Why the two glosses on the wide-versus-deep counts were dropped

Both commands and the wide-versus-deep distinction stay inline in Step 1, because they are what the
act needs. The sentences explaining what each number means were restating the command that produces
it.
