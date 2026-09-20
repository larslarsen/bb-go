package social

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	format "github.com/ipfs/go-ipld-format"
	"github.com/larslarsen/bb-go/modern/attachment"
	"github.com/larslarsen/bb-go/modern/network"
	"github.com/larslarsen/bb-go/modern/publiccontent"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	richRecordVersion  = 1
	maxRichRecordBytes = 96 << 10
	maxRichRecordDepth = 12

	richStatePreparing = "preparing"
	richStateLive      = "live"
	richStateAborted   = "aborted"
	richStateDeleting  = "deleting"
	richStateDeleted   = "deleted"
)

var richRecordPrefix = datastore.NewKey("/bitbook/social/rich-post/v1/record")

type richClaim struct {
	AttachmentID string
	UploadID     attachment.ReferenceID
	TargetID     attachment.ReferenceID
	File         network.PublicFile
}

type richRecord struct {
	Version        int
	State          string
	ID             string
	RequestDigest  string
	Envelope       SignedPost
	Claims         []richClaim
	ClaimSignature []byte
}

type richClaimWire struct {
	AttachmentID string `json:"attachmentId"`
	UploadID     string `json:"uploadId"`
	TargetID     string `json:"targetId"`
	CID          string `json:"cid"`
	ByteLength   int64  `json:"byteLength"`
}

type richRecordWire struct {
	Version        int             `json:"version"`
	State          string          `json:"state"`
	ID             string          `json:"id"`
	RequestDigest  string          `json:"requestDigest"`
	Envelope       SignedPost      `json:"envelope"`
	Claims         []richClaimWire `json:"claims"`
	ClaimSignature []byte          `json:"claimSignature"`
}

type richClaimCertificateWire struct {
	ID            string          `json:"id"`
	PostCID       string          `json:"postCID"`
	RequestDigest string          `json:"requestDigest"`
	Claims        []richClaimWire `json:"claims"`
}

type richTombstoneWire struct {
	Version       int    `json:"version"`
	State         string `json:"state"`
	ID            string `json:"id"`
	RequestDigest string `json:"requestDigest"`
}

type preparedRichRequest struct {
	content          publiccontent.Content
	canonicalContent json.RawMessage
	status           string
	claims           []richClaim
	digest           string
}

func NewStoreWithAttachments(ctx context.Context, node *network.Node, attachments *attachment.Store, limits RichPostLimits) (*Store, error) {
	if ctx == nil {
		return nil, errors.New("nil rich post store context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if node == nil || node.Datastore == nil || node.Blockstore == nil || node.PrivateKey == nil {
		return nil, errors.Join(ErrRichPostUnavailable, errors.New("unavailable network node"))
	}
	if attachments == nil || !attachments.IsForNode(node) {
		return nil, errors.Join(ErrRichPostUnavailable, errors.New("attachment store does not belong to node"))
	}
	if err := validateRichLimits(limits); err != nil {
		return nil, err
	}
	store := &Store{
		node: node, now: time.Now, attachments: attachments, richLimits: limits,
		richRecords: make(map[string]richRecord),
	}
	if err := store.loadAndRecoverRich(ctx); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) AddRichPost(ctx context.Context, id string, content json.RawMessage, uploads map[string]attachment.ReferenceID) (SignedPost, error) {
	if ctx == nil {
		return SignedPost{}, ErrInvalidRichPost
	}
	if err := ctx.Err(); err != nil {
		return SignedPost{}, err
	}
	if !validRichID(id) || len(content) == 0 || len(content) > 65536 || len(uploads) > 8 {
		return SignedPost{}, ErrInvalidRichPost
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireRichLocked(); err != nil {
		return SignedPost{}, err
	}
	request, err := s.prepareRichRequestLocked(id, content, uploads)
	if err != nil {
		return SignedPost{}, err
	}
	if existing, ok := s.richRecords[id]; ok {
		if existing.RequestDigest != request.digest {
			return SignedPost{}, ErrRichPostConflict
		}
		switch existing.State {
		case richStateLive:
			if err := s.validateLiveClaimsLocked(ctx, existing); err != nil {
				return SignedPost{}, err
			}
			return cloneSignedPost(existing.Envelope), nil
		case richStateDeleted:
			return SignedPost{}, ErrRichPostDeleted
		case richStateAborted:
			return s.resumeAbortedRichLocked(ctx, existing)
		default:
			s.recoveryRequired = true
			return SignedPost{}, ErrRichPostRecoveryRequired
		}
	}
	legacy, err := s.loadPosts(ctx)
	if err != nil {
		return SignedPost{}, err
	}
	if postSlugExists("rich-"+id, legacy) || legacyPostIDExists(id, legacy) {
		return SignedPost{}, ErrRichPostConflict
	}
	if err := s.validateRichUploadsLocked(ctx, request.claims); err != nil {
		return SignedPost{}, err
	}
	claims, err := s.assignRichTargetsLocked(ctx, request.claims)
	if err != nil {
		return SignedPost{}, err
	}
	timestamp := s.now().UTC()
	if timestamp.Before(time.Unix(0, 0)) {
		return SignedPost{}, ErrInvalidRichPost
	}
	payload := parsedRichPayload{
		Schema: richPostSchema, ID: id, Author: s.node.ID(), Timestamp: timestamp,
		TimestampText: timestamp.Format(time.RFC3339Nano), Slug: "rich-" + id,
		PostType: "POST", Status: request.status, Content: request.content,
		CanonicalContent: slices.Clone(request.canonicalContent),
	}
	canonical, err := encodeRichPayload(payload)
	if err != nil {
		return SignedPost{}, err
	}
	signature, err := s.node.PrivateKey.Sign(canonical)
	if err != nil {
		return SignedPost{}, privateRichError("signing rich post", err)
	}
	publicKey, err := crypto.MarshalPublicKey(s.node.PrivateKey.GetPublic())
	if err != nil {
		return SignedPost{}, privateRichError("encoding rich post key", err)
	}
	envelope := SignedPost{Post: slices.Clone(canonical), Signature: slices.Clone(signature), PublicKey: slices.Clone(publicKey)}
	immutable, err := immutablePostBytes(envelope)
	if err != nil {
		return SignedPost{}, err
	}
	envelope.CID = blocks.NewBlock(immutable).Cid().String()
	record := richRecord{Version: richRecordVersion, State: richStatePreparing, ID: id, RequestDigest: request.digest, Envelope: envelope, Claims: claims}
	claimPreimage, err := richClaimCertificatePreimage(record)
	if err != nil {
		return SignedPost{}, err
	}
	record.ClaimSignature, err = s.node.PrivateKey.Sign(claimPreimage)
	if err != nil {
		return SignedPost{}, privateRichError("signing private rich post claims", err)
	}
	if err := s.checkRichRecordQuotaLocked(record); err != nil {
		return SignedPost{}, err
	}
	return s.createRichLocked(ctx, record, immutable)
}

func (s *Store) DeleteRichPost(ctx context.Context, id string) error {
	if ctx == nil || !validRichID(id) {
		return ErrInvalidRichPost
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireRichLocked(); err != nil {
		return err
	}
	record, ok := s.richRecords[id]
	if !ok {
		return datastore.ErrNotFound
	}
	if record.State == richStateDeleted {
		return nil
	}
	if record.State != richStateLive && record.State != richStateAborted {
		s.recoveryRequired = true
		return ErrRichPostRecoveryRequired
	}
	record.State = richStateDeleting
	if err := s.putRichRecordLocked(ctx, record); err != nil {
		s.recoveryRequired = true
		return err
	}
	for _, claim := range record.Claims {
		if err := s.attachments.Release(ctx, claim.TargetID); err != nil {
			s.recoveryRequired = true
			return privateRichError("releasing rich post claim", err)
		}
	}
	tombstone := richRecord{Version: richRecordVersion, State: richStateDeleted, ID: id, RequestDigest: record.RequestDigest}
	if err := s.putRichRecordLocked(ctx, tombstone); err != nil {
		s.recoveryRequired = true
		return err
	}
	return nil
}

func (s *Store) prepareRichRequestLocked(id string, raw json.RawMessage, uploads map[string]attachment.ReferenceID) (preparedRichRequest, error) {
	content, err := publiccontent.Parse(raw)
	if err != nil {
		return preparedRichRequest{}, errors.Join(ErrInvalidRichPost, err)
	}
	canonical, err := publiccontent.Marshal(content)
	if err != nil {
		return preparedRichRequest{}, errors.Join(ErrInvalidRichPost, err)
	}
	status, err := publiccontent.PlainText(content)
	if err != nil || len(content.Attachments) != len(uploads) {
		return preparedRichRequest{}, ErrInvalidRichPost
	}
	claims := make([]richClaim, len(content.Attachments))
	for index, descriptor := range content.Attachments {
		uploadID, ok := uploads[descriptor.AttachmentID]
		if !ok || uploadID == (attachment.ReferenceID{}) || descriptor.ByteLength > math.MaxInt64 {
			return preparedRichRequest{}, ErrInvalidRichPost
		}
		root, err := cid.Decode(descriptor.CID)
		if err != nil || root.String() != descriptor.CID {
			return preparedRichRequest{}, ErrInvalidRichPost
		}
		claims[index] = richClaim{AttachmentID: descriptor.AttachmentID, UploadID: uploadID, File: network.PublicFile{CID: root, ByteLength: int64(descriptor.ByteLength)}}
	}
	digest := digestRichRequest(id, canonical, claims)
	return preparedRichRequest{content: content, canonicalContent: slices.Clone(canonical), status: status, claims: claims, digest: digest}, nil
}

func (s *Store) validateRichUploadsLocked(ctx context.Context, claims []richClaim) error {
	for _, claim := range claims {
		reference, err := s.attachments.GetPublic(ctx, claim.UploadID)
		if err != nil {
			return privateRichError("checking rich post upload", errors.Join(ErrRichPostUnavailable, err))
		}
		if reference.File != claim.File {
			return errors.Join(ErrInvalidRichPost, attachment.ErrConflict)
		}
	}
	return nil
}

func (s *Store) assignRichTargetsLocked(ctx context.Context, claims []richClaim) ([]richClaim, error) {
	assigned := make([]richClaim, len(claims))
	used := make(map[attachment.ReferenceID]struct{}, len(claims)*2+len(s.richRecords))
	for _, claim := range claims {
		used[claim.UploadID] = struct{}{}
	}
	for _, record := range s.richRecords {
		for _, claim := range record.Claims {
			used[claim.TargetID] = struct{}{}
			used[claim.UploadID] = struct{}{}
		}
	}
	for index, claim := range claims {
		found := false
		for attempt := 0; attempt < 8; attempt++ {
			id, err := attachment.NewReferenceID()
			if err != nil {
				return nil, privateRichError("generating rich post claim", err)
			}
			if _, duplicate := used[id]; duplicate {
				continue
			}
			if _, err := s.attachments.GetPublic(ctx, id); err == nil {
				continue
			} else if !errors.Is(err, attachment.ErrNotFound) {
				return nil, privateRichError("checking rich post claim availability", err)
			}
			claim.TargetID = id
			used[id] = struct{}{}
			assigned[index] = claim
			found = true
			break
		}
		if !found {
			return nil, ErrRichPostConflict
		}
	}
	return assigned, nil
}

func (s *Store) createRichLocked(ctx context.Context, record richRecord, immutable []byte) (SignedPost, error) {
	if err := s.putRichRecordLocked(ctx, record); err != nil {
		s.recoveryRequired = true
		return SignedPost{}, err
	}
	for _, claim := range record.Claims {
		if _, err := s.attachments.ClonePublic(ctx, claim.UploadID, claim.TargetID, claim.File); err != nil {
			s.recoveryRequired = true
			return SignedPost{}, privateRichError("creating rich post claim", err)
		}
	}
	if err := ctx.Err(); err != nil {
		s.recoveryRequired = true
		return SignedPost{}, privateRichError("creating rich post", err)
	}
	stored, err := s.node.Put(ctx, immutable)
	if err != nil {
		s.recoveryRequired = true
		return SignedPost{}, privateRichError("storing rich post envelope", err)
	}
	if stored.String() != record.Envelope.CID {
		s.recoveryRequired = true
		return SignedPost{}, ErrCorruptRichPostState
	}
	if err := s.node.Datastore.Sync(ctx, datastore.NewKey("/blocks")); err != nil {
		s.recoveryRequired = true
		return SignedPost{}, privateRichError("syncing rich post envelope", err)
	}
	record.State = richStateLive
	if err := s.putRichRecordLocked(ctx, record); err != nil {
		s.recoveryRequired = true
		return SignedPost{}, err
	}
	return cloneSignedPost(record.Envelope), nil
}

func (s *Store) resumeAbortedRichLocked(ctx context.Context, record richRecord) (SignedPost, error) {
	for _, claim := range record.Claims {
		if _, err := s.attachments.GetPublic(ctx, claim.TargetID); err == nil {
			s.recoveryRequired = true
			return SignedPost{}, ErrRichPostRecoveryRequired
		} else if !errors.Is(err, attachment.ErrNotFound) {
			return SignedPost{}, privateRichError("checking aborted rich post claim", err)
		}
	}
	immutable, err := immutablePostBytes(record.Envelope)
	if err != nil {
		return SignedPost{}, errors.Join(ErrCorruptRichPostState, err)
	}
	record.State = richStatePreparing
	return s.createRichLocked(ctx, record, immutable)
}

func (s *Store) requireRichLocked() error {
	if s.attachments == nil || !s.attachments.IsForNode(s.node) {
		return ErrRichPostUnavailable
	}
	if s.recoveryRequired {
		return ErrRichPostRecoveryRequired
	}
	return nil
}

func (s *Store) validateLiveClaimsLocked(ctx context.Context, record richRecord) error {
	for _, claim := range record.Claims {
		got, err := s.attachments.GetPublic(ctx, claim.TargetID)
		if err != nil || got.File != claim.File {
			if err == nil {
				err = attachment.ErrConflict
			}
			return privateRichError("rich post claim unavailable", errors.Join(ErrRichPostUnavailable, err))
		}
	}
	return nil
}

func (s *Store) liveRichPostsLocked(ctx context.Context) ([]richRecord, error) {
	if s.attachments == nil {
		return nil, nil
	}
	if err := s.requireRichLocked(); err != nil {
		return nil, err
	}
	result := make([]richRecord, 0, len(s.richRecords))
	for _, record := range s.richRecords {
		if record.State != richStateLive {
			continue
		}
		if err := s.validateLiveClaimsLocked(ctx, record); err != nil {
			return nil, err
		}
		result = append(result, cloneRichRecord(record))
	}
	return result, nil
}

func (s *Store) putRichRecordLocked(ctx context.Context, record richRecord) error {
	encoded, err := encodeRichRecord(record)
	if err != nil {
		return err
	}
	old, exists := s.richRecords[record.ID]
	oldSize := 0
	if exists {
		oldEncoded, err := encodeRichRecord(old)
		if err != nil {
			return err
		}
		oldSize = len(oldEncoded)
	} else if len(s.richRecords) >= s.richLimits.MaxRecords {
		return ErrRichPostQuota
	}
	prospective := s.richBytes - int64(oldSize) + int64(len(encoded))
	if prospective < 0 || prospective > s.richLimits.MaxBytes {
		return ErrRichPostQuota
	}
	if err := s.node.Datastore.Put(ctx, richRecordKey(record.ID), encoded); err != nil {
		return privateRichError("writing rich post journal", err)
	}
	if err := s.node.Datastore.Sync(ctx, richRecordPrefix); err != nil {
		return privateRichError("syncing rich post journal", err)
	}
	s.richRecords[record.ID] = cloneRichRecord(record)
	s.richBytes = prospective
	return nil
}

func (s *Store) checkRichRecordQuotaLocked(record richRecord) error {
	encoded, err := encodeRichRecord(record)
	if err != nil {
		return err
	}
	oldSize := 0
	if old, exists := s.richRecords[record.ID]; exists {
		oldEncoded, err := encodeRichRecord(old)
		if err != nil {
			return err
		}
		oldSize = len(oldEncoded)
	} else if len(s.richRecords) >= s.richLimits.MaxRecords {
		return ErrRichPostQuota
	}
	prospective := s.richBytes - int64(oldSize) + int64(len(encoded))
	if prospective < 0 || prospective > s.richLimits.MaxBytes {
		return ErrRichPostQuota
	}
	return nil
}

func (s *Store) loadAndRecoverRich(ctx context.Context) error {
	results, err := s.node.Datastore.Query(ctx, query.Query{Prefix: richRecordPrefix.String(), Limit: s.richLimits.MaxRecords + 1})
	if err != nil {
		return privateRichError("querying rich post journal", errors.Join(ErrCorruptRichPostState, err))
	}
	resultsClosed := false
	defer func() {
		if !resultsClosed {
			_ = results.Close()
		}
	}()
	targetOwners := make(map[attachment.ReferenceID]string)
	uploadIDs := make(map[attachment.ReferenceID]struct{})
	for result := range results.Next() {
		if result.Error != nil {
			return privateRichError("reading rich post journal", errors.Join(ErrCorruptRichPostState, result.Error))
		}
		if len(s.richRecords) >= s.richLimits.MaxRecords || len(result.Value) == 0 || len(result.Value) > maxRichRecordBytes {
			return errors.Join(ErrCorruptRichPostState, ErrRichPostQuota)
		}
		idText := strings.TrimPrefix(result.Key, richRecordPrefix.String()+"/")
		if idText == result.Key || strings.Contains(idText, "/") || !validRichID(idText) {
			return ErrCorruptRichPostState
		}
		record, err := decodeRichRecord(result.Value)
		if err != nil || record.ID != idText {
			return errors.Join(ErrCorruptRichPostState, err)
		}
		if _, duplicate := s.richRecords[record.ID]; duplicate {
			return ErrCorruptRichPostState
		}
		if err := s.validateLoadedRichRecord(record, targetOwners, uploadIDs); err != nil {
			return err
		}
		s.richBytes += int64(len(result.Value))
		if s.richBytes > s.richLimits.MaxBytes {
			return errors.Join(ErrCorruptRichPostState, ErrRichPostQuota)
		}
		s.richRecords[record.ID] = cloneRichRecord(record)
	}
	closeErr := results.Close()
	resultsClosed = true
	if closeErr != nil {
		return privateRichError("closing rich post journal query", errors.Join(ErrCorruptRichPostState, closeErr))
	}
	for targetID := range targetOwners {
		if _, reservedUpload := uploadIDs[targetID]; reservedUpload {
			return ErrCorruptRichPostState
		}
	}
	for _, record := range s.richRecords {
		if err := s.validateLoadedClaimState(ctx, record); err != nil {
			return err
		}
	}
	for id, record := range s.richRecords {
		switch record.State {
		case richStatePreparing:
			for _, claim := range record.Claims {
				if err := s.attachments.Release(ctx, claim.TargetID); err != nil {
					return privateRichError("recovering prepared rich post", err)
				}
			}
			record.State = richStateAborted
			if err := s.putRichRecordLocked(ctx, record); err != nil {
				return err
			}
		case richStateDeleting:
			for _, claim := range record.Claims {
				if err := s.attachments.Release(ctx, claim.TargetID); err != nil {
					return privateRichError("recovering deleting rich post", err)
				}
			}
			if err := s.putRichRecordLocked(ctx, richRecord{Version: richRecordVersion, State: richStateDeleted, ID: id, RequestDigest: record.RequestDigest}); err != nil {
				return err
			}
		case richStateLive:
			if err := s.recoverLiveEnvelope(ctx, record); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) validateLoadedRichRecord(record richRecord, targetOwners map[attachment.ReferenceID]string, uploadIDs map[attachment.ReferenceID]struct{}) error {
	if record.State == richStateDeleted {
		return nil
	}
	if record.Envelope.CID == "" {
		return ErrCorruptRichPostState
	}
	payload, err := validatePersistedEnvelope(s.node.ID(), record.Envelope)
	if err != nil || payload.ID != record.ID || len(payload.Content.Attachments) != len(record.Claims) {
		return errors.Join(ErrCorruptRichPostState, err)
	}
	for index, claim := range record.Claims {
		descriptor := payload.Content.Attachments[index]
		if claim.AttachmentID != descriptor.AttachmentID || claim.File.CID.String() != descriptor.CID ||
			claim.File.ByteLength != int64(descriptor.ByteLength) || claim.UploadID == claim.TargetID ||
			claim.UploadID == (attachment.ReferenceID{}) || claim.TargetID == (attachment.ReferenceID{}) {
			return ErrCorruptRichPostState
		}
		if _, duplicate := targetOwners[claim.TargetID]; duplicate {
			return ErrCorruptRichPostState
		}
		targetOwners[claim.TargetID] = record.ID
		uploadIDs[claim.UploadID] = struct{}{}
	}
	if digestRichRequest(record.ID, payload.CanonicalContent, record.Claims) != record.RequestDigest {
		return ErrCorruptRichPostState
	}
	preimage, err := richClaimCertificatePreimage(record)
	if err != nil {
		return errors.Join(ErrCorruptRichPostState, err)
	}
	publicKey, err := crypto.UnmarshalPublicKey(record.Envelope.PublicKey)
	if err != nil {
		return errors.Join(ErrCorruptRichPostState, err)
	}
	valid, err := publicKey.Verify(preimage, record.ClaimSignature)
	if err != nil || !valid {
		return ErrCorruptRichPostState
	}
	switch record.State {
	case richStateLive, richStateAborted, richStatePreparing, richStateDeleting:
	default:
		return ErrCorruptRichPostState
	}
	return nil
}

func (s *Store) validateLoadedClaimState(ctx context.Context, record richRecord) error {
	if record.State == richStateDeleted {
		return nil
	}
	for _, claim := range record.Claims {
		reference, err := s.attachments.GetPublic(ctx, claim.TargetID)
		switch record.State {
		case richStateLive:
			if err != nil || reference.File != claim.File {
				return privateRichError("validating live rich post claim", errors.Join(ErrCorruptRichPostState, err))
			}
		case richStateAborted:
			if err == nil || !errors.Is(err, attachment.ErrNotFound) {
				return ErrCorruptRichPostState
			}
		case richStatePreparing, richStateDeleting:
			if err == nil {
				if reference.File != claim.File {
					return ErrCorruptRichPostState
				}
			} else if !errors.Is(err, attachment.ErrNotFound) {
				return privateRichError("validating partial rich post claim", errors.Join(ErrCorruptRichPostState, err))
			}
		}
	}
	return nil
}

func validatePersistedEnvelope(author peer.ID, post SignedPost) (parsedRichPayload, error) {
	publicKey, err := crypto.UnmarshalPublicKey(post.PublicKey)
	if err != nil {
		return parsedRichPayload{}, err
	}
	claimed, err := peer.IDFromPublicKey(publicKey)
	if err != nil || claimed != author {
		return parsedRichPayload{}, ErrInvalidRichPost
	}
	valid, err := publicKey.Verify(post.Post, post.Signature)
	if err != nil || !valid {
		return parsedRichPayload{}, ErrInvalidRichPost
	}
	return validateRichEnvelope(author, post)
}

func (s *Store) recoverLiveEnvelope(ctx context.Context, record richRecord) error {
	immutable, err := immutablePostBytes(record.Envelope)
	if err != nil {
		return errors.Join(ErrCorruptRichPostState, err)
	}
	id, err := cid.Decode(record.Envelope.CID)
	if err != nil {
		return ErrCorruptRichPostState
	}
	block, err := s.node.Blockstore.Get(ctx, id)
	if err == nil {
		if !bytes.Equal(block.RawData(), immutable) {
			return ErrCorruptRichPostState
		}
		return nil
	}
	if !format.IsNotFound(err) {
		return privateRichError("checking rich post block", err)
	}
	stored, err := s.node.Put(ctx, immutable)
	if err != nil || !stored.Equals(id) {
		return privateRichError("recovering rich post block", errors.Join(ErrCorruptRichPostState, err))
	}
	if err := s.node.Datastore.Sync(ctx, datastore.NewKey("/blocks")); err != nil {
		return privateRichError("syncing recovered rich post block", err)
	}
	return nil
}

func encodeRichRecord(record richRecord) ([]byte, error) {
	if record.Version != richRecordVersion || !validRichID(record.ID) || !validDigest(record.RequestDigest) {
		return nil, ErrCorruptRichPostState
	}
	var encoded []byte
	var err error
	if record.State == richStateDeleted {
		if len(record.Envelope.Post) != 0 || len(record.Claims) != 0 || len(record.ClaimSignature) != 0 {
			return nil, ErrCorruptRichPostState
		}
		encoded, err = json.Marshal(richTombstoneWire{Version: record.Version, State: record.State, ID: record.ID, RequestDigest: record.RequestDigest})
	} else {
		if record.State != richStatePreparing && record.State != richStateLive && record.State != richStateAborted && record.State != richStateDeleting {
			return nil, ErrCorruptRichPostState
		}
		if len(record.ClaimSignature) == 0 || len(record.ClaimSignature) > maxRichSignatureBytes {
			return nil, ErrCorruptRichPostState
		}
		encoded, err = json.Marshal(richRecordWire{Version: record.Version, State: record.State, ID: record.ID, RequestDigest: record.RequestDigest, Envelope: cloneSignedPost(record.Envelope), Claims: richClaimWires(record.Claims), ClaimSignature: slices.Clone(record.ClaimSignature)})
	}
	if err != nil || len(encoded) == 0 || len(encoded) > maxRichRecordBytes {
		return nil, ErrCorruptRichPostState
	}
	return encoded, nil
}

func decodeRichRecord(encoded []byte) (richRecord, error) {
	if len(encoded) == 0 || len(encoded) > maxRichRecordBytes || !utf8.Valid(encoded) || !validRichJSONStringEscapes(encoded) || preflightRichJSON(encoded, maxRichRecordDepth) != nil {
		return richRecord{}, ErrCorruptRichPostState
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return richRecord{}, ErrCorruptRichPostState
	}
	var state string
	if decodeRequiredJSON(fields["state"], &state) != nil {
		return richRecord{}, ErrCorruptRichPostState
	}
	if state == richStateDeleted {
		if !exactFieldSet(fields, "version", "state", "id", "requestDigest") {
			return richRecord{}, ErrCorruptRichPostState
		}
		var wire richTombstoneWire
		if decodeExactJSON(encoded, &wire) != nil {
			return richRecord{}, ErrCorruptRichPostState
		}
		record := richRecord{Version: wire.Version, State: wire.State, ID: wire.ID, RequestDigest: wire.RequestDigest}
		canonical, err := encodeRichRecord(record)
		if err != nil || !bytes.Equal(canonical, encoded) {
			return richRecord{}, ErrCorruptRichPostState
		}
		return record, nil
	}
	if !exactFieldSet(fields, "version", "state", "id", "requestDigest", "envelope", "claims", "claimSignature") {
		return richRecord{}, ErrCorruptRichPostState
	}
	var wire richRecordWire
	if decodeExactJSON(encoded, &wire) != nil || len(wire.Claims) > 8 {
		return richRecord{}, ErrCorruptRichPostState
	}
	if len(wire.ClaimSignature) == 0 || len(wire.ClaimSignature) > maxRichSignatureBytes {
		return richRecord{}, ErrCorruptRichPostState
	}
	record := richRecord{Version: wire.Version, State: wire.State, ID: wire.ID, RequestDigest: wire.RequestDigest, Envelope: cloneSignedPost(wire.Envelope), Claims: make([]richClaim, len(wire.Claims)), ClaimSignature: slices.Clone(wire.ClaimSignature)}
	for index, claim := range wire.Claims {
		uploadID, err := attachment.ParseReferenceID(claim.UploadID)
		if err != nil {
			return richRecord{}, ErrCorruptRichPostState
		}
		targetID, err := attachment.ParseReferenceID(claim.TargetID)
		if err != nil {
			return richRecord{}, ErrCorruptRichPostState
		}
		root, err := cid.Decode(claim.CID)
		if err != nil || root.String() != claim.CID || claim.ByteLength < 0 {
			return richRecord{}, ErrCorruptRichPostState
		}
		record.Claims[index] = richClaim{AttachmentID: claim.AttachmentID, UploadID: uploadID, TargetID: targetID, File: network.PublicFile{CID: root, ByteLength: claim.ByteLength}}
	}
	canonical, err := encodeRichRecord(record)
	if err != nil || !bytes.Equal(canonical, encoded) {
		return richRecord{}, ErrCorruptRichPostState
	}
	return record, nil
}

func digestRichRequest(id string, canonical []byte, claims []richClaim) string {
	hash := sha256.New()
	hash.Write([]byte("bitbook-rich-post-request-v1\x00"))
	writeDigestFrame(hash, []byte(id))
	writeDigestFrame(hash, canonical)
	var count [4]byte
	binary.BigEndian.PutUint32(count[:], uint32(len(claims)))
	hash.Write(count[:])
	for _, claim := range claims {
		writeDigestFrame(hash, []byte(claim.AttachmentID))
		writeDigestFrame(hash, claim.UploadID[:])
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func writeDigestFrame(writer io.Writer, value []byte) {
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(value)))
	_, _ = writer.Write(size[:])
	_, _ = writer.Write(value)
}

func validateRichLimits(limits RichPostLimits) error {
	if limits.MaxRecords < 1 || limits.MaxRecords > 4096 || limits.MaxBytes < 96<<10 || limits.MaxBytes > 64<<20 {
		return ErrInvalidRichPost
	}
	return nil
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for index := range value {
		if (value[index] < '0' || value[index] > '9') && (value[index] < 'a' || value[index] > 'f') {
			return false
		}
	}
	return true
}

func exactFieldSet(fields map[string]json.RawMessage, names ...string) bool {
	if len(fields) != len(names) {
		return false
	}
	for _, name := range names {
		if _, ok := fields[name]; !ok {
			return false
		}
	}
	return true
}

func richRecordKey(id string) datastore.Key { return richRecordPrefix.ChildString(id) }

func cloneRichRecord(record richRecord) richRecord {
	record.Envelope = cloneSignedPost(record.Envelope)
	record.Claims = slices.Clone(record.Claims)
	record.ClaimSignature = slices.Clone(record.ClaimSignature)
	return record
}

func richClaimWires(claims []richClaim) []richClaimWire {
	wires := make([]richClaimWire, len(claims))
	for index, claim := range claims {
		wires[index] = richClaimWire{AttachmentID: claim.AttachmentID, UploadID: claim.UploadID.String(), TargetID: claim.TargetID.String(), CID: claim.File.CID.String(), ByteLength: claim.File.ByteLength}
	}
	return wires
}

func richClaimCertificatePreimage(record richRecord) ([]byte, error) {
	if !validRichID(record.ID) || record.Envelope.CID == "" || !validDigest(record.RequestDigest) {
		return nil, ErrCorruptRichPostState
	}
	certificate, err := json.Marshal(richClaimCertificateWire{ID: record.ID, PostCID: record.Envelope.CID, RequestDigest: record.RequestDigest, Claims: richClaimWires(record.Claims)})
	if err != nil {
		return nil, ErrCorruptRichPostState
	}
	preimage := make([]byte, 0, len("bitbook.local-post-claims/1")+1+len(certificate))
	preimage = append(preimage, "bitbook.local-post-claims/1"...)
	preimage = append(preimage, 0)
	preimage = append(preimage, certificate...)
	return preimage, nil
}

type privateRichCause struct {
	message string
	cause   error
}

func (err privateRichCause) Error() string { return err.message }
func (err privateRichCause) Unwrap() error { return err.cause }

func privateRichError(message string, cause error) error {
	if cause == nil {
		return errors.New(message)
	}
	return privateRichCause{message: message, cause: cause}
}
