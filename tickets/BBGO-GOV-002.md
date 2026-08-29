# BBGO-GOV-002 — Adopt SQLite/Keel Test-First Strategy

Status: ACCEPTED

Reviewer: Lead Engineer/Reviewer — Codex

Implementation actor: Reviewer (governance-only publication)

Source baseline: `60b1189e1ec20b7213344c61f0cda29793626cb1`

## Objective

Make test-first development, test falsification, regression coverage, hostile-boundary
testing, real multi-node proofs, security scanning, release SBOM evidence, and coverage
ratchets standing requirements for future daemon implementation tickets.

## Authorized Paths

- `AGENTS.md`
- `TESTING.md`
- `docs/handoff/CURRENT_TASK.md`
- `tickets/BBGO-GOV-002.md`

## Acceptance

- The policy requires red-before-green evidence and falsification of important tests.
- It covers fuzz, property, failure-injection, race, regression, and real multi-node
  testing without mandating release builds on every push.
- It requires pinned source, dependency, and secret scanning plus release-time SBOM and
  artifact evidence, with ticketed handling of inherited findings and suppressions.
- No production or test source, dependency, generated state, or network behavior changes.
- `git diff --check` passes for the authorized paths.

## Reviewer Acceptance

Accepted as a governance-only testing baseline. No implementation task is authorized.
