package payment

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func FuzzDecodeSignedObject(f *testing.F) {
	payee, payer := newIdentity(f), newIdentity(f)
	signed, err := SignRequest(payee.priv, liveRequest(payee.id, payer.id))
	if err != nil {
		f.Fatal(err)
	}
	raw, err := json.Marshal(signed)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(raw)
	f.Add([]byte(`{"version":1,"kind":"request"}`))
	f.Add([]byte(`{"version":2,"kind":"status","canonical":"{}","public_key":"","signature":""}`))
	f.Add([]byte(`{`))
	f.Add([]byte{0xff, 0xfe, 0xfd})
	f.Fuzz(func(t *testing.T, data []byte) {
		obj, err := DecodeSignedObject(data)
		if err != nil {
			return
		}
		if obj.Version != 1 || (obj.Kind != KindRequest && obj.Kind != KindStatus) {
			t.Fatalf("decoder accepted an open envelope: %+v", obj)
		}
		if _, err := json.Marshal(obj); err != nil {
			t.Fatal(err)
		}
	})
}

func FuzzDecodeWireEnvelope(f *testing.F) {
	f.Add([]byte(`{"version":1,"kind":"request","canonical":"{}","public_key":"AA==","signature":"AA=="}`))
	f.Add([]byte(`{"version":1,"kind":"status","canonical":"{ }","public_key":"AA==","signature":"AA==","extra":true}`))
	f.Add([]byte(`[]`))
	f.Add([]byte("null"))
	f.Fuzz(func(t *testing.T, data []byte) {
		obj, err := DecodeSignedObject(data)
		if err != nil {
			return
		}
		rewritten, err := json.Marshal(obj)
		if err != nil {
			t.Fatal(err)
		}
		again, err := DecodeSignedObject(rewritten)
		if err != nil {
			t.Fatalf("closed envelope failed a rewrite round trip: %v", err)
		}
		if again.Kind != obj.Kind || again.Version != obj.Version {
			t.Fatalf("rewrite changed envelope identity: %+v -> %+v", obj, again)
		}
	})
}

func FuzzParseFrame(f *testing.F) {
	var prefix [4]byte
	payload := []byte(`{"version":1}`)
	binary.BigEndian.PutUint32(prefix[:], uint32(len(payload)))
	f.Add(append(prefix[:], payload...))
	f.Add([]byte{0, 0, 0, 0})
	f.Add([]byte{0xff, 0xff, 0xff, 0xff})
	f.Add([]byte{0, 0})
	f.Add(append(prefix[:], bytes.Repeat([]byte("x"), MaxFrameBytes)...))
	f.Fuzz(func(t *testing.T, data []byte) {
		reader := bytes.NewReader(data)
		got, err := ReadFrame(reader)
		if err != nil {
			return
		}
		if len(got) == 0 || len(got) > MaxFrameBytes {
			t.Fatalf("frame decoder returned %d bytes", len(got))
		}
		if reader.Len() != 0 {
			t.Fatalf("accepted frame left %d trailing or coalesced bytes", reader.Len())
		}
	})
}

func FuzzJCSDeterminism(f *testing.F) {
	compact := mustJSON(f, baseRequest())
	pretty, err := json.MarshalIndent(baseRequest(), "", " ")
	if err != nil {
		f.Fatal(err)
	}
	reordered := []byte(`{"memo":"coffee","v":1,"request_id":"00112233445566778899aabbccddeeff","payer_peer_id":"12D3KooWPayer","payee_peer_id":"12D3KooWPayee","asset":"ZEC","network":"zec-testnet","amount_atomic":"100000000","receiver":"u1testreceiver","receiver_kind":"zec-ua-orchard-protocol","nonce":"ffeeddccbbaa99887766554433221100","created_at":"2026-08-30T12:00:00Z","expires_at":"2026-08-30T12:15:00Z"}`)
	f.Add(compact)
	f.Add(pretty)
	f.Add(reordered)
	f.Fuzz(func(t *testing.T, data []byte) {
		canonical, err := CanonicalJSON(data)
		if err != nil {
			return
		}
		again, err := CanonicalJSON(canonical)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(canonical, again) {
			t.Fatalf("JCS is not idempotent: %s -> %s", canonical, again)
		}
		req, decoded, digest, err := DecodePaymentRequest(data)
		if err != nil {
			return
		}
		if !bytes.Equal(decoded, canonical) {
			t.Fatal("request decode canonical diverged from CanonicalJSON")
		}
		if digest != DigestHex(DomainSeparatorRequest, canonical) {
			t.Fatal("digest diverged")
		}
		_ = req
	})
}

func FuzzSignatureMutation(f *testing.F) {
	payee, payer := newIdentity(f), newIdentity(f)
	signed, err := SignRequest(payee.priv, liveRequest(payee.id, payer.id))
	if err != nil {
		f.Fatal(err)
	}
	raw, err := json.Marshal(signed)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(raw)
	f.Add(append(bytes.Clone(raw), 0x20))
	f.Add(bytes.ReplaceAll(raw, []byte("coffee"), []byte("tea!!!")))
	f.Fuzz(func(t *testing.T, data []byte) {
		obj, err := DecodeSignedObject(data)
		if err != nil {
			return
		}
		verified, err := VerifySignedObject(obj, payee.id, payer.id)
		if err != nil {
			return
		}
		if verified.Digest != DigestHex(DomainSeparatorRequest, []byte(signed.Canonical)) {
			t.Fatalf("mutated bytes verified under a different digest: %s", verified.Digest)
		}
		if !bytes.Equal(obj.Signature, signed.Signature) || !bytes.Equal(obj.PublicKey, signed.PublicKey) {
			t.Fatal("mutated key or signature verified")
		}
	})
}

func FuzzReplayKeyCollision(f *testing.F) {
	first := mustJSON(f, baseRequest())
	secondMap := baseRequest()
	secondMap["memo"] = "tea"
	second := mustJSON(f, secondMap)
	thirdMap := baseRequest()
	thirdMap["request_id"] = "abcdabcdabcdabcdabcdabcdabcdabcd"
	third := mustJSON(f, thirdMap)
	statusFirst := mustJSON(f, baseStatus())
	statusSecondMap := baseStatus()
	statusSecondMap["at"] = "2026-08-30T12:06:00Z"
	statusSecond := mustJSON(f, statusSecondMap)
	statusThirdMap := baseStatus()
	statusThirdMap["event_id"] = "ccccddddaaaabbbbffffeeee00001111"
	statusThird := mustJSON(f, statusThirdMap)
	f.Add(first, second, statusFirst, statusSecond)
	f.Add(first, first, statusFirst, statusFirst)
	f.Add(first, third, statusFirst, statusThird)
	f.Fuzz(func(t *testing.T, a, b, statusRawA, statusRawB []byte) {
		reqA, _, digestA, errA := DecodePaymentRequest(a)
		reqB, _, digestB, errB := DecodePaymentRequest(b)
		if errA != nil || errB != nil {
			return
		}
		err := CheckRequestReplay(reqA, digestA, reqB, digestB)
		sameBytes := digestA == digestB
		collision := !sameBytes && (reqA.RequestID == reqB.RequestID || reqA.Nonce == reqB.Nonce)
		if collision && codeOf(err) != CodeReplay {
			t.Fatalf("replay collision not detected for %s/%s vs %s/%s", reqA.RequestID, reqA.Nonce, reqB.RequestID, reqB.Nonce)
		}
		if sameBytes && err != nil {
			t.Fatalf("identical digest treated as replay: %v", err)
		}
		statusA, _, digestSA, errSA := DecodePaymentStatus(statusRawA)
		statusB, _, digestSB, errSB := DecodePaymentStatus(statusRawB)
		if errSA != nil || errSB != nil {
			return
		}
		statusErr := CheckStatusReplay(statusA, digestSA, statusB, digestSB)
		statusSameBytes := digestSA == digestSB
		statusCollision := !statusSameBytes &&
			(statusA.EventID == statusB.EventID || statusA.Nonce == statusB.Nonce)
		if statusCollision && codeOf(statusErr) != CodeReplay {
			t.Fatalf("status replay collision not detected for %s/%s vs %s/%s", statusA.EventID, statusA.Nonce, statusB.EventID, statusB.Nonce)
		}
		if statusSameBytes && statusErr != nil {
			t.Fatalf("identical status digest treated as replay: %v", statusErr)
		}
	})
}
