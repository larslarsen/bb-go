package attachment

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

const (
	recordVersion     = 1
	maxRecordBytes    = 512
	maxPublicFileSize = int64(100 << 20)

	stateRetaining = "retaining"
	stateReady     = "ready"
	stateReleasing = "releasing"
)

type persistedRecord struct {
	Version    int
	State      string
	ID         ReferenceID
	CID        cid.Cid
	ByteLength int64
}

type wireRecord struct {
	Version    int    `json:"version"`
	State      string `json:"state"`
	ID         string `json:"id"`
	CID        string `json:"cid"`
	ByteLength int64  `json:"byteLength"`
}

func encodeRecord(record persistedRecord) ([]byte, error) {
	if err := validatePersistedRecord(record); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(wireRecord{
		Version:    record.Version,
		State:      record.State,
		ID:         record.ID.String(),
		CID:        record.CID.String(),
		ByteLength: record.ByteLength,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: encoding record: %v", ErrCorruptState, err)
	}
	if len(encoded) > maxRecordBytes {
		return nil, fmt.Errorf("%w: encoded record exceeds %d bytes", ErrCorruptState, maxRecordBytes)
	}
	return encoded, nil
}

func decodeRecord(encoded []byte) (persistedRecord, error) {
	if len(encoded) == 0 || len(encoded) > maxRecordBytes {
		return persistedRecord{}, fmt.Errorf("%w: record size %d is outside 1..%d", ErrCorruptState, len(encoded), maxRecordBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return persistedRecord{}, fmt.Errorf("%w: record is not a JSON object", ErrCorruptState)
	}
	var wire wireRecord
	seen := make(map[string]struct{}, 5)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return persistedRecord{}, fmt.Errorf("%w: reading field: %v", ErrCorruptState, err)
		}
		name, ok := token.(string)
		if !ok {
			return persistedRecord{}, fmt.Errorf("%w: non-string field name", ErrCorruptState)
		}
		if _, duplicate := seen[name]; duplicate {
			return persistedRecord{}, fmt.Errorf("%w: duplicate field %q", ErrCorruptState, name)
		}
		seen[name] = struct{}{}
		switch name {
		case "version":
			err = decodeRequiredScalar(decoder, &wire.Version)
		case "state":
			err = decodeRequiredScalar(decoder, &wire.State)
		case "id":
			err = decodeRequiredScalar(decoder, &wire.ID)
		case "cid":
			err = decodeRequiredScalar(decoder, &wire.CID)
		case "byteLength":
			err = decodeRequiredScalar(decoder, &wire.ByteLength)
		default:
			return persistedRecord{}, fmt.Errorf("%w: unknown field %q", ErrCorruptState, name)
		}
		if err != nil {
			return persistedRecord{}, fmt.Errorf("%w: decoding field %q: %v", ErrCorruptState, name, err)
		}
	}
	if len(seen) != 5 {
		return persistedRecord{}, fmt.Errorf("%w: record has %d fields, want 5", ErrCorruptState, len(seen))
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return persistedRecord{}, fmt.Errorf("%w: unterminated record", ErrCorruptState)
	}
	if token, err := decoder.Token(); err != io.EOF {
		return persistedRecord{}, fmt.Errorf("%w: trailing data %v", ErrCorruptState, token)
	}
	id, err := ParseReferenceID(wire.ID)
	if err != nil {
		return persistedRecord{}, fmt.Errorf("%w: record ID: %v", ErrCorruptState, err)
	}
	root, err := cid.Decode(wire.CID)
	if err != nil || root.String() != wire.CID {
		return persistedRecord{}, fmt.Errorf("%w: invalid or non-canonical CID", ErrCorruptState)
	}
	record := persistedRecord{Version: wire.Version, State: wire.State, ID: id, CID: root, ByteLength: wire.ByteLength}
	if err := validatePersistedRecord(record); err != nil {
		return persistedRecord{}, err
	}
	return record, nil
}

func decodeRequiredScalar(decoder *json.Decoder, target any) error {
	var encoded json.RawMessage
	if err := decoder.Decode(&encoded); err != nil {
		return err
	}
	if bytes.Equal(bytes.TrimSpace(encoded), []byte("null")) {
		return errors.New("required scalar is null")
	}
	return json.Unmarshal(encoded, target)
}

func validatePersistedRecord(record persistedRecord) error {
	if record.Version != recordVersion {
		return fmt.Errorf("%w: unsupported record version %d", ErrCorruptState, record.Version)
	}
	if record.State != stateRetaining && record.State != stateReady && record.State != stateReleasing {
		return fmt.Errorf("%w: invalid record state %q", ErrCorruptState, record.State)
	}
	if err := validateReferenceID(record.ID); err != nil {
		return fmt.Errorf("%w: %v", ErrCorruptState, err)
	}
	if err := validateStoredFile(record.CID, record.ByteLength); err != nil {
		return err
	}
	return nil
}

func validateStoredFile(root cid.Cid, length int64) error {
	if length < 0 || length > maxPublicFileSize {
		return fmt.Errorf("%w: byte length %d is outside 0..%d", ErrCorruptState, length, maxPublicFileSize)
	}
	if !root.Defined() {
		return fmt.Errorf("%w: undefined CID", ErrCorruptState)
	}
	prefix := root.Prefix()
	if root.Version() != 1 || prefix.MhType != mh.SHA2_256 || prefix.MhLength != 32 || (prefix.Codec != cid.Raw && prefix.Codec != cid.DagProtobuf) {
		return fmt.Errorf("%w: unsupported CID profile %s", ErrCorruptState, root)
	}
	return nil
}
