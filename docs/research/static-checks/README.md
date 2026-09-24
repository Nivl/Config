# Finding new commit-time checks

The review loop is expensive and most of what it finds needs judgement. A small share has a shape a
pattern over the staged diff can catch at commit time, and those are cheaper to deny than to review.
This directory is how a candidate is found and measured before it becomes a hook. The hooks built
this way are described in `shared_config/.claude/AGENTS.md`, "The commit-time code check".

## Where candidates come from

1. **The Final Report's Static-check candidates block.** Review roles fill a `pattern` field on a
   finding that a mechanical check could have caught, and each `review-and-fix` run lists them. A
   pattern that appears in several runs is a candidate. A pattern marked `covered by <hook>` is a
   miss for that hook, and the hook needs tuning rather than a sibling.
2. **A periodic sweep of the run logs**, every ten runs or so, for what the roles did not mark:

   ```
   mkdir -p /tmp/claude/static-checks
   python3 docs/research/static-checks/extract_titles.py > /tmp/claude/static-checks/titles.tsv
   ```

   Then give [TAGGER.md](TAGGER.md) to one Sonnet agent. Its summary counts findings per detector,
   including hook misses and `NEW:` rules.

## Measuring a candidate

A check is worth building when it would have caught several findings and rarely fires on human
commits. Measure the second number on a real repo before writing the hook:

```
python3 docs/research/static-checks/scan.py ~/Dev/repos/github.com/calm/api 300
```

Add the candidate to `CANDIDATES` in [scan.py](scan.py) first. Read the samples as well as the rate,
since a human commit that fires the pattern can still be a real violation.

The first sweep, 2026-09-23, over 948 findings and 300 api commits:

| Check | Findings it would have caught | Fires on master commits | Built |
|---|---|---|---|
| Comment shape (length, dashes) | 35 | length already in the prose hook | punctuation added |
| Dead reference | 18 | 1.7% | yes |
| Vacuous test | 17 | 1.0% | yes |
| Swallowed error | 13 | needs judgement | no |
| Duplicated helper | 12 | needs a clone detector | no |
| Log and throw | 6 | 3%, allowed case common | no |
| Cast or `!` | 5 | 11% of production commits | no |
| `as unknown as` in production | 0 | 2.3% | yes, on AGENTS.md's rule |

829 of the 948 needed judgement, so checks trim the edges of the review and do not replace it.

## After building or changing a check

Replay it over past commits, so the firing rate in the hook matches what was measured:

```
python3 docs/research/static-checks/replay.py ~/Dev/repos/github.com/calm/api 300
```

Update the table above and the AGENTS.md section with the new rate.
