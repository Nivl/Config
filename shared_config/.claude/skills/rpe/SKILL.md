---
name: rpe
description: >
  Reverse Prompt Engineering (RPE) — refine vague or incomplete requests into a precise, LLM-optimized task specification before any implementation begins.
  Use this skill when a prompt contains `[rpe]`, or when the user explicitly asks to clarify, refine, or spec out a request before acting on it.
  Do NOT use for requests that are already well-specified and ready to implement.
---

# Reverse Prompt Engineering (RPE)

The goal is to surface what the user hasn't specified yet, then produce a complete task specification they can confirm before work begins.

## Workflow

1. **Analyse the request** — identify ambiguities, missing constraints, unstated assumptions, and edge cases.
2. **Ask clarifying questions** — use `ask_user` to ask one focused question at a time. Prioritise the most impactful unknowns first. Stop asking once you have enough to write a solid spec.
3. **Write the spec** — rewrite the original request as a complete, self-contained task specification optimised for an LLM to understand and execute. Include:
   - Clear goal and success criteria
   - Scope (what's in and out)
   - Constraints and non-negotiables
   - Relevant context gathered from questions
   - Edge cases and how to handle them
4. **Confirm** — present the spec and ask the user to confirm it is correct before proceeding. Do not start implementation until confirmed.

## Guidelines

- Prefer multiple-choice questions over open-ended ones when the answer space is predictable.
- Always include an "Other" option in multiple-choice questions to capture unexpected answers.
- If the request touches multiple domains, ask about the most architecturally significant one first.
- The spec should be written so that a fresh LLM with no prior context could execute it correctly.
- Keep the spec concise — include only what affects implementation decisions.
- The specs you write should be actionable and testable, not vague aspirations.
- If the user provides new information after confirming the spec, repeat the process to update the spec before proceeding.
- The specs should be written in a way that they can be directly fed into an LLM for execution, without needing further interpretation or assumptions.
