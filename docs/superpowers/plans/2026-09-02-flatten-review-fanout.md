# Flatten Review Fan-out Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Run in-depth-review's roles as leaf agents behind a Workflow barrier, driven from the main thread, so a role's result can no longer be delivered to the wrong recipient.

**Architecture:** One saved workflow, `review-roles`, dispatches `instances x active_roles` leaf agents through `parallel()` and returns raw findings tagged `(instance, role)`, with `null` for a role that died twice. Standalone `in-depth-review`, `pr-review`, and `review-and-fix` all invoke it and do their own dedup, scoring, and filtering afterwards. The Agent-tool wrapper layer that nested the roles is deleted.

**Tech Stack:** Go (`internal/claude/sync`, testify), bash test harness under `tests/`, a JavaScript Workflow script, markdown skill prose under `shared_config/.claude/`.

**Spec:** `docs/superpowers/specs/2026-09-02-flatten-review-fanout-design.md` (untracked, read from disk)

## Global Constraints

- Prose follows `~/.claude/AGENTS.md`: plain ASCII, no em-dash or ` - ` or `:` joining two clauses, one thought per sentence, name a set rather than quantifying it.
- Never commit anything under `docs/superpowers/`. Gitignored, and the user wants the spec and plan local.
- The workflow script's text, comments included, must not contain any of `in-depth-review`, `pr-review`, `review-and-fix`, `work-on`, `open-ticket`. `in-depth-review-role` is allowed and is the one agent type the script names.
- The workflow file is named `review-roles.js`. Its `meta.name` is `review-roles`.
- `~/.claude/workflows/` must end up a real directory containing per-file symlinks, matching `~/.claude/skills/`. It must never itself be a symlink.
- Thresholds stay: 70 standalone, 60 `pr-review`, 50 `review-and-fix`.
- gh-style-review, `review-scorer`, and the approach pair are not touched.
- The user runs `melvin-config claude sync`. It escapes the sandbox and cannot be run from here.
- One commit per task, message in this repo's style.

---

### Task 1: Sync links the `workflows` directory per entry

**Files:**
- Modify: `internal/claude/sync/sync.go:241`
- Test: `internal/claude/sync/linkdir_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: after `melvin-config claude sync`, `~/.claude/workflows/<file>` is a symlink to `shared_config/.claude/workflows/<file>` for every file in that directory. Task 2 depends on this to make `Workflow({ name: 'review-roles' })` resolve.

- [ ] **Step 1: Write the failing test**

Append to `internal/claude/sync/linkdir_test.go`. The existing helper `newLinkDirEnv` hardcodes `skills`, so this case builds its own env:

```go
// TestLinkDirEntries_WorkflowsDirLinksEachFile - the workflows dir is
// linked per entry exactly as skills is, so ~/.claude/workflows/ is a
// real directory holding one symlink per repo file, never a symlink
// itself. A workflow entry is a single .js file rather than a
// directory, and the linker does not care which.
func TestLinkDirEntries_WorkflowsDirLinksEachFile(t *testing.T) {
	tmp := t.TempDir()
	p := state.NewPaths(filepath.Join(tmp, "config"), filepath.Join(tmp, "home"))
	repoWorkflows := filepath.Join(p.RepoDir, "workflows")
	require.NoError(t, os.MkdirAll(repoWorkflows, 0o755))
	require.NoError(t, os.MkdirAll(p.HomeDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repoWorkflows, "review-roles.js"),
		[]byte("export const meta = { name: 'review-roles' }\n"), 0o644))

	require.NoError(t, LinkDirEntries(p, "workflows", &bytes.Buffer{}, liveOpts()))

	homeWorkflows := filepath.Join(p.HomeDir, "workflows")
	info, err := os.Lstat(homeWorkflows)
	require.NoError(t, err)
	assert.True(t, info.IsDir(), "~/.claude/workflows must be a real directory")
	assert.Zero(t, info.Mode()&os.ModeSymlink, "~/.claude/workflows must not itself be a symlink")

	link, err := os.Readlink(filepath.Join(homeWorkflows, "review-roles.js"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(repoWorkflows, "review-roles.js"), link)
}

// TestLinkedDirs_IncludesWorkflows - the constant Sync iterates names
// both per-entry directories. Without this, LinkDirEntries works when
// called directly and Sync never calls it for workflows.
func TestLinkedDirs_IncludesWorkflows(t *testing.T) {
	assert.Contains(t, linkedDirs, "skills")
	assert.Contains(t, linkedDirs, "workflows")
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/claude/sync/ -run 'TestLinkDirEntries_WorkflowsDirLinksEachFile|TestLinkedDirs_IncludesWorkflows' -v`
Expected: `TestLinkDirEntries_WorkflowsDirLinksEachFile` PASS (the linker is generic), `TestLinkedDirs_IncludesWorkflows` FAIL on the `workflows` assertion.

- [ ] **Step 3: Add the directory to the constant**

In `internal/claude/sync/sync.go` line 241, change:

```go
	linkedDirs = []string{"skills"}             //nolint:gochecknoglobals // ordered constant
```

to:

```go
	linkedDirs = []string{"skills", "workflows"} //nolint:gochecknoglobals // ordered constant
```

- [ ] **Step 4: Run the whole package**

Run: `go test ./internal/claude/sync/...`
Expected: PASS, including `TestSync_CopyModeLinksSkillsAndMergesAgents` and every existing linkdir case.

- [ ] **Step 5: Check the sync's own listing of what it manages**

Run: `grep -n "curatedItems" internal/claude/sync/sync.go`
Read the function. It derives from `linkedDirs`, so `workflows` is now listed by `melvin-config claude sync --dry-run` with no further change. If it hardcodes a list instead, add `workflows` there too.

- [ ] **Step 6: Commit**

```bash
git add internal/claude/sync/sync.go internal/claude/sync/linkdir_test.go
git commit -F <message file>
```

Message covers: first saved workflow needs a home; per-entry symlinks like skills so `~/.claude/workflows/` is a real directory; the one-word change and the two tests that pin it.

---

### Task 2: The `review-roles` workflow and its hook test

**Files:**
- Create: `shared_config/.claude/workflows/review-roles.js`
- Test: `tests/deny_review_in_workflow_test.sh`

**Interfaces:**
- Consumes: Task 1, so the file is linked into `~/.claude/workflows/` after sync.
- Produces: `Workflow({ name: 'review-roles', args })` where `args` is `{ target, mode, instances, active_roles, role_prompts, common_fragment, skip_ticket }`, returning `{ results: [{ instance, role, findings | null, tickets_examined | null }], instances, active_roles }`. Tasks 3, 4, 5 invoke it with exactly this shape.

- [ ] **Step 1: Write the hook test case first**

Append to `tests/deny_review_in_workflow_test.sh`, before the final summary line. The hook emits nothing on allow, which the harness reports as `silent`:

```bash
# ---- Allow: the real saved role-dispatch script, fed by scriptPath ----
# This is the one workflow the review skills drive from the main thread. Its
# body names in-depth-review-role, which the trailing word boundary allows, and
# must name none of the five denied skills anywhere, comments included. Feeding
# the real file rather than a quote is what makes a later edit that adds a
# denied name fail here instead of at run time.
REAL_SCRIPT="$(cd "$(dirname "$0")/.." && pwd)/shared_config/.claude/workflows/review-roles.js"
assert_eq "silent_real_review_roles_script" "silent" \
  "$(decision scriptPath "$REAL_SCRIPT")"
assert_eq "silent_real_review_roles_by_name" "silent" \
  "$(decision name "review-roles")"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/deny_review_in_workflow_test.sh`
Expected: FAIL on `silent_real_review_roles_script`, because the file does not exist yet and the hook's `scriptPath` resolution reports it.

- [ ] **Step 3: Create the workflow**

Create `shared_config/.claude/workflows/review-roles.js`:

```javascript
// The role dispatch for the review skills, driven from the main thread.
//
// Why a workflow and not Agent-tool nesting. A nested agent's completion
// notification is delivered to the session root, not to the agent that spawned
// it. Measured in one run: two wrapper agents launched 24 roles between them,
// every role finished, and the wrappers received 5 and 7 of their 12 results.
// The other results sat in the root session where the wrappers could not see
// them, and one wrapper ran `bash true` 111 times waiting. parallel() below is a
// barrier in code. Each agent() resolves to output or to null, and nothing has
// to be routed or polled for.
//
// Wording constraint. A hook denies any workflow whose text names one of the
// fan-out skills. This file names only the leaf agent type, which is allowed,
// and its comments are written to keep it that way. Do not add a skill name
// here, even in prose. tests/deny_review_in_workflow_test.sh feeds this file to
// the hook and expects allow.

export const meta = {
  name: 'review-roles',
  description: 'Run the specialized review roles, N instances each, as leaf agents behind one barrier',
  phases: [{ title: 'Review', detail: 'one leaf agent per (instance, role), one retry for a role that returns nothing' }],
}

// The shape one role returns. Validated by agent() so a malformed return is a
// retry rather than a silent hole in the pool.
const FINDING = {
  type: 'object',
  required: ['id', 'title', 'file', 'line_range', 'category', 'description', 'suggested_fix', 'role_agreement'],
  properties: {
    id: { type: 'string' },
    title: { type: 'string' },
    file: { type: 'string' },
    line_range: { type: 'string' },
    category: { type: 'string' },
    ticket_id: { type: ['string', 'null'] },
    description: { type: 'string' },
    suggested_fix: { type: 'string' },
    confidence: { type: 'null' },
    role_agreement: { type: 'integer' },
    citation_verified: { type: ['boolean', 'null'] },
    permalink: { type: ['string', 'null'] },
  },
}

const ROLE_OUTPUT = {
  type: 'object',
  required: ['findings'],
  properties: {
    findings: { type: 'array', items: FINDING },
    tickets_examined: {
      type: 'array',
      items: {
        type: 'object',
        required: ['id', 'gaps', 'status'],
        properties: { id: { type: 'string' }, gaps: { type: 'integer' }, status: { type: 'string' } },
      },
    },
  },
}

// skip_ticket drops the ticket-intent role, matching the caller's flag of the
// same name. Done here so every caller gets the same rule.
const roles = args.skip_ticket ? args.active_roles.filter((r) => r !== 10) : args.active_roles

const jobs = []
for (let inst = 1; inst <= args.instances; inst++) {
  for (const role of roles) jobs.push({ inst, role })
}
log(`dispatching ${jobs.length} role agents: ${args.instances} instance(s) x ${roles.length} role(s)`)

const run = (j) =>
  agent(
    `${args.common_fragment}

${args.role_prompts[String(j.role)]}

## Target

${args.target} (${args.mode} mode). You are instance ${j.inst} of ${args.instances}. Other instances
run the same role independently and you must not coordinate with them.

Return your findings per the schema. Leave \`confidence\` as null. It is scored downstream by a
different model, and a number you invent would collapse that separation.`,
    { label: `inst${j.inst}:role${j.role}`, phase: 'Review', schema: ROLE_OUTPUT, agentType: 'in-depth-review-role' },
  )

// One retry per role that returned nothing, then null is final. A stall is
// usually transient, and one relaunch is cheap. Unbounded relaunches would be
// the same bug with extra bookkeeping.
const results = await parallel(
  jobs.map((j) => async () => {
    let out = await run(j)
    if (!out) {
      log(`inst${j.inst} role${j.role} returned nothing, retrying once`)
      out = await run(j)
    }
    if (!out) log(`inst${j.inst} role${j.role} returned nothing twice, recorded as missing`)
    return {
      instance: j.inst,
      role: j.role,
      findings: out ? out.findings : null,
      tickets_examined: out ? (out.tickets_examined ?? null) : null,
    }
  }),
)

const missing = results.filter((r) => r.findings === null)
if (missing.length) log(`${missing.length} role(s) missing after retry: ${missing.map((r) => `inst${r.instance}:role${r.role}`).join(', ')}`)

return { results, instances: args.instances, active_roles: roles }
```

- [ ] **Step 4: Run the hook test to verify it passes**

Run: `bash tests/deny_review_in_workflow_test.sh`
Expected: all cases `ok`, including both new ones.

- [ ] **Step 5: Parse-check the script**

Run: `sed -e 's/^export const meta = {/const meta = {/' -e 's/^return {/export default {/' shared_config/.claude/workflows/review-roles.js > /tmp/claude/rr.mjs && node --check /tmp/claude/rr.mjs && echo PARSE_OK`
Expected: `PARSE_OK`. This is the same rewrite `tests/work_on_validation_test.sh` uses, because `--check` treats the file as a module.

- [ ] **Step 6: Grep the file for the five denied names**

Run: `grep -ciE "in-depth-review[^-]|pr-review|review-and-fix|work-on|open-ticket" shared_config/.claude/workflows/review-roles.js`
Expected: `0`. The hook test in Step 4 is the real check. This is the fast one to run after any edit.

- [ ] **Step 7: Commit**

```bash
git add shared_config/.claude/workflows/review-roles.js tests/deny_review_in_workflow_test.sh
git commit -F <message file>
```

Message covers: the measured routing defect (5 and 7 of 12, 111 `true` calls); why a barrier in code and not a fourth wording of the polling protocol; the retry; the wording constraint and the test that feeds the real file to the hook.

---

### Task 3: Standalone `in-depth-review` drives the workflow

**Files:**
- Modify: `shared_config/.claude/skills/in-depth-review/SKILL.md:196-379` (Step 1)
- Modify: `shared_config/.claude/skills/in-depth-review/COLLECTING.md` (cut to scorers)
- Modify: `shared_config/.claude/skills/in-depth-review/OUTPUT-JSON.md` (drop `deferred`, keep `roles_missing`)

**Interfaces:**
- Consumes: Task 2's `Workflow({ name: 'review-roles', args })` and its return shape.
- Produces: the pattern for reading role files and building `args`, which Tasks 4 and 5 repeat verbatim.

- [ ] **Step 1: Rewrite Step 1's dispatch**

In `SKILL.md`, replace the body of `## Step 1` from the launch instructions through the `subagent_type: in-depth-review-role` paragraphs (lines 196 to 379, keep the role table at 334 to 345) with:

```markdown
## Step 1: Run the specialized reviewers behind one barrier

The roles run as leaf agents inside a saved workflow rather than as Agent-tool spawns from here.
The reason is a measured routing defect. A nested agent's completion notification is delivered to
the session root, not to the agent that spawned it, so any orchestrator that is not itself the root
loses results. This skill standalone IS the root, so the old dispatch worked here and failed
everywhere the skill was nested. One dispatch for every caller is what keeps that from recurring.

1. Read `roles/_common-fragment.md` and, for every role in `<ROLE_SET>`, `roles/NN-<name>.md` per
   the table below. A workflow script cannot read files, so the prompts travel in `args`.
2. Invoke the workflow:

   ```
   Workflow({
     name: 'review-roles',
     args: {
       target: '<PR number or commit range>',
       mode: '<pr | branch>',
       instances: 1,
       active_roles: <ROLE_SET as an array of numbers>,
       role_prompts: { '1': '<contents of roles/01-agents-md.md>', ... },
       common_fragment: '<contents of roles/_common-fragment.md>',
       skip_ticket: <true when --skip-ticket was passed>,
     },
   })
   ```

   Pass no `model` and no `effort`. The `in-depth-review-role` agent file pins `opus` at `low`
   and the workflow spawns by that `agentType`.
3. The call returns `{ results, instances, active_roles }`. Each entry of `results` is
   `{ instance, role, findings, tickets_examined }`. `findings` is an array when the role ran and
   `null` when it returned nothing twice. An empty array is a role that ran and found nothing.
   The two are different and the accounting below reads them differently.
4. `roles_launched` is `active_roles` from the return. `roles_missing` is every result whose
   `findings` is `null`, each as `{ role, reason: "returned nothing after one retry" }`.
   `coverage` is `partial` when `roles_missing` is non-empty and `complete` otherwise.
5. Pool every non-null `findings` array into one list, each finding tagged with its `role`, and
   continue to Step 2.

**If the `Workflow` tool is absent from your tool list**, you cannot run the roles at all. Emit
`coverage: "impossible"` with the `REVIEW_UNAVAILABLE_NO_FANOUT` line, exactly as the old dispatch
did when the `Agent` tool was absent. Same abort, different missing tool.

There is no collection protocol for roles any more. `parallel()` inside the workflow is the
barrier, and it resolves every agent to output or to `null` before the call returns. Nothing is
polled for, and no turn has to be kept alive.
```

Keep the role table (numbers, names, categories, prompt files) exactly as it is.

- [ ] **Step 2: Remove the deferral flag**

In `SKILL.md`, delete the `--defer-scoring` bullet from the flag list, the `^--defer-scoring$` parser line, and Step 2's sub-step 3 (the skip block that begins "When `--defer-scoring` is in effect"). Renumber Step 2's sub-steps so the batch-launch step is 3 again. Then change the MANDATORY paragraph back to:

```markdown
   **The scoring stage is MANDATORY and is not yours to perform.** Confidence is a second-stage
   judgment by a different model than the one that proposed the finding. That two-stage split is
   the whole reason a confidence number means anything here.
```

The flag existed so a nested caller could skip scoring and score after its merge. No caller
invokes this skill any more, so it has no caller.

- [ ] **Step 3: Cut `COLLECTING.md` to scorers**

Replace the opening paragraph and the "Shared by Step 1 ... and Step 2.1" framing with:

```markdown
# Collecting scorer results

Used by Step 2 to collect the batched scoring agents. The reviewer roles are no longer collected
this way. They run inside the `review-roles` workflow, whose `parallel()` is a barrier in code, and
this protocol does not apply to them.

Scorers are still Agent-tool spawns from this skill's own context, and this skill runs standalone
in the session root, so their notifications do arrive here. The protocol below is how to receive
them.
```

Delete every sentence that names roles, `roles_missing`, or Step 1. Keep the async-delivery
explanation, the numbered protocol, the give-up bound, and "An agent that never reports has
reported nothing." Replace "missing bookkeeping" with "mark the finding `unscored: true` with
`confidence: null`" wherever it appears.

- [ ] **Step 4: Update the JSON contract**

In `OUTPUT-JSON.md`, remove `"deferred"` from the `scoring` block and delete the three paragraphs
about `deferred: true` being a third case. Change the `roles_missing` `reason` enum to:

```
"reason": "<returned nothing after one retry | skipped by harness>"
```

The old values (`no notification received`, `launch failed`, `empty response`, `unparseable`,
`errored`) described Agent-tool failure modes that the barrier does not have.

- [ ] **Step 5: Verify**

Run: `grep -n "defer-scoring\|deferred" shared_config/.claude/skills/in-depth-review/*.md`
Expected: no output.

Run: `grep -n "subagent_type: in-depth-review-role\|Launch.*in parallel\|no notification received" shared_config/.claude/skills/in-depth-review/SKILL.md`
Expected: no output. The agent type is now named only inside the workflow.

Run: `grep -c "review-roles" shared_config/.claude/skills/in-depth-review/SKILL.md`
Expected: at least `2`.

- [ ] **Step 6: Commit**

```bash
git add shared_config/.claude/skills/in-depth-review/SKILL.md shared_config/.claude/skills/in-depth-review/COLLECTING.md shared_config/.claude/skills/in-depth-review/OUTPUT-JSON.md
git commit -F <message file>
```

Message covers: standalone worked only because it was the root; one dispatch for all callers; `--defer-scoring` deleted because its reason was the nesting; `COLLECTING.md` cut to scorers and why it stays.

---

### Task 4: `review-and-fix` drives the workflow

**Files:**
- Modify: `shared_config/.claude/skills/review-and-fix/SKILL.md:145-244` (Step 1)
- Modify: `shared_config/.claude/skills/review-and-fix/AGGREGATING.md` (collection, dedup, kind retry)
- Delete: `shared_config/.claude/skills/review-and-fix/PROMPT-IN-DEPTH.md`
- Delete: `shared_config/.claude/agents/pr-review-finder-indepth.md`

**Interfaces:**
- Consumes: Task 2's workflow, and Task 3's args-building pattern.
- Produces: the one-pass tag-based dedup wording, which Task 5 repeats.

- [ ] **Step 1: Rewrite Step 1's in-depth half**

In `SKILL.md` Step 1, replace the in-depth launch (the reviewer table row for in-depth, the
`subagent_type: pr-review-finder-indepth` instruction, and the `PROMPT-IN-DEPTH.md` reference) with:

```markdown
**The in-depth roles run inside the `review-roles` workflow, not as sub-agents.** Read
`in-depth-review/roles/_common-fragment.md` and every `in-depth-review/roles/NN-<name>.md` in
`<ACTIVE_ROLES>`, then invoke:

```
Workflow({
  name: 'review-roles',
  args: {
    target: '<TARGET_ARG>',
    mode: '<pr | branch>',
    instances: 2,
    active_roles: <ACTIVE_ROLES>,
    role_prompts: { '<n>': '<contents of that role file>', ... },
    common_fragment: '<contents of _common-fragment.md>',
    skip_ticket: <SKIP_TICKET>,
  },
})
```

Launch gh-style-review as an Agent-tool sub-agent in the same message, by
`subagent_type: pr-review-finder-ghstyle`, exactly as before. It is one agent spawned from this
thread, which is the session root, so its notification arrives here.

Why the two kinds are dispatched differently. Nesting the in-depth roles inside a wrapper agent
lost their results, because a nested agent's completion notification goes to the session root and
not to the wrapper. Measured on one run: wrappers received 5 and 7 of their 12 roles while every
role had finished. The workflow's `parallel()` is a barrier in code and has no notification to
route. gh-style spawns nothing, so it never had the problem.

The workflow returns `{ results, instances, active_roles }`. Each `results` entry is
`{ instance, role, findings, tickets_examined }`, with `findings: null` for a role that returned
nothing twice. That return is the in-depth kind's report. It cannot fall short as a kind, because
the barrier resolves every role before the call returns.
```

- [ ] **Step 2: Cut the collection section to gh-style**

In `AGGREGATING.md` "Collecting sub-agent results", keep the async protocol but scope every
sentence to the gh-style instance. Replace the opening with:

```markdown
## Collecting the gh-style result

The in-depth roles arrive as the `review-roles` workflow's return value and need no collecting.
This section covers the one Agent-tool spawn left, the gh-style instance.
```

Delete the sentences about unioning `roles_missing` across in-depth instances and about two roles
being silent in both instances. Replace with:

```markdown
`roles_missing` per in-depth instance is `results.filter(r => r.instance === N && r.findings === null)`.
It is computed from the workflow's return, not inferred from which notifications arrived. The
unioned `roles_missing` across instances keeps its meaning for row 1 and row 1c. A role that no
instance returned is still a hole, and the union of findings still cannot be used to claim
`complete`.
```

- [ ] **Step 3: Drop the in-depth kind retry**

In `AGGREGATING.md` "A missing reviewer is not a clean reviewer", the kind-level retry stays for
gh-style only. Add after the retry rule:

```markdown
The in-depth kind has no retry here, because it cannot fall short as a kind any more. The
`review-roles` workflow returns for every role, with `null` where a role returned nothing after its
own one retry inside the barrier. A null role is a missing ROLE, recorded in that instance's
`roles_missing`, and it is not a kind that failed to report. Relaunching the whole workflow for one
null role would rerun 23 roles that already answered.
```

- [ ] **Step 4: Collapse dedup to one pass**

In `AGGREGATING.md` "Pooling, deduplication, and merge", replace steps 1 and 2 with:

```markdown
1. **Pool every finding** from every non-null `results[*].findings` array and from the gh-style
   result into one flat pool. Each in-depth finding carries the `instance` and `role` of the
   result it came from. Each gh-style finding carries `source: "gh-style-review"` and no instance.

2. **One dedup pass, across roles and instances together.** Two findings are duplicates if they
   refer to the same file, have overlapping line ranges, and describe substantially the same
   problem. This used to be two layers, one inside each in-depth instance across its roles and one
   here across instances. The workflow returns raw per-role findings, so the layers collapse into
   this single pass, and both agreement counts fall out of the tags:
   - `role_agreement`: distinct `role` values among the group's members from any one instance,
     taking the larger instance's count.
   - `raised_by`: the SET of distinct `instance` values among the group's members.
   - `cross_instance_agreement`: the size of `raised_by`.
```

Step 3's field list keeps `raised_by` and `cross_instance_agreement` as already written, and drops
the sentence saying `role_agreement` is carried through from an incoming finding.

- [ ] **Step 5: Delete the dead files**

```bash
git rm shared_config/.claude/skills/review-and-fix/PROMPT-IN-DEPTH.md
git rm shared_config/.claude/agents/pr-review-finder-indepth.md
```

Then in `review-and-fix/SKILL.md`, remove the reviewer-table row and the tier-table row naming
`pr-review-finder-indepth`, and remove the "different flags" bullet's in-depth half, leaving the
gh-style `--raw` sentence.

- [ ] **Step 6: Verify**

Run: `grep -rn "PROMPT-IN-DEPTH\|pr-review-finder-indepth\|defer-scoring\|deferred" shared_config/.claude/skills/review-and-fix/`
Expected: no output.

Run: `grep -c "review-roles" shared_config/.claude/skills/review-and-fix/SKILL.md shared_config/.claude/skills/review-and-fix/AGGREGATING.md`
Expected: at least `1` in each.

Run: `bash tests/work_on_validation_test.sh`
Expected: `ok`. Unrelated, but it is the one skill suite in the repo and confirms nothing shared broke.

- [ ] **Step 7: Commit**

```bash
git add -A shared_config/.claude/skills/review-and-fix/ shared_config/.claude/agents/
git commit -F <message file>
```

Message covers: the measured defect; in-depth via the workflow and gh-style unchanged, and why they differ; two-layer dedup collapsed to one tag-based pass; the in-depth kind retry deleted because a null role is not a short kind; the wrapper agent and prompt file deleted.

---

### Task 5: `pr-review` drives the workflow

**Files:**
- Modify: `shared_config/.claude/skills/pr-review/SKILL.md:174-233` (Step 1)
- Modify: `shared_config/.claude/skills/pr-review/AGGREGATING.md` (collection, dedup)
- Delete: `shared_config/.claude/skills/pr-review/PROMPT-FINDER.md`

**Interfaces:**
- Consumes: Task 2's workflow, Task 3's args pattern, Task 4's dedup wording.
- Produces: nothing further.

- [ ] **Step 1: Rewrite Step 1**

In `SKILL.md` Step 1, replace the in-depth rows and the `PROMPT-FINDER.md` reference with the same
block as Task 4 Step 1, changing `<TARGET_ARG>` to `<PR>`, `mode` to `'pr'`, and `<ACTIVE_ROLES>`
to the full role list. Repeat the block in full here rather than pointing at Task 4:

```markdown
**The in-depth roles run inside the `review-roles` workflow, not as sub-agents.** Read
`in-depth-review/roles/_common-fragment.md` and every `in-depth-review/roles/NN-<name>.md`, then
invoke:

```
Workflow({
  name: 'review-roles',
  args: {
    target: '<PR>',
    mode: 'pr',
    instances: 2,
    active_roles: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12],
    role_prompts: { '1': '<contents of roles/01-agents-md.md>', ... },
    common_fragment: '<contents of roles/_common-fragment.md>',
    skip_ticket: <SKIP_TICKET>,
  },
})
```

Launch gh-style-review as an Agent-tool sub-agent in the same message, by
`subagent_type: pr-review-finder-ghstyle`, exactly as before.

Why the two kinds differ. Nesting the in-depth roles inside wrapper agents lost their results. A
nested agent's completion notification is delivered to the session root and not to the wrapper, and
one measured run had its two wrappers receive 5 and 7 of their 12 roles while every role had
finished. The workflow's `parallel()` is a barrier in code. gh-style spawns nothing and never had
the problem.

The return is `{ results, instances, active_roles }`, one `{ instance, role, findings, tickets_examined }`
per role per instance, with `findings: null` where a role returned nothing twice.
```

Update the skill's opening paragraph, which says "three parallel reviewer sub-agents", to say the
in-depth roles run inside a workflow and gh-style is the one sub-agent.

- [ ] **Step 2: Cut the collection section and collapse dedup**

In `AGGREGATING.md`, apply Task 4 Steps 2 and 4 with `pr-review`'s wording. The collection section
becomes gh-style only, `roles_missing` per instance is computed from nulls, and the pooling step
carries `instance` and `role` tags with `raised_by`, `role_agreement`, and
`cross_instance_agreement` derived from them. Write the text out in full in this file. The retry
section needs no change, because this skill already reports a miss rather than retrying.

- [ ] **Step 3: Delete the prompt file**

```bash
git rm shared_config/.claude/skills/pr-review/PROMPT-FINDER.md
```

Remove its reference from `SKILL.md`, and remove the in-depth half of "Why the sub-agents get
different flags", leaving the gh-style `--raw` half. Update the description in the frontmatter,
which names "three parallel reviewer sub-agents".

- [ ] **Step 4: Verify**

Run: `grep -rn "PROMPT-FINDER\|pr-review-finder-indepth\|defer-scoring\|deferred" shared_config/.claude/skills/pr-review/`
Expected: no output.

Run: `grep -c "review-roles" shared_config/.claude/skills/pr-review/SKILL.md shared_config/.claude/skills/pr-review/AGGREGATING.md`
Expected: at least `1` in each.

- [ ] **Step 5: Commit**

```bash
git add -A shared_config/.claude/skills/pr-review/
git commit -F <message file>
```

Message covers: same defect, same fix; the finder prompt deleted; the opening paragraph and description corrected.

---

### Task 6: Cross-file sweep and the sync handoff

**Files:**
- Read only, then fix whatever the sweep finds in the file that owns it.

**Interfaces:**
- Consumes: every task above.
- Produces: nothing.

- [ ] **Step 1: Nothing references the deleted things**

Run: `grep -rn "COLLECTING.md" shared_config/.claude/skills/ | grep -v "in-depth-review/SKILL.md\|in-depth-review/COLLECTING.md"`
Expected: no output. Only the standalone skill's scorer step reads it now.

Run: `grep -rn "pr-review-finder-indepth\|PROMPT-FINDER\|PROMPT-IN-DEPTH\|defer-scoring\|scoring.deferred\|deferred: true" shared_config/.claude/`
Expected: no output.

- [ ] **Step 2: The stale prose that names the old shape**

Run: `grep -rn "wrapper\|sub-agent 1 of\|sub-agent 2 of\|two in-depth instances\|2x in-depth" shared_config/.claude/skills/pr-review/ shared_config/.claude/skills/review-and-fix/ shared_config/.claude/AGENTS.md`
Read each hit. A sentence describing in-depth as a wrapped sub-agent, or counting "sub-agents"
with in-depth included, is now false and gets rewritten in place. gh-style is still a wrapped
sub-agent and sentences about it stay.

- [ ] **Step 3: AGENTS.md's fan-out section**

Run: `grep -n "in-depth-review\` cannot launch\|pr-review-finder-indepth\|DENIED_AGENT_TYPES" shared_config/.claude/AGENTS.md`
The section "Fan-out skills must run in the main thread" names `pr-review-finder-indepth` in
`DENIED_AGENT_TYPES`. That agent no longer exists. Remove it from the hook's list in
`hooks/deny-review-in-workflow.py`, from its test, and from the AGENTS.md sentence, then run
`bash tests/deny_review_in_workflow_test.sh` and expect `ok`. Add one paragraph to that section
saying the review skills now drive their roles through `review-roles` from the main thread, which
is the shape the section already blesses for `work-on-validate`.

- [ ] **Step 4: All three test suites**

Run: `go test ./internal/claude/sync/... && bash tests/deny_review_in_workflow_test.sh && bash tests/work_on_validation_test.sh`
Expected: all pass.

- [ ] **Step 5: Hand off the sync**

Tell the user to run `melvin-config claude sync` in a regular terminal, then verify:

Run: `ls -la ~/.claude/workflows/`
Expected: a real directory containing `review-roles.js -> /Users/melvin/.melvin/config/shared_config/.claude/workflows/review-roles.js`.

- [ ] **Step 6: One standalone run**

After sync, the user runs `/in-depth-review <small range>` in a repo. Check the output's
`roles_launched` equals the role set, and `roles_missing` is empty or names roles the workflow's own
`log()` lines reported as missing after retry. Then `/pr-review` on a small PR and confirm no agent
transcript under the session's `subagents/` contains `"command":"true"`.

- [ ] **Step 7: Commit any fixes the sweep found**

If Steps 1 to 4 were all clean and Step 3 changed nothing, there is nothing to commit. Say so
rather than inventing a change.

---

## Self-Review

**Spec coverage.** The sync change is Task 1. The workflow and its hook test are Task 2. Standalone
is Task 3, including the `--defer-scoring` deletion and the `COLLECTING.md` cut the spec's inline
correction called for. `review-and-fix` is Task 4 and `pr-review` is Task 5, each with the one-pass
dedup and the deleted wrapper artifacts. The in-depth kind retry deletion is Task 4 Step 3. The
attribution ledger needs no task, because `raised_by` is derived from the same `instance` tag the
dedup already reads. Verification is Task 6. The two confirm-during-implementation items from the
spec are exercised by Task 6 Step 6, the live runs.

**Placeholder scan.** Every prose insertion is written out. Task 5 Step 1 repeats the Task 4 block
in full rather than pointing at it. Task 5 Step 2 says to apply Task 4's steps but requires the text
to be written out in the file, which is the instruction, not a placeholder for one.

**Naming consistency.** `review-roles`, `Workflow({ name: 'review-roles', args })`, the `args`
keys `target mode instances active_roles role_prompts common_fragment skip_ticket`, the return
`{ results, instances, active_roles }`, and the result entry `{ instance, role, findings, tickets_examined }`
are spelled identically in Tasks 2 through 5. `agentType: 'in-depth-review-role'` appears only in
Task 2's script.

**One gap found and closed while reviewing.** The spec's first draft deleted `COLLECTING.md`
outright. Standalone Step 2 still collects Agent-tool scorers with it, so Task 3 Step 3 cuts it to
scorers instead, and the spec was corrected before this plan was written.
