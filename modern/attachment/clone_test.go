package attachment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	pinning "github.com/ipfs/boxo/pinning/pinner"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/larslarsen/bb-go/modern/network"
)

func TestMEDIA001M1BClonePublicIndependentRetention(t *testing.T) {
	ctx := context.Background()
	node := newAttachmentNode(t, ctx, nil)
	defer node.Close()
	store, err := Open(ctx, node, Limits{MaxReferences: 16, MaxLogicalBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	content := bytes.Repeat([]byte("post-owned-retention"), 1024)
	upload, err := store.ImportPublic(ctx, referenceID(1), bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	target := referenceID(2)
	cloned, err := store.ClonePublic(ctx, upload.ID, target, upload.File)
	if err != nil {
		t.Fatal(err)
	}
	if cloned.ID != target || cloned.File != upload.File {
		t.Fatalf("clone=%+v upload=%+v", cloned, upload)
	}
	if err := store.Release(ctx, upload.ID); err != nil {
		t.Fatal(err)
	}
	if retry, err := store.ClonePublic(ctx, upload.ID, target, upload.File); err != nil || retry != cloned {
		t.Fatalf("idempotent clone=%+v err=%v", retry, err)
	}
	var copied bytes.Buffer
	if _, err := node.CopyPublicFile(ctx, cloned.File, &copied); err != nil || !bytes.Equal(copied.Bytes(), content) {
		t.Fatalf("copy after upload release bytes=%d err=%v", copied.Len(), err)
	}
	if !store.IsForNode(node) || store.IsForNode(nil) {
		t.Fatal("node ownership check failed")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if store.IsForNode(node) {
		t.Fatal("closed store still reports node ownership")
	}
}

func TestMEDIA001M1BPrivateClaimErrors(t *testing.T) {
	ctx := context.Background()
	node := newAttachmentNode(t, ctx, nil)
	defer node.Close()
	store, err := Open(ctx, node, Limits{MaxReferences: 8, MaxLogicalBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	sourceID, targetID := referenceID(201), referenceID(202)
	file := network.PublicFile{CID: mustCID(t, []byte("missing private source")), ByteLength: 22}
	_, err = store.ClonePublic(ctx, sourceID, targetID, file)
	if err == nil {
		t.Fatal("missing private source accepted")
	}
	for _, secret := range []string{sourceID.String(), targetID.String()} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("clone error disclosed private reference %s: %v", secret, err)
		}
	}

	ready, err := store.ImportPublic(ctx, sourceID, bytes.NewReader([]byte("private conflict")), 64)
	if err != nil {
		t.Fatal(err)
	}
	occupied, err := store.ImportPublic(ctx, targetID, bytes.NewReader([]byte("other private file")), 64)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.ClonePublic(ctx, sourceID, targetID, ready.File)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("occupied target error=%v", err)
	}
	for _, secret := range []string{sourceID.String(), targetID.String(), occupied.ID.String()} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("conflict error disclosed private reference %s: %v", secret, err)
		}
	}
	writeTarget := referenceID(203)
	failure := errors.New("controlled clone record write")
	baseRecords := store.records
	store.records = &faultDatastore{Batching: baseRecords, failPutState: stateRetaining, err: failure}
	_, err = store.ClonePublic(ctx, sourceID, writeTarget, ready.File)
	store.records = baseRecords
	if !errors.Is(err, failure) {
		t.Fatalf("clone write error=%v", err)
	}
	if strings.Contains(err.Error(), sourceID.String()) || strings.Contains(err.Error(), writeTarget.String()) {
		t.Fatalf("clone write error disclosed private reference: %v", err)
	}
}

func TestMEDIA001M1BClonePublicValidationAndRace(t *testing.T) {
	ctx := context.Background()
	node := newAttachmentNode(t, ctx, nil)
	defer node.Close()
	store, err := Open(ctx, node, Limits{MaxReferences: 64, MaxLogicalBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	upload, err := store.ImportPublic(ctx, referenceID(3), bytes.NewReader([]byte("race-source")), 64)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClonePublic(nil, upload.ID, referenceID(4), upload.File); err == nil {
		t.Fatal("nil context accepted")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.ClonePublic(cancelled, upload.ID, referenceID(4), upload.File); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled clone error=%v", err)
	}
	if _, err := store.ClonePublic(ctx, upload.ID, upload.ID, upload.File); !errors.Is(err, ErrInvalidReferenceID) {
		t.Fatalf("same source/target error=%v", err)
	}
	if _, err := store.ClonePublic(ctx, referenceID(9), referenceID(4), upload.File); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing source error=%v", err)
	}
	wrong := network.PublicFile{CID: upload.File.CID, ByteLength: upload.File.ByteLength + 1}
	if _, err := store.ClonePublic(ctx, upload.ID, referenceID(4), wrong); !errors.Is(err, ErrConflict) {
		t.Fatalf("mismatched expected error=%v", err)
	}

	for iteration := byte(10); iteration < 30; iteration += 2 {
		sourceID, targetID := referenceID(iteration), referenceID(iteration+1)
		source, err := store.RetainPublic(ctx, sourceID, upload.File)
		if err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		var cloneErr, releaseErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, cloneErr = store.ClonePublic(ctx, sourceID, targetID, source.File)
		}()
		go func() {
			defer wg.Done()
			<-start
			releaseErr = store.Release(ctx, sourceID)
		}()
		close(start)
		wg.Wait()
		if releaseErr != nil {
			t.Fatalf("release race error=%v", releaseErr)
		}
		if cloneErr == nil {
			if got, err := store.GetPublic(ctx, targetID); err != nil || got.File != upload.File {
				t.Fatalf("successful raced clone=%+v err=%v", got, err)
			}
		} else if !errors.Is(cloneErr, ErrNotFound) {
			t.Fatalf("raced clone error=%v", cloneErr)
		}
	}
}

func TestMEDIA001M1BClonePublicRejectsUnavailableSource(t *testing.T) {
	ctx := context.Background()
	node := newAttachmentNode(t, ctx, nil)
	defer node.Close()
	store, err := Open(ctx, node, Limits{MaxReferences: 2, MaxLogicalBytes: 32})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	file := network.PublicFile{CID: mustCID(t, []byte("absent")), ByteLength: 6}
	if _, err := store.ClonePublic(ctx, referenceID(1), referenceID(2), file); !errors.Is(err, ErrNotFound) {
		t.Fatalf("absent source error=%v", err)
	}
	if _, err := store.GetPublic(ctx, referenceID(2)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("failed clone created target: %v", err)
	}
	present, err := node.Blockstore.Has(ctx, file.CID)
	if err != nil || present {
		t.Fatalf("absent root present=%t err=%v", present, err)
	}
}

func TestMEDIA001M1BCloneLastSourceInterleaving(t *testing.T) {
	for iteration := byte(1); iteration <= 16; iteration++ {
		t.Run(fmt.Sprintf("iteration-%d", iteration), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			node := newAttachmentNode(t, ctx, nil)
			defer node.Close()
			store, err := Open(ctx, node, Limits{MaxReferences: 4, MaxLogicalBytes: 1 << 20})
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			sourceID, targetID := referenceID(iteration), referenceID(iteration+100)
			source, err := store.ImportPublic(ctx, sourceID, bytes.NewReader(bytes.Repeat([]byte{iteration}, 1000)), 4096)
			if err != nil {
				t.Fatal(err)
			}
			start := make(chan struct{})
			cloneDone, releaseDone := make(chan error, 1), make(chan error, 1)
			go func() {
				<-start
				_, err := store.ClonePublic(ctx, sourceID, targetID, source.File)
				cloneDone <- err
			}()
			go func() {
				<-start
				releaseDone <- store.Release(ctx, sourceID)
			}()
			close(start)
			var cloneErr, releaseErr error
			for completed := 0; completed < 2; completed++ {
				select {
				case cloneErr = <-cloneDone:
					cloneDone = nil
				case releaseErr = <-releaseDone:
					releaseDone = nil
				case <-ctx.Done():
					t.Fatalf("interleaving did not complete: %v", ctx.Err())
				}
			}
			if releaseErr != nil {
				t.Fatalf("release error=%v", releaseErr)
			}
			_, pinned, err := store.recursivePinName(ctx, source.File.CID)
			if err != nil {
				t.Fatal(err)
			}
			if cloneErr == nil {
				if _, err := store.GetPublic(ctx, targetID); err != nil || !pinned {
					t.Fatalf("successful clone target error=%v pinned=%t", err, pinned)
				}
			} else {
				if !errors.Is(cloneErr, ErrNotFound) || pinned {
					t.Fatalf("failed clone error=%v pinned=%t", cloneErr, pinned)
				}
				if _, err := store.GetPublic(ctx, targetID); !errors.Is(err, ErrNotFound) {
					t.Fatalf("failed clone target error=%v", err)
				}
			}
		})
	}
}

func TestMEDIA001M1BCloneCloseRace(t *testing.T) {
	for iteration := byte(1); iteration <= 8; iteration++ {
		t.Run(fmt.Sprintf("iteration-%d", iteration), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			node := newAttachmentNode(t, ctx, nil)
			defer node.Close()
			store, err := Open(ctx, node, Limits{MaxReferences: 4, MaxLogicalBytes: 1 << 20})
			if err != nil {
				t.Fatal(err)
			}
			source, err := store.ImportPublic(ctx, referenceID(iteration), bytes.NewReader(bytes.Repeat([]byte{iteration}, 1000)), 4096)
			if err != nil {
				t.Fatal(err)
			}
			start := make(chan struct{})
			cloneDone, closeDone := make(chan error, 1), make(chan error, 1)
			go func() {
				<-start
				_, err := store.ClonePublic(ctx, source.ID, referenceID(iteration+200), source.File)
				cloneDone <- err
			}()
			go func() {
				<-start
				closeDone <- store.Close()
			}()
			close(start)
			var cloneErr, closeErr error
			for completed := 0; completed < 2; completed++ {
				select {
				case cloneErr = <-cloneDone:
					cloneDone = nil
				case closeErr = <-closeDone:
					closeDone = nil
				case <-ctx.Done():
					t.Fatalf("clone/close interleaving did not complete: %v", ctx.Err())
				}
			}
			if closeErr != nil {
				t.Fatalf("close error=%v", closeErr)
			}
			if cloneErr != nil && !errors.Is(cloneErr, ErrClosed) {
				t.Fatalf("clone error=%v", cloneErr)
			}
		})
	}
}

func TestMEDIA001M1BCloneFailureOrdering(t *testing.T) {
	ctx := context.Background()
	node := newAttachmentNode(t, ctx, nil)
	defer node.Close()
	limits := Limits{MaxReferences: 16, MaxLogicalBytes: 1 << 20}
	store, err := Open(ctx, node, limits)
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.ImportPublic(ctx, referenceID(180), bytes.NewReader(bytes.Repeat([]byte("clone fault"), 100)), 4096)
	if err != nil {
		t.Fatal(err)
	}
	baseRecords := store.records

	putFailure := errors.New("controlled clone retaining Put failure")
	store.records = &faultDatastore{Batching: baseRecords, failPutState: stateRetaining, err: putFailure}
	if _, err := store.ClonePublic(ctx, source.ID, referenceID(181), source.File); !errors.Is(err, putFailure) {
		t.Fatalf("retaining Put error=%v", err)
	}
	store.records = baseRecords
	if _, err := store.GetPublic(ctx, referenceID(181)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("failed retaining Put target=%v", err)
	}

	syncFailure := errors.New("controlled clone retaining Sync failure")
	store.records = &faultDatastore{Batching: baseRecords, failSyncState: stateRetaining, err: syncFailure}
	if _, err := store.ClonePublic(ctx, source.ID, referenceID(182), source.File); !errors.Is(err, syncFailure) {
		t.Fatalf("retaining Sync error=%v", err)
	}
	store.records = baseRecords
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, node, limits)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPublic(ctx, referenceID(182)); err != nil {
		t.Fatalf("retaining Sync recovery=%v", err)
	}
	baseRecords = store.records

	readyPutFailure := errors.New("controlled clone ready Put failure")
	store.records = &faultDatastore{Batching: baseRecords, failPutState: stateReady, err: readyPutFailure}
	if _, err := store.ClonePublic(ctx, source.ID, referenceID(183), source.File); !errors.Is(err, readyPutFailure) {
		t.Fatalf("ready Put error=%v", err)
	}
	store.records = baseRecords
	if _, err := store.GetPublic(ctx, referenceID(183)); !errors.Is(err, ErrNotReady) {
		t.Fatalf("ready-Put-failed clone visibility=%v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, node, limits)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPublic(ctx, referenceID(183)); err != nil {
		t.Fatalf("ready Put recovery=%v", err)
	}
	baseRecords = store.records

	readyFailure := errors.New("controlled clone ready Sync failure")
	store.records = &faultDatastore{Batching: baseRecords, failSyncState: stateReady, err: readyFailure}
	if _, err := store.ClonePublic(ctx, source.ID, referenceID(184), source.File); !errors.Is(err, readyFailure) {
		t.Fatalf("ready Sync error=%v", err)
	}
	store.records = baseRecords
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, node, limits)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPublic(ctx, referenceID(184)); err != nil {
		t.Fatalf("ready Sync recovery=%v", err)
	}

	basePinner := store.pinner
	pinFailure := errors.New("controlled clone recursive pin failure")
	store.pinner = &disappearingPinPinner{Pinner: basePinner, pinErr: pinFailure}
	if _, err := store.ClonePublic(ctx, source.ID, referenceID(185), source.File); !errors.Is(err, pinFailure) {
		t.Fatalf("recursive pin error=%v", err)
	}
	store.pinner = basePinner
	if _, err := store.GetPublic(ctx, referenceID(185)); !errors.Is(err, ErrNotReady) {
		t.Fatalf("pin-failed clone visibility=%v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, node, limits)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPublic(ctx, referenceID(185)); err != nil {
		t.Fatalf("recursive pin recovery=%v", err)
	}

	flushFailure := errors.New("controlled clone pin flush failure")
	store.pinner = &faultPinner{Pinner: store.pinner, flushErr: flushFailure}
	if _, err := store.ClonePublic(ctx, source.ID, referenceID(186), source.File); !errors.Is(err, flushFailure) {
		t.Fatalf("pin flush error=%v", err)
	}
	if _, err := store.GetPublic(ctx, referenceID(186)); !errors.Is(err, ErrNotReady) {
		t.Fatalf("flush-failed clone visibility=%v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, node, limits)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.GetPublic(ctx, referenceID(186)); err != nil {
		t.Fatalf("flush failure recovery=%v", err)
	}
}

type disappearingPinPinner struct {
	pinning.Pinner
	calls  int
	pinErr error
}

func (p *disappearingPinPinner) CheckIfPinnedWithType(ctx context.Context, mode pinning.Mode, includeNames bool, cids ...cid.Cid) ([]pinning.Pinned, error) {
	p.calls++
	if p.calls == 1 {
		return p.Pinner.CheckIfPinnedWithType(ctx, mode, includeNames, cids...)
	}
	return nil, nil
}

func (p *disappearingPinPinner) Pin(context.Context, format.Node, bool, string) error {
	return p.pinErr
}
