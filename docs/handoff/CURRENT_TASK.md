# Current Task

ACTIVE: [BBGO-ACC-002](../../tickets/BBGO-ACC-002.md), review 01. Sol's three
test files are source-accepted for the expected-red compile check (1,897 lines,
11 tests and one fuzz target); all nine frozen inputs match. Owner relays the same
ticket to Hermes on a free Nous Portal model. Run its exact automatic capture
command and retain the designated execution report. No source edits, production,
scans or Git work in this phase. No actor has been launched by Codex and no tests
have been executed by Codex. Reviewer effort stays High; naming is not a gate.

QUEUED NETWORK DIRECTION: [BBGO-NET-001](../../tickets/BBGO-NET-001.md), use the
public IPFS swarm/bootstrap infrastructure without a required BitBook seed fleet.
Owner reiterated this on 2026-09-17. The old bootstrap-only ticket was incomplete:
current DHT/Bitswap protocol prefixes still isolate BitBook. The revised ticket
records the required compatibility work; no network migration has occurred.
ACC-002 proceeds unchanged. Trust-profile bootstrap is unrelated and unchanged.

ACCEPTED AND PUBLISHED: [BBGO-ACC-001](../../tickets/BBGO-ACC-001.md), portable
account grants and revocation verifier, closed by review 08. Feature ad52bf01 and
final report 4bd56126 are verified on remote master. Source hashes match and CI
passed. Reviewer recovered actual successful secret-scan tool results; review 08
documents the manually transcribed captures, incorrect timestamps and small report
edit after its scan. No further executor task, report correction or test rerun is
needed under ACC-001. This isolated package adds no UI behavior yet. ACC-002 now
bounds durable state; network revocation discovery remains subsequent integration.
Codex has not launched an actor or executed
tests/scanners. Preserve unrelated work.
The owner confirms reviewer effort High. Review and phase evidence remain in the same
ticket; no separate execution handoff is needed.

Account architecture: a backed-up controller and independent revocable device keys.
The ticket freezes record bytes, capabilities, known-revocation rules, bounds and
test/acceptance obligations. This is an isolated package with no current daemon/UI
integration. Existing v1 peer-ID/signature bindings remain frozen. Naming is provisional
and is not a gate. Read the
[account decision](../../../bb-desktop/docs/architecture/BB-ACCOUNT-RECIPIENT-PROPOSAL-01.md).
Current reviewer governance publication is limited to this file, ACC-002 and NET-001
in bb-go; review 01 records the scope and source pins. Preserve unrelated work.

BBGO-PAY-003 — ACCEPTED AND PUBLISHED. Its authorizations below are historical and
do not restrict or expand ACC-001's explicit scope. DEV-001 remains cancelled.

Feature commit: `82ed5f9c62ab22687a4972ba0ad59731bf43013e` on origin/master.
CI: [Go 1.27, run 35017544700](https://github.com/larslarsen/bb-go/actions/runs/35017544700) passed.
Hermes closeout: `290825cb40f08f683d72993e8412a703d6a11836`, independently verified
on remote master. Final review: [BBGO-PAY-003-FINAL-REVIEW-01.md](../testing/BBGO-PAY-003-FINAL-REVIEW-01.md).

Owner wants end-to-end peer payment requests and payments; this slice added the
localclient HTTP server with authenticated request delivery, streaming regression
tests, and fuzz coverage.

Publication: [BBGO-PAY-003-PUBLICATION-01.md](../testing/BBGO-PAY-003-PUBLICATION-01.md).

Source, tests, and binary are frozen. No further implementation, executor task,
local testing, or developer Git work is authorized. Further payment product work
needs a new reviewer-bounded contract.
The additional test-file scan findings are adjudicated in the final review; no
source repair or rerun is needed. Codex's only closeout mutation is the exact
three-file reviewer governance/review publication enumerated there. Desktop payment
inbox integration remains a subsequent task; the local daemon has not been restarted.

DEV-001 is cancelled. All DEV-001 handoffs below are historical and cancelled.

HISTORICAL DEV-001 FINISH:

Owner directed reducing the test/process overhead. Hermes signal red is accepted;
all 12 source pins, real binary and before/after/current Git state match. No more
test additions, formal-red phases, broad discovery or separate falsification run.
Relay docs/handoff/GROK_BUILD_BBGO_DEV_001_FINISH_01.md to Grok Build 4.6 High:
fix only helper cancellation and the README test cwd, run the existing fixture
suite, then return for review. Hermes next performs the actual build-only refresh
and scoped publication. No real daemon restart. Earlier handoffs below are historical.

HISTORICAL SIGNAL RED 01:

Signal regressions are source-accepted at 1107 lines, SHA-256
0eabddae499cc425e89f0596e9e237ecc0bf26974e8ddc8659ce2ba93b78f399.
Exactly 18 methods; the prior 16 tests and their fixture source reconstruct to the
accepted old hash. Review: docs/testing/BBGO-DEV-001-SIGNAL-TEST-SOURCE-REVIEW-01.md.
Owner may relay docs/handoff/HERMES_BBGO_DEV_001_SIGNAL_RED_01.md to Hermes on a
free Nous Portal model. It captures the two-test/four-failure signal red and whole
18-test target with the same four failures. All 12 source pins and the real binary
are frozen. No production edits, actual Go/daemon, scans or Git mutation. Grok test
authority is closed. Codex must accept the red before helper/README repair.
Earlier source instructions below are historical. No actor launched.

HISTORICAL SIGNAL TESTS 01:

First five-path production drop requires correction: SIGINT/SIGTERM can be swallowed
after the last interruption check and before publication/exec. README also lost the
modern test-command cwd. See docs/testing/BBGO-DEV-001-PRODUCTION-SOURCE-REVIEW-01.md.
Owner may relay docs/handoff/GROK_BUILD_BBGO_DEV_001_SIGNAL_TESTS_01.md to Grok Build
4.6 High. Only scripts/dev_bitbookd_test.py may change to add the two bounded
real-signal regressions; retain the original 16 tests. All five production paths,
nonwritable original inputs and actual daemon binary are frozen. No Hermes green,
real build/refresh, daemon restart, scans or Git mutation. No actor launched.

HISTORICAL PRODUCTION 01:

Formal red is accepted: 16 methods / 24 assertion failures, zero errors/skips,
exit 1; all failures follow Python exit 2 for the missing copied helper. All nine
input pins, accepted tests and actual daemon binary match. Review:
docs/testing/BBGO-DEV-001-EXPECTED-RED-REVIEW-01.md. The review records the
executor's removed out-of-scope temporary driver; no red rerun is needed.
Owner may relay docs/handoff/GROK_BUILD_BBGO_DEV_001_PRODUCTION_01.md to Grok Build
4.6 High. Exactly five production paths are writable; tests and real binaries stay
frozen. Grok may iterate only the focused fixture suite. No actual Go build, daemon,
broader acceptance, scans or Git mutation. Hermes formal green, verified local
build-only refresh and publication require later reviewer handoffs. No actor launched.

HISTORICAL EXPECTED RED 01:

Corrected test source is accepted: 843 lines, SHA-256
ceafeb66aa78a569a293bff628d09343876df3e74415c7c334d57f3815ecbfbb;
16 test methods. All nine original inputs and the real daemon binary are unchanged;
production helper remains absent. See docs/testing/BBGO-DEV-001-TEST-SOURCE-REVIEW-02.md.
Owner may relay docs/handoff/HERMES_BBGO_DEV_001_EXPECTED_RED_01.md to Hermes on a
free Nous Portal model. Its supplied driver captures the offline fixture red and
before/after pins. No source edits, actual Go/daemon, build, Git mutation or restart.
Grok authority is closed; Codex must accept the red before authorizing production.
Earlier source/correction instructions below are historical. No actor was launched.

HISTORICAL TEST CORRECTION 01:

First 544-line test drop requires correction: failed build is not exercised in run
mode, compiler lookup is not isolated, signal readiness races handler installation,
and process cleanup/CLI-boundary proofs need repair. See
docs/testing/BBGO-DEV-001-TEST-SOURCE-REVIEW-01.md. Owner may relay
docs/handoff/GROK_BUILD_BBGO_DEV_001_TESTS_CORRECTION_01.md to Grok Build 4.6 High.
Only scripts/dev_bitbookd_test.py may change. Nine input pins and the real local
binary remain unchanged; production helper remains absent. No formal Hermes red
or production is authorized. The original create-only handoff below is historical.

WAL-019 is complete in bb-desktop. The next bounded task addresses the owner's
stale local daemon: its current executable still records pre-PAY-002 revision
801f5d55d80fe02c6eb512ff35f8c09acfd679af. Read tickets/BBGO-DEV-001.md and relay
docs/handoff/GROK_BUILD_BBGO_DEV_001_TESTS_CORRECTION_01.md to Grok Build 4.6 High.
Only scripts/dev_bitbookd_test.py may be corrected. Grok may run the ticket's offline
fixture tests, which never invoke real Go or the actual daemon. Production, binary
refresh, formal acceptance and publication each require later reviewer handoffs.
Preserve modern/bitbookd and all unrelated work. No actor has been launched.
Payment authenticated-client access remains the next payment dependency; mobile UI
and profile pictures remain queued. No real funds, mainnet or deployment.

BBGO-PAY-002 phase A remains ACCEPTED AND PUBLISHED. All older active-task/actor
instructions below are historical and cannot authorize execution.

WALLET UI DIRECTION: owner wants mobile ZEC wallet references considered for a likely
future mobile BitBook version. Desktop docs/architecture/BBD-WAL-MOBILE-DESIGN-DIRECTION-01.md
records Zodl/YWallet/MonteZecret references and proposed consistent phone/desktop
flows. No daemon change, mobile framework choice or active-task replacement follows.

LOCAL BUILD FOLLOW-UP: owner expects development updates to refresh the executable
used locally. The documented modern/bitbookd path is correct, but its inspected Go
build metadata is still revision 801f5d55d80fe02c6eb512ff35f8c09acfd679af, before
PAY-002. Git updates and Go tests do not replace that executable. No maintained
build-before-launch task was found; the root Makefile's bitbookd target builds the
legacy daemon. Queue a bounded modern development build/launch task and an explicit
verified local-binary refresh step in the integration workflow. Build failure must
stop launch rather than silently run a stale binary; preserve launch arguments and
data directories. This is a workflow gap, not a wrong executable location. No local
binary replacement or process restart has happened. BBGO-DEV-001 now bounds this follow-up.

HISTORICAL WAL-019 ROUTING: payments and usable UI. The then-active task was in
bb-desktop: tickets/BBD-WAL-019.md, one-click native wallet Sync publication 01,
via docs/handoff/HERMES_BBD_WAL_019_PUBLICATION_01.md in that repository.
Runtime and the local wallet resource refresh are accepted; see its
docs/testing/BBD-WAL-019-GREEN-REVIEW-01.md. Hermes may correct its reports and
scan/commit/push the exact desktop path set. All source is frozen. No bb-go mutation,
daemon rebuild, process restart or test execution is authorized here.
Remaining payment sequence is recorded in bb-desktop's
docs/architecture/BBD-PAY-END-TO-END-STATUS-01.md: authenticated request access,
native receive/request/approval integration, then submission and confirmation.
Social profile pictures get a separate recovery ticket. Advertising remains
deferred. No real-fund transaction, mainnet, or deployment is authorized.

Owner planning context: separate frontend feature recovery (profile pictures first
among named gaps), Rust ZEC wallet UX/sync redesign, peer-payment completion, and a
private advertising service with on-demand receiving addresses and server-held ad
budgets. Local cross-project planning note:
[PRODUCT_WORKSTREAM_CONTEXT.md](../../../PRODUCT_WORKSTREAM_CONTEXT.md).
Advertising design is deferred. For wallet/payment planning, treat the future service
as a specialized node transacting with users' wallets similarly to tips and tip
requests. Preserve authenticated counterparties and explicit wallet approval; do not
make ad-ledger, auction, or search design prerequisites for payment completion.
The overview itself authorizes no source task; WAL-019 has its own bounded desktop
contract above. Keep private service internals out of public
publication scope. OB1 search reuse remains a deferred compatibility candidate.

Owner wants end-to-end peer payment requests and payments; this slice
connects the accepted payment service to the real daemon lifecycle.

Source review: [BBGO-PAY-002-TEST-SOURCE-REVIEW-02.md](../testing/BBGO-PAY-002-TEST-SOURCE-REVIEW-02.md).
Execution review: [BBGO-PAY-002-EXPECTED-RED-REVIEW-02.md](../testing/BBGO-PAY-002-EXPECTED-RED-REVIEW-02.md).
Production review: [BBGO-PAY-002-PRODUCTION-SOURCE-REVIEW-01.md](../testing/BBGO-PAY-002-PRODUCTION-SOURCE-REVIEW-01.md).
Active actor: none.
Runtime acceptance: [BBGO-PAY-002-GREEN-REVIEW-01.md](../testing/BBGO-PAY-002-GREEN-REVIEW-01.md).
Final acceptance: [BBGO-PAY-002-FINAL-REVIEW-01.md](../testing/BBGO-PAY-002-FINAL-REVIEW-01.md).
Active handoff: none. Hermes publication 01 is complete.
Published feature commit: c00764d4a84eb0e149ec2779d1746248f86ba3a4 on origin/master.
CI: [Go 1.27, run 34916979825](https://github.com/larslarsen/bb-go/actions/runs/34916979825) passed on that exact commit.
Original source baseline: 801f5d55d80fe02c6eb512ff35f8c09acfd679af.
Accepted test: modern/cmd/bitbookd/payment_test.go, 733 lines, SHA-256
f3e2188ebd2b3706e27903c26f6f8064dc5bf97e84d14b5015b82ef91b5314a2.
Accepted main.go: 225 lines, SHA-256
6333d04275944872f2c1ba533642fe16f06b807e7c8e6af537cecdd504c1269b.
Its six inserted lines import/start payment service with error propagation and close
it before the shared node/datastore. Grok's focused test command passed both behavior
tests, exit 0, 1.313s wall time, 0.150s package duration. The test and remaining eight
original inputs stay frozen.

Hermes verified the exact integrated drop, ran focused green, temporarily removed
registration and proved the same protocol-negotiation red, restored the exact accepted
source and ran focused green again, then ran the five-package race suite:

Stage 1 focused green: PASS exit 0, both TestPaymentDaemon tests, package 0.153s wall ~1s.
Stage 2 falsification: main.go restored to original 219-line baseline
(9c7aba19576d162b322dce3dddb61990f8d2a5a65f12b681100bf6e23f8a12ab). Both daemon
tests failed on protocols not supported: [/bitbook/payment/1.0.0] — same expected-red,
proving the test binds to the production registration. Source then restored to accepted
225-line hash.
Stage 3 restored green: PASS exit 0, both tests green again, package 0.154s.
Stage 4 race suite (cmd/bitbookd, payment, api, direct, network): all five packages
PASS, exit 0, ~29s wall. Corrected counts excluding package-level pass events:
api 5, cmd/bitbookd 12 (4 top-level + 8 sub), direct 10 (6+4), network 12 (10+2),
payment 278 (57+221). Totals: 82 top-level and 235 subtest entries. The daemon helper
and six payment fuzz seed entrypoints are included. No native fuzz campaign is claimed.
No race, panic, timeout, or cleanup diagnostic appears in the retained output.

Evidence: [BBGO-PAY-002-GREEN-01.md](../testing/BBGO-PAY-002-GREEN-01.md).

Codex accepts phase A runtime, publication, and CI. The feature commit contains the
exact 23 authorized paths; the pinned staged-content secret scan reports no leaks.
The remote master ref and successful CI head were independently verified. No further
implementation, executor task, local testing, or developer Git work is authorized.
The reviewer publishes only the four-path final governance/review closeout named in
the final review. Further work requires a new reviewer-bounded contract.

Preserve the owner-owned untracked modern/bitbookd and all ignored runner artifacts.
No release binary, deployment, public-peer, wallet/coin, payment HTTP, desktop, or
cross-repository work is authorized. Further payment product work needs a new contract.

The prior BBGO-PAY-001 records below are historical acceptance, not active routing.

Ticket: BBGO-PAY-001

State: ACCEPTED

Reviewer: Lead Engineer/Reviewer — Codex at XHigh

Source actor: Principal Dev — Codex Sol (`gpt-5.6-sol`, High)

Production source baseline: `0560b6426b9af29a16a151dacc7c2f3021a3dc0d`

Integrated frozen-test baseline: `403df23a63f413c11e13085719fc7e767c2f15be`

Active handoff: none

Consolidated evidence: [BBGO-PAY-001-GREEN-01.md](../testing/BBGO-PAY-001-GREEN-01.md)

Accepted feature commit: `6bbb0629cb0dfaca9958f9cac7d57216760630ae`

Final review: [BBGO-PAY-001-FINAL-REVIEW-01.md](../testing/BBGO-PAY-001-FINAL-REVIEW-01.md)

No further work is authorized under this ticket.

Evidence: [BBGO-PAY-001-EXPECTED-RED.md](../testing/BBGO-PAY-001-EXPECTED-RED.md)

Gap evidence: [BBGO-PAY-001-GAP-EXPECTED-RED-03.md](../testing/BBGO-PAY-001-GAP-EXPECTED-RED-03.md)

[BBGO-PAY-001](../../tickets/BBGO-PAY-001.md) remains the only authorized task in this
repository. Grok Build authored the original seven-path test drop in the foreground;
Codex XHigh rejected five files before execution; Codex Sol corrected only those five
files. The final 49 ordinary tests and six fuzz entrypoints are accepted in
[BBGO-PAY-001-TEST-SOURCE-REVIEW-02.md](../testing/BBGO-PAY-001-TEST-SOURCE-REVIEW-02.md).
The desktop oracle remains byte-identical, the payment protocol test remains frozen, and
the full drop is format-clean.

The two authorized expected-red commands produced only missing-production API
diagnostics and are accepted in
[BBGO-PAY-001-EXPECTED-RED-REVIEW.md](../testing/BBGO-PAY-001-EXPECTED-RED-REVIEW.md).
Sol's seven-path production drop was rejected before execution in
[BBGO-PAY-001-PRODUCTION-SOURCE-REVIEW-01.md](../testing/BBGO-PAY-001-PRODUCTION-SOURCE-REVIEW-01.md).
Sol's three gap tests are statically accepted in
[BBGO-PAY-001-GAP-TEST-SOURCE-REVIEW-01.md](../testing/BBGO-PAY-001-GAP-TEST-SOURCE-REVIEW-01.md),
and the seven uncommitted production paths remain byte-identical to their rejected review
inventory. Luna's first focused run was rejected before test execution in
[BBGO-PAY-001-GAP-EXPECTED-RED-ATTEMPT-01.md](../testing/BBGO-PAY-001-GAP-EXPECTED-RED-ATTEMPT-01.md)
because two frozen transport assertions bound two results from the four-result
`DecodePaymentStatus` API. Sol's exact two-line correction is accepted in
[BBGO-PAY-001-TEST-COMPILE-CORRECTION-REVIEW-01.md](../testing/BBGO-PAY-001-TEST-COMPILE-CORRECTION-REVIEW-01.md).
Luna's second attempt reached the two offline assertions but the restricted sandbox
denied the two-node test's ephemeral loopback bind, as recorded in
[BBGO-PAY-001-GAP-EXPECTED-RED-ATTEMPT-02.md](../testing/BBGO-PAY-001-GAP-EXPECTED-RED-ATTEMPT-02.md).
The final focused command ran with a loopback-only sandbox override and produced the
exact three intended missing-protection failures. Codex XHigh accepts that result in
[BBGO-PAY-001-GAP-EXPECTED-RED-REVIEW-03.md](../testing/BBGO-PAY-001-GAP-EXPECTED-RED-REVIEW-03.md).
Sol's typed UTF-8, 32-container parse bound, linked nonce separation, and linear
stored-record correction is accepted as a complete seven-path production drop in
[BBGO-PAY-001-PRODUCTION-SOURCE-REVIEW-02.md](../testing/BBGO-PAY-001-PRODUCTION-SOURCE-REVIEW-02.md).
Tidy, focused/full tests, three falsifications, vet, race, and fuzz target 1 completed
successfully before Luna's agent turn stuck at fuzz target 2. The recovered state and
non-duplicative boundary are recorded in
[BBGO-PAY-001-GREEN-RECOVERY-01.md](../testing/BBGO-PAY-001-GREEN-RECOVERY-01.md).
The exact native target-2 invocation then silently stalled a second time for about 626
seconds before reviewer interruption, as recorded in
[BBGO-PAY-001-FUZZ-HANG-01.md](../testing/BBGO-PAY-001-FUZZ-HANG-01.md). No result or
process remains and state is unchanged. Sol's static audit found no proven source defect;
Codex XHigh accepts its bounded diagnostic design in
[BBGO-PAY-001-FUZZ-HANG-AUDIT-REVIEW-01.md](../testing/BBGO-PAY-001-FUZZ-HANG-AUDIT-REVIEW-01.md).
A fresh Luna completed all three bounded stages; the native single-worker campaign passed
4,967 executions in three seconds. Codex XHigh accepts target 2 and classifies the prior
stalls as earlier executor/transport behavior in
[BBGO-PAY-001-FUZZ-DIAGNOSTIC-REVIEW-01.md](../testing/BBGO-PAY-001-FUZZ-DIAGNOSTIC-REVIEW-01.md).
Luna completed targets 3–6 under the same watchdog pattern, with every target passing a
nonzero native fuzz campaign. Codex XHigh accepts all six targets in
[BBGO-PAY-001-FUZZ-COMPLETION-REVIEW-01.md](../testing/BBGO-PAY-001-FUZZ-COMPLETION-REVIEW-01.md).
Security phase A policy suites and Actionlint passed. Two agent-channel scanner attempts
produced no verdict and are preserved in
[BBGO-PAY-001-SECURITY-A-RECOVERY-01.md](../testing/BBGO-PAY-001-SECURITY-A-RECOVERY-01.md)
and the final review. The visible reviewer-channel official-database probe returned HTTP
200; bounded source Govulncheck accepted only reviewed `GO-2024-3218` plus two
non-reachable notes; and Gosec reported zero issues across 17 files / 4,579 lines. Codex
XHigh accepts the complete phase in
[BBGO-PAY-001-SECURITY-A-REVIEW-01.md](../testing/BBGO-PAY-001-SECURITY-A-REVIEW-01.md).
Luna's fail-closed validator accepted the exact reviewed 25-entry redacted baseline, and
the pinned fully redacted history scan passed 3,406 commits / approximately 313.91 MB
with zero new leaks. Codex XHigh accepts phase B in
[BBGO-PAY-001-SECURITY-B-REVIEW-01.md](../testing/BBGO-PAY-001-SECURITY-B-REVIEW-01.md).
Every execution and security gate is complete. Feature commit
`6bbb0629cb0dfaca9958f9cac7d57216760630ae` and its remote Go 1.27 workflow are green;
Codex XHigh accepts the ticket in
[BBGO-PAY-001-FINAL-REVIEW-01.md](../testing/BBGO-PAY-001-FINAL-REVIEW-01.md). No further
engineering, public network, wallet, rate, transaction, hardware, device, release binary,
SBOM, or unrelated work is authorized under BBGO-PAY-001.

BBGO-SEC-001 and BBGO-SEC-002 remain accepted. Their existing reviewed exceptions and
re-review dates are unchanged. `../go-ipfs` is deprecated and receives no wallet work.
