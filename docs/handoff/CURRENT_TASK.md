# Current Task

Ticket: BBGO-PAY-001

State: GROK TEST DROP REJECTED — SOL CORRECTION AUTHORIZED

Reviewer: Lead Engineer/Reviewer — Codex at XHigh

Correction actor: Principal Dev — Codex Sol (`gpt-5.6-sol`, High)

Integration actor: Jr Dev — Codex Luna (`gpt-5.6-luna`)

Production source baseline: `0560b6426b9af29a16a151dacc7c2f3021a3dc0d`

Governance head before this handoff: `01d709c62fa27cca4ec4869edada3581ced90daa`

Active handoff: [CODEX_SOL_BBGO_PAY_001_TESTS_CORRECTION_01.md](CODEX_SOL_BBGO_PAY_001_TESTS_CORRECTION_01.md)

[BBGO-PAY-001](../../tickets/BBGO-PAY-001.md) remains the only authorized
implementation task in this repository. Grok Build ran in the foreground after its time
gate, changed only the seven authorized test paths, copied the 231-line desktop oracle
byte-for-byte, and ran no Go, tests, Git, or network command.

The fixture and 20-line payment protocol-ID test are reviewer-accepted and frozen. The
five payment test files are rejected before execution in
[BBGO-PAY-001-TEST-SOURCE-REVIEW-01.md](../testing/BBGO-PAY-001-TEST-SOURCE-REVIEW-01.md).
Static review found a mechanically broken split reader, incomplete request/status field
signature mutations, a vacuous validation-order proof, weak stable-code assertions,
service replay/terminal checks that can pass without the durable invariant, a vacuous
negative-capability scan, loose leak detection, and fuzz/status boundary gaps.

Those corrections affect signature trust, attacker-controlled framing, durable replay,
and concurrency, so the repository routing policy assigns them to Sol High. Sol may edit
only the five payment test files named in the active handoff. The frozen fixture and
protocol test, all production, execution, integration, evidence, Git, GitHub, wallet,
rate, transaction, public-peer, and other paths remain unauthorized.

BBGO-SEC-001 and BBGO-SEC-002 remain accepted. Their existing reviewed exceptions and
re-review dates are unchanged. `../go-ipfs` is deprecated and receives no wallet work.
