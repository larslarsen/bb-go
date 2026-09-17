package accountstore

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	datastore "github.com/ipfs/go-datastore"
	"github.com/larslarsen/bb-go/modern/accountauth"
)

const storeKeyPrefix = "/bitbook/accountauth/v1/"

// Create initializes and durably persists an empty account state.
func Create(ctx context.Context, backend datastore.Datastore, controller [32]byte) (*Store, error) {
	if backend == nil {
		return nil, fmt.Errorf("%w: nil backend", ErrUnavailable)
	}

	key, err := storeKey(controller)
	if err != nil {
		return nil, fmt.Errorf("accountstore: invalid controller: %w", err)
	}
	state, err := accountauth.NewKnownState(controller)
	if err != nil {
		return nil, fmt.Errorf("accountstore: initialize state: %w", err)
	}

	_, err = backend.Get(ctx, key)
	switch {
	case err == nil:
		return nil, ErrExists
	case !errors.Is(err, datastore.ErrNotFound):
		return nil, fmt.Errorf("accountstore: check existing state: %w", err)
	}

	snapshot, err := encodeSnapshot(controller, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: encode empty state: %v", ErrUnavailable, err)
	}
	if err := backend.Put(ctx, key, snapshot); err != nil {
		return nil, storageError("put empty state", err)
	}
	if err := backend.Sync(ctx, key); err != nil {
		return nil, storageError("sync empty state", err)
	}

	return &Store{
		backend:    backend,
		key:        key,
		controller: controller,
		state:      state,
		facts:      make(map[factIdentity]struct{}),
		available:  true,
	}, nil
}

// Open loads and fully re-verifies an existing account state without changing it.
func Open(ctx context.Context, backend datastore.Datastore, controller [32]byte) (*Store, error) {
	if backend == nil {
		return nil, fmt.Errorf("%w: nil backend", ErrUnavailable)
	}

	key, err := storeKey(controller)
	if err != nil {
		return nil, fmt.Errorf("accountstore: invalid controller: %w", err)
	}
	raw, err := backend.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("accountstore: read state: %w", err)
	}

	decoded, err := decodeSnapshot(raw, controller)
	if err != nil {
		return nil, err
	}
	return &Store{
		backend:    backend,
		key:        key,
		controller: controller,
		state:      decoded.state,
		records:    decoded.records,
		facts:      decoded.facts,
		available:  true,
	}, nil
}

// Apply verifies and durably appends one signed authorization fact.
func (s *Store) Apply(ctx context.Context, raw []byte) error {
	if s == nil {
		return ErrUnavailable
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.usableLocked() {
		return ErrUnavailable
	}

	record, err := verifyForController(raw, s.controller)
	if err != nil {
		return err
	}
	identity, err := identityFor(record)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.state.Saturated() {
		return ErrSaturated
	}
	if _, duplicate := s.facts[identity]; duplicate {
		return nil
	}
	if len(s.records) > accountauth.MaxKnownRecords {
		s.disableLocked()
		return fmt.Errorf("%w: invalid in-memory record count", ErrUnavailable)
	}

	owned := record.Bytes()
	nextRecords := make([][]byte, len(s.records)+1)
	copy(nextRecords, s.records)
	nextRecords[len(s.records)] = owned
	snapshot, err := encodeSnapshot(s.controller, nextRecords)
	if err != nil {
		s.disableLocked()
		return fmt.Errorf("%w: encode next state: %v", ErrUnavailable, err)
	}
	if err := s.backend.Put(ctx, s.key, snapshot); err != nil {
		s.disableLocked()
		return storageError("put state", err)
	}
	if err := s.backend.Sync(ctx, s.key); err != nil {
		s.disableLocked()
		return storageError("sync state", err)
	}

	atCapacity := len(s.records) == accountauth.MaxKnownRecords
	applyErr := s.state.Apply(owned)
	if atCapacity {
		if applyErr == nil || !s.state.Saturated() {
			s.disableLocked()
			return fmt.Errorf("%w: overflow witness invariant", ErrUnavailable)
		}
		s.records = nextRecords
		s.facts[identity] = struct{}{}
		return ErrSaturated
	}
	if applyErr != nil || s.state.Saturated() {
		s.disableLocked()
		return fmt.Errorf("%w: committed-state invariant", ErrUnavailable)
	}

	s.records = nextRecords
	s.facts[identity] = struct{}{}
	return nil
}

// AuthorizesKnown reports authority from the committed in-memory state.
func (s *Store) AuthorizesKnown(grant accountauth.GrantID, device [32]byte, required accountauth.Capability) (bool, error) {
	if s == nil {
		return false, ErrUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.usableLocked() {
		return false, ErrUnavailable
	}
	if s.state.Saturated() {
		return false, nil
	}
	return s.state.AuthorizesKnown(grant, device, required), nil
}

// Saturated reports whether a durable overflow witness has denied this account.
func (s *Store) Saturated() (bool, error) {
	if s == nil {
		return false, ErrUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.usableLocked() {
		return false, ErrUnavailable
	}
	return s.state.Saturated(), nil
}

// Records returns owned copies of the committed records in arrival order.
func (s *Store) Records() ([][]byte, error) {
	if s == nil {
		return nil, ErrUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.usableLocked() {
		return nil, ErrUnavailable
	}

	records := make([][]byte, len(s.records))
	for i, record := range s.records {
		records[i] = append([]byte(nil), record...)
	}
	return records, nil
}

// Close disables this handle without closing or syncing its caller-owned backend.
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.available {
		s.disableLocked()
	}
	return nil
}

func storeKey(controller [32]byte) (datastore.Key, error) {
	accountID, err := accountauth.AccountIDFor(controller)
	if err != nil {
		return datastore.Key{}, err
	}
	return datastore.NewKey(storeKeyPrefix + hex.EncodeToString(accountID[:])), nil
}

func verifyForController(raw []byte, controller [32]byte) (accountauth.Record, error) {
	record, err := accountauth.VerifyRecord(raw)
	if err != nil {
		return accountauth.Record{}, fmt.Errorf("accountstore: verify record: %w", err)
	}
	if len(raw) < 34 || !bytes.Equal(raw[2:34], controller[:]) {
		return accountauth.Record{}, errors.New("accountstore: record belongs to another controller")
	}
	return record, nil
}

func identityFor(record accountauth.Record) (factIdentity, error) {
	switch record.Kind() {
	case accountauth.Grant, accountauth.RevokeGrant:
		return factIdentity{kind: record.Kind(), target: [32]byte(record.GrantID())}, nil
	case accountauth.RevokeDevice:
		return factIdentity{kind: record.Kind(), target: record.DeviceKey()}, nil
	default:
		return factIdentity{}, errors.New("accountstore: unsupported verified record kind")
	}
}

func storageError(operation string, err error) error {
	return fmt.Errorf("%w: %s: %w", ErrUnavailable, operation, err)
}

func (s *Store) usableLocked() bool {
	return s.available && s.backend != nil && s.state != nil && s.facts != nil
}

func (s *Store) disableLocked() {
	s.available = false
	s.backend = nil
	s.state = nil
	s.records = nil
	s.facts = nil
}
