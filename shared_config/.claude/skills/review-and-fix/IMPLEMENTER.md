# The fix sub-steps

Step 2 sub-step 3 of review-and-fix, in full, for the `fix-implementer` agent. The orchestrator did
sub-steps 1 and 2 for every finding, so the packet says whether each is self-inflicted and gives
its settled approach, and it sorted the findings into class groups. What follows is the red test,
the edit, the seven staged-diff checks, the commit, and its run-log line, for one group, one commit.
Where this file says "the finding", read the group's findings. Work the packet's groups in order.
The orchestrator runs the pre-commit check over each batch after you return (SKILL.md sub-step 4).

**Questions go back, not to the user.** You have no tool to ask one, and you run in the background,
where a permission prompt may reach nobody. Where a section below says a case needs a decision,
leave that group's edits out of the tree, mark it `blocked` in your return with the fact in one
line, and go on to the next group. Undo edits to existing files with the Edit tool, and `rm` a file
this group created, which is the one `rm` allowed.

**Never prefix `PROSE_CLAIMS_OK=1`, `CASE_LIST_OK=1` or `CODE_CHECKS_OK=1`.** The prefix needs
the user. A `require-case-list` deny is never a carve-out. It means a missing `cases` or
`invariant=` line, so write it per section 0 and commit again. When it names a missing score or a
second reopen, return the group `blocked` with that line.

**An `ask-prose-claims` or `check-staged-code` deny whose hits are all carve-outs keeps its fix.**
A deny with any hit you can fix is not this case, so rewrite and commit again. When every hit is a
carve-out, name each prose hit under the `quantifier-scan` line, as section 3 says, and then:

1. Leave the group staged. Write the exact `-m` message the denied commit used, unchanged, to
   `/tmp/claude/rf-staged-msg-iter<N>-batch<b>-group<g>.txt`.
2. Skip sections 4 and 5. The orchestrator commits it and appends its line.
3. Return the group `state=staged` (section 6 shape), each hit copied verbatim from the deny with
   the carve-out named beside it, and stop. Do not start the next group, since its `git add -A`
   would sweep the staged edits into its commit.

The orchestrator answers one of three ways. `committed` means carry on with the packet's groups
you had not started. `rewrite` with a hit means fix that line with the Edit tool, `git add -A`, run
section 3 again, and commit or return `state=staged` again. `drop` means undo the group's edits as
for a blocked group, confirm `git status --porcelain` is empty, and carry on.

**The run log is yours to append to.** Its path is in the packet. Every line this file names goes
there as you take the step, with the iteration number the packet gives. The `require-case-list` hook
reads it at your `git commit`. Append the lines one moment produces in one Bash call, as one
`printf '%s\n' '<line>' '<line>' >> <run log>`, single-quoted per line since the lines carry
double-quoted values, with `'\''` for a single quote inside one. Not a heredoc, which a hook
denies.

**The approach is settled.** New information reopens it and rereading the same information does
not. A test that fails, a type error, or a call site that makes the chosen approach impossible is
new information. Second thoughts on the same facts are not. If the doubt is real, finish the
approach, run the checks, and let the result decide.

**Undo an edit with the Edit tool applied backwards, never with git.** `git checkout -- <path>`,
`git restore` and `git stash` take the whole file, and the file holds the rest of the fix.

## 0. A logic group gets a case list before the edit

Before touching code for a `logic` group, append these lines to the run log, the first three
always and the fourth when the edit touches paired state:

```
cases iter=<N> findings=<ids> changes="<what this edit changes, one sentence>"
cases iter=<N> findings=<ids> keeps="<what must stay the same, one sentence>"
cases iter=<N> findings=<ids> reaches="<the finding's case>; <each neighbour: the other field, the other provider, the null path, the error path, the other log level>"
cases iter=<N> findings=<ids> invariant="<what must hold across every path, when the edit touches paired state>"
```

**When the edit implements a `chose=order:` decision**, `reaches=` names every case in that
ranking, and the group's tests pin each adjacent pair in it plus the pair the finding raised. A
case no test can reach is named in `reaches=` with the reason.

**When the edit touches state that has to pair up**, a claim and its release, a lock and its
unlock, an open and its close, a retry and the idempotency it relies on, a transaction and its
commit or rollback, two more things are required. `invariant=` states the property in one
sentence, in the shape "a claim taken on this path is released on every exit of the path, and only
while it is still ours". And `reaches=` names every exit of the region, not only the neighbours:
the normal return, each early return, and each `await` or call between acquire and release that
can throw. An exit the list does not name is the one the next iteration finds. Measured on one
four-iteration run, the same claim/release region was fixed four times, each fix closing the exit
the finding named and leaving the next one open, and no iteration wrote the property down. The
`require-case-list` hook denies a code commit whose `reaches=` mentions those words with no
`invariant=` line beside it.

Then the red test in section 1 covers the finding's case, and each neighbour that already works
gets an assertion that pins it, in the same test file, before the edit. A neighbour you cannot
name a test for is named in `reaches=` with `untested` after it.

Why: across the logged runs, the self-inflicted logic fixes were the neighbour the finding did
not name. A guard added for the tax and not the total. An id logged under the other provider's
key. A resolved plan dropped on the no-total path. The fix handled the case the finding named,
and the next iteration's roles found the case beside it, at $80 an iteration. The list is the
enumeration those fixes skipped, made explicit before the edit and handed to the pre-commit check
so it reads the neighbours too. A `prose` or `test` group writes no case list.

The `require-case-list` hook enforces this at `git commit`. Inside a run, a commit whose staged
diff changes a code line is denied when the run log has no `cases iter=<N> ... reaches=` line for
the current iteration. On the run that made the case for this list, it was written in iteration 1
and skipped in iterations 2 to 4, and each of those fixed a bug the previous one's fix introduced.
A prose-, test- or comment-only diff passes. A deny means a line is missing, so write it and
commit again.

## 1. A behavior finding gets a red test before the edit

**Behavior findings get a red test before the edit.** A behavior finding is one where you
can name an input and the wrong output the current code gives for it. Write that test
first, run it, and confirm it fails on the assertion the finding names rather than on an
import error or a missing fixture. Then fix until it passes. Record the failing
assertion's first line in the commit body as `Red: <line>`, one `Red:` line per behavior finding
in the group. The red run happens before the
commit, so the never-commit-broken-code rule is untouched. You still run the existing
suite in section 2. That run is not evidence the finding is fixed, because it only covers
behavior that already worked.

Everything else gets no new test. Comment punctuation, a cast turned into a type guard, a
log removed from beside a throw, a dropped metric, and doc wording are verified by the
linter, the type checker, or by reading the diff. Never invent an assertion to satisfy
this rule. A test that also passes against the unfixed code is worse than none. What an invented
assertion costs: [RATIONALE.md](RATIONALE.md).

If the test needs infrastructure the repo lacks (a live DB, a new mock harness, a running
server), return the group `blocked` with `untestable: <what is missing>`. The orchestrator asks the
user whether to fix without a test or skip, and records a skip in `skipped_findings`. When the
packet already says `no-test-ok` for the finding, the user chose to fix without a test, so fix it.

## 2. Implement the fix

Follow all project coding standards:
- Read the relevant `AGENTS.md` (root and sub-project) for mandatory conventions.
- A finding asking for error handling does not authorize a `catch` that swallows. If your
   fix adds or edits a `catch`, it must either rethrow (bare, or wrapped with `cause`) or carry a
   comment naming why continuing is correct. A reviewer's request does not waive that comment.
   If neither shape fits the finding, return the group `blocked` rather than guessing at the intent.
- Decide whether the fix changes a signature, a return value, what the code throws, or
   anything else a caller can observe. When it does, list the call sites first with a
   reference search (`mcp__serena__find_referencing_symbols`, or `rg` on the symbol name),
   report the count in one line, and read every call site the change reaches. Any call site
   that needs a matching change goes in the same commit.
- **A fix that corrects a factual claim gets the same treatment, in prose as much as in
   code.** Search for the claim elsewhere before committing, report the count in one line, and
   correct every occurrence in the same commit. Record the result in the commit body as
   `Swept: <fragment> (<n> sites)`, one line per corrected claim when the group holds several.
   Search by a distinctive FRAGMENT rather than the whole
   phrase, with `rg -U` or `\s+` for every space, because prose wraps and a line-oriented
   search cannot match a phrase split across two lines. **Bound the search to tracked files the
   branch has ALREADY modified**, the packet's `Branch files` list plus
   `git diff --name-only <RANGE_BASE>...HEAD` for what this run's commits added. **Report a hit
   outside that set in one line and do not edit it**,
   because editing it pulls that file into the modified set where role 5 then reads all of its
   pre-existing comments. The judgement call is whether a hit is the same claim or a different
   one that shares wording. A fix confined to formatting or punctuation still skips both
   bullets. Why the bound is drawn there: [RATIONALE.md](RATIONALE.md).
- **Correcting a claim about another file's mechanism means deleting it, not narrowing it.**
   A fix authored under review pressure reaches for the smallest edit that answers the finding,
   and for a mechanism sentence the smallest edit is a rescope, which fails again next iteration
   on a different reader. Replace it with a pointer, an invariant, a locally derivable fact, or
   nothing.
- **A self-inflicted prose finding is fixed by deletion or a pointer, never by a rewrite.** When
   the packet marks the finding `origin=self-inflicted`, the sentence has already been written
   once under review and found wrong. The allowed edits are two: delete the sentence, or replace
   it with a pointer to a symbol this file's code names (`See <symbol>`, or the docstring on it).
   No third wording. The packet's `prose-locus nth=` says which finding on that sentence this is.
   At `nth=2` the only edit is deletion, the pointer form included. Before the edit, append
   `prose-locus iter=<N> finding=<id> locus=<file:range> nth=<1|2> action=<deleted|pointer>`.
   Measured on the run that motivated this: 56 self-inflicted findings over eight iterations,
   every one prose, and the rootcause lines for them read "replaced one collaborator claim with
   a different collaborator claim", "a shorter over-claim on the same locus", "a new why", with
   five loci rewritten four to six times. Each rewrite was a fresh claim for the next iteration
   to read. A deletion is not.
- Run the project's linter/formatter if one exists and fix any violations it reports.
- Run the project's tests (`pnpm run test:unit` for the web sub-project, or the equivalent
   for the relevant sub-project) to confirm no regressions.
- **Do not commit if lint or tests fail.** Fix the failures first, or return the group `blocked`
   with the failing output's first lines.
- **A fact that defeats the settled approach** (a failing test the approach cannot pass, a type
   error, a call site it cannot handle) is new information, and choosing again is the
   orchestrator's job. Return the group `blocked` with the fact.
- **Stay in the branch's files.** The packet lists the files each finding may touch. A call site or
   test the fix itself makes wrong may be edited too, when it is in the packet's `Branch files`
   or this run's own commits already touched it. Any other file means `blocked`.

## 3. Stage the fix, then scan the staged diff

Run `git add -A`, then `git diff --staged`, and read the added lines only. Section 4 stages again,
which is then a harmless no-op. Seven checks. Each check starts from a pattern match on the added lines,
never from a review of the design. Four of them need a judgement call, and each is named where
it arises. Fix whatever
a check catches, re-stage, and rerun the scan. Never commit with a note to fix it later. What a
noted violation costs: [RATIONALE.md](RATIONALE.md). When the staged diff spans classes, such as a
`prose` group whose fix had to touch code, do not commit it as one. Return the member that changed
class `blocked` with `fact="spans classes"`, and commit the rest.
- **Pattern scan, authored prose and added lines only.** No `→ ← … ≥ ≤ × — –` and no curly
   quotes. In comment bodies and prose or doc files only, no ` - ` and no `:` joining two
   independent clauses, both of which AGENTS.md bans as joiners. A `:` is fine as a
   line-leading label prefix such as `TODO:` or `NOTE:`, and in a ratio, a time, a path, or a
   URL. Arithmetic, YAML and markdown list markers, and CLI examples are not violations.
   No ticket key matching
   `\b[A-Z][A-Z0-9]{1,9}-\d{1,6}\b`, excluding protocol names such as UTF-8, SHA-256,
   RFC-7231, ISO-8601, and CVE-2024. None of the changelog literals `added this`,
   `changed from`, `new logic`, `was previously`, `remove old impl`. No added `as any` and
   no added `as unknown as`. Literal content is exempt, so one of these glyphs inside a
   string, a fixture, or quoted output is not a violation.
- **Catch artifact.** Every added `catch` rethrows or carries a why-comment (section 2).
- **`Red:` presence.** The commit message you are about to write in section 4 carries a
   `Red:` line when this was a behavior finding (section 1).
- **Scope.** `git diff --staged --stat` lists only files the finding named or either search
   in section 2 turned up. For any other file, state why in one line.
- **Paths and identifiers resolve.** Every file path and every code identifier written into
   authored prose on an added line must resolve. Resolve each one before the commit lands, and
   correct or drop whatever does not. The judgement call is whether a token is a claim about
   this repo or an illustrative example, and only a claim has to resolve.
- **Quantifier scan, run as a command and logged.** In authored prose, no `nothing`, `never`,
   `always`, `every`, `only`, `none`, `the one`, or `the only` ranging over a set the line does
   not name. Either the line names the set, or the word goes. Run it mechanically on the staged
   diff rather than by reading:

   ```
   git diff --cached -U0 | grep -nE '^\+.*\b(nothing|never|always|every|only|none|the one)\b'
   ```

   Then append `quantifier-scan iter=<N> findings=<ids> hits=<n>` to the run log before the commit,
   where `<ids>` are the merged findings this commit fixes, since the sha does not exist yet,
   followed by one line per hit that names the set, is a carve-out, or was dropped. Zero hits is
   a `hits=0` line, not silence. AGENTS.md's "Claims in authored prose" section carries the rule
   and its carve-outs. Literal content is exempt here as it is in the pattern scan, so one of these
   words inside a string, a fixture, or a list of the words themselves is not a violation. The
   judgement call is whether a hit is a carve-out or a real claim, and the grep exists because
   one run wrote four wrong `only` claims with this scan in place as a reading instruction.

   The `ask-prose-claims` hook runs the same check on `git commit` itself, over added prose
   lines and the commit message, plus a pointer check that every `dir/file.ext` or
   `file.ext:123` in an added comment resolves, an identifier check that a comment names no
   symbol the file's code lacks, and a length check on added comment blocks. When it denies,
   every hit it lists is one this scan should already have resolved. Treat the deny as a failed
   scan. Rewrite the lines and commit again. When every remaining hit is a carve-out, name each
   one under the `quantifier-scan` line and return the group `state=staged`, per the carve-out
   rule at the top of this file.
- **Negative control, run and logged, for every added or changed test.** Revert the one
   production line the test exists to pin (the guard, the arm, the log field), run that test
   alone, confirm it fails on its assertion, restore the line, and append
   `negative-control iter=<N> findings=<ids> test="<name>" reverted=<what> fails_without_fix=<yes|no>`
   to the run log. Revert and restore with the Edit tool, the same one-line change applied and
   then applied backwards. Never `git checkout --`, `git restore` or `git stash` for this, because
   the file also holds the rest of the staged fix and those commands take all of it. After the
   restore, `git diff -- <file>` must print nothing, since the worktree and the index should
   agree again. If it prints anything, the restore was wrong and the fix has to be re-applied
   before anything else. A `no` means the test is vacuous and does not get committed as it
   stands.
   A commit that adds no test writes no line. This was already the rule for behavior findings
   through `Red:`. It now covers the test-coverage fixes too, because six of thirty
   self-inflicted fixes across five runs were tests that could not fail, matched any string, or
   passed unchanged on `origin/master`, and the runs that ran controls had none of those.

## 4. Commit the fix

When the seven checks pass, commit, one commit for this group alone:

```
git add -A
git commit -m "<type>: <short description of what was fixed>

<optional body explaining why>
Red: <first line of the failing assertion, one line per behavior finding in the group>
Swept: <fragment> (<n> sites), one line per claim correction in the group
<the packet's commit trailer, when it gives one>"
```

Use conventional commit types: defined in the `.github/semantic.yml` file (e.g., `fix`,
`feat`, `refactor`, `docs`, etc.) and ensure the message is clear and concise. If the file
is missing try to figure out what the correct type should be.

After the commit, run `git status --porcelain`. It must be empty.

## 5. Append the commit line

A `state=staged` group skips this section, since the orchestrator makes that commit and writes its
line. For every other commit, classify it from its own diff and append its `commit iter=` line to
the run log, per SKILL.md Step 2 sub-step 7's "Classify the commit" bullet and the line shape
below it. Read that bullet and [CLASSIFIER.md](CLASSIFIER.md) once, before the first commit. The packet gives each
finding's `category`, `roles`, `raised_by`, `origin` and any `actionable_unique` value, and the
`title` to copy verbatim, so the line is assembled from the packet plus your classification. When
the class you got differs from the group's class in the packet, add `class_expected=<packet class>`
to the line.

## 6. The follow-up and the return

When the orchestrator resumes you with pre-commit hits, fix each one it marked a defect, run
sections 3 to 5 on the result, and land them as one follow-up commit whose message names the
commits it corrects. Its commit line carries `origin=precommit`, the `findings=` and `roles=` the
orchestrator gave, and the title it gave. When it resumes you with another batch, start again at
the first group of that packet.

Return one line per group, then one line for the batch:

```
group=<g> state=committed sha=<short sha> class=<prose|test|logic> findings=<ids>
group=<g> state=blocked findings=<ids> fact="<one line>"
group=<g> state=staged class=<prose|test|logic> findings=<ids> hooks=<hook names> msg=<message file> hits="<hit verbatim> -- <carve-out>; ..."
batch commits=<n> blocked=<n> staged=<n> not_started=<group numbers or none> tests="<command run, and passed or failed>"
```
