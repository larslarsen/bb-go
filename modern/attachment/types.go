// Package attachment provides durable retention claims for public BitBook files.
package attachment

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/larslarsen/bb-go/modern/network"
)

const (
	maxReferences   = 65536
	maxLogicalBytes = int64(1 << 40)
)

var (
	ErrInvalidReferenceID = errors.New("invalid attachment reference ID")
	ErrNotFound           = errors.New("attachment reference not found")
	ErrNotReady           = errors.New("attachment reference not ready")
	ErrConflict           = errors.New("attachment reference conflict")
	ErrQuotaExceeded      = errors.New("attachment retention quota exceeded")
	ErrCorruptState       = errors.New("corrupt attachment retention state")
	ErrClosed             = errors.New("attachment store closed")

	randomReader io.Reader = rand.Reader
)

// ReferenceID is an opaque local identifier for one attachment retention claim.
type ReferenceID [16]byte

// NewReferenceID returns a cryptographically random nonzero reference ID.
func NewReferenceID() (ReferenceID, error) {
	var id ReferenceID
	if _, err := io.ReadFull(randomReader, id[:]); err != nil {
		return ReferenceID{}, fmt.Errorf("generating attachment reference ID: %w", err)
	}
	if id == (ReferenceID{}) {
		return ReferenceID{}, fmt.Errorf("%w: random source returned zero", ErrInvalidReferenceID)
	}
	return id, nil
}

// ParseReferenceID parses the canonical 32-character lowercase hexadecimal form.
func ParseReferenceID(value string) (ReferenceID, error) {
	if len(value) != 32 {
		return ReferenceID{}, fmt.Errorf("%w: expected 32 lowercase hexadecimal characters", ErrInvalidReferenceID)
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return ReferenceID{}, fmt.Errorf("%w: non-canonical character", ErrInvalidReferenceID)
		}
	}
	var id ReferenceID
	if _, err := hex.Decode(id[:], []byte(value)); err != nil {
		return ReferenceID{}, fmt.Errorf("%w: %v", ErrInvalidReferenceID, err)
	}
	if id == (ReferenceID{}) {
		return ReferenceID{}, fmt.Errorf("%w: zero ID", ErrInvalidReferenceID)
	}
	return id, nil
}

// String returns the canonical lowercase hexadecimal representation.
func (id ReferenceID) String() string { return hex.EncodeToString(id[:]) }

// Limits bounds ready references and distinct-root logical bytes.
type Limits struct {
	MaxReferences   int
	MaxLogicalBytes int64
}

// PublicReference is one durable retention claim for a public file.
type PublicReference struct {
	ID   ReferenceID
	File network.PublicFile
}

func validateReferenceID(id ReferenceID) error {
	if id == (ReferenceID{}) {
		return fmt.Errorf("%w: zero ID", ErrInvalidReferenceID)
	}
	return nil
}

func validateLimits(limits Limits) error {
	if limits.MaxReferences < 1 || limits.MaxReferences > maxReferences {
		return fmt.Errorf("MaxReferences must be between 1 and %d", maxReferences)
	}
	if limits.MaxLogicalBytes < 1 || limits.MaxLogicalBytes > maxLogicalBytes {
		return fmt.Errorf("MaxLogicalBytes must be between 1 and %d", maxLogicalBytes)
	}
	return nil
}
