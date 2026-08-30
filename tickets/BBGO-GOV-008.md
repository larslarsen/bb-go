# BBGO-GOV-008 — Prohibit Unreviewable Recursive Deletion

Status: ACCEPTED

Reviewer: Lead Engineer/Reviewer — Codex

Implementation actor: Reviewer (governance/review publication)

Source baseline: `5b5893008007559c285592545632b039dc4125b1`

## Authorized Paths

- `AGENTS.md`
- `tickets/BBGO-SEC-001.md`
- `docs/handoff/CODEX_LUNA_BBGO_SEC_001.md`
- `docs/security/BBGO-SEC-001-EVIDENCE.md`
- `docs/handoff/CURRENT_TASK.md`
- `tickets/BBGO-GOV-008.md`

## Acceptance

- The rejected recursive deletion and interruption are recorded without claiming it ran.
- Recursive or indirect destructive targets are forbidden in standing and active policy.
- The task requires no recursive cleanup; temporary state is left for system cleanup.
- Falsification uses an in-memory test rather than a temporary deletion tree.
- No developer source, test source, workflow, dependency, generated artifact, or GitHub
  state changes in this publication.
- `git diff --check` passes for the authorized governance/evidence paths.

## Reviewer Acceptance

Accepted as an owner-directed destructive-action safety correction.
