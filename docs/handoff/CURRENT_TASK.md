# Current Task

NO ACTIVE IMPLEMENTATION: BBGO-PAY-002 phase A — ACCEPTED AND PUBLISHED.
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
