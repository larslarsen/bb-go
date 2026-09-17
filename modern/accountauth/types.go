package accountauth

// AccountID identifies the stable account controlled by one Ed25519 public key.
type AccountID [32]byte

// GrantID identifies a grant by its signed payload, excluding its signatures.
type GrantID [32]byte

// Capability is a mask of independently granted account operations.
type Capability uint64

// Kind identifies an account authorization record kind.
type Kind byte

const (
	Grant        Kind = 1
	RevokeGrant  Kind = 2
	RevokeDevice Kind = 3
)

const (
	CapMessage Capability = 1 << iota
	CapPaymentRequest
	CapPolicyWrite
	CapAssessment
	CapProfileUpdate
)

const (
	MaxRecordBytes  = 234
	MaxKnownRecords = 4096
)

// Record is a completely verified account authorization record.
type Record struct {
	kind         Kind
	accountID    AccountID
	controller   [32]byte
	grantID      GrantID
	device       [32]byte
	capabilities Capability
	raw          []byte
}

// Kind returns the record kind.
func (r Record) Kind() Kind {
	return r.kind
}

// AccountID returns the account derived from the record's controller.
func (r Record) AccountID() AccountID {
	return r.accountID
}

// GrantID returns a grant's derived ID or a grant revocation's target.
func (r Record) GrantID() GrantID {
	return r.grantID
}

// DeviceKey returns a grant's bound device or a device revocation's target.
func (r Record) DeviceKey() [32]byte {
	return r.device
}

// Capabilities returns a grant's capability mask.
func (r Record) Capabilities() Capability {
	return r.capabilities
}

// Bytes returns an owned copy of the verified record bytes.
func (r Record) Bytes() []byte {
	if r.raw == nil {
		return nil
	}
	raw := make([]byte, len(r.raw))
	copy(raw, r.raw)
	return raw
}
