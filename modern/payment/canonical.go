package payment

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

var requestFields = []string{
	"v", "request_id", "payer_peer_id", "payee_peer_id", "asset", "network",
	"amount_atomic", "receiver", "receiver_kind", "memo", "nonce", "created_at", "expires_at",
}

var statusFields = []string{"v", "request_id", "event_id", "nonce", "status", "at", "tx_ref"}

const maxJSONContainerDepth = 32

func CanonicalJSON(raw []byte) ([]byte, error) {
	value, err := parseStrictJSON(raw)
	if err != nil {
		return nil, err
	}
	var canonical bytes.Buffer
	if err := encodeCanonical(&canonical, value); err != nil {
		return nil, err
	}
	return canonical.Bytes(), nil
}

func DecodePaymentRequest(raw []byte) (PaymentRequestV1, []byte, string, error) {
	value, err := parseStrictJSON(raw)
	if err != nil {
		return PaymentRequestV1{}, nil, "", err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return PaymentRequestV1{}, nil, "", schemaError("payment request must be an object")
	}
	if err := requireFields(object, requestFields); err != nil {
		return PaymentRequestV1{}, nil, "", err
	}
	request, err := requestFromObject(object)
	if err != nil {
		return PaymentRequestV1{}, nil, "", err
	}
	canonical, err := canonicalValue(value)
	if err != nil {
		return PaymentRequestV1{}, nil, "", err
	}
	return request, canonical, DigestHex(DomainSeparatorRequest, canonical), nil
}

func DecodePaymentStatus(raw []byte) (PaymentStatusEventV1, []byte, string, error) {
	value, err := parseStrictJSON(raw)
	if err != nil {
		return PaymentStatusEventV1{}, nil, "", err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return PaymentStatusEventV1{}, nil, "", schemaError("payment status must be an object")
	}
	if err := requireFields(object, statusFields); err != nil {
		return PaymentStatusEventV1{}, nil, "", err
	}
	status, err := statusFromObject(object)
	if err != nil {
		return PaymentStatusEventV1{}, nil, "", err
	}
	canonical, err := canonicalValue(value)
	if err != nil {
		return PaymentStatusEventV1{}, nil, "", err
	}
	return status, canonical, DigestHex(DomainSeparatorStatus, canonical), nil
}

func CheckCanonicalCopy(canonical, supplied []byte) error {
	if !bytes.Equal(canonical, supplied) {
		return schemaError("canonical copy does not match RFC 8785 bytes")
	}
	return nil
}

func DigestHex(domain string, canonical []byte) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte(domain))
	_, _ = digest.Write(canonical)
	return hex.EncodeToString(digest.Sum(nil))
}

func parseStrictJSON(raw []byte) (any, error) {
	return parseStrictJSONMode(raw, false)
}

func parseStrictJSONAllowBoolean(raw []byte) (any, error) {
	return parseStrictJSONMode(raw, true)
}

func parseStrictJSONMode(raw []byte, allowBoolean bool) (any, error) {
	if len(raw) == 0 || !utf8.Valid(raw) || bytes.HasPrefix(raw, []byte{0xef, 0xbb, 0xbf}) {
		return nil, schemaError("invalid UTF-8 JSON")
	}
	if !validEscapedSurrogates(raw) {
		return nil, schemaError("JSON contains an unpaired surrogate")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := parseJSONValue(decoder, allowBoolean, 0)
	if err != nil {
		return nil, schemaError(err.Error())
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return nil, schemaError("trailing JSON data")
		}
		return nil, schemaError("invalid trailing JSON data")
	}
	return value, nil
}

func parseJSONValue(decoder *json.Decoder, allowBoolean bool, depth int) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			if depth >= maxJSONContainerDepth {
				return nil, fmt.Errorf("JSON exceeds %d nested containers", maxJSONContainerDepth)
			}
			object := make(map[string]any)
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyToken.(string)
				if !ok || key == "" {
					return nil, fmt.Errorf("invalid object key")
				}
				if err := validateSignedString(key); err != nil {
					return nil, err
				}
				if _, duplicate := object[key]; duplicate {
					return nil, fmt.Errorf("duplicate object key %q", key)
				}
				child, err := parseJSONValue(decoder, allowBoolean, depth+1)
				if err != nil {
					return nil, err
				}
				object[key] = child
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim('}') {
				return nil, fmt.Errorf("unterminated object")
			}
			return object, nil
		case '[':
			if depth >= maxJSONContainerDepth {
				return nil, fmt.Errorf("JSON exceeds %d nested containers", maxJSONContainerDepth)
			}
			array := make([]any, 0)
			for decoder.More() {
				child, err := parseJSONValue(decoder, allowBoolean, depth+1)
				if err != nil {
					return nil, err
				}
				array = append(array, child)
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim(']') {
				return nil, fmt.Errorf("unterminated array")
			}
			return array, nil
		default:
			return nil, fmt.Errorf("unexpected JSON delimiter")
		}
	case string:
		if err := validateSignedString(value); err != nil {
			return nil, err
		}
		return value, nil
	case json.Number:
		if value.String() != "1" {
			return nil, fmt.Errorf("unsupported JSON number %q", value)
		}
		return value, nil
	case bool:
		if allowBoolean {
			return value, nil
		}
		return nil, fmt.Errorf("unsupported JSON Boolean")
	default:
		return nil, fmt.Errorf("unsupported JSON value")
	}
}

func validEscapedSurrogates(raw []byte) bool {
	inString := false
	for index := 0; index < len(raw); index++ {
		if raw[index] == '"' {
			inString = !inString
			continue
		}
		if !inString || raw[index] != '\\' {
			continue
		}
		index++
		if index >= len(raw) {
			return false
		}
		if raw[index] != 'u' {
			continue
		}
		value, ok := hexQuad(raw, index+1)
		if !ok {
			return false
		}
		index += 4
		if value >= 0xdc00 && value <= 0xdfff {
			return false
		}
		if value < 0xd800 || value > 0xdbff {
			continue
		}
		if index+6 >= len(raw) || raw[index+1] != '\\' || raw[index+2] != 'u' {
			return false
		}
		low, ok := hexQuad(raw, index+3)
		if !ok || low < 0xdc00 || low > 0xdfff {
			return false
		}
		index += 6
	}
	return !inString
}

func hexQuad(raw []byte, start int) (uint16, bool) {
	if start+4 > len(raw) {
		return 0, false
	}
	var value uint16
	for _, digit := range raw[start : start+4] {
		value <<= 4
		switch {
		case digit >= '0' && digit <= '9':
			value += uint16(digit - '0')
		case digit >= 'a' && digit <= 'f':
			value += uint16(digit-'a') + 10
		case digit >= 'A' && digit <= 'F':
			value += uint16(digit-'A') + 10
		default:
			return 0, false
		}
	}
	return value, true
}

func canonicalValue(value any) ([]byte, error) {
	var output bytes.Buffer
	if err := encodeCanonical(&output, value); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func encodeCanonical(output *bytes.Buffer, value any) error {
	switch value := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool { return lessUTF16(keys[i], keys[j]) })
		output.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				output.WriteByte(',')
			}
			writeJSONString(output, key)
			output.WriteByte(':')
			if err := encodeCanonical(output, value[key]); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	case []any:
		output.WriteByte('[')
		for index, child := range value {
			if index > 0 {
				output.WriteByte(',')
			}
			if err := encodeCanonical(output, child); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	case string:
		writeJSONString(output, value)
	case json.Number:
		if value.String() != "1" {
			return schemaError("unsupported JSON number")
		}
		output.WriteByte('1')
	case bool:
		if value {
			output.WriteString("true")
		} else {
			output.WriteString("false")
		}
	default:
		return schemaError("unsupported JSON value")
	}
	return nil
}

func lessUTF16(left, right string) bool {
	a := utf16.Encode([]rune(left))
	b := utf16.Encode([]rune(right))
	for index := 0; index < len(a) && index < len(b); index++ {
		if a[index] != b[index] {
			return a[index] < b[index]
		}
	}
	return len(a) < len(b)
}

func writeJSONString(output *bytes.Buffer, value string) {
	output.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"', '\\':
			output.WriteByte('\\')
			output.WriteRune(r)
		case '\b':
			output.WriteString(`\b`)
		case '\t':
			output.WriteString(`\t`)
		case '\n':
			output.WriteString(`\n`)
		case '\f':
			output.WriteString(`\f`)
		case '\r':
			output.WriteString(`\r`)
		default:
			if r < 0x20 {
				fmt.Fprintf(output, `\u%04x`, r)
			} else {
				output.WriteRune(r)
			}
		}
	}
	output.WriteByte('"')
}

func requestFromObject(object map[string]any) (PaymentRequestV1, error) {
	if err := requireVersion(object); err != nil {
		return PaymentRequestV1{}, err
	}
	values, err := stringFields(object, requestFields[1:])
	if err != nil {
		return PaymentRequestV1{}, err
	}
	request := PaymentRequestV1{
		V: 1, RequestID: values["request_id"], PayerPeerID: values["payer_peer_id"],
		PayeePeerID: values["payee_peer_id"], Asset: values["asset"], Network: values["network"],
		AmountAtomic: values["amount_atomic"], Receiver: values["receiver"],
		ReceiverKind: values["receiver_kind"], Memo: values["memo"], Nonce: values["nonce"],
		CreatedAt: values["created_at"], ExpiresAt: values["expires_at"],
	}
	if !isLowerHex32(request.RequestID) || !isLowerHex32(request.Nonce) {
		return PaymentRequestV1{}, schemaError("request ID and nonce must be 32 lowercase hex")
	}
	if !isPrintableASCII(request.PayerPeerID, false) || !isPrintableASCII(request.PayeePeerID, false) ||
		!isPrintableASCII(request.Receiver, false) {
		return PaymentRequestV1{}, schemaError("peer IDs and receiver must be nonempty printable ASCII")
	}
	if !validAssetRelation(request.Asset, request.Network, request.ReceiverKind) {
		return PaymentRequestV1{}, schemaError("invalid asset, network, or receiver-kind relation")
	}
	if !validAmount(request.AmountAtomic) {
		return PaymentRequestV1{}, schemaError("invalid atomic amount")
	}
	if len(request.Memo) > 512 || !norm.NFC.IsNormalString(request.Memo) {
		return PaymentRequestV1{}, schemaError("memo must be NFC and at most 512 bytes")
	}
	created, err := parseTimestamp(request.CreatedAt)
	if err != nil {
		return PaymentRequestV1{}, err
	}
	expires, err := parseTimestamp(request.ExpiresAt)
	if err != nil {
		return PaymentRequestV1{}, err
	}
	if !expires.After(created) {
		return PaymentRequestV1{}, schemaError("expiry must be after creation")
	}
	return request, nil
}

func statusFromObject(object map[string]any) (PaymentStatusEventV1, error) {
	if err := requireVersion(object); err != nil {
		return PaymentStatusEventV1{}, err
	}
	values, err := stringFields(object, statusFields[1:])
	if err != nil {
		return PaymentStatusEventV1{}, err
	}
	status := PaymentStatusEventV1{
		V: 1, RequestID: values["request_id"], EventID: values["event_id"],
		Nonce: values["nonce"], Status: values["status"], At: values["at"], TxRef: values["tx_ref"],
	}
	if !isLowerHex32(status.RequestID) || !isLowerHex32(status.EventID) || !isLowerHex32(status.Nonce) {
		return PaymentStatusEventV1{}, schemaError("status IDs and nonce must be 32 lowercase hex")
	}
	if _, err := parseTimestamp(status.At); err != nil {
		return PaymentStatusEventV1{}, err
	}
	if !isPrintableASCII(status.TxRef, true) {
		return PaymentStatusEventV1{}, schemaError("transaction reference must be printable ASCII")
	}
	switch status.Status {
	case StatusPaid:
		if status.TxRef == "" {
			return PaymentStatusEventV1{}, schemaError("paid status requires a transaction reference")
		}
	case StatusCancelled, StatusExpired:
		if status.TxRef != "" {
			return PaymentStatusEventV1{}, schemaError("non-paid status cannot include a transaction reference")
		}
	default:
		return PaymentStatusEventV1{}, schemaError("unsupported payment status")
	}
	return status, nil
}

func requireFields(object map[string]any, fields []string) error {
	if len(object) != len(fields) {
		return schemaError("closed schema field count mismatch")
	}
	for _, field := range fields {
		if _, ok := object[field]; !ok {
			return schemaError("missing required field " + field)
		}
	}
	return nil
}

func requireVersion(object map[string]any) error {
	version, ok := object["v"].(json.Number)
	if !ok || version.String() != "1" {
		return schemaError("version must be integer 1")
	}
	return nil
}

func stringFields(object map[string]any, fields []string) (map[string]string, error) {
	result := make(map[string]string, len(fields))
	for _, field := range fields {
		value, ok := object[field].(string)
		if !ok {
			return nil, schemaError(field + " must be a string")
		}
		result[field] = value
	}
	return result, nil
}

func validateSignedString(value string) error {
	if !utf8.ValidString(value) {
		return schemaError("signed string is not valid UTF-8")
	}
	for _, r := range value {
		if r <= 0x1f || (r >= 0x7f && r <= 0x9f) ||
			(r >= 0xfdd0 && r <= 0xfdef) || r&0xffff == 0xfffe || r&0xffff == 0xffff ||
			(r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069) ||
			(r >= 0x200b && r <= 0x200f) || r == 0x061c || r == 0x2060 ||
			(r >= 0x206a && r <= 0x206f) || r == 0xfeff ||
			(r >= 0xfff9 && r <= 0xfffb) || (r >= 0xe0001 && r <= 0xe007f) {
			return schemaError("signed string contains a prohibited code point")
		}
	}
	return nil
}

func isPrintableASCII(value string, allowEmpty bool) bool {
	if value == "" {
		return allowEmpty
	}
	for index := 0; index < len(value); index++ {
		if value[index] < 0x20 || value[index] > 0x7e {
			return false
		}
	}
	return true
}

func isLowerHex32(value string) bool {
	if len(value) != 32 {
		return false
	}
	for index := range value {
		if !((value[index] >= '0' && value[index] <= '9') || (value[index] >= 'a' && value[index] <= 'f')) {
			return false
		}
	}
	return true
}

func validAmount(value string) bool {
	if len(value) < 1 || len(value) > 20 || value[0] < '1' || value[0] > '9' {
		return false
	}
	for index := 1; index < len(value); index++ {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

func validAssetRelation(asset, network, receiverKind string) bool {
	switch asset {
	case "ZEC":
		return receiverKind == "zec-ua-orchard-protocol" &&
			(network == "zec-mainnet" || network == "zec-testnet" || network == "zec-regtest")
	case "XMR":
		return receiverKind == "xmr-subaddress" &&
			(network == "xmr-mainnet" || network == "xmr-stagenet" || network == "xmr-testnet")
	default:
		return false
	}
}

func parseTimestamp(value string) (time.Time, error) {
	if len(value) != len("2026-08-30T12:00:00Z") || value[4] != '-' || value[7] != '-' ||
		value[10] != 'T' || value[13] != ':' || value[16] != ':' || value[19] != 'Z' {
		return time.Time{}, schemaError("timestamp must use second-precision UTC Z form")
	}
	for _, index := range []int{0, 1, 2, 3, 5, 6, 8, 9, 11, 12, 14, 15, 17, 18} {
		if value[index] < '0' || value[index] > '9' {
			return time.Time{}, schemaError("timestamp contains a non-digit")
		}
	}
	parsed, err := time.Parse("2006-01-02T15:04:05Z", value)
	if err != nil || parsed.Year() < 2020 || parsed.Year() > 2100 || parsed.UTC().Format("2006-01-02T15:04:05Z") != value {
		return time.Time{}, schemaError("timestamp is not a valid in-range Gregorian instant")
	}
	return parsed.UTC(), nil
}

func schemaError(message string) error {
	return paymentError(CodeSchema, strings.TrimSpace(message))
}
