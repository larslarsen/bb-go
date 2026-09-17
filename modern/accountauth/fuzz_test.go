package accountauth

import (
	"bytes"
	"encoding/binary"
	"testing"
)

type recordView struct {
	kind         Kind
	account      AccountID
	grant        GrantID
	device       [32]byte
	capabilities Capability
	raw          []byte
}

func viewRecord(record Record) recordView {
	return recordView{
		kind:         record.Kind(),
		account:      record.AccountID(),
		grant:        record.GrantID(),
		device:       record.DeviceKey(),
		capabilities: record.Capabilities(),
		raw:          testCloneBytes(record.Bytes()),
	}
}

func equalRecordView(left, right recordView) bool {
	return left.kind == right.kind &&
		left.account == right.account &&
		left.grant == right.grant &&
		left.device == right.device &&
		left.capabilities == right.capabilities &&
		bytes.Equal(left.raw, right.raw)
}

func FuzzVerifyRecord(f *testing.F) {
	keys := newTestKeys(f)
	grant := testGrant(f, keys.controllerPrivate, keys.devicePrivate,
		CapMessage|CapPaymentRequest|CapAssessment, testNonce(30_001))
	grantID := testGrantID(grant)
	revokeGrant := testRevokeGrant(f, keys.controllerPrivate, grantID)
	revokeDevice := testRevokeDevice(f, keys.controllerPrivate, keys.device)

	badSignature := testCloneBytes(grant)
	badSignature[len(badSignature)-1] ^= 1
	unknownVersionPayload := testCloneBytes(grant[:106])
	unknownVersionPayload[0] = 2
	unknownVersion := testSignedGrantPayload(unknownVersionPayload, keys.controllerPrivate, keys.devicePrivate)
	zeroCapabilityPayload := testCloneBytes(grant[:106])
	clear(zeroCapabilityPayload[66:74])
	zeroCapability := testSignedGrantPayload(zeroCapabilityPayload, keys.controllerPrivate, keys.devicePrivate)
	crossDomain := testSignedRevocationPayload(
		testRevocationPayload(2, keys.controller, [32]byte(grantID)),
		testRevokeDeviceDomain,
		keys.controllerPrivate,
	)
	forgedRevokeGrant := testSignedRevocationPayload(
		revokeGrant[:66], testRevokeGrantDomain, keys.devicePrivate)
	forgedRevokeDevice := testSignedRevocationPayload(
		revokeDevice[:66], testRevokeDeviceDomain, keys.devicePrivate)

	for _, seed := range [][]byte{
		grant,
		revokeGrant,
		revokeDevice,
		badSignature,
		unknownVersion,
		zeroCapability,
		crossDomain,
		forgedRevokeGrant,
		forgedRevokeDevice,
		testCloneBytes(grant[:len(grant)-1]),
		append(testCloneBytes(revokeGrant), 0),
		nil,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		original := testCloneBytes(raw)
		first, firstErr := VerifyRecord(raw)
		second, secondErr := VerifyRecord(raw)
		if (firstErr == nil) != (secondErr == nil) {
			t.Fatalf("verification was nondeterministic: first=%v second=%v", firstErr, secondErr)
		}
		if firstErr != nil {
			assertZeroRecord(t, first)
			assertZeroRecord(t, second)
			return
		}

		firstView := viewRecord(first)
		secondView := viewRecord(second)
		if !equalRecordView(firstView, secondView) {
			t.Fatalf("verified records differ: first=%+v second=%+v", firstView, secondView)
		}
		if !bytes.Equal(firstView.raw, original) {
			t.Fatalf("verified bytes = %x, want input %x", firstView.raw, original)
		}

		if len(raw) > 0 {
			raw[0] ^= 0xff
		}
		if got := viewRecord(first); !equalRecordView(got, firstView) {
			t.Fatalf("input mutation changed verified record: before=%+v after=%+v", firstView, got)
		}
		returned := first.Bytes()
		if len(returned) > 0 {
			returned[len(returned)-1] ^= 0xff
		}
		if got := viewRecord(first); !equalRecordView(got, firstView) {
			t.Fatalf("returned-byte mutation changed verified record: before=%+v after=%+v", firstView, got)
		}
	})
}

type knownProbe struct {
	grant    GrantID
	device   [32]byte
	required Capability
}

type knownView struct {
	saturated bool
	decisions []bool
}

func viewKnown(state *KnownState, probes []knownProbe) knownView {
	view := knownView{
		saturated: state.Saturated(),
		decisions: make([]bool, len(probes)),
	}
	for i, probe := range probes {
		view.decisions[i] = state.AuthorizesKnown(probe.grant, probe.device, probe.required)
	}
	return view
}

func equalKnownView(left, right knownView) bool {
	if left.saturated != right.saturated || len(left.decisions) != len(right.decisions) {
		return false
	}
	for i := range left.decisions {
		if left.decisions[i] != right.decisions[i] {
			return false
		}
	}
	return true
}

func packFuzzRecords(records ...[]byte) []byte {
	var packed []byte
	for _, record := range records {
		var size [2]byte
		binary.BigEndian.PutUint16(size[:], uint16(len(record)))
		packed = append(packed, size[:]...)
		packed = append(packed, record...)
	}
	return packed
}

func unpackFuzzRecords(packed []byte) [][]byte {
	const attempts = 4
	records := make([][]byte, 0, attempts)
	for len(packed) > 0 && len(records) < attempts {
		if len(packed) < 2 {
			records = append(records, testCloneBytes(packed))
			break
		}
		size := int(binary.BigEndian.Uint16(packed[:2]))
		packed = packed[2:]
		if size > len(packed) {
			size = len(packed)
		}
		if size > MaxRecordBytes+1 {
			size = MaxRecordBytes + 1
		}
		records = append(records, testCloneBytes(packed[:size]))
		packed = packed[size:]
	}
	return records
}

func FuzzKnownState(f *testing.F) {
	keys := newTestKeys(f)
	grant := testGrant(f, keys.controllerPrivate, keys.devicePrivate,
		CapMessage|CapPaymentRequest, testNonce(31_001))
	grantID := testGrantID(grant)
	second := testGrant(f, keys.controllerPrivate, keys.secondPrivate,
		CapAssessment, testNonce(31_002))
	secondID := testGrantID(second)
	revokeGrant := testRevokeGrant(f, keys.controllerPrivate, grantID)
	revokeDevice := testRevokeDevice(f, keys.controllerPrivate, keys.device)
	foreign := testGrant(f, keys.secondPrivate, keys.replacementPrivate,
		CapProfileUpdate, testNonce(31_003))
	foreignRevokeGrant := testRevokeGrant(f, keys.secondPrivate, grantID)
	foreignRevokeDevice := testRevokeDevice(f, keys.secondPrivate, keys.device)
	badSignature := testCloneBytes(grant)
	badSignature[len(badSignature)-1] ^= 1

	for _, seed := range [][]byte{
		packFuzzRecords(grant),
		packFuzzRecords(revokeGrant),
		packFuzzRecords(revokeDevice),
		packFuzzRecords(grant, revokeGrant, second),
		packFuzzRecords(revokeDevice, grant),
		packFuzzRecords(foreign),
		packFuzzRecords(foreignRevokeGrant),
		packFuzzRecords(foreignRevokeDevice),
		packFuzzRecords(grant, foreignRevokeGrant),
		packFuzzRecords(grant, foreignRevokeDevice),
		packFuzzRecords(badSignature, append(testCloneBytes(second), 0)),
		nil,
	} {
		f.Add(seed)
	}

	account, err := AccountIDFor(keys.controller)
	if err != nil {
		f.Fatalf("AccountIDFor: %v", err)
	}
	f.Fuzz(func(t *testing.T, packed []byte) {
		left := requireKnownState(t, keys.controller)
		right := requireKnownState(t, keys.controller)
		probes := []knownProbe{
			{grant: grantID, device: keys.device, required: CapMessage},
			{grant: grantID, device: keys.device, required: CapMessage | CapPaymentRequest},
			{grant: secondID, device: keys.second, required: CapAssessment},
		}

		for attempt, candidate := range unpackFuzzRecords(packed) {
			record, verifyErr := VerifyRecord(candidate)
			if verifyErr == nil && record.Kind() == Grant {
				probes = append(probes, knownProbe{
					grant: record.GrantID(), device: record.DeviceKey(), required: record.Capabilities(),
				})
			}
			beforeLeft := viewKnown(left, probes)
			beforeRight := viewKnown(right, probes)
			if !equalKnownView(beforeLeft, beforeRight) {
				t.Fatalf("attempt %d: states diverged before Apply", attempt)
			}

			leftInput := testCloneBytes(candidate)
			rightInput := testCloneBytes(candidate)
			leftErr := left.Apply(leftInput)
			rightErr := right.Apply(rightInput)
			if (leftErr == nil) != (rightErr == nil) {
				t.Fatalf("attempt %d: Apply was nondeterministic: left=%v right=%v", attempt, leftErr, rightErr)
			}

			isLocalValid := verifyErr == nil && record.AccountID() == account
			if isLocalValid && leftErr != nil {
				t.Fatalf("attempt %d: valid same-account record failed: %v", attempt, leftErr)
			}
			if !isLocalValid && leftErr == nil {
				t.Fatalf("attempt %d: invalid or foreign record applied", attempt)
			}

			afterLeft := viewKnown(left, probes)
			afterRight := viewKnown(right, probes)
			if !equalKnownView(afterLeft, afterRight) {
				t.Fatalf("attempt %d: identical inputs produced different state", attempt)
			}
			if !isLocalValid && !equalKnownView(beforeLeft, afterLeft) {
				t.Fatalf("attempt %d: invalid or foreign input mutated state", attempt)
			}

			clear(leftInput)
			if got := viewKnown(left, probes); !equalKnownView(got, afterRight) {
				t.Fatalf("attempt %d: input mutation changed state", attempt)
			}
		}
	})
}
