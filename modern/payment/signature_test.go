package payment

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TestRequestAndStatusSignaturesRoundTrip(t *testing.T) {
	payee, payer := newIdentity(t), newIdentity(t)
	req := liveRequest(payee.id, payer.id)
	signedReq, err := SignRequest(payee.priv, req)
	if err != nil {
		t.Fatal(err)
	}
	verifiedReq, err := VerifySignedObject(signedReq, payee.id, payer.id)
	if err != nil {
		t.Fatal(err)
	}
	if verifiedReq.PeerID != payee.id {
		t.Fatalf("request peer ID = %s, want %s", verifiedReq.PeerID, payee.id)
	}
	if verifiedReq.Request == nil || verifiedReq.Request.RequestID != req.RequestID {
		t.Fatalf("verified request missing payload: %+v", verifiedReq.Request)
	}
	if verifiedReq.Digest == "" || verifiedReq.Digest != DigestHex(DomainSeparatorRequest, []byte(signedReq.Canonical)) {
		t.Fatalf("request digest was empty or mismatched: %q", verifiedReq.Digest)
	}

	event := liveCancel(req.RequestID)
	signedStatus, err := SignStatus(payee.priv, event)
	if err != nil {
		t.Fatal(err)
	}
	verifiedStatus, err := VerifySignedObject(signedStatus, payee.id, payer.id)
	if err != nil {
		t.Fatal(err)
	}
	if verifiedStatus.PeerID != payee.id || verifiedStatus.Status == nil || verifiedStatus.Status.Status != StatusCancelled {
		t.Fatalf("status round-trip failed: %+v", verifiedStatus)
	}
}

func TestSuccessfulVerificationProvesDerivedPeerIdentity(t *testing.T) {
	payee, payer := newIdentity(t), newIdentity(t)
	signed, err := SignRequest(payee.priv, liveRequest(payee.id, payer.id))
	if err != nil {
		t.Fatal(err)
	}
	verified, err := VerifySignedObject(signed, payee.id, payer.id)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := crypto.UnmarshalPublicKey(signed.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	derived, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	if derived != payee.id || verified.PeerID != derived || derived.String() != verified.Request.PayeePeerID {
		t.Fatalf("derived identity %s did not prove payee %s", derived, payee.id)
	}
}

func TestProducerCannotSignForAnotherPayee(t *testing.T) {
	payee, payer, other := newIdentity(t), newIdentity(t), newIdentity(t)
	req := liveRequest(other.id, payer.id)
	if _, err := SignRequest(payee.priv, req); codeOf(err) != CodePayee {
		t.Fatalf("foreign payee sign code = %q (%v)", codeOf(err), err)
	}
}

func TestSignRequestRejectsInvalidUTF8MemoBeforeMarshal(t *testing.T) {
	payee, payer := newIdentity(t), newIdentity(t)
	request := liveRequest(payee.id, payer.id)
	request.Memo = string([]byte{'c', 'o', 0xff, 'f', 'f', 'e', 'e'})
	if utf8.ValidString(request.Memo) {
		t.Fatal("invalid UTF-8 producer fixture was unexpectedly valid")
	}
	if _, err := SignRequest(payee.priv, request); codeOf(err) != CodeSchema {
		t.Fatalf("invalid UTF-8 producer code = %q, want %q (%v)", codeOf(err), CodeSchema, err)
	}
}

func TestSignatureRejectsEveryDeclaredFieldMutation(t *testing.T) {
	payee, payer, other := newIdentity(t), newIdentity(t), newIdentity(t)
	request := liveRequest(payee.id, payer.id)
	signedRequest, err := SignRequest(payee.priv, request)
	if err != nil {
		t.Fatal(err)
	}
	requestCases := []struct {
		name     string
		field    string
		value    any
		wantCode string
	}{
		{name: "requestVersion", field: "v", value: 2, wantCode: CodeSchema},
		{name: "requestID", field: "request_id", value: "10112233445566778899aabbccddeeff", wantCode: CodeSignature},
		{name: "payerPeerID", field: "payer_peer_id", value: other.id.String(), wantCode: CodeSignature},
		{name: "payeePeerID", field: "payee_peer_id", value: other.id.String(), wantCode: CodeSignature},
		{name: "asset", field: "asset", value: "XMR", wantCode: CodeSchema},
		{name: "network", field: "network", value: "zec-regtest", wantCode: CodeSignature},
		{name: "amountAtomic", field: "amount_atomic", value: "100000001", wantCode: CodeSignature},
		{name: "receiver", field: "receiver", value: "u1otherreceiver", wantCode: CodeSignature},
		{name: "receiverKind", field: "receiver_kind", value: "xmr-subaddress", wantCode: CodeSchema},
		{name: "memo", field: "memo", value: "tea", wantCode: CodeSignature},
		{name: "nonce", field: "nonce", value: "00112233445566778899aabbccddeeff", wantCode: CodeSignature},
		{name: "createdAt", field: "created_at", value: "2026-08-30T11:59:59Z", wantCode: CodeSignature},
		{name: "expiresAt", field: "expires_at", value: "2026-08-30T12:16:00Z", wantCode: CodeSignature},
	}
	for _, test := range requestCases {
		t.Run(test.name, func(t *testing.T) {
			fields := paymentObjectMap(t, request)
			fields[test.field] = test.value
			mutated := cloneSignedObject(signedRequest)
			raw := mustJSON(t, fields)
			if test.wantCode == CodeSchema {
				mutated.Canonical = string(raw)
			} else {
				_, canonical, _, decodeErr := DecodePaymentRequest(raw)
				if decodeErr != nil {
					t.Fatalf("schema-valid mutation was rejected before verification: %v", decodeErr)
				}
				mutated.Canonical = string(canonical)
			}
			assertSignedObjectChanged(t, signedRequest, mutated)
			if got := codeOf(verifyErr(t, mutated, payee.id, payer.id)); got != test.wantCode {
				t.Fatalf("code = %q, want %q", got, test.wantCode)
			}
		})
	}

	status := liveCancel(request.RequestID)
	signedStatus, err := SignStatus(payee.priv, status)
	if err != nil {
		t.Fatal(err)
	}
	statusCases := []struct {
		name     string
		field    string
		value    any
		wantCode string
	}{
		{name: "statusVersion", field: "v", value: 2, wantCode: CodeSchema},
		{name: "statusRequestID", field: "request_id", value: "10112233445566778899aabbccddeeff", wantCode: CodeSignature},
		{name: "eventID", field: "event_id", value: "21112222333344445555666677778888", wantCode: CodeSignature},
		{name: "eventNonce", field: "nonce", value: "8999aaaabbbbccccddddeeeeffff0000", wantCode: CodeSignature},
		{name: "status", field: "status", value: StatusExpired, wantCode: CodeSignature},
		{name: "statusAt", field: "at", value: "2026-08-30T12:06:00Z", wantCode: CodeSignature},
		{name: "txRef", field: "tx_ref", value: "txid", wantCode: CodeSchema},
	}
	for _, test := range statusCases {
		t.Run(test.name, func(t *testing.T) {
			fields := paymentObjectMap(t, status)
			fields[test.field] = test.value
			mutated := cloneSignedObject(signedStatus)
			raw := mustJSON(t, fields)
			if test.wantCode == CodeSchema {
				mutated.Canonical = string(raw)
			} else {
				_, canonical, _, decodeErr := DecodePaymentStatus(raw)
				if decodeErr != nil {
					t.Fatalf("schema-valid mutation was rejected before verification: %v", decodeErr)
				}
				mutated.Canonical = string(canonical)
			}
			assertSignedObjectChanged(t, signedStatus, mutated)
			if got := codeOf(verifyErr(t, mutated, payee.id, payer.id)); got != test.wantCode {
				t.Fatalf("code = %q, want %q", got, test.wantCode)
			}
		})
	}

	otherPublicKey, err := crypto.MarshalPublicKey(other.priv.GetPublic())
	if err != nil {
		t.Fatal(err)
	}
	canonicalFields := paymentObjectMap(t, request)
	canonicalFields["memo"] = "outer-canonical-mutation"
	_, alteredCanonical, _, err := DecodePaymentRequest(mustJSON(t, canonicalFields))
	if err != nil {
		t.Fatal(err)
	}
	envelopeCases := []struct {
		name     string
		wantCode string
		mutate   func(*SignedObject)
	}{
		{name: "envelopeVersion", wantCode: CodeSchema, mutate: func(obj *SignedObject) { obj.Version = 2 }},
		{name: "envelopeKind", wantCode: CodeSchema, mutate: func(obj *SignedObject) { obj.Kind = KindStatus }},
		{name: "envelopeCanonical", wantCode: CodeSignature, mutate: func(obj *SignedObject) { obj.Canonical = string(alteredCanonical) }},
		{name: "envelopePublicKey", wantCode: CodeRemote, mutate: func(obj *SignedObject) { obj.PublicKey = bytes.Clone(otherPublicKey) }},
		{name: "envelopeSignature", wantCode: CodeSignature, mutate: func(obj *SignedObject) { obj.Signature[0] ^= 0xff }},
	}
	for _, test := range envelopeCases {
		t.Run(test.name, func(t *testing.T) {
			mutated := cloneSignedObject(signedRequest)
			test.mutate(&mutated)
			assertSignedObjectChanged(t, signedRequest, mutated)
			if got := codeOf(verifyErr(t, mutated, payee.id, payer.id)); got != test.wantCode {
				t.Fatalf("code = %q, want %q", got, test.wantCode)
			}
		})
	}
}

func TestSignatureRejectsWrongDomainSeparator(t *testing.T) {
	payee, payer := newIdentity(t), newIdentity(t)
	req := liveRequest(payee.id, payer.id)
	_, canonical, _, err := DecodePaymentRequest(mustJSON(t, req))
	if err != nil {
		t.Fatal(err)
	}
	pub, err := crypto.MarshalPublicKey(payee.priv.GetPublic())
	if err != nil {
		t.Fatal(err)
	}
	sig, err := payee.priv.Sign(append([]byte(DomainSeparatorStatus), canonical...))
	if err != nil {
		t.Fatal(err)
	}
	signed := SignedObject{Version: 1, Kind: KindRequest, Canonical: string(canonical), PublicKey: pub, Signature: sig}
	if _, err := VerifySignedObject(signed, payee.id, payer.id); codeOf(err) != CodeSignature {
		t.Fatalf("wrong domain code = %q, want %q (%v)", codeOf(err), CodeSignature, err)
	}
}

func TestSignatureRejectsDigestInsteadOfCanonicalBytes(t *testing.T) {
	payee, payer := newIdentity(t), newIdentity(t)
	req := liveRequest(payee.id, payer.id)
	_, canonical, _, err := DecodePaymentRequest(mustJSON(t, req))
	if err != nil {
		t.Fatal(err)
	}
	pub, err := crypto.MarshalPublicKey(payee.priv.GetPublic())
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(append([]byte(DomainSeparatorRequest), canonical...))
	sig, err := payee.priv.Sign(sum[:])
	if err != nil {
		t.Fatal(err)
	}
	signed := SignedObject{Version: 1, Kind: KindRequest, Canonical: string(canonical), PublicKey: pub, Signature: sig}
	if _, err := VerifySignedObject(signed, payee.id, payer.id); codeOf(err) != CodeSignature {
		t.Fatalf("digest-signature code = %q, want %q (%v)", codeOf(err), CodeSignature, err)
	}
}

func TestSignatureRejectsWrongPublicKey(t *testing.T) {
	payee, payer, other := newIdentity(t), newIdentity(t), newIdentity(t)
	signed, err := SignRequest(payee.priv, liveRequest(payee.id, payer.id))
	if err != nil {
		t.Fatal(err)
	}
	pub, err := crypto.MarshalPublicKey(other.priv.GetPublic())
	if err != nil {
		t.Fatal(err)
	}
	signed.PublicKey = pub
	if _, err := VerifySignedObject(signed, payee.id, payer.id); codeOf(err) != CodeRemote {
		t.Fatalf("wrong public key code = %q, want %q (%v)", codeOf(err), CodeRemote, err)
	}
}

func TestSignatureRejectsWrongRemotePeer(t *testing.T) {
	payee, payer, other := newIdentity(t), newIdentity(t), newIdentity(t)
	signed, err := SignRequest(payee.priv, liveRequest(payee.id, payer.id))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifySignedObject(signed, other.id, payer.id); codeOf(err) != CodeRemote {
		t.Fatalf("wrong remote code = %q (%v)", codeOf(err), err)
	}
}

func TestStatusSignatureRejectsWrongRemotePeer(t *testing.T) {
	payee, payer, other := newIdentity(t), newIdentity(t), newIdentity(t)
	signed, err := SignStatus(payee.priv, liveCancel("00112233445566778899aabbccddeeff"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifySignedObject(signed, other.id, payer.id); codeOf(err) != CodeRemote {
		t.Fatalf("wrong status remote code = %q, want %q (%v)", codeOf(err), CodeRemote, err)
	}
}

func TestSignatureRejectsWrongPayer(t *testing.T) {
	payee, payer, other := newIdentity(t), newIdentity(t), newIdentity(t)
	signed, err := SignRequest(payee.priv, liveRequest(payee.id, payer.id))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifySignedObject(signed, payee.id, other.id); codeOf(err) != CodePayer {
		t.Fatalf("wrong payer code = %q (%v)", codeOf(err), err)
	}
}

func TestSignatureRejectsWrongPayee(t *testing.T) {
	payee, payer, other := newIdentity(t), newIdentity(t), newIdentity(t)
	req := liveRequest(other.id, payer.id)
	_, canonical, _, err := DecodePaymentRequest(mustJSON(t, req))
	if err != nil {
		t.Fatal(err)
	}
	pub, err := crypto.MarshalPublicKey(payee.priv.GetPublic())
	if err != nil {
		t.Fatal(err)
	}
	sig, err := payee.priv.Sign(append([]byte(DomainSeparatorRequest), canonical...))
	if err != nil {
		t.Fatal(err)
	}
	signed := SignedObject{Version: 1, Kind: KindRequest, Canonical: string(canonical), PublicKey: pub, Signature: sig}
	if _, err := VerifySignedObject(signed, payee.id, payer.id); codeOf(err) != CodePayee {
		t.Fatalf("wrong payee code = %q (%v)", codeOf(err), err)
	}
}

func TestNoncanonicalCopyIsRejectedBeforeSignature(t *testing.T) {
	payee, payer := newIdentity(t), newIdentity(t)
	req := liveRequest(payee.id, payer.id)
	signed, err := SignRequest(payee.priv, req)
	if err != nil {
		t.Fatal(err)
	}
	pretty, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if string(pretty) == signed.Canonical {
		t.Fatal("pretty canonical-copy mutation did not change the signed bytes")
	}
	signed.Canonical = string(pretty)
	err = verifyErr(t, signed, payee.id, payer.id)
	if codeOf(err) != CodeSchema {
		t.Fatalf("noncanonical copy code = %q (%v)", codeOf(err), err)
	}
}

func TestMalformedKeyAndSignatureFail(t *testing.T) {
	payee, payer := newIdentity(t), newIdentity(t)
	signed, err := SignRequest(payee.priv, liveRequest(payee.id, payer.id))
	if err != nil {
		t.Fatal(err)
	}
	t.Run("malformedKey", func(t *testing.T) {
		bad := signed
		bad.PublicKey = []byte("not-a-libp2p-key")
		if _, err := VerifySignedObject(bad, payee.id, payer.id); codeOf(err) != CodeSignature {
			t.Fatalf("malformed key code = %q (%v)", codeOf(err), err)
		}
	})
	t.Run("malformedSignature", func(t *testing.T) {
		bad := signed
		bad.Signature = []byte{0x00}
		if _, err := VerifySignedObject(bad, payee.id, payer.id); codeOf(err) != CodeSignature {
			t.Fatalf("malformed signature code = %q (%v)", codeOf(err), err)
		}
	})
}

type testIdentity struct {
	priv crypto.PrivKey
	id   peer.ID
}

var identitySeedState = struct {
	sync.Mutex
	next map[string]uint64
}{next: make(map[string]uint64)}

func newIdentity(t testing.TB) testIdentity {
	t.Helper()
	identitySeedState.Lock()
	index := identitySeedState.next[t.Name()]
	identitySeedState.next[t.Name()] = index + 1
	identitySeedState.Unlock()
	seed := sha256.Sum256([]byte(fmt.Sprintf("bitbook-pay-001-test-identity:%s:%d", t.Name(), index)))
	privateBytes := ed25519.NewKeyFromSeed(seed[:])
	priv, err := crypto.UnmarshalEd25519PrivateKey(privateBytes)
	if err != nil {
		t.Fatal(err)
	}
	id, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return testIdentity{priv: priv, id: id}
}

func paymentObjectMap(t testing.TB, value any) map[string]any {
	t.Helper()
	raw := mustJSON(t, value)
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	return fields
}

func cloneSignedObject(value SignedObject) SignedObject {
	value.PublicKey = bytes.Clone(value.PublicKey)
	value.Signature = bytes.Clone(value.Signature)
	return value
}

func assertSignedObjectChanged(t testing.TB, original, mutated SignedObject) {
	t.Helper()
	if original.Version == mutated.Version && original.Kind == mutated.Kind &&
		original.Canonical == mutated.Canonical && bytes.Equal(original.PublicKey, mutated.PublicKey) &&
		bytes.Equal(original.Signature, mutated.Signature) {
		t.Fatal("signed-object mutation did not change its candidate")
	}
}

func liveRequest(payee, payer peer.ID) PaymentRequestV1 {
	return PaymentRequestV1{
		V:            1,
		RequestID:    "00112233445566778899aabbccddeeff",
		PayerPeerID:  payer.String(),
		PayeePeerID:  payee.String(),
		Asset:        "ZEC",
		Network:      "zec-testnet",
		AmountAtomic: "100000000",
		Receiver:     "u1testreceiver",
		ReceiverKind: "zec-ua-orchard-protocol",
		Memo:         "coffee",
		Nonce:        "ffeeddccbbaa99887766554433221100",
		CreatedAt:    "2026-08-30T12:00:00Z",
		ExpiresAt:    "2026-08-30T12:15:00Z",
	}
}

func liveCancel(requestID string) PaymentStatusEventV1 {
	return PaymentStatusEventV1{
		V:         1,
		RequestID: requestID,
		EventID:   "11112222333344445555666677778888",
		Nonce:     "9999aaaabbbbccccddddeeeeffff0000",
		Status:    StatusCancelled,
		At:        "2026-08-30T12:05:00Z",
		TxRef:     "",
	}
}

func verifyErr(t *testing.T, signed SignedObject, remote, recipient peer.ID) error {
	t.Helper()
	_, err := VerifySignedObject(signed, remote, recipient)
	if err == nil {
		t.Fatal("expected verification error")
	}
	return err
}
