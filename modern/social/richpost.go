package social

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	blocks "github.com/ipfs/go-block-format"
	"github.com/larslarsen/bb-go/modern/publiccontent"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	richPostSchema        = "bitbook.public-post/1"
	maxRichPayloadBytes   = 70 << 10
	maxRichSignatureBytes = 1 << 10
	maxRichPublicKeyBytes = 4 << 10
	maxRichEnvelopeBytes  = 80 << 10
	maxRichPayloadDepth   = 10
)

var (
	ErrInvalidRichPost          = errors.New("invalid rich post")
	ErrRichPostConflict         = errors.New("rich post conflict")
	ErrRichPostDeleted          = errors.New("rich post deleted")
	ErrRichPostQuota            = errors.New("rich post quota exceeded")
	ErrRichPostUnavailable      = errors.New("rich post store unavailable")
	ErrRichPostRecoveryRequired = errors.New("rich post recovery required")
	ErrCorruptRichPostState     = errors.New("corrupt rich post state")
)

// RichPostLimits bounds all durable rich-post journal records.
type RichPostLimits struct {
	MaxRecords int
	MaxBytes   int64
}

type parsedRichPayload struct {
	Schema           string
	ID               string
	Author           peer.ID
	Timestamp        time.Time
	TimestampText    string
	Slug             string
	PostType         string
	Status           string
	Content          publiccontent.Content
	CanonicalContent json.RawMessage
}

type richPayloadWire struct {
	Schema    string          `json:"schema"`
	ID        string          `json:"id"`
	VendorID  richVendorWire  `json:"vendorID"`
	Timestamp string          `json:"timestamp"`
	Slug      string          `json:"slug"`
	PostType  string          `json:"postType"`
	Status    string          `json:"status"`
	Content   json.RawMessage `json:"content"`
}

type richVendorWire struct {
	PeerID string `json:"peerID"`
}

type rawRichPayload struct {
	Schema    json.RawMessage `json:"schema"`
	ID        json.RawMessage `json:"id"`
	VendorID  json.RawMessage `json:"vendorID"`
	Timestamp json.RawMessage `json:"timestamp"`
	Slug      json.RawMessage `json:"slug"`
	PostType  json.RawMessage `json:"postType"`
	Status    json.RawMessage `json:"status"`
	Content   json.RawMessage `json:"content"`
}

type rawRichVendor struct {
	PeerID json.RawMessage `json:"peerID"`
}

func parseRichPayload(expectedAuthor peer.ID, raw []byte) (parsedRichPayload, error) {
	if len(raw) == 0 || len(raw) > maxRichPayloadBytes || !utf8.Valid(raw) || !validRichJSONStringEscapes(raw) {
		return parsedRichPayload{}, ErrInvalidRichPost
	}
	if err := preflightRichJSON(raw, maxRichPayloadDepth); err != nil {
		return parsedRichPayload{}, ErrInvalidRichPost
	}
	var wire rawRichPayload
	if err := decodeExactJSON(raw, &wire); err != nil {
		return parsedRichPayload{}, ErrInvalidRichPost
	}
	var schema, id, timestamp, slug, postType, status string
	for _, field := range []struct {
		raw    json.RawMessage
		target *string
	}{
		{wire.Schema, &schema}, {wire.ID, &id}, {wire.Timestamp, &timestamp},
		{wire.Slug, &slug}, {wire.PostType, &postType}, {wire.Status, &status},
	} {
		if err := decodeRequiredJSON(field.raw, field.target); err != nil {
			return parsedRichPayload{}, ErrInvalidRichPost
		}
	}
	if schema != richPostSchema || !validRichID(id) || slug != "rich-"+id || postType != "POST" {
		return parsedRichPayload{}, ErrInvalidRichPost
	}
	var vendor rawRichVendor
	if len(wire.VendorID) == 0 || bytes.Equal(wire.VendorID, []byte("null")) || decodeExactJSON(wire.VendorID, &vendor) != nil {
		return parsedRichPayload{}, ErrInvalidRichPost
	}
	var encodedAuthor string
	if decodeRequiredJSON(vendor.PeerID, &encodedAuthor) != nil {
		return parsedRichPayload{}, ErrInvalidRichPost
	}
	author, err := peer.Decode(encodedAuthor)
	if err != nil || author.String() != encodedAuthor || author != expectedAuthor {
		return parsedRichPayload{}, ErrInvalidRichPost
	}
	parsedTime, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil || !strings.HasSuffix(timestamp, "Z") || parsedTime.Before(time.Unix(0, 0)) || parsedTime.UTC().Format(time.RFC3339Nano) != timestamp {
		return parsedRichPayload{}, ErrInvalidRichPost
	}
	if len(wire.Content) == 0 || bytes.Equal(wire.Content, []byte("null")) {
		return parsedRichPayload{}, ErrInvalidRichPost
	}
	content, err := publiccontent.Parse(wire.Content)
	if err != nil {
		return parsedRichPayload{}, errors.Join(ErrInvalidRichPost, err)
	}
	canonicalContent, err := publiccontent.Marshal(content)
	if err != nil || !bytes.Equal(canonicalContent, wire.Content) {
		return parsedRichPayload{}, ErrInvalidRichPost
	}
	projection, err := publiccontent.PlainText(content)
	if err != nil || projection != status {
		return parsedRichPayload{}, ErrInvalidRichPost
	}
	parsed := parsedRichPayload{
		Schema: schema, ID: id, Author: author, Timestamp: parsedTime, TimestampText: timestamp,
		Slug: slug, PostType: postType, Status: status, Content: content,
		CanonicalContent: slices.Clone(canonicalContent),
	}
	canonical, err := encodeRichPayload(parsed)
	if err != nil || !bytes.Equal(canonical, raw) {
		return parsedRichPayload{}, ErrInvalidRichPost
	}
	return parsed, nil
}

func encodeRichPayload(payload parsedRichPayload) ([]byte, error) {
	encoded, err := json.Marshal(richPayloadWire{
		Schema: payload.Schema, ID: payload.ID, VendorID: richVendorWire{PeerID: payload.Author.String()},
		Timestamp: payload.TimestampText, Slug: payload.Slug, PostType: payload.PostType,
		Status: payload.Status, Content: payload.CanonicalContent,
	})
	if err != nil || len(encoded) > maxRichPayloadBytes {
		return nil, ErrInvalidRichPost
	}
	return encoded, nil
}

func immutablePostBytes(post SignedPost) ([]byte, error) {
	post = cloneSignedPost(post)
	post.CID = ""
	encoded, err := json.Marshal(post)
	if err != nil {
		return nil, err
	}
	if len(encoded) > maxRichEnvelopeBytes {
		return nil, ErrInvalidRichPost
	}
	return encoded, nil
}

func validateRichEnvelope(author peer.ID, post SignedPost) (parsedRichPayload, error) {
	if len(post.Post) > maxRichPayloadBytes || len(post.Signature) == 0 || len(post.Signature) > maxRichSignatureBytes ||
		len(post.PublicKey) == 0 || len(post.PublicKey) > maxRichPublicKeyBytes {
		return parsedRichPayload{}, ErrInvalidRichPost
	}
	payload, err := parseRichPayload(author, post.Post)
	if err != nil {
		return parsedRichPayload{}, err
	}
	immutable, err := immutablePostBytes(post)
	if err != nil {
		return parsedRichPayload{}, errors.Join(ErrInvalidRichPost, err)
	}
	if post.CID != "" {
		computed := blocks.NewBlock(immutable).Cid().String()
		if computed != post.CID {
			return parsedRichPayload{}, ErrInvalidRichPost
		}
	}
	return payload, nil
}

func classifyRichCandidate(raw []byte) (bool, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	opening, err := decoder.Token()
	if err != nil {
		return false, err
	}
	if opening != json.Delim('{') {
		if delimiter, container := opening.(json.Delim); container {
			if err := skipRichJSONContainerBounded(decoder, delimiter, 64); err != nil {
				return false, err
			}
		}
		if token, err := decoder.Token(); err != io.EOF || token != nil {
			return false, errors.New("trailing JSON")
		}
		return false, nil
	}
	rich := false
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return false, err
		}
		name, ok := token.(string)
		if !ok {
			return false, errors.New("invalid top-level object key")
		}
		if name == "schema" || name == "content" {
			rich = true
		}
		if err := skipRichJSONValueBounded(decoder, 64); err != nil {
			return false, err
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return false, errors.New("unterminated top-level object")
	}
	if token, err := decoder.Token(); err != io.EOF || token != nil {
		return false, errors.New("trailing JSON")
	}
	return rich, nil
}

func validRichID(value string) bool {
	if len(value) != 32 || value == strings.Repeat("0", 32) {
		return false
	}
	for index := range value {
		if (value[index] < '0' || value[index] > '9') && (value[index] < 'a' || value[index] > 'f') {
			return false
		}
	}
	return true
}

func decodeExactJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if token, err := decoder.Token(); err != io.EOF || token != nil {
		return errors.New("trailing JSON")
	}
	return nil
}

func decodeRequiredJSON(raw json.RawMessage, target any) error {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.New("missing or null required field")
	}
	return decodeExactJSON(raw, target)
}

func preflightRichJSON(raw []byte, maxDepth int) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := scanRichJSONValue(decoder, 0, maxDepth); err != nil {
		return err
	}
	if token, err := decoder.Token(); err != io.EOF || token != nil {
		return errors.New("trailing JSON")
	}
	return nil
}

func scanRichJSONValue(decoder *json.Decoder, depth, maxDepth int) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, container := token.(json.Delim)
	if !container {
		return nil
	}
	if depth+1 > maxDepth {
		return errors.New("JSON depth exceeded")
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := token.(string)
			if !ok {
				return errors.New("invalid object key")
			}
			if _, duplicate := seen[name]; duplicate {
				return errors.New("duplicate object key")
			}
			seen[name] = struct{}{}
			if err := scanRichJSONValue(decoder, depth+1, maxDepth); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return errors.New("unterminated object")
		}
	case '[':
		for decoder.More() {
			if err := scanRichJSONValue(decoder, depth+1, maxDepth); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return errors.New("unterminated array")
		}
	default:
		return errors.New("unexpected delimiter")
	}
	return nil
}

func skipRichJSONValueBounded(decoder *json.Decoder, maxDepth int) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, container := token.(json.Delim)
	if !container {
		return nil
	}
	return skipRichJSONContainerBounded(decoder, delimiter, maxDepth)
}

func skipRichJSONContainerBounded(decoder *json.Decoder, delimiter json.Delim, maxDepth int) error {
	if delimiter != '{' && delimiter != '[' {
		return errors.New("unexpected delimiter")
	}
	stack := []json.Delim{delimiter}
	for len(stack) > 0 {
		if len(stack) > maxDepth {
			return errors.New("JSON depth exceeded")
		}
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			continue
		}
		switch delim {
		case '{', '[':
			stack = append(stack, delim)
		case '}', ']':
			opening := stack[len(stack)-1]
			if (opening == '{' && delim != '}') || (opening == '[' && delim != ']') {
				return errors.New("mismatched JSON delimiter")
			}
			stack = stack[:len(stack)-1]
		default:
			return errors.New("unexpected delimiter")
		}
	}
	return nil
}

func validRichJSONStringEscapes(raw []byte) bool {
	for index := 0; index < len(raw); index++ {
		if raw[index] != '"' {
			continue
		}
		index++
		for index < len(raw) && raw[index] != '"' {
			if raw[index] < 0x20 {
				return false
			}
			if raw[index] != '\\' {
				index++
				continue
			}
			if index+1 >= len(raw) {
				return false
			}
			escape := raw[index+1]
			if escape != 'u' {
				if !strings.ContainsRune(`"\\/bfnrt`, rune(escape)) {
					return false
				}
				index += 2
				continue
			}
			value, ok := parseRichHex16(raw, index+2)
			if !ok {
				return false
			}
			index += 6
			switch {
			case value >= 0xd800 && value <= 0xdbff:
				if index+6 > len(raw) || raw[index] != '\\' || raw[index+1] != 'u' {
					return false
				}
				low, ok := parseRichHex16(raw, index+2)
				if !ok || low < 0xdc00 || low > 0xdfff {
					return false
				}
				index += 6
			case value >= 0xdc00 && value <= 0xdfff:
				return false
			}
		}
		if index >= len(raw) {
			return false
		}
	}
	return true
}

func parseRichHex16(raw []byte, start int) (uint16, bool) {
	if start+4 > len(raw) {
		return 0, false
	}
	var value uint16
	for index := start; index < start+4; index++ {
		digit, err := strconv.ParseUint(string(raw[index]), 16, 4)
		if err != nil {
			return 0, false
		}
		value = value<<4 | uint16(digit)
	}
	return value, true
}
