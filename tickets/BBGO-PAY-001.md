# BBGO-PAY-001 — Signed Payer-Bound Payment Objects and Transport

Status: GROK TEST DROP REJECTED — SOL CORRECTION AUTHORIZED

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

## Current authorization — test source only

At or after the time gate, Grok Build may create or edit only:

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

No production source, API, command wiring, module/lock input, documentation, workflow,
policy implementation, evidence, Git, GitHub, network command, public peer, wallet, node
process, rate provider, transaction, hardware, or other path is authorized in this phase.

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

After reviewer acceptance of the source drop, Codex Luna runs from `modern/`:

```text
GOTOOLCHAIN=go1.27.0 go test ./payment -count=1
GOTOOLCHAIN=go1.27.0 go test ./network -run TestPaymentProtocolCurrent -count=1
```

Both must exit nonzero only because the reserved `payment` production API and
`network.PaymentProtocolCurrent` do not exist. The source actor must not execute them.

A later production handoff will freeze the exact exported Go API after test-source
review. Green acceptance will include targeted tests, `go vet ./...`, `go test -race
./... -count=1`, bounded native fuzz runs, current pinned security-policy/scanner gates,
and exact falsifications/restores for domain separation, remote-peer binding, and durable
replay conflict detection. No release binary or SBOM is regenerated on this routine
implementation ticket.
