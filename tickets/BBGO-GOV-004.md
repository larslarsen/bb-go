# BBGO-GOV-004 — Publish Security Triage and Durable Agent Handoffs

Status: ACCEPTED

Reviewer: Lead Engineer/Reviewer — Codex

Implementation actor: Reviewer (governance-only publication)

Source baseline: `f62d7879fca86db6080ea2ce4e83f6709f1351eb`

## Objective

Preserve the live Dependabot evidence, the decision to retire obsolete marketplace QA,
and the complete Grok Build and Hermes prompts in the repository before implementation
begins. Authorize the bounded `BBGO-SEC-002` implementation task.

## Authorized Paths

- `docs/security/DEPENDABOT_TRIAGE_2026-08-29.md`
- `docs/handoff/GROK_BUILD_BBGO_SEC_002.md`
- `docs/handoff/HERMES_BBGO_SEC_002.md`
- `docs/handoff/CURRENT_TASK.md`
- `tickets/BBGO-SEC-002.md`
- `tickets/BBGO-GOV-004.md`

## Acceptance

- All four current alerts are recorded with advisory and fixed-version metadata.
- The retained-versus-removed decision is supported by repository evidence.
- Both agent prompts are complete without relying on chat history.
- Source, integration, test execution, evidence, and Git ownership remain separated.
- No implementation source, test source, dependency, alert state, or generated artifact
  changes in this governance publication.
- `git diff --check` passes.

## Reviewer Acceptance

Accepted as a governance-only security triage and handoff publication.
