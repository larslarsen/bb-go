# BBGO-GOV-010 — Align Local Scanner Binaries with Go 1.27

Status: ACCEPTED

Reviewer: Lead Engineer/Reviewer — Codex

Implementation actor: Reviewer (governance/review publication)

Source baseline: `a1228a234287fd323f3dbbe416a9c9b3e9a433e5`

## Authorized Paths

- `tickets/BBGO-SEC-001.md`
- `docs/handoff/CODEX_LUNA_BBGO_SEC_001.md`
- `docs/security/BBGO-SEC-001-EVIDENCE.md`
- `docs/handoff/CURRENT_TASK.md`
- `tickets/BBGO-GOV-010.md`

## Acceptance

- The Govulncheck result is recorded as a toolchain/package-loading failure, not a
  vulnerability result.
- Embedded build metadata for all five tools is preserved in the evidence disposition.
- The correction rebuilds only the four Go-1.26 tools at unchanged pinned versions and
  exact disk-backed paths; CycloneDX is not reinstalled.
- Scanner execution resumes only after all five tools report Go 1.27.0.
- No source, test, workflow, dependency, artifact cleanup, or GitHub state changes.
- `git diff --check` passes for the authorized governance/evidence paths.

## Reviewer Acceptance

Accepted as a bounded local-toolchain correction.
