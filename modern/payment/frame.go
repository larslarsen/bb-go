package payment

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
)

var envelopeFields = []string{"version", "kind", "canonical", "public_key", "signature"}

func FrameLength(size int) (uint32, error) {
	if size < 1 || size > MaxFrameBytes {
		return 0, paymentError(CodeFrame, "payment frame length is out of bounds")
	}
	return uint32(size), nil
}

func WriteFrame(writer io.Writer, payload []byte) error {
	length, err := FrameLength(len(payload))
	if err != nil {
		return err
	}
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], length)
	if err := writeAll(writer, prefix[:]); err != nil {
		return paymentError(CodeFrame, "writing payment frame prefix")
	}
	if err := writeAll(writer, payload); err != nil {
		return paymentError(CodeFrame, "writing payment frame payload")
	}
	return nil
}

func ReadFrame(reader io.Reader) ([]byte, error) {
	var prefix [4]byte
	if _, err := io.ReadFull(reader, prefix[:]); err != nil {
		return nil, paymentError(CodeFrame, "reading payment frame prefix")
	}
	size := binary.BigEndian.Uint32(prefix[:])
	if size == 0 || size > MaxFrameBytes {
		return nil, paymentError(CodeFrame, "payment frame length is out of bounds")
	}
	payload := make([]byte, int(size))
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, paymentError(CodeFrame, "reading payment frame payload")
	}
	var trailing [1]byte
	if count, err := io.ReadFull(reader, trailing[:]); err != io.EOF || count != 0 {
		return nil, paymentError(CodeFrame, "payment frame contains trailing data")
	}
	return payload, nil
}

func DecodeSignedObject(raw []byte) (SignedObject, error) {
	value, err := parseStrictJSON(raw)
	if err != nil {
		return SignedObject{}, err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return SignedObject{}, schemaError("signed envelope must be an object")
	}
	if err := requireFields(object, envelopeFields); err != nil {
		return SignedObject{}, err
	}
	version, ok := object["version"].(json.Number)
	if !ok || version.String() != "1" {
		return SignedObject{}, schemaError("signed envelope version must be integer 1")
	}
	kindText, ok := object["kind"].(string)
	if !ok || (Kind(kindText) != KindRequest && Kind(kindText) != KindStatus) {
		return SignedObject{}, schemaError("unsupported signed envelope kind")
	}
	canonical, ok := object["canonical"].(string)
	if !ok {
		return SignedObject{}, schemaError("signed envelope canonical must be a string")
	}
	publicKey, err := decodeBase64Field(object, "public_key")
	if err != nil {
		return SignedObject{}, err
	}
	signature, err := decodeBase64Field(object, "signature")
	if err != nil {
		return SignedObject{}, err
	}
	return SignedObject{
		Version: 1, Kind: Kind(kindText), Canonical: canonical,
		PublicKey: publicKey, Signature: signature,
	}, nil
}

func ReceiveSignedObject(reader io.Reader) (SignedObject, error) {
	payload, err := ReadFrame(reader)
	if err != nil {
		return SignedObject{}, err
	}
	return DecodeSignedObject(payload)
}

func writeAll(writer io.Writer, raw []byte) error {
	for len(raw) > 0 {
		written, err := writer.Write(raw)
		if err != nil {
			return err
		}
		if written < 1 || written > len(raw) {
			return io.ErrShortWrite
		}
		raw = raw[written:]
	}
	return nil
}

func decodeBase64Field(object map[string]any, field string) ([]byte, error) {
	encoded, ok := object[field].(string)
	if !ok {
		return nil, schemaError(field + " must be a base64 string")
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || base64.StdEncoding.EncodeToString(decoded) != encoded {
		return nil, schemaError(field + " is not canonical base64")
	}
	return decoded, nil
}
