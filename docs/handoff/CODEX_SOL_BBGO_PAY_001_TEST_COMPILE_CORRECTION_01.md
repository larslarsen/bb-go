# Codex Sol Handoff — BBGO-PAY-001 Test Compile Correction 01

You are **Principal Dev — Codex Sol** using `gpt-5.6-sol` at High. This is the complete
durable prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Governance baseline: the commit containing this handoff

Read completely before acting: `AGENTS.md`, `TESTING.md`,
`docs/engineering/DEVELOPMENT_ROLES.md`, `tickets/BBGO-PAY-001.md`,
`docs/handoff/CURRENT_TASK.md`,
`docs/testing/BBGO-PAY-001-GAP-EXPECTED-RED-ATTEMPT-01.md`, and
`docs/testing/BBGO-PAY-001-GAP-TEST-SOURCE-REVIEW-01.md`.

## Exact edit authorization

Edit only `modern/payment/transport_test.go` with `apply_patch`. In
`TestNetworkStatusIsCancellationOnly`, change exactly these two bindings:

```text
if _, err := DecodePaymentStatus(mustJSON(t, paid)); err != nil {
```

to:

```text
if _, _, _, err := DecodePaymentStatus(mustJSON(t, paid)); err != nil {
```

and:

```text
if _, err := DecodePaymentStatus(mustJSON(t, expired)); err != nil {
```

to:

```text
if _, _, _, err := DecodePaymentStatus(mustJSON(t, expired)); err != nil {
```

Make no other textual, semantic, formatting, or path change. This correction only aligns
the already-accepted assertions with the four-result public API; it does not change what
they test.

Do not run commands, tests, formatters, Git, GitHub, builds, scanners, or network tools.
Do not edit production, another test, fixture, module/lock input, document, workflow,
policy, wallet, rate, transaction, hardware, device, release-binary, or SBOM path. Do not
use root, `sudo`, deletion, cleanup, `rm`, `/tmp`, globs, unresolved targets, public peers,
external services, or background work. Report the one edited path and both exact changes
for Codex XHigh review.
