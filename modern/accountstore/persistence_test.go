package accountstore

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	datastore "github.com/ipfs/go-datastore"
	leveldb "github.com/ipfs/go-ds-leveldb"
	"github.com/larslarsen/bb-go/modern/accountauth"
)

const (
	processKillMarkerEnv = "BBGO_ACC002_PROCESS_KILL_CHILD"
	processKillPathEnv   = "BBGO_ACC002_PROCESS_KILL_PATH"
	processKillAck       = "BBGO_ACC002_REVOCATION_ACKNOWLEDGED"
)

type fixtureLockedBuffer struct {
	mu sync.Mutex
	bytes.Buffer
}

func (b *fixtureLockedBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.Write(value)
}

func (b *fixtureLockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.String()
}

func fixtureOpenLevelDB(t testing.TB, path string) *leveldb.Datastore {
	t.Helper()
	backend, err := leveldb.NewDatastore(path, nil)
	if err != nil {
		t.Fatalf("open LevelDB: %v", err)
	}
	return backend
}

func fixtureScheduleStoreAndLevelDBCleanup(t testing.TB, store *Store, backend *leveldb.Datastore) {
	t.Helper()
	t.Cleanup(func() {
		if store != nil {
			_ = store.Close()
		}
		if backend != nil {
			_ = backend.Close()
		}
	})
}

func fixtureCloseStoreAndLevelDB(t testing.TB, store *Store, backend *leveldb.Datastore) {
	t.Helper()
	if store != nil {
		if err := store.Close(); err != nil {
			t.Fatalf("close Store: %v", err)
		}
	}
	if backend != nil {
		if err := backend.Close(); err != nil {
			t.Fatalf("close LevelDB: %v", err)
		}
	}
}

func TestRevocationSurvivesReopen(t *testing.T) {
	ctx := context.Background()
	keys := fixtureKeySet()
	databasePath := filepath.Join(t.TempDir(), "account-leveldb")
	firstGrant := fixtureGrant(keys.controllerPrivate, keys.devicePrivate, accountauth.CapMessage, fixtureNonce(100))
	secondGrant := fixtureGrant(keys.controllerPrivate, keys.secondPrivate, accountauth.CapPaymentRequest, fixtureNonce(101))
	firstID := fixtureGrantID(firstGrant)
	secondID := fixtureGrantID(secondGrant)

	backend := fixtureOpenLevelDB(t, databasePath)
	store, err := Create(ctx, backend, keys.controller)
	fixtureScheduleStoreAndLevelDBCleanup(t, store, backend)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.Apply(ctx, firstGrant); err != nil {
		t.Fatalf("Apply first grant: %v", err)
	}
	if err := store.Apply(ctx, secondGrant); err != nil {
		t.Fatalf("Apply second grant: %v", err)
	}
	if authorized, err := store.AuthorizesKnown(firstID, keys.device, accountauth.CapMessage); err != nil || !authorized {
		t.Fatalf("first grant before reopen = %v, %v", authorized, err)
	}
	if authorized, err := store.AuthorizesKnown(secondID, keys.second, accountauth.CapPaymentRequest); err != nil || !authorized {
		t.Fatalf("second grant before reopen = %v, %v", authorized, err)
	}
	if records, err := store.Records(); err != nil || len(records) != 2 ||
		!bytes.Equal(records[0], firstGrant) || !bytes.Equal(records[1], secondGrant) {
		t.Fatalf("grant arrival order before reopen = %d records, %v", len(records), err)
	}
	fixtureCloseStoreAndLevelDB(t, store, backend)

	backend = fixtureOpenLevelDB(t, databasePath)
	store, err = Open(ctx, backend, keys.controller)
	fixtureScheduleStoreAndLevelDBCleanup(t, store, backend)
	if err != nil {
		t.Fatalf("Open after grants: %v", err)
	}
	if authorized, err := store.AuthorizesKnown(firstID, keys.device, accountauth.CapMessage); err != nil || !authorized {
		t.Fatalf("first grant after reopen = %v, %v", authorized, err)
	}
	if authorized, err := store.AuthorizesKnown(secondID, keys.second, accountauth.CapPaymentRequest); err != nil || !authorized {
		t.Fatalf("second grant after reopen = %v, %v", authorized, err)
	}
	if records, err := store.Records(); err != nil || len(records) != 2 ||
		!bytes.Equal(records[0], firstGrant) || !bytes.Equal(records[1], secondGrant) {
		t.Fatalf("grant arrival order after reopen = %d records, %v", len(records), err)
	}
	if err := store.Apply(ctx, fixtureRevokeDevice(keys.controllerPrivate, keys.device)); err != nil {
		t.Fatalf("Apply device revocation: %v", err)
	}
	if authorized, err := store.AuthorizesKnown(firstID, keys.device, accountauth.CapMessage); err != nil || authorized {
		t.Fatalf("device-revoked grant before reopen = %v, %v", authorized, err)
	}
	if authorized, err := store.AuthorizesKnown(secondID, keys.second, accountauth.CapPaymentRequest); err != nil || !authorized {
		t.Fatalf("unrelated device before reopen = %v, %v", authorized, err)
	}
	fixtureCloseStoreAndLevelDB(t, store, backend)

	backend = fixtureOpenLevelDB(t, databasePath)
	store, err = Open(ctx, backend, keys.controller)
	fixtureScheduleStoreAndLevelDBCleanup(t, store, backend)
	if err != nil {
		t.Fatalf("Open after device revocation: %v", err)
	}
	if authorized, err := store.AuthorizesKnown(firstID, keys.device, accountauth.CapMessage); err != nil || authorized {
		t.Fatalf("device revocation after reopen = %v, %v", authorized, err)
	}
	if authorized, err := store.AuthorizesKnown(secondID, keys.second, accountauth.CapPaymentRequest); err != nil || !authorized {
		t.Fatalf("other device after revocation reopen = %v, %v", authorized, err)
	}

	replacementGrant := fixtureGrant(keys.controllerPrivate, keys.replacementPrivate, accountauth.CapAssessment, fixtureNonce(102))
	replacementID := fixtureGrantID(replacementGrant)
	if err := store.Apply(ctx, replacementGrant); err != nil {
		t.Fatalf("Apply replacement grant: %v", err)
	}
	if err := store.Apply(ctx, fixtureRevokeGrant(keys.controllerPrivate, replacementID)); err != nil {
		t.Fatalf("Apply grant revocation: %v", err)
	}
	if authorized, err := store.AuthorizesKnown(replacementID, keys.replacement, accountauth.CapAssessment); err != nil || authorized {
		t.Fatalf("grant-specific revocation before reopen = %v, %v", authorized, err)
	}

	latePrivate := fixturePrivateKey("late-device")
	lateDevice := fixturePublicKey(latePrivate)
	lateGrant := fixtureGrant(keys.controllerPrivate, latePrivate, accountauth.CapProfileUpdate, fixtureNonce(103))
	lateID := fixtureGrantID(lateGrant)
	if err := store.Apply(ctx, fixtureRevokeGrant(keys.controllerPrivate, lateID)); err != nil {
		t.Fatalf("Apply revoke-before-grant: %v", err)
	}
	if err := store.Apply(ctx, lateGrant); err != nil {
		t.Fatalf("Apply grant after tombstone: %v", err)
	}
	if authorized, err := store.AuthorizesKnown(lateID, lateDevice, accountauth.CapProfileUpdate); err != nil || authorized {
		t.Fatalf("revoke-before-grant before reopen = %v, %v", authorized, err)
	}
	fixtureCloseStoreAndLevelDB(t, store, backend)

	backend = fixtureOpenLevelDB(t, databasePath)
	store, err = Open(ctx, backend, keys.controller)
	fixtureScheduleStoreAndLevelDBCleanup(t, store, backend)
	if err != nil {
		t.Fatalf("final Open: %v", err)
	}
	if authorized, err := store.AuthorizesKnown(firstID, keys.device, accountauth.CapMessage); err != nil || authorized {
		t.Fatalf("device revocation final = %v, %v", authorized, err)
	}
	if authorized, err := store.AuthorizesKnown(secondID, keys.second, accountauth.CapPaymentRequest); err != nil || !authorized {
		t.Fatalf("independent device final = %v, %v", authorized, err)
	}
	if authorized, err := store.AuthorizesKnown(replacementID, keys.replacement, accountauth.CapAssessment); err != nil || authorized {
		t.Fatalf("grant revocation final = %v, %v", authorized, err)
	}
	if authorized, err := store.AuthorizesKnown(lateID, lateDevice, accountauth.CapProfileUpdate); err != nil || authorized {
		t.Fatalf("revoke-before-grant final = %v, %v", authorized, err)
	}
	fixtureCloseStoreAndLevelDB(t, store, backend)
}

type fixtureBoundary struct {
	records    [][]byte
	firstGrant []byte
	firstID    accountauth.GrantID
	device     [32]byte
	revokedID  accountauth.GrantID
}

func fixture4095Facts(keys fixtureKeys) fixtureBoundary {
	records := make([][]byte, 0, 4095)
	var firstGrant []byte
	var revokedGrant []byte
	for i := 1; i <= 4093; i++ {
		grant := fixtureGrant(keys.controllerPrivate, keys.devicePrivate,
			accountauth.CapMessage, fixtureNonce(uint64(10_000+i)))
		if i == 1 {
			firstGrant = grant
		}
		if i == 2 {
			revokedGrant = grant
		}
		records = append(records, grant)
	}
	revokedID := fixtureGrantID(revokedGrant)
	records = append(records, fixtureRevokeGrant(keys.controllerPrivate, revokedID))
	unusedDevice := fixturePublicKey(fixturePrivateKey("boundary-unused-device"))
	records = append(records, fixtureRevokeDevice(keys.controllerPrivate, unusedDevice))
	return fixtureBoundary{
		records:    records,
		firstGrant: firstGrant,
		firstID:    fixtureGrantID(firstGrant),
		device:     keys.device,
		revokedID:  revokedID,
	}
}

func TestDurableSaturationBoundary(t *testing.T) {
	ctx := context.Background()
	keys := fixtureKeySet()
	boundary := fixture4095Facts(keys)
	backend := newFixtureBackend()
	key := fixtureStoreKey(keys.controller)
	backend.seedDurable(key, fixtureSnapshot(keys.controller, boundary.records))

	store, err := Open(ctx, backend, keys.controller)
	if err != nil {
		t.Fatalf("Open 4095 facts: %v", err)
	}
	if saturated, err := store.Saturated(); err != nil || saturated {
		t.Fatalf("4095 Saturated = %v, %v", saturated, err)
	}
	if authorized, err := store.AuthorizesKnown(boundary.firstID, boundary.device, accountauth.CapMessage); err != nil || !authorized {
		t.Fatalf("first grant at 4095 = %v, %v", authorized, err)
	}
	if authorized, err := store.AuthorizesKnown(boundary.revokedID, boundary.device, accountauth.CapMessage); err != nil || authorized {
		t.Fatalf("retained revoked grant at 4095 = %v, %v", authorized, err)
	}
	backend.resetOperations()

	last := fixtureGrant(keys.controllerPrivate, keys.secondPrivate, accountauth.CapPaymentRequest, fixtureNonce(20_000))
	lastID := fixtureGrantID(last)
	if err := store.Apply(ctx, last); err != nil {
		t.Fatalf("Apply 4096th fact: %v", err)
	}
	if saturated, err := store.Saturated(); err != nil || saturated {
		t.Fatalf("4096 Saturated = %v, %v", saturated, err)
	}
	if authorized, err := store.AuthorizesKnown(lastID, keys.second, accountauth.CapPaymentRequest); err != nil || !authorized {
		t.Fatalf("4096th grant = %v, %v", authorized, err)
	}
	_, putsAfterLast, syncsAfterLast, _, _ := backend.operationCounts()
	if putsAfterLast != 1 || syncsAfterLast != 1 {
		t.Fatalf("4096th persistence = put:%d sync:%d", putsAfterLast, syncsAfterLast)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close at 4096 facts: %v", err)
	}
	store, err = Open(ctx, backend, keys.controller)
	if err != nil {
		t.Fatalf("Open 4096 facts: %v", err)
	}
	if saturated, err := store.Saturated(); err != nil || saturated {
		t.Fatalf("reopened 4096 Saturated = %v, %v", saturated, err)
	}
	if authorized, err := store.AuthorizesKnown(lastID, keys.second, accountauth.CapPaymentRequest); err != nil || !authorized {
		t.Fatalf("reopened 4096th grant = %v, %v", authorized, err)
	}

	invalid := fixtureClone(last)
	invalid[len(invalid)-1] ^= 1
	if err := store.Apply(ctx, invalid); err == nil {
		t.Fatal("invalid input at capacity unexpectedly applied")
	}
	foreign := fixtureGrant(keys.foreignPrivate, keys.replacementPrivate,
		accountauth.CapAssessment, fixtureNonce(20_001))
	if err := store.Apply(ctx, foreign); err == nil {
		t.Fatal("foreign input at capacity unexpectedly applied")
	}
	if err := store.Apply(ctx, boundary.firstGrant); err != nil {
		t.Fatalf("duplicate at capacity: %v", err)
	}
	_, puts, syncs, _, _ := backend.operationCounts()
	if puts != putsAfterLast || syncs != syncsAfterLast {
		t.Fatalf("invalid, foreign or duplicate input wrote at capacity: put:%d sync:%d", puts, syncs)
	}
	if saturated, err := store.Saturated(); err != nil || saturated {
		t.Fatalf("invalid/foreign/duplicate saturated state = %v, %v", saturated, err)
	}

	records4096, err := store.Records()
	if err != nil || len(records4096) != 4096 {
		t.Fatalf("Records at capacity = %d, %v", len(records4096), err)
	}
	witness := fixtureGrant(keys.controllerPrivate, keys.replacementPrivate,
		accountauth.CapAssessment, fixtureNonce(20_002))
	if err := store.Apply(ctx, witness); !errors.Is(err, ErrSaturated) {
		t.Fatalf("4097th Apply = %v, want ErrSaturated", err)
	}
	if saturated, err := store.Saturated(); err != nil || !saturated {
		t.Fatalf("4097 Saturated = %v, %v", saturated, err)
	}
	if authorized, err := store.AuthorizesKnown(boundary.firstID, boundary.device, accountauth.CapMessage); err != nil || authorized {
		t.Fatalf("saturated authorization = %v, %v", authorized, err)
	}
	_, puts, syncs, _, _ = backend.operationCounts()
	if puts != putsAfterLast+1 || syncs != syncsAfterLast+1 {
		t.Fatalf("overflow witness persistence = put:%d sync:%d", puts, syncs)
	}
	records4097, err := store.Records()
	if err != nil || len(records4097) != MaxStoredRecords || !bytes.Equal(records4097[len(records4097)-1], witness) {
		t.Fatalf("overflow Records = %d, %v", len(records4097), err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close saturated Store: %v", err)
	}

	reopened, err := Open(ctx, backend, keys.controller)
	if err != nil {
		t.Fatalf("Open saturated snapshot: %v", err)
	}
	if saturated, err := reopened.Saturated(); err != nil || !saturated {
		t.Fatalf("reopened Saturated = %v, %v", saturated, err)
	}
	if authorized, err := reopened.AuthorizesKnown(lastID, keys.second, accountauth.CapPaymentRequest); err != nil || authorized {
		t.Fatalf("reopened saturated authorization = %v, %v", authorized, err)
	}
	backend.resetOperations()
	if err := reopened.Apply(ctx, boundary.firstGrant); !errors.Is(err, ErrSaturated) {
		t.Fatalf("duplicate Apply after saturation = %v", err)
	}
	novelAfterSaturation := fixtureGrant(keys.controllerPrivate, keys.secondPrivate,
		accountauth.CapAssessment, fixtureNonce(20_003))
	if err := reopened.Apply(ctx, novelAfterSaturation); !errors.Is(err, ErrSaturated) {
		t.Fatalf("novel Apply after saturation = %v", err)
	}
	invalidAfterSaturation := fixtureClone(novelAfterSaturation)
	invalidAfterSaturation[len(invalidAfterSaturation)-1] ^= 1
	if err := reopened.Apply(ctx, invalidAfterSaturation); err == nil || errors.Is(err, ErrSaturated) {
		t.Fatalf("invalid Apply after saturation = %v", err)
	}
	foreignAfterSaturation := fixtureGrant(keys.foreignPrivate, keys.devicePrivate,
		accountauth.CapPolicyWrite, fixtureNonce(20_004))
	if err := reopened.Apply(ctx, foreignAfterSaturation); err == nil || errors.Is(err, ErrSaturated) {
		t.Fatalf("foreign Apply after saturation = %v", err)
	}
	_, puts, syncs, _, _ = backend.operationCounts()
	if puts != 0 || syncs != 0 {
		t.Fatal("input after saturation wrote storage")
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("close reopened saturated Store: %v", err)
	}

	for name, badWitness := range map[string][]byte{
		"forged": func() []byte {
			forged := fixtureClone(witness)
			forged[len(forged)-1] ^= 1
			return forged
		}(),
		"duplicate": records4096[0],
	} {
		t.Run(name+" witness", func(t *testing.T) {
			badRecords := fixtureCloneRecords(records4096)
			badRecords = append(badRecords, fixtureClone(badWitness))
			badBackend := newFixtureBackend()
			badBackend.seedDurable(key, fixtureSnapshot(keys.controller, badRecords))
			opened, err := Open(ctx, badBackend, keys.controller)
			if opened != nil {
				t.Fatal("Open returned a handle for an invalid overflow witness")
			}
			fixtureRequireErrorIs(t, err, ErrCorrupt)
			_, puts, syncs, deletes, _ := badBackend.operationCounts()
			if puts != 0 || syncs != 0 || deletes != 0 {
				t.Fatal("Open repaired an invalid overflow witness")
			}
		})
	}
}

func TestStorageAcknowledgmentBarrier(t *testing.T) {
	ctx := context.Background()
	keys := fixtureKeySet()
	backend := newFixtureBackend()
	store, err := Create(ctx, backend, keys.controller)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	grant := fixtureGrant(keys.controllerPrivate, keys.devicePrivate, accountauth.CapMessage, fixtureNonce(300))
	grantID := fixtureGrantID(grant)
	backend.resetOperations()
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseSync := func() {
		releaseOnce.Do(func() { close(release) })
	}
	t.Cleanup(releaseSync)
	backend.configureSyncBarrier(entered, release)

	applyDone := make(chan error, 1)
	go func() {
		applyDone <- store.Apply(ctx, grant)
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Apply did not reach Sync barrier")
	}
	select {
	case err := <-applyDone:
		t.Fatalf("Apply returned before Sync acknowledgment: %v", err)
	default:
	}

	type authorizationResult struct {
		authorized bool
		err        error
	}
	readDone := make(chan authorizationResult, 1)
	go func() {
		authorized, err := store.AuthorizesKnown(grantID, keys.device, accountauth.CapMessage)
		readDone <- authorizationResult{authorized: authorized, err: err}
	}()
	readReturned := false
	select {
	case result := <-readDone:
		readReturned = true
		if result.err != nil || result.authorized {
			t.Fatalf("authority escaped before Sync: %v, %v", result.authorized, result.err)
		}
	default:
	}

	releaseSync()
	select {
	case err := <-applyDone:
		if err != nil {
			t.Fatalf("Apply after Sync release: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Apply did not return after Sync release")
	}
	if !readReturned {
		select {
		case result := <-readDone:
			if result.err != nil {
				t.Fatalf("concurrent read after Sync: %v", result.err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent read remained blocked after Sync")
		}
	}
	if authorized, err := store.AuthorizesKnown(grantID, keys.device, accountauth.CapMessage); err != nil || !authorized {
		t.Fatalf("authority after Sync = %v, %v", authorized, err)
	}
	_, puts, syncs, _, _ := backend.operationCounts()
	if puts != 1 || syncs != 1 {
		t.Fatalf("Apply operations = put:%d sync:%d", puts, syncs)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func fixtureStoreWithGrant(t testing.TB) (*Store, *fixtureBackend, fixtureKeys, []byte, accountauth.GrantID) {
	t.Helper()
	ctx := context.Background()
	keys := fixtureKeySet()
	backend := newFixtureBackend()
	store, err := Create(ctx, backend, keys.controller)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	grant := fixtureGrant(keys.controllerPrivate, keys.devicePrivate, accountauth.CapMessage, fixtureNonce(400))
	if err := store.Apply(ctx, grant); err != nil {
		t.Fatalf("Apply grant: %v", err)
	}
	backend.resetOperations()
	return store, backend, keys, grant, fixtureGrantID(grant)
}

func TestStorageFailuresDisableHandleAndNeverRollback(t *testing.T) {
	backendFailure := errors.New("fixture storage failure")
	cases := []struct {
		name      string
		putErr    error
		retainPut bool
		syncErr   error
		outcomes  []string
		wantSyncs int
	}{
		{name: "Put before retaining bytes", putErr: backendFailure, outcomes: []string{"old"}},
		{name: "Put after retaining bytes", putErr: backendFailure, retainPut: true, outcomes: []string{"old", "new", "corrupt"}},
		{name: "Sync", syncErr: backendFailure, outcomes: []string{"old", "new", "corrupt"}, wantSyncs: 1},
	}

	for _, tc := range cases {
		for _, outcome := range tc.outcomes {
			t.Run(tc.name+"/"+outcome, func(t *testing.T) {
				ctx := context.Background()
				store, backend, keys, grant, grantID := fixtureStoreWithGrant(t)
				backend.configureFailure(tc.putErr, tc.retainPut, tc.syncErr)
				revocation := fixtureRevokeDevice(keys.controllerPrivate, keys.device)
				err := store.Apply(ctx, revocation)
				fixtureRequireErrorIs(t, err, ErrUnavailable)
				if errors.Is(err, ErrSaturated) {
					t.Fatalf("storage failure reported ErrSaturated: %v", err)
				}
				if authorized, err := store.AuthorizesKnown(grantID, keys.device, accountauth.CapMessage); authorized || !errors.Is(err, ErrUnavailable) {
					t.Fatalf("disabled AuthorizesKnown = %v, %v", authorized, err)
				}
				if saturated, err := store.Saturated(); saturated || !errors.Is(err, ErrUnavailable) {
					t.Fatalf("disabled Saturated = %v, %v", saturated, err)
				}
				if records, err := store.Records(); records != nil || !errors.Is(err, ErrUnavailable) {
					t.Fatalf("disabled Records = %v, %v", records, err)
				}
				fixtureRequireErrorIs(t, store.Apply(ctx, grant), ErrUnavailable)
				_, puts, syncs, deletes, _ := backend.operationCounts()
				if puts != 1 || syncs != tc.wantSyncs || deletes != 0 {
					t.Fatalf("failure operations = put:%d sync:%d delete:%d", puts, syncs, deletes)
				}
				if err := store.Close(); err != nil {
					t.Fatalf("close disabled Store: %v", err)
				}

				key := fixtureStoreKey(keys.controller)
				if err := backend.resolvePending(key, outcome); err != nil {
					t.Fatalf("resolve %s outcome: %v", outcome, err)
				}
				reopened, openErr := Open(ctx, backend, keys.controller)
				if outcome == "corrupt" {
					if reopened != nil {
						t.Fatal("corrupt uncertain outcome returned a handle")
					}
					fixtureRequireErrorIs(t, openErr, ErrCorrupt)
					return
				}
				if openErr != nil {
					t.Fatalf("Open %s outcome: %v", outcome, openErr)
				}
				authorized, queryErr := reopened.AuthorizesKnown(grantID, keys.device, accountauth.CapMessage)
				if queryErr != nil {
					t.Fatalf("AuthorizesKnown %s outcome: %v", outcome, queryErr)
				}
				wantAuthorized := outcome == "old"
				if authorized != wantAuthorized {
					t.Fatalf("%s outcome authorized = %v, want %v", outcome, authorized, wantAuthorized)
				}
				if err := reopened.Close(); err != nil {
					t.Fatalf("close reopened Store: %v", err)
				}
			})
		}
	}
}

func TestCanceledContextDoesNotWriteOrDisable(t *testing.T) {
	store, backend, keys, _, grantID := fixtureStoreWithGrant(t)
	revocation := fixtureRevokeDevice(keys.controllerPrivate, keys.device)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	err := store.Apply(canceled, revocation)
	fixtureRequireErrorIs(t, err, context.Canceled)
	_, puts, syncs, deletes, _ := backend.operationCounts()
	if puts != 0 || syncs != 0 || deletes != 0 {
		t.Fatalf("canceled Apply wrote storage: put:%d sync:%d delete:%d", puts, syncs, deletes)
	}
	if authorized, err := store.AuthorizesKnown(grantID, keys.device, accountauth.CapMessage); err != nil || !authorized {
		t.Fatalf("canceled Apply changed or disabled state = %v, %v", authorized, err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestAcknowledgedRevocationSurvivesProcessKill(t *testing.T) {
	if os.Getenv(processKillMarkerEnv) == "1" {
		fixtureRunProcessKillChild(t)
		return
	}

	databasePath := filepath.Join(t.TempDir(), "killed-child-leveldb")
	commandCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(commandCtx, os.Args[0], "-test.run=^TestAcknowledgedRevocationSurvivesProcessKill$", "-test.v")
	cmd.Env = append(os.Environ(), processKillMarkerEnv+"=1", processKillPathEnv+"="+databasePath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("child stdout pipe: %v", err)
	}
	var stderr fixtureLockedBuffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()
	waited := false
	t.Cleanup(func() {
		if !waited {
			_ = cmd.Process.Kill()
			select {
			case <-waitDone:
			case <-time.After(5 * time.Second):
			}
		}
	})

	acknowledged := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if scanner.Text() == processKillAck {
				acknowledged <- nil
				return
			}
		}
		if err := scanner.Err(); err != nil {
			acknowledged <- err
			return
		}
		acknowledged <- errors.New("child exited without acknowledgment")
	}()

	select {
	case err := <-acknowledged:
		if err != nil {
			t.Fatalf("child acknowledgment: %v; stderr=%s", err, stderr.String())
		}
	case err := <-waitDone:
		waited = true
		t.Fatalf("child exited before acknowledgment: %v; stderr=%s", err, stderr.String())
	case <-commandCtx.Done():
		t.Fatalf("child acknowledgment timeout: %v; stderr=%s", commandCtx.Err(), stderr.String())
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill acknowledged child: %v", err)
	}
	select {
	case <-waitDone:
		waited = true
	case <-time.After(5 * time.Second):
		t.Fatal("killed child did not exit")
	}

	keys := fixtureKeySet()
	grant := fixtureGrant(keys.controllerPrivate, keys.devicePrivate, accountauth.CapMessage, fixtureNonce(900))
	grantID := fixtureGrantID(grant)
	secondGrant := fixtureGrant(keys.controllerPrivate, keys.secondPrivate, accountauth.CapPaymentRequest, fixtureNonce(901))
	secondID := fixtureGrantID(secondGrant)
	backend := fixtureOpenLevelDB(t, databasePath)
	store, err := Open(context.Background(), backend, keys.controller)
	fixtureScheduleStoreAndLevelDBCleanup(t, store, backend)
	if err != nil {
		t.Fatalf("Open after process kill: %v", err)
	}
	if authorized, err := store.AuthorizesKnown(grantID, keys.device, accountauth.CapMessage); err != nil || authorized {
		t.Fatalf("acknowledged revocation after process kill = %v, %v", authorized, err)
	}
	if authorized, err := store.AuthorizesKnown(secondID, keys.second, accountauth.CapPaymentRequest); err != nil || !authorized {
		t.Fatalf("independent device after process kill = %v, %v", authorized, err)
	}
	fixtureCloseStoreAndLevelDB(t, store, backend)
}

func fixtureRunProcessKillChild(t testing.TB) {
	t.Helper()
	databasePath := os.Getenv(processKillPathEnv)
	if databasePath == "" {
		t.Fatal("process-kill child missing database path")
	}
	keys := fixtureKeySet()
	backend := fixtureOpenLevelDB(t, databasePath)
	store, err := Create(context.Background(), backend, keys.controller)
	fixtureScheduleStoreAndLevelDBCleanup(t, store, backend)
	if err != nil {
		t.Fatalf("child Create: %v", err)
	}
	grant := fixtureGrant(keys.controllerPrivate, keys.devicePrivate, accountauth.CapMessage, fixtureNonce(900))
	secondGrant := fixtureGrant(keys.controllerPrivate, keys.secondPrivate, accountauth.CapPaymentRequest, fixtureNonce(901))
	if err := store.Apply(context.Background(), grant); err != nil {
		t.Fatalf("child Apply grant: %v", err)
	}
	if err := store.Apply(context.Background(), secondGrant); err != nil {
		t.Fatalf("child Apply second grant: %v", err)
	}
	if err := store.Apply(context.Background(), fixtureRevokeDevice(keys.controllerPrivate, keys.device)); err != nil {
		t.Fatalf("child Apply revocation: %v", err)
	}
	if authorized, err := store.AuthorizesKnown(fixtureGrantID(grant), keys.device, accountauth.CapMessage); err != nil || authorized {
		t.Fatalf("child revocation control = %v, %v", authorized, err)
	}
	if authorized, err := store.AuthorizesKnown(fixtureGrantID(secondGrant), keys.second, accountauth.CapPaymentRequest); err != nil || !authorized {
		t.Fatalf("child independent-device control = %v, %v", authorized, err)
	}
	if _, err := fmt.Fprintln(os.Stdout, processKillAck); err != nil {
		t.Fatalf("child acknowledgment write: %v", err)
	}
	select {}
}

var _ datastore.Datastore = (*fixtureBackend)(nil)
