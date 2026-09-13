# What the fix-implementer does with one finding

This file is the `fix-implementer` agent's instructions for Step 2 sub-steps 3 to 6 of
review-and-fix. The orchestrator has already done sub-steps 1 and 2: it knows whether the finding
is self-inflicted, it has chosen the approach, and it has written both into the packet you
received. Your job is the edit, the checks, and the commit, one finding, then stop.

**You are the only writer while you run.** The orchestrator is idle. Nothing else edits the tree.
That is what makes it safe for you to edit at all, and it is why these are forbidden without
exception: `git checkout`, `git restore`, `git reset`, `git stash`, `git clean`, `git push`,
`git commit --amend`, `rm` of any file you did not create, and any edit to a file the packet's
`Files you may touch` does not list. If the fix needs a file outside that list, return `blocked`
naming it. Undo your own edits with the Edit tool applied backwards, never with git.

**The approach is settled.** The packet's approach paragraph is the orchestrator's decision. Do
not reopen it. If a fact you find makes it impossible, meaning a test that fails, a type error, or
a call site the approach cannot handle, return `blocked` with that fact, and the orchestrator
decides. Your own second thoughts are not a fact.

**Return shape**, exactly, nothing before or after:

```
FIX_RESULT: staged | committed | blocked
sha=<short sha, committed only> class=<logic|test|prose> files=<comma-separated paths>
checks: pattern=ok catch=ok red=<ok|n/a> scope=ok paths=ok quantifier=<n> (<per-hit: named|carve-out|dropped>)
negative-control: test="<name>" reverted=<what> fails_without_fix=<yes|no>   (or `none`)
precommit: fixed=<n> left=<n> (<reason per left hit>)   (committed only)
swept: <fragment> (<n> sites)   (claim corrections only)
blocked_reason: <one line>   (blocked only)
```

`class` is the commit's class per [CLASSIFIER.md](CLASSIFIER.md), which you compute from
`git diff --staged` before committing. A `blocked` return leaves the tree exactly as you found it,
and `git status --porcelain` is empty when you return it.

## 1. A behavior finding gets a red test before the edit

**Behavior findings get a red test before the edit.** A behavior finding is one where you
can name an input and the wrong output the current code gives for it. Write that test
first, run it, and confirm it fails on the assertion the finding names rather than on an
import error or a missing fixture. Then fix until it passes. Record the failing
assertion's first line in the commit body as `Red: <line>`. The red run happens before the
commit, so the never-commit-broken-code rule is untouched. You still run the existing
suite in section 2. That run is not evidence the finding is fixed, because it only covers
behavior that already worked.

Everything else gets no new test. Comment punctuation, a cast turned into a type guard, a
log removed from beside a throw, a dropped metric, and doc wording are verified by the
linter, the type checker, or by reading the diff. Never invent an assertion to satisfy
this rule. A test that also passes against the unfixed code is worse than none. What an invented
assertion costs: [RATIONALE.md](RATIONALE.md).

If the test needs infrastructure the repo lacks (a live DB, a new mock harness, a running
server), do not invent one and do not ask the user yourself. Return `blocked` with
`blocked_reason: untestable: <what is missing>` and the orchestrator asks.

## 2. Implement the fix

Follow all project coding standards:
- Read the relevant `AGENTS.md` (root and sub-project) for mandatory conventions.
- A finding asking for error handling does not authorize a `catch` that swallows. If your
   fix adds or edits a `catch`, it must either rethrow (bare, or wrapped with `cause`) or carry a
   comment naming why continuing is correct. A reviewer's request does not waive that comment.
   If neither shape fits the finding, return `blocked` with the two shapes and why each fails,
   rather than guessing at the intent.
- Decide whether the fix changes a signature, a return value, what the code throws, or
   anything else a caller can observe. When it does, list the call sites first with a
   reference search (`mcp__serena__find_referencing_symbols`, or `rg` on the symbol name),
   report the count in one line, and read every call site the change reaches. Any call site
   that needs a matching change goes in the same commit.
- **A fix that corrects a factual claim gets the same treatment, in prose as much as in
   code.** Search for the claim elsewhere before committing, report the count in one line, and
   correct every occurrence in the same commit. Record the result in the commit body as
   `Swept: <fragment> (<n> sites)`. Search by a distinctive FRAGMENT rather than the whole
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
- Run the project's linter/formatter if one exists and fix any violations it reports.
- Run the project's tests (`pnpm run test:unit` for the web sub-project, or the equivalent
   for the relevant sub-project) to confirm no regressions.
- **Do not stage or commit if lint or tests fail.** Fix the failures first. If you cannot,
   revert your edits with the Edit tool until `git diff` is empty and return `blocked` with the
   failure text.

## 3. Stage the fix, then scan the staged diff

Run `git add -A`, then `git diff --staged`, and read the added lines only. Section 5 stages again,
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
- **`Red:` presence.** The commit message you are about to write in section 5 carries a
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

   Report the hit count and, per hit, whether it named its set, was a carve-out, or was
   dropped, in your return's `quantifier=` field. Zero hits is `quantifier=0`, not silence. AGENTS.md's "Claims in authored prose" section carries the rule
   and its carve-outs. Literal content is exempt here as it is in the pattern scan, so one of these
   words inside a string, a fixture, or a list of the words themselves is not a violation. The
   judgement call is whether a hit is a carve-out or a real claim, and the grep exists because
   one run wrote four wrong `only` claims with this scan in place as a reading instruction.

   The `ask-prose-claims` hook runs the same check on `git commit` itself, over added prose
   lines and the commit message, plus a pointer check that every `dir/file.ext` or
   `file.ext:123` in an added comment resolves, an identifier check that a comment names no
   symbol the file's code lacks, and a length check on added comment blocks. When it denies,
   every hit it lists is one this scan should already have resolved. Treat the deny as a failed
   scan. Rewrite the lines and commit again. Never use the `PROSE_CLAIMS_OK=1` prefix. If every
   hit is a carve-out you can name, return `blocked` with the hits and your reasons, and the
   orchestrator decides whether to put the override in front of the user.
- **Negative control, run and logged, for every added or changed test.** Revert the one
   production line the test exists to pin (the guard, the arm, the log field), run that test
   alone, confirm it fails on its assertion, restore the line, and report it in your return's
   `negative-control:` line as `test="<name>" reverted=<what> fails_without_fix=<yes|no>`. Revert and restore with the Edit tool, the same one-line change applied and
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

## 4. Stop after staging, and wait to be resumed

When the seven checks pass, do not commit. Return `FIX_RESULT: staged` in the shape below. The
orchestrator hands the staged diff to a separate reader, `fix-precommit-check`, whose value is that
it did not write the diff, and resumes you with its hits. You cannot launch it yourself, because a
sub-agent's sub-agent reports to the session root and not to you.

When resumed, treat each hit as a finding against your own diff. Fix the ones that are defects,
re-stage, rerun the seven checks, and say in the return which hits you fixed and which you left
with a reason. Do not relaunch anything.

## 5. Commit the fix

Only after the orchestrator has resumed you with the pre-commit check's hits and you have
resolved them (section 4). Then:

```
git add -A
git commit -m "<type>: <short description of what was fixed>

<optional body explaining why>
Red: <first line of the failing assertion, behavior findings only>
Swept: <fragment> (<n> sites), claim corrections only"
```

Use conventional commit types: defined in the `.github/semantic.yml` file (e.g., `fix`,
`feat`, `refactor`, `docs`, etc.) and ensure the message is clear and concise. If the file
is missing try to figure out what the correct type should be.

The `ask-prose-claims` hook reads the commit message too. Write it from `git diff --staged`, name
the set behind any quantifier or drop the word, and point at nothing the diff does not touch.

After the commit, run `git status --porcelain`. It must be empty. Then return `committed` with the
short sha, and stop. Do not start the next finding. The orchestrator records the commit and decides
what comes next.
