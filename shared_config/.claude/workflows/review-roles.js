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
