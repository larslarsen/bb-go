# BBGO-PAY-002 phase A — corrected test-source review 02

Date: 2026-09-14. Reviewer: Codex.
Decision: ACCEPTED FOR FOCUSED EXPECTED-RED EXECUTION; implementation is not accepted.

## Reviewed identity

HEAD: `801f5d55d80fe02c6eb512ff35f8c09acfd679af`.
Drop: `modern/cmd/bitbookd/payment_test.go`, 733 lines.
SHA-256: `f3e2188ebd2b3706e27903c26f6f8064dc5bf97e84d14b5015b82ef91b5314a2`.
All nine frozen inputs match the original
[inventory](../handoff/GROK_BUILD_BBGO_PAY_002_DAEMON_TESTS_01.md).

The drop contains two daemon behavior tests, `TestParseDaemonIdentity` with eight
table cases, and the guarded `TestPaymentDaemonChild` helper. This is source inventory,
not an executed test count. No compilation, formatting, tests, or acceptance commands
were executed by the reviewer. Runtime behavior and format validation remain pending.

## Findings resolved

- Test lines 443–490 now extract identity values from timestamped log lines without
  altering production logging. Lines 103–181 cover ordinary/microsecond timestamps,
  splash and non-loopback output, missing fields, zero port, and malformed/mismatched
  identities. Existing direct-protocol negotiation still precedes payment delivery.
- Lines 55–58 restart, confirm readiness and the same peer ID, then stop/reap and
  inspect storage without resending. Lines 60–66 separately exercise another restart
  and identical resend. This removes the recreation loophole from review 01 and its
  original reviewer-authored contract.
- Lines 265–408 retain pipe readers, publish a single process-wait result through
  a completion channel, bound process and reader waits, close readers during cleanup,
  and reject forced-kill/unconfirmed shutdown. Cleanup reports payee/reap errors.
  Storage inspection and restart follow successful normal reaping.

The actual run() child, shared daemon identity/datastore boundary, signed public API
fixtures, exact stored-record assertions, wrong-payer positive control, and direct
remote PAYER rejection remain intact. No further blocking source finding was identified.
Expected red must still be observed; this review does not claim persistence, rejection,
shutdown ordering, race safety, or final feature acceptance from static inspection alone.

## Next authority

[Hermes expected-red handoff](../handoff/HERMES_BBGO_PAY_002_EXPECTED_RED_01.md)
authorizes mechanical formatting of this exact drop, one parser check, one focused
daemon expected-red command, and bounded evidence records. Preserve the incoming and
formatted identities. Hermes uses a free Nous Portal model and does not author tests.
Production edits, broader acceptance suites, source falsification, Git mutation, and
public-network activity remain unauthorized. Codex reviews the resulting evidence
before authorizing production wiring.
