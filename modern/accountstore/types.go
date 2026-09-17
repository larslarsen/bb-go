package accountstore

import (
	"errors"
	"sync"

	datastore "github.com/ipfs/go-datastore"
	"github.com/larslarsen/bb-go/modern/accountauth"
)

const (
	MaxStoredRecords = accountauth.MaxKnownRecords + 1
	MaxSnapshotBytes = 74 + MaxStoredRecords*(2+accountauth.MaxRecordBytes)
)

var (
	ErrExists      = errors.New("accountstore: account state already exists")
	ErrCorrupt     = errors.New("accountstore: corrupt account state")
	ErrUnavailable = errors.New("accountstore: unavailable")
	ErrSaturated   = errors.New("accountstore: saturated")
)

type factIdentity struct {
	kind   accountauth.Kind
	target [32]byte
}

// Store owns one serialized in-memory view of a persisted account state.
// Store values must not be copied after first use.
type Store struct {
	mu sync.Mutex

	backend    datastore.Datastore
	key        datastore.Key
	controller [32]byte
	state      *accountauth.KnownState
	records    [][]byte
	facts      map[factIdentity]struct{}
	available  bool
}
