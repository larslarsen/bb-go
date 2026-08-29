package social

import (
	"encoding/json"
	"time"
)

const ManifestVersion = 1

// Manifest is the small signed-IPNS target that links a peer's independently
// addressable social sections.
type Manifest struct {
	Version   int       `json:"version"`
	Author    string    `json:"author"`
	Profile   string    `json:"profile"`
	Posts     string    `json:"posts"`
	Following string    `json:"following"`
	Followers string    `json:"followers,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// SignedPost is an immutable post plus the author's libp2p signature. CID is
// the address of the encoded post envelope and is populated in indexes/API
// responses rather than included in the signed block itself.
type SignedPost struct {
	Post      json.RawMessage `json:"post"`
	Signature []byte          `json:"signature"`
	PublicKey []byte          `json:"publicKey"`
	CID       string          `json:"hash,omitempty"`
}

// State is the resolved social state for one peer.
type State struct {
	Manifest  Manifest        `json:"manifest"`
	Profile   json.RawMessage `json:"profile"`
	Posts     []SignedPost    `json:"posts"`
	Following []string        `json:"following"`
	Followers []string        `json:"followers"`
}

type followerState struct {
	Following bool      `json:"following"`
	UpdatedAt time.Time `json:"updatedAt"`
}
