package attachment

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
	"sync"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/ipld/merkledag"
	unixfspb "github.com/ipfs/boxo/ipld/unixfs/pb"
	pinning "github.com/ipfs/boxo/pinning/pinner"
	"github.com/ipfs/boxo/pinning/pinner/dspinner"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/ipfs/go-datastore/query"
	format "github.com/ipfs/go-ipld-format"
	"github.com/larslarsen/bb-go/modern/network"
	mh "github.com/multiformats/go-multihash"
	"google.golang.org/protobuf/proto"
)

const (
	pinNamePrefix                  = "bitbook-attachment-public-v1:"
	maxEncodedFileBlockBytes       = 2 << 20
	maxFileLinks                   = 1024
	maxFileDepth                   = 32
	maxFileNodeOccurrences         = 4096
	maxFileEncodedBytes      int64 = 256 << 20
)

var (
	referencePrefix = datastore.NewKey("/bitbook/attachment/public/v1/reference")
	pinnerPrefix    = datastore.NewKey("/bitbook/attachment/public/v1/pinner")
	pinnerDirtyKey  = datastore.NewKey("/pins/state/dirty")
)

type rootKey struct {
	cid    string
	length int64
}

type reservation struct {
	bytes int64
	root  *rootKey
}

// Store owns attachment reference state and its package-scoped pinner.
type Store struct {
	node    *network.Node
	records datastore.Batching
	pinner  pinning.Pinner
	limits  Limits

	mu            sync.Mutex
	entries       map[ReferenceID]persistedRecord
	roots         map[rootKey]int
	accounted     map[rootKey]struct{}
	pending       map[ReferenceID]reservation
	pendingRoots  map[rootKey]int
	logicalBytes  int64
	reservedRefs  int
	reservedBytes int64

	lifeMu    sync.Mutex
	closed    bool
	stopCtx   context.Context
	stop      context.CancelFunc
	wg        sync.WaitGroup
	closeOnce sync.Once
	closeErr  error
}

// Open loads and recovers the attachment store over the node's existing storage.
func Open(ctx context.Context, node *network.Node, limits Limits) (*Store, error) {
	if ctx == nil {
		return nil, errors.New("nil attachment store context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if node == nil || isNilInterface(node.Datastore) || isNilInterface(node.Blockstore) || node.Bitswap == nil {
		return nil, errors.New("nil or unavailable network node")
	}
	if err := validateLimits(limits); err != nil {
		return nil, err
	}
	dagService := merkledag.NewDAGService(blockservice.New(node.Blockstore, node.Bitswap))
	pinnerStore := namespace.Wrap(node.Datastore, pinnerPrefix)
	if err := validatePinnerMarker(ctx, pinnerStore); err != nil {
		return nil, err
	}
	pinner, err := dspinner.New(ctx, pinnerStore, dagService)
	if err != nil {
		return nil, fmt.Errorf("opening attachment pinner: %w", err)
	}
	stopCtx, stop := context.WithCancel(context.Background())
	store := &Store{
		node:         node,
		records:      node.Datastore,
		pinner:       pinner,
		limits:       limits,
		entries:      make(map[ReferenceID]persistedRecord),
		roots:        make(map[rootKey]int),
		accounted:    make(map[rootKey]struct{}),
		pending:      make(map[ReferenceID]reservation),
		pendingRoots: make(map[rootKey]int),
		stopCtx:      stopCtx,
		stop:         stop,
	}
	if err := store.loadAndRecover(ctx); err != nil {
		stop()
		_ = pinner.Close()
		return nil, err
	}
	return store, nil
}

// ImportPublic imports a public file and retains it under id.
func (s *Store) ImportPublic(ctx context.Context, id ReferenceID, src io.Reader, maxBytes int64) (PublicReference, error) {
	opctx, done, err := s.begin(ctx)
	if err != nil {
		return PublicReference{}, err
	}
	defer done()
	if err := validateReferenceID(id); err != nil {
		return PublicReference{}, err
	}
	if maxBytes < 1 || maxBytes > maxPublicFileSize {
		return PublicReference{}, fmt.Errorf("%w: maxBytes must be between 1 and %d", network.ErrInvalidPublicFile, maxPublicFileSize)
	}
	if isNilInterface(src) {
		return PublicReference{}, fmt.Errorf("%w: nil source reader", network.ErrInvalidPublicFile)
	}

	existing, reserved, err := s.reserve(id, maxBytes, nil, false)
	if err != nil || !reserved {
		return existing, err
	}
	converted := false
	defer func() {
		if !converted {
			s.releaseReservation(id)
		}
	}()
	file, err := s.node.ImportPublicFile(opctx, src, maxBytes)
	if err != nil {
		return PublicReference{}, err
	}
	reference, converted, err := s.finishRetain(opctx, id, file)
	return reference, err
}

// RetainPublic validates and retains an existing public file under id.
func (s *Store) RetainPublic(ctx context.Context, id ReferenceID, file network.PublicFile) (PublicReference, error) {
	opctx, done, err := s.begin(ctx)
	if err != nil {
		return PublicReference{}, err
	}
	defer done()
	if err := validateReferenceID(id); err != nil {
		return PublicReference{}, err
	}
	if err := validatePublicDescriptor(file); err != nil {
		return PublicReference{}, err
	}
	key := fileRootKey(file)
	existing, reserved, err := s.reserve(id, file.ByteLength, &key, true)
	if err != nil || !reserved {
		return existing, err
	}
	converted := false
	defer func() {
		if !converted {
			s.releaseReservation(id)
		}
	}()
	if _, err := s.node.CopyPublicFile(opctx, file, io.Discard); err != nil {
		return PublicReference{}, err
	}
	reference, converted, err := s.finishRetain(opctx, id, file)
	return reference, err
}

// GetPublic returns a ready public attachment reference.
func (s *Store) GetPublic(ctx context.Context, id ReferenceID) (PublicReference, error) {
	_, done, err := s.begin(ctx)
	if err != nil {
		return PublicReference{}, err
	}
	defer done()
	if err := validateReferenceID(id); err != nil {
		return PublicReference{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.entries[id]
	if !ok {
		if _, pending := s.pending[id]; pending {
			return PublicReference{}, fmt.Errorf("%w: %s", ErrNotReady, id)
		}
		return PublicReference{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if record.State != stateReady {
		return PublicReference{}, fmt.Errorf("%w: %s is %s", ErrNotReady, id, record.State)
	}
	return publicReference(record), nil
}

// Release removes one retention claim. Releasing an absent ID succeeds.
func (s *Store) Release(ctx context.Context, id ReferenceID) error {
	opctx, done, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer done()
	if err := validateReferenceID(id); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, pending := s.pending[id]; pending {
		return fmt.Errorf("%w: %s", ErrNotReady, id)
	}
	record, ok := s.entries[id]
	if !ok {
		return nil
	}
	if record.State == stateRetaining {
		return fmt.Errorf("%w: %s is retaining", ErrNotReady, id)
	}
	if record.State == stateReady {
		record.State = stateReleasing
		written, err := s.putRecord(opctx, record)
		if written {
			s.entries[id] = record
		}
		if err != nil {
			return err
		}
	} else {
		if _, err := s.putRecord(opctx, record); err != nil {
			return err
		}
	}
	if s.roots[recordRootKey(record)] > 1 {
		if err := s.ensureRootPinned(opctx, record.CID); err != nil {
			return err
		}
	} else if err := s.unpinRoot(opctx, record.CID); err != nil {
		return err
	}
	if err := s.deleteRecord(opctx, id); err != nil {
		return err
	}
	s.removeEntryLocked(record)
	return nil
}

func validatePinnerMarker(ctx context.Context, store datastore.Datastore) error {
	marker, err := store.Get(ctx, pinnerDirtyKey)
	if err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("reading attachment pinner marker: %w", err)
	}
	if len(marker) != 1 || (marker[0] != 0 && marker[0] != 1) {
		return fmt.Errorf("%w: invalid attachment pinner dirty marker", ErrCorruptState)
	}
	return nil
}

// Close cancels admitted work, waits for it, and closes only the owned pinner.
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		s.lifeMu.Lock()
		s.closed = true
		s.stop()
		s.lifeMu.Unlock()
		s.wg.Wait()
		s.closeErr = s.pinner.Close()
	})
	return s.closeErr
}

func (s *Store) begin(parent context.Context) (context.Context, func(), error) {
	if s == nil {
		return nil, nil, ErrClosed
	}
	if parent == nil {
		return nil, nil, errors.New("nil attachment operation context")
	}
	if err := parent.Err(); err != nil {
		return nil, nil, err
	}
	s.lifeMu.Lock()
	if s.closed {
		s.lifeMu.Unlock()
		return nil, nil, ErrClosed
	}
	s.wg.Add(1)
	opctx, cancel := context.WithCancel(parent)
	stopStoreCancel := context.AfterFunc(s.stopCtx, cancel)
	s.lifeMu.Unlock()
	done := func() {
		stopStoreCancel()
		cancel()
		s.wg.Done()
	}
	return opctx, done, nil
}

func (s *Store) reserve(id ReferenceID, bytes int64, knownRoot *rootKey, requireMatch bool) (PublicReference, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if record, ok := s.entries[id]; ok {
		if record.State != stateReady {
			return PublicReference{}, false, fmt.Errorf("%w: %s is %s", ErrNotReady, id, record.State)
		}
		reference := publicReference(record)
		if requireMatch && (knownRoot == nil || *knownRoot != recordRootKey(record)) {
			return PublicReference{}, false, fmt.Errorf("%w: reference %s already names another file", ErrConflict, id)
		}
		return reference, false, nil
	}
	if _, exists := s.pending[id]; exists {
		return PublicReference{}, false, fmt.Errorf("%w: %s has an active operation", ErrNotReady, id)
	}
	if len(s.entries)+s.reservedRefs >= s.limits.MaxReferences {
		return PublicReference{}, false, ErrQuotaExceeded
	}
	reserved := reservation{}
	if knownRoot != nil {
		root := *knownRoot
		reserved.root = &root
		if _, accounted := s.accounted[root]; !accounted {
			if bytes < 0 || s.logicalBytes > s.limits.MaxLogicalBytes-s.reservedBytes ||
				bytes > s.limits.MaxLogicalBytes-s.logicalBytes-s.reservedBytes {
				return PublicReference{}, false, ErrQuotaExceeded
			}
			s.accounted[root] = struct{}{}
			s.logicalBytes += bytes
		}
		s.pendingRoots[root]++
	} else {
		if bytes < 0 || s.logicalBytes > s.limits.MaxLogicalBytes-s.reservedBytes ||
			bytes > s.limits.MaxLogicalBytes-s.logicalBytes-s.reservedBytes {
			return PublicReference{}, false, ErrQuotaExceeded
		}
		reserved.bytes = bytes
		s.reservedBytes += bytes
	}
	s.reservedRefs++
	s.pending[id] = reserved
	return PublicReference{}, true, nil
}

func (s *Store) releaseReservation(id ReferenceID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	reservation, ok := s.pending[id]
	if !ok {
		return
	}
	delete(s.pending, id)
	s.reservedRefs--
	s.reservedBytes -= reservation.bytes
	if reservation.root != nil {
		key := *reservation.root
		s.pendingRoots[key]--
		if s.pendingRoots[key] == 0 {
			delete(s.pendingRoots, key)
			if s.roots[key] == 0 {
				delete(s.accounted, key)
				s.logicalBytes -= key.length
			}
		}
	}
}

func (s *Store) finishRetain(ctx context.Context, id ReferenceID, file network.PublicFile) (PublicReference, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return PublicReference{}, false, err
	}
	if err := s.checkPinOwnership(ctx, file.CID); err != nil {
		return PublicReference{}, false, err
	}
	record := persistedRecord{Version: recordVersion, State: stateRetaining, ID: id, CID: file.CID, ByteLength: file.ByteLength}
	written, err := s.putRecord(ctx, record)
	if written {
		s.convertReservationLocked(id, record)
	}
	if err != nil {
		return PublicReference{}, written, err
	}
	if err := s.ensureRootPinned(ctx, file.CID); err != nil {
		return PublicReference{}, true, fmt.Errorf("pinning retained public file %s: %w", file.CID, err)
	}
	record.State = stateReady
	if _, err := s.putRecord(ctx, record); err != nil {
		return PublicReference{}, true, err
	}
	s.entries[id] = record
	return publicReference(record), true, nil
}

func (s *Store) loadAndRecover(ctx context.Context) error {
	results, err := s.records.Query(ctx, query.Query{Prefix: referencePrefix.String()})
	if err != nil {
		return fmt.Errorf("querying attachment records: %w", err)
	}
	defer results.Close()
	for result := range results.Next() {
		if result.Error != nil {
			return fmt.Errorf("reading attachment records: %w", result.Error)
		}
		if len(s.entries) >= s.limits.MaxReferences {
			return errors.Join(ErrCorruptState, ErrQuotaExceeded, errors.New("persisted reference count exceeds configured limit"))
		}
		idText := strings.TrimPrefix(result.Key, referencePrefix.String()+"/")
		if idText == result.Key || strings.Contains(idText, "/") {
			return fmt.Errorf("%w: invalid reference key %q", ErrCorruptState, result.Key)
		}
		id, err := ParseReferenceID(idText)
		if err != nil {
			return fmt.Errorf("%w: key %q: %v", ErrCorruptState, result.Key, err)
		}
		record, err := decodeRecord(result.Value)
		if err != nil {
			return fmt.Errorf("reference %s: %w", id, err)
		}
		if record.ID != id {
			return fmt.Errorf("%w: key ID %s does not match record ID %s", ErrCorruptState, id, record.ID)
		}
		if _, duplicate := s.entries[id]; duplicate {
			return fmt.Errorf("%w: duplicate reference %s", ErrCorruptState, id)
		}
		s.addEntryLocked(record)
		if s.logicalBytes > s.limits.MaxLogicalBytes {
			return errors.Join(ErrCorruptState, ErrQuotaExceeded, fmt.Errorf("persisted logical bytes exceed %d", s.limits.MaxLogicalBytes))
		}
	}
	for _, record := range s.entries {
		if err := verifyLocalPublicFile(ctx, s.node, network.PublicFile{CID: record.CID, ByteLength: record.ByteLength}); err != nil {
			return fmt.Errorf("%w: reference %s root %s: %w", ErrCorruptState, record.ID, record.CID, err)
		}
	}
	pins, err := s.recursivePins(ctx)
	if err != nil {
		return err
	}
	for _, record := range s.entries {
		if record.State != stateReady {
			continue
		}
		name, pinned := pins[record.CID.String()]
		if !pinned || name != pinName(record.CID) {
			return fmt.Errorf("%w: ready reference %s root %s is not recursively pinned with its package name", ErrCorruptState, record.ID, record.CID)
		}
	}
	for id, record := range s.entries {
		switch record.State {
		case stateRetaining:
			if err := s.ensureRootPinned(ctx, record.CID); err != nil {
				return fmt.Errorf("recovering retaining reference %s: %w", id, err)
			}
			record.State = stateReady
			if _, err := s.putRecord(ctx, record); err != nil {
				return err
			}
			s.entries[id] = record
		case stateReleasing:
			if s.roots[recordRootKey(record)] > 1 {
				if err := s.ensureRootPinned(ctx, record.CID); err != nil {
					return fmt.Errorf("recovering shared releasing reference %s: %w", id, err)
				}
			} else if err := s.unpinRoot(ctx, record.CID); err != nil {
				return fmt.Errorf("recovering releasing reference %s: %w", id, err)
			}
			if err := s.deleteRecord(ctx, id); err != nil {
				return err
			}
			s.removeEntryLocked(record)
		}
	}
	pins, err = s.recursivePins(ctx)
	if err != nil {
		return err
	}
	for encodedCID, name := range pins {
		if !strings.HasPrefix(name, pinNamePrefix) {
			continue
		}
		root, err := cid.Decode(encodedCID)
		if err != nil {
			return fmt.Errorf("%w: pinner returned invalid CID %q", ErrCorruptState, encodedCID)
		}
		tracked := false
		for key := range s.roots {
			if key.cid == encodedCID {
				tracked = true
				break
			}
		}
		if !tracked {
			if err := s.unpinRoot(ctx, root); err != nil {
				return fmt.Errorf("removing orphan attachment pin %s: %w", root, err)
			}
		}
	}
	return nil
}

func (s *Store) recursivePins(ctx context.Context) (map[string]string, error) {
	pins := make(map[string]string)
	for result := range s.pinner.RecursiveKeys(ctx, true) {
		if result.Err != nil {
			return nil, fmt.Errorf("listing attachment pins: %w", result.Err)
		}
		if _, exists := pins[result.Pin.Key.String()]; !exists && len(pins) >= s.limits.MaxReferences {
			return nil, fmt.Errorf("%w: recursive pin count exceeds configured limit", ErrCorruptState)
		}
		pins[result.Pin.Key.String()] = result.Pin.Name
	}
	return pins, nil
}

func (s *Store) ensureRootPinned(ctx context.Context, root cid.Cid) error {
	name, pinned, err := s.recursivePinName(ctx, root)
	if err != nil {
		return err
	}
	if pinned {
		if name != pinName(root) {
			return fmt.Errorf("%w: recursive pin %s is owned by name %q", ErrConflict, root, name)
		}
		return s.pinner.Flush(ctx)
	}
	block, err := s.node.Blockstore.Get(ctx, root)
	if err != nil {
		return err
	}
	var rootNode format.Node
	switch root.Prefix().Codec {
	case cid.Raw:
		rootNode, err = merkledag.DecodeRawBlock(block)
	case cid.DagProtobuf:
		rootNode, err = merkledag.DecodeProtobufBlock(block)
	default:
		err = fmt.Errorf("unsupported root codec %d", root.Prefix().Codec)
	}
	if err != nil {
		return err
	}
	if err := s.pinner.Pin(ctx, rootNode, true, pinName(root)); err != nil {
		return err
	}
	return s.pinner.Flush(ctx)
}

func (s *Store) checkPinOwnership(ctx context.Context, root cid.Cid) error {
	name, pinned, err := s.recursivePinName(ctx, root)
	if err != nil {
		return err
	}
	if pinned && name != pinName(root) {
		return fmt.Errorf("%w: recursive pin %s is owned by name %q", ErrConflict, root, name)
	}
	return nil
}

func (s *Store) recursivePinName(ctx context.Context, root cid.Cid) (string, bool, error) {
	results, err := s.pinner.CheckIfPinnedWithType(ctx, pinning.Recursive, true, root)
	if err != nil {
		return "", false, err
	}
	if len(results) != 1 || results[0].Mode != pinning.Recursive {
		return "", false, nil
	}
	return results[0].Name, true, nil
}

func (s *Store) unpinRoot(ctx context.Context, root cid.Cid) error {
	err := s.pinner.Unpin(ctx, root, true)
	if err != nil && !errors.Is(err, pinning.ErrNotPinned) {
		return err
	}
	return s.pinner.Flush(ctx)
}

func (s *Store) putRecord(ctx context.Context, record persistedRecord) (bool, error) {
	encoded, err := encodeRecord(record)
	if err != nil {
		return false, err
	}
	if err := s.records.Put(ctx, referenceKey(record.ID), encoded); err != nil {
		return false, fmt.Errorf("writing attachment reference %s in state %s: %w", record.ID, record.State, err)
	}
	if err := s.records.Sync(ctx, referencePrefix); err != nil {
		return true, fmt.Errorf("syncing attachment reference %s in state %s: %w", record.ID, record.State, err)
	}
	return true, nil
}

func (s *Store) deleteRecord(ctx context.Context, id ReferenceID) error {
	if err := s.records.Delete(ctx, referenceKey(id)); err != nil {
		return fmt.Errorf("deleting attachment reference %s: %w", id, err)
	}
	if err := s.records.Sync(ctx, referencePrefix); err != nil {
		return fmt.Errorf("syncing deleted attachment reference %s: %w", id, err)
	}
	return nil
}

func (s *Store) addEntryLocked(record persistedRecord) {
	s.entries[record.ID] = record
	key := recordRootKey(record)
	if _, accounted := s.accounted[key]; !accounted {
		if record.ByteLength > math.MaxInt64-s.logicalBytes {
			s.logicalBytes = math.MaxInt64
		} else {
			s.logicalBytes += record.ByteLength
		}
		s.accounted[key] = struct{}{}
	}
	s.roots[key]++
}

func (s *Store) convertReservationLocked(id ReferenceID, record persistedRecord) {
	reservation := s.pending[id]
	delete(s.pending, id)
	s.reservedRefs--
	s.reservedBytes -= reservation.bytes
	s.addEntryLocked(record)
	if reservation.root != nil {
		key := *reservation.root
		s.pendingRoots[key]--
		if s.pendingRoots[key] == 0 {
			delete(s.pendingRoots, key)
		}
	}
}

func (s *Store) removeEntryLocked(record persistedRecord) {
	delete(s.entries, record.ID)
	key := recordRootKey(record)
	s.roots[key]--
	if s.roots[key] == 0 {
		delete(s.roots, key)
		if s.pendingRoots[key] == 0 {
			delete(s.accounted, key)
			s.logicalBytes -= record.ByteLength
		}
	}
}

func referenceKey(id ReferenceID) datastore.Key { return referencePrefix.ChildString(id.String()) }
func pinName(root cid.Cid) string               { return pinNamePrefix + root.String() }
func recordRootKey(record persistedRecord) rootKey {
	return rootKey{cid: record.CID.String(), length: record.ByteLength}
}
func fileRootKey(file network.PublicFile) rootKey {
	return rootKey{cid: file.CID.String(), length: file.ByteLength}
}
func publicReference(record persistedRecord) PublicReference {
	return PublicReference{ID: record.ID, File: network.PublicFile{CID: record.CID, ByteLength: record.ByteLength}}
}

func validatePublicDescriptor(file network.PublicFile) error {
	if file.ByteLength < 0 || file.ByteLength > maxPublicFileSize {
		return fmt.Errorf("%w: byte length %d is outside 0..%d", network.ErrInvalidPublicFile, file.ByteLength, maxPublicFileSize)
	}
	if !file.CID.Defined() {
		return fmt.Errorf("%w: undefined CID", network.ErrInvalidPublicFile)
	}
	prefix := file.CID.Prefix()
	if file.CID.Version() != 1 || prefix.MhType != mh.SHA2_256 || prefix.MhLength != 32 || (prefix.Codec != cid.Raw && prefix.Codec != cid.DagProtobuf) {
		return fmt.Errorf("%w: unsupported CID profile %s", network.ErrInvalidPublicFile, file.CID)
	}
	return nil
}

type localFileVerifier struct {
	ctx     context.Context
	node    *network.Node
	seen    int
	encoded int64
}

func verifyLocalPublicFile(ctx context.Context, node *network.Node, file network.PublicFile) error {
	if err := validatePublicDescriptor(file); err != nil {
		return err
	}
	verifier := &localFileVerifier{ctx: ctx, node: node}
	size, err := verifier.verifyNode(file.CID, 0)
	if err != nil {
		return err
	}
	if size != uint64(file.ByteLength) {
		return fmt.Errorf("%w: decoded length %d does not equal declared length %d", network.ErrInvalidPublicFile, size, file.ByteLength)
	}
	return nil
}

func (v *localFileVerifier) verifyNode(id cid.Cid, depth int) (uint64, error) {
	if err := v.ctx.Err(); err != nil {
		return 0, err
	}
	if depth > maxFileDepth {
		return 0, fmt.Errorf("%w: traversal depth exceeds %d", network.ErrInvalidPublicFile, maxFileDepth)
	}
	v.seen++
	if v.seen > maxFileNodeOccurrences {
		return 0, fmt.Errorf("%w: traversal exceeds %d node occurrences", network.ErrInvalidPublicFile, maxFileNodeOccurrences)
	}
	block, err := v.node.Blockstore.Get(v.ctx, id)
	if err != nil {
		return 0, fmt.Errorf("getting local attachment block %s: %w", id, err)
	}
	data := block.RawData()
	if len(data) > maxEncodedFileBlockBytes || int64(len(data)) > maxFileEncodedBytes-v.encoded {
		return 0, fmt.Errorf("%w: encoded traversal limit exceeded", network.ErrInvalidPublicFile)
	}
	v.encoded += int64(len(data))
	computed, err := id.Prefix().Sum(data)
	if err != nil || !computed.Equals(id) {
		return 0, fmt.Errorf("%w: block bytes do not match CID %s", network.ErrInvalidPublicFile, id)
	}
	if id.Prefix().Codec == cid.Raw {
		return uint64(len(data)), nil
	}
	dagNode, err := merkledag.DecodeProtobuf(data)
	if err != nil {
		return 0, fmt.Errorf("%w: decoding DAG-PB block: %v", network.ErrInvalidPublicFile, err)
	}
	message := new(unixfspb.Data)
	if err := proto.Unmarshal(dagNode.Data(), message); err != nil {
		return 0, fmt.Errorf("%w: decoding UnixFS data: %v", network.ErrInvalidPublicFile, err)
	}
	if message.Type == nil || message.Filesize == nil || (message.GetType() != unixfspb.Data_File && message.GetType() != unixfspb.Data_Raw) {
		return 0, fmt.Errorf("%w: invalid UnixFS file metadata", network.ErrInvalidPublicFile)
	}
	links := dagNode.Links()
	if len(links) > maxFileLinks || len(message.Blocksizes) != len(links) {
		return 0, fmt.Errorf("%w: invalid UnixFS link table", network.ErrInvalidPublicFile)
	}
	total := uint64(len(message.Data))
	for i, link := range links {
		if link == nil || link.Name != "" {
			return 0, fmt.Errorf("%w: invalid file link %d", network.ErrInvalidPublicFile, i)
		}
		if err := validatePublicDescriptor(network.PublicFile{CID: link.Cid, ByteLength: 0}); err != nil {
			return 0, err
		}
		declared := message.Blocksizes[i]
		if declared > uint64(maxPublicFileSize) || total > uint64(maxPublicFileSize)-declared {
			return 0, fmt.Errorf("%w: UnixFS size overflow", network.ErrInvalidPublicFile)
		}
		childSize, err := v.verifyNode(link.Cid, depth+1)
		if err != nil {
			return 0, err
		}
		if childSize != declared {
			return 0, fmt.Errorf("%w: child %d size %d does not equal %d", network.ErrInvalidPublicFile, i, childSize, declared)
		}
		total += childSize
	}
	if message.GetFilesize() != total {
		return 0, fmt.Errorf("%w: UnixFS size %d does not equal %d", network.ErrInvalidPublicFile, message.GetFilesize(), total)
	}
	return total, nil
}

func isNilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
