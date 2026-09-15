# BBGO-PAY-002 phase A — test-source review 01

Date: 2026-09-14. Reviewer: Codex.
Decision: REJECTED before execution; bounded Grok correction authorized.

## Reviewed identity and scope

HEAD: `801f5d55d80fe02c6eb512ff35f8c09acfd679af`.
Drop: `modern/cmd/bitbookd/payment_test.go`, 555 lines.
SHA-256: `201c570a9f36d5c5bec0d739a459869079d74be40519db3ee13dd62c1670e80b`.
Two behavior tests and one guarded child helper:

- `TestPaymentDaemonReceivesAndPersistsRequests`
- `TestPaymentDaemonRejectsWrongPayer`
- `TestPaymentDaemonChild`

All nine frozen input hashes match the original
[handoff](../handoff/GROK_BUILD_BBGO_PAY_002_DAEMON_TESTS_01.md).
The existing governance/handoff changes and owner binary are outside the test drop.
This is static source review: no compilation, tests, daemon, scanners, or acceptance
commands were executed. No passing or expected-red result is claimed.

## Blocking findings

1. **P1 — Readiness parser rejects normal daemon logs.** At test lines 279 and 287,
   `strings.CutPrefix` expects bare `peer ID:` and `p2p:` lines. Production main.go
   lines 108–110 use the standard timestamp-prefixed logger; neither the helper nor
   run() removes those flags. Both tests time out before payment negotiation. Parse
   the actual timestamped output while retaining peer-ID and numeric-loopback checks.
2. **P1 — Resend can conceal lost records across restart.** Test lines 54–62 resend
   before the first datastore inspection. If startup discards prior payment records,
   the resend can recreate the record and satisfy every assertion. Prove persistence
   after a restart without resending, using inspection only after normal stop/reaping.
   The original reviewer handoff prescribed this insufficient sequence; this correction
   strengthens that contract rather than attributing the entire gap to the developer.
3. **P2 — Reader cleanup is unbounded after process timeout.** Test lines 235–239 can
   reach an unconditional `readers.Wait()` after kill-and-wait has timed out. Retain
   pipe ownership, close readers on failure, and bound reader completion. Cleanup at
   lines 135–142 also discards errors; report failures so an unreaped process or failed
   resource close cannot be silently treated as successful cleanup.

## Retained strengths and disposition

The child invokes actual run(); the parent uses existing signing/framing APIs. The
wrong-payer case first requires successful delivery and then sends directly to the
remote handler, requiring the stable PAYER rejection. Preserve these properties.

Active correction:
[GROK_BUILD_BBGO_PAY_002_DAEMON_TESTS_CORRECTION_01.md](../handoff/GROK_BUILD_BBGO_PAY_002_DAEMON_TESTS_CORRECTION_01.md).
Only the existing test file may be corrected. Source review remains required before
execution. Hermes owns later integration and acceptance evidence under a separate
handoff; no execution, production wiring, integration, commit, or push is authorized
by this review.
