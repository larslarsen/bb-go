package core

// RuntimeMode controls which product-level features a node exposes while
// retaining the same underlying identity, IPFS, and libp2p infrastructure.
type RuntimeMode string

const (
	// RuntimeModeFull preserves the historical OpenBazaar behavior.
	RuntimeModeFull RuntimeMode = "full"
	// RuntimeModeSocial runs the BitBook social feature set without marketplace
	// endpoints, peer messages, or background workers.
	RuntimeModeSocial RuntimeMode = "social"
)

// IsSocialOnly reports whether marketplace behavior should be disabled.
func (n *OpenBazaarNode) IsSocialOnly() bool {
	return n != nil && n.RuntimeMode == RuntimeModeSocial
}
