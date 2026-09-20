package social

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/boxo/blockstore"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	dsync "github.com/ipfs/go-datastore/sync"
	"github.com/larslarsen/bb-go/modern/attachment"
	"github.com/larslarsen/bb-go/modern/network"
	"github.com/larslarsen/bb-go/modern/publiccontent"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	testRichID       = "11111111111111111111111111111111"
	testAttachmentID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

func TestMEDIA001M1BSignedPayloadValidation(t *testing.T) {
	ctx := context.Background()
	fixture := newRichStoreFixture(t, ctx, nil)
	defer fixture.close(t)
	fixture.store.now = func() time.Time { return time.Date(2026, 9, 20, 1, 2, 3, 4, time.UTC) }
	content := textRichContent(t, "signed semantics")
	post, err := fixture.store.AddRichPost(ctx, testRichID, content, map[string]attachment.ReferenceID{})
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyPost(fixture.node.ID(), post); err != nil {
		t.Fatalf("valid rich post rejected: %v", err)
	}
	if post.CID == "" {
		t.Fatal("valid rich post lacks CID")
	}
	key := fixture.node.PrivateKey
	validPayload := append([]byte(nil), post.Post...)

	var decoded map[string]any
	if err := json.Unmarshal(validPayload, &decoded); err != nil {
		t.Fatal(err)
	}
	decoded["status"] = "forged projection"
	badStatus, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	assertRichVerificationFails(t, fixture.node.ID(), signedFixture(t, key, badStatus), "status")

	nonCanonical := []byte(fmt.Sprintf(`{"content":%s,"status":"signed semantics","postType":"POST","slug":"rich-%s","timestamp":"2026-09-20T01:02:03.000000004Z","vendorID":{"peerID":%q},"id":"%s","schema":"bitbook.public-post/1"}`,
		content, testRichID, fixture.node.ID().String(), testRichID))
	assertRichVerificationFails(t, fixture.node.ID(), signedFixture(t, key, nonCanonical), "noncanonical")

	duplicateNested := bytes.Replace(validPayload, []byte(`"marks":[]`), []byte(`"marks":[],"\u006darks":[]`), 1)
	assertRichVerificationFails(t, fixture.node.ID(), signedFixture(t, key, duplicateNested), "nested duplicate")

	unknownSchema := bytes.Replace(validPayload, []byte(`bitbook.public-post/1`), []byte(`bitbook.public-post/2`), 1)
	assertRichVerificationFails(t, fixture.node.ID(), signedFixture(t, key, unknownSchema), "unknown schema")

	badSlug := bytes.Replace(validPayload, []byte(`rich-`+testRichID), []byte(`rich-22222222222222222222222222222222`), 1)
	assertRichVerificationFails(t, fixture.node.ID(), signedFixture(t, key, badSlug), "slug")

	badAuthor := bytes.Replace(validPayload, []byte(fixture.node.ID().String()), []byte("12D3KooWInvalidPeerIdentity"), 1)
	assertRichVerificationFails(t, fixture.node.ID(), signedFixture(t, key, badAuthor), "author")

	hashMismatch := cloneSignedPost(post)
	hashMismatch.CID = blocks.NewBlock([]byte("other")).Cid().String()
	assertRichVerificationFails(t, fixture.node.ID(), hashMismatch, "CID")

	legacy := signedFixture(t, key, []byte(`{"status":"legacy remains signed"}`))
	legacy.CID = ""
	if err := VerifyPost(fixture.node.ID(), legacy); err != nil {
		t.Fatalf("legacy signature control rejected: %v", err)
	}
}

func TestMEDIA001M1BClassificationFailsClosed(t *testing.T) {
	ctx := context.Background()
	fixture := newRichStoreFixture(t, ctx, nil)
	defer fixture.close(t)
	fixture.store.now = func() time.Time { return time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC) }
	valid, err := fixture.store.AddRichPost(ctx, testRichID, textRichContent(t, "classification"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyPost(fixture.node.ID(), valid); err != nil {
		t.Fatalf("rich positive control: %v", err)
	}
	legacy := signedFixture(t, fixture.node.PrivateKey, []byte(`{"status":"legacy classification"}`))
	if err := VerifyPost(fixture.node.ID(), legacy); err != nil {
		t.Fatalf("legacy positive control: %v", err)
	}
	variants := map[string][]byte{
		"large-number-before-schema":   bytes.Replace(valid.Post, []byte(`{`), []byte(`{"ignored":1e9999,`), 1),
		"nested-large-number":          bytes.Replace(valid.Post, []byte(`{`), []byte(`{"ignored":{"nested":1e9999},`), 1),
		"escaped-schema-after-number":  bytes.Replace(bytes.Replace(valid.Post, []byte(`{`), []byte(`{"ignored":1e9999,`), 1), []byte(`"schema"`), []byte(`"\u0073chema"`), 1),
		"trailing-rich-json":           append(append([]byte(nil), valid.Post...), []byte(`{}`)...),
		"truncated-after-marker":       []byte(`{"ignored":1e9999,"schema":`),
		"malformed-non-object-framing": append([]byte(`null`), valid.Post...),
		"rich-payload-over-limit":      []byte(`{"schema":"bitbook.public-post/1","padding":"` + strings.Repeat("x", maxRichPayloadBytes) + `"}`),
	}
	for name, payload := range variants {
		t.Run(name, func(t *testing.T) {
			post := signedFixtureWithoutCID(t, fixture.node.PrivateKey, payload)
			key, err := crypto.UnmarshalPublicKey(post.PublicKey)
			if err != nil {
				t.Fatal(err)
			}
			ok, err := key.Verify(post.Post, post.Signature)
			if err != nil || !ok {
				t.Fatalf("primitive signature=%t err=%v", ok, err)
			}
			if err := VerifyPost(fixture.node.ID(), post); !errors.Is(err, ErrInvalidRichPost) {
				t.Fatalf("classification error=%v", err)
			}
		})
	}
}

func TestMEDIA001M1BRecoveryOwnershipBinding(t *testing.T) {
	for _, sameFile := range []bool{false, true} {
		name := "changed-target-different-file"
		if sameFile {
			name = "changed-target-same-file"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			fixture, record, upload := newOwnershipFixture(t, ctx)
			defer fixture.close(t)
			unrelatedID := richReferenceID(102)
			var unrelated attachment.PublicReference
			var err error
			if sameFile {
				unrelated, err = fixture.attachments.RetainPublic(ctx, unrelatedID, upload.File)
			} else {
				unrelated, err = fixture.attachments.ImportPublic(ctx, unrelatedID, bytes.NewReader(bytes.Repeat([]byte("unrelated"), 100)), 4096)
			}
			if err != nil {
				t.Fatal(err)
			}
			record.State = richStatePreparing
			record.Claims[0].TargetID = unrelated.ID
			persistRichRecordFixture(t, ctx, fixture, record)
			assertRichOpenFailsWithoutRelease(t, ctx, fixture, unrelatedID)
		})
	}

	t.Run("target-is-sibling-upload", func(t *testing.T) {
		ctx := context.Background()
		fixture, record, upload := newOwnershipFixture(t, ctx)
		defer fixture.close(t)
		siblingID := richReferenceID(103)
		if _, err := fixture.attachments.RetainPublic(ctx, siblingID, upload.File); err != nil {
			t.Fatal(err)
		}
		record.State = richStateDeleting
		record.Claims[0].TargetID = siblingID
		persistRichRecordFixture(t, ctx, fixture, record)
		assertRichOpenFailsWithoutRelease(t, ctx, fixture, siblingID)
	})

	for _, missing := range []bool{true, false} {
		name := "missing-certificate"
		if !missing {
			name = "changed-certificate"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			fixture, record, _ := newOwnershipFixture(t, ctx)
			defer fixture.close(t)
			target := record.Claims[0].TargetID
			if missing {
				encoded, err := encodeRichRecord(record)
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(encoded, &fields); err != nil {
					t.Fatal(err)
				}
				delete(fields, "claimSignature")
				encoded, err = json.Marshal(fields)
				if err != nil {
					t.Fatal(err)
				}
				persistRawRichRecordFixture(t, ctx, fixture, record.ID, encoded)
			} else {
				record.ClaimSignature[0] ^= 0xff
				persistRichRecordFixture(t, ctx, fixture, record)
			}
			assertRichOpenFailsWithoutRelease(t, ctx, fixture, target)
		})
	}

	t.Run("valid-certificate-actual-descriptor-mismatch", func(t *testing.T) {
		ctx := context.Background()
		fixture, record, _ := newOwnershipFixture(t, ctx)
		defer fixture.close(t)
		unrelatedID := richReferenceID(104)
		if _, err := fixture.attachments.ImportPublic(ctx, unrelatedID, bytes.NewReader(bytes.Repeat([]byte("descriptor mismatch"), 100)), 4096); err != nil {
			t.Fatal(err)
		}
		record.State = richStatePreparing
		record.Claims[0].TargetID = unrelatedID
		resignRichClaimFixture(t, fixture, &record)
		persistRichRecordFixture(t, ctx, fixture, record)
		assertRichOpenFailsWithoutRelease(t, ctx, fixture, unrelatedID)
	})

	t.Run("empty-envelope-cid-prevents-all-cleanup", func(t *testing.T) {
		ctx := context.Background()
		fixture, recoverable, _ := newOwnershipFixture(t, ctx)
		defer fixture.close(t)
		target := recoverable.Claims[0].TargetID
		recoverable.State = richStatePreparing
		persistRichRecordFixture(t, ctx, fixture, recoverable)
		badID := strings.Repeat("b", 32)
		if _, err := fixture.store.AddRichPost(ctx, badID, textRichContent(t, "bad empty CID"), nil); err != nil {
			t.Fatal(err)
		}
		bad := fixture.store.richRecords[badID]
		bad.Envelope.CID = ""
		persistRichRecordFixture(t, ctx, fixture, bad)
		if _, err := NewStoreWithAttachments(ctx, fixture.node, fixture.attachments, testRichLimits()); !errors.Is(err, ErrCorruptRichPostState) {
			t.Fatalf("empty CID open error=%v", err)
		}
		if _, err := fixture.attachments.GetPublic(ctx, target); err != nil {
			t.Fatalf("invalid later record allowed earlier cleanup: %v", err)
		}
	})

	for _, state := range []string{richStatePreparing, richStateDeleting} {
		t.Run("valid-partial-"+state, func(t *testing.T) {
			ctx := context.Background()
			fixture, record, _ := newOwnershipFixture(t, ctx)
			defer fixture.close(t)
			target := record.Claims[0].TargetID
			record.State = state
			persistRichRecordFixture(t, ctx, fixture, record)
			reopened, err := NewStoreWithAttachments(ctx, fixture.node, fixture.attachments, testRichLimits())
			if err != nil {
				t.Fatal(err)
			}
			wantState := richStateAborted
			if state == richStateDeleting {
				wantState = richStateDeleted
			}
			if reopened.richRecords[record.ID].State != wantState {
				t.Fatalf("recovered state=%q want=%q", reopened.richRecords[record.ID].State, wantState)
			}
			if _, err := fixture.attachments.GetPublic(ctx, target); !errors.Is(err, attachment.ErrNotFound) {
				t.Fatalf("partial target after recovery: %v", err)
			}
		})
	}
}

func newOwnershipFixture(t testing.TB, ctx context.Context) (*richStoreFixture, richRecord, attachment.PublicReference) {
	t.Helper()
	fixture := newRichStoreFixture(t, ctx, nil)
	uploadID := richReferenceID(101)
	upload, err := fixture.attachments.ImportPublic(ctx, uploadID, bytes.NewReader(bytes.Repeat([]byte("post file"), 100)), 4096)
	if err != nil {
		fixture.close(t)
		t.Fatal(err)
	}
	if _, err := fixture.store.AddRichPost(ctx, testRichID, attachmentRichContent(t, upload.File, "post.jpg"), map[string]attachment.ReferenceID{testAttachmentID: uploadID}); err != nil {
		fixture.close(t)
		t.Fatal(err)
	}
	return fixture, cloneRichRecord(fixture.store.richRecords[testRichID]), upload
}

func persistRichRecordFixture(t testing.TB, ctx context.Context, fixture *richStoreFixture, record richRecord) {
	t.Helper()
	encoded, err := encodeRichRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	persistRawRichRecordFixture(t, ctx, fixture, record.ID, encoded)
}

func persistRawRichRecordFixture(t testing.TB, ctx context.Context, fixture *richStoreFixture, id string, encoded []byte) {
	t.Helper()
	if err := fixture.node.Datastore.Put(ctx, richRecordKey(id), encoded); err != nil {
		t.Fatal(err)
	}
	if err := fixture.node.Datastore.Sync(ctx, richRecordPrefix); err != nil {
		t.Fatal(err)
	}
}

func resignRichClaimFixture(t testing.TB, fixture *richStoreFixture, record *richRecord) {
	t.Helper()
	preimage, err := richClaimCertificatePreimage(*record)
	if err != nil {
		t.Fatal(err)
	}
	record.ClaimSignature, err = fixture.node.PrivateKey.Sign(preimage)
	if err != nil {
		t.Fatal(err)
	}
}

func assertRichOpenFailsWithoutRelease(t testing.TB, ctx context.Context, fixture *richStoreFixture, protected attachment.ReferenceID) {
	t.Helper()
	if _, err := NewStoreWithAttachments(ctx, fixture.node, fixture.attachments, testRichLimits()); !errors.Is(err, ErrCorruptRichPostState) {
		t.Fatalf("corrupt ownership open error=%v", err)
	}
	if _, err := fixture.attachments.GetPublic(ctx, protected); err != nil {
		t.Fatalf("opening corrupt journal released protected claim: %v", err)
	}
}

func TestMEDIA001M1BLegacyRichSlugCompatibility(t *testing.T) {
	ctx := context.Background()
	fixture := newRichStoreFixture(t, ctx, nil)
	defer fixture.close(t)
	legacy, err := fixture.store.AddPost(ctx, json.RawMessage(`{"status":"rich content"}`))
	if err != nil {
		t.Fatal(err)
	}
	if postSlug(legacy) != "rich-content" {
		t.Fatalf("legacy slug=%q", postSlug(legacy))
	}
	if err := fixture.store.DeletePost(ctx, "rich-content"); err != nil {
		t.Fatalf("deleting ordinary legacy rich slug: %v", err)
	}
	historical := signedFixture(t, fixture.node.PrivateKey, []byte(`{"status":"historical","slug":"rich-historical"}`))
	if err := fixture.store.saveJSON(ctx, postsKey, []SignedPost{historical}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.DeletePost(ctx, "rich-historical"); err != nil {
		t.Fatalf("deleting historical explicit rich-prefixed legacy slug: %v", err)
	}
	rich, err := fixture.store.AddRichPost(ctx, testRichID, textRichContent(t, "rich owner"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.DeletePost(ctx, "rich-"+testRichID); !errors.Is(err, ErrInvalidRichPost) {
		t.Fatalf("legacy path deleted rich slug: %v", err)
	}
	if err := fixture.store.DeletePost(ctx, rich.CID); !errors.Is(err, ErrInvalidRichPost) {
		t.Fatalf("legacy path deleted rich CID: %v", err)
	}
	collision, err := fixture.store.AddPost(ctx, json.RawMessage(`{"status":"rich `+testRichID+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	if postSlug(collision) == "rich-"+testRichID {
		t.Fatal("automatically generated legacy slug collided with rich identity")
	}
	if err := fixture.store.DeleteRichPost(ctx, testRichID); err != nil {
		t.Fatal(err)
	}
	afterDelete, err := fixture.store.AddPost(ctx, json.RawMessage(`{"status":"rich `+testRichID+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	if postSlug(afterDelete) == "rich-"+testRichID {
		t.Fatal("automatically generated legacy slug collided with deleted rich identity")
	}
}

func TestMEDIA001M1BPostJournalFailuresRequireRecovery(t *testing.T) {
	t.Run("block-write", func(t *testing.T) {
		ctx := context.Background()
		fixture := newRichStoreFixture(t, ctx, nil)
		defer fixture.close(t)
		failure := errors.New("controlled envelope block write failure")
		fixture.node.Blockstore = &failOnePutBlockstore{Blockstore: fixture.node.Blockstore, err: failure, fail: true}
		if _, err := fixture.store.AddRichPost(ctx, testRichID, textRichContent(t, "block failure"), nil); !errors.Is(err, failure) {
			t.Fatalf("block failure error=%v", err)
		}
		assertRichStorePoisoned(t, ctx, fixture.store)
		reopened, err := NewStoreWithAttachments(ctx, fixture.node, fixture.attachments, testRichLimits())
		if err != nil {
			t.Fatal(err)
		}
		if reopened.richRecords[testRichID].State != richStateAborted {
			t.Fatalf("block failure recovery state=%q", reopened.richRecords[testRichID].State)
		}
	})

	t.Run("block-sync", func(t *testing.T) {
		ctx := context.Background()
		base := dsync.MutexWrap(datastore.NewMapDatastore())
		failure := errors.New("controlled envelope block sync failure")
		faults := &richFaultDatastore{Batching: base, err: failure, failBlockSync: true}
		fixture := newRichStoreFixture(t, ctx, faults)
		defer fixture.close(t)
		if _, err := fixture.store.AddRichPost(ctx, testRichID, textRichContent(t, "block sync failure"), nil); !errors.Is(err, failure) {
			t.Fatalf("block sync failure error=%v", err)
		}
		assertRichStorePoisoned(t, ctx, fixture.store)
		reopened, err := NewStoreWithAttachments(ctx, fixture.node, fixture.attachments, testRichLimits())
		if err != nil {
			t.Fatal(err)
		}
		if reopened.richRecords[testRichID].State != richStateAborted {
			t.Fatalf("block sync failure recovery state=%q", reopened.richRecords[testRichID].State)
		}
	})

	t.Run("later-clone", func(t *testing.T) {
		ctx := context.Background()
		base := dsync.MutexWrap(datastore.NewMapDatastore())
		callbacks := &richPreparingCallbackDatastore{Batching: base}
		fixture := newRichStoreFixture(t, ctx, callbacks)
		defer fixture.close(t)
		uploadID := richReferenceID(111)
		upload, err := fixture.attachments.ImportPublic(ctx, uploadID, bytes.NewReader(bytes.Repeat([]byte("clone failure"), 100)), 4096)
		if err != nil {
			t.Fatal(err)
		}
		callbacks.callback = func() error { return fixture.attachments.Release(ctx, uploadID) }
		_, err = fixture.store.AddRichPost(ctx, testRichID, attachmentRichContent(t, upload.File, "clone.jpg"), map[string]attachment.ReferenceID{testAttachmentID: uploadID})
		if !errors.Is(err, attachment.ErrNotFound) {
			t.Fatalf("later clone error=%v", err)
		}
		assertRichStorePoisoned(t, ctx, fixture.store)
		target := fixture.store.richRecords[testRichID].Claims[0].TargetID
		reopened, err := NewStoreWithAttachments(ctx, fixture.node, fixture.attachments, testRichLimits())
		if err != nil {
			t.Fatal(err)
		}
		if reopened.richRecords[testRichID].State != richStateAborted {
			t.Fatalf("clone failure recovery state=%q", reopened.richRecords[testRichID].State)
		}
		if _, err := fixture.attachments.GetPublic(ctx, target); !errors.Is(err, attachment.ErrNotFound) {
			t.Fatalf("failed clone target survived recovery: %v", err)
		}
	})
}

func TestMEDIA001M1BCancellationAcrossLifecycle(t *testing.T) {
	t.Run("creation-before-journal", func(t *testing.T) {
		fixture := newRichStoreFixture(t, context.Background(), nil)
		defer fixture.close(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := fixture.store.AddRichPost(ctx, testRichID, textRichContent(t, "cancel before journal"), nil); !errors.Is(err, context.Canceled) {
			t.Fatalf("creation cancellation error=%v", err)
		}
		if fixture.store.recoveryRequired || len(fixture.store.richRecords) != 0 {
			t.Fatal("pre-journal cancellation changed store state")
		}
	})

	t.Run("creation-after-journal", func(t *testing.T) {
		base := dsync.MutexWrap(datastore.NewMapDatastore())
		callbacks := &richPreparingCallbackDatastore{Batching: base}
		fixture := newRichStoreFixture(t, context.Background(), callbacks)
		defer fixture.close(t)
		ctx, cancel := context.WithCancel(context.Background())
		callbacks.callback = func() error {
			cancel()
			return nil
		}
		if _, err := fixture.store.AddRichPost(ctx, testRichID, textRichContent(t, "cancel after journal"), nil); !errors.Is(err, context.Canceled) {
			t.Fatalf("creation cancellation error=%v", err)
		}
		if !fixture.store.recoveryRequired {
			t.Fatal("post-journal cancellation did not poison store")
		}
		reopened, err := NewStoreWithAttachments(context.Background(), fixture.node, fixture.attachments, testRichLimits())
		if err != nil || reopened.richRecords[testRichID].State != richStateAborted {
			t.Fatalf("creation cancellation recovery state=%q err=%v", reopened.richRecords[testRichID].State, err)
		}
	})

	t.Run("loading", func(t *testing.T) {
		fixture := newRichStoreFixture(t, context.Background(), nil)
		defer fixture.close(t)
		if _, err := fixture.store.AddRichPost(context.Background(), testRichID, textRichContent(t, "cancel loading"), nil); err != nil {
			t.Fatal(err)
		}
		original := fixture.node.Datastore
		fixture.node.Datastore = &cancelQueryDatastore{Batching: original}
		_, err := NewStoreWithAttachments(context.Background(), fixture.node, fixture.attachments, testRichLimits())
		fixture.node.Datastore = original
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("loading cancellation error=%v", err)
		}
	})

	t.Run("recovery", func(t *testing.T) {
		base := dsync.MutexWrap(datastore.NewMapDatastore())
		faults := &richFaultDatastore{Batching: base, err: context.Canceled}
		fixture := newRichStoreFixture(t, context.Background(), faults)
		defer fixture.close(t)
		uploadID := richReferenceID(141)
		upload, err := fixture.attachments.ImportPublic(context.Background(), uploadID, bytes.NewReader(bytes.Repeat([]byte("cancel recovery"), 100)), 4096)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.store.AddRichPost(context.Background(), testRichID, attachmentRichContent(t, upload.File, "cancel.jpg"), map[string]attachment.ReferenceID{testAttachmentID: uploadID}); err != nil {
			t.Fatal(err)
		}
		record := cloneRichRecord(fixture.store.richRecords[testRichID])
		record.State = richStatePreparing
		persistRichRecordFixture(t, context.Background(), fixture, record)
		recoveryCtx, cancel := context.WithCancel(context.Background())
		faults.mu.Lock()
		faults.failAttachmentReleaseAt = 1
		faults.cancelOnAttachmentRelease = cancel
		faults.mu.Unlock()
		if _, err := NewStoreWithAttachments(recoveryCtx, fixture.node, fixture.attachments, testRichLimits()); !errors.Is(err, context.Canceled) {
			t.Fatalf("recovery cancellation error=%v", err)
		}
		reopened, err := NewStoreWithAttachments(context.Background(), fixture.node, fixture.attachments, testRichLimits())
		if err != nil || reopened.richRecords[testRichID].State != richStateAborted {
			t.Fatalf("recovery cancellation retry state=%q err=%v", reopened.richRecords[testRichID].State, err)
		}
	})
}

func assertRichStorePoisoned(t testing.TB, ctx context.Context, store *Store) {
	t.Helper()
	checks := []struct {
		name string
		run  func() error
	}{
		{"AddPost", func() error { _, err := store.AddPost(ctx, json.RawMessage(`{"status":"after failure"}`)); return err }},
		{"AddRichPost", func() error {
			_, err := store.AddRichPost(ctx, strings.Repeat("2", 32), textRichContent(t, "after failure"), nil)
			return err
		}},
		{"DeletePost", func() error { return store.DeletePost(ctx, "unknown") }},
		{"DeleteRichPost", func() error { return store.DeleteRichPost(ctx, testRichID) }},
		{"LocalPosts", func() error { _, err := store.LocalPosts(ctx); return err }},
		{"Commit", func() error { _, err := store.Commit(ctx); return err }},
		{"Publish", func() error { _, err := store.Publish(ctx); return err }},
		{"PublishRoot", func() error { return store.PublishRoot(ctx, blocks.NewBlock([]byte("root")).Cid()) }},
	}
	for _, check := range checks {
		if err := check.run(); !errors.Is(err, ErrRichPostRecoveryRequired) {
			t.Errorf("%s after journal failure error=%v", check.name, err)
		}
	}
}

type failOnePutBlockstore struct {
	blockstore.Blockstore
	err  error
	fail bool
}

func (store *failOnePutBlockstore) Put(ctx context.Context, block blocks.Block) error {
	if store.fail {
		store.fail = false
		return store.err
	}
	return store.Blockstore.Put(ctx, block)
}

type richPreparingCallbackDatastore struct {
	datastore.Batching
	mu       sync.Mutex
	prepared bool
	fired    bool
	callback func() error
}

type cancelQueryDatastore struct{ datastore.Batching }

func (store *cancelQueryDatastore) Query(ctx context.Context, request query.Query) (query.Results, error) {
	results, err := store.Batching.Query(ctx, request)
	if err != nil {
		return nil, err
	}
	entries, err := results.Rest()
	if closeErr := results.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	return query.ResultsWithContext(request, func(_ context.Context, output chan<- query.Result) {
		if len(entries) > 0 {
			output <- query.Result{Entry: entries[0]}
		}
		output <- query.Result{Error: context.Canceled}
	}), nil
}

func (store *richPreparingCallbackDatastore) Put(ctx context.Context, key datastore.Key, value []byte) error {
	store.mu.Lock()
	if bytes.Contains(value, []byte(`"state":"preparing"`)) {
		store.prepared = true
	}
	store.mu.Unlock()
	return store.Batching.Put(ctx, key, value)
}

func (store *richPreparingCallbackDatastore) Sync(ctx context.Context, prefix datastore.Key) error {
	if err := store.Batching.Sync(ctx, prefix); err != nil {
		return err
	}
	store.mu.Lock()
	shouldFire := store.prepared && !store.fired && store.callback != nil && prefix.String() == richRecordPrefix.String()
	if shouldFire {
		store.fired = true
	}
	callback := store.callback
	store.mu.Unlock()
	if shouldFire {
		return callback()
	}
	return nil
}

func TestMEDIA001M1BRetryDeleteAndLegacyGuards(t *testing.T) {
	ctx := context.Background()
	fixture := newRichStoreFixture(t, ctx, nil)
	defer fixture.close(t)
	fixed := time.Date(2026, 9, 20, 2, 3, 4, 5, time.UTC)
	fixture.store.now = func() time.Time { return fixed }
	content := textRichContent(t, "retry safe")
	first, err := fixture.store.AddRichPost(ctx, testRichID, content, map[string]attachment.ReferenceID{})
	if err != nil {
		t.Fatal(err)
	}
	firstCopy := cloneSignedPost(first)
	clear(first.Post)
	clear(first.Signature)
	second, err := fixture.store.AddRichPost(ctx, testRichID, append([]byte(nil), content...), map[string]attachment.ReferenceID{})
	if err != nil || !reflect.DeepEqual(second, firstCopy) {
		t.Fatalf("retry changed envelope: equal=%t err=%v", reflect.DeepEqual(second, firstCopy), err)
	}
	changed := textRichContent(t, "different")
	if _, err := fixture.store.AddRichPost(ctx, testRichID, changed, map[string]attachment.ReferenceID{}); !errors.Is(err, ErrRichPostConflict) {
		t.Fatalf("changed retry error=%v", err)
	}
	posts, err := fixture.store.LocalPosts(ctx)
	if err != nil || len(posts) != 1 || posts[0].CID != firstCopy.CID {
		t.Fatalf("local posts=%+v err=%v", posts, err)
	}
	if err := fixture.store.DeletePost(ctx, "rich-"+testRichID); !errors.Is(err, ErrInvalidRichPost) {
		t.Fatalf("legacy rich delete error=%v", err)
	}
	if err := fixture.store.DeletePost(ctx, firstCopy.CID); !errors.Is(err, ErrInvalidRichPost) {
		t.Fatalf("legacy CID delete error=%v", err)
	}
	if err := fixture.store.DeleteRichPost(ctx, testRichID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.DeleteRichPost(ctx, testRichID); err != nil {
		t.Fatalf("deleted retry=%v", err)
	}
	if _, err := fixture.store.AddRichPost(ctx, testRichID, content, nil); !errors.Is(err, ErrRichPostDeleted) {
		t.Fatalf("deleted recreation error=%v", err)
	}
	if err := fixture.store.DeleteRichPost(ctx, strings.Repeat("2", 32)); !errors.Is(err, datastore.ErrNotFound) {
		t.Fatalf("unknown deletion error=%v", err)
	}
	for _, field := range []string{"schema", "content", "requestId", "uploadReferences"} {
		raw := json.RawMessage(fmt.Sprintf(`{"status":"legacy","%s":"x"}`, field))
		if _, err := fixture.store.AddPost(ctx, raw); err == nil {
			t.Fatalf("legacy reserved field %q accepted", field)
		}
	}
	if _, err := fixture.store.AddPost(ctx, json.RawMessage(`{"status":"legacy","slug":"rich-user"}`)); err == nil {
		t.Fatal("legacy rich slug accepted")
	}
}

func TestMEDIA001M1BAttachmentClaimsAndRestart(t *testing.T) {
	ctx := context.Background()
	fixture := newRichStoreFixture(t, ctx, nil)
	defer fixture.close(t)
	data := bytes.Repeat([]byte("shared-media"), 500)
	uploadID := richReferenceID(1)
	upload, err := fixture.attachments.ImportPublic(ctx, uploadID, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	content := attachmentRichContent(t, upload.File, "photo.jpg")
	uploads := map[string]attachment.ReferenceID{testAttachmentID: uploadID}
	first, err := fixture.store.AddRichPost(ctx, testRichID, content, uploads)
	if err != nil {
		t.Fatal(err)
	}
	secondID := strings.Repeat("2", 32)
	second, err := fixture.store.AddRichPost(ctx, secondID, content, uploads)
	if err != nil {
		t.Fatal(err)
	}
	firstClaim := fixture.store.richRecords[testRichID].Claims[0].TargetID
	secondClaim := fixture.store.richRecords[secondID].Claims[0].TargetID
	if firstClaim == secondClaim || firstClaim == uploadID || secondClaim == uploadID {
		t.Fatal("post claims are not independent opaque IDs")
	}
	if bytes.Contains(first.Post, []byte(uploadID.String())) || bytes.Contains(first.Post, []byte(firstClaim.String())) {
		t.Fatal("signed payload exposes local claim material")
	}
	if err := fixture.attachments.Release(ctx, uploadID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.attachments.GetPublic(ctx, firstClaim); err != nil {
		t.Fatalf("first claim after upload release: %v", err)
	}
	reopened, err := NewStoreWithAttachments(ctx, fixture.node, fixture.attachments, testRichLimits())
	if err != nil {
		t.Fatal(err)
	}
	retry, err := reopened.AddRichPost(ctx, testRichID, content, uploads)
	if err != nil || !reflect.DeepEqual(retry, first) {
		t.Fatalf("restart retry equal=%t err=%v", reflect.DeepEqual(retry, first), err)
	}
	if err := reopened.DeleteRichPost(ctx, testRichID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.attachments.GetPublic(ctx, firstClaim); !errors.Is(err, attachment.ErrNotFound) {
		t.Fatalf("deleted first claim error=%v", err)
	}
	if _, err := fixture.attachments.GetPublic(ctx, secondClaim); err != nil {
		t.Fatalf("second claim released with first: %v", err)
	}
	if err := VerifyPost(fixture.node.ID(), second); err != nil {
		t.Fatal(err)
	}
}

func TestMEDIA001M1BColdRestartSharedMediaRetention(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	dataDir := t.TempDir()
	open := func() (*network.PersistentNode, *attachment.Store, *Store) {
		runtimeNode, err := network.Open(ctx, dataDir, network.Config{ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"}, BootstrapPeers: []peer.AddrInfo{}, DHTMode: dht.ModeServer, AllowPrivateAddresses: true})
		if err != nil {
			t.Fatal(err)
		}
		attachments, err := attachment.Open(ctx, runtimeNode.Node, attachment.Limits{MaxReferences: 128, MaxLogicalBytes: 1 << 30})
		if err != nil {
			runtimeNode.Close()
			t.Fatal(err)
		}
		store, err := NewStoreWithAttachments(ctx, runtimeNode.Node, attachments, testRichLimits())
		if err != nil {
			attachments.Close()
			runtimeNode.Close()
			t.Fatal(err)
		}
		return runtimeNode, attachments, store
	}
	closeAll := func(runtimeNode *network.PersistentNode, attachments *attachment.Store) {
		if err := attachments.Close(); err != nil {
			t.Fatal(err)
		}
		if err := runtimeNode.Close(); err != nil {
			t.Fatal(err)
		}
	}

	runtimeNode, attachments, store := open()
	data := bytes.Repeat([]byte("cold recursive media"), 2000)
	uploadID := richReferenceID(121)
	upload, err := attachments.ImportPublic(ctx, uploadID, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	repeatedContent, repeatedUploads := repeatedAttachmentRichContent(t, upload.File, uploadID)
	firstID, secondID := strings.Repeat("c", 32), strings.Repeat("d", 32)
	if _, err := store.AddRichPost(ctx, firstID, repeatedContent, repeatedUploads); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddRichPost(ctx, secondID, attachmentRichContent(t, upload.File, "shared.jpg"), map[string]attachment.ReferenceID{testAttachmentID: uploadID}); err != nil {
		t.Fatal(err)
	}
	firstClaims := slices.Clone(store.richRecords[firstID].Claims)
	secondClaim := store.richRecords[secondID].Claims[0]
	if len(firstClaims) != 2 || firstClaims[0].TargetID == firstClaims[1].TargetID || firstClaims[0].TargetID == secondClaim.TargetID || firstClaims[1].TargetID == secondClaim.TargetID {
		t.Fatal("same-file descriptors did not receive distinct private claims")
	}
	if err := attachments.Release(ctx, uploadID); err != nil {
		t.Fatal(err)
	}
	closeAll(runtimeNode, attachments)

	runtimeNode, attachments, store = open()
	for _, claim := range append(slices.Clone(firstClaims), secondClaim) {
		if _, err := attachments.GetPublic(ctx, claim.TargetID); err != nil {
			t.Fatalf("cold-reopened claim unavailable: %v", err)
		}
	}
	var copied bytes.Buffer
	if _, err := runtimeNode.Node.CopyPublicFile(ctx, upload.File, &copied); err != nil || !bytes.Equal(copied.Bytes(), data) {
		t.Fatalf("cold-reopened media bytes=%d err=%v", copied.Len(), err)
	}
	if err := store.DeleteRichPost(ctx, firstID); err != nil {
		t.Fatal(err)
	}
	for _, claim := range firstClaims {
		if _, err := attachments.GetPublic(ctx, claim.TargetID); !errors.Is(err, attachment.ErrNotFound) {
			t.Fatalf("deleted repeated claim error=%v", err)
		}
	}
	if _, err := attachments.GetPublic(ctx, secondClaim.TargetID); err != nil {
		t.Fatalf("shared post claim lost: %v", err)
	}
	closeAll(runtimeNode, attachments)

	runtimeNode, attachments, store = open()
	if _, err := attachments.GetPublic(ctx, secondClaim.TargetID); err != nil {
		t.Fatalf("surviving claim lost on second cold reopen: %v", err)
	}
	if err := store.DeleteRichPost(ctx, secondID); err != nil {
		t.Fatal(err)
	}
	if _, err := attachments.GetPublic(ctx, secondClaim.TargetID); !errors.Is(err, attachment.ErrNotFound) {
		t.Fatalf("final post claim error=%v", err)
	}
	closeAll(runtimeNode, attachments)

	runtimeNode, attachments, _ = open()
	for _, claim := range append(slices.Clone(firstClaims), secondClaim) {
		if _, err := attachments.GetPublic(ctx, claim.TargetID); !errors.Is(err, attachment.ErrNotFound) {
			t.Fatalf("final cold reopen retained private claim: %v", err)
		}
	}
	closeAll(runtimeNode, attachments)
}

func TestMEDIA001M1BLiveSyncRequired(t *testing.T) {
	ctx := context.Background()
	base := dsync.MutexWrap(datastore.NewMapDatastore())
	faults := &richFaultDatastore{Batching: base, failState: richStateLive, err: errors.New("controlled live sync failure")}
	fixture := newRichStoreFixture(t, ctx, faults)
	defer fixture.close(t)
	_, err := fixture.store.AddRichPost(ctx, testRichID, textRichContent(t, "sync required"), map[string]attachment.ReferenceID{})
	if !errors.Is(err, faults.err) {
		t.Fatalf("live sync error=%v", err)
	}
	if !fixture.store.recoveryRequired {
		t.Fatal("ambiguous live transition did not poison store")
	}
	if _, err := fixture.store.LocalPosts(ctx); !errors.Is(err, ErrRichPostRecoveryRequired) {
		t.Fatalf("poisoned LocalPosts error=%v", err)
	}
	if faults.liveSyncs != 1 {
		t.Fatalf("live sync attempts=%d want=1", faults.liveSyncs)
	}
}

func TestMEDIA001M1BAmbiguousLiveSyncRecoveryOutcomes(t *testing.T) {
	for _, discard := range []bool{false, true} {
		name := "retain-unsynced-live"
		if discard {
			name = "discard-unsynced-live"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			base := dsync.MutexWrap(datastore.NewMapDatastore())
			fault := errors.New("controlled ambiguous live sync")
			faults := &richFaultDatastore{Batching: base, failState: richStateLive, err: fault, discardUnsynced: discard}
			fixture := newRichStoreFixture(t, ctx, faults)
			defer fixture.close(t)
			data := bytes.Repeat([]byte("ambiguous live"), 100)
			uploadID := richReferenceID(61)
			upload, err := fixture.attachments.ImportPublic(ctx, uploadID, bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatal(err)
			}
			content := attachmentRichContent(t, upload.File, "ambiguous.jpg")
			if _, err := fixture.store.AddRichPost(ctx, testRichID, content, map[string]attachment.ReferenceID{testAttachmentID: uploadID}); !errors.Is(err, fault) {
				t.Fatalf("live sync error=%v", err)
			}
			claim := fixture.store.richRecords[testRichID].Claims[0].TargetID
			reopened, err := NewStoreWithAttachments(ctx, fixture.node, fixture.attachments, testRichLimits())
			if err != nil {
				t.Fatal(err)
			}
			posts, err := reopened.LocalPosts(ctx)
			if err != nil {
				t.Fatal(err)
			}
			_, claimErr := fixture.attachments.GetPublic(ctx, claim)
			if discard {
				if len(posts) != 0 || reopened.richRecords[testRichID].State != richStateAborted || !errors.Is(claimErr, attachment.ErrNotFound) {
					t.Fatalf("discard recovery posts=%d state=%q claimErr=%v", len(posts), reopened.richRecords[testRichID].State, claimErr)
				}
			} else if len(posts) != 1 || reopened.richRecords[testRichID].State != richStateLive || claimErr != nil {
				t.Fatalf("retained recovery posts=%d state=%q claimErr=%v", len(posts), reopened.richRecords[testRichID].State, claimErr)
			}
		})
	}
}

func TestMEDIA001M1BJournalPutSyncRecoveryMatrix(t *testing.T) {
	for _, transition := range []string{richStatePreparing, richStateLive, richStateDeleting, richStateDeleted} {
		for _, operation := range []string{"put", "sync"} {
			for _, discard := range []bool{false, true} {
				name := operation + "-" + transition + "-retain"
				if discard {
					name = operation + "-" + transition + "-discard"
				}
				t.Run(name, func(t *testing.T) {
					ctx := context.Background()
					base := dsync.MutexWrap(datastore.NewMapDatastore())
					fault := errors.New("controlled " + name)
					faults := &richFaultDatastore{Batching: base, err: fault}
					fixture := newRichStoreFixture(t, ctx, faults)
					defer fixture.close(t)
					content := textRichContent(t, "journal matrix")
					if transition == richStateDeleting || transition == richStateDeleted {
						if _, err := fixture.store.AddRichPost(ctx, testRichID, content, nil); err != nil {
							t.Fatal(err)
						}
					}
					faults.mu.Lock()
					if operation == "put" {
						faults.failPutState = transition
						faults.discardFailedPut = discard
					} else {
						faults.failState = transition
						faults.discardUnsynced = discard
					}
					faults.mu.Unlock()
					var err error
					if transition == richStatePreparing || transition == richStateLive {
						_, err = fixture.store.AddRichPost(ctx, testRichID, content, nil)
					} else {
						err = fixture.store.DeleteRichPost(ctx, testRichID)
					}
					if !errors.Is(err, fault) || !fixture.store.recoveryRequired {
						t.Fatalf("transition error=%v poisoned=%t", err, fixture.store.recoveryRequired)
					}
					reopened, err := NewStoreWithAttachments(ctx, fixture.node, fixture.attachments, testRichLimits())
					if err != nil {
						t.Fatal(err)
					}
					record, exists := reopened.richRecords[testRichID]
					switch transition {
					case richStatePreparing:
						if discard {
							if exists {
								t.Fatalf("discarded preparing Put left state=%q", record.State)
							}
						} else if !exists || record.State != richStateAborted {
							t.Fatalf("preparing recovery exists=%t state=%q", exists, record.State)
						}
					case richStateLive:
						want := richStateLive
						if discard {
							want = richStateAborted
						}
						if !exists || record.State != want {
							t.Fatalf("live recovery exists=%t state=%q want=%q", exists, record.State, want)
						}
					case richStateDeleting:
						want := richStateDeleted
						if discard {
							want = richStateLive
						}
						if !exists || record.State != want {
							t.Fatalf("deleting recovery exists=%t state=%q want=%q", exists, record.State, want)
						}
					case richStateDeleted:
						if !exists || record.State != richStateDeleted {
							t.Fatalf("deleted recovery exists=%t state=%q", exists, record.State)
						}
					}
				})
			}
		}
	}
}

func TestMEDIA001M1BConstructorsAndRecordCodec(t *testing.T) {
	ctx := context.Background()
	fixture := newRichStoreFixture(t, ctx, nil)
	defer fixture.close(t)
	if _, err := NewStoreWithAttachments(nil, fixture.node, fixture.attachments, testRichLimits()); err == nil {
		t.Fatal("nil context accepted")
	}
	if _, err := NewStoreWithAttachments(ctx, fixture.node, fixture.attachments, RichPostLimits{}); err == nil {
		t.Fatal("invalid limits accepted")
	}
	other := newTestNode(t, ctx)
	defer other.Close()
	if _, err := NewStoreWithAttachments(ctx, other, fixture.attachments, testRichLimits()); err == nil {
		t.Fatal("mismatched attachment store accepted")
	}

	post, err := fixture.store.AddRichPost(ctx, testRichID, textRichContent(t, "record"), nil)
	if err != nil {
		t.Fatal(err)
	}
	record := fixture.store.richRecords[testRichID]
	encoded, err := encodeRichRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeRichRecord(encoded)
	if err != nil || !reflect.DeepEqual(decoded, record) {
		t.Fatalf("record round trip equal=%t err=%v", reflect.DeepEqual(decoded, record), err)
	}
	if !bytes.Contains(encoded, []byte(post.CID)) {
		t.Fatal("live record omitted frozen envelope CID")
	}
	duplicate := bytes.Replace(encoded, []byte(`"version":1`), []byte(`"version":1,"\u0076ersion":1`), 1)
	if _, err := decodeRichRecord(duplicate); !errors.Is(err, ErrCorruptRichPostState) {
		t.Fatalf("duplicate record error=%v", err)
	}
	if _, err := NewStore(fixture.node); !errors.Is(err, ErrRichPostUnavailable) {
		t.Fatalf("legacy constructor with rich state error=%v", err)
	}
}

func TestMEDIA001M1BClaimCertificateLiteral(t *testing.T) {
	uploadID, err := attachment.ParseReferenceID("01010101010101010101010101010101")
	if err != nil {
		t.Fatal(err)
	}
	targetID, err := attachment.ParseReferenceID("02020202020202020202020202020202")
	if err != nil {
		t.Fatal(err)
	}
	claimCID, err := cid.Decode("bafkreibzsacz6kfnmwxztclqftvham5xusz2mh5eoxctnzjxqawpmwlw2a")
	if err != nil {
		t.Fatal(err)
	}
	record := richRecord{
		ID: testRichID, RequestDigest: strings.Repeat("a", 64),
		Envelope: SignedPost{CID: "QmdXVo8TAGL8cVo52MBk4oQzPTPYLh2HEiFYAVMbwt2Dsh"},
		Claims:   []richClaim{{AttachmentID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", UploadID: uploadID, TargetID: targetID, File: network.PublicFile{CID: claimCID, ByteLength: 36}}},
	}
	preimage, err := richClaimCertificatePreimage(record)
	if err != nil {
		t.Fatal(err)
	}
	wantPreimage := []byte("bitbook.local-post-claims/1\x00" + `{"id":"11111111111111111111111111111111","postCID":"QmdXVo8TAGL8cVo52MBk4oQzPTPYLh2HEiFYAVMbwt2Dsh","requestDigest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","claims":[{"attachmentId":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","uploadId":"01010101010101010101010101010101","targetId":"02020202020202020202020202020202","cid":"bafkreibzsacz6kfnmwxztclqftvham5xusz2mh5eoxctnzjxqawpmwlw2a","byteLength":36}]}`)
	if !bytes.Equal(preimage, wantPreimage) {
		t.Fatalf("claim preimage mismatch\n got=%q\nwant=%q", preimage, wantPreimage)
	}
	wantSignature, err := base64.StdEncoding.DecodeString("Nz50skU2sy/Wct4njhoTAd7QppQl7qEIlsywkRY53GCVuPR1rA9OuOe3KMpw93XZ+G3SSqG9q90hzg3jw/Z5AQ==")
	if err != nil {
		t.Fatal(err)
	}
	key := deterministicRichKey(t)
	signature, err := key.Sign(preimage)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(signature, wantSignature) {
		t.Fatalf("claim signature=%s", base64.StdEncoding.EncodeToString(signature))
	}
	ok, err := key.GetPublic().Verify(wantPreimage, wantSignature)
	if err != nil || !ok {
		t.Fatalf("literal certificate primitive verification=%t err=%v", ok, err)
	}
}

func TestMEDIA001M1BQuotaAndInputBounds(t *testing.T) {
	ctx := context.Background()
	fixture := newRichStoreFixture(t, ctx, nil)
	defer fixture.close(t)
	fixture.store.richLimits = RichPostLimits{MaxRecords: 1, MaxBytes: 96 << 10}
	created, err := fixture.store.AddRichPost(ctx, testRichID, textRichContent(t, "within quota"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.AddRichPost(ctx, strings.Repeat("2", 32), textRichContent(t, "over count"), nil); !errors.Is(err, ErrRichPostQuota) {
		t.Fatalf("record quota error=%v", err)
	}
	if fixture.store.recoveryRequired {
		t.Fatal("pre-journal quota rejection poisoned store")
	}
	if posts, err := fixture.store.LocalPosts(ctx); err != nil || len(posts) != 1 {
		t.Fatalf("store after quota posts=%d err=%v", len(posts), err)
	}
	tooLarge := json.RawMessage(bytes.Repeat([]byte{'x'}, 65537))
	if _, err := fixture.store.AddRichPost(ctx, strings.Repeat("3", 32), tooLarge, nil); !errors.Is(err, ErrInvalidRichPost) {
		t.Fatalf("oversize input error=%v", err)
	}
	tooMany := make(map[string]attachment.ReferenceID, 9)
	for index := byte(1); index <= 9; index++ {
		tooMany[fmt.Sprintf("%032x", index)] = richReferenceID(index)
	}
	if _, err := fixture.store.AddRichPost(ctx, strings.Repeat("3", 32), textRichContent(t, "mapping bound"), tooMany); !errors.Is(err, ErrInvalidRichPost) {
		t.Fatalf("oversize mapping error=%v", err)
	}
	if _, err := decodeRichRecord(bytes.Repeat([]byte{'x'}, maxRichRecordBytes+1)); !errors.Is(err, ErrCorruptRichPostState) {
		t.Fatalf("oversize record error=%v", err)
	}
	bounded := cloneSignedPost(created)
	bounded.CID = ""
	bounded.Signature = make([]byte, maxRichSignatureBytes)
	if _, err := validateRichEnvelope(fixture.node.ID(), bounded); err != nil {
		t.Fatalf("signature defensive ceiling rejected: %v", err)
	}
	bounded.Signature = make([]byte, maxRichSignatureBytes+1)
	if _, err := validateRichEnvelope(fixture.node.ID(), bounded); !errors.Is(err, ErrInvalidRichPost) {
		t.Fatalf("signature above ceiling error=%v", err)
	}
	bounded = cloneSignedPost(created)
	bounded.CID = ""
	bounded.PublicKey = make([]byte, maxRichPublicKeyBytes)
	if _, err := validateRichEnvelope(fixture.node.ID(), bounded); err != nil {
		t.Fatalf("public-key defensive ceiling rejected: %v", err)
	}
	bounded.PublicKey = make([]byte, maxRichPublicKeyBytes+1)
	if _, err := validateRichEnvelope(fixture.node.ID(), bounded); !errors.Is(err, ErrInvalidRichPost) {
		t.Fatalf("public key above ceiling error=%v", err)
	}

	aggregate := newRichStoreFixture(t, ctx, nil)
	defer aggregate.close(t)
	aggregate.store.richLimits = RichPostLimits{MaxRecords: 128, MaxBytes: 96 << 10}
	aggregate.store.now = func() time.Time { return time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC) }
	succeeded := 0
	for index := 1; index <= 128; index++ {
		id := fmt.Sprintf("%032x", index)
		_, err := aggregate.store.AddRichPost(ctx, id, textRichContent(t, "aggregate boundary"), nil)
		if errors.Is(err, ErrRichPostQuota) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		succeeded++
	}
	if succeeded == 0 || succeeded >= 128 || aggregate.store.richBytes > aggregate.store.richLimits.MaxBytes {
		t.Fatalf("aggregate boundary succeeded=%d bytes=%d", succeeded, aggregate.store.richBytes)
	}
	if aggregate.store.recoveryRequired {
		t.Fatal("aggregate quota rejection poisoned store")
	}
	if err := aggregate.store.DeleteRichPost(ctx, fmt.Sprintf("%032x", 1)); err != nil {
		t.Fatalf("deletion at aggregate quota: %v", err)
	}
}

func TestMEDIA001M1BConcurrentIdenticalCreation(t *testing.T) {
	ctx := context.Background()
	fixture := newRichStoreFixture(t, ctx, nil)
	defer fixture.close(t)
	content := textRichContent(t, "concurrent")
	const callers = 12
	results := make([]SignedPost, callers)
	errs := make([]error, callers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for index := range callers {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			results[index], errs[index] = fixture.store.AddRichPost(ctx, testRichID, content, map[string]attachment.ReferenceID{})
		}(index)
	}
	close(start)
	wg.Wait()
	for index := range callers {
		if errs[index] != nil || !reflect.DeepEqual(results[index], results[0]) {
			t.Fatalf("caller %d equal=%t err=%v", index, reflect.DeepEqual(results[index], results[0]), errs[index])
		}
	}
	posts, err := fixture.store.LocalPosts(ctx)
	if err != nil || len(posts) != 1 {
		t.Fatalf("visible posts=%d err=%v", len(posts), err)
	}
}

func TestMEDIA001M1BMixedOrderingAndCallerOwnership(t *testing.T) {
	ctx := context.Background()
	fixture := newRichStoreFixture(t, ctx, nil)
	defer fixture.close(t)
	fixed := time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC)
	fixture.store.now = func() time.Time { return fixed }
	if _, err := fixture.store.AddPost(ctx, json.RawMessage(`{"status":"legacy-one"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.AddPost(ctx, json.RawMessage(`{"status":"legacy-two"}`)); err != nil {
		t.Fatal(err)
	}
	contentTwo := textRichContent(t, "rich-two")
	second, err := fixture.store.AddRichPost(ctx, strings.Repeat("2", 32), contentTwo, map[string]attachment.ReferenceID{})
	if err != nil {
		t.Fatal(err)
	}
	contentOne := textRichContent(t, "rich-one")
	first, err := fixture.store.AddRichPost(ctx, strings.Repeat("1", 32), contentOne, map[string]attachment.ReferenceID{})
	if err != nil {
		t.Fatal(err)
	}
	clear(first.Post)
	clear(first.Signature)
	clear(second.PublicKey)
	clear(contentOne)
	clear(contentTwo)

	posts, err := fixture.store.LocalPosts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 4 {
		t.Fatalf("mixed post count=%d", len(posts))
	}
	got := make([]string, len(posts))
	for index, post := range posts {
		var decoded struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(post.Post, &decoded); err != nil {
			t.Fatal(err)
		}
		got[index] = decoded.Status
	}
	want := []string{"legacy-two", "legacy-one", "rich-one", "rich-two"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mixed order=%v want=%v", got, want)
	}
	retry, err := fixture.store.AddRichPost(ctx, strings.Repeat("1", 32), textRichContent(t, "rich-one"), nil)
	if err != nil || len(retry.Post) == 0 || len(retry.Signature) == 0 || len(retry.PublicKey) == 0 {
		t.Fatalf("caller mutation changed stored envelope: %+v err=%v", retry, err)
	}
}

func TestMEDIA001M1BInterruptedTransitionsRecover(t *testing.T) {
	ctx := context.Background()
	base := dsync.MutexWrap(datastore.NewMapDatastore())
	faults := &richFaultDatastore{Batching: base, failState: richStatePreparing, err: errors.New("controlled preparing sync failure")}
	fixture := newRichStoreFixture(t, ctx, faults)
	defer fixture.close(t)
	content := textRichContent(t, "recover prepare")
	if _, err := fixture.store.AddRichPost(ctx, testRichID, content, nil); !errors.Is(err, faults.err) {
		t.Fatalf("preparing sync error=%v", err)
	}
	if !fixture.store.recoveryRequired {
		t.Fatal("ambiguous preparing transition did not poison store")
	}
	reopened, err := NewStoreWithAttachments(ctx, fixture.node, fixture.attachments, testRichLimits())
	if err != nil {
		t.Fatal(err)
	}
	if reopened.richRecords[testRichID].State != richStateAborted {
		t.Fatalf("recovered state=%q", reopened.richRecords[testRichID].State)
	}
	if _, err := reopened.AddRichPost(ctx, testRichID, content, nil); err != nil {
		t.Fatalf("retry after preparing recovery: %v", err)
	}

	data := bytes.Repeat([]byte("delete recovery"), 100)
	uploadID := richReferenceID(71)
	upload, err := fixture.attachments.ImportPublic(ctx, uploadID, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	deleteID := strings.Repeat("7", 32)
	deleteContent := attachmentRichContent(t, upload.File, "delete.jpg")
	if _, err := reopened.AddRichPost(ctx, deleteID, deleteContent, map[string]attachment.ReferenceID{testAttachmentID: uploadID}); err != nil {
		t.Fatal(err)
	}
	claim := reopened.richRecords[deleteID].Claims[0].TargetID
	faults.mu.Lock()
	faults.failState = richStateDeleting
	faults.err = errors.New("controlled deleting sync failure")
	faults.mu.Unlock()
	if err := reopened.DeleteRichPost(ctx, deleteID); !errors.Is(err, faults.err) {
		t.Fatalf("deleting sync error=%v", err)
	}
	if _, err := fixture.attachments.GetPublic(ctx, claim); err != nil {
		t.Fatalf("failed deleting sync released claim: %v", err)
	}
	recovered, err := NewStoreWithAttachments(ctx, fixture.node, fixture.attachments, testRichLimits())
	if err != nil {
		t.Fatal(err)
	}
	if recovered.richRecords[deleteID].State != richStateDeleted {
		t.Fatalf("recovered deletion state=%q", recovered.richRecords[deleteID].State)
	}
	if _, err := fixture.attachments.GetPublic(ctx, claim); !errors.Is(err, attachment.ErrNotFound) {
		t.Fatalf("recovered deletion claim error=%v", err)
	}
}

func TestMEDIA001M1BMultiClaimCleanupRetry(t *testing.T) {
	ctx := context.Background()
	base := dsync.MutexWrap(datastore.NewMapDatastore())
	faults := &richFaultDatastore{Batching: base, err: errors.New("controlled second cleanup release failure")}
	fixture := newRichStoreFixture(t, ctx, faults)
	defer fixture.close(t)
	uploadID := richReferenceID(131)
	upload, err := fixture.attachments.ImportPublic(ctx, uploadID, bytes.NewReader(bytes.Repeat([]byte("multiple cleanup"), 100)), 4096)
	if err != nil {
		t.Fatal(err)
	}
	content, uploads := repeatedAttachmentRichContent(t, upload.File, uploadID)
	if _, err := fixture.store.AddRichPost(ctx, testRichID, content, uploads); err != nil {
		t.Fatal(err)
	}
	record := cloneRichRecord(fixture.store.richRecords[testRichID])
	record.State = richStatePreparing
	persistRichRecordFixture(t, ctx, fixture, record)
	faults.mu.Lock()
	faults.failAttachmentReleaseAt = 2
	faults.mu.Unlock()
	if _, err := NewStoreWithAttachments(ctx, fixture.node, fixture.attachments, testRichLimits()); !errors.Is(err, faults.err) {
		t.Fatalf("cleanup failure open error=%v", err)
	}
	if _, err := fixture.attachments.GetPublic(ctx, record.Claims[0].TargetID); !errors.Is(err, attachment.ErrNotFound) {
		t.Fatalf("first cleanup claim error=%v", err)
	}
	if _, err := fixture.attachments.GetPublic(ctx, record.Claims[1].TargetID); err != nil {
		t.Fatalf("failed cleanup claim was released: %v", err)
	}
	reopened, err := NewStoreWithAttachments(ctx, fixture.node, fixture.attachments, testRichLimits())
	if err != nil {
		t.Fatal(err)
	}
	if reopened.richRecords[testRichID].State != richStateAborted {
		t.Fatalf("retry cleanup state=%q", reopened.richRecords[testRichID].State)
	}
	for _, claim := range record.Claims {
		if _, err := fixture.attachments.GetPublic(ctx, claim.TargetID); !errors.Is(err, attachment.ErrNotFound) {
			t.Fatalf("cleanup retry leaked claim: %v", err)
		}
	}
}

func TestMEDIA001M1BCorruptOwnershipFailsBeforeRecovery(t *testing.T) {
	ctx := context.Background()
	fixture := newRichStoreFixture(t, ctx, nil)
	defer fixture.close(t)
	data := bytes.Repeat([]byte("duplicate owner"), 100)
	uploadID := richReferenceID(81)
	upload, err := fixture.attachments.ImportPublic(ctx, uploadID, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	content := attachmentRichContent(t, upload.File, "owner.jpg")
	firstID, secondID := strings.Repeat("8", 32), strings.Repeat("9", 32)
	if _, err := fixture.store.AddRichPost(ctx, firstID, content, map[string]attachment.ReferenceID{testAttachmentID: uploadID}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.AddRichPost(ctx, secondID, content, map[string]attachment.ReferenceID{testAttachmentID: uploadID}); err != nil {
		t.Fatal(err)
	}
	first := fixture.store.richRecords[firstID]
	second := fixture.store.richRecords[secondID]
	first.State = richStatePreparing
	second.Claims[0].TargetID = first.Claims[0].TargetID
	resignRichClaimFixture(t, fixture, &second)
	for _, record := range []richRecord{first, second} {
		encoded, err := encodeRichRecord(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := fixture.node.Datastore.Put(ctx, richRecordKey(record.ID), encoded); err != nil {
			t.Fatal(err)
		}
	}
	if err := fixture.node.Datastore.Sync(ctx, richRecordPrefix); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStoreWithAttachments(ctx, fixture.node, fixture.attachments, testRichLimits()); !errors.Is(err, ErrCorruptRichPostState) {
		t.Fatalf("duplicate ownership open error=%v", err)
	}
	if _, err := fixture.attachments.GetPublic(ctx, first.Claims[0].TargetID); err != nil {
		t.Fatalf("validation released preparing claim: %v", err)
	}
}

func TestMEDIA001M1BTwoNodeMixedFetchDoesNotRetainMedia(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	author := newRichStoreFixture(t, ctx, nil)
	defer author.close(t)
	reader := newTestNode(t, ctx)
	defer reader.Close()
	connectTestNodes(t, ctx, author.node, reader)
	readerStore, err := NewStore(reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := author.store.SetProfile(ctx, json.RawMessage(`{"name":"media author"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := author.store.AddPost(ctx, json.RawMessage(`{"status":"legacy remote"}`)); err != nil {
		t.Fatal(err)
	}
	data := bytes.Repeat([]byte("remote media remains remote"), 200)
	uploadID := richReferenceID(91)
	upload, err := author.attachments.ImportPublic(ctx, uploadID, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	rich, err := author.store.AddRichPost(ctx, strings.Repeat("a", 32), attachmentRichContent(t, upload.File, "remote.jpg"), map[string]attachment.ReferenceID{testAttachmentID: uploadID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := author.store.Publish(ctx); err != nil {
		t.Fatal(err)
	}
	state, err := readerStore.Fetch(ctx, author.node.ID())
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Posts) != 2 || state.Posts[0].CID != rich.CID {
		t.Fatalf("mixed remote posts=%+v", state.Posts)
	}
	present, err := reader.Blockstore.Has(ctx, upload.File.CID)
	if err != nil || present {
		t.Fatalf("reader attachment root present=%t err=%v", present, err)
	}
	results, err := reader.Datastore.Query(ctx, query.Query{Prefix: "/bitbook/attachment/public/v1/reference", Limit: 1, KeysOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer results.Close()
	if _, ok := results.NextSync(); ok {
		t.Fatal("remote fetch created an attachment retention claim")
	}
}

type publicPostVector struct {
	Name      string `json:"name"`
	Author    string `json:"author"`
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
	PublicKey string `json:"publicKey"`
	Hash      string `json:"hash"`
	Valid     bool   `json:"valid"`
	Legacy    bool   `json:"legacy"`
}

func TestMEDIA001M1BLiteralVectors(t *testing.T) {
	raw, err := os.ReadFile("testdata/public-post-v1-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []publicPostVector
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&vectors); err != nil {
		t.Fatal(err)
	}
	if len(vectors) < 3 {
		t.Fatalf("vector count=%d", len(vectors))
	}
	for _, vector := range vectors {
		t.Run(vector.Name, func(t *testing.T) {
			author, err := peer.Decode(vector.Author)
			if err != nil {
				t.Fatal(err)
			}
			signature, err := base64.StdEncoding.DecodeString(vector.Signature)
			if err != nil {
				t.Fatal(err)
			}
			publicKeyBytes, err := base64.StdEncoding.DecodeString(vector.PublicKey)
			if err != nil {
				t.Fatal(err)
			}
			publicKey, err := crypto.UnmarshalPublicKey(publicKeyBytes)
			if err != nil {
				t.Fatal(err)
			}
			primitiveOK, err := publicKey.Verify([]byte(vector.Payload), signature)
			if err != nil || !primitiveOK {
				t.Fatalf("independent primitive verification=%t err=%v", primitiveOK, err)
			}
			post := SignedPost{Post: json.RawMessage(vector.Payload), Signature: signature, PublicKey: publicKeyBytes, CID: vector.Hash}
			err = VerifyPost(author, post)
			if vector.Valid && err != nil {
				t.Fatalf("valid vector rejected: %v", err)
			}
			if !vector.Valid && !errors.Is(err, ErrInvalidRichPost) {
				t.Fatalf("invalid vector error=%v", err)
			}
		})
	}
}

type richStoreFixture struct {
	node        *network.Node
	attachments *attachment.Store
	store       *Store
}

func newRichStoreFixture(t testing.TB, ctx context.Context, records datastore.Batching) *richStoreFixture {
	t.Helper()
	if records == nil {
		records = dsync.MutexWrap(datastore.NewMapDatastore())
	}
	node, err := network.New(ctx, network.Config{
		Datastore:             records,
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	attachments, err := attachment.Open(ctx, node, attachment.Limits{MaxReferences: 128, MaxLogicalBytes: 1 << 30})
	if err != nil {
		node.Close()
		t.Fatal(err)
	}
	store, err := NewStoreWithAttachments(ctx, node, attachments, testRichLimits())
	if err != nil {
		attachments.Close()
		node.Close()
		t.Fatal(err)
	}
	return &richStoreFixture{node: node, attachments: attachments, store: store}
}

func (fixture *richStoreFixture) close(t testing.TB) {
	t.Helper()
	if fixture.attachments != nil {
		if err := fixture.attachments.Close(); err != nil {
			t.Error(err)
		}
	}
	if fixture.node != nil {
		if err := fixture.node.Close(); err != nil {
			t.Error(err)
		}
	}
}

func testRichLimits() RichPostLimits { return RichPostLimits{MaxRecords: 64, MaxBytes: 4 << 20} }

func textRichContent(t testing.TB, text string) json.RawMessage {
	t.Helper()
	encoded, err := publiccontent.Marshal(publiccontent.Content{
		Schema:      publiccontent.SchemaV1,
		Body:        []publiccontent.Paragraph{{Type: "paragraph", Children: []publiccontent.InlineNode{{Type: "text", Text: text}}}},
		Attachments: []publiccontent.AttachmentDescriptor{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func attachmentRichContent(t testing.TB, file network.PublicFile, filename string) json.RawMessage {
	t.Helper()
	encoded, err := publiccontent.Marshal(publiccontent.Content{
		Schema: publiccontent.SchemaV1,
		Body:   []publiccontent.Paragraph{{Type: "paragraph", Children: []publiccontent.InlineNode{{Type: "attachment", AttachmentID: testAttachmentID}}}},
		Attachments: []publiccontent.AttachmentDescriptor{{
			AttachmentID: testAttachmentID, CID: file.CID.String(), MediaType: "image/jpeg",
			ByteLength: uint64(file.ByteLength), Width: 1, Height: 1, Filename: filename,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func repeatedAttachmentRichContent(t testing.TB, file network.PublicFile, uploadID attachment.ReferenceID) (json.RawMessage, map[string]attachment.ReferenceID) {
	t.Helper()
	secondAttachmentID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	encoded, err := publiccontent.Marshal(publiccontent.Content{
		Schema: publiccontent.SchemaV1,
		Body: []publiccontent.Paragraph{{Type: "paragraph", Children: []publiccontent.InlineNode{
			{Type: "attachment", AttachmentID: testAttachmentID},
			{Type: "attachment", AttachmentID: secondAttachmentID},
		}}},
		Attachments: []publiccontent.AttachmentDescriptor{
			{AttachmentID: testAttachmentID, CID: file.CID.String(), MediaType: "image/jpeg", ByteLength: uint64(file.ByteLength), Width: 1, Height: 1, Filename: "repeat-a.jpg"},
			{AttachmentID: secondAttachmentID, CID: file.CID.String(), MediaType: "image/jpeg", ByteLength: uint64(file.ByteLength), Width: 1, Height: 1, Filename: "repeat-b.jpg"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return encoded, map[string]attachment.ReferenceID{testAttachmentID: uploadID, secondAttachmentID: uploadID}
}

func richReferenceID(value byte) attachment.ReferenceID {
	var id attachment.ReferenceID
	for index := range id {
		id[index] = value
	}
	return id
}

func deterministicRichKey(t testing.TB) crypto.PrivKey {
	t.Helper()
	seed := sha256.Sum256([]byte("BitBook M1B named public deterministic test key"))
	key, err := crypto.UnmarshalEd25519PrivateKey(ed25519.NewKeyFromSeed(seed[:]))
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func signedFixture(t testing.TB, key crypto.PrivKey, payload []byte) SignedPost {
	t.Helper()
	signature, err := key.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := crypto.MarshalPublicKey(key.GetPublic())
	if err != nil {
		t.Fatal(err)
	}
	post := SignedPost{Post: append([]byte(nil), payload...), Signature: signature, PublicKey: publicKey}
	blockBytes, err := immutablePostBytes(post)
	if err != nil {
		t.Fatal(err)
	}
	post.CID = blocks.NewBlock(blockBytes).Cid().String()
	return post
}

func signedFixtureWithoutCID(t testing.TB, key crypto.PrivKey, payload []byte) SignedPost {
	t.Helper()
	signature, err := key.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := crypto.MarshalPublicKey(key.GetPublic())
	if err != nil {
		t.Fatal(err)
	}
	return SignedPost{Post: append([]byte(nil), payload...), Signature: signature, PublicKey: publicKey}
}

func assertRichVerificationFails(t testing.TB, author peer.ID, post SignedPost, name string) {
	t.Helper()
	if err := VerifyPost(author, post); !errors.Is(err, ErrInvalidRichPost) {
		t.Fatalf("%s verification error=%v", name, err)
	}
}

type richFaultDatastore struct {
	datastore.Batching
	mu                        sync.Mutex
	lastState                 string
	lastKey                   datastore.Key
	prior                     []byte
	priorFound                bool
	failState                 string
	err                       error
	liveSyncs                 int
	discardUnsynced           bool
	failPutState              string
	discardFailedPut          bool
	failBlockSync             bool
	attachmentReleaseCount    int
	failAttachmentReleaseAt   int
	cancelOnAttachmentRelease context.CancelFunc
}

func (store *richFaultDatastore) Put(ctx context.Context, key datastore.Key, value []byte) error {
	store.mu.Lock()
	if bytes.Contains(value, []byte(`"state":"releasing"`)) {
		store.attachmentReleaseCount++
		if store.failAttachmentReleaseAt != 0 && store.attachmentReleaseCount == store.failAttachmentReleaseAt {
			store.failAttachmentReleaseAt = 0
			cancel := store.cancelOnAttachmentRelease
			store.cancelOnAttachmentRelease = nil
			failure := store.err
			store.mu.Unlock()
			if cancel != nil {
				cancel()
			}
			return failure
		}
	}
	matchedState := ""
	for _, state := range []string{richStatePreparing, richStateLive, richStateDeleting, richStateDeleted, richStateAborted} {
		if bytes.Contains(value, []byte(`"state":"`+state+`"`)) {
			matchedState = state
			store.lastState = state
			store.lastKey = key
			previous, err := store.Batching.Get(ctx, key)
			store.priorFound = err == nil
			store.prior = append(store.prior[:0], previous...)
		}
	}
	fail := store.failPutState != "" && matchedState == store.failPutState
	if fail {
		store.failPutState = ""
	}
	discard := store.discardFailedPut
	failure := store.err
	store.mu.Unlock()
	if fail && discard {
		return failure
	}
	if err := store.Batching.Put(ctx, key, value); err != nil {
		return err
	}
	if fail {
		return failure
	}
	return nil
}

func (store *richFaultDatastore) Sync(ctx context.Context, prefix datastore.Key) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.failBlockSync && prefix.String() == datastore.NewKey("/blocks").String() {
		store.failBlockSync = false
		return store.err
	}
	if store.lastState == richStateLive {
		store.liveSyncs++
	}
	if store.failState != "" && store.lastState == store.failState {
		store.failState = ""
		if store.discardUnsynced {
			if store.priorFound {
				_ = store.Batching.Put(ctx, store.lastKey, store.prior)
			} else {
				_ = store.Batching.Delete(ctx, store.lastKey)
			}
		}
		return store.err
	}
	return store.Batching.Sync(ctx, prefix)
}
