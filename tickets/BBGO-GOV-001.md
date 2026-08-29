# BBGO-GOV-001 — Establish Reviewer and Grok Build Workflow

Status: ACCEPTED

Reviewer: Lead Engineer/Reviewer — Codex

Implementation actor: Reviewer (governance-only publication)

Source baseline: `c533b7023a4b7bf0343fdaf5294ac4c38b806791`

## Objective

Establish the compact reviewer/Sr Dev — Grok Build workflow already used by the owner's
other projects without authorizing a production implementation task.

## Authorized Paths

- `AGENTS.md`
- `docs/handoff/CURRENT_TASK.md`
- `tickets/BBGO-GOV-001.md`

## Acceptance

- The roles, repository boundary, workflow, and stop condition are explicit.
- Grok Build is restricted to future reviewer-bounded source and test work.
- No production source, test source, dependency, generated state, or network behavior is
  changed.
- `git diff --check` passes for the authorized paths.

## Reviewer Acceptance

Accepted as a governance-only baseline. No next implementation task is authorized.
