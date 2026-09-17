package accountstore

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/larslarsen/bb-go/modern/accountauth"
)

const (
	snapshotMagic           = "BBACST01"
	snapshotHeaderBytes     = 42
	snapshotChecksumBytes   = sha256.Size
	minimumSnapshotBytes    = snapshotHeaderBytes + snapshotChecksumBytes
	revocationRecordBytes   = 130
	recordLengthPrefixBytes = 2
)

type decodedSnapshot struct {
	state   *accountauth.KnownState
	records [][]byte
	facts   map[factIdentity]struct{}
}

func encodeSnapshot(controller [32]byte, records [][]byte) ([]byte, error) {
	if len(records) > MaxStoredRecords {
		return nil, fmt.Errorf("record count %d exceeds limit", len(records))
	}

	size := minimumSnapshotBytes
	for _, record := range records {
		if len(record) != revocationRecordBytes && len(record) != accountauth.MaxRecordBytes {
			return nil, fmt.Errorf("unsupported record length %d", len(record))
		}
		size += recordLengthPrefixBytes + len(record)
	}
	if size > MaxSnapshotBytes {
		return nil, fmt.Errorf("snapshot size %d exceeds limit", size)
	}

	raw := make([]byte, snapshotHeaderBytes, size)
	copy(raw[:8], snapshotMagic)
	copy(raw[8:40], controller[:])
	binary.BigEndian.PutUint16(raw[40:42], uint16(len(records)))
	for _, record := range records {
		var length [recordLengthPrefixBytes]byte
		binary.BigEndian.PutUint16(length[:], uint16(len(record)))
		raw = append(raw, length[:]...)
		raw = append(raw, record...)
	}
	digest := sha256.Sum256(raw)
	raw = append(raw, digest[:]...)
	return raw, nil
}

func decodeSnapshot(raw []byte, controller [32]byte) (*decodedSnapshot, error) {
	if len(raw) < minimumSnapshotBytes || len(raw) > MaxSnapshotBytes {
		return nil, corruptError("snapshot size is outside bounds")
	}
	if !bytes.Equal(raw[:8], []byte(snapshotMagic)) {
		return nil, corruptError("unsupported snapshot magic")
	}
	if !bytes.Equal(raw[8:40], controller[:]) {
		return nil, corruptError("snapshot controller mismatch")
	}

	payloadEnd := len(raw) - snapshotChecksumBytes
	digest := sha256.Sum256(raw[:payloadEnd])
	if !bytes.Equal(raw[payloadEnd:], digest[:]) {
		return nil, corruptError("snapshot checksum mismatch")
	}

	count := int(binary.BigEndian.Uint16(raw[40:42]))
	if count > MaxStoredRecords {
		return nil, corruptError("snapshot record count exceeds limit")
	}
	bodyBytes := payloadEnd - snapshotHeaderBytes
	minimumBodyBytes := count * (recordLengthPrefixBytes + revocationRecordBytes)
	maximumBodyBytes := count * (recordLengthPrefixBytes + accountauth.MaxRecordBytes)
	if bodyBytes < minimumBodyBytes || bodyBytes > maximumBodyBytes {
		return nil, corruptError("snapshot body does not match record count")
	}

	state, err := accountauth.NewKnownState(controller)
	if err != nil {
		return nil, corruptError("invalid snapshot controller")
	}
	records := make([][]byte, 0, count)
	facts := make(map[factIdentity]struct{}, count)
	offset := snapshotHeaderBytes
	for i := 0; i < count; i++ {
		if payloadEnd-offset < recordLengthPrefixBytes {
			return nil, corruptError("truncated record length")
		}
		length := int(binary.BigEndian.Uint16(raw[offset : offset+recordLengthPrefixBytes]))
		offset += recordLengthPrefixBytes
		if length != revocationRecordBytes && length != accountauth.MaxRecordBytes {
			return nil, corruptError("unsupported record length")
		}
		if length > payloadEnd-offset {
			return nil, corruptError("truncated record")
		}

		recordBytes := append([]byte(nil), raw[offset:offset+length]...)
		offset += length
		record, err := verifyForController(recordBytes, controller)
		if err != nil {
			return nil, corruptError("record verification failed")
		}
		identity, err := identityFor(record)
		if err != nil {
			return nil, corruptError("record identity is invalid")
		}
		if _, duplicate := facts[identity]; duplicate {
			return nil, corruptError("duplicate semantic fact")
		}

		applyErr := state.Apply(recordBytes)
		if i < accountauth.MaxKnownRecords {
			if applyErr != nil || state.Saturated() {
				return nil, corruptError("record replay failed")
			}
		} else if applyErr == nil || !state.Saturated() {
			return nil, corruptError("overflow witness did not establish saturation")
		}
		facts[identity] = struct{}{}
		records = append(records, recordBytes)
	}
	if offset != payloadEnd {
		return nil, corruptError("snapshot has trailing bytes")
	}
	if count <= accountauth.MaxKnownRecords && state.Saturated() {
		return nil, corruptError("snapshot saturated below the record limit")
	}
	if count == MaxStoredRecords && !state.Saturated() {
		return nil, corruptError("snapshot lacks durable saturation")
	}

	return &decodedSnapshot{state: state, records: records, facts: facts}, nil
}

func corruptError(reason string) error {
	return fmt.Errorf("%w: %s", ErrCorrupt, reason)
}
