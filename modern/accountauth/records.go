package accountauth

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

const (
	recordVersion          byte = 1
	grantPayloadBytes           = 106
	revocationPayloadBytes      = 66
	revocationRecordBytes       = revocationPayloadBytes + ed25519.SignatureSize

	accountDomain      = "bitbook/accountauth/v1/account\x00"
	grantRootDomain    = "bitbook/accountauth/v1/grant/root\x00"
	grantDeviceDomain  = "bitbook/accountauth/v1/grant/device\x00"
	revokeGrantDomain  = "bitbook/accountauth/v1/revoke-grant\x00"
	revokeDeviceDomain = "bitbook/accountauth/v1/revoke-device\x00"
	grantIDDomain      = "bitbook/accountauth/v1/grant-id\x00"
)

const allCapabilities = CapMessage | CapPaymentRequest | CapPolicyWrite | CapAssessment | CapProfileUpdate

var (
	errInvalidController = errors.New("accountauth: invalid controller")
	errInvalidFraming    = errors.New("accountauth: invalid record framing")
	errInvalidGrant      = errors.New("accountauth: invalid grant")
	errInvalidRevocation = errors.New("accountauth: invalid revocation")
	errInvalidSignature  = errors.New("accountauth: invalid signature")
)

// AccountIDFor derives an account ID from a nonzero controller public key.
func AccountIDFor(controller [32]byte) (AccountID, error) {
	if isZero32(controller) {
		return AccountID{}, errInvalidController
	}
	return AccountID(digestWithDomain(accountDomain, controller[:])), nil
}

// VerifyRecord parses and authenticates one complete version 1 record.
func VerifyRecord(raw []byte) (Record, error) {
	if len(raw) < 2 || len(raw) > MaxRecordBytes {
		return Record{}, errInvalidFraming
	}
	if raw[0] != recordVersion {
		return Record{}, errInvalidFraming
	}

	kind := Kind(raw[1])
	switch kind {
	case Grant:
		if len(raw) != MaxRecordBytes {
			return Record{}, errInvalidFraming
		}
	case RevokeGrant, RevokeDevice:
		if len(raw) != revocationRecordBytes {
			return Record{}, errInvalidFraming
		}
	default:
		return Record{}, errInvalidFraming
	}

	var controller [32]byte
	copy(controller[:], raw[2:34])
	if isZero32(controller) {
		return Record{}, errInvalidController
	}
	accountID := AccountID(digestWithDomain(accountDomain, controller[:]))

	if kind == Grant {
		return verifyGrant(raw, controller, accountID)
	}
	return verifyRevocation(raw, kind, controller, accountID)
}

func verifyGrant(raw []byte, controller [32]byte, accountID AccountID) (Record, error) {
	var device [32]byte
	copy(device[:], raw[34:66])
	if isZero32(device) || device == controller {
		return Record{}, errInvalidGrant
	}

	capabilities := Capability(binary.BigEndian.Uint64(raw[66:74]))
	if capabilities == 0 || capabilities&^allCapabilities != 0 {
		return Record{}, errInvalidGrant
	}
	if isZeroBytes(raw[74:grantPayloadBytes]) {
		return Record{}, errInvalidGrant
	}

	payload := raw[:grantPayloadBytes]
	controllerMessage := messageForDomain(grantRootDomain, payload)
	if !ed25519.Verify(ed25519.PublicKey(controller[:]), controllerMessage,
		raw[grantPayloadBytes:grantPayloadBytes+ed25519.SignatureSize]) {
		return Record{}, errInvalidSignature
	}
	deviceMessage := messageForDomain(grantDeviceDomain, payload)
	if !ed25519.Verify(ed25519.PublicKey(device[:]), deviceMessage,
		raw[grantPayloadBytes+ed25519.SignatureSize:MaxRecordBytes]) {
		return Record{}, errInvalidSignature
	}

	return Record{
		kind:         Grant,
		accountID:    accountID,
		controller:   controller,
		grantID:      GrantID(digestWithDomain(grantIDDomain, payload)),
		device:       device,
		capabilities: capabilities,
		raw:          cloneRecordBytes(raw),
	}, nil
}

func verifyRevocation(raw []byte, kind Kind, controller [32]byte, accountID AccountID) (Record, error) {
	var target [32]byte
	copy(target[:], raw[34:66])
	if isZero32(target) || kind == RevokeDevice && target == controller {
		return Record{}, errInvalidRevocation
	}

	domain := revokeGrantDomain
	if kind == RevokeDevice {
		domain = revokeDeviceDomain
	}
	if !ed25519.Verify(ed25519.PublicKey(controller[:]), messageForDomain(domain, raw[:revocationPayloadBytes]),
		raw[revocationPayloadBytes:revocationRecordBytes]) {
		return Record{}, errInvalidSignature
	}

	record := Record{
		kind:       kind,
		accountID:  accountID,
		controller: controller,
		raw:        cloneRecordBytes(raw),
	}
	if kind == RevokeGrant {
		record.grantID = GrantID(target)
	} else {
		record.device = target
	}
	return record, nil
}

func digestWithDomain(domain string, payload []byte) [32]byte {
	preimage := make([]byte, len(domain)+len(payload))
	copy(preimage, domain)
	copy(preimage[len(domain):], payload)
	return sha256.Sum256(preimage)
}

func messageForDomain(domain string, payload []byte) []byte {
	message := make([]byte, len(domain)+len(payload))
	copy(message, domain)
	copy(message[len(domain):], payload)
	return message
}

func cloneRecordBytes(raw []byte) []byte {
	copyOfRaw := make([]byte, len(raw))
	copy(copyOfRaw, raw)
	return copyOfRaw
}

func isZero32(value [32]byte) bool {
	return value == ([32]byte{})
}

func isZeroBytes(value []byte) bool {
	for _, b := range value {
		if b != 0 {
			return false
		}
	}
	return true
}
