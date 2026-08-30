# BBGO-GOV-012 — Authorize Localhost Socket Test Execution

Status: ACCEPTED

Reviewer: Lead Engineer/Reviewer — Codex

Implementation actor: Reviewer (governance/review publication)

Source baseline: `c9b2089262e3c1c10cc047fe2b457ca01c0ed8f0`

## Authorized Paths

- `tickets/BBGO-SEC-001.md`
- `docs/handoff/CODEX_LUNA_BBGO_SEC_001.md`
- `docs/handoff/CURRENT_TASK.md`
- `docs/security/BBGO-SEC-001-EVIDENCE.md`
- `tickets/BBGO-GOV-012.md`

## Acceptance

- The failed red run is recorded as pre-assertion sandbox socket denial, not a source or
  test result.
- Only the exact targeted red/green and maintained race commands receive loopback
  ephemeral-port authority.
- Tests remain offline, in-process, credential-free, and use exact disk-backed paths.
- All restoration, hash, stop, no-cleanup, no-destructive-action, and no-Git rules remain.
- No developer source, test, workflow, dependency, artifact, or GitHub state change.

## Reviewer Acceptance

Accepted as a bounded test execution-environment correction.
