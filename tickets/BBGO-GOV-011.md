# BBGO-GOV-011 — Authorize Govulncheck Advisory-Database Access

Status: ACCEPTED

Reviewer: Lead Engineer/Reviewer — Codex

Implementation actor: Reviewer (governance/review publication)

Source baseline: `9beb22e138ff6c420078d89905585afba670ab4d`

## Authorized Paths

- `tickets/BBGO-SEC-001.md`
- `docs/handoff/CODEX_LUNA_BBGO_SEC_001.md`
- `docs/security/BBGO-SEC-001-EVIDENCE.md`
- `docs/handoff/CURRENT_TASK.md`
- `tickets/BBGO-GOV-011.md`

## Acceptance

- The failed run is recorded as pre-analysis sandbox DNS/network denial, not a
  vulnerability result.
- Only the exact source and binary Govulncheck commands receive external network access
  for `vuln.go.dev`; no credentials or other command receives expanded authority.
- Exact disk-backed paths and all stop/safety rules remain unchanged.
- No source, test, workflow, dependency, artifact cleanup, or GitHub state changes.
- `git diff --check` passes for the authorized governance/evidence paths.

## Reviewer Acceptance

Accepted as a bounded scanner execution-environment correction.
