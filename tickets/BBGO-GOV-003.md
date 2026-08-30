# BBGO-GOV-003 — Establish the Complete Agent Role Workflow

Status: ACCEPTED

Reviewer: Lead Engineer/Reviewer — Codex

Implementation actor: Reviewer (governance-only publication)

Source baseline: `6088b79dd2523c710b7df0eaa18cd299f23b11a0`

## Objective

Add the missing Codex Spark implementation and Hermes integration roles, correct the
Grok Build and reviewer boundaries, and make test execution, evidence, and developer-drop
Git ownership unambiguous.

## Authorized Paths

- `AGENTS.md`
- `TESTING.md`
- `docs/engineering/DEVELOPMENT_ROLES.md`
- `docs/handoff/CURRENT_TASK.md`
- `tickets/BBGO-GOV-003.md`
- `tickets/BBGO-SEC-001.md`

## Acceptance

- Codex Spark is identified as GPT-5.3-Codex-Spark High and limited to bounded mechanical
  source and test-source authoring.
- Grok Build is limited to bounded senior source and test-source authoring.
- Hermes owns integration, command execution, evidence records, and developer-drop Git.
- Reviewer acceptance and next-ticket authorization remain exclusive.
- `BBGO-SEC-001` is present as a draft and does not authorize implementation.
- No production source, test source, dependency, CI execution, generated state, or
  network behavior changes.
- `git diff --check` passes for the authorized paths.

## Reviewer Acceptance

Accepted as a governance-only role correction and draft-ticket publication. No
implementation task is authorized.
