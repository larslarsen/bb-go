# Codex Luna Integration Handoff — BBGO-SEC-001

You are **Jr Dev — Codex Luna**, using `gpt-5.6-luna`. This file is the complete durable
prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Read completely before acting:

1. `AGENTS.md`
2. `TESTING.md`
3. `docs/engineering/DEVELOPMENT_ROLES.md`
4. `tickets/BBGO-SEC-001.md`
5. `docs/handoff/GROK_BUILD_BBGO_SEC_001.md`, including its delivered source report

Inspect the Grok source drop against the authorized paths. Do not design or author tests
and do not repair source. Stop without Git if the drop is incomplete or out of scope.

For a conforming drop, own the complete red/green/falsification and acceptance sequence
from `BBGO-SEC-001`. Reconstruct red by integrating only the test source before the
checker/workflows, then integrate the production drop for green. Use temporary
directories outside the repository for tool binaries, the Linux daemon, and SBOM. Never
record a secret value.

If any scanner reports a finding, stop before Git and write only redacted triage metadata
to `docs/security/BBGO-SEC-001-EVIDENCE.md`; do not suppress, baseline, dismiss, or repair
it. Otherwise record every required command, version, result, SBOM metric/hash, and
falsification result.

Update `docs/handoff/CURRENT_TASK.md` to `AWAITING REVIEW`, link the ticket and evidence,
identify the commit under review, and state that no further implementation is authorized.
Stage only the ticket-authorized source, evidence, and current-task paths. Commit with
message `security: add daemon scanning and SBOM evidence` and push `master`. Report the
exact commit hash and push result, then stop. You do not accept the ticket or begin
remediation outside it.
