# Codex Sol Handoff — BBGO-PAY-001 Production Source 01

You are **Principal Dev — Codex Sol**, using `gpt-5.6-sol` at High. This is the complete
durable prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Governance baseline: the commit containing this handoff

Integrated frozen-test baseline: `403df23a63f413c11e13085719fc7e767c2f15be`

Original production baseline: `0560b6426b9af29a16a151dacc7c2f3021a3dc0d`

Read completely before acting: `AGENTS.md`, `TESTING.md`,
`docs/engineering/DEVELOPMENT_ROLES.md`, `docs/engineering/PAYMENT_ROADMAP_ROUTING.md`,
`tickets/BBGO-PAY-001.md`, `docs/handoff/CURRENT_TASK.md`,
`docs/testing/BBGO-PAY-001-TEST-SOURCE-REVIEW-02.md`,
`docs/testing/BBGO-PAY-001-EXPECTED-RED.md`,
`docs/testing/BBGO-PAY-001-EXPECTED-RED-REVIEW.md`, BBD-WAL-001 §§11 and 14 in
`../bb-desktop/docs/architecture/BBD-WAL-001-REVIEW.md`, all six frozen payment test
files, the frozen protocol test, `modern/network/node.go`, `modern/network/protocols.go`,
and the existing direct-service stream/datastore patterns.

## Sole task and exact paths

Implement the frozen contract by creating or editing exactly:

- `modern/network/protocols.go`
- `modern/payment/types.go`
- `modern/payment/canonical.go`
- `modern/payment/signature.go`
- `modern/payment/frame.go`
- `modern/payment/replay.go`
- `modern/payment/service.go`

Use `apply_patch`. Production source only. Do not edit tests, fixture, module/lock input,
API or command wiring, documentation, evidence, workflows, policy, security baseline, or
any other path.

## Frozen public surface

Provide these types, constants, fields, and functions with the names and signatures
required by the accepted tests. JSON tags shown here are normative.

```go
type Kind string
const KindRequest Kind = "request"
const KindStatus Kind = "status"

type Direction string
const DirectionInbound Direction = "inbound"
const DirectionOutbound Direction = "outbound"

const StatusCancelled = "cancelled"
const StatusPaid = "paid"
const StatusExpired = "expired"

const DomainSeparatorRequest = "bitbook-payment-request-v1\n"
const DomainSeparatorStatus = "bitbook-payment-status-v1\n"
const MaxFrameBytes = 64 << 10

const CodeSchema = "SCHEMA"
const CodeFrame = "FRAME"
const CodeSignature = "SIGNATURE"
const CodeRemote = "REMOTE"
const CodePayer = "PAYER"
const CodePayee = "PAYEE"
const CodeReplay = "REPLAY"
const CodeLinkage = "LINKAGE"
const CodeExpired = "EXPIRED"
const CodeStatus = "STATUS"
const CodeStorage = "STORAGE"

type PaymentRequestV1 struct {
    V int `json:"v"`
    RequestID string `json:"request_id"`
    PayerPeerID string `json:"payer_peer_id"`
    PayeePeerID string `json:"payee_peer_id"`
    Asset string `json:"asset"`
    Network string `json:"network"`
    AmountAtomic string `json:"amount_atomic"`
    Receiver string `json:"receiver"`
    ReceiverKind string `json:"receiver_kind"`
    Memo string `json:"memo"`
    Nonce string `json:"nonce"`
    CreatedAt string `json:"created_at"`
    ExpiresAt string `json:"expires_at"`
}

type PaymentStatusEventV1 struct {
    V int `json:"v"`
    RequestID string `json:"request_id"`
    EventID string `json:"event_id"`
    Nonce string `json:"nonce"`
    Status string `json:"status"`
    At string `json:"at"`
    TxRef string `json:"tx_ref"`
}

type SignedObject struct {
    Version int `json:"version"`
    Kind Kind `json:"kind"`
    Canonical string `json:"canonical"`
    PublicKey []byte `json:"public_key"`
    Signature []byte `json:"signature"`
}

type VerifiedObject struct {
    PeerID peer.ID
    Request *PaymentRequestV1
    Status *PaymentStatusEventV1
    Digest string
}

type Acknowledgement struct {
    Accepted bool `json:"accepted"`
    Digest string `json:"digest"`
    Code string `json:"code"`
}

type RecordedObject struct {
    Signed SignedObject `json:"signed"`
    Digest string `json:"digest"`
    Direction Direction `json:"direction"`
    ReceivedAt time.Time `json:"received_at"`
}

type Error struct {
    Code string
    Message string
}
func (e *Error) Error() string

func CanonicalJSON(raw []byte) ([]byte, error)
func DecodePaymentRequest(raw []byte) (PaymentRequestV1, []byte, string, error)
func DecodePaymentStatus(raw []byte) (PaymentStatusEventV1, []byte, string, error)
func CheckCanonicalCopy(canonical, supplied []byte) error
func DigestHex(domain string, canonical []byte) string

func SignRequest(key crypto.PrivKey, request PaymentRequestV1) (SignedObject, error)
func SignStatus(key crypto.PrivKey, status PaymentStatusEventV1) (SignedObject, error)
func VerifySignedObject(signed SignedObject, remote, recipient peer.ID) (VerifiedObject, error)

func FrameLength(size int) (uint32, error)
func WriteFrame(writer io.Writer, payload []byte) error
func ReadFrame(reader io.Reader) ([]byte, error)
func DecodeSignedObject(raw []byte) (SignedObject, error)
func ReceiveSignedObject(reader io.Reader) (SignedObject, error)

func CheckRequestReplay(first PaymentRequestV1, firstDigest string, second PaymentRequestV1, secondDigest string) error
func CheckStatusReplay(first PaymentStatusEventV1, firstDigest string, second PaymentStatusEventV1, secondDigest string) error

type ServiceOption func(*serviceConfig) error
func WithClock(now func() time.Time) ServiceOption
func WithDatastore(store datastore.Batching) ServiceOption
func NewService(node *network.Node, options ...ServiceOption) (*Service, error)
func (s *Service) Close()
func (s *Service) SendRequest(ctx context.Context, request PaymentRequestV1) (Acknowledgement, error)
func (s *Service) SendStatus(ctx context.Context, status PaymentStatusEventV1) (Acknowledgement, error)
func (s *Service) SendSigned(ctx context.Context, recipient peer.ID, signed SignedObject) (Acknowledgement, error)
func (s *Service) GetRequest(ctx context.Context, requestID string) (RecordedObject, error)
func (s *Service) GetEvent(ctx context.Context, eventID string) (RecordedObject, error)
func (s *Service) List(ctx context.Context) ([]RecordedObject, error)
```

An additional internal transport error code is permitted, but the named branches must
return exactly the frozen codes. Do not add any wallet, rate, quote, account, coin-client,
transaction, broadcast, HTTP, USB, device, broker, or API capability.

## Canonical/schema contract

- Decode raw bytes as UTF-8 JSON with no BOM, no trailing data, no duplicate key at any
  object depth, and exactly the closed keys. Do not rely on `encoding/json` alone for
  duplicate-key or invalid-UTF-8 rejection.
- Reject null, Boolean, float, alternate numeric encoding, CBOR, missing/unknown/empty
  keys, and wrong types. `v` is the JSON integer token `1`, not `1.0` or a string.
- Implement RFC 8785 output for the closed signed-object value domain used here: objects,
  arrays where a later closed schema permits them, UTF-8 strings, and the exact integer
  token `1`. Sort object keys by UTF-16 code units and use minimal JSON string escaping.
  Reject unsupported JSON values/numbers instead of approximating ECMAScript number
  serialization. This is deliberately dependency-free and sufficient for every v1
  payment/status object; do not introduce a general JCS dependency.
- `CanonicalJSON` must use that strict parser/encoder and be deterministic/idempotent.
  The request/status decoders validate their schemas before returning the same canonical
  bytes and domain-separated lowercase SHA-256 digest.
- Enforce the exact six asset/network relations and two receiver kinds in the ticket.
  IDs/nonces are 32 lowercase hex; amount is `^[1-9][0-9]{0,19}$`.
- Peer IDs and receivers are nonempty printable ASCII at schema time. Do not call
  `peer.Decode` in schema validation because the accepted independent oracle uses
  syntactically representative placeholder peer strings; cryptographic verification
  performs the real identity binding.
- Timestamp shape is exactly second-precision UTC `Z`; parse a real Gregorian date,
  require years 2020–2100 inclusive, reformat byte-for-byte, reject leap seconds and
  impossible dates, and require request expiry strictly after creation.
- Every signed string rejects malformed UTF-8, BOM, C0/C1, noncharacters, the exact bidi
  and format-control ranges in BBD-WAL-001 §11. `memo` additionally must already be NFC
  and at most 512 UTF-8 bytes. Use the already-resolved
  `golang.org/x/text/unicode/norm`; do not edit `go.mod` or `go.sum`. Peer IDs, receiver,
  enums, IDs, timestamps, and `tx_ref` are ASCII; `tx_ref` is nonempty only for `paid`.
- `CheckCanonicalCopy` is byte equality against recomputed canonical output and returns
  `SCHEMA` on mismatch.

## Signature and identity order

- Sign `domain || canonical` directly with the libp2p private key. Never sign the hex
  digest, ordinary struct JSON, envelope, or CBOR.
- `SignRequest` validates/canonicalizes first, derives the signer peer ID, requires it to
  equal `payee_peer_id` (`PAYEE` otherwise), and produces the exact five-field envelope.
  `SignStatus` validates/canonicalizes and signs; request linkage occurs in the service.
- Verification order is fixed: envelope version/kind; decode and schema/canonical-copy;
  unmarshal public key (`SIGNATURE` if malformed); derive and compare live remote peer
  (`REMOTE`); verify signature (`SIGNATURE`); then request payee (`PAYEE`) and recipient
  payer (`PAYER`) bindings. This order is required by the frozen mutation tests.
- A verified status returns the signer peer but does not invent payer/payee fields;
  durable service linkage supplies those checks.

## Framing and stream contract

- Add `network.PaymentProtocolCurrent` exactly `/bitbook/payment/1.0.0` without changing
  any existing protocol value.
- Frames are one four-byte big-endian unsigned length followed by 1..65536 payload bytes.
  Read exactly, reject before allocation on zero/oversize, handle short reads/writes, and
  map malformed framing to `FRAME`.
- `DecodeSignedObject` rejects invalid UTF-8/base64, duplicate/unknown/missing envelope
  fields, trailing JSON, unsupported version/kind, and wrong field types as `SCHEMA`.
- `ReceiveSignedObject` accepts exactly one frame and then requires EOF; trailing or
  coalesced bytes are `FRAME`. On libp2p streams the sender must half-close its write side
  after the request frame, and the receiver must half-close after the acknowledgement,
  so exact-one-frame checks terminate without weakening the property.
- Use context/stream deadlines with a bounded duration. Never start retry or background
  delivery goroutines.

## Durable service contract

- Register only `network.PaymentProtocolCurrent`; `Close` removes that handler, is
  idempotent, prevents new handler work, and waits for admitted handlers to finish.
- Default to `node.Datastore`; `WithDatastore` overrides only payment persistence.
  `WithClock` controls expiry decisions and exact inbound/outbound `ReceivedAt` values.
- Persist a single JSON record collection at a payment-namespaced datastore key. Protect
  load/check/replace/Put with one service mutex. A single failing Put/Batch must leave no
  partial record or secondary index. Decode stored canonical bytes on lookup; corrupted
  stored state is `STORAGE`.
- Store only `SignedObject`, digest, direction, and receipt time. A receiver persists
  after all schema, canonical, identity, signature, payer, expiry, linkage, replay, and
  status checks and before sending an accepted acknowledgement. A sender persists its
  outbound record only after receiving an accepted acknowledgement.
- Identical digest delivery is idempotent and does not duplicate or retimestamp state.
  Different bytes reusing request ID/nonce or event ID/nonce return `REPLAY`.
- Incoming request expiry is `now >= expires_at` and returns `EXPIRED` without storage.
- For incoming status, locate the referenced stored request (`LINKAGE` if absent), then
  require signer equals that request's payee (`PAYEE`). Check identical/replay ID and
  nonce before terminal-state policy. Therefore conflicts against an existing cancel are
  `REPLAY`; a distinct later event for the same request is `STATUS` and is not stored.
- v1 network status is cancellation only with empty `tx_ref`. Codec-valid paid/expired
  values return `STATUS` from both convenience send and receive defenses.
- `SendRequest` signs with the node identity and sends to decoded `payer_peer_id`.
  `SendStatus` finds the local stored request only to identify its payer; it must not
  locally short-circuit replay or terminal conflicts that the receiver must classify.
  `SendSigned` may locally verify its signature/envelope but must not require local status
  linkage, so a non-payee sender reaches the receiver and receives the required `PAYEE`.
- Rejected acknowledgements return both the received acknowledgement and a typed `Error`
  carrying the same stable code. Do not automatically retry or consult the DHT/public
  peers; the caller supplies connectivity.

## Source-actor stop conditions

Do not execute Go, tests, builds, formatters, fuzzers, race tests, vet, scanners, Git,
GitHub, network commands, daemons, public peers, wallets, rates, transactions, hardware,
devices, release work, binaries, or SBOM generation. Do not use root, `sudo`, `/tmp`,
deletion, cleanup, `rm`, globs, unresolved destructive targets, or another repository.

Stop after the seven production paths are authored. Report every changed path, line
count, SHA-256, all exported symbols, persistence key/layout, validation order, replay
ordering, handler shutdown design, and confirmation that tests/fixtures/module inputs
and prohibited capabilities were untouched. Codex XHigh must review the source before
Luna runs `go mod tidy`, green tests, race/fuzz/falsification, or security gates.
