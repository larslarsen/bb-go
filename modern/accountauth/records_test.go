package accountauth

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

const (
	testAccountDomain      = "bitbook/accountauth/v1/account\x00"
	testGrantRootDomain    = "bitbook/accountauth/v1/grant/root\x00"
	testGrantDeviceDomain  = "bitbook/accountauth/v1/grant/device\x00"
	testRevokeGrantDomain  = "bitbook/accountauth/v1/revoke-grant\x00"
	testRevokeDeviceDomain = "bitbook/accountauth/v1/revoke-device\x00"
	testGrantIDDomain      = "bitbook/accountauth/v1/grant-id\x00"
)

type testKeys struct {
	controllerPrivate  ed25519.PrivateKey
	controller         [32]byte
	devicePrivate      ed25519.PrivateKey
	device             [32]byte
	secondPrivate      ed25519.PrivateKey
	second             [32]byte
	replacementPrivate ed25519.PrivateKey
	replacement        [32]byte
}

func mustHex(t testing.TB, value string) []byte {
	t.Helper()
	b, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("decode fixture hex: %v", err)
	}
	return b
}

func privateKeyFromSeedHex(t testing.TB, seedHex string) ed25519.PrivateKey {
	t.Helper()
	seed := mustHex(t, seedHex)
	if len(seed) != ed25519.SeedSize {
		t.Fatalf("seed length = %d, want %d", len(seed), ed25519.SeedSize)
	}
	return ed25519.NewKeyFromSeed(seed)
}

func testPublicArray(private ed25519.PrivateKey) [32]byte {
	var public [32]byte
	copy(public[:], private[32:])
	return public
}

func newTestKeys(t testing.TB) testKeys {
	t.Helper()
	controller := privateKeyFromSeedHex(t, "9d61b19deffd5a60ba844af492ec2cc4"+
		"4449c5697b326919703bac031cae7f60")
	device := privateKeyFromSeedHex(t, "4ccd089b28ff96da9db6c346ec114e0f"+
		"5b8a319f35aba624da8cf6ed4fb8a6fb")
	second := privateKeyFromSeedHex(t, "c5aa8df43f9f837bedb7442f31dcb7b1"+
		"66d38535076f094b85ce3a2e0b4458f7")
	replacement := privateKeyFromSeedHex(t, "000102030405060708090a0b0c0d0e0f"+
		"101112131415161718191a1b1c1d1e1f")
	return testKeys{
		controllerPrivate:  controller,
		controller:         testPublicArray(controller),
		devicePrivate:      device,
		device:             testPublicArray(device),
		secondPrivate:      second,
		second:             testPublicArray(second),
		replacementPrivate: replacement,
		replacement:        testPublicArray(replacement),
	}
}

func testNonce(value uint64) [32]byte {
	var nonce [32]byte
	binary.BigEndian.PutUint64(nonce[24:], value)
	return nonce
}

func testGrantPayload(controller, device [32]byte, capabilities Capability, nonce [32]byte) []byte {
	payload := make([]byte, 106)
	payload[0] = 1
	payload[1] = 1
	copy(payload[2:34], controller[:])
	copy(payload[34:66], device[:])
	binary.BigEndian.PutUint64(payload[66:74], uint64(capabilities))
	copy(payload[74:106], nonce[:])
	return payload
}

func testSignedGrantPayload(payload []byte, controllerPrivate, devicePrivate ed25519.PrivateKey) []byte {
	record := append([]byte(nil), payload...)
	record = append(record, ed25519.Sign(controllerPrivate, append([]byte(testGrantRootDomain), payload...))...)
	record = append(record, ed25519.Sign(devicePrivate, append([]byte(testGrantDeviceDomain), payload...))...)
	return record
}

func testGrant(t testing.TB, controllerPrivate, devicePrivate ed25519.PrivateKey, capabilities Capability, nonce [32]byte) []byte {
	t.Helper()
	return testSignedGrantPayload(
		testGrantPayload(testPublicArray(controllerPrivate), testPublicArray(devicePrivate), capabilities, nonce),
		controllerPrivate,
		devicePrivate,
	)
}

func testRevocationPayload(kind byte, controller, target [32]byte) []byte {
	payload := make([]byte, 66)
	payload[0] = 1
	payload[1] = kind
	copy(payload[2:34], controller[:])
	copy(payload[34:66], target[:])
	return payload
}

func testSignedRevocationPayload(payload []byte, domain string, controllerPrivate ed25519.PrivateKey) []byte {
	record := append([]byte(nil), payload...)
	record = append(record, ed25519.Sign(controllerPrivate, append([]byte(domain), payload...))...)
	return record
}

func testRevokeGrant(t testing.TB, controllerPrivate ed25519.PrivateKey, grant GrantID) []byte {
	t.Helper()
	return testSignedRevocationPayload(
		testRevocationPayload(2, testPublicArray(controllerPrivate), [32]byte(grant)),
		testRevokeGrantDomain,
		controllerPrivate,
	)
}

func testRevokeDevice(t testing.TB, controllerPrivate ed25519.PrivateKey, device [32]byte) []byte {
	t.Helper()
	return testSignedRevocationPayload(
		testRevocationPayload(3, testPublicArray(controllerPrivate), device),
		testRevokeDeviceDomain,
		controllerPrivate,
	)
}

func testGrantID(raw []byte) GrantID {
	preimage := append([]byte(testGrantIDDomain), raw[:106]...)
	return GrantID(sha256.Sum256(preimage))
}

func testCloneBytes(raw []byte) []byte {
	return append([]byte(nil), raw...)
}

func assertZeroRecord(t testing.TB, record Record) {
	t.Helper()
	if record.Kind() != 0 {
		t.Errorf("zero Record Kind = %d", record.Kind())
	}
	if record.AccountID() != (AccountID{}) {
		t.Errorf("zero Record AccountID = %x", record.AccountID())
	}
	if record.GrantID() != (GrantID{}) {
		t.Errorf("zero Record GrantID = %x", record.GrantID())
	}
	if record.DeviceKey() != ([32]byte{}) {
		t.Errorf("zero Record DeviceKey = %x", record.DeviceKey())
	}
	if record.Capabilities() != 0 {
		t.Errorf("zero Record Capabilities = %d", record.Capabilities())
	}
	if record.Bytes() != nil {
		t.Errorf("zero Record Bytes = %x, want nil", record.Bytes())
	}
}

func requireInvalidRecord(t testing.TB, raw []byte) {
	t.Helper()
	record, err := VerifyRecord(raw)
	if err == nil {
		t.Fatalf("VerifyRecord(%x) unexpectedly succeeded", raw)
	}
	assertZeroRecord(t, record)
}

// RFC 8032 section 7.1, Ed25519 TEST 1:
// https://www.rfc-editor.org/rfc/rfc8032#section-7.1
func TestRFC8032Fixture(t *testing.T) {
	private := privateKeyFromSeedHex(t, "9d61b19deffd5a60ba844af492ec2cc4"+
		"4449c5697b326919703bac031cae7f60")
	wantPublic := mustHex(t, "d75a980182b10ab7d54bfed3c964073a"+
		"0ee172f3daa62325af021a68f707511a")
	if got := private.Public().(ed25519.PublicKey); !bytes.Equal(got, wantPublic) {
		t.Fatalf("public key = %x, want %x", got, wantPublic)
	}
	wantSignature := mustHex(t, "e5564300c360ac729086e2cc806e828a"+
		"84877f1eb8e5d974d873e06522490155"+
		"5fb8821590a33bacc61e39701cf9b46b"+
		"d25bf5f0595bbe24655141438e7a100b")
	gotSignature := ed25519.Sign(private, nil)
	if !bytes.Equal(gotSignature, wantSignature) {
		t.Fatalf("signature = %x, want %x", gotSignature, wantSignature)
	}
	if !ed25519.Verify(ed25519.PublicKey(wantPublic), nil, wantSignature) {
		t.Fatal("RFC 8032 signature did not verify")
	}
}

func TestLiteralPayloadAndIndependentHashes(t *testing.T) {
	keys := newTestKeys(t)
	var nonce [32]byte
	for i := range nonce {
		nonce[i] = byte(i)
	}

	payload := testGrantPayload(keys.controller, keys.device, CapMessage|CapProfileUpdate, nonce)
	wantPayload := mustHex(t,
		"0101"+
			"d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a"+
			"3d4017c3e843895a92b70aa74d1b7ebc9c982ccf2ec4968cc0cd55f12af4660c"+
			"0000000000000011"+
			"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	if !bytes.Equal(payload, wantPayload) {
		t.Fatalf("payload = %x, want literal %x", payload, wantPayload)
	}

	// These preimages are specified independently of the test encoder and production code.
	accountPreimage := append([]byte("bitbook/accountauth/v1/account\x00"), mustHex(t,
		"d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a")...)
	wantAccount := AccountID(sha256.Sum256(accountPreimage))
	gotAccount, err := AccountIDFor(keys.controller)
	if err != nil {
		t.Fatalf("AccountIDFor: %v", err)
	}
	if gotAccount != wantAccount {
		t.Fatalf("AccountID = %x, want %x", gotAccount, wantAccount)
	}

	grantPreimage := append([]byte("bitbook/accountauth/v1/grant-id\x00"), wantPayload...)
	wantGrant := GrantID(sha256.Sum256(grantPreimage))
	record, err := VerifyRecord(testSignedGrantPayload(wantPayload, keys.controllerPrivate, keys.devicePrivate))
	if err != nil {
		t.Fatalf("VerifyRecord: %v", err)
	}
	if record.GrantID() != wantGrant {
		t.Fatalf("GrantID = %x, want %x", record.GrantID(), wantGrant)
	}
}

func TestVerifyRecordValidKinds(t *testing.T) {
	keys := newTestKeys(t)
	grantRaw := testGrant(t, keys.controllerPrivate, keys.devicePrivate, CapMessage|CapAssessment, testNonce(1))
	grantID := testGrantID(grantRaw)
	wantAccount, err := AccountIDFor(keys.controller)
	if err != nil {
		t.Fatalf("AccountIDFor: %v", err)
	}

	grant, err := VerifyRecord(grantRaw)
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	if grant.Kind() != Grant || grant.AccountID() != wantAccount || grant.GrantID() != grantID ||
		grant.DeviceKey() != keys.device || grant.Capabilities() != CapMessage|CapAssessment {
		t.Fatalf("unexpected grant accessors: kind=%d account=%x grant=%x device=%x capabilities=%d",
			grant.Kind(), grant.AccountID(), grant.GrantID(), grant.DeviceKey(), grant.Capabilities())
	}

	revokeGrantRaw := testRevokeGrant(t, keys.controllerPrivate, grantID)
	revokeGrant, err := VerifyRecord(revokeGrantRaw)
	if err != nil {
		t.Fatalf("revoke grant: %v", err)
	}
	if revokeGrant.Kind() != RevokeGrant || revokeGrant.AccountID() != wantAccount ||
		revokeGrant.GrantID() != grantID || revokeGrant.DeviceKey() != ([32]byte{}) || revokeGrant.Capabilities() != 0 {
		t.Fatalf("unexpected grant revocation accessors: kind=%d account=%x grant=%x device=%x capabilities=%d",
			revokeGrant.Kind(), revokeGrant.AccountID(), revokeGrant.GrantID(), revokeGrant.DeviceKey(), revokeGrant.Capabilities())
	}

	revokeDeviceRaw := testRevokeDevice(t, keys.controllerPrivate, keys.device)
	revokeDevice, err := VerifyRecord(revokeDeviceRaw)
	if err != nil {
		t.Fatalf("revoke device: %v", err)
	}
	if revokeDevice.Kind() != RevokeDevice || revokeDevice.AccountID() != wantAccount ||
		revokeDevice.GrantID() != (GrantID{}) || revokeDevice.DeviceKey() != keys.device || revokeDevice.Capabilities() != 0 {
		t.Fatalf("unexpected device revocation accessors: kind=%d account=%x grant=%x device=%x capabilities=%d",
			revokeDevice.Kind(), revokeDevice.AccountID(), revokeDevice.GrantID(), revokeDevice.DeviceKey(), revokeDevice.Capabilities())
	}

	if len(grantRaw) != 234 || len(revokeGrantRaw) != 130 || len(revokeDeviceRaw) != 130 {
		t.Fatalf("record lengths = grant %d, revoke-grant %d, revoke-device %d", len(grantRaw), len(revokeGrantRaw), len(revokeDeviceRaw))
	}
	if MaxRecordBytes != 234 {
		t.Fatalf("MaxRecordBytes = %d, want 234", MaxRecordBytes)
	}
	if Grant != 1 || RevokeGrant != 2 || RevokeDevice != 3 {
		t.Fatalf("kind values = %d, %d, %d", Grant, RevokeGrant, RevokeDevice)
	}
}

func TestVerifyRecordRejectsSignatureAndBindingMutations(t *testing.T) {
	keys := newTestKeys(t)
	valid := testGrant(t, keys.controllerPrivate, keys.devicePrivate, CapMessage|CapPaymentRequest, testNonce(7))

	mutate := func(offset int) []byte {
		raw := testCloneBytes(valid)
		raw[offset] ^= 0x80
		return raw
	}
	swapped := testCloneBytes(valid)
	copy(swapped[106:170], valid[170:234])
	copy(swapped[170:234], valid[106:170])
	missingControllerProof := testCloneBytes(valid)
	clear(missingControllerProof[106:170])
	missingDeviceProof := testCloneBytes(valid)
	clear(missingDeviceProof[170:234])

	payload := testCloneBytes(valid[:106])
	rootDigest := sha256.Sum256(append([]byte(testGrantRootDomain), payload...))
	deviceDigest := sha256.Sum256(append([]byte(testGrantDeviceDomain), payload...))
	digestSigned := append([]byte(nil), payload...)
	digestSigned = append(digestSigned, ed25519.Sign(keys.controllerPrivate, rootDigest[:])...)
	digestSigned = append(digestSigned, ed25519.Sign(keys.devicePrivate, deviceDigest[:])...)
	changedPayload := testCloneBytes(valid[:106])
	binary.BigEndian.PutUint64(changedPayload[66:74], uint64(CapMessage|CapAssessment))
	staleCapabilityProofs := append(testCloneBytes(changedPayload), valid[106:]...)
	controllerOnly := testCloneBytes(changedPayload)
	controllerOnly = append(controllerOnly,
		ed25519.Sign(keys.controllerPrivate, append([]byte(testGrantRootDomain), changedPayload...))...)
	controllerOnly = append(controllerOnly, valid[170:234]...)
	deviceOnly := append(testCloneBytes(changedPayload), valid[106:170]...)
	deviceOnly = append(deviceOnly,
		ed25519.Sign(keys.devicePrivate, append([]byte(testGrantDeviceDomain), changedPayload...))...)

	cases := map[string][]byte{
		"wrong controller signature":                    mutate(106),
		"wrong device signature":                        mutate(170),
		"substituted controller":                        mutate(2),
		"substituted device":                            mutate(34),
		"valid capabilities with stale proofs":          staleCapabilityProofs,
		"valid capabilities with controller proof only": controllerOnly,
		"valid capabilities with device proof only":     deviceOnly,
		"substituted nonce":                             mutate(105),
		"swapped signatures":                            swapped,
		"missing controller proof":                      missingControllerProof,
		"missing device proof":                          missingDeviceProof,
		"signed digest substitution":                    digestSigned,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			requireInvalidRecord(t, raw)
		})
	}

	renewed := testSignedGrantPayload(changedPayload, keys.controllerPrivate, keys.devicePrivate)
	record, err := VerifyRecord(renewed)
	if err != nil {
		t.Fatalf("valid capability change with both proofs renewed: %v", err)
	}
	if record.Capabilities() != CapMessage|CapAssessment {
		t.Fatalf("renewed capability change = %d, want %d", record.Capabilities(), CapMessage|CapAssessment)
	}
}

func TestVerifyRecordRejectsRevocationAuthenticationMutations(t *testing.T) {
	keys := newTestKeys(t)
	firstGrant := testGrant(t, keys.controllerPrivate, keys.devicePrivate, CapMessage, testNonce(8))
	secondGrant := testGrant(t, keys.controllerPrivate, keys.secondPrivate, CapMessage, testNonce(9))

	cases := []struct {
		name          string
		kind          Kind
		raw           []byte
		changedTarget [32]byte
		domain        string
		otherDomain   string
	}{
		{
			name:          "revoke grant",
			kind:          RevokeGrant,
			raw:           testRevokeGrant(t, keys.controllerPrivate, testGrantID(firstGrant)),
			changedTarget: [32]byte(testGrantID(secondGrant)),
			domain:        testRevokeGrantDomain,
			otherDomain:   testRevokeDeviceDomain,
		},
		{
			name:          "revoke device",
			kind:          RevokeDevice,
			raw:           testRevokeDevice(t, keys.controllerPrivate, keys.device),
			changedTarget: keys.second,
			domain:        testRevokeDeviceDomain,
			otherDomain:   testRevokeGrantDomain,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record, err := VerifyRecord(tc.raw)
			if err != nil {
				t.Fatalf("positive revocation verification: %v", err)
			}
			if record.Kind() != tc.kind {
				t.Fatalf("positive revocation kind = %d, want %d", record.Kind(), tc.kind)
			}

			corrupted := testCloneBytes(tc.raw)
			corrupted[66] ^= 1
			zeroSignature := testCloneBytes(tc.raw)
			clear(zeroSignature[66:130])
			deviceForged := testSignedRevocationPayload(tc.raw[:66], tc.domain, keys.devicePrivate)
			changedTarget := testCloneBytes(tc.raw)
			copy(changedTarget[34:66], tc.changedTarget[:])
			otherDomain := testSignedRevocationPayload(tc.raw[:66], tc.otherDomain, keys.controllerPrivate)

			for name, raw := range map[string][]byte{
				"corrupted signature":     corrupted,
				"zero signature":          zeroSignature,
				"device-forged signature": deviceForged,
				"changed valid target":    changedTarget,
				"other revocation domain": otherDomain,
			} {
				t.Run(name, func(t *testing.T) {
					requireInvalidRecord(t, raw)
				})
			}
		})
	}
}

func TestVerifyRecordRejectsMalformedSignedPayloads(t *testing.T) {
	keys := newTestKeys(t)
	validPayload := testGrantPayload(keys.controller, keys.device, CapMessage, testNonce(9))

	grantCase := func(change func([]byte)) []byte {
		payload := testCloneBytes(validPayload)
		change(payload)
		return testSignedGrantPayload(payload, keys.controllerPrivate, keys.devicePrivate)
	}
	rootEqualsDevice := testGrantPayload(keys.controller, keys.controller, CapMessage, testNonce(10))
	rootEqualsDeviceRaw := testSignedGrantPayload(rootEqualsDevice, keys.controllerPrivate, keys.controllerPrivate)

	zeroGrantTarget := testSignedRevocationPayload(
		testRevocationPayload(2, keys.controller, [32]byte{}), testRevokeGrantDomain, keys.controllerPrivate)
	zeroDeviceTarget := testSignedRevocationPayload(
		testRevocationPayload(3, keys.controller, [32]byte{}), testRevokeDeviceDomain, keys.controllerPrivate)
	crossKindDomain := testSignedRevocationPayload(
		testRevocationPayload(2, keys.controller, [32]byte(testGrantID(testGrant(t, keys.controllerPrivate, keys.devicePrivate, CapMessage, testNonce(11))))),
		testRevokeDeviceDomain, keys.controllerPrivate)

	cases := map[string][]byte{
		"unknown version":              grantCase(func(payload []byte) { payload[0] = 2 }),
		"unknown kind":                 grantCase(func(payload []byte) { payload[1] = 9 }),
		"zero controller":              grantCase(func(payload []byte) { clear(payload[2:34]) }),
		"zero device":                  grantCase(func(payload []byte) { clear(payload[34:66]) }),
		"controller equals device":     rootEqualsDeviceRaw,
		"zero capabilities":            grantCase(func(payload []byte) { clear(payload[66:74]) }),
		"unknown capability":           grantCase(func(payload []byte) { binary.BigEndian.PutUint64(payload[66:74], 32) }),
		"known and unknown capability": grantCase(func(payload []byte) { binary.BigEndian.PutUint64(payload[66:74], 33) }),
		"zero nonce":                   grantCase(func(payload []byte) { clear(payload[74:106]) }),
		"zero grant target":            zeroGrantTarget,
		"zero device target":           zeroDeviceTarget,
		"cross-kind domain":            crossKindDomain,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			requireInvalidRecord(t, raw)
		})
	}
}

func TestVerifyRecordCapabilityBoundary(t *testing.T) {
	if CapMessage != Capability(1) || CapPaymentRequest != Capability(2) ||
		CapPolicyWrite != Capability(4) || CapAssessment != Capability(8) ||
		CapProfileUpdate != Capability(16) {
		t.Fatalf("capability wire bits = message:%d payment:%d policy:%d assessment:%d profile:%d",
			CapMessage, CapPaymentRequest, CapPolicyWrite, CapAssessment, CapProfileUpdate)
	}
	keys := newTestKeys(t)
	valid := testGrant(t, keys.controllerPrivate, keys.devicePrivate, Capability(31), testNonce(20))
	record, err := VerifyRecord(valid)
	if err != nil {
		t.Fatalf("capability mask 31: %v", err)
	}
	if record.Capabilities() != Capability(31) {
		t.Fatalf("Capabilities = %d, want 31", record.Capabilities())
	}

	for _, invalid := range []Capability{0, 32, 63, 1 << 63} {
		raw := testGrant(t, keys.controllerPrivate, keys.devicePrivate, invalid, testNonce(uint64(invalid)+21))
		requireInvalidRecord(t, raw)
	}
}

func TestVerifyRecordLengthBoundaries(t *testing.T) {
	keys := newTestKeys(t)
	grant := testGrant(t, keys.controllerPrivate, keys.devicePrivate, CapMessage, testNonce(30))
	revocation := testRevokeGrant(t, keys.controllerPrivate, testGrantID(grant))

	cases := []struct {
		name  string
		raw   []byte
		valid bool
	}{
		{name: "nil", raw: nil},
		{name: "empty", raw: []byte{}},
		{name: "129", raw: testCloneBytes(revocation[:129])},
		{name: "130", raw: testCloneBytes(revocation), valid: true},
		{name: "131", raw: append(testCloneBytes(revocation), 0)},
		{name: "233", raw: testCloneBytes(grant[:233])},
		{name: "234", raw: testCloneBytes(grant), valid: true},
		{name: "235", raw: append(testCloneBytes(grant), 0)},
		{name: "oversized", raw: make([]byte, MaxRecordBytes+100)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record, err := VerifyRecord(tc.raw)
			if tc.valid {
				if err != nil {
					t.Fatalf("VerifyRecord: %v", err)
				}
				if !bytes.Equal(record.Bytes(), tc.raw) {
					t.Fatalf("record bytes differ from input")
				}
				return
			}
			if err == nil {
				t.Fatal("VerifyRecord unexpectedly succeeded")
			}
			assertZeroRecord(t, record)
		})
	}
}

func TestAccountIDRejectsZeroController(t *testing.T) {
	_, err := AccountIDFor([32]byte{})
	if err == nil {
		t.Fatal("AccountIDFor(zero) unexpectedly succeeded")
	}
}

func TestRecordBytesAreOwned(t *testing.T) {
	keys := newTestKeys(t)
	raw := testGrant(t, keys.controllerPrivate, keys.devicePrivate, CapMessage|CapPolicyWrite, testNonce(40))
	want := testCloneBytes(raw)
	record, err := VerifyRecord(raw)
	if err != nil {
		t.Fatalf("VerifyRecord: %v", err)
	}
	wantGrantID := record.GrantID()
	wantDevice := record.DeviceKey()
	wantCapabilities := record.Capabilities()

	clear(raw)
	if got := record.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("input mutation changed Record.Bytes: %x", got)
	}
	first := record.Bytes()
	first[0] ^= 0xff
	if got := record.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("returned-copy mutation changed Record.Bytes: %x", got)
	}
	if record.GrantID() != wantGrantID || record.DeviceKey() != wantDevice || record.Capabilities() != wantCapabilities {
		t.Fatal("byte mutation changed record accessors")
	}
}
