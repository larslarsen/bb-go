# BBGO-GOV-009 — Move Security Work Off RAM-Backed `/tmp`

Status: ACCEPTED

Reviewer: Lead Engineer/Reviewer — Codex

Implementation actor: Reviewer (governance/review publication)

Source baseline: `9cf097e73bb0a08e8d851d5d22b29f69c809523b`

## Authorized Paths

- `AGENTS.md`
- `tickets/BBGO-SEC-001.md`
- `docs/handoff/CODEX_LUNA_BBGO_SEC_001.md`
- `docs/security/BBGO-SEC-001-EVIDENCE.md`
- `docs/handoff/CURRENT_TASK.md`
- `tickets/BBGO-GOV-009.md`

## Acceptance

- The RAM-drive risk and interruption are recorded without claiming scanners ran.
- Existing completed binaries are preserved at one explicit disk-backed path rather than
  reinstalled or deleted.
- All remaining local tool, Go cache/temp, binary, and SBOM paths are exact and
  disk-backed; local `/tmp` and `mktemp` are prohibited for this ticket.
- Task directories are left in place and no recursive cleanup is authorized.
- No developer source, test source, workflow, dependency, or GitHub state changes.
- `git diff --check` passes for the authorized governance/evidence paths.

## Reviewer Acceptance

Accepted as an owner-directed resource-safety correction.
