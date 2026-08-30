# Hermes Integration Handoff — BBGO-SEC-002

You are **Jr Dev — Hermes**, using the best reliable free Nous Portal model currently
available. This file is the complete durable prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Read completely before acting:

1. `AGENTS.md`
2. `TESTING.md`
3. `docs/engineering/DEVELOPMENT_ROLES.md`
4. `docs/security/DEPENDABOT_TRIAGE_2026-08-29.md`
5. `tickets/BBGO-SEC-002.md`
6. `docs/handoff/GROK_BUILD_BBGO_SEC_002.md`, including its delivered source report

Inspect the Grok source drop against the authorized paths. You do not design or author
tests and you do not repair source. If the drop is incomplete or out of scope, record the
problem and stop without Git.

If the drop is conforming, own the complete red/green/falsification and acceptance
sequence in `BBGO-SEC-002`. The test-only red state may be reconstructed from the source
drop by temporarily holding the `qa/` deletion outside the worktree; restore the exact
drop before green. Preserve actionable output, but do not create untracked copies inside
the repository.

Author `docs/security/BBGO-SEC-002-EVIDENCE.md` with all evidence enumerated by the
ticket. Update `docs/handoff/CURRENT_TASK.md` to identify `BBGO-SEC-002`, state
`AWAITING REVIEW`, name Hermes as actor, link the ticket and evidence, list the commit
under review, and state that no further implementation is authorized.

Stage only the ticket-authorized source, deletion, evidence, and current-task paths.
Commit with message `security: retire inherited marketplace QA` and push `master`.
Report the exact commit hash and remote push result, then stop. You do not accept the
ticket and do not start `BBGO-SEC-001`.
