package accountauth

import "errors"

var (
	errInvalidState   = errors.New("accountauth: invalid state")
	errForeignAccount = errors.New("accountauth: foreign account")
	errStateSaturated = errors.New("accountauth: state saturated")
)

type grantFact struct {
	device       [32]byte
	capabilities Capability
}

// KnownState retains verified grants and revocation tombstones for one controller.
type KnownState struct {
	controller     [32]byte
	grants         map[GrantID]grantFact
	revokedGrants  map[GrantID]struct{}
	revokedDevices map[[32]byte]struct{}
	recordCount    int
	saturated      bool
	initialized    bool
}

// NewKnownState creates an empty verifier pinned to controller.
func NewKnownState(controller [32]byte) (*KnownState, error) {
	if isZero32(controller) {
		return nil, errInvalidController
	}
	return &KnownState{
		controller:     controller,
		grants:         make(map[GrantID]grantFact),
		revokedGrants:  make(map[GrantID]struct{}),
		revokedDevices: make(map[[32]byte]struct{}),
		initialized:    true,
	}, nil
}

// Apply verifies and retains one record for this state's controller.
func (s *KnownState) Apply(raw []byte) error {
	if s == nil || !s.initialized {
		return errInvalidState
	}

	record, err := VerifyRecord(raw)
	if err != nil {
		return err
	}
	if record.controller != s.controller {
		return errForeignAccount
	}

	if s.contains(record) {
		return nil
	}
	if s.saturated {
		return errStateSaturated
	}
	if s.recordCount >= MaxKnownRecords {
		s.saturated = true
		return errStateSaturated
	}

	s.insert(record)
	s.recordCount++
	return nil
}

// AuthorizesKnown reports whether one retained, unrevoked grant contains required.
func (s *KnownState) AuthorizesKnown(grant GrantID, device [32]byte, required Capability) bool {
	if s == nil || !s.initialized || s.saturated || required == 0 || required&^allCapabilities != 0 {
		return false
	}
	fact, ok := s.grants[grant]
	if !ok || fact.device != device || fact.capabilities&required != required {
		return false
	}
	if _, revoked := s.revokedGrants[grant]; revoked {
		return false
	}
	if _, revoked := s.revokedDevices[device]; revoked {
		return false
	}
	return true
}

// Saturated reports whether a distinct record exceeded the retained-record bound.
func (s *KnownState) Saturated() bool {
	return s != nil && s.initialized && s.saturated
}

func (s *KnownState) contains(record Record) bool {
	switch record.kind {
	case Grant:
		_, ok := s.grants[record.grantID]
		return ok
	case RevokeGrant:
		_, ok := s.revokedGrants[record.grantID]
		return ok
	case RevokeDevice:
		_, ok := s.revokedDevices[record.device]
		return ok
	default:
		return false
	}
}

func (s *KnownState) insert(record Record) {
	switch record.kind {
	case Grant:
		s.grants[record.grantID] = grantFact{
			device:       record.device,
			capabilities: record.capabilities,
		}
	case RevokeGrant:
		s.revokedGrants[record.grantID] = struct{}{}
	case RevokeDevice:
		s.revokedDevices[record.device] = struct{}{}
	}
}
