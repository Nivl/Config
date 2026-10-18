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
  required: ['id', 'title', 'file', 'line_range', 'severity', 'category', 'description', 'suggested_fix', 'role_agreement'],
  properties: {
    id: { type: 'string' },
    title: { type: 'string' },
    file: { type: 'string' },
    line_range: { type: 'string' },
    // The role prompt asks for a severity and the callers order, floor and
    // count on it. A schema without the field silently drops it, which one
    // run logged as "severity: not recorded".
    severity: { type: 'string', enum: ['critical', 'major', 'minor', 'suggestion'] },
    category: { type: 'string' },
    ticket_id: { type: ['string', 'null'] },
    description: { type: 'string' },
    suggested_fix: { type: 'string' },
    confidence: { type: 'null' },
    role_agreement: { type: 'integer' },
    citation_verified: { type: ['boolean', 'null'] },
    permalink: { type: ['string', 'null'] },
    // A regex or one-line rule over added diff lines that would have caught
    // this finding before review, or null. The fix loop's Final Report lists
    // these, and a pattern that repeats across runs is a hook candidate.
    pattern: { type: ['string', 'null'] },
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
const dropTicket = (rs) => (args.skip_ticket ? rs.filter((r) => r !== 10) : rs)

// Each instance runs args.active_roles unless args.instance_roles names a set
// for it. A caller that wants two identical instances passes no instance_roles
// and nothing changes for it. A caller that wants a full first instance and a
// narrow second one passes instance_roles: { '2': [11, 9, 2] }. Measured on
// 45 attributed fixes, 41 were raised by two or more roles, so a second full
// instance mostly re-finds what the first already found, and its cost is
// half the fan-out. The narrow set keeps triangulation where sole-raiser
// fixes actually came from.
const rolesFor = (inst) => dropTicket((args.instance_roles ?? {})[String(inst)] ?? args.active_roles)

const jobs = []
const rolesByInstance = {}
for (let inst = 1; inst <= args.instances; inst++) {
  const rs = rolesFor(inst)
  rolesByInstance[String(inst)] = rs
  for (const role of rs) jobs.push({ inst, role })
}
log(`dispatching ${jobs.length} role agents: ${Object.entries(rolesByInstance).map(([i, rs]) => `inst${i}=[${rs.join(',')}]`).join(' ')}`)

// Putting the shared fragment first instead was tried and measured. The
// harness caches at the end of each whole prompt, so roles with different
// tails still each wrote the fragment, about 59K tokens apiece.
//
// The first line of every prompt is a machine-readable stamp, because the
// harness stores no label for a workflow agent. Its transcript's first record is
// the prompt verbatim, and that is the only place inst and role survive the run.
// The caller's usage accounting greps this line to join a transcript's token
// counts back to the role that spent them. attempt distinguishes a retry from the
// launch it replaced, so a role that died once shows both costs. args.tag is
// whatever the caller needs to tell this invocation apart from the last one on
// the same target, such as an iteration number.
const stamp = (j, attempt) =>
  `<!-- review-roles inst=${j.inst} role=${j.role} attempt=${attempt} tag=${args.tag ?? 'none'} target=${args.target} -->`

// Roles 4 and 10 read GitHub, Jira and Datadog through MCP tools. The rest need
// only the read tools and run on the lite agent, whose restricted tool list
// keeps about 25K tokens of tool and skill listings out of each prompt. See the
// lite agent file for the measurement.
const MCP_ROLES = [4, 10]
const agentTypeFor = (role) => (MCP_ROLES.includes(Number(role)) ?'in-depth-review-role' : 'in-depth-review-role-lite')

// args.examined lists what earlier iterations of the same run already dropped
// or the user already decided. Roles see each iteration fresh, and one run's
// fifth iteration raised fifteen findings of which ten were such repeats.
const examinedBlock = args.examined
  ? `

## Already examined in this run

These were raised in an earlier iteration and either dropped or decided by the user. Do not raise
one again unless you have evidence the earlier finding lacked, such as a new commit on its lines or
a path that reaches it which it did not name. When you do, say in the description what is new.

${args.examined}`
  : ''

const run = (j, attempt) =>
  agent(
    `${stamp(j, attempt)}

${args.common_fragment}

${args.role_prompts[String(j.role)]}

## Target

${args.target} (${args.mode} mode). You are instance ${j.inst} of ${args.instances}. Other instances
run the same role independently and you must not coordinate with them.

Return your findings per the schema. Leave \`confidence\` as null. It is scored downstream by a
different model, and a number you invent would collapse that separation.

Set \`pattern\` only when a mechanical check over the diff's added lines could have caught the
finding with no judgement: a regex, or a one-line rule such as "a symbol removed from code but
still named in a markdown file". Name what to match, not the fix. Leave it null for anything that
needs you to understand intent, behaviour or a caller, which is most findings.${examinedBlock}`,
    {
      label: `inst${j.inst}:role${j.role}`, phase: 'Review', schema: ROLE_OUTPUT, agentType: agentTypeFor(j.role),
      // args.instance_models overrides the agent file's model pin for one
      // instance, for an A/B of a cheaper tier against the pinned one on the
      // same diff. Effort stays the agent file's.
      ...((args.instance_models ?? {})[String(j.inst)] ? { model: args.instance_models[String(j.inst)] } : {}),
    },
  )

// One retry per role that returned nothing, then null is final. A stall is
// usually transient, and one relaunch is cheap. Unbounded relaunches would be
// the same bug with extra bookkeeping.
const results = await parallel(
  jobs.map((j) => async () => {
    let out = await run(j, 1)
    if (!out) {
      log(`inst${j.inst} role${j.role} returned nothing, retrying once`)
      out = await run(j, 2)
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

// roles_by_instance is what each instance actually ran, after skip_ticket and
// any per-instance override. A caller computing roles_missing per instance
// compares against this, not against active_roles, or a narrow second instance
// reads as having lost every role it was never asked to run.
return { results, instances: args.instances, roles_by_instance: rolesByInstance }
