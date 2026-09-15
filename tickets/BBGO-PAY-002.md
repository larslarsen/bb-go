# BBGO-PAY-002 — Connect payment requests to the running daemon

Status: PHASE A ACCEPTED AND PUBLISHED — no further implementation authorized.
Reviewer: Codex. Grok completed the source and focused development test; Hermes completed
integrated green, falsification/restoration, the five-package race suite, and publication.
Feature commit: c00764d4a84eb0e149ec2779d1746248f86ba3a4 on origin/master; CI passed.
Active actor: none.

Review: [BBGO-PAY-002-TEST-SOURCE-REVIEW-02.md](../docs/testing/BBGO-PAY-002-TEST-SOURCE-REVIEW-02.md).
Execution review: [BBGO-PAY-002-EXPECTED-RED-REVIEW-02.md](../docs/testing/BBGO-PAY-002-EXPECTED-RED-REVIEW-02.md).
Production review: [BBGO-PAY-002-PRODUCTION-SOURCE-REVIEW-01.md](../docs/testing/BBGO-PAY-002-PRODUCTION-SOURCE-REVIEW-01.md).
Runtime acceptance: [BBGO-PAY-002-GREEN-REVIEW-01.md](../docs/testing/BBGO-PAY-002-GREEN-REVIEW-01.md).
Final acceptance: [BBGO-PAY-002-FINAL-REVIEW-01.md](../docs/testing/BBGO-PAY-002-FINAL-REVIEW-01.md).
Active handoff: none. [Publication handoff](../docs/handoff/HERMES_BBGO_PAY_002_PUBLISH_01.md) is complete.

Owner requirement: send payment requests and payments between BitBook users. This
ticket advances request delivery, not coin custody or settlement. BBGO-PAY-001 supplied
the signed libp2p transport component but expressly omitted startup/client integration.

## Baselines

Original bb-go baseline: 801f5d55d80fe02c6eb512ff35f8c09acfd679af (accepted PAY-001).
Published phase A: c00764d4a84eb0e149ec2779d1746248f86ba3a4.
bb-desktop read-only baseline: 2b5ad193f01af31f2a2bdcc7b0d1080dd5a2f68e, with existing
dirty wallet work. See its docs/architecture/BBD-PAY-END-TO-END-STATUS-01.md.
The untracked modern/bitbookd binary belongs to the owner; do not run/overwrite/delete it.
No go-ipfs work. No cross-repository source change or commit is authorized here.

## Phase A invariant

A normally started daemon registers /bitbook/payment/1.0.0 using its existing social
identity and datastore, accepts a correctly signed request bound to its peer ID,
retains verified records across a normal stop/restart, and does not persist a request
bound to another payer. Identical resends remain idempotent. Shutdown closes payment
handlers before the shared node/datastore. No new retry worker or payment HTTP surface.

The authorized bounded implementation imports modern/payment in cmd/bitbookd/main.go,
creates payment.NewService(node.Node) immediately after the existing defer node.Close(),
propagates construction errors, and defers payment Close so it runs before node.Close.
Reviewed test source, recovered expected red, production source, and supporting focused
green are accepted. Main.go is frozen at 225 lines, SHA-256
6333d04275944872f2c1ba533642fe16f06b807e7c8e6af537cecdd504c1269b,
with exact restoration confirmed after accepted falsification. All source is now frozen.
The existing codec, signature, replay, datastore and transport implementations are frozen.

## Test and verification contract

The corrected modern/cmd/bitbookd/payment_test.go is frozen by source review 02;
its hash remained unchanged through recovered expected red. No test edits or test
formatting are authorized during wiring. The tests invoke the
actual run() lifecycle in an owned child process, not instantiate a payment service in
place of daemon startup. Use only local peers and owned ephemeral state. No payment
HTTP route, wallet, coin node, public peer or real funds.

Readiness must parse real timestamped daemon logs. Persistence must be inspected after
a normal restart/stop with no intervening payment resend, before testing resend
idempotence. Process and reader cleanup must be bounded and failures reported.

Completed focused expected-red/green selector (expected red, integrated green,
falsified red, and restored green are accepted; this is historical validation,
not authorization to rerun tests):

```text
GOTOOLCHAIN=go1.27.0 GOPROXY=off GOSUMDB=off go test ./cmd/bitbookd -run '^TestPaymentDaemon(ReceivesAndPersistsRequests|RejectsWrongPayer)$' -count=1
```

Workdir: modern. Intended red is failure to negotiate the payment protocol with the
running daemon, not compile errors or a readiness timeout. Later green must prove actual
delivery/digest/persistence and reject the wrong payer. Falsification: executor later
temporarily removes only production payment-service registration, observes the same
transport failure, then restores the exact source identity. Broader later checks:
go test -race ./cmd/bitbookd ./payment ./api ./direct ./network, using the same offline
toolchain settings; record source/go.mod/go.sum unchanged and no newly introduced
authority/dependency. No dependency or supply-chain change is planned in this slice.

Client-facing request creation/retrieval, desktop UI, receiver derivation, wallet approval,
chain submission and paid receipts are subsequent contracts, not authorized here.
In particular, the current social API's wildcard CORS is not approval for payment
request endpoints. Existing network paid-status rejection stays intact. Integration,
commits and pushes under Hermes publication 01 are complete. The reviewer closeout
records the published feature and successful CI. No further source work, executor
task, local retesting, deployment, or broader product acceptance is authorized. Existing
security exceptions remain unchanged; no dependency/parser/cryptographic-core change
occurred in this slice.
