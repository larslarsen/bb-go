# BBGO-GOV-006 — Accept QA Retirement and Authorize Security Evidence

Status: ACCEPTED

Reviewer: Lead Engineer/Reviewer — Codex

Implementation actor: Reviewer (governance/review publication)

Source baseline: `5289c564490a54f1adc5be1d451277d2576f7090`

## Objective

Record final local and remote acceptance of `BBGO-SEC-002`, then authorize the pinned
scanner, immutable-Action, reduced-trigger, and manual SBOM work in `BBGO-SEC-001` with
complete Grok Build and Codex Luna handoffs.

## Authorized Paths

- `tickets/BBGO-SEC-002.md`
- `docs/security/BBGO-SEC-002-EVIDENCE.md`
- `docs/security/DEPENDABOT_TRIAGE_2026-08-29.md`
- `tickets/BBGO-SEC-001.md`
- `docs/handoff/GROK_BUILD_BBGO_SEC_001.md`
- `docs/handoff/CODEX_LUNA_BBGO_SEC_001.md`
- `docs/handoff/CURRENT_TASK.md`
- `tickets/BBGO-GOV-006.md`

## Acceptance

- `BBGO-SEC-002` records passed remote Go and dependency-graph runs and zero open alerts.
- No alert dismissal or unsupported exploitability claim is recorded.
- `BBGO-SEC-001` has an exact current baseline, paths, tools, commands, failure policy,
  and durable source/integration prompts.
- Existing Go CI is included in immutable pinning and usage reduction.
- No implementation source, test source, dependency, or generated artifact changes.
- `git diff --check` passes.

## Reviewer Acceptance

Accepted as a bounded review and governance publication.
