# The fix sub-steps

Step 2 sub-step 3 of review-and-fix, in full. You have done sub-steps 1 and 2 for the finding, so
you know whether it is self-inflicted and you have chosen the approach, and SKILL.md has sorted
the findings into class groups. What follows is the red test, the edit, the seven staged-diff
checks, and the commit, for one group, one commit. Where this file says "the finding", read the
group's findings. Sub-step 4 in SKILL.md runs the pre-commit check over every six of these commits, and
sub-step 7 records each one.

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
A prose-, test- or comment-only diff passes. `CASE_LIST_OK=1` in front of the command overrides
and puts the commit in front of the user.

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
server), use `AskUserQuestion` and offer to fix without a test or to skip the finding. Record a
skip in `skipped_findings`, keyed by the finding's `file` plus `title`, so later iterations do
not re-prompt. An untestable finding never blocks the loop.

## 2. Implement the fix

Follow all project coding standards:
- Read the relevant `AGENTS.md` (root and sub-project) for mandatory conventions.
- A finding asking for error handling does not authorize a `catch` that swallows. If your
   fix adds or edits a `catch`, it must either rethrow (bare, or wrapped with `cause`) or carry a
   comment naming why continuing is correct. A reviewer's request does not waive that comment.
   If neither shape fits the finding, use `AskUserQuestion` rather than guessing at the intent.
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
   branch has ALREADY modified. Report a hit outside that set in one line and do not edit it**,
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
   sub-step 1 blamed the finding's sentence on a commit of this run, the sentence has already
   been written once under review and found wrong. The allowed edits are two: delete the sentence,
   or replace it with a pointer to a symbol this file's code names (`See <symbol>`, or the
   docstring on it). No third wording. Grep the run log for `prose-locus` lines at the same
   `file:range`. When one exists, this is the sentence's second self-inflicted finding, and the
   only edit is deletion, the pointer form included. Before the edit, append
   `prose-locus iter=<N> finding=<id> locus=<file:range> nth=<1|2> action=<deleted|pointer>`.
   Measured on the run that motivated this: 56 self-inflicted findings over eight iterations,
   every one prose, and the rootcause lines for them read "replaced one collaborator claim with
   a different collaborator claim", "a shorter over-claim on the same locus", "a new why", with
   five loci rewritten four to six times. Each rewrite was a fresh claim for the next iteration
   to read. A deletion is not.
- Run the project's linter/formatter if one exists and fix any violations it reports.
- Run the project's tests (`pnpm run test:unit` for the web sub-project, or the equivalent
   for the relevant sub-project) to confirm no regressions.
- **Do not commit if lint or tests fail.** Fix the failures first or escalate to the user.

## 3. Stage the fix, then scan the staged diff

Run `git add -A`, then `git diff --staged`, and read the added lines only. Section 4 stages again,
which is then a harmless no-op. Seven checks. Each check starts from a pattern match on the added lines,
never from a review of the design. Four of them need a judgement call, and each is named where
it arises. Fix whatever
a check catches, re-stage, and rerun the scan. Never commit with a note to fix it later. What a
noted violation costs: [RATIONALE.md](RATIONALE.md).
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
   scan. Rewrite the lines and commit again. Use the `PROSE_CLAIMS_OK=1` prefix only when each
   hit is a carve-out you have named in the `quantifier-scan` lines, because that prefix puts
   the commit in front of the user.
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
Swept: <fragment> (<n> sites), one line per claim correction in the group"
```

Use conventional commit types: defined in the `.github/semantic.yml` file (e.g., `fix`,
`feat`, `refactor`, `docs`, etc.) and ensure the message is clear and concise. If the file
is missing try to figure out what the correct type should be.

After the commit, run `git status --porcelain`. It must be empty. Then SKILL.md sub-step 7.
