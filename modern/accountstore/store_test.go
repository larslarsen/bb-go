package accountstore

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"testing"

	datastore "github.com/ipfs/go-datastore"
	query "github.com/ipfs/go-datastore/query"
	"github.com/larslarsen/bb-go/modern/accountauth"
)

const (
	fixtureSnapshotMagic      = "BBACST01"
	fixtureAccountDomain      = "bitbook/accountauth/v1/account\x00"
	fixtureGrantRootDomain    = "bitbook/accountauth/v1/grant/root\x00"
	fixtureGrantDeviceDomain  = "bitbook/accountauth/v1/grant/device\x00"
	fixtureRevokeGrantDomain  = "bitbook/accountauth/v1/revoke-grant\x00"
	fixtureRevokeDeviceDomain = "bitbook/accountauth/v1/revoke-device\x00"
	fixtureGrantIDDomain      = "bitbook/accountauth/v1/grant-id\x00"
	fixtureStoreKeyPrefix     = "/bitbook/accountauth/v1/"
)

type fixtureKeys struct {
	controllerPrivate  ed25519.PrivateKey
	controller         [32]byte
	devicePrivate      ed25519.PrivateKey
	device             [32]byte
	secondPrivate      ed25519.PrivateKey
	second             [32]byte
	replacementPrivate ed25519.PrivateKey
	replacement        [32]byte
	foreignPrivate     ed25519.PrivateKey
	foreign            [32]byte
}

func fixturePrivateKey(label string) ed25519.PrivateKey {
	seed := sha256.Sum256([]byte("bitbook/accountstore/test/" + label))
	return ed25519.NewKeyFromSeed(seed[:])
}

func fixturePublicKey(private ed25519.PrivateKey) [32]byte {
	var public [32]byte
	copy(public[:], private[32:])
	return public
}

func fixtureKeySet() fixtureKeys {
	controller := fixturePrivateKey("controller")
	device := fixturePrivateKey("device")
	second := fixturePrivateKey("second-device")
	replacement := fixturePrivateKey("replacement-device")
	foreign := fixturePrivateKey("foreign-controller")
	return fixtureKeys{
		controllerPrivate:  controller,
		controller:         fixturePublicKey(controller),
		devicePrivate:      device,
		device:             fixturePublicKey(device),
		secondPrivate:      second,
		second:             fixturePublicKey(second),
		replacementPrivate: replacement,
		replacement:        fixturePublicKey(replacement),
		foreignPrivate:     foreign,
		foreign:            fixturePublicKey(foreign),
	}
}

func fixtureNonce(value uint64) [32]byte {
	var nonce [32]byte
	binary.BigEndian.PutUint64(nonce[24:], value)
	return nonce
}

func fixtureGrant(controllerPrivate, devicePrivate ed25519.PrivateKey, capabilities accountauth.Capability, nonce [32]byte) []byte {
	controller := fixturePublicKey(controllerPrivate)
	device := fixturePublicKey(devicePrivate)
	payload := make([]byte, 106)
	payload[0] = 1
	payload[1] = 1
	copy(payload[2:34], controller[:])
	copy(payload[34:66], device[:])
	binary.BigEndian.PutUint64(payload[66:74], uint64(capabilities))
	copy(payload[74:106], nonce[:])

	raw := fixtureClone(payload)
	raw = append(raw, ed25519.Sign(controllerPrivate,
		append([]byte(fixtureGrantRootDomain), payload...))...)
	raw = append(raw, ed25519.Sign(devicePrivate,
		append([]byte(fixtureGrantDeviceDomain), payload...))...)
	return raw
}

func fixtureGrantID(raw []byte) accountauth.GrantID {
	preimage := append([]byte(fixtureGrantIDDomain), raw[:106]...)
	return accountauth.GrantID(sha256.Sum256(preimage))
}

func fixtureRevocation(kind byte, domain string, controllerPrivate ed25519.PrivateKey, target [32]byte) []byte {
	controller := fixturePublicKey(controllerPrivate)
	payload := make([]byte, 66)
	payload[0] = 1
	payload[1] = kind
	copy(payload[2:34], controller[:])
	copy(payload[34:66], target[:])
	raw := fixtureClone(payload)
	raw = append(raw, ed25519.Sign(controllerPrivate, append([]byte(domain), payload...))...)
	return raw
}

func fixtureRevokeGrant(controllerPrivate ed25519.PrivateKey, grant accountauth.GrantID) []byte {
	return fixtureRevocation(2, fixtureRevokeGrantDomain, controllerPrivate, [32]byte(grant))
}

func fixtureRevokeDevice(controllerPrivate ed25519.PrivateKey, device [32]byte) []byte {
	return fixtureRevocation(3, fixtureRevokeDeviceDomain, controllerPrivate, device)
}

func fixtureStoreKey(controller [32]byte) datastore.Key {
	preimage := append([]byte(fixtureAccountDomain), controller[:]...)
	accountID := sha256.Sum256(preimage)
	return datastore.NewKey(fixtureStoreKeyPrefix + hex.EncodeToString(accountID[:]))
}

func fixtureSnapshot(controller [32]byte, records [][]byte) []byte {
	return fixtureSnapshotWithCount(controller, len(records), records)
}

func fixtureSnapshotWithCount(controller [32]byte, count int, records [][]byte) []byte {
	capacity := 42 + 32
	for _, record := range records {
		capacity += 2 + len(record)
	}
	raw := make([]byte, 42, capacity)
	copy(raw[:8], []byte(fixtureSnapshotMagic))
	copy(raw[8:40], controller[:])
	binary.BigEndian.PutUint16(raw[40:42], uint16(count))
	for _, record := range records {
		var size [2]byte
		binary.BigEndian.PutUint16(size[:], uint16(len(record)))
		raw = append(raw, size[:]...)
		raw = append(raw, record...)
	}
	digest := sha256.Sum256(raw)
	return append(raw, digest[:]...)
}

func fixtureSnapshotWithBody(controller [32]byte, count int, body []byte) []byte {
	raw := make([]byte, 42, 42+len(body)+32)
	copy(raw[:8], []byte(fixtureSnapshotMagic))
	copy(raw[8:40], controller[:])
	binary.BigEndian.PutUint16(raw[40:42], uint16(count))
	raw = append(raw, body...)
	digest := sha256.Sum256(raw)
	return append(raw, digest[:]...)
}

func fixtureRechecksum(raw []byte) {
	if len(raw) < sha256.Size {
		return
	}
	digest := sha256.Sum256(raw[:len(raw)-sha256.Size])
	copy(raw[len(raw)-sha256.Size:], digest[:])
}

func fixtureClone(raw []byte) []byte {
	return append([]byte(nil), raw...)
}

func fixtureCloneRecords(records [][]byte) [][]byte {
	cloned := make([][]byte, len(records))
	for i, record := range records {
		cloned[i] = fixtureClone(record)
	}
	return cloned
}

func fixtureRequireErrorIs(t testing.TB, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("error = %v, want errors.Is(_, %v)", err, target)
	}
}

var errFixtureBackendClosed = errors.New("fixture backend closed")

type fixtureBackend struct {
	mu sync.Mutex

	durable     map[string][]byte
	pending     map[string][]byte
	closed      bool
	closeCalls  int
	getCalls    int
	putCalls    int
	syncCalls   int
	deleteCalls int
	lastPutKey  datastore.Key
	lastSyncKey datastore.Key

	getErr           error
	putErr           error
	retainOnPutError bool
	syncErr          error
	syncEntered      chan struct{}
	syncRelease      chan struct{}
	borrowReadBytes  bool
}

func newFixtureBackend() *fixtureBackend {
	return &fixtureBackend{
		durable: make(map[string][]byte),
		pending: make(map[string][]byte),
	}
}

func (b *fixtureBackend) Get(ctx context.Context, key datastore.Key) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.getCalls++
	if b.closed {
		return nil, errFixtureBackendClosed
	}
	if b.getErr != nil {
		return nil, b.getErr
	}
	if value, ok := b.pending[key.String()]; ok {
		if b.borrowReadBytes {
			return value, nil
		}
		return fixtureClone(value), nil
	}
	value, ok := b.durable[key.String()]
	if !ok {
		return nil, datastore.ErrNotFound
	}
	if b.borrowReadBytes {
		return value, nil
	}
	return fixtureClone(value), nil
}

func (b *fixtureBackend) Put(ctx context.Context, key datastore.Key, value []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.putCalls++
	b.lastPutKey = key
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.closed {
		return errFixtureBackendClosed
	}
	if b.putErr != nil {
		if b.retainOnPutError {
			b.pending[key.String()] = fixtureClone(value)
		}
		return b.putErr
	}
	b.pending[key.String()] = fixtureClone(value)
	return nil
}

func (b *fixtureBackend) Sync(ctx context.Context, key datastore.Key) error {
	b.mu.Lock()
	b.syncCalls++
	b.lastSyncKey = key
	if b.closed {
		b.mu.Unlock()
		return errFixtureBackendClosed
	}
	entered := b.syncEntered
	release := b.syncRelease
	syncErr := b.syncErr
	b.mu.Unlock()

	if entered != nil {
		select {
		case entered <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if syncErr != nil {
		return syncErr
	}
	if value, ok := b.pending[key.String()]; ok {
		b.durable[key.String()] = fixtureClone(value)
		delete(b.pending, key.String())
	}
	return nil
}

func (b *fixtureBackend) Has(ctx context.Context, key datastore.Key) (bool, error) {
	_, err := b.Get(ctx, key)
	if errors.Is(err, datastore.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (b *fixtureBackend) GetSize(ctx context.Context, key datastore.Key) (int, error) {
	value, err := b.Get(ctx, key)
	if err != nil {
		return -1, err
	}
	return len(value), nil
}

func (b *fixtureBackend) Delete(ctx context.Context, key datastore.Key) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.deleteCalls++
	if b.closed {
		return errFixtureBackendClosed
	}
	delete(b.pending, key.String())
	delete(b.durable, key.String())
	return nil
}

func (b *fixtureBackend) Query(ctx context.Context, q query.Query) (query.Results, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, errFixtureBackendClosed
	}
	visible := make(map[string][]byte, len(b.durable)+len(b.pending))
	for key, value := range b.durable {
		visible[key] = value
	}
	for key, value := range b.pending {
		visible[key] = value
	}
	entries := make([]query.Entry, 0, len(visible))
	for key, value := range visible {
		entry := query.Entry{Key: key, Size: len(value)}
		if !q.KeysOnly {
			entry.Value = fixtureClone(value)
		}
		entries = append(entries, entry)
	}
	return query.ResultsWithEntries(q, entries), nil
}

func (b *fixtureBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closeCalls++
	b.closed = true
	return nil
}

func (b *fixtureBackend) seedDurable(key datastore.Key, value []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.durable[key.String()] = fixtureClone(value)
}

func (b *fixtureBackend) exposeReadBytes() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.borrowReadBytes = true
}

func (b *fixtureBackend) durableValue(key datastore.Key) []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return fixtureClone(b.durable[key.String()])
}

func (b *fixtureBackend) resetOperations() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.getCalls = 0
	b.putCalls = 0
	b.syncCalls = 0
	b.deleteCalls = 0
	b.lastPutKey = datastore.Key{}
	b.lastSyncKey = datastore.Key{}
	b.getErr = nil
	b.putErr = nil
	b.retainOnPutError = false
	b.syncErr = nil
	b.syncEntered = nil
	b.syncRelease = nil
}

func (b *fixtureBackend) operationCounts() (gets, puts, syncs, deletes, closes int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.getCalls, b.putCalls, b.syncCalls, b.deleteCalls, b.closeCalls
}

func (b *fixtureBackend) configureFailure(putErr error, retainOnPutError bool, syncErr error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.putErr = putErr
	b.retainOnPutError = retainOnPutError
	b.syncErr = syncErr
}

func (b *fixtureBackend) configureSyncBarrier(entered, release chan struct{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.syncEntered = entered
	b.syncRelease = release
}

func (b *fixtureBackend) resolvePending(key datastore.Key, outcome string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	value, pending := b.pending[key.String()]
	switch outcome {
	case "old":
		delete(b.pending, key.String())
		return nil
	case "new":
		if !pending {
			return errors.New("no pending value for new outcome")
		}
		b.durable[key.String()] = fixtureClone(value)
		delete(b.pending, key.String())
		return nil
	case "corrupt":
		if !pending {
			return errors.New("no pending value for corrupt outcome")
		}
		cut := len(value) / 2
		if cut == 0 {
			cut = 1
		}
		b.durable[key.String()] = fixtureClone(value[:cut])
		delete(b.pending, key.String())
		return nil
	default:
		return fmt.Errorf("unknown outcome %q", outcome)
	}
}

func TestCreateOpenAndStoreKey(t *testing.T) {
	ctx := context.Background()
	keys := fixtureKeySet()
	backend := newFixtureBackend()
	unrelatedKey := datastore.NewKey("/unrelated/value")
	backend.seedDurable(unrelatedKey, []byte("preserve me"))

	if MaxStoredRecords != accountauth.MaxKnownRecords+1 {
		t.Fatalf("MaxStoredRecords = %d, want %d", MaxStoredRecords, accountauth.MaxKnownRecords+1)
	}
	if MaxSnapshotBytes != 74+MaxStoredRecords*(2+accountauth.MaxRecordBytes) {
		t.Fatalf("MaxSnapshotBytes = %d, want formula result", MaxSnapshotBytes)
	}
	if MaxSnapshotBytes != 966966 {
		t.Fatalf("MaxSnapshotBytes = %d, want 966966", MaxSnapshotBytes)
	}

	store, err := Create(ctx, backend, keys.controller)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	wantKey := fixtureStoreKey(keys.controller)
	wantEmpty := fixtureSnapshot(keys.controller, nil)
	if got := backend.durableValue(wantKey); !bytes.Equal(got, wantEmpty) {
		t.Fatalf("created snapshot = %x, want %x", got, wantEmpty)
	}
	gets, puts, syncs, deletes, closes := backend.operationCounts()
	if gets != 1 || puts != 1 || syncs != 1 || deletes != 0 || closes != 0 {
		t.Fatalf("Create operations = get:%d put:%d sync:%d delete:%d close:%d", gets, puts, syncs, deletes, closes)
	}
	if backend.lastPutKey.String() != wantKey.String() || backend.lastSyncKey.String() != wantKey.String() {
		t.Fatalf("Create keys = put:%q sync:%q, want %q", backend.lastPutKey, backend.lastSyncKey, wantKey)
	}
	if len(wantKey.String()) != len(fixtureStoreKeyPrefix)+64 {
		t.Fatalf("store key = %q, want prefix plus 64 lowercase hex characters", wantKey)
	}
	for _, char := range wantKey.String()[len(fixtureStoreKeyPrefix):] {
		if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f') {
			t.Fatalf("store key contains non-lowercase-hex character %q", char)
		}
	}

	beforeDuplicate := backend.durableValue(wantKey)
	backend.resetOperations()
	duplicate, err := Create(ctx, backend, keys.controller)
	if duplicate != nil {
		t.Fatal("duplicate Create returned a handle")
	}
	fixtureRequireErrorIs(t, err, ErrExists)
	_, puts, syncs, deletes, _ = backend.operationCounts()
	if puts != 0 || syncs != 0 || deletes != 0 {
		t.Fatalf("duplicate Create changed storage: put:%d sync:%d delete:%d", puts, syncs, deletes)
	}
	if got := backend.durableValue(wantKey); !bytes.Equal(got, beforeDuplicate) {
		t.Fatal("duplicate Create overwrote the existing snapshot")
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	_, _, _, _, closes = backend.operationCounts()
	if closes != 0 {
		t.Fatalf("Store.Close closed the shared backend %d times", closes)
	}

	backend.resetOperations()
	reopened, err := Open(ctx, backend, keys.controller)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if records, err := reopened.Records(); err != nil || len(records) != 0 {
		t.Fatalf("empty Records = %d, %v", len(records), err)
	}
	_, puts, syncs, deletes, _ = backend.operationCounts()
	if puts != 0 || syncs != 0 || deletes != 0 {
		t.Fatalf("Open wrote storage: put:%d sync:%d delete:%d", puts, syncs, deletes)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("close reopened Store: %v", err)
	}
	if got, err := backend.Get(ctx, unrelatedKey); err != nil || string(got) != "preserve me" {
		t.Fatalf("unrelated key = %q, %v", got, err)
	}

	backend.resetOperations()
	missing, err := Open(ctx, backend, keys.foreign)
	if missing != nil {
		t.Fatal("missing Open returned a handle")
	}
	fixtureRequireErrorIs(t, err, datastore.ErrNotFound)
	_, puts, syncs, deletes, _ = backend.operationCounts()
	if puts != 0 || syncs != 0 || deletes != 0 {
		t.Fatal("missing Open created or repaired data")
	}
}

func TestCreateOpenRejectInvalidInputsAndExistingCorruption(t *testing.T) {
	ctx := context.Background()
	keys := fixtureKeySet()

	for name, open := range map[string]func(context.Context, datastore.Datastore, [32]byte) (*Store, error){
		"Create": Create,
		"Open":   Open,
	} {
		t.Run(name+" nil backend", func(t *testing.T) {
			store, err := open(ctx, nil, keys.controller)
			if err == nil || store != nil {
				t.Fatalf("result = %#v, %v; want nil handle and error", store, err)
			}
		})
		t.Run(name+" zero controller", func(t *testing.T) {
			backend := newFixtureBackend()
			store, err := open(ctx, backend, [32]byte{})
			if err == nil || store != nil {
				t.Fatalf("result = %#v, %v; want nil handle and error", store, err)
			}
			gets, puts, syncs, deletes, _ := backend.operationCounts()
			if gets != 0 || puts != 0 || syncs != 0 || deletes != 0 {
				t.Fatal("zero controller accessed storage")
			}
		})
	}

	t.Run("existing corrupt Create", func(t *testing.T) {
		backend := newFixtureBackend()
		key := fixtureStoreKey(keys.controller)
		corrupt := []byte("not a snapshot")
		backend.seedDurable(key, corrupt)
		store, err := Create(ctx, backend, keys.controller)
		if store != nil {
			t.Fatal("Create returned a handle for existing corrupt data")
		}
		fixtureRequireErrorIs(t, err, ErrExists)
		if got := backend.durableValue(key); !bytes.Equal(got, corrupt) {
			t.Fatal("Create repaired or overwrote corrupt data")
		}
		_, puts, syncs, deletes, _ := backend.operationCounts()
		if puts != 0 || syncs != 0 || deletes != 0 {
			t.Fatal("Create wrote existing corrupt data")
		}
	})

	t.Run("corrupt Open", func(t *testing.T) {
		backend := newFixtureBackend()
		key := fixtureStoreKey(keys.controller)
		corrupt := []byte("not a snapshot")
		backend.seedDurable(key, corrupt)
		store, err := Open(ctx, backend, keys.controller)
		if store != nil {
			t.Fatal("Open returned a handle for corrupt data")
		}
		fixtureRequireErrorIs(t, err, ErrCorrupt)
		if got := backend.durableValue(key); !bytes.Equal(got, corrupt) {
			t.Fatal("Open repaired corrupt data")
		}
		_, puts, syncs, deletes, _ := backend.operationCounts()
		if puts != 0 || syncs != 0 || deletes != 0 {
			t.Fatal("Open wrote corrupt data")
		}
	})

	t.Run("snapshot copied to another account key", func(t *testing.T) {
		backend := newFixtureBackend()
		copied := fixtureSnapshot(keys.controller, nil)
		foreignKey := fixtureStoreKey(keys.foreign)
		backend.seedDurable(foreignKey, copied)
		store, err := Open(ctx, backend, keys.foreign)
		if store != nil {
			t.Fatal("Open returned a handle for a foreign snapshot")
		}
		fixtureRequireErrorIs(t, err, ErrCorrupt)
		if got := backend.durableValue(foreignKey); !bytes.Equal(got, copied) {
			t.Fatal("Open modified the foreign snapshot")
		}
	})

	t.Run("backend Get errors propagate", func(t *testing.T) {
		backendErr := errors.New("fixture get failure")
		for name, open := range map[string]func(context.Context, datastore.Datastore, [32]byte) (*Store, error){
			"Create": Create,
			"Open":   Open,
		} {
			t.Run(name, func(t *testing.T) {
				backend := newFixtureBackend()
				backend.getErr = backendErr
				store, err := open(ctx, backend, keys.controller)
				if store != nil {
					t.Fatal("backend Get error returned a handle")
				}
				fixtureRequireErrorIs(t, err, backendErr)
				_, puts, syncs, deletes, _ := backend.operationCounts()
				if puts != 0 || syncs != 0 || deletes != 0 {
					t.Fatal("backend Get error was followed by a write")
				}
			})
		}
	})

	for _, tc := range []struct {
		name    string
		putErr  error
		syncErr error
	}{
		{name: "Put failure", putErr: errors.New("fixture create put failure")},
		{name: "Sync failure", syncErr: errors.New("fixture create sync failure")},
	} {
		t.Run("Create "+tc.name, func(t *testing.T) {
			backend := newFixtureBackend()
			backend.putErr = tc.putErr
			backend.syncErr = tc.syncErr
			store, err := Create(ctx, backend, keys.controller)
			if store != nil {
				t.Fatal("failed Create returned a handle")
			}
			fixtureRequireErrorIs(t, err, ErrUnavailable)
			if tc.putErr != nil {
				if _, _, syncs, _, _ := backend.operationCounts(); syncs != 0 {
					t.Fatal("Create called Sync after Put failed")
				}
			}
		})
	}
}

func TestStoreUnavailableBindingsAndOwnedRecords(t *testing.T) {
	keys := fixtureKeySet()
	grant := fixtureGrant(keys.controllerPrivate, keys.devicePrivate,
		accountauth.CapMessage|accountauth.CapAssessment, fixtureNonce(1))
	grantID := fixtureGrantID(grant)

	var nilStore *Store
	fixtureRequireErrorIs(t, nilStore.Apply(context.Background(), grant), ErrUnavailable)
	if authorized, err := nilStore.AuthorizesKnown(grantID, keys.device, accountauth.CapMessage); authorized || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("nil AuthorizesKnown = %v, %v", authorized, err)
	}
	if saturated, err := nilStore.Saturated(); saturated || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("nil Saturated = %v, %v", saturated, err)
	}
	if records, err := nilStore.Records(); records != nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("nil Records = %v, %v", records, err)
	}
	if err := nilStore.Close(); err != nil {
		t.Fatalf("nil Close: %v", err)
	}

	var zero Store
	fixtureRequireErrorIs(t, zero.Apply(context.Background(), grant), ErrUnavailable)
	if authorized, err := zero.AuthorizesKnown(grantID, keys.device, accountauth.CapMessage); authorized || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("zero AuthorizesKnown = %v, %v", authorized, err)
	}
	if saturated, err := zero.Saturated(); saturated || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("zero Saturated = %v, %v", saturated, err)
	}
	if records, err := zero.Records(); records != nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("zero Records = %v, %v", records, err)
	}
	if err := zero.Close(); err != nil {
		t.Fatalf("zero Close: %v", err)
	}

	backend := newFixtureBackend()
	store, err := Create(context.Background(), backend, keys.controller)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	original := fixtureClone(grant)
	if err := store.Apply(context.Background(), grant); err != nil {
		t.Fatalf("Apply grant: %v", err)
	}
	clear(grant)
	backend.resetOperations()
	if authorized, err := store.AuthorizesKnown(grantID, keys.device, accountauth.CapMessage|accountauth.CapAssessment); err != nil || !authorized {
		t.Fatalf("authorized control = %v, %v", authorized, err)
	}
	if authorized, err := store.AuthorizesKnown(grantID, keys.second, accountauth.CapMessage); err != nil || authorized {
		t.Fatalf("wrong-device binding = %v, %v", authorized, err)
	}
	if authorized, err := store.AuthorizesKnown(grantID, keys.device, accountauth.CapPaymentRequest); err != nil || authorized {
		t.Fatalf("ungranted capability = %v, %v", authorized, err)
	}

	records, err := store.Records()
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if len(records) != 1 || !bytes.Equal(records[0], original) {
		t.Fatalf("Records = %x, want original grant", records)
	}
	records[0][0] ^= 0xff
	records[0] = nil
	records = append(records, []byte("caller mutation"))
	again, err := store.Records()
	if err != nil {
		t.Fatalf("second Records: %v", err)
	}
	if len(again) != 1 || !bytes.Equal(again[0], original) {
		t.Fatal("returned-slice mutation changed stored records")
	}
	gets, puts, syncs, deletes, _ := backend.operationCounts()
	if gets != 0 || puts != 0 || syncs != 0 || deletes != 0 {
		t.Fatalf("in-memory reads touched storage: get:%d put:%d sync:%d delete:%d", gets, puts, syncs, deletes)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	fixtureRequireErrorIs(t, store.Apply(context.Background(), original), ErrUnavailable)
	if authorized, err := store.AuthorizesKnown(grantID, keys.device, accountauth.CapMessage); authorized || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("closed AuthorizesKnown = %v, %v", authorized, err)
	}
	if saturated, err := store.Saturated(); saturated || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("closed Saturated = %v, %v", saturated, err)
	}
	if records, err := store.Records(); records != nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("closed Records = %v, %v", records, err)
	}
	if _, _, _, _, closes := backend.operationCounts(); closes != 0 {
		t.Fatalf("Store.Close closed shared backend %d times", closes)
	}

	reopened, err := Open(context.Background(), backend, keys.controller)
	if err != nil {
		t.Fatalf("Open after Store.Close: %v", err)
	}
	if authorized, err := reopened.AuthorizesKnown(grantID, keys.device, accountauth.CapMessage); err != nil || !authorized {
		t.Fatalf("reopened authorization = %v, %v", authorized, err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("close reopened Store: %v", err)
	}
}

func TestConcurrentApplyReadAndClose(t *testing.T) {
	ctx := context.Background()
	keys := fixtureKeySet()
	backend := newFixtureBackend()
	store, err := Create(ctx, backend, keys.controller)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	first := fixtureGrant(keys.controllerPrivate, keys.devicePrivate, accountauth.CapMessage, fixtureNonce(10))
	second := fixtureGrant(keys.controllerPrivate, keys.secondPrivate, accountauth.CapPaymentRequest, fixtureNonce(11))
	firstID := fixtureGrantID(first)
	secondID := fixtureGrantID(second)
	revokeFirstDevice := fixtureRevokeDevice(keys.controllerPrivate, keys.device)

	start := make(chan struct{})
	errorsCh := make(chan error, 3)
	for _, raw := range [][]byte{first, second, revokeFirstDevice} {
		raw := fixtureClone(raw)
		go func() {
			<-start
			errorsCh <- store.Apply(ctx, raw)
		}()
	}
	readErrors := make(chan error, 4)
	var readers sync.WaitGroup
	readers.Add(4)
	for range 4 {
		go func() {
			defer readers.Done()
			<-start
			for range 100 {
				if _, err := store.AuthorizesKnown(firstID, keys.device, accountauth.CapMessage); err != nil {
					readErrors <- err
					return
				}
				if _, err := store.Records(); err != nil {
					readErrors <- err
					return
				}
			}
		}()
	}
	close(start)
	for range 3 {
		if err := <-errorsCh; err != nil {
			t.Fatalf("concurrent Apply: %v", err)
		}
	}
	readers.Wait()
	close(readErrors)
	for err := range readErrors {
		t.Fatalf("concurrent read: %v", err)
	}

	if authorized, err := store.AuthorizesKnown(firstID, keys.device, accountauth.CapMessage); err != nil || authorized {
		t.Fatalf("revoked concurrent grant = %v, %v", authorized, err)
	}
	if authorized, err := store.AuthorizesKnown(secondID, keys.second, accountauth.CapPaymentRequest); err != nil || !authorized {
		t.Fatalf("independent concurrent grant = %v, %v", authorized, err)
	}
	records, err := store.Records()
	if err != nil || len(records) != 3 {
		t.Fatalf("concurrent Records = %d, %v", len(records), err)
	}

	third := fixtureGrant(keys.controllerPrivate, keys.replacementPrivate, accountauth.CapAssessment, fixtureNonce(12))
	raceStart := make(chan struct{})
	applyDone := make(chan error, 1)
	closeDone := make(chan error, 1)
	go func() {
		<-raceStart
		applyDone <- store.Apply(ctx, third)
	}()
	go func() {
		<-raceStart
		closeDone <- store.Close()
	}()
	close(raceStart)
	applyErr := <-applyDone
	if applyErr != nil && !errors.Is(applyErr, ErrUnavailable) {
		t.Fatalf("Apply racing Close = %v", applyErr)
	}
	if err := <-closeDone; err != nil {
		t.Fatalf("Close racing Apply = %v", err)
	}
	if _, err := store.Records(); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Records after Close race = %v", err)
	}
}

func TestOpenSnapshotValidation(t *testing.T) {
	keys := fixtureKeySet()
	grant := fixtureGrant(keys.controllerPrivate, keys.devicePrivate, accountauth.CapMessage, fixtureNonce(20))
	revokeGrant := fixtureRevokeGrant(keys.controllerPrivate, fixtureGrantID(grant))
	revokeDevice := fixtureRevokeDevice(keys.controllerPrivate, keys.device)
	foreignGrant := fixtureGrant(keys.foreignPrivate, keys.secondPrivate, accountauth.CapMessage, fixtureNonce(21))

	badMagic := fixtureSnapshot(keys.controller, nil)
	copy(badMagic[:8], []byte("BBACST02"))
	fixtureRechecksum(badMagic)
	wrongController := fixtureSnapshot(keys.controller, nil)
	copy(wrongController[8:40], keys.foreign[:])
	fixtureRechecksum(wrongController)
	badChecksum := fixtureSnapshot(keys.controller, [][]byte{grant})
	badChecksum[len(badChecksum)-1] ^= 1
	trailing := fixtureSnapshot(keys.controller, [][]byte{grant})
	trailingBody := append(fixtureClone(trailing[:len(trailing)-sha256.Size]), 0)
	trailingDigest := sha256.Sum256(trailingBody)
	trailing = append(trailingBody, trailingDigest[:]...)
	countMismatch := fixtureSnapshot(keys.controller, [][]byte{grant})
	binary.BigEndian.PutUint16(countMismatch[40:42], 2)
	fixtureRechecksum(countMismatch)
	invalidSignature := fixtureClone(grant)
	invalidSignature[len(invalidSignature)-1] ^= 1
	invalidLengthSnapshot := func(length int) []byte {
		body := make([]byte, 2+length)
		binary.BigEndian.PutUint16(body[:2], uint16(length))
		return fixtureSnapshotWithBody(keys.controller, 1, body)
	}
	tooMany := fixtureSnapshotWithCount(keys.controller, MaxStoredRecords+1, nil)
	duplicate := fixtureSnapshot(keys.controller, [][]byte{grant, grant})
	validGrantSnapshot := fixtureSnapshot(keys.controller, [][]byte{grant})

	maxRecords := make([][]byte, MaxStoredRecords)
	for i := range maxRecords {
		maxRecords[i] = grant
	}
	maximumSizedDuplicate := fixtureSnapshot(keys.controller, maxRecords)
	if len(maximumSizedDuplicate) != MaxSnapshotBytes {
		t.Fatalf("maximum fixture length = %d, want %d", len(maximumSizedDuplicate), MaxSnapshotBytes)
	}

	cases := []struct {
		name        string
		raw         []byte
		wantRecords int
		wantErr     error
	}{
		{name: "minimum empty", raw: fixtureSnapshot(keys.controller, nil)},
		{name: "valid grant", raw: validGrantSnapshot, wantRecords: 1},
		{name: "valid grant revocation", raw: fixtureSnapshot(keys.controller, [][]byte{revokeGrant}), wantRecords: 1},
		{name: "valid device revocation", raw: fixtureSnapshot(keys.controller, [][]byte{revokeDevice}), wantRecords: 1},
		{name: "nil", raw: nil, wantErr: ErrCorrupt},
		{name: "one below minimum", raw: make([]byte, 73), wantErr: ErrCorrupt},
		{name: "one-byte truncation", raw: validGrantSnapshot[:len(validGrantSnapshot)-1], wantErr: ErrCorrupt},
		{name: "one above maximum", raw: make([]byte, MaxSnapshotBytes+1), wantErr: ErrCorrupt},
		{name: "bad magic with checksum", raw: badMagic, wantErr: ErrCorrupt},
		{name: "wrong controller with checksum", raw: wrongController, wantErr: ErrCorrupt},
		{name: "bad checksum", raw: badChecksum, wantErr: ErrCorrupt},
		{name: "trailing byte with checksum", raw: trailing, wantErr: ErrCorrupt},
		{name: "count mismatch with checksum", raw: countMismatch, wantErr: ErrCorrupt},
		{name: "zero record length with checksum", raw: invalidLengthSnapshot(0), wantErr: ErrCorrupt},
		{name: "record length 129 with checksum", raw: invalidLengthSnapshot(129), wantErr: ErrCorrupt},
		{name: "record length 131 with checksum", raw: invalidLengthSnapshot(131), wantErr: ErrCorrupt},
		{name: "record length 233 with checksum", raw: invalidLengthSnapshot(233), wantErr: ErrCorrupt},
		{name: "record length 235 with checksum", raw: invalidLengthSnapshot(235), wantErr: ErrCorrupt},
		{name: "invalid signature with checksum", raw: fixtureSnapshot(keys.controller, [][]byte{invalidSignature}), wantErr: ErrCorrupt},
		{name: "foreign signed record with checksum", raw: fixtureSnapshot(keys.controller, [][]byte{foreignGrant}), wantErr: ErrCorrupt},
		{name: "duplicate semantic fact", raw: duplicate, wantErr: ErrCorrupt},
		{name: "count 4098", raw: tooMany, wantErr: ErrCorrupt},
		{name: "maximum bytes duplicate", raw: maximumSizedDuplicate, wantErr: ErrCorrupt},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			backend := newFixtureBackend()
			key := fixtureStoreKey(keys.controller)
			input := fixtureClone(tc.raw)
			backend.seedDurable(key, tc.raw)
			store, err := Open(context.Background(), backend, keys.controller)
			if tc.wantErr != nil {
				if store != nil {
					t.Fatal("Open returned a handle for invalid snapshot")
				}
				fixtureRequireErrorIs(t, err, tc.wantErr)
			} else {
				if err != nil {
					t.Fatalf("Open: %v", err)
				}
				records, recordsErr := store.Records()
				if recordsErr != nil || len(records) != tc.wantRecords {
					t.Fatalf("Records = %d, %v; want %d", len(records), recordsErr, tc.wantRecords)
				}
				if closeErr := store.Close(); closeErr != nil {
					t.Fatalf("Close: %v", closeErr)
				}
			}
			_, puts, syncs, deletes, _ := backend.operationCounts()
			if puts != 0 || syncs != 0 || deletes != 0 {
				t.Fatalf("Open changed storage: put:%d sync:%d delete:%d", puts, syncs, deletes)
			}
			if got := backend.durableValue(key); !bytes.Equal(got, input) {
				t.Fatal("Open mutated supplied snapshot bytes")
			}
		})
	}
}

func (b *fixtureBackend) String() string {
	gets, puts, syncs, deletes, closes := b.operationCounts()
	return fmt.Sprintf("fixtureBackend{get:%d put:%d sync:%d delete:%d close:%d}", gets, puts, syncs, deletes, closes)
}
