# BBGO-PAY-002 — expected-red recovery 01 evidence

Date: 2026-09-14. Actor: Hermes, free Nous Portal model (meituan/longcat-2.0:free),
manually relayed by owner. Reviewer: Codex.

## Model

meituan/longcat-2.0:free via Nous Portal (provider: nous).

## Baseline

HEAD: `801f5d55d80fe02c6eb512ff35f8c09acfd679af`.
Drop: `modern/cmd/bitbookd/payment_test.go`, 733 lines, SHA-256
`f3e2188ebd2b3706e27903c26f6f8064dc5bf97e84d14b5015b82ef91b5314a2`.
All nine frozen inputs matched the original inventory. No source edits.

## Earlier toolchain failure acknowledged

The first execution attempt (HERMES_BBGO_PAY_002_EXPECTED_RED_01.md) used
`/usr/lib/go/bin/go` with `GOTOOLCHAIN=auto` after the prescribed
`GOTOOLCHAIN=go1.27.0` failed with `checksum database disabled by GOSUMDB=off`.
The retained parser log from that attempt contains only the toolchain-download
failure; the passing parser output was not captured. The daemon log from that
attempt did show both exact missing-payment-protocol failures. That evidence was
rejected in
[BBGO-PAY-002-EXPECTED-RED-REVIEW-01.md](../testing/BBGO-PAY-002-EXPECTED-RED-REVIEW-01.md)
because the toolchain identity was unestablished and the parser prerequisite log
was missing. This recovery uses the exact cached toolchain the reviewer
identified.

## Verified toolchain

```
$HOME/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin/go
```

SHA-256: `1db869c560a193573a71be466a34e0d4abb7792d78165c6102cdda069276a3a8`.
Version: `go version go1.27.0 linux/amd64`. Exit 0.

## Filesystem

`findmnt -T modern`: `ext4 /`.
Reuse: `modern/dist/pay002-red-01/cache` (existing, disk-backed, owned).
New: `modern/dist/pay002-red-recovery-01/tmp`, `modern/dist/pay002-red-recovery-01/logs`.
Ordinary owned directories, no symlink components. Old logs preserved.

## Commands and environment

Variable: `BBGO_PAY002_GO="$HOME/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin/go"`.
Environment: `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off
GOCACHE=$PWD/dist/pay002-red-01/cache GOTMPDIR=$PWD/dist/pay002-red-recovery-01/tmp
TMPDIR=$PWD/dist/pay002-red-recovery-01/tmp`.

### Parser prerequisite

Command:
```
timeout --signal=TERM --kill-after=10s 600s env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/dist/pay002-red-01/cache" GOTMPDIR="$PWD/dist/pay002-red-recovery-01/tmp" TMPDIR="$PWD/dist/pay002-red-recovery-01/tmp" "$BBGO_PAY002_GO" test ./cmd/bitbookd -run '^TestParseDaemonIdentity$' -count=1 -v -timeout=30s
```

Start: 2026-09-14T23:35:49Z. End: 2026-09-14T23:36:07Z. Elapsed: ~18s.
Shell exit code: 0. Test result: PASS, 8/8 subtests
(timestampedStdFlags, timestampedMicroseconds, splashThenTimestamped,
missingPeerID, missingLoopback, tcpZero, mismatchedPeerID, malformedPeerID).
Package duration: 0.008s. Log SHA-256:
`92c4fb1954a4f2e4089a726a53ef91b531fd220f1719fee107ba10c8a910a2ea`.

### Focused daemon expected-red

Command:
```
timeout --signal=TERM --kill-after=10s 600s env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/dist/pay002-red-01/cache" GOTMPDIR="$PWD/dist/pay002-red-recovery-01/tmp" TMPDIR="$PWD/dist/pay002-red-recovery-01/tmp" "$BBGO_PAY002_GO" test ./cmd/bitbookd -run '^TestPaymentDaemon(ReceivesAndPersistsRequests|RejectsWrongPayer)$' -count=1 -v -timeout=180s
```

Start: 2026-09-14T23:36:18Z. End: 2026-09-14T23:36:19Z. Elapsed: ~1s.
Shell exit code: 1. Test result: FAIL. Raw output:

```
=== RUN   TestPaymentDaemonReceivesAndPersistsRequests
    payment_test.go:50: payment protocol negotiation or delivery failed: failed to negotiate protocol: protocols not supported: [/bitbook/payment/1.0.0]
--- FAIL: TestPaymentDaemonReceivesAndPersistsRequests (0.03s)
=== RUN   TestPaymentDaemonRejectsWrongPayer
    payment_test.go:76: payment protocol negotiation or delivery failed: failed to negotiate protocol: protocols not supported: [/bitbook/payment/1.0.0]
--- FAIL: TestPaymentDaemonRejectsWrongPayer (0.03s)
FAIL
FAIL	github.com/larslarsen/bb-go/modern/cmd/bitbookd	0.073s
FAIL
```

Log SHA-256: `798d2a1ce5c0c43e52bef1d6cc4bcda09701bbb5ad58d3590d529d4631a8f8e0`.

No compilation/dependency/fixture error, readiness timeout, loopback-bind failure,
panic, watchdog exit, cleanup error, or unexpected pass. Both named tests failed
only at payment protocol negotiation with the exact expected message. Ordinary
startup and direct-protocol readiness succeeded; normal child cleanup occurred.
No public peers, downloads, or broader suite execution.

## Verdict

EXPECTED RED RECOVERED — REVIEWER ACCEPTANCE REQUIRED.

No production implementation is authorized. Codex must review and accept this
recovered evidence before any production wiring of payment service integration
proceeds.
