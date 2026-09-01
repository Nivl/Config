# Output: sub-agent JSON shape

### If invoked as a sub-agent (by `review-and-fix` or `pr-review`)

Return this exact JSON shape:

```json
{
  "scope": "<SCOPE_DESCRIPTION>",
  "mode": "pr" | "branch",
  "pr_head_sha": "<sha or null>",
  "raw": <true if --raw was set, else false>,
  "summary": "<short summary of what changed>",
  "findings": [
    {
      "id": "<file:line-range>",
      "title": "<one-line description>",
      "file": "<path>",
      "line_range": "<L<start>-L<end>>",
      "category": "<bug | AGENTS.md | history | prior PR | comment guidance | db | security | error-handling | test coverage | motivation | ticket | types>",
      "ticket_id": "<JIRA-ID this gap traces to, or null for non-ticket findings>",
      "description": "<full text>",
      "suggested_fix": "<text or code snippet>",
      "confidence": <0..100, or null when unscored>,
      "role_agreement": <1..12>,
      "citation_verified": <true | false | null>,
      "unscored": <true when no scorer produced this finding's confidence; omit or false otherwise>,
      "permalink": "<github blob URL with full SHA, if available; null otherwise>"
    }
  ],
  "roles_launched": [<role numbers actually spawned, after gates and any --roles subset>],
  "roles_missing": [
    {
      "role": <number>,
      "reason": "<launch failed | empty response | unparseable | errored | skipped by harness | no notification received>"
    }
  ],
  "coverage": "complete | partial | impossible",
  "scoring": {
    "unique_findings": <count after pre-score dedup>,
    "findings_scored": <count of findings that came back with a score; omit when deferred>,
    "scorers_spawned": <count of scoring sub-agents actually spawned; informational only>,
    "complete": <true when findings_scored == unique_findings, else false; omit when deferred>,
    "deferred": <true only when --defer-scoring was passed; omit otherwise>
  },
  "tickets_examined": [
    { "id": "<JIRA-ID>", "gaps": <count of surviving ticket findings for this id>, "status": "ok | gaps | unread" }
  ],
  "ticket_review": { "status": "ran | skipped | denied | unavailable", "note": "<reason when denied/unavailable, else null>" },
  "skipped_reason": "<if the skill bailed out early, why; otherwise omit>"
}
```

`roles_launched`, `roles_missing`, and `coverage` are **required.** Emit them even when
`roles_missing` is empty and `coverage` is `"complete"`. `coverage` is `"partial"` whenever
`roles_missing` is non-empty. A caller that sees `"partial"` knows not to read a short findings
list as a clean bill of health. A run whose launch calls all failed with the `Agent` tool present is
one of those partial runs. Every role it could not launch sits in `roles_missing` with reason
`launch failed`, `roles_launched` is empty, and `coverage` is `"partial"`. `coverage` is
`"impossible"` only when no reviewer role could be launched at all, because the `Agent` tool is
absent from this context. That is not a degraded review. It is the absence of one. A caller must
never map `"impossible"` onto `"partial"`, never fold it into an unavailable-reviewer kind, never let
it satisfy a stop rule, and never report it as coverage of any kind. Callers aggregating several
instances (`pr-review`, `review-and-fix`) rely on these fields to avoid asserting coverage nobody
delivered.

The no-fanout abort has one exact payload. When the `Agent` tool is unavailable so that no reviewer
role could be launched (Step 1 of [SKILL.md](SKILL.md)), these five fields carry exactly these
values:

```json
{
  "coverage": "impossible",
  "roles_launched": [],
  "roles_missing": [],
  "findings": [],
  "skipped_reason": "no fanout: the Agent tool is unavailable in this context, so no reviewer role could be launched"
}
```

`roles_missing` stays empty here rather than listing every role in `<ROLE_SET>`. No launch was ever
attempted, because there was no tool to attempt one with, so no role went missing. The hole is the
whole run, and `coverage` is the field that reports it. A run that DID attempt its launches and had
them fail is a different case. Those roles go in `roles_missing` with reason `launch failed`, which
makes the run `partial`. Whether a launch was attempted is the discriminator, not whether
`roles_launched` came out empty. That object is the entire return value. Emit no other field. The `scoring`, `ticket_review`, and
`tickets_examined` rules below describe a run that reached Step 2, and a no-fanout abort never does.

`scoring` is **required in every run that reaches Step 2**, and it exists so a caller can tell a
filtered result from an unfiltered one. `scoring.complete: false` means the confidence numbers in
`findings` did not all come from the two-stage process and must not be trusted as a filter. A caller
seeing it should treat the run as leads, not conclusions. Never omit the block to make a run look
clean. Any finding carrying `unscored: true` has `confidence: null` and sits below every threshold by
construction.

`complete` keys on `findings_scored`, never on `scorers_spawned`. Scorers run in batches of about
ten findings, so `scorers_spawned` is roughly a tenth of `unique_findings` on a healthy run and a
comparison against it would report every run degraded. It stays in the block because a batch count
is useful in a run log, and it answers no question about coverage. One batch returning eight scores
for its ten findings leaves two findings unscored while the agent itself reported.

**`deferred: true` is a THIRD case, and it is neither `complete: true` nor `complete: false`.** It
means the caller passed `--defer-scoring`, so this skill scored nothing and the caller will score the
merged set itself. A deferred block carries no `complete` and no `findings_scored`, because both
would be zero and a reader keying on `complete: false` would then exclude every finding from a
perfectly healthy instance. That is the whole review. Every reader of this block handles three cases.

Findings from a deferred instance carry `confidence: null` and **do not** carry `unscored: true`.
That field means a scorer was owed and did not deliver, which is a different thing, and callers
exclude on it before merging. A deferred run is not a degraded run.

Each finding a caller has scored carries `scored_by`, one of `scorer`, `scorer-retry`, or
`inline-fallback`, naming which rung of the caller's recovery ladder produced its number. Absent on
a deferred instance's own output, since it produced no numbers.

`citation_verified` is `true` when the finding cites a commit / PR / branch that was checked and
resolved, `false` when it cites one that could not be verified, and `null` when there is nothing to
verify. Roles #3 and #4 set it at emit time, so it survives a skipped scoring stage. A `false`
caps the finding at 60 regardless of any score.

Populate `ticket_review` from Role #10's response: a findings list or `NO_ISSUES_FOUND` ->
`{ "status": "ran", "note": null }`; `TICKET_REVIEW_SKIPPED: access denied` ->
`{ "status": "denied", "note": "access denied" }`; `TICKET_REVIEW_UNAVAILABLE: <r>` ->
`{ "status": "unavailable", "note": "<r>" }`; `--skip-ticket` passed (role not launched) ->
`{ "status": "skipped", "note": null }`.

Populate `tickets_examined` from Role #10's `TICKETS_EXAMINED:` line: one entry per `<id>=<status>`,
with `gaps` = number of that ticket's surviving findings. When `--skip-ticket` was passed, Role #10
returned `NO_ISSUES_FOUND` with no `TICKETS_EXAMINED:` line, or `ticket_review.status` is `denied`/
`unavailable`, set `tickets_examined` to `[]`.

