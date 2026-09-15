# BBGO-PAY-002 phase A — expected red 01 evidence

Date: 2026-09-14. Actor: Hermes, free Nous Portal model (meituan/longcat-2.0:free),
manually relayed by owner. Reviewer: Codex.

## Model

meituan/longcat-2.0:free via Nous Portal (provider: nous).

## Baseline

HEAD: `801f5d55d80fe02c6eb512ff35f8c09acfd679af`.
Incoming drop: `modern/cmd/bitbookd/payment_test.go`, 733 lines, SHA-256
`f3e2188ebd2b3706e27903c26f6f8064dc5bf97e84d14b5015b82ef91b5314a2`.
All nine frozen inputs matched the original inventory.

## Filesystem

`findmnt -T modern`: `ext4 /`.
Owned runner directories created after filesystem check:
`modern/dist/pay002-red-01/cache`, `modern/dist/pay002-red-01/tmp`,
`modern/dist/pay002-red-01/logs`. Ordinary owned directories, no symlink
components, disk-backed, not RAM-backed. No Git ignore changes.

## Formatting

gofmt produced no diff against the incoming drop. Formatted identity unchanged:
733 lines, SHA-256 `f3e2188ebd2b3706e27903c26f6f8064dc5bf97e84d14b5015b82ef91b5314a2`.
No semantic edits; all nine other input hashes unchanged.

## Commands and environment

Go binary: `/usr/lib/go/bin/go` (go1.27.0, detected by `/usr/bin/go` symlink to
`../lib/go-1.26/bin/go`, so `/usr/lib/go/bin` was prepended to PATH).
Environment: `GOTOOLCHAIN=auto GOPROXY=off GOSUMDB=off
GOCACHE=$PWD/dist/pay002-red-01/cache GOTMPDIR=$PWD/dist/pay002-red-01/tmp
TMPDIR=$PWD/dist/pay002-red-01/tmp`.

### Parser prerequisite

```
/usr/lib/go/bin/go test ./cmd/bitbookd -run '^TestParseDaemonIdentity$' -count=1 -v -timeout=30s
```

Result: PASS. 8/8 subtests passed (timestampedStdFlags, timestampedMicroseconds,
splashThenTimestamped, missingPeerID, missingLoopback, tcpZero, mismatchedPeerID,
malformedPeerID). Duration: 0.008s. No compilation errors.

### Focused daemon expected-red

```
timeout --signal=TERM --kill-after=10s 600s env GOTOOLCHAIN=auto GOPROXY=off GOSUMDB=off GOCACHE="$PWD/dist/pay002-red-01/cache" GOTMPDIR="$PWD/dist/pay002-red-01/tmp" TMPDIR="$PWD/dist/pay002-red-01/tmp" /usr/lib/go/bin/go test ./cmd/bitbookd -run '^TestPaymentDaemon(ReceivesAndPersistsRequests|RejectsWrongPayer)$' -count=1 -v -timeout=180s
```

Result: FAIL. Exit code 1. Both named tests failed at payment protocol negotiation:

```
=== RUN   TestPaymentDaemonReceivesAndPersistsRequests
    payment_test.go:50: payment protocol negotiation or delivery failed: failed to negotiate protocol: protocols not supported: [/bitbook/payment/1.0.0]
--- FAIL: TestPaymentDaemonReceivesAndPersistsRequests (0.03s)
=== RUN   TestPaymentDaemonRejectsWrongPayer
    payment_test.go:76: payment protocol negotiation or delivery failed: failed to negotiate protocol: protocols not supported: [/bitbook/payment/1.0.0]
--- FAIL: TestPaymentDaemonRejectsWrongPayer (0.03s)
FAIL
FAIL    github.com/larslarsen/bb-go/modern/cmd/bitbookd  0.070s
```

No compilation/dependency/fixture error, readiness timeout, loopback-bind failure,
panic, watchdog exit, cleanup error, or unexpected pass. The failure is exactly the
expected red: the unchanged daemon has no `/bitbook/payment/1.0.0` service
registration, so the test's attempt to deliver a payment request fails at protocol
negotiation. Ordinary startup and direct-protocol readiness succeeded; normal child
cleanup occurred. No public peers, downloads, or broader suite execution.

## Verdict

EXPECTED RED CAPTURED — REVIEWER ACCEPTANCE REQUIRED.

No production implementation is authorized. Codex must review and accept this
evidence before any production wiring of payment service integration proceeds.
