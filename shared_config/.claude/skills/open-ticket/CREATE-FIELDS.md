# Create fields

Step 10 of [SKILL.md](SKILL.md). Read this before the first create, not after one fails. Every id here is a literal. A paraphrased field id produces a failed call, and a wrong one produces a ticket that looks right and carries the wrong data.

## Markdown works on create

`createJiraIssue` has `contentFormat`, enum `["markdown", "adf"]`, the same as `editJiraIssue`. No hand-built ADF is needed for `description`. Pass `contentFormat: "markdown"` explicitly on every call. The shared enum text hedges with "Defaults vary by tool when omitted". An unstated default can route a markdown string down the ADF path, and that produces the literal-markdown failure the next paragraph links out to. Stating the format explicitly costs nothing.

See [JIRA-FORMAT.md](../work-on/JIRA-FORMAT.md) for what survives the markdown-to-ADF conversion and for the read-back check that confirms it landed. That file is the single copy of both, so nothing here restates its rules.

## The complete createJiraIssue parameter list

Required is `cloudId`, `projectKey`, `issueTypeName`, `summary`. `additionalProperties: false`, 11 parameters. There is no `reporter`, `labels`, `components`, `priority`, `sprint`, `storyPoints`, `epicLink` or `issuelinks` parameter. `additionalProperties: false` means any of those names sent as a top-level key is not a call this schema accepts. Everything past the named parameters below goes through `additional_fields`.

| Parameter | Notes |
|---|---|
| `cloudId` | required |
| `projectKey` | required, a key |
| `issueTypeName` | required, a name and not an id. Copy verbatim from `getJiraProjectIssueTypesMetadata`. |
| `summary` | required, plain text with no formatting |
| `description` | markdown string, max 32000, or an ADF doc |
| `contentFormat` | `markdown` or `adf` |
| `responseContentFormat` | `markdown` or `adf`. Controls the shape of what the call returns, not what it sends. |
| `assignee_account_id` | an accountId, not an email |
| `parent` | the issue key |
| `additional_fields` | the only route to every `customfield_*`. Also the only route to `labels` and `priority`, which this skill never sets. |
| `transition` | `{id: string}`. Not needed. A new GRO issue lands in To Do on its own. |

This skill never sets `transition`. To Do is where a freshly filed ticket belongs, so there is nothing for a create call to move it out of.

## The create-screen table

Copied verbatim from the design spec's field facts for project GRO. It is the whole reason three types need two calls.

| Field | id | Epic | Story | Task | Subtask | Bug | Bug Subtask |
|---|---|---|---|---|---|---|---|
| Description | `description` | no | yes | yes | yes | no | no |
| Story Points | `customfield_10028` | no | yes | yes | yes | no | no |
| Sprint | `customfield_10021` | no | yes | yes | yes | yes | no |
| Assignee | `assignee` | yes | yes | yes | yes | yes | no |
| Parent | `parent` | yes | yes | yes | required | yes | required |
| Labels | `labels` | yes | yes | yes | yes | yes | yes |
| Priority | `priority` | yes | yes | yes | yes | yes | yes |
| Components | `components` | no | no | no | no | no | no |
| Epic Link | `customfield_10014` | no | no | no | no | no | no |

## Per-type recipes

Story and Task take one call. `description` and `customfield_10028` sit on both create screens.

```
createJiraIssue({
  cloudId, projectKey: "GRO", issueTypeName: "Story",
  summary: "...", parent: "GRO-1234", contentFormat: "markdown",
  description: "...",
  additional_fields: { customfield_10028: 3, customfield_10021: 9511 }
})
```

There is no `Subtask` recipe here because this skill never emits `Subtask`. Step 7 of [SKILL.md](SKILL.md) carries the reason. The `Subtask` column stays in the create-screen table above, because that table documents the project and not this skill's choices. The subtask type this skill emits is `Bug Subtask`, and that one needs two calls.

Bug, Epic and Bug Subtask all need two calls, because both `description` and the points field are absent from their create screens. The first call carries whatever that type's screen has and this skill sets. The second is an `editJiraIssue` with `contentFormat: "markdown"` that adds the description and the points. The three first calls are not the same shape.

`editJiraIssue` is `additionalProperties: false` too, and its whole parameter list is `cloudId`, `issueIdOrKey`, `fields`, `contentFormat` and `responseContentFormat`. There is no `additional_fields` on it, and no `assignee_account_id` either. Everything the second call sets goes inside `fields`, keyed by field name or `customfield_*` id. Carrying the create call's shape over to the edit fails the call outright, and it fails after the create has already landed, so the issue exists with no description and no points.

| Type | First call carries | Second call adds |
|---|---|---|
| `Bug` | summary, parent, assignee, sprint | description, points |
| `Epic` | summary, assignee | description, Eng Weeks |
| `Bug Subtask` | summary, parent | description, points, assignee |

```
createJiraIssue({
  cloudId, projectKey: "GRO", issueTypeName: "Bug", parent: "GRO-1234",
  summary: "...", assignee_account_id: "...",
  additional_fields: { customfield_10021: 9511 }
})
editJiraIssue({
  cloudId, issueIdOrKey: "GRO-1235", contentFormat: "markdown",
  fields: { description: "...", customfield_10028: 5 }
})
```

```
createJiraIssue({
  cloudId, projectKey: "GRO", issueTypeName: "Epic",
  summary: "...", assignee_account_id: "..."
})
editJiraIssue({
  cloudId, issueIdOrKey: "GRO-1236", contentFormat: "markdown",
  fields: { description: "...", customfield_10503: 3 }
})
```

```
createJiraIssue({
  cloudId, projectKey: "GRO", issueTypeName: "Bug Subtask", parent: "GRO-1235",
  summary: "..."
})
editJiraIssue({
  cloudId, issueIdOrKey: "GRO-1237", contentFormat: "markdown",
  fields: { description: "...", customfield_10028: 1, assignee: { accountId: "..." } }
})
```

`Bug Subtask` has no `assignee` and no `reporter` on its create screen at all. That is why assignee moves to the second call. Whether it takes there is unproven. Step 11 checks it, and nothing here assumes the answer.

`Epic` gets no sprint. `customfield_10021` is not on its create screen, and an epic spanning sprints does not belong to one. `Bug Subtask` gets no sprint either, and it inherits its parent's board placement in practice.

## No priority and no labels

This skill sets neither one, which is why no recipe above carries them. GRO requires neither on create, so nothing fails for their absence. Nothing in the pipeline decides a value for either. No step infers a label and no step infers a priority, so any value sent from here would be invented after the one approval at Step 9, and the user would have approved a plan that did not contain it.

Labels are the worse half of that. They drive board filters, so a label nobody chose puts the ticket in a view nobody expected, which is the same failure the wrong-issue-type argument in Step 7 is about.

Either one can be added by hand once the tree is filed. That costs a few seconds per issue and it puts the choice with the person who has a reason for it.

## Never write

`customfield_10014`, Epic Link. It mirrors `parent` and sits on no create screen. Writing it produces a ticket that looks linked correctly while carrying a second, wrong linkage underneath.

`customfield_10016`, Story point estimate. It is the team-managed twin of `customfield_10028` and has one non-empty value in all of GRO. Writing it produces a ticket whose points estimate silently disagrees with the field everyone else reads.

Never send components or fixVersions. Neither field is on any GRO create screen, and `component is not EMPTY` returns zero across the project. The create tool's own docstring advertises a components example. A wrapper following that example forwards the field, and Jira rejects the whole call.

`customfield_10377`, Acceptance Criteria. Its name matches a template section heading exactly, so the run-time enumeration under Portability below offers it as the obvious home for that section. Acceptance criteria go in `description`, where every template already puts them. Whether `contentFormat` reaches a textarea custom field at all was never established, so writing it can leave literal `##` and `**` sitting in a field nothing in Step 11 reads back.

## Parent for every level

Write parent for every level, including Story to Epic and Bug to Epic. GRO is company-managed and still uses the unified `parent` field. There is no separate epic-link mechanism to reach for. Skip `parent` at any level, and that issue shows unparented on the epic's own board, as scope nobody is tracking against it.

The route is unproven for anything above a subtask, because the parameter's own docstring reads `Parent for subtasks`. Read `parent` back on the run's first epic child and report what it says. Step 11's per-type read-back already covers the field, so this adds no call. A route that silently drops the link would otherwise land the whole tree unparented before anyone looks.

## Portability

Every id in this file is specific to the GRO instance. Re-check them against `getJiraIssueTypeMetaWithFields(requiredFieldsOnly: false)` at run time, and resolve fresh ids for any project that is not GRO. Reusing GRO's ids on another project can target a field that project does not carry, or a different field entirely sitting under the same number, and either way the write lands wrong or the call fails.

## The two unproven write shapes

The sprint value. Try bare `9511` against `[9511]`. Confirm whichever one works on the run's first create, read the result back, then reuse that proven shape for every later call in the run.

Assignee on `Bug Subtask`. Attempt it through the second call's edit, since the create screen has no field for it. The value shape is unproven along with the route. The edit takes a raw Jira field object under `fields`, so `assignee: {accountId: "..."}` is the shape to try, and not the create call's flat `assignee_account_id`. Report plainly whether it stuck.

Both of these get confirmed once against a real write, then reported in the run's final report. A guessed field shape here causes a real failed create.

Use a `GRO` key in every example. The recipes above use a range of GRO keys on purpose, because a two-call type's second call is an `editJiraIssue` that must target the key its own create call returned, not the parent's. GRO is not the only project key on this site, which is what the Portability section above is for. Do not copy an example key out of another skill's documentation. A key from a project this site cannot resolve makes the create call fail.
