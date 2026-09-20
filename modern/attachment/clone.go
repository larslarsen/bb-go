package attachment

import (
	"context"
	"fmt"

	"github.com/larslarsen/bb-go/modern/network"
)

// ClonePublic creates an independent retention claim for an already-ready local
// public file. The source lookup and target transition share the store lock with
// Release, so a source release cannot race between validation and retention.
func (s *Store) ClonePublic(ctx context.Context, sourceID, targetID ReferenceID, expected network.PublicFile) (PublicReference, error) {
	opctx, done, err := s.begin(ctx)
	if err != nil {
		return PublicReference{}, err
	}
	defer done()
	if err := validateReferenceID(sourceID); err != nil {
		return PublicReference{}, err
	}
	if err := validateReferenceID(targetID); err != nil {
		return PublicReference{}, err
	}
	if sourceID == targetID {
		return PublicReference{}, fmt.Errorf("%w: clone source and target must differ", ErrInvalidReferenceID)
	}
	if err := validatePublicDescriptor(expected); err != nil {
		return PublicReference{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if target, ok := s.entries[targetID]; ok {
		if target.State != stateReady {
			return PublicReference{}, privateCloneError("clone target is not ready", ErrNotReady)
		}
		if recordRootKey(target) != fileRootKey(expected) {
			return PublicReference{}, privateCloneError("clone target names another file", ErrConflict)
		}
		return publicReference(target), nil
	}
	if _, pending := s.pending[targetID]; pending {
		return PublicReference{}, privateCloneError("clone target has an active operation", ErrNotReady)
	}
	source, ok := s.entries[sourceID]
	if !ok {
		return PublicReference{}, privateCloneError("clone source was not found", ErrNotFound)
	}
	if source.State != stateReady {
		return PublicReference{}, privateCloneError("clone source is not ready", ErrNotReady)
	}
	if recordRootKey(source) != fileRootKey(expected) {
		return PublicReference{}, privateCloneError("clone source does not match expected file", ErrConflict)
	}
	if len(s.entries)+s.reservedRefs >= s.limits.MaxReferences {
		return PublicReference{}, ErrQuotaExceeded
	}
	if err := verifyLocalPublicFile(opctx, s.node, expected); err != nil {
		return PublicReference{}, err
	}
	if err := s.checkPinOwnership(opctx, expected.CID); err != nil {
		return PublicReference{}, err
	}

	record := persistedRecord{Version: recordVersion, State: stateRetaining, ID: targetID, CID: expected.CID, ByteLength: expected.ByteLength}
	written, err := s.putRecord(opctx, record)
	if written {
		s.addEntryLocked(record)
	}
	if err != nil {
		return PublicReference{}, privateCloneError("writing cloned attachment claim", err)
	}
	if err := s.ensureRootPinned(opctx, expected.CID); err != nil {
		return PublicReference{}, fmt.Errorf("pinning cloned public file %s: %w", expected.CID, err)
	}
	record.State = stateReady
	if _, err := s.putRecord(opctx, record); err != nil {
		return PublicReference{}, privateCloneError("finalizing cloned attachment claim", err)
	}
	s.entries[targetID] = record
	return publicReference(record), nil
}

type clonePrivateError struct {
	message string
	cause   error
}

func (err clonePrivateError) Error() string { return err.message }
func (err clonePrivateError) Unwrap() error { return err.cause }

func privateCloneError(message string, cause error) error {
	return clonePrivateError{message: message, cause: cause}
}

// IsForNode reports whether this open store is associated with the exact node.
func (s *Store) IsForNode(node *network.Node) bool {
	if s == nil || node == nil {
		return false
	}
	s.lifeMu.Lock()
	defer s.lifeMu.Unlock()
	return !s.closed && s.node == node
}
