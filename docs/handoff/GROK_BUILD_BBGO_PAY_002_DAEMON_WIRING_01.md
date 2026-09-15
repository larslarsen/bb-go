# BBGO-PAY-002 phase A — Grok daemon wiring 01

Actor: Sr Dev — Grok Build 4.6 High, manually relayed by owner.
Reviewer: Codex. Status: COMPLETE — source and supporting focused green accepted in
[production review 01](../testing/BBGO-PAY-002-PRODUCTION-SOURCE-REVIEW-01.md).
Next authority: [Hermes green 01](HERMES_BBGO_PAY_002_GREEN_01.md).
The source/test authority below is historical; no further Grok work is authorized here.

Read AGENTS.md, TESTING.md, CURRENT_TASK.md, tickets/BBGO-PAY-002.md,
[source review 02](../testing/BBGO-PAY-002-TEST-SOURCE-REVIEW-02.md), and
[accepted expected-red review 02](../testing/BBGO-PAY-002-EXPECTED-RED-REVIEW-02.md).
Test source and the intended red are accepted. This handoff explicitly authorizes
Sr Dev focused test execution; it is not a source-only phase.

## Exact baseline and paths

HEAD: `801f5d55d80fe02c6eb512ff35f8c09acfd679af`.
Only production edit: `modern/cmd/bitbookd/main.go`, incoming 219 lines, SHA-256
`9c7aba19576d162b322dce3dddb61990f8d2a5a65f12b681100bf6e23f8a12ab`.
Frozen test: `modern/cmd/bitbookd/payment_test.go`, 733 lines, SHA-256
`f3e2188ebd2b3706e27903c26f6f8064dc5bf97e84d14b5015b82ef91b5314a2`.
The other eight inputs in the original
[inventory](GROK_BUILD_BBGO_PAY_002_DAEMON_TESTS_01.md) stay frozen.
Verify these and preserve the dirty governance files, all evidence, and the owner's
untracked `modern/bitbookd`; do not execute, overwrite, or delete that binary.
Stop on baseline differences or unexpected source changes.

## Fixed implementation contract

Import `github.com/larslarsen/bb-go/modern/payment` in main.go. Immediately after the
successful network.Open and its existing `defer node.Close()`, before social.NewStore,
construct `paymentService, err := payment.NewService(node.Node)`. Return the error if
construction fails; after success register `defer paymentService.Close()`.

This is the complete production change. The service uses the existing daemon identity
and datastore and registers before clients are served. Go's reverse defer order closes
payment handlers before the shared node/datastore, including later startup-error exits.
Keep existing direct-service and HTTP lifecycles intact. Do not add a goroutine, retry
worker, HTTP endpoint, service option, clock override, readiness/logging change, test
seam, new dependency, or change to payment/transport/storage/security semantics.

No new test source is needed or authorized: the accepted tests already fail because
this registration is absent. Do not weaken, skip, filter out, or edit them to get green.
Format only main.go with gofmt; use the verified cached toolchain's sibling gofmt.

## Authorized focused test

Sr Dev may run the command below once after the bounded source edit and formatting,
then report the result. This is development feedback; Hermes will own integrated
acceptance evidence later. Read-only Git status/diff, file/hash/line checks, tool
version lookup, filesystem inspection, and owned-process inspection are authorized.

From `modern`, set:

```text
BBGO_PAY002_GO="$HOME/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin/go"
sha256sum "$BBGO_PAY002_GO"
timeout --signal=TERM --kill-after=5s 15s env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off "$BBGO_PAY002_GO" version
```

Require executable hash
`1db869c560a193573a71be466a34e0d4abb7792d78165c6102cdda069276a3a8`
and `go version go1.27.0 linux/amd64`, exit 0. Missing/mismatched toolchain means stop;
do not use automatic selection, substitute another binary, or download tools.

Recheck disk backing and ordinary owned/non-symlink directory components before
creating `modern/dist/pay002-grok-focused-01/tmp` and `logs`. Reuse the existing
`modern/dist/pay002-red-01/cache`. These runner artifacts/cache are the only writable
paths besides main.go. Do not mutate repository records or prior evidence/logs.

```text
timeout --signal=TERM --kill-after=10s 600s env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/dist/pay002-red-01/cache" GOTMPDIR="$PWD/dist/pay002-grok-focused-01/tmp" TMPDIR="$PWD/dist/pay002-grok-focused-01/tmp" "$BBGO_PAY002_GO" test ./cmd/bitbookd -run '^TestPaymentDaemon(ReceivesAndPersistsRequests|RejectsWrongPayer)$' -count=1 -v -timeout=180s
```

Capture complete stdout/stderr, shell exit status, start/end time, and whole-command
duration in the owned runner logs. Require exit 0 and both behavior tests passing,
without cleanup errors or timeouts. If anything fails, preserve the exact source and
output and return for review; do not edit outside the fixed contract, retry, run
additional tests, or change environment/toolchain to chase green.

## Return and stop

Return the main.go diff, incoming/final SHA-256 and line counts, frozen-input checks,
exact focused command/result, raw log path/hash, toolchain identity, and any limitation.
Explain why payment Close precedes node.Close. No test/evidence/control-plane edits,
Git mutations, integration, commits, or pushes. No broader suites, race, fuzz,
falsification, scanners, wallet/coin work, public peers, or cross-repository changes.
Codex reviews the source and focused result, then issues Hermes's integration and
acceptance handoff. Passing this focused command alone is not ticket acceptance.
