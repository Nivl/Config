# Flatten the review fan-out onto a Workflow barrier

Design settled 2026-09-02. Covers `in-depth-review`, `pr-review`, `review-and-fix`, the sync tool,
and one new saved workflow.

## The defect, measured

Session `0304256d` in the `ml-pr-review` worktree, a `pr-review` run on PR #15892. Two in-depth
wrapper agents each launched 12 role agents. Every one of the 24 roles finished, between 11:07 and
11:15. Wrapper #2 was alive until 11:17 and wrapper #1 until 11:22. Wrapper #1 received completion
notifications for 5 of its 12 roles. Wrapper #2 reported 7 of 12. The main session's transcript
holds 105 `<task-notification>` blocks and every one of the 30 agent ids launched in the run, roles
included.

So a nested agent's completion notification is delivered to the session root, not to the agent that
spawned it. The wrapper waits for a result that went to someone else, gives up, and reports the role
silent. `COLLECTING.md` describes this at line 27 as a consequence of the parent ending its turn. The
parent here did not end its turn. It ran `bash true` 111 times to stay alive and still lost 7 of 12.
The routing is unreliable whether or not the parent is polling.

Three costs followed from it, and the first two were recorded in the run's own transcripts:

- Role 7 (OWASP) finished with 332 KB of output in both instances and was reported silent in both.
- Wrapper #1 ran the database role's greps on the diff itself, which `AGGREGATING.md` bans.
- Wrapper #1's final output carries three PR numbers that role 4 found and that appear in none of
  the wrapper's logged inputs. Whether the harness delivered them unlogged or the wrapper produced
  them without receiving them cannot be settled from the transcript. Neither reading is acceptable.

The 111 `true` calls are the price of the polling protocol. Each is a model call against a
transcript approaching 1 MB.

## Why the Workflow tool, and why not more prose

`COLLECTING.md` is 77 lines of instructions trying to make an agent poll a mailbox that is not
receiving its mail. A fourth wording will not fix routing.

`Workflow`'s `parallel()` is a barrier in code. Each `agent()` call resolves to that agent's output
or to `null`, deterministically, with no notification to route and no turn to keep alive.
`work-on` Step 2 already fans out through it, and `AGENTS.md`'s "Fan-out skills must run in the
main thread" section names that as the blessed shape: a workflow whose agents are all leaves, driven
from the main thread. `in-depth-review-role` is a leaf and is explicitly allowed inside a workflow.

The routing bug only bites one level down. When `in-depth-review` runs standalone its orchestrator
IS the session root, so its roles' notifications arrive. That is why standalone worked and nested did
not, and it is why the nesting is the whole problem.

## Decisions taken

**One dispatch, three callers.** A saved workflow runs the roles. Standalone `in-depth-review`,
`pr-review`, and `review-and-fix` all invoke it. There is no Agent-tool role dispatch left anywhere,
so a role-prompt or collection change lands once.

**Roles only.** gh-style-review, the `review-scorer`, and `pr-review`'s approach pair are spawned
from the main thread today, where routing works, and they stay as they are. Moving them into the
workflow is a follow-on with two unknowns (whether a workflow agent has the `Skill` tool, and whether
dedup as a leaf agent holds quality) and is not part of this design.

**Dedup stays in the callers.** Grouping two findings as "substantially the same problem" is a
judgment, not a script. The workflow returns raw findings tagged by instance and role, and the caller
does one dedup pass across both dimensions. That replaces today's two layers, one inside each
instance and one across instances, with a single pass that computes `role_agreement` and
`cross_instance_agreement` from the tags.

**Retry inside the workflow, once per dead role.** `agent()` returning `null` is the whole
detection. One relaunch of that role, then `null` is final. Deterministic, per role, one line.

**Per-entry symlinks for the new directory.** `~/.claude/workflows/` will contain symlinks to each
file under `shared_config/.claude/workflows/`, exactly as `~/.claude/skills/` does. It will not
itself be a symlink.

## Design

### The workflow: `shared_config/.claude/workflows/review-roles.js`

Invoked as `Workflow({ name: 'review-roles', args })`.

`args`:

```
{
  target: '<PR number or commit range>',
  mode: 'pr' | 'branch',
  instances: 1 | 2,
  active_roles: [1, 3, 5, 9],          // role numbers to run
  role_prompts: { '1': '<contents of roles/01-agents-md.md>', ... },
  common_fragment: '<contents of roles/_common-fragment.md>',
  skip_ticket: bool
}
```

The caller reads the role files and passes their contents. A workflow script has no filesystem
access, and this is the same shape `work-on` uses for its lens prompts. `skip_ticket: true` removes
role 10 from `active_roles` before dispatch, matching today's flag.

The script:

```
export const meta = {
  name: 'review-roles',
  description: 'Run the review roles, N instances each, as leaf agents behind one barrier',
  phases: [{ title: 'Review' }],
}
const jobs = []
for (let inst = 1; inst <= args.instances; inst++)
  for (const role of args.active_roles) jobs.push({ inst, role })

const run = (j) => agent(
  `${args.common_fragment}\n\n${args.role_prompts[String(j.role)]}\n\nTarget: ${args.target} (${args.mode} mode)`,
  { label: `inst${j.inst}:role${j.role}`, phase: 'Review', schema: ROLE_FINDINGS_SCHEMA,
    agentType: 'in-depth-review-role' },
)

const results = await parallel(jobs.map((j) => async () => {
  let out = await run(j)
  if (!out) { log(`inst${j.inst} role${j.role} returned nothing, retrying once`); out = await run(j) }
  return { instance: j.inst, role: j.role, findings: out ? out.findings : null,
           tickets_examined: out?.tickets_examined ?? null }
}))

return { results, instances: args.instances, active_roles: args.active_roles }
```

`ROLE_FINDINGS_SCHEMA` is the shape a role returns today. It lives in the script so
`agent()` validates it. `findings: null` means the role died twice. An empty array means it ran and
found nothing. The two are different and downstream reads them differently.

**The hook constraint.** `hooks/deny-review-in-workflow.py` denies a workflow whose script text
contains `in-depth-review`, `pr-review`, `review-and-fix`, `work-on`, or `open-ticket`, comments
included. The script above contains none of them. `in-depth-review-role` is allowed by the hook's
trailing word boundary and is the one agent type it needs. The file is named `review-roles` so the
filename cannot trip the check either. This constraint is written at the top of the script as a
comment, worded without any of the five names.

### The sync tool

`internal/claude/sync/sync.go:241` has `linkedDirs = []string{"skills"}`. It becomes
`[]string{"skills", "workflows"}`. `LinkDirEntries` already does per-entry symlinking and already
prunes a link whose repo target is gone. `linkdir_test.go` gains a case for the second directory.
This is the only compiled code in the change and the only part with a real test.

### Standalone `in-depth-review`

Step 1 becomes: parse flags as today, read `roles/*.md` and `_common-fragment.md`, invoke
`review-roles` with `instances: 1`. Then Steps 2 through 4 as today: pre-score dedup, scoring in
batches, filter at 70, print. `roles_missing` is `results.filter(r => r.findings === null).map(r => r.role)`.
`coverage` is `partial` when that list is non-empty, `complete` otherwise. The `impossible` value and
`REVIEW_UNAVAILABLE_NO_FANOUT` stay, for the case where the `Workflow` tool itself is absent, which
is the same abort as before with a different missing tool.

`COLLECTING.md` stays, cut down to scorers only. Standalone Step 2 still spawns its batched
scorers through the Agent tool and collects their notifications, and that works because the
standalone orchestrator is the session root. The role half of the file, everything about collecting
reviewer roles, is deleted, because no roles are collected that way any more. The scorer half keeps
the protocol it has. The first draft of this spec deleted the whole file, which would have left
standalone scoring with no collection rule.

### `pr-review` and `review-and-fix`

Step 1 in each: read the role files, invoke `review-roles` with `instances: 2` and the iteration's
`active_roles`, and spawn gh-style-review as today. The `parallel()` barrier and the gh-style
`Agent` call both resolve before Step 2, so the "collecting sub-agent results" section of each
`AGGREGATING.md` shrinks to the gh-style case only.

`AGGREGATING.md` in each: one dedup pass over `results[*].findings` plus gh-style's findings.
`role_agreement` for a merged group is the count of distinct `role` values among its members from
one instance. `cross_instance_agreement` is the count of distinct `instance` values. `raised_by` is
the set of `instance` values, so the attribution ledger keeps working unchanged. Then the
`review-scorer` step and the threshold, as they are now.

`roles_missing` per instance comes from the nulls. The unioned `roles_missing` and its effect on row
1 and row 1c are unchanged in meaning. What changes is that the number is computed rather than
inferred from which notifications happened to arrive.

**Deleted.** `agents/pr-review-finder-indepth.md`, `pr-review/PROMPT-FINDER.md`,
`review-and-fix/PROMPT-IN-DEPTH.md`. No caller invokes the skill any more, so `--defer-scoring` is
deleted from `in-depth-review` and from `OUTPUT-JSON.md`. `scoring.deferred` goes with it. The
kind-level retry for in-depth (`reviewer_retries`, row 1b's in-depth half) is deleted, because a
role that died twice inside the barrier is not a kind that fell short. It is a role that is missing,
and `roles_missing` already carries that. gh-style keeps its kind-level retry, since it is still an
`Agent` spawn.

`agents/in-depth-review-role.md` stays. It is the leaf, and its `opus` `low` pin is what
`agentType` applies.

### The attribution ledger

Unchanged in shape. `raw` per instance is the sum of finding counts across that instance's roles.
`pooled` equals `raw`, because nothing is excluded per instance any more. The
`scoring.complete: false` exclusion, which produced `54 -> 1` in one measured run, cannot happen
here, because no instance scores. The ledger's `state` field is `reported` when the instance has no
null roles, `partial` when it has some, and `failed` when every role is null.

## What this does not change

- Which roles exist, what each one reads, and the prompt files under `roles/`.
- gh-style-review, its wrapper agent, and its `--raw` flag.
- The `review-scorer` agent, the five-rung ladder, and `scored_by`.
- The approach pair in `pr-review` Step 2.7.
- The thresholds: 70 standalone, 60 for `pr-review`, 50 for `review-and-fix`.
- The rows of `review-and-fix`'s Step 3 table other than row 1b's in-depth half.
- `work-on`'s own workflow, which this copies the shape of and does not touch.

## Verification

- `go test ./internal/claude/sync/...` passes with the `workflows` case added.
- `melvin-config claude sync` leaves `~/.claude/workflows/review-roles.js` as a symlink into the
  repo, and `~/.claude/workflows/` as a real directory. The user runs sync, since it escapes the
  sandbox.
- `tests/deny_review_in_workflow_test.sh` gains a case feeding the real `review-roles.js` body to the
  hook and expecting `allow`. This is the check that the script's wording cannot trip the deny.
- One `/in-depth-review` standalone run on a small range. `roles_launched` matches `active_roles`,
  and `roles_missing` is empty or names roles that show as `null` in the workflow's return.
- One `/pr-review` run. The attribution ledger shows `pooled == raw` on both instances, and the run
  produces no `bash true` calls in any agent transcript.
- `grep -rn "COLLECTING.md\|defer-scoring\|PROMPT-FINDER\|PROMPT-IN-DEPTH\|pr-review-finder-indepth" shared_config/.claude/` returns nothing.

## Two things to confirm during implementation

- That `agentType: 'in-depth-review-role'` in a workflow applies the agent file's `model` and
  `effort`. If it does not, pass `model: 'opus', effort: 'low'` explicitly in the script and say so
  in the agent file, so the pin has one owner.
- That the concurrency cap of `min(16, cpus - 2)` on 24 role agents does not push the slowest roles
  past any timeout `agent()` applies. `work-on` runs up to 21 agents through the same cap without
  incident, so this is a check rather than a worry.

## Deferred

Moving gh-style, dedup, and scoring into the workflow. Once `review-roles` has run clean a few
times, that is the natural next step and it would make each review one deterministic pass. It needs
the two spikes named above first.
