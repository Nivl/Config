# Tagger prompt

Give this to one Sonnet agent with `titles.tsv` from `extract_titles.py`. It classifies every
finding into the static check that would have caught it before review, or NONE. Edit the detector
list when a check is added, so a finding the new check covers is tagged with that check's name and
not re-counted as a new candidate.

---

Read-only classification task. Do not edit any file except the output file named below. Do not run
git.

Input: `/tmp/claude/static-checks/titles.tsv`, tab-separated: origin, class, title. Each title is
the title of one fix commit made by an automated review-and-fix loop on a TypeScript/Node codebase
(mocha and sinon tests, calmLogger logging). A title often joins several findings with "; ". Split
on "; " and classify each finding separately.

For each finding, decide which ONE static commit-time check, run over the staged diff's added lines
plus a repo grep where noted, would have caught the defect before review, or NONE. Be strict. Tag a
detector only if a mechanical pattern check could plausibly have flagged it. Anything that needs
semantics, intent, level choice or behaviour is NONE.

Existing checks (a finding here is a miss for that hook, not a new candidate):
- PROSE_HOOK: an unnamed quantifier, an unresolved pointer, a comment naming a symbol the file does
  not name, a comment block over 8 lines, or non-ASCII punctuation.
- DEAD_REF: a symbol the diff removed that is still named in a doc or comment.
- VACUOUS_TEST: an added test whose assertions are only call absence, bare existence, a bare throw
  or rejection, or a loose sinon.match.
- DOUBLE_CAST: `as unknown as` in production code.
- CASE_LIST: a logic fix in a review-and-fix run with no case list.

Candidates not built yet:
- CAST: `as X`, `as any`, or a non-null `!`.
- LOG_THROW: an error-level log beside a throw of the same failure.
- FINALLY_AWAIT: `await` inside `finally`.
- SWALLOW: a catch that continues with no comment saying why.
- COMMENT_SHAPE: a comment restating the next line, a changelog comment, a ticket id in a comment.
- METRIC: a counter added beside an error log.
- DUPLICATE: a helper or constant copied from one that exists.
- TEST_STUB_GAP: production code gains a call that the function's existing tests do not stub.
- NEW: a mechanical shape none of the above names. Write a short rule for it in the finding text
  column, in the form `NEW: <rule>`.
- NONE: needs judgement.

Write `/tmp/claude/static-checks/tags.tsv`, one line per finding: origin, class, detector, finding
text trimmed to 120 characters. Return only a summary table: per detector, total, and counts with
origin self-inflicted, precommit and branch, plus three example findings. List every NEW rule with
its count. Give the NONE count and the total.
