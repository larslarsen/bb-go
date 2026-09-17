package accountstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/larslarsen/bb-go/modern/accountauth"
)

func FuzzOpenSnapshot(f *testing.F) {
	keys := fixtureKeySet()
	grant := fixtureGrant(keys.controllerPrivate, keys.devicePrivate,
		accountauth.CapMessage|accountauth.CapAssessment, fixtureNonce(50_000))
	grantID := fixtureGrantID(grant)
	revokeGrant := fixtureRevokeGrant(keys.controllerPrivate, grantID)
	revokeDevice := fixtureRevokeDevice(keys.controllerPrivate, keys.device)
	foreignGrant := fixtureGrant(keys.foreignPrivate, keys.secondPrivate,
		accountauth.CapPaymentRequest, fixtureNonce(50_001))

	badChecksum := fixtureSnapshot(keys.controller, [][]byte{grant})
	badChecksum[len(badChecksum)-1] ^= 1
	badMagic := fixtureSnapshot(keys.controller, nil)
	copy(badMagic[:8], []byte("BBACST02"))
	fixtureRechecksum(badMagic)
	truncated := fixtureSnapshot(keys.controller, [][]byte{grant})
	truncated = truncated[:len(truncated)-1]
	trailing := fixtureSnapshot(keys.controller, [][]byte{grant})
	trailingBody := append(fixtureClone(trailing[:len(trailing)-sha256.Size]), 0)
	trailingDigest := sha256.Sum256(trailingBody)
	trailing = append(trailingBody, trailingDigest[:]...)
	invalidSignature := fixtureClone(grant)
	invalidSignature[len(invalidSignature)-1] ^= 1
	count4098 := fixtureSnapshotWithCount(keys.controller, MaxStoredRecords+1, nil)
	invalidLengthBody := make([]byte, 2+131)
	binary.BigEndian.PutUint16(invalidLengthBody[:2], 131)
	invalidLength := fixtureSnapshotWithBody(keys.controller, 1, invalidLengthBody)

	for _, seed := range [][]byte{
		fixtureSnapshot(keys.controller, nil),
		fixtureSnapshot(keys.controller, [][]byte{grant}),
		fixtureSnapshot(keys.controller, [][]byte{revokeGrant}),
		fixtureSnapshot(keys.controller, [][]byte{revokeDevice}),
		fixtureSnapshot(keys.controller, [][]byte{grant, revokeGrant}),
		badChecksum,
		badMagic,
		truncated,
		trailing,
		fixtureSnapshot(keys.controller, [][]byte{foreignGrant}),
		fixtureSnapshot(keys.controller, [][]byte{grant, grant}),
		fixtureSnapshot(keys.controller, [][]byte{invalidSignature}),
		invalidLength,
		count4098,
		nil,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, fuzzed []byte) {
		inputDigest := sha256.Sum256(fuzzed)
		candidate := fuzzed
		if len(candidate) > MaxSnapshotBytes+1 {
			candidate = candidate[:MaxSnapshotBytes+1]
		}
		original := fixtureClone(candidate)
		backend := newFixtureBackend()
		key := fixtureStoreKey(keys.controller)
		backend.seedDurable(key, candidate)
		backend.exposeReadBytes()

		store, err := Open(context.Background(), backend, keys.controller)
		if err != nil {
			if store != nil {
				t.Fatal("Open returned both a handle and an error")
			}
			if !errors.Is(err, ErrCorrupt) {
				t.Fatalf("in-memory Open error = %v, want ErrCorrupt", err)
			}
		} else {
			if store == nil {
				t.Fatal("Open succeeded with a nil handle")
			}
			fixtureAssertStoreMatchesReplay(t, store, keys.controller)
			if closeErr := store.Close(); closeErr != nil {
				t.Fatalf("Close: %v", closeErr)
			}
		}

		_, puts, syncs, deletes, closes := backend.operationCounts()
		if puts != 0 || syncs != 0 || deletes != 0 || closes != 0 {
			t.Fatalf("Open mutated backend: put:%d sync:%d delete:%d close:%d", puts, syncs, deletes, closes)
		}
		if got := backend.durableValue(key); !bytes.Equal(got, original) {
			t.Fatal("Open mutated stored snapshot bytes")
		}
		if sha256.Sum256(fuzzed) != inputDigest {
			t.Fatal("Open mutated supplied fuzz bytes")
		}
	})
}

func fixtureAssertStoreMatchesReplay(t testing.TB, store *Store, controller [32]byte) {
	t.Helper()
	records, err := store.Records()
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	state, err := accountauth.NewKnownState(controller)
	if err != nil {
		t.Fatalf("NewKnownState: %v", err)
	}

	for i, raw := range records {
		applyErr := state.Apply(raw)
		if i < accountauth.MaxKnownRecords {
			if applyErr != nil {
				t.Fatalf("replay record %d: %v", i, applyErr)
			}
		} else if applyErr == nil || !state.Saturated() {
			t.Fatalf("overflow replay record %d = %v, saturated %v", i, applyErr, state.Saturated())
		}
	}
	storeSaturated, err := store.Saturated()
	if err != nil {
		t.Fatalf("Saturated: %v", err)
	}
	if storeSaturated != state.Saturated() {
		t.Fatalf("saturation = store:%v replay:%v", storeSaturated, state.Saturated())
	}

	for i, raw := range records {
		record, err := accountauth.VerifyRecord(raw)
		if err != nil {
			t.Fatalf("VerifyRecord %d after successful Open: %v", i, err)
		}
		if record.Kind() != accountauth.Grant {
			continue
		}
		got, err := store.AuthorizesKnown(record.GrantID(), record.DeviceKey(), record.Capabilities())
		if err != nil {
			t.Fatalf("AuthorizesKnown record %d: %v", i, err)
		}
		want := state.AuthorizesKnown(record.GrantID(), record.DeviceKey(), record.Capabilities())
		if got != want {
			t.Fatalf("authorization record %d = store:%v replay:%v", i, got, want)
		}
	}

	if len(records) > 0 {
		wantFirst := fixtureClone(records[0])
		records[0][0] ^= 0xff
		records[0] = nil
		again, err := store.Records()
		if err != nil {
			t.Fatalf("Records after caller mutation: %v", err)
		}
		if len(again) == 0 || !bytes.Equal(again[0], wantFirst) {
			t.Fatal("caller mutation changed successfully loaded records")
		}
	}
}
