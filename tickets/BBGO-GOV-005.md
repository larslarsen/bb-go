# BBGO-GOV-005 — Correct Jr Dev Role to Codex Luna

Status: ACCEPTED

Reviewer: Lead Engineer/Reviewer — Codex

Implementation actor: Reviewer (governance-only publication)

Source baseline: `838083dfed68ae9bb793516513f715def062b3ad`

## Objective

Correct the Jr Dev actor from the obsolete Hermes label to Codex Luna and ensure the
active integration handoff is executable directly by `gpt-5.6-luna` without ephemeral
context.

## Authorized Paths

- `AGENTS.md`
- `TESTING.md`
- `docs/engineering/DEVELOPMENT_ROLES.md`
- `docs/handoff/CURRENT_TASK.md`
- `docs/handoff/HERMES_BBGO_SEC_002.md` (delete)
- `docs/handoff/CODEX_LUNA_BBGO_SEC_002.md` (add)
- `tickets/BBGO-SEC-001.md`
- `tickets/BBGO-SEC-002.md`
- `tickets/BBGO-GOV-005.md`

## Acceptance

- Standing and active governance name Jr Dev — Codex Luna using `gpt-5.6-luna`.
- No active handoff routes work to Hermes.
- Codex Luna retains integration, execution, evidence, commit, and push ownership and
  does not gain test-design or source-authoring authority.
- No implementation source, test source, dependency, or generated state changes under
  this governance correction.
- `git diff --check` passes for the authorized governance paths.

## Reviewer Acceptance

Accepted as a governance-only role correction. It supersedes the Jr Dev naming in
`BBGO-GOV-003` without rewriting that historical record.
