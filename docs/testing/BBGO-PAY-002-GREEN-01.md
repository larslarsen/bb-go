# BBGO-PAY-002 phase A — integrated green 01 evidence

Date: 2026-09-15. Actor: Hermes, free Nous Portal model (meituan/longcat-2.0:free),
manually relayed by owner. Reviewer: Codex.

## Model

meituan/longcat-2.0:free via Nous Portal (provider: nous).

## Baseline

HEAD: `801f5d55d80fe02c6eb512ff35f8c09acfd679af`.
Accepted main.go: 225 lines, SHA-256
`6333d04275944872f2c1ba533642fe16f06b807e7c8e6af537cecdd504c1269b`.
Frozen test: `modern/cmd/bitbookd/payment_test.go`, 733 lines, SHA-256
`f3e2188ebd2b3706e27903c26f6f8064dc5bf97e84d14b5015b82ef91b5314a2`.
All other eight original frozen inputs matched. No source edits to main.go except
temporary falsification.

## Six-line production change

diff --git a/modern/cmd/bitbookd/main.go b/modern/cmd/bitbookd/main.go
--- a/modern/cmd/bitbookd/main.go
+++ b/modern/cmd/bitbookd/main.go
@@ -17,6 +17,7 @@
     "github.com/larslarsen/bb-go/modern/api"
     "github.com/larslarsen/bb-go/modern/direct"
     "github.com/larslarsen/bb-go/modern/network"
+    "github.com/larslarsen/bb-go/modern/payment"
     "github.com/larslarsen/bb-go/modern/social"
     dht "github.com/libp2p/go-libp2p-kad-dht"
     "github.com/libp2p/go-libp2p/core/peer"
@@ -83,6 +84,11 @@
     }
     defer node.Close()
+    paymentService, err := payment.NewService(node.Node)
+    if err != nil {
+        return err
+    }
+    defer paymentService.Close()

     store, err := social.NewStore(node.Node)
     if err != nil {
         return err

Why payment Close precedes node.Close: defers run in LIFO order, so
`paymentService.Close()` executes before `node.Close()` on any return path —
including later startup-error exits. Payment handlers close before the shared
node/datastore, isolating payment shutdown from the shared social/direct/API
lifecycle.

## Verified toolchain

```
$HOME/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin/go
```

SHA-256: `1db869c560a193573a71be466a34e0d4abb7792d78165c6102cdda069276a3a8`.
Version: `go version go1.27.0 linux/amd64`. Exit 0.

## Filesystem

`findmnt -T modern`: `ext4 /`.
Reuse: `modern/dist/pay002-red-01/cache` (disk-backed, owned).
New: `modern/dist/pay002-green-01/tmp`, `modern/dist/pay002-green-01/logs`,
`modern/dist/pay002-green-01/main.go.backup`.

## Commands and environment

Variable: `BBGO_PAY002_GO="$HOME/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin/go"`.
Environment: `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off
GOCACHE=$PWD/dist/pay002-red-01/cache GOTMPDIR=$PWD/dist/pay002-green-01/tmp
TMPDIR=$PWD/dist/pay002-green-01/tmp`.

### Stage 1 — focused green on accepted source

Command:
```
timeout --signal=TERM --kill-after=10s 600s env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/dist/pay002-red-01/cache" GOTMPDIR="$PWD/dist/pay002-green-01/tmp" TMPDIR="$PWD/dist/pay002-green-01/tmp" "$BBGO_PAY002_GO" test ./cmd/bitbookd -run '^TestPaymentDaemon(ReceivesAndPersistsRequests|RejectsWrongPayer)$' -count=1 -v -timeout=180s
```

Start: 2026-09-15T00:41:58Z. End: 2026-09-15T00:41:59Z. Wall: ~1s.
Shell exit: 0. Result: PASS. Both behavior tests passed:

```
=== RUN   TestPaymentDaemonReceivesAndPersistsRequests
--- PASS: TestPaymentDaemonReceivesAndPersistsRequests (0.10s)
=== RUN   TestPaymentDaemonRejectsWrongPayer
--- PASS: TestPaymentDaemonRejectsWrongPayer (0.04s)
PASS
ok  \tgithub.com/larslarsen/bb-go/modern/cmd/bitbookd\t0.153s
```

Log: `dist/pay002-green-01/logs/stage1-focused-green.log`, SHA-256
`481b01c82c1bc9ea20b360264cf12abb9f582a8209bf7f70e7c324929a42072d`.

### Stage 2 — falsification

Backup: `dist/pay002-green-01/main.go.backup`, SHA-256
`6333d04275944872f2c1ba533642fe16f06b807e7c8e6af537cecdd504c1269b` (accepted hash).
Falsified main.go: 219 lines, SHA-256
`9c7aba19576d162b322dce3dddb61990f8d2a5a65f12b681100bf6e23f8a12ab` (original
baseline — confirmed exact restoration of the original source minus Grok's payment
registration).

Falsified command (same Stage 1 command against mutated source):

Start: 2026-09-15T00:42:50Z. End: 2026-09-15T00:42:51Z. Wall: ~1s.
Shell exit: 1. Result: FAIL. Both tests failed on unsupported protocol:

```
=== RUN   TestPaymentDaemonReceivesAndPersistsRequests
    payment_test.go:50: payment protocol negotiation or delivery failed: failed to negotiate protocol: protocols not supported: [/bitbook/payment/1.0.0]
--- FAIL: TestPaymentDaemonReceivesAndPersistsRequests (0.03s)
=== RUN   TestPaymentDaemonRejectsWrongPayer
    payment_test.go:76: payment protocol negotiation or delivery failed: failed to negotiate protocol: protocols not supported: [/bitbook/payment/1.0.0]
--- FAIL: TestPaymentDaemonRejectsWrongPayer (0.03s)
FAIL
FAIL\tgithub.com/larslarsen/bb-go/modern/cmd/bitbookd\t0.067s
```

No compile/fixture/bind/cleanup error. Same expected-red message as the earlier
expected-red run, proving the test fails only because payment registration is
removed — not from an unrelated defect.

Log: `dist/pay002-green-01/logs/stage2-falsified.log`, SHA-256
`cf4e572c12931a71148d0a0763ef687730f54ab2405679510aac100833dbe55f`.

Restoration: `cp dist/pay002-green-01/main.go.backup cmd/bitbookd/main.go`.
Post-restore verification: 225 lines, SHA-256
`6333d04275944872f2c1ba533642fe16f06b807e7c8e6af537cecdd504c1269b`.

### Stage 3 — restored focused green

Command (exact Stage 1 command):

Start: 2026-09-15T00:43:04Z. End: 2026-09-15T00:43:05Z. Wall: ~1s.
Shell exit: 0. Result: PASS. Both tests green again after restoration.

Log: `dist/pay002-green-01/logs/stage3-restored-green.log`, SHA-256
`70d8c91975223f370ffca544090797e2c22e5009f6a6d762f7a2b402c37dfec4`.

### Stage 4 — five-package race suite

Command:
```
timeout --signal=TERM --kill-after=10s 900s env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/dist/pay002-red-01/cache" GOTMPDIR="$PWD/dist/pay002-green-01/tmp" TMPDIR="$PWD/dist/pay002-green-01/tmp" "$BBGO_PAY002_GO" test -race -json ./cmd/bitbookd ./payment ./api ./direct ./network -count=1 -timeout=300s
```

Start: 2026-09-15T00:43:28Z. End: 2026-09-15T00:43:57Z. Wall: ~29s.
Shell exit: 0. Result: PASS. All five packages passing, no race, panic, timeout,
or cleanup diagnostic.

| Package | Top-level tests | Subtests | Total pass | Package time |
|---|---|---|---|---|
| api | 6 | 0 | 6 | (see JSON) |
| cmd/bitbookd | 5 | 8 | 13 | 5.591s |
| direct | 7 | 4 | 11 | (see JSON) |
| network | 11 | 2 | 13 | (see JSON) |
| payment | 58 | 221 | 279 | (see JSON) |

Log: `dist/pay002-green-01/logs/stage4-race.log`, SHA-256
`016eb884185c6d59eb24dc3fcbd81ded656b088e849012b1f60c56654bd2c6f2`.

No new dependencies, no go.mod/go.sum change, no broader suite or scanner run.

## Verdict

INTEGRATED GREEN CAPTURED — REVIEWER ACCEPTANCE REQUIRED.

No further source, test, or Git work is authorized. Codex must review and accept
this evidence before any commit, push, or publication authority.
