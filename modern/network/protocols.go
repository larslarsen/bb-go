package network

import "github.com/libp2p/go-libp2p/core/protocol"

const (
	// DHTProtocolPrefix joins the public IPFS DHT.
	DHTProtocolPrefix  protocol.ID = "/ipfs"
	DHTProtocolCurrent protocol.ID = "/ipfs/kad/1.0.0"

	// An empty Bitswap prefix retains Boxo's standard public IPFS protocol IDs.
	BitswapProtocolPrefix protocol.ID = ""

	BitswapProtocolCurrent protocol.ID = "/ipfs/bitswap/1.2.0"

	// DiscoveryProtocolCurrent confirms that an authenticated libp2p peer is
	// participating in BitBook after public-DHT routing discovery finds it.
	DiscoveryProtocolCurrent protocol.ID = "/bitbook/discovery/1.0.0"
	DiscoveryNamespace                   = "/bitbook/peers/1.0.0"

	// DirectProtocolCurrent carries signed follows, chat messages, typing
	// indicators, and read receipts between authenticated BitBook peers.
	DirectProtocolCurrent protocol.ID = "/bitbook/direct/1.0.0"

	// PaymentProtocolCurrent carries signed payer-bound payment objects.
	PaymentProtocolCurrent protocol.ID = "/bitbook/payment/1.0.0"
)
