# BBGO-PAY-001 — Signed Payer-Bound Payment Objects and Transport

Status: SECURITY COMPLETE — LUNA EVIDENCE/INTEGRATION AUTHORIZED

Reviewer: Lead Engineer/Reviewer — Codex at XHigh

Test source actor: Sr Dev — Grok Build (Grok 4.6 High)

Integration actor: Jr Dev — Codex Luna (`gpt-5.6-luna`)

Daemon baseline: `0560b6426b9af29a16a151dacc7c2f3021a3dc0d`

Cross-repository oracle baseline: `../bb-desktop` commit
`d472785ab896bb5d1367c4117ffd659a9a8512ae`

Oracle source: `../bb-desktop/test/fixtures/wallet-contract/golden-v1.json`, 231 lines,
SHA-256 `08905d2b082fa6370211ebd494130d62e3696db0f8314636f3685ba5887f929f`

## Objective

Add a wallet-free, rate-free daemon service that validates and social-signs exact
`PaymentRequestV1` and `PaymentStatusEventV1` bytes, transports payer-bound objects over
an authenticated libp2p stream, and durably retains verified objects for a later client
API ticket. The Go implementation must independently match the accepted Node golden
canonical bytes and hashes.

The daemon signs with its existing libp2p social identity. It never derives a receiver,
possesses a coin key, computes a rate, constructs a transaction, calls a wallet, or
broadcasts. This ticket adds no payment HTTP route and no renderer integration.

## Fixed object and trust contract

- The schemas, validation order, timestamp/calendar range, Unicode policy, ASCII
  restrictions, amount rules, asset/network/receiver-kind relations, JCS bytes, domain
  separators, and SHA-256 digests are exactly BBD-WAL-001 §11 and the accepted fixture.
- Producers sign `domain_separator || JCS(object)` directly with the existing libp2p
  private key. They do not sign a hex digest, ordinary `encoding/json` output, a transport
  envelope, or CBOR.
- A signed wire object is a closed ordinary-JSON envelope with exactly: version `1`, kind
  `request` or `status`, the exact canonical JSON bytes encoded as a JSON string,
  marshalled libp2p public-key bytes, and the signature. Binary members use Go's standard
  base64 JSON representation. The signed bytes are recovered from the canonical string;
  any noncanonical copy is rejected before signature verification.
- The public key must derive the live remote peer ID. For a request it must also equal
  `payee_peer_id`; the authenticated recipient must equal `payer_peer_id`. The local
  producer may sign only when `payee_peer_id` is its own peer ID.
- Status objects are accepted only when the receiver already has the referenced verified
  request and the signer equals that request's payee. v1 transport exposes only payee
  cancellation (`status=cancelled`, empty `tx_ref`). The codec still validates all three
  architecture-defined status shapes, but network `paid` remains blocked pending the
  owner decision on social receipts and network `expired` remains local policy.
- Protocol ID is exactly `/bitbook/payment/1.0.0`. Transport uses one four-byte
  big-endian length-prefixed envelope per stream with a 64-KiB maximum. Unknown fields,
  zero/oversize/truncated frames, malformed UTF-8/JSON/base64, trailing bytes, unsupported
  version/kind, and diagnostics mixed into the stream fail closed.
- A receiver persists only after canonical, signature, remote/payee, payer, expiry, and
  linkage validation. Identical re-delivery is idempotent. Reuse of `request_id`,
  request nonce, `event_id`, or event nonce with different signed bytes is rejected. The
  first accepted terminal cancellation blocks a conflicting later status.
- The service does not automatically retry. A caller may resend the identical signed
  object and receive an idempotent acknowledgement. No background goroutine, public peer,
  wall clock, HTTP service, or mutable external dependency is required by tests.
- Stored data contains the signed public object, public key, signature, digest, direction,
  and receipt time only. No wallet, quote, account, intent, seed, spend key, viewing key,
  raw transaction, provider, fiat value, or RPC field exists.

## Test-source scope

The test-source phase was restricted to these seven paths:

- `modern/payment/testdata/golden-v1.json`
- `modern/payment/canonical_test.go`
- `modern/payment/signature_test.go`
- `modern/payment/transport_test.go`
- `modern/payment/service_test.go`
- `modern/payment/fuzz_test.go`
- `modern/network/protocols_test.go`

The copied fixture must be byte-identical to the named desktop oracle. Tests must not
read the Node implementation in `../bb-desktop/wallet-contract/`; the Go path is an
independent implementation.

During test-source authorship, no production source, API, command wiring, module/lock
input, documentation, workflow, policy implementation, evidence, Git, GitHub, network
command, public peer, wallet, node process, rate provider, transaction, hardware, or
other path was authorized.

## Reviewer test-source correction 01

Grok authored only the seven authorized paths and copied the fixture exactly, but the
drop is rejected before execution in
`docs/testing/BBGO-PAY-001-TEST-SOURCE-REVIEW-01.md`. The fixture and protocol-ID test
are frozen. Blocking findings in the other five files affect signature mutation and
validation order, framing, real durable replay, receipt-clock injection, leak detection,
fuzz non-vacuity, and the negative wallet/rate capability boundary.

Under `docs/handoff/CODEX_SOL_BBGO_PAY_001_TESTS_CORRECTION_01.md`, Principal Dev —
Codex Sol at High may edit only the five named payment test files. No execution,
integration, production, module/lock, evidence, Git, GitHub, wallet, rate, public peer,
or other work is authorized.

## Reviewer test-source acceptance 02

Codex Sol corrected exactly the five authorized payment test files. Codex XHigh accepted
the resulting 49 ordinary tests and six fuzz entrypoints in
`docs/testing/BBGO-PAY-001-TEST-SOURCE-REVIEW-02.md`. The accepted source closes every
review-01 blocker: every-field signature mutation and validation order, exact frame
codes and consumption, deterministic identity and clock injection, real two-node durable
replay/terminal invariants, strict negative-capability inspection, repeated-cycle leak
detection, all-network/status boundaries, and parameterized request/status replay fuzzing.

Jr Dev — Codex Luna may execute only the two expected-red commands and integrate the
frozen seven-path test drop under
`docs/handoff/CODEX_LUNA_BBGO_PAY_001_EXPECTED_RED.md`. Production source, module/lock
input, broad validation, scanners, public network, wallet, rate, transaction, hardware,
device, release binary, and SBOM work remain unauthorized pending reviewer acceptance of
the expected-red evidence.

## Expected-red acceptance and production authorization

Luna integrated the seven frozen test paths at
`403df23a63f413c11e13085719fc7e767c2f15be`. Both authorized commands failed solely on
the absent reserved production API, and Codex XHigh accepted that evidence in
`docs/testing/BBGO-PAY-001-EXPECTED-RED-REVIEW.md`.

Principal Dev — Codex Sol at High may now author exactly the seven production paths in
`docs/handoff/CODEX_SOL_BBGO_PAY_001_PRODUCTION_01.md`. That handoff freezes the public
Go surface, closed-domain RFC 8785 behavior, signature/identity validation order,
one-frame half-close transport, single-key atomic persistence, replay-before-terminal
ordering, cancellation-only network policy, handler shutdown, and negative wallet/rate
boundary. No execution, test edit, fixture edit, module/lock input, integration, Git,
public peer, wallet, rate, transaction, hardware, device, release binary, or SBOM is
authorized until reviewer source acceptance.

## Planned green and security gates (not yet executable)

After Codex XHigh accepts the production source, a separate Luna handoff will authorize
serial foreground execution of `go mod tidy`, the two targeted green commands, full
`go vet ./...`, `go test -race ./... -count=1`, and one bounded run of each of the six
native payment fuzz entrypoints. It will also prescribe exact reversible falsifications
for domain separation, remote-peer binding, and durable replay conflict detection.

The same handoff will run the current pinned BBGO-SEC-001 source ratchet: Govulncheck
through `scripts/govulncheck_policy.py source`, Gosec v2.29.0, the reviewed Gitleaks
baseline validator and Gitleaks v8.30.1 history scan, and all security-policy,
Govulncheck-policy, and Gitleaks-baseline unit tests. Any new or unreviewed finding stops
acceptance; it is not silently suppressed or added to a baseline.

The existing manual release SBOM workflow remains mandatory for a release, but this
routine source ticket does not regenerate binaries or an SBOM. That preserves the
owner's CI-usage decision while keeping SBOM generation and scan policy in the release
gate.

## Production source review 01

Sol authored only the seven authorized production paths and ran nothing. Codex XHigh
rejected the drop before execution in
`docs/testing/BBGO-PAY-001-PRODUCTION-SOURCE-REVIEW-01.md`. Typed invalid UTF-8 can be
rewritten before signing, recursive JSON nesting is unbounded, linked request/event nonce
reuse is not rejected, and stored-record integrity validation is quadratic.

Under `docs/handoff/CODEX_SOL_BBGO_PAY_001_GAP_TESTS_01.md`, Sol may edit only the three
named payment test files and must leave the seven uncommitted production paths untouched.
The new regression tests must be statically reviewed and captured red before any
production correction. All other source, execution, integration, Git, module/lock,
network, wallet, rate, transaction, hardware, device, release, binary, and SBOM work is
unauthorized.

## Gap-test source acceptance 01

Sol added the three requested regression paths without touching the preserved production
drop. Codex XHigh accepted their exact source in
`docs/testing/BBGO-PAY-001-GAP-TEST-SOURCE-REVIEW-01.md`: typed invalid UTF-8 is exercised
through the real producer, the recursive parser is tested below/at/above a 32-container
limit, and linked request/event nonce reuse is exercised through the real two-node
transport and durable store.

Jr Dev — Codex Luna may run only the focused expected-red command and integrate only the
three accepted test files under
`docs/handoff/CODEX_LUNA_BBGO_PAY_001_GAP_EXPECTED_RED_01.md`. The seven production paths
must remain dirty, uncommitted, and byte-identical to production source review 01. No
production correction or broader validation is authorized until Codex XHigh accepts the
gap expected-red evidence.

## Gap expected-red attempt 01 rejection

Luna's exact focused command compiled the whole payment test package before selecting
the three requested tests and exposed two stale two-result bindings in the frozen
`TestNetworkStatusIsCancellationOnly`. The attempt and complete diagnostics are recorded
in `docs/testing/BBGO-PAY-001-GAP-EXPECTED-RED-ATTEMPT-01.md`; none of the three gap tests
ran, so this is not accepted red evidence.

Under `docs/handoff/CODEX_SOL_BBGO_PAY_001_TEST_COMPILE_CORRECTION_01.md`, Sol may change
only those two bindings to discard the first three results and retain the error from the
frozen four-result `DecodePaymentStatus` API. No production edit, execution, integration,
or other scope is authorized.

## Test compile-correction acceptance 01

Sol made only the two authorized result-binding corrections. Codex XHigh accepted the
exact diff and final hash in
`docs/testing/BBGO-PAY-001-TEST-COMPILE-CORRECTION-REVIEW-01.md`; the assertions' behavior
is unchanged. Luna may retry only the focused expected-red command and, on acceptable
red, integrate the four changed test files under
`docs/handoff/CODEX_LUNA_BBGO_PAY_001_GAP_EXPECTED_RED_02.md`. Production correction and
broader execution remain unauthorized.

## Gap expected-red attempt 02 rejection

The corrected package compiled and both offline regressions failed for their intended
missing protections, but the restricted execution sandbox denied the real two-node
test's ephemeral `127.0.0.1` listener. The exact result is recorded in
`docs/testing/BBGO-PAY-001-GAP-EXPECTED-RED-ATTEMPT-02.md` and is not accepted red.

Luna may request a one-command sandbox override and retry the exact focused command under
`docs/handoff/CODEX_LUNA_BBGO_PAY_001_GAP_EXPECTED_RED_03.md`. This is loopback-only test
execution, not root or public-network access. No production correction or other broad
execution is authorized.

## Gap expected-red acceptance 03

The final focused run compiled and produced exactly the three intended failures. The
real two-node path accepted and stored a status reusing its linked request nonce; the
offline paths accepted depth 33 and signed repaired invalid UTF-8. Codex XHigh accepts
the evidence in `docs/testing/BBGO-PAY-001-GAP-EXPECTED-RED-REVIEW-03.md`.

Under `docs/handoff/CODEX_SOL_BBGO_PAY_001_PRODUCTION_CORRECTION_01.md`, Sol may edit only
`canonical.go`, `signature.go`, and `service.go` to close those regressions and replace
quadratic stored-record validation with map-based linear validation. No execution,
integration, module/lock, wallet, rate, public-network, release-binary, or SBOM work is
authorized until XHigh accepts the corrected source.

## Production source acceptance 02 and green authorization

Sol corrected only `canonical.go`, `signature.go`, and `service.go`. Codex XHigh accepts
the resulting complete seven-path source for execution in
`docs/testing/BBGO-PAY-001-PRODUCTION-SOURCE-REVIEW-02.md`. The parser is bounded at 32
containers, typed producers reject invalid UTF-8 before marshal, linked request/status
nonce reuse is replay, and stored-record integrity validation is O(n).

Luna may now execute only
`docs/handoff/CODEX_LUNA_BBGO_PAY_001_GREEN_01.md`: module tidy, focused/full/race/fuzz
tests, three exact reversible falsifications, current pinned source-security gates,
evidence, and integration. Routine binaries and SBOM are explicitly excluded; they
remain manual release gates. No wallet, exchange-rate, transaction, hardware/device,
public-peer, or unrelated work is authorized.

## Green recovery 01

The first green turn completed tidy, focused/full tests, all three falsifications, vet,
race, and native fuzz target 1 before its agent turn stopped responding at fuzz target 2.
The reviewer recovered the same Luna execution history without rerunning or restoring
anything. No process remained; fuzz target 2 had no output/result; all temporary hunks
were restored at accepted hashes. Exact state is recorded in
`docs/testing/BBGO-PAY-001-GREEN-RECOVERY-01.md`.

Luna may now run only native fuzz targets 2–6 under
`docs/handoff/CODEX_LUNA_BBGO_PAY_001_CONTINUE_FUZZ_01.md`. Completed gates must not be
duplicated. Security scanning, evidence integration, Git, binaries, and SBOM remain
unauthorized pending reviewer recording of the bounded continuation.

## Fuzz target 2 repeated hang

The exact three-second `FuzzDecodeWireEnvelope` command silently stalled twice before
emitting a fuzz banner or yielding an execution result; the second invocation remained
inside foreground exec for approximately 626.1 seconds. Both were interrupted only after
extended waits. No process, output, source change, or accepted fuzz result remains. See
`docs/testing/BBGO-PAY-001-FUZZ-HANG-01.md`.

Sol may now perform only the static target/decoder audit in
`docs/handoff/CODEX_SOL_BBGO_PAY_001_FUZZ_HANG_AUDIT_01.md`. A third unchanged invocation,
security scans, integration, binaries, and SBOM are unauthorized pending XHigh review of
that audit.

## Fuzz-hang static-audit acceptance 01

Sol's command-free audit found no source-proven nontermination, failing round-trip input,
blocking/shared state, or recursion escape. Codex XHigh accepts the reasoning and a
three-stage diagnostic with one worker, JSON events, Go's internal timeout, and an outer
wall-clock watchdog in
`docs/testing/BBGO-PAY-001-FUZZ-HANG-AUDIT-REVIEW-01.md`.

A fresh Luna may run only
`docs/handoff/CODEX_LUNA_BBGO_PAY_001_FUZZ_DIAGNOSTIC_01.md`. It must stop at the first
seed, one-iteration, sustained-fuzz, watchdog, or executor failure. No source correction,
other target, fresh cache, scanner, integration, binary, or SBOM is authorized.

## Fuzz target 2 diagnostic acceptance

A fresh Luna passed the ordinary seeds, one native fuzz iteration, and a single-worker
three-second native campaign under inner/outer watchdogs. The final run executed 4,967
inputs with 67 total interesting inputs and no failure or watchdog. XHigh accepts target
2 and the executor/transport classification in
`docs/testing/BBGO-PAY-001-FUZZ-DIAGNOSTIC-REVIEW-01.md`.

Luna may now run only targets 3–6 under
`docs/handoff/CODEX_LUNA_BBGO_PAY_001_CONTINUE_FUZZ_02.md`. Targets 1–2 and prior gates
must not be rerun. Security, integration, binaries, and SBOM remain unauthorized.

## Native fuzz completion acceptance

Luna completed targets 3–6 under the same single-worker inner/outer watchdog pattern.
Together with accepted targets 1–2, all six native fuzz campaigns passed with nonzero
execution counts and no failure/artifact. Exact counts are accepted in
`docs/testing/BBGO-PAY-001-FUZZ-COMPLETION-REVIEW-01.md`. No fuzz target may be rerun.

Luna may now run only security phase A under
`docs/handoff/CODEX_LUNA_BBGO_PAY_001_SECURITY_A_01.md`: complete policy suites,
Actionlint, policy-adjudicated source Govulncheck, and Gosec. Gitleaks, integration,
binaries, and SBOM remain separately gated.

## Security phase A recovery 01

Luna passed all three policy-unit suites (51, 49, and 27 tests) and pinned Actionlint
over all three workflows. The sandboxed source Govulncheck had no usable result because
the official database was inaccessible. Its network-authorized retry then produced no
output or exit result for approximately 1,929.9 seconds before recovery. No scanner
process or state change remained; Gosec did not run. This is recorded without a false
pass or vulnerability verdict in
`docs/testing/BBGO-PAY-001-SECURITY-A-RECOVERY-01.md`.

Luna may now perform only the hard-bounded official-database probe, policy-adjudicated
source Govulncheck, and conditional Gosec under
`docs/handoff/CODEX_LUNA_BBGO_PAY_001_SECURITY_A_CONTINUE_01.md`. Completed policy suites
and Actionlint must not be duplicated. Gitleaks and integration remain separately gated.

## Security phase A acceptance 01

Luna's exact preflight passed, but its network-authority probe stalled in the sub-agent
tool channel without an HTTP or exit result. After confirming no process remained, the
reviewer recovered its execution history and ran the content-suppressed official-database
probe in the visible foreground reviewer channel: HTTP 200, exit 0. The bounded source
policy then exited 0 with only reviewed `GO-2024-3218` on exact DHT v0.42.2 and two
non-reachable `golang.org/x/crypto` notes. Pinned Gosec exited 0 after 17 files / 4,579
lines with zero issues and zero suppressions. Exact commands, timing, findings metadata,
execution-actor disposition, and unchanged state are accepted in
`docs/testing/BBGO-PAY-001-SECURITY-A-REVIEW-01.md`.

Luna may now run only the reviewed Gitleaks baseline validator, exact pinned redacted
history scan, and final read-only checks under
`docs/handoff/CODEX_LUNA_BBGO_PAY_001_SECURITY_B_01.md`. All earlier gates must not be
rerun. Integration remains separately gated; binaries and SBOM remain manual release
work and are not authorized by this routine source ticket.

## Security phase B acceptance 01 and integration authorization

The fail-closed validator accepted the exact 25-entry reviewed redacted baseline with
reviewer ownership and expiry 2026-11-29. Pinned Gitleaks v8.30.1 then scanned 3,406
commits / approximately 313.91 MB in 20.4 seconds with zero new leaks and exit 0; only
the known exhaustive-rename warnings appeared. No secret or match value was accessed or
recorded. Exact safe metadata and unchanged state are accepted in
`docs/testing/BBGO-PAY-001-SECURITY-B-REVIEW-01.md`.

Every execution/security gate is now complete and must not be rerun. Luna may create the
consolidated green evidence and perform only the exact ten-path integration under
`docs/handoff/CODEX_LUNA_BBGO_PAY_001_INTEGRATE_01.md`. Final engineering acceptance
remains owned by Codex XHigh. Binaries and SBOM remain manual release gates and are not
regenerated for this routine source ticket.

## Required test groups

1. **Independent canonical oracle:** every accepted request/status fixture matches exact
   JCS and digest; all fixture-invalid cases fail with the named stable code; add table
   coverage for every BBD-WAL-001 §11 timestamp, Unicode, type, closed-schema, amount,
   network, status/tx-ref, rate, and canonical-copy boundary not already present.
2. **Identity and signature properties:** both request and status signatures round-trip;
   every field mutation, wrong domain, digest-instead-of-bytes signing, wrong public key,
   wrong remote, wrong payer, wrong payee, noncanonical copy, and malformed key/signature
   fails. Successful verification must prove the derived libp2p peer identity.
3. **Transport framing:** split prefix/payload, byte-at-a-time, exact limit and limit+1,
   zero/truncated/coalesced/trailing frames, invalid UTF-8/JSON/base64, closed envelope,
   protocol mismatch, cancellation-only network status, explicit acknowledgement, and no
   goroutine/file-descriptor leak.
4. **Two-node real path:** in-process libp2p nodes use the actual stream handler and
   `/bitbook/payment/1.0.0`; a payee sends a payer-bound signed request, the payer verifies
   and persists it, then accepts a linked payee-signed cancellation. Tests may not
   manually invoke the receive handler or pre-seed the payer's expected object.
5. **Persistence and replay:** restart from the same in-memory datastore view, identical
   retry idempotence, conflicting request/event ID and nonce rejection, status-before-
   request rejection, wrong signer/link rejection, expired request rejection, concurrent
   duplicate delivery, storage failure before acknowledgement, and no partial accepted
   record.
6. **Negative capability contract:** package imports and public types contain no wallet,
   coin-library, HTTP client, exchange-rate, quote, account, transaction, broadcast, USB,
   or device capability. No `/ob/exchangerates` or product wallet route is introduced.
7. **Fuzz/property seeds:** strict signed-object decoding, wire-envelope decoding, frame
   parsing, JCS determinism across key-order/whitespace variants, signature mutation, and
   replay-key collision seeds. Fuzz entrypoints are bounded and ordinary runs are offline.

Tests must use deterministic clocks and in-memory datastores. In-process libp2p transport
may require sandbox approval for loopback binds when Luna executes it; the source actor
does not execute it.

## Expected red and later acceptance

After reviewer acceptance of the test-source drop, Codex Luna ran from `modern/`:

```text
GOTOOLCHAIN=go1.27.0 go test ./payment -count=1
GOTOOLCHAIN=go1.27.0 go test ./network -run TestPaymentProtocolCurrent -count=1
```

Both exited nonzero only because the reserved `payment` production API and
`network.PaymentProtocolCurrent` did not exist. The source actor did not execute them.

The production handoff freezes the exact exported Go API after test-source review.
Green acceptance will include targeted tests, `go vet ./...`, `go test -race
./... -count=1`, bounded native fuzz runs, current pinned security-policy/scanner gates,
and exact falsifications/restores for domain separation, remote-peer binding, and durable
replay conflict detection. No release binary or SBOM is regenerated on this routine
implementation ticket.
