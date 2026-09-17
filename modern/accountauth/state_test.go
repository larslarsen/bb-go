package accountauth

import (
	"sync"
	"testing"
)

func requireApply(t testing.TB, state *KnownState, raw []byte) {
	t.Helper()
	if err := state.Apply(raw); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}

func requireKnownState(t testing.TB, controller [32]byte) *KnownState {
	t.Helper()
	state, err := NewKnownState(controller)
	if err != nil {
		t.Fatalf("NewKnownState: %v", err)
	}
	if state == nil {
		t.Fatal("NewKnownState returned nil without error")
	}
	return state
}

func TestKnownStateRevocation(t *testing.T) {
	keys := newTestKeys(t)
	state := requireKnownState(t, keys.controller)

	first := testGrant(t, keys.controllerPrivate, keys.devicePrivate, CapMessage, testNonce(101))
	secondForDevice := testGrant(t, keys.controllerPrivate, keys.devicePrivate, CapPaymentRequest, testNonce(102))
	otherDevice := testGrant(t, keys.controllerPrivate, keys.secondPrivate, CapAssessment, testNonce(103))
	firstID := testGrantID(first)
	secondID := testGrantID(secondForDevice)
	otherID := testGrantID(otherDevice)

	requireApply(t, state, first)
	requireApply(t, state, secondForDevice)
	requireApply(t, state, otherDevice)
	if !state.AuthorizesKnown(firstID, keys.device, CapMessage) ||
		!state.AuthorizesKnown(secondID, keys.device, CapPaymentRequest) ||
		!state.AuthorizesKnown(otherID, keys.second, CapAssessment) {
		t.Fatal("valid grants did not initially authorize both devices")
	}

	revokeFirst := testRevokeGrant(t, keys.controllerPrivate, firstID)
	requireApply(t, state, revokeFirst)
	if state.AuthorizesKnown(firstID, keys.device, CapMessage) {
		t.Fatal("revoked grant remained authorized")
	}
	if !state.AuthorizesKnown(secondID, keys.device, CapPaymentRequest) {
		t.Fatal("grant revocation denied another grant for the same device")
	}
	if !state.AuthorizesKnown(otherID, keys.second, CapAssessment) {
		t.Fatal("grant revocation denied the second device")
	}
	requireApply(t, state, first)
	if state.AuthorizesKnown(firstID, keys.device, CapMessage) {
		t.Fatal("grant replay removed its revocation tombstone")
	}

	revokeDevice := testRevokeDevice(t, keys.controllerPrivate, keys.device)
	requireApply(t, state, revokeDevice)
	if state.AuthorizesKnown(secondID, keys.device, CapPaymentRequest) {
		t.Fatal("device revocation left another grant for the device authorized")
	}
	requireApply(t, state, secondForDevice)
	if state.AuthorizesKnown(secondID, keys.device, CapPaymentRequest) {
		t.Fatal("grant replay removed a device revocation tombstone")
	}

	regrant := testGrant(t, keys.controllerPrivate, keys.devicePrivate, CapProfileUpdate, testNonce(104))
	regrantID := testGrantID(regrant)
	requireApply(t, state, regrant)
	if state.AuthorizesKnown(regrantID, keys.device, CapProfileUpdate) {
		t.Fatal("new grant reauthorized a revoked device key")
	}
	if !state.AuthorizesKnown(otherID, keys.second, CapAssessment) {
		t.Fatal("revoking one device denied the other device")
	}

	beforeGrant := requireKnownState(t, keys.controller)
	requireApply(t, beforeGrant, revokeDevice)
	requireApply(t, beforeGrant, secondForDevice)
	if beforeGrant.AuthorizesKnown(secondID, keys.device, CapPaymentRequest) {
		t.Fatal("revocation delivered before a grant did not win")
	}

	replacement := testGrant(t, keys.controllerPrivate, keys.replacementPrivate, CapMessage|CapPaymentRequest, testNonce(105))
	replacementID := testGrantID(replacement)
	requireApply(t, state, replacement)
	if !state.AuthorizesKnown(replacementID, keys.replacement, CapMessage|CapPaymentRequest) {
		t.Fatal("newly keyed replacement device was not authorized")
	}
}

func TestKnownStateCapabilities(t *testing.T) {
	keys := newTestKeys(t)
	state := requireKnownState(t, keys.controller)
	capabilities := []Capability{
		CapMessage,
		CapPaymentRequest,
		CapPolicyWrite,
		CapAssessment,
		CapProfileUpdate,
	}

	for i, capability := range capabilities {
		raw := testGrant(t, keys.controllerPrivate, keys.devicePrivate, capability, testNonce(uint64(200+i)))
		grantID := testGrantID(raw)
		requireApply(t, state, raw)
		if !state.AuthorizesKnown(grantID, keys.device, capability) {
			t.Errorf("single capability %d was not authorized", capability)
		}
		for _, other := range capabilities {
			if other != capability && state.AuthorizesKnown(grantID, keys.device, other) {
				t.Errorf("capability %d elevated to %d", capability, other)
			}
		}
		if state.AuthorizesKnown(grantID, keys.second, capability) {
			t.Errorf("capability %d authorized the wrong device", capability)
		}
	}

	all := CapMessage | CapPaymentRequest | CapPolicyWrite | CapAssessment | CapProfileUpdate
	combined := testGrant(t, keys.controllerPrivate, keys.devicePrivate, all, testNonce(210))
	combinedID := testGrantID(combined)
	requireApply(t, state, combined)
	if !state.AuthorizesKnown(combinedID, keys.device, CapMessage|CapAssessment|CapProfileUpdate) {
		t.Fatal("one grant did not satisfy a combined subset request")
	}
	if !state.AuthorizesKnown(combinedID, keys.device, all) {
		t.Fatal("one grant did not satisfy its complete capability set")
	}

	message := testGrant(t, keys.controllerPrivate, keys.secondPrivate, CapMessage, testNonce(211))
	payment := testGrant(t, keys.controllerPrivate, keys.secondPrivate, CapPaymentRequest, testNonce(212))
	messageID := testGrantID(message)
	paymentID := testGrantID(payment)
	requireApply(t, state, message)
	requireApply(t, state, payment)
	if !state.AuthorizesKnown(messageID, keys.second, CapMessage) ||
		!state.AuthorizesKnown(paymentID, keys.second, CapPaymentRequest) {
		t.Fatal("separate grants did not independently authorize their capabilities")
	}
	if state.AuthorizesKnown(messageID, keys.second, CapMessage|CapPaymentRequest) ||
		state.AuthorizesKnown(paymentID, keys.second, CapMessage|CapPaymentRequest) {
		t.Fatal("separate grants were unioned for a compound request")
	}

	if state.AuthorizesKnown(combinedID, keys.device, 0) {
		t.Fatal("zero requested capability authorized")
	}
	if state.AuthorizesKnown(combinedID, keys.device, Capability(32)) {
		t.Fatal("unknown requested capability authorized")
	}
	if state.AuthorizesKnown(combinedID, keys.device, CapMessage|Capability(32)) {
		t.Fatal("mixed known and unknown requested capabilities authorized")
	}
	if state.AuthorizesKnown(GrantID{1}, keys.device, CapMessage) {
		t.Fatal("unknown grant authorized")
	}
}

func TestKnownStateCapabilityReductionRequiresRevocation(t *testing.T) {
	keys := newTestKeys(t)
	state := requireKnownState(t, keys.controller)
	broad := testGrant(t, keys.controllerPrivate, keys.devicePrivate, CapMessage|CapPaymentRequest|CapPolicyWrite, testNonce(300))
	narrow := testGrant(t, keys.controllerPrivate, keys.devicePrivate, CapMessage, testNonce(301))
	broadID := testGrantID(broad)
	narrowID := testGrantID(narrow)
	requireApply(t, state, broad)
	requireApply(t, state, narrow)

	if !state.AuthorizesKnown(broadID, keys.device, CapPaymentRequest|CapPolicyWrite) {
		t.Fatal("older broad grant was implicitly reduced by a newer narrow grant")
	}
	if state.AuthorizesKnown(narrowID, keys.device, CapPaymentRequest) {
		t.Fatal("narrow grant acquired a removed capability")
	}

	requireApply(t, state, testRevokeGrant(t, keys.controllerPrivate, broadID))
	if state.AuthorizesKnown(broadID, keys.device, CapPaymentRequest) {
		t.Fatal("explicitly revoked broad grant remained active")
	}
	if !state.AuthorizesKnown(narrowID, keys.device, CapMessage) {
		t.Fatal("revoking the broad grant disabled the narrow grant")
	}
}

func TestKnownStateIdempotenceAndPermutations(t *testing.T) {
	keys := newTestKeys(t)
	first := testGrant(t, keys.controllerPrivate, keys.devicePrivate, CapMessage, testNonce(400))
	second := testGrant(t, keys.controllerPrivate, keys.devicePrivate, CapPaymentRequest, testNonce(401))
	other := testGrant(t, keys.controllerPrivate, keys.secondPrivate, CapAssessment, testNonce(402))
	firstID := testGrantID(first)
	secondID := testGrantID(second)
	otherID := testGrantID(other)
	facts := [][]byte{
		first,
		second,
		other,
		testRevokeGrant(t, keys.controllerPrivate, firstID),
		testRevokeDevice(t, keys.controllerPrivate, keys.second),
	}

	var orders [][][]byte
	var permute func(int)
	permute = func(index int) {
		if index == len(facts) {
			order := append([][]byte(nil), facts...)
			orders = append(orders, order)
			return
		}
		for i := index; i < len(facts); i++ {
			facts[index], facts[i] = facts[i], facts[index]
			permute(index + 1)
			facts[index], facts[i] = facts[i], facts[index]
		}
	}
	permute(0)
	if len(orders) != 120 {
		t.Fatalf("permutation count = %d, want 120", len(orders))
	}

	for i, order := range orders {
		state := requireKnownState(t, keys.controller)
		for _, fact := range order {
			requireApply(t, state, fact)
			requireApply(t, state, fact)
		}
		if state.AuthorizesKnown(firstID, keys.device, CapMessage) {
			t.Fatalf("permutation %d authorized revoked first grant", i)
		}
		if !state.AuthorizesKnown(secondID, keys.device, CapPaymentRequest) {
			t.Fatalf("permutation %d denied unaffected second grant", i)
		}
		if state.AuthorizesKnown(otherID, keys.second, CapAssessment) {
			t.Fatalf("permutation %d authorized device-revoked grant", i)
		}
		if state.Saturated() {
			t.Fatalf("permutation %d unexpectedly saturated", i)
		}
	}
}

func TestKnownStateRejectsForeignAndInvalidWithoutMutation(t *testing.T) {
	keys := newTestKeys(t)
	state := requireKnownState(t, keys.controller)
	local := testGrant(t, keys.controllerPrivate, keys.devicePrivate, CapMessage, testNonce(500))
	secondLocal := testGrant(t, keys.controllerPrivate, keys.devicePrivate, CapPaymentRequest, testNonce(501))
	localID := testGrantID(local)
	secondLocalID := testGrantID(secondLocal)
	requireApply(t, state, local)
	requireApply(t, state, secondLocal)
	assertLocalAuthority := func(t testing.TB) {
		t.Helper()
		if !state.AuthorizesKnown(localID, keys.device, CapMessage) ||
			!state.AuthorizesKnown(secondLocalID, keys.device, CapPaymentRequest) {
			t.Fatal("rejected input mutated existing authorization")
		}
		if state.Saturated() {
			t.Fatal("rejected input saturated state")
		}
	}

	foreign := testGrant(t, keys.secondPrivate, keys.replacementPrivate, CapPaymentRequest, testNonce(502))
	if err := state.Apply(foreign); err == nil {
		t.Fatal("foreign-account grant unexpectedly applied")
	}
	assertLocalAuthority(t)

	foreignRevocations := []struct {
		name string
		raw  []byte
		kind Kind
	}{
		{
			name: "revoke grant",
			raw:  testRevokeGrant(t, keys.secondPrivate, localID),
			kind: RevokeGrant,
		},
		{
			name: "revoke device",
			raw:  testRevokeDevice(t, keys.secondPrivate, keys.device),
			kind: RevokeDevice,
		},
	}
	for _, tc := range foreignRevocations {
		t.Run("foreign "+tc.name, func(t *testing.T) {
			record, err := VerifyRecord(tc.raw)
			if err != nil {
				t.Fatalf("VerifyRecord valid foreign revocation: %v", err)
			}
			if record.Kind() != tc.kind {
				t.Fatalf("foreign revocation kind = %d, want %d", record.Kind(), tc.kind)
			}
			if err := state.Apply(tc.raw); err == nil {
				t.Fatal("foreign revocation unexpectedly applied")
			}
			assertLocalAuthority(t)
		})
	}

	forgedRevocations := []struct {
		name string
		raw  []byte
	}{
		{
			name: "revoke grant",
			raw: testSignedRevocationPayload(
				testRevocationPayload(2, keys.controller, [32]byte(localID)),
				testRevokeGrantDomain,
				keys.devicePrivate,
			),
		},
		{
			name: "revoke device",
			raw: testSignedRevocationPayload(
				testRevocationPayload(3, keys.controller, keys.device),
				testRevokeDeviceDomain,
				keys.devicePrivate,
			),
		},
	}
	for _, tc := range forgedRevocations {
		t.Run("forged local "+tc.name, func(t *testing.T) {
			requireInvalidRecord(t, tc.raw)
			if err := state.Apply(tc.raw); err == nil {
				t.Fatal("forged local revocation unexpectedly applied")
			}
			assertLocalAuthority(t)
		})
	}

	invalid := testCloneBytes(local)
	invalid[len(invalid)-1] ^= 1
	if err := state.Apply(invalid); err == nil {
		t.Fatal("invalid signature unexpectedly applied")
	}
	assertLocalAuthority(t)

	requireApply(t, state, testRevokeGrant(t, keys.controllerPrivate, localID))
	if state.AuthorizesKnown(localID, keys.device, CapMessage) {
		t.Fatal("valid local grant revocation did not remove authority")
	}
	if !state.AuthorizesKnown(secondLocalID, keys.device, CapPaymentRequest) {
		t.Fatal("local grant revocation removed another live grant")
	}
	requireApply(t, state, testRevokeDevice(t, keys.controllerPrivate, keys.device))
	if state.AuthorizesKnown(secondLocalID, keys.device, CapPaymentRequest) {
		t.Fatal("valid local device revocation did not remove authority")
	}
}

func TestKnownStateZeroAndNilReceivers(t *testing.T) {
	keys := newTestKeys(t)
	raw := testGrant(t, keys.controllerPrivate, keys.devicePrivate, CapMessage, testNonce(600))
	grantID := testGrantID(raw)

	var nilState *KnownState
	if err := nilState.Apply(raw); err == nil {
		t.Fatal("nil state Apply unexpectedly succeeded")
	}
	if nilState.AuthorizesKnown(grantID, keys.device, CapMessage) {
		t.Fatal("nil state authorized")
	}
	if nilState.Saturated() {
		t.Fatal("nil state reported saturation")
	}

	var zeroState KnownState
	if err := zeroState.Apply(raw); err == nil {
		t.Fatal("zero state Apply unexpectedly succeeded")
	}
	if zeroState.AuthorizesKnown(grantID, keys.device, CapMessage) {
		t.Fatal("zero state authorized")
	}
	if zeroState.Saturated() {
		t.Fatal("zero state reported saturation")
	}

	_, err := NewKnownState([32]byte{})
	if err == nil {
		t.Fatal("NewKnownState(zero) unexpectedly succeeded")
	}
}

func TestKnownStateOwnsAppliedInputAndReadsDoNotMutate(t *testing.T) {
	keys := newTestKeys(t)
	state := requireKnownState(t, keys.controller)
	raw := testGrant(t, keys.controllerPrivate, keys.devicePrivate, CapMessage|CapAssessment, testNonce(700))
	grantID := testGrantID(raw)
	requireApply(t, state, raw)
	clear(raw)
	if !state.AuthorizesKnown(grantID, keys.device, CapMessage|CapAssessment) {
		t.Fatal("input mutation changed stored authorization")
	}

	const readers = 8
	const readsPerReader = 500
	var wait sync.WaitGroup
	wait.Add(readers)
	for i := 0; i < readers; i++ {
		go func() {
			defer wait.Done()
			for j := 0; j < readsPerReader; j++ {
				if !state.AuthorizesKnown(grantID, keys.device, CapMessage) {
					t.Errorf("read %d unexpectedly denied", j)
					return
				}
				if state.Saturated() {
					t.Errorf("read %d unexpectedly saturated state", j)
					return
				}
			}
		}()
	}
	wait.Wait()
	if !state.AuthorizesKnown(grantID, keys.device, CapAssessment) || state.Saturated() {
		t.Fatal("reads mutated final state")
	}
}

func TestKnownStateRecordLimit(t *testing.T) {
	if MaxKnownRecords != 4096 {
		t.Fatalf("MaxKnownRecords = %d, want 4096", MaxKnownRecords)
	}
	keys := newTestKeys(t)
	state := requireKnownState(t, keys.controller)

	var first []byte
	var revoked []byte
	for i := 1; i <= 4093; i++ {
		raw := testGrant(t, keys.controllerPrivate, keys.devicePrivate, CapMessage, testNonce(uint64(10_000+i)))
		if i == 1 {
			first = raw
		}
		if i == 2 {
			revoked = raw
		}
		requireApply(t, state, raw)
	}
	firstID := testGrantID(first)
	revokedID := testGrantID(revoked)

	grantTombstone := testRevokeGrant(t, keys.controllerPrivate, revokedID)
	var absentDevice [32]byte
	absentDevice[0] = 0xd1
	absentDevice[31] = 0x01
	deviceTombstone := testRevokeDevice(t, keys.controllerPrivate, absentDevice)
	requireApply(t, state, grantTombstone)
	requireApply(t, state, deviceTombstone)
	if state.Saturated() {
		t.Fatal("state saturated at 4095 distinct records")
	}
	if !state.AuthorizesKnown(firstID, keys.device, CapMessage) {
		t.Fatal("state denied a grant at 4095 distinct records")
	}
	if state.AuthorizesKnown(revokedID, keys.device, CapMessage) {
		t.Fatal("retained grant was not denied by its separately counted tombstone")
	}

	last := testGrant(t, keys.controllerPrivate, keys.devicePrivate, CapPaymentRequest, testNonce(20_000))
	invalid := testCloneBytes(last)
	invalid[len(invalid)-1] ^= 1
	if err := state.Apply(invalid); err == nil {
		t.Fatal("invalid input just below capacity unexpectedly applied")
	}
	if state.Saturated() {
		t.Fatal("invalid input just below capacity saturated state")
	}
	foreign := testGrant(t, keys.secondPrivate, keys.replacementPrivate, CapMessage, testNonce(20_001))
	if err := state.Apply(foreign); err == nil {
		t.Fatal("foreign input just below capacity unexpectedly applied")
	}
	if state.Saturated() {
		t.Fatal("foreign input just below capacity saturated state")
	}

	requireApply(t, state, last)
	lastID := testGrantID(last)
	if state.Saturated() {
		t.Fatal("state saturated at exactly 4096 distinct records")
	}
	if !state.AuthorizesKnown(lastID, keys.device, CapPaymentRequest) {
		t.Fatal("last distinct valid fact did not fit at capacity")
	}
	requireApply(t, state, last)
	requireApply(t, state, grantTombstone)
	requireApply(t, state, deviceTombstone)
	if state.Saturated() {
		t.Fatal("grant or revocation replay at capacity saturated state")
	}
	assertCapacityAuthority := func(t testing.TB, input string) {
		t.Helper()
		if state.Saturated() {
			t.Fatalf("%s input at capacity saturated state", input)
		}
		if !state.AuthorizesKnown(firstID, keys.device, CapMessage) {
			t.Fatalf("%s input at capacity denied the first grant", input)
		}
		if !state.AuthorizesKnown(lastID, keys.device, CapPaymentRequest) {
			t.Fatalf("%s input at capacity denied the last grant", input)
		}
	}
	if err := state.Apply(invalid); err == nil {
		t.Fatal("invalid input at capacity unexpectedly applied")
	}
	assertCapacityAuthority(t, "invalid")
	if err := state.Apply(foreign); err == nil {
		t.Fatal("foreign input at capacity unexpectedly applied")
	}
	assertCapacityAuthority(t, "foreign")
	if !state.AuthorizesKnown(firstID, keys.device, CapMessage) {
		t.Fatal("state denied an existing grant before overflow")
	}

	var overflowTarget GrantID
	overflowTarget[0] = 0xa2
	overflowTarget[31] = 0x02
	overflow := testRevokeGrant(t, keys.controllerPrivate, overflowTarget)
	if err := state.Apply(overflow); err == nil {
		t.Fatal("4097th distinct valid record unexpectedly applied")
	}
	if !state.Saturated() {
		t.Fatal("4097th distinct valid record did not permanently saturate state")
	}
	if state.AuthorizesKnown(firstID, keys.device, CapMessage) {
		t.Fatal("saturated state did not fail closed")
	}

	_ = state.Apply(first)
	if !state.Saturated() || state.AuthorizesKnown(firstID, keys.device, CapMessage) {
		t.Fatal("stored-record replay cleared saturation")
	}
	newFact := testGrant(t, keys.controllerPrivate, keys.devicePrivate, CapAssessment, testNonce(20_002))
	if err := state.Apply(newFact); err == nil {
		t.Fatal("saturated state accepted a new fact")
	}
	if !state.Saturated() {
		t.Fatal("new fact cleared permanent saturation")
	}
}
