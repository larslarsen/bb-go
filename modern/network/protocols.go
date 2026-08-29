package network

import "github.com/libp2p/go-libp2p/core/protocol"

const (
	// DHTProtocolPrefix produces /bitbook/kad/1.0.0 with the current DHT.
	DHTProtocolPrefix  protocol.ID = "/bitbook"
	DHTProtocolCurrent protocol.ID = "/bitbook/kad/1.0.0"

	// BitswapProtocolPrefix produces /bitbook/ipfs/bitswap/<version>. The
	// embedded "ipfs" component is retained by Boxo's prefix API. This is the
	// version-two BitBook wire namespace and intentionally does not join the
	// public IPFS Bitswap network or the legacy BitBook 1.1 network.
	BitswapProtocolPrefix protocol.ID = "/bitbook"

	BitswapProtocolCurrent protocol.ID = "/bitbook/ipfs/bitswap/1.2.0"

	// DirectProtocolCurrent carries signed follows, chat messages, typing
	// indicators, and read receipts between authenticated BitBook peers.
	DirectProtocolCurrent protocol.ID = "/bitbook/direct/1.0.0"
)
