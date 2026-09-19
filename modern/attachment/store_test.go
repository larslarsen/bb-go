package attachment

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/boxo/ipld/merkledag"
	pinning "github.com/ipfs/boxo/pinning/pinner"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	dsync "github.com/ipfs/go-datastore/sync"
	leveldb "github.com/ipfs/go-ds-leveldb"
	format "github.com/ipfs/go-ipld-format"
	"github.com/larslarsen/bb-go/modern/network"
	dht "github.com/libp2p/go-libp2p-kad-dht"
)

const testTimeout = 60 * time.Second

func TestMEDIA001M2BReferenceID(t *testing.T) {
	id, err := NewReferenceID()
	if err != nil {
		t.Fatal(err)
	}
	if id == (ReferenceID{}) || len(id.String()) != 32 {
		t.Fatalf("generated ID = %q", id)
	}
	parsed, err := ParseReferenceID(id.String())
	if err != nil || parsed != id {
		t.Fatalf("round trip = %v, %v", parsed, err)
	}
	for _, invalid := range []string{"", "00000000000000000000000000000000", "ABCDEF0123456789abcdef0123456789", " abcdef0123456789abcdef0123456789", "abcdef01-2345-6789-abcd-ef0123456789", "abcdef0123456789abcdef0123456789="} {
		if _, err := ParseReferenceID(invalid); !errors.Is(err, ErrInvalidReferenceID) {
			t.Errorf("ParseReferenceID(%q) = %v", invalid, err)
		}
	}
	original := randomReader
	randomReader = errorReader{err: io.ErrUnexpectedEOF}
	t.Cleanup(func() { randomReader = original })
	if _, err := NewReferenceID(); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("random failure = %v", err)
	}
}

func TestMEDIA001M2BLimitsAndReservations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	for _, limits := range []Limits{{}, {MaxReferences: 1, MaxLogicalBytes: 1}, {MaxReferences: 65536, MaxLogicalBytes: 1 << 40}} {
		node := newAttachmentNode(t, ctx, nil)
		store, err := Open(ctx, node, limits)
		if limits.MaxReferences == 0 {
			if err == nil {
				t.Fatal("zero limits unexpectedly accepted")
			}
			_ = node.Close()
			continue
		}
		if err != nil {
			t.Fatalf("valid limits %+v: %v", limits, err)
		}
		_ = store.Close()
		_ = node.Close()
	}
	for _, limits := range []Limits{{MaxReferences: 65537, MaxLogicalBytes: 1}, {MaxReferences: 1, MaxLogicalBytes: 0}, {MaxReferences: 1, MaxLogicalBytes: (1 << 40) + 1}} {
		node := newAttachmentNode(t, ctx, nil)
		if store, err := Open(ctx, node, limits); err == nil {
			_ = store.Close()
			t.Fatalf("invalid limits %+v accepted", limits)
		}
		_ = node.Close()
	}

	node := newAttachmentNode(t, ctx, nil)
	defer node.Close()
	store, err := Open(ctx, node, Limits{MaxReferences: 2, MaxLogicalBytes: 8})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	started := make(chan struct{})
	unblock := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		_, err := store.ImportPublic(ctx, referenceID(1), &blockingReader{started: started, unblock: unblock, data: []byte("a")}, 8)
		firstDone <- err
	}()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	probe := &countingReader{reader: bytes.NewReader([]byte("b"))}
	if _, err := store.ImportPublic(ctx, referenceID(2), probe, 1); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("overcommitted import = %v", err)
	}
	if probe.reads != 0 {
		t.Fatalf("quota failure consumed %d reads", probe.reads)
	}
	close(unblock)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}

	store.mu.Lock()
	store.logicalBytes = math.MaxInt64
	store.mu.Unlock()
	if _, err := store.ImportPublic(ctx, referenceID(3), bytes.NewReader([]byte("x")), 1); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("overflow reservation = %v", err)
	}

	capNode := newAttachmentNode(t, ctx, nil)
	defer capNode.Close()
	capStore, err := Open(ctx, capNode, Limits{MaxReferences: 1, MaxLogicalBytes: 16})
	if err != nil {
		t.Fatal(err)
	}
	defer capStore.Close()
	if _, err := capStore.ImportPublic(ctx, referenceID(10), bytes.NewReader([]byte("one")), 3); err != nil {
		t.Fatal(err)
	}
	refProbe := &countingReader{reader: bytes.NewReader([]byte("two"))}
	if _, err := capStore.ImportPublic(ctx, referenceID(11), refProbe, 3); !errors.Is(err, ErrQuotaExceeded) || refProbe.reads != 0 {
		t.Fatalf("reference quota = %v, reads=%d", err, refProbe.reads)
	}
}

func TestMEDIA001M2BImportsPinAndReopen(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "datastore")

	firstDB, err := leveldb.NewDatastore(dbPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = firstDB.Close() })
	firstNode := newAttachmentNode(t, ctx, firstDB)
	t.Cleanup(func() { _ = firstNode.Close() })
	firstStore, err := Open(ctx, firstNode, Limits{MaxReferences: 8, MaxLogicalBytes: 8 << 20})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = firstStore.Close() })
	contents := [][]byte{{}, []byte("one chunk"), bytes.Repeat([]byte("multi"), 300000)}
	references := make([]PublicReference, len(contents))
	for i, content := range contents {
		references[i], err = firstStore.ImportPublic(ctx, referenceID(byte(i+1)), bytes.NewReader(content), int64(len(content))+1)
		if err != nil {
			t.Fatalf("import %d: %v", i, err)
		}
		assertRecursiveDAGPinned(t, ctx, firstStore, firstNode, references[i].File.CID)
	}
	if err := firstStore.Close(); err != nil {
		t.Fatal(err)
	}
	if err := firstNode.Close(); err != nil {
		t.Fatal(err)
	}
	if err := firstDB.Close(); err != nil {
		t.Fatal(err)
	}

	secondDB, err := leveldb.NewDatastore(dbPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = secondDB.Close() })
	secondNode := newAttachmentNode(t, ctx, secondDB)
	t.Cleanup(func() { _ = secondNode.Close() })
	secondStore, err := Open(ctx, secondNode, Limits{MaxReferences: 8, MaxLogicalBytes: 8 << 20})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = secondStore.Close() })
	for i, want := range contents {
		got, err := secondStore.GetPublic(ctx, references[i].ID)
		if err != nil || got != references[i] {
			t.Fatalf("reopened reference %d = %+v, %v", i, got, err)
		}
		var copied bytes.Buffer
		if _, err := secondNode.CopyPublicFile(ctx, got.File, &copied); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(copied.Bytes(), want) {
			t.Fatalf("reopened bytes %d differ", i)
		}
	}
}

func TestMEDIA001M2BDuplicateReferencesAndIDRules(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	node := newAttachmentNode(t, ctx, nil)
	defer node.Close()
	store, err := Open(ctx, node, Limits{MaxReferences: 4, MaxLogicalBytes: 4})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	one, err := store.ImportPublic(ctx, referenceID(1), bytes.NewReader([]byte("same")), 4)
	if err != nil {
		t.Fatal(err)
	}
	two, err := store.RetainPublic(ctx, referenceID(2), one.File)
	if err != nil {
		t.Fatal(err)
	}
	if !one.File.CID.Equals(two.File.CID) || store.logicalBytes != 4 {
		t.Fatalf("duplicate root accounting: one=%s two=%s bytes=%d", one.File.CID, two.File.CID, store.logicalBytes)
	}
	retry := &countingReader{reader: bytes.NewReader([]byte("different"))}
	got, err := store.ImportPublic(ctx, one.ID, retry, 4)
	if err != nil || got != one || retry.reads != 0 {
		t.Fatalf("ID retry = %+v, %v, reads=%d", got, err, retry.reads)
	}
	if _, err := store.RetainPublic(ctx, one.ID, network.PublicFile{CID: one.File.CID, ByteLength: one.File.ByteLength + 1}); !errors.Is(err, ErrConflict) {
		t.Fatalf("descriptor conflict = %v", err)
	}
	if err := store.Release(ctx, one.ID); err != nil {
		t.Fatal(err)
	}
	assertRootPinned(t, ctx, store, two.File.CID, true)
	if _, err := store.GetPublic(ctx, one.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("released GetPublic = %v", err)
	}
	if err := store.Release(ctx, two.ID); err != nil {
		t.Fatal(err)
	}
	assertRootPinned(t, ctx, store, two.File.CID, false)
	if err := store.Release(ctx, two.ID); err != nil {
		t.Fatalf("absent release = %v", err)
	}
	three, err := store.RetainPublic(ctx, referenceID(3), one.File)
	if err != nil {
		t.Fatal(err)
	}
	four, err := store.RetainPublic(ctx, referenceID(4), one.File)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Release(ctx, four.ID); err != nil {
		t.Fatal(err)
	}
	assertRootPinned(t, ctx, store, three.File.CID, true)
	if err := store.Release(ctx, three.ID); err != nil {
		t.Fatal(err)
	}
	assertRootPinned(t, ctx, store, three.File.CID, false)
}

func TestMEDIA001M2BQuotaBoundariesAndInvalidFiles(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	node := newAttachmentNode(t, ctx, nil)
	defer node.Close()
	store, err := Open(ctx, node, Limits{MaxReferences: 2, MaxLogicalBytes: 10})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.ImportPublic(ctx, referenceID(1), bytes.NewReader([]byte("ab")), 2); err != nil {
		t.Fatal(err)
	}
	probe := &countingReader{reader: bytes.NewReader([]byte("c"))}
	if _, err := store.ImportPublic(ctx, referenceID(2), probe, 9); !errors.Is(err, ErrQuotaExceeded) || probe.reads != 0 {
		t.Fatalf("logical quota = %v, reads=%d", err, probe.reads)
	}
	if _, err := store.RetainPublic(ctx, ReferenceID{}, network.PublicFile{}); !errors.Is(err, ErrInvalidReferenceID) {
		t.Fatalf("zero ID = %v", err)
	}
	if _, err := store.RetainPublic(ctx, referenceID(2), network.PublicFile{}); !errors.Is(err, network.ErrInvalidPublicFile) {
		t.Fatalf("malformed descriptor = %v", err)
	}
	missing := network.PublicFile{CID: mustCID(t, []byte("missing")), ByteLength: 7}
	missingCtx, stopMissing := context.WithTimeout(ctx, 100*time.Millisecond)
	if _, err := store.RetainPublic(missingCtx, referenceID(2), missing); !errors.Is(err, context.DeadlineExceeded) {
		stopMissing()
		t.Fatalf("missing file = %v", err)
	}
	stopMissing()
	corruptCID := mustCID(t, []byte("expected"))
	corruptNode, err := merkledag.NewRawNodeWPrefix([]byte("expected"), corruptCID.Prefix())
	if err != nil {
		t.Fatal(err)
	}
	if err := node.Blockstore.Put(ctx, corruptNode); err != nil {
		t.Fatal(err)
	}
	corruptKey := datastore.NewKey("/blocks/" + corruptCID.Hash().B58String())
	results, err := node.Datastore.Query(ctx, query.Query{Prefix: "/blocks"})
	if err != nil {
		t.Fatal(err)
	}
	for entry := range results.Next() {
		if entry.Error == nil && bytes.Equal(entry.Value, corruptNode.RawData()) {
			corruptKey = datastore.NewKey(entry.Key)
			break
		}
	}
	if err := results.Close(); err != nil {
		t.Fatal(err)
	}
	if err := node.Datastore.Put(ctx, corruptKey, []byte("corrupt")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RetainPublic(ctx, referenceID(2), network.PublicFile{CID: corruptCID, ByteLength: 8}); !errors.Is(err, network.ErrInvalidPublicFile) {
		t.Fatalf("corrupt file = %v", err)
	}
	first, err := store.GetPublic(ctx, referenceID(1))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.RetainPublic(ctx, referenceID(2), network.PublicFile{CID: first.File.CID, ByteLength: 1}); !errors.Is(err, network.ErrInvalidPublicFile) {
		t.Fatalf("length mismatch = %v", err)
	}
}

func TestMEDIA001M2BTransitionRecoveryAndOrphanPins(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	node := newAttachmentNode(t, ctx, nil)
	defer node.Close()
	limits := Limits{MaxReferences: 8, MaxLogicalBytes: 1 << 20}

	store, err := Open(ctx, node, limits)
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("ready write failed")
	store.records = &faultDatastore{Batching: store.records, failPutState: stateReady, err: failure}
	ref, err := store.ImportPublic(ctx, referenceID(1), bytes.NewReader([]byte("recover retaining")), 32)
	if !errors.Is(err, failure) {
		t.Fatalf("ready failure = %+v, %v", ref, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, node, limits)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := store.GetPublic(ctx, referenceID(1))
	if err != nil {
		t.Fatal(err)
	}
	assertRootPinned(t, ctx, store, ready.File.CID, true)

	basePinner := store.pinner
	failure = errors.New("pin failed")
	store.pinner = &faultPinner{Pinner: basePinner, pinErr: failure}
	if _, err := store.ImportPublic(ctx, referenceID(3), bytes.NewReader([]byte("recover pin")), 16); !errors.Is(err, failure) {
		t.Fatalf("pin failure = %v", err)
	}
	store.pinner = basePinner
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, node, limits)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPublic(ctx, referenceID(3)); err != nil {
		t.Fatalf("pin recovery = %v", err)
	}

	basePinner = store.pinner
	failure = errors.New("pin flush failed")
	store.pinner = &faultPinner{Pinner: basePinner, flushErr: failure}
	if _, err := store.ImportPublic(ctx, referenceID(4), bytes.NewReader([]byte("recover flush")), 16); !errors.Is(err, failure) {
		t.Fatalf("pin flush failure = %v", err)
	}
	store.pinner = basePinner
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, node, limits)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPublic(ctx, referenceID(4)); err != nil {
		t.Fatalf("pin flush recovery = %v", err)
	}

	failure = errors.New("unpin failed")
	store.pinner = &faultPinner{Pinner: store.pinner, unpinErr: failure}
	if err := store.Release(ctx, ready.ID); !errors.Is(err, failure) {
		t.Fatalf("unpin failure = %v", err)
	}
	store.pinner = store.pinner.(*faultPinner).Pinner
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, node, limits)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPublic(ctx, ready.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("releasing recovery = %v", err)
	}

	orphan, err := store.ImportPublic(ctx, referenceID(2), bytes.NewReader([]byte("orphan")), 8)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.records.Delete(ctx, referenceKey(orphan.ID)); err != nil {
		t.Fatal(err)
	}
	if err := store.records.Sync(ctx, referencePrefix); err != nil {
		t.Fatal(err)
	}
	unrelated, err := node.ImportPublicFile(ctx, bytes.NewReader([]byte("unrelated pin")), 32)
	if err != nil {
		t.Fatal(err)
	}
	pinWithName(t, ctx, store.pinner, node, unrelated.CID, "unrelated-owner")
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, node, limits)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	assertRootPinned(t, ctx, store, orphan.File.CID, false)
	assertRootPinned(t, ctx, store, unrelated.CID, true)
}

func TestMEDIA001M2BReadyAfterReopenRequiresRecursivePin(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	node := newAttachmentNode(t, ctx, nil)
	defer node.Close()
	limits := Limits{MaxReferences: 2, MaxLogicalBytes: 1024}
	store, err := Open(ctx, node, limits)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := store.ImportPublic(ctx, referenceID(1), bytes.NewReader([]byte("must stay pinned")), 32)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.pinner.Unpin(ctx, ref.File.CID, true); err != nil {
		t.Fatal(err)
	}
	if err := store.pinner.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if reopened, err := Open(ctx, node, limits); !errors.Is(err, ErrCorruptState) {
		if reopened != nil {
			_ = reopened.Close()
		}
		t.Fatalf("ready unpinned reopen = %v", err)
	}
}

func TestMEDIA001M2BReadyPersistsRecursivePinAfterReopen(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	node := newAttachmentNode(t, ctx, nil)
	defer node.Close()
	limits := Limits{MaxReferences: 2, MaxLogicalBytes: 1024}
	store, err := Open(ctx, node, limits)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := store.ImportPublic(ctx, referenceID(2), bytes.NewReader([]byte("survives reopen")), 32)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, node, limits)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if got, err := store.GetPublic(ctx, reference.ID); err != nil || got != reference {
		t.Fatalf("ready reference after reopen = %+v, %v", got, err)
	}
	assertRootPinned(t, ctx, store, reference.File.CID, true)
}

func TestMEDIA001M2BDurableStoreFailureOrdering(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	node := newAttachmentNode(t, ctx, nil)
	defer node.Close()
	limits := Limits{MaxReferences: 8, MaxLogicalBytes: 1 << 20}
	store, err := Open(ctx, node, limits)
	if err != nil {
		t.Fatal(err)
	}

	baseRecords := store.records
	failure := errors.New("retaining put failed")
	store.records = &faultDatastore{Batching: baseRecords, failPutState: stateRetaining, err: failure}
	if _, err := store.ImportPublic(ctx, referenceID(5), bytes.NewReader([]byte("put")), 8); !errors.Is(err, failure) {
		t.Fatalf("retaining put failure = %v", err)
	}
	store.records = baseRecords
	if _, err := store.GetPublic(ctx, referenceID(5)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("failed retaining put became visible: %v", err)
	}

	failure = errors.New("retaining sync failed")
	store.records = &faultDatastore{Batching: baseRecords, failSyncState: stateRetaining, err: failure}
	if _, err := store.ImportPublic(ctx, referenceID(6), bytes.NewReader([]byte("sync retaining")), 32); !errors.Is(err, failure) {
		t.Fatalf("retaining sync failure = %v", err)
	}
	store.records = baseRecords
	if _, err := store.GetPublic(ctx, referenceID(6)); !errors.Is(err, ErrNotReady) {
		t.Fatalf("unsynced retaining reference = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, node, limits)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPublic(ctx, referenceID(6)); err != nil {
		t.Fatalf("retaining sync recovery = %v", err)
	}

	baseRecords = store.records
	failure = errors.New("ready sync failed")
	store.records = &faultDatastore{Batching: baseRecords, failSyncState: stateReady, err: failure}
	if _, err := store.ImportPublic(ctx, referenceID(7), bytes.NewReader([]byte("sync ready")), 32); !errors.Is(err, failure) {
		t.Fatalf("ready sync failure = %v", err)
	}
	store.records = baseRecords
	if _, err := store.GetPublic(ctx, referenceID(7)); !errors.Is(err, ErrNotReady) {
		t.Fatalf("unsynced ready reference = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, node, limits)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPublic(ctx, referenceID(7)); err != nil {
		t.Fatalf("ready sync recovery = %v", err)
	}

	baseRecords = store.records
	failure = errors.New("releasing put failed")
	store.records = &faultDatastore{Batching: baseRecords, failPutState: stateReleasing, err: failure}
	if err := store.Release(ctx, referenceID(6)); !errors.Is(err, failure) {
		t.Fatalf("releasing put failure = %v", err)
	}
	store.records = baseRecords
	if _, err := store.GetPublic(ctx, referenceID(6)); err != nil {
		t.Fatalf("failed releasing put hid ready reference: %v", err)
	}

	failure = errors.New("releasing sync failed")
	store.records = &faultDatastore{Batching: baseRecords, failSyncState: stateReleasing, err: failure}
	if err := store.Release(ctx, referenceID(6)); !errors.Is(err, failure) {
		t.Fatalf("releasing sync failure = %v", err)
	}
	store.records = baseRecords
	if _, err := store.GetPublic(ctx, referenceID(6)); !errors.Is(err, ErrNotReady) {
		t.Fatalf("releasing transition remained usable: %v", err)
	}
	if err := store.Release(ctx, referenceID(6)); err != nil {
		t.Fatalf("releasing retry = %v", err)
	}

	failure = errors.New("delete failed")
	store.records = &faultDatastore{Batching: baseRecords, failDelete: true, err: failure}
	if err := store.Release(ctx, referenceID(7)); !errors.Is(err, failure) {
		t.Fatalf("delete failure = %v", err)
	}
	store.records = baseRecords
	if _, err := store.GetPublic(ctx, referenceID(7)); !errors.Is(err, ErrNotReady) {
		t.Fatalf("failed delete remained usable: %v", err)
	}
	if err := store.Release(ctx, referenceID(7)); err != nil {
		t.Fatalf("delete retry = %v", err)
	}

	ref, err := store.ImportPublic(ctx, referenceID(8), bytes.NewReader([]byte("delete sync")), 32)
	if err != nil {
		t.Fatal(err)
	}
	failure = errors.New("delete sync failed")
	store.records = &faultDatastore{Batching: baseRecords, failSyncState: mutationDelete, err: failure}
	if err := store.Release(ctx, ref.ID); !errors.Is(err, failure) {
		t.Fatalf("delete sync failure = %v", err)
	}
	store.records = baseRecords
	if err := store.Release(ctx, ref.ID); err != nil {
		t.Fatalf("delete sync retry = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMEDIA001M2BSharedReleaseSyncFailure(t *testing.T) {
	for _, test := range []struct {
		name                     string
		restoreDeletedOnSyncFail bool
	}{
		{name: "delete_durable"},
		{name: "delete_not_durable", restoreDeletedOnSyncFail: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
			defer cancel()
			node := newAttachmentNode(t, ctx, nil)
			t.Cleanup(func() { _ = node.Close() })
			limits := Limits{MaxReferences: 4, MaxLogicalBytes: 1024}
			store, err := Open(ctx, node, limits)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = store.Close() })

			released, err := store.ImportPublic(ctx, referenceID(21), bytes.NewReader([]byte("shared release")), 32)
			if err != nil {
				t.Fatal(err)
			}
			survivor, err := store.RetainPublic(ctx, referenceID(22), released.File)
			if err != nil {
				t.Fatal(err)
			}

			baseRecords := store.records
			failure := errors.New("shared delete sync failed")
			store.records = &faultDatastore{
				Batching:                    baseRecords,
				failSyncState:               mutationDelete,
				restoreDeletedOnSyncFailure: test.restoreDeletedOnSyncFail,
				err:                         failure,
			}
			if err := store.Release(ctx, released.ID); !errors.Is(err, failure) {
				t.Fatalf("shared release sync failure = %v", err)
			}
			if _, err := store.GetPublic(ctx, released.ID); !errors.Is(err, ErrNotReady) {
				t.Fatalf("uncertain shared release remained visible: %v", err)
			}
			if _, err := store.RetainPublic(ctx, released.ID, released.File); !errors.Is(err, ErrNotReady) {
				t.Fatalf("uncertain shared retain retry = %v", err)
			}
			probe := &countingReader{reader: bytes.NewReader([]byte("shared release"))}
			if _, err := store.ImportPublic(ctx, released.ID, probe, 32); !errors.Is(err, ErrNotReady) || probe.reads != 0 {
				t.Fatalf("uncertain shared import retry = %v, reads=%d", err, probe.reads)
			}
			if got, err := store.GetPublic(ctx, survivor.ID); err != nil || got != survivor {
				t.Fatalf("surviving reference = %+v, %v", got, err)
			}
			assertRootPinned(t, ctx, store, survivor.File.CID, true)

			store.records = baseRecords
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			store, err = Open(ctx, node, limits)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.GetPublic(ctx, released.ID); !errors.Is(err, ErrNotFound) {
				t.Fatalf("released reference after reopen = %v", err)
			}
			if err := store.Release(ctx, released.ID); err != nil {
				t.Fatalf("release retry after reopen = %v", err)
			}
			if got, err := store.GetPublic(ctx, survivor.ID); err != nil || got != survivor {
				t.Fatalf("surviving reference after reopen = %+v, %v", got, err)
			}
			assertRootPinned(t, ctx, store, survivor.File.CID, true)
		})
	}
}

func TestMEDIA001M2BCancellationCloseAndConcurrentAccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	node := newAttachmentNode(t, ctx, nil)
	defer node.Close()
	store, err := Open(ctx, node, Limits{MaxReferences: 32, MaxLogicalBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	if _, err := store.ImportPublic(cancelled, referenceID(1), bytes.NewReader([]byte("x")), 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled import = %v", err)
	}
	var typedNilReader *bytes.Reader
	if _, err := store.ImportPublic(ctx, referenceID(1), typedNilReader, 1); !errors.Is(err, network.ErrInvalidPublicFile) {
		t.Fatalf("typed-nil reader = %v", err)
	}
	importCtx, cancelImport := context.WithCancel(ctx)
	cancelSource := &cancellableReader{firstRead: make(chan struct{}), secondRead: make(chan struct{}), release: make(chan struct{})}
	importDone := make(chan error, 1)
	go func() {
		_, err := store.ImportPublic(importCtx, referenceID(1), cancelSource, 32)
		importDone <- err
	}()
	select {
	case <-cancelSource.secondRead:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	cancelImport()
	close(cancelSource.release)
	if err := <-importDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("in-flight cancelled import = %v", err)
	}
	if _, err := store.GetPublic(ctx, referenceID(1)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cancelled import became visible: %v", err)
	}
	if _, err := store.RetainPublic(nil, referenceID(1), network.PublicFile{}); err == nil {
		t.Fatal("nil context accepted")
	}

	base, err := store.ImportPublic(ctx, referenceID(2), bytes.NewReader([]byte("concurrent")), 16)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n byte) {
			defer wg.Done()
			id := referenceID(10 + n)
			if _, err := store.RetainPublic(ctx, id, base.File); err != nil {
				t.Errorf("retain %d: %v", n, err)
				return
			}
			if _, err := store.GetPublic(ctx, id); err != nil {
				t.Errorf("get %d: %v", n, err)
			}
			if err := store.Release(ctx, id); err != nil {
				t.Errorf("release %d: %v", n, err)
			}
		}(byte(i))
	}
	wg.Wait()
	type importResult struct {
		reference PublicReference
		err       error
	}
	startImports := make(chan struct{})
	importResults := make(chan importResult, 2)
	for _, id := range []ReferenceID{referenceID(20), referenceID(21)} {
		go func(id ReferenceID) {
			<-startImports
			reference, err := store.ImportPublic(ctx, id, bytes.NewReader([]byte("identical concurrent import")), 32)
			importResults <- importResult{reference: reference, err: err}
		}(id)
	}
	close(startImports)
	firstImport := <-importResults
	secondImport := <-importResults
	if firstImport.err != nil || secondImport.err != nil || !firstImport.reference.File.CID.Equals(secondImport.reference.File.CID) {
		t.Fatalf("concurrent identical imports = %+v, %+v", firstImport, secondImport)
	}
	store.mu.Lock()
	duplicateRootCount := store.roots[fileRootKey(firstImport.reference.File)]
	store.mu.Unlock()
	if duplicateRootCount != 2 {
		t.Fatalf("concurrent duplicate root count = %d", duplicateRootCount)
	}
	unretained, err := node.ImportPublicFile(ctx, bytes.NewReader([]byte("close cancels pin")), 32)
	if err != nil {
		t.Fatal(err)
	}
	blocking := &blockingPinPinner{Pinner: store.pinner, started: make(chan struct{})}
	store.pinner = blocking
	retainDone := make(chan error, 1)
	go func() {
		_, err := store.RetainPublic(ctx, referenceID(30), unretained)
		retainDone <- err
	}()
	select {
	case <-blocking.started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	closeDone := make(chan error, 1)
	go func() { closeDone <- store.Close() }()
	if err := <-retainDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("close-cancelled retain = %v", err)
	}
	if err := <-closeDone; err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPublic(ctx, base.ID); !errors.Is(err, ErrClosed) {
		t.Fatalf("post-close GetPublic = %v", err)
	}
	if err := store.Release(ctx, base.ID); !errors.Is(err, ErrClosed) {
		t.Fatalf("post-close Release = %v", err)
	}
}

func TestMEDIA001M2BRecordCodecAndStoreErrors(t *testing.T) {
	record := persistedRecord{Version: recordVersion, State: stateReady, ID: referenceID(1), CID: mustCID(t, []byte("record")), ByteLength: 6}
	encoded, err := encodeRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeRecord(encoded)
	if err != nil || decoded != record {
		t.Fatalf("record round trip = %+v, %v", decoded, err)
	}
	for _, malformed := range [][]byte{
		nil,
		[]byte(`{"version":2,"state":"ready","id":"01010101010101010101010101010101","cid":"bafkqaaa","byteLength":0}`),
		append(append([]byte(nil), encoded...), 'x'),
		[]byte(`{"version":1,"state":"ready","id":"01010101010101010101010101010101","cid":"bafkqaaa","byteLength":0,"extra":1}`),
	} {
		if _, err := decodeRecord(malformed); !errors.Is(err, ErrCorruptState) {
			t.Errorf("decode %q = %v", malformed, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	node := newAttachmentNode(t, ctx, nil)
	defer node.Close()
	badKey := referenceKey(referenceID(9))
	badValue := []byte(`{"version":1}`)
	if err := node.Datastore.Put(ctx, badKey, badValue); err != nil {
		t.Fatal(err)
	}
	if err := node.Datastore.Sync(ctx, referencePrefix); err != nil {
		t.Fatal(err)
	}
	if store, err := Open(ctx, node, Limits{MaxReferences: 2, MaxLogicalBytes: 1024}); !errors.Is(err, ErrCorruptState) {
		if store != nil {
			_ = store.Close()
		}
		t.Fatalf("malformed persisted state = %v", err)
	}
	stillBad, err := node.Datastore.Get(ctx, badKey)
	if err != nil || !bytes.Equal(stillBad, badValue) {
		t.Fatalf("malformed state mutated: %q, %v", stillBad, err)
	}
}

func TestMEDIA001M2BCorruptPinnerMarker(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	markerKey := pinnerPrefix.Child(datastore.NewKey("/pins/state/dirty"))
	limits := Limits{MaxReferences: 2, MaxLogicalBytes: 1024}

	for _, test := range []struct {
		name  string
		value []byte
	}{
		{name: "empty", value: []byte{}},
		{name: "unknown_value", value: []byte{2}},
		{name: "extra_bytes", value: []byte{0, 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			node := newAttachmentNode(t, ctx, nil)
			t.Cleanup(func() { _ = node.Close() })
			if err := node.Datastore.Put(ctx, markerKey, test.value); err != nil {
				t.Fatal(err)
			}
			if store, err := Open(ctx, node, limits); !errors.Is(err, ErrCorruptState) {
				if store != nil {
					_ = store.Close()
				}
				t.Fatalf("corrupt pinner marker = %v", err)
			}
			got, err := node.Datastore.Get(ctx, markerKey)
			if err != nil || !bytes.Equal(got, test.value) {
				t.Fatalf("corrupt pinner marker mutated: %v, %v", got, err)
			}
		})
	}

	for _, test := range []struct {
		name  string
		value byte
	}{
		{name: "clean", value: 0},
		{name: "dirty", value: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			node := newAttachmentNode(t, ctx, nil)
			t.Cleanup(func() { _ = node.Close() })
			if err := node.Datastore.Put(ctx, markerKey, []byte{test.value}); err != nil {
				t.Fatal(err)
			}
			store, err := Open(ctx, node, limits)
			if err != nil {
				t.Fatalf("valid pinner marker %d: %v", test.value, err)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMEDIA001M2BRecordNullScalar(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	node := newAttachmentNode(t, ctx, nil)
	t.Cleanup(func() { _ = node.Close() })
	empty, err := node.ImportPublicFile(ctx, bytes.NewReader(nil), 1)
	if err != nil {
		t.Fatal(err)
	}
	record := persistedRecord{Version: recordVersion, State: stateReady, ID: referenceID(31), CID: empty.CID, ByteLength: 0}
	encoded, err := encodeRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	if decoded, err := decodeRecord(encoded); err != nil || decoded != record {
		t.Fatalf("numeric zero control = %+v, %v", decoded, err)
	}
	for _, field := range []struct {
		name string
		old  []byte
	}{
		{name: "version", old: []byte(`"version":1`)},
		{name: "state", old: []byte(`"state":"ready"`)},
		{name: "id", old: []byte(`"id":"` + record.ID.String() + `"`)},
		{name: "cid", old: []byte(`"cid":"` + record.CID.String() + `"`)},
		{name: "byteLength", old: []byte(`"byteLength":0`)},
	} {
		t.Run(field.name, func(t *testing.T) {
			malformed := bytes.Replace(encoded, field.old, []byte(`"`+field.name+`":null`), 1)
			if bytes.Equal(malformed, encoded) {
				t.Fatalf("field %q was not replaced in %s", field.name, encoded)
			}
			if _, err := decodeRecord(malformed); !errors.Is(err, ErrCorruptState) {
				t.Fatalf("null %s = %v", field.name, err)
			}
		})
	}
}

func newAttachmentNode(t *testing.T, ctx context.Context, store datastore.Batching) *network.Node {
	t.Helper()
	if store == nil {
		store = dsync.MutexWrap(datastore.NewMapDatastore())
	}
	node, err := network.New(ctx, network.Config{
		Datastore:             store,
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return node
}

func referenceID(value byte) ReferenceID {
	var id ReferenceID
	for i := range id {
		id[i] = value
	}
	return id
}

func mustCID(t *testing.T, data []byte) cid.Cid {
	t.Helper()
	id, err := cid.Prefix{Version: 1, Codec: cid.Raw, MhType: 0x12, MhLength: 32}.Sum(data)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func assertRootPinned(t *testing.T, ctx context.Context, store *Store, id cid.Cid, want bool) {
	t.Helper()
	_, got, err := store.pinner.IsPinnedWithType(ctx, id, pinning.Recursive)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("recursive pin %s = %t, want %t", id, got, want)
	}
}

func assertRecursiveDAGPinned(t *testing.T, ctx context.Context, store *Store, node *network.Node, root cid.Cid) {
	t.Helper()
	seen := make(map[string]struct{})
	var walk func(cid.Cid, bool)
	walk = func(id cid.Cid, isRoot bool) {
		if _, visited := seen[id.KeyString()]; visited {
			return
		}
		seen[id.KeyString()] = struct{}{}
		if isRoot {
			assertRootPinned(t, ctx, store, id, true)
		} else {
			_, pinned, err := store.pinner.IsPinned(ctx, id)
			if err != nil || !pinned {
				t.Fatalf("child pin %s = %t, %v", id, pinned, err)
			}
		}
		if id.Prefix().Codec != cid.DagProtobuf {
			return
		}
		block, err := node.Blockstore.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		dagNode, err := merkledag.DecodeProtobuf(block.RawData())
		if err != nil {
			t.Fatal(err)
		}
		for _, link := range dagNode.Links() {
			walk(link.Cid, false)
		}
	}
	walk(root, true)
}

func pinWithName(t *testing.T, ctx context.Context, pinner pinning.Pinner, node *network.Node, root cid.Cid, name string) {
	t.Helper()
	block, err := node.Blockstore.Get(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	var rootNode format.Node
	if root.Prefix().Codec == cid.Raw {
		rootNode, err = merkledag.DecodeRawBlock(block)
	} else {
		rootNode, err = merkledag.DecodeProtobufBlock(block)
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := pinner.Pin(ctx, rootNode, true, name); err != nil {
		t.Fatal(err)
	}
	if err := pinner.Flush(ctx); err != nil {
		t.Fatal(err)
	}
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

type countingReader struct {
	reader io.Reader
	reads  int
}

func (r *countingReader) Read(p []byte) (int, error) {
	r.reads++
	return r.reader.Read(p)
}

type blockingReader struct {
	started chan struct{}
	once    sync.Once
	unblock <-chan struct{}
	data    []byte
	done    bool
}

type cancellableReader struct {
	firstRead  chan struct{}
	secondRead chan struct{}
	release    chan struct{}
	once       sync.Once
	calls      int
}

func (r *cancellableReader) Read(p []byte) (int, error) {
	r.calls++
	if r.calls == 1 {
		r.once.Do(func() { close(r.firstRead) })
		p[0] = 'x'
		return 1, nil
	}
	if r.calls == 2 {
		close(r.secondRead)
		<-r.release
		return 0, nil
	}
	return 0, io.EOF
}

func (r *blockingReader) Read(p []byte) (int, error) {
	r.once.Do(func() { close(r.started) })
	<-r.unblock
	if r.done {
		return 0, io.EOF
	}
	r.done = true
	return copy(p, r.data), nil
}

type faultDatastore struct {
	datastore.Batching
	failPutState                string
	failSyncState               string
	failDelete                  bool
	restoreDeletedOnSyncFailure bool
	lastMutation                string
	deletedKey                  datastore.Key
	deletedValue                []byte
	err                         error
}

func (s *faultDatastore) Put(ctx context.Context, key datastore.Key, value []byte) error {
	if s.failPutState != "" && bytes.Contains(value, []byte(`"state":"`+s.failPutState+`"`)) {
		state := s.failPutState
		s.failPutState = ""
		return errors.Join(s.err, errors.New("put "+state))
	}
	if err := s.Batching.Put(ctx, key, value); err != nil {
		return err
	}
	for _, state := range []string{stateRetaining, stateReady, stateReleasing} {
		if bytes.Contains(value, []byte(`"state":"`+state+`"`)) {
			s.lastMutation = state
		}
	}
	return nil
}

const mutationDelete = "delete"

func (s *faultDatastore) Delete(ctx context.Context, key datastore.Key) error {
	if s.failDelete {
		s.failDelete = false
		return errors.Join(s.err, errors.New("delete"))
	}
	if s.restoreDeletedOnSyncFailure {
		value, err := s.Batching.Get(ctx, key)
		if err != nil {
			return err
		}
		s.deletedKey = key
		s.deletedValue = append([]byte(nil), value...)
	}
	if err := s.Batching.Delete(ctx, key); err != nil {
		return err
	}
	s.lastMutation = mutationDelete
	return nil
}

func (s *faultDatastore) Sync(ctx context.Context, prefix datastore.Key) error {
	if s.failSyncState != "" && s.failSyncState == s.lastMutation {
		state := s.failSyncState
		s.failSyncState = ""
		if state == mutationDelete && s.restoreDeletedOnSyncFailure {
			if err := s.Batching.Put(ctx, s.deletedKey, s.deletedValue); err != nil {
				return errors.Join(s.err, errors.New("restoring unsynced delete"), err)
			}
		}
		return errors.Join(s.err, errors.New("sync "+state))
	}
	return s.Batching.Sync(ctx, prefix)
}

type faultPinner struct {
	pinning.Pinner
	unpinErr error
	pinErr   error
	flushErr error
}

func (p *faultPinner) Pin(ctx context.Context, node format.Node, recursive bool, name string) error {
	if p.pinErr != nil {
		err := p.pinErr
		p.pinErr = nil
		return err
	}
	return p.Pinner.Pin(ctx, node, recursive, name)
}

func (p *faultPinner) Unpin(ctx context.Context, id cid.Cid, recursive bool) error {
	if p.unpinErr != nil {
		err := p.unpinErr
		p.unpinErr = nil
		return err
	}
	return p.Pinner.Unpin(ctx, id, recursive)
}

func (p *faultPinner) Flush(ctx context.Context) error {
	if p.flushErr != nil {
		err := p.flushErr
		p.flushErr = nil
		return err
	}
	return p.Pinner.Flush(ctx)
}

type blockingPinPinner struct {
	pinning.Pinner
	started chan struct{}
	once    sync.Once
}

func (p *blockingPinPinner) Pin(ctx context.Context, _ format.Node, _ bool, _ string) error {
	p.once.Do(func() { close(p.started) })
	<-ctx.Done()
	return ctx.Err()
}

var _ format.Node = (*merkledag.ProtoNode)(nil)
