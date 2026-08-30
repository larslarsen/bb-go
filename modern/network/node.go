package network

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/ipfs/boxo/bitswap"
	bsnet "github.com/ipfs/boxo/bitswap/network/bsnet"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/boxo/namesys"
	boxopath "github.com/ipfs/boxo/path"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/amino"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	ma "github.com/multiformats/go-multiaddr"
)

var defaultListenAddrs = []string{
	"/ip4/0.0.0.0/tcp/4001",
	"/ip4/0.0.0.0/udp/4001/quic-v1",
}

const ipnsRecordLifetime = 7 * 24 * time.Hour

// Config contains the network-owned settings needed to construct a BitBook
// peer. Application and marketplace settings deliberately do not belong here.
type Config struct {
	PrivateKey     crypto.PrivKey
	Datastore      datastore.Batching
	ListenAddrs    []string
	BootstrapPeers []peer.AddrInfo
	DHTMode        dht.ModeOpt

	// AllowPrivateAddresses permits RFC1918 and loopback peers in the WAN DHT.
	// Enable it for local development or intentionally LAN-scoped networks.
	AllowPrivateAddresses bool
}

// Node is the maintained BitBook networking core. It owns a libp2p host, a
// namespaced Kademlia DHT, a local blockstore, and a namespaced Bitswap client
// and server.
type Node struct {
	Host       host.Host
	DHT        *dht.IpfsDHT
	Datastore  datastore.Batching
	Blockstore blockstore.Blockstore
	Bitswap    *bitswap.Bitswap
	Publisher  *namesys.IPNSPublisher
	Resolver   *namesys.IPNSResolver
	PrivateKey crypto.PrivKey

	cancel    context.CancelFunc
	closeOnce sync.Once
	closeErr  error
}

// New constructs an online node. A nil datastore selects an in-memory store;
// production callers should supply a persistent datastore.
func New(parent context.Context, cfg Config) (_ *Node, err error) {
	if parent == nil {
		return nil, errors.New("nil parent context")
	}

	ctx, cancel := context.WithCancel(parent)
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	if cfg.Datastore == nil {
		cfg.Datastore = dsync.MutexWrap(datastore.NewMapDatastore())
	}
	if len(cfg.ListenAddrs) == 0 {
		cfg.ListenAddrs = slices.Clone(defaultListenAddrs)
	}
	if cfg.DHTMode == 0 {
		cfg.DHTMode = dht.ModeAuto
	}

	hostOptions := []libp2p.Option{libp2p.ListenAddrStrings(cfg.ListenAddrs...)}
	if cfg.PrivateKey != nil {
		hostOptions = append(hostOptions, libp2p.Identity(cfg.PrivateKey))
	}

	h, err := libp2p.New(hostOptions...)
	if err != nil {
		return nil, fmt.Errorf("creating libp2p host: %w", err)
	}
	defer func() {
		if err != nil {
			_ = h.Close()
		}
	}()

	dhtOptions := []dht.Option{
		dht.Datastore(cfg.Datastore),
		dht.Mode(cfg.DHTMode),
		dht.ProtocolPrefix(DHTProtocolPrefix),
		dht.RoutingTablePeerDiversityFilter(dht.NewRTPeerDiversityFilter(h, amino.DefaultMaxPeersPerIPGroupPerCpl, amino.DefaultMaxPeersPerIPGroup)),
	}
	if len(cfg.BootstrapPeers) > 0 {
		dhtOptions = append(dhtOptions, dht.BootstrapPeers(cfg.BootstrapPeers...))
	}
	if cfg.AllowPrivateAddresses {
		dhtOptions = append(dhtOptions,
			dht.AddressFilter(nil),
			dht.QueryFilter(func(_ any, _ peer.AddrInfo) bool { return true }),
			dht.RoutingTableFilter(func(_ any, _ peer.ID) bool { return true }),
		)
	}

	kad, err := dht.New(h, dhtOptions...)
	if err != nil {
		return nil, fmt.Errorf("creating BitBook DHT: %w", err)
	}
	defer func() {
		if err != nil {
			_ = kad.Close()
		}
	}()

	store := blockstore.NewIdStore(blockstore.NewBlockstore(cfg.Datastore))
	bitswapNetwork := bsnet.NewFromIpfsHost(h, bsnet.Prefix(BitswapProtocolPrefix))
	exchange := bitswap.New(ctx, bitswapNetwork, kad, store)
	privateKey := h.Peerstore().PrivKey(h.ID())
	if privateKey == nil {
		return nil, errors.New("libp2p host did not retain its private identity key")
	}

	n := &Node{
		Host:       h,
		DHT:        kad,
		Datastore:  cfg.Datastore,
		Blockstore: store,
		Bitswap:    exchange,
		Publisher:  namesys.NewIPNSPublisher(kad, cfg.Datastore),
		Resolver:   namesys.NewIPNSResolver(kad),
		PrivateKey: privateKey,
		cancel:     cancel,
	}

	if err := kad.Bootstrap(ctx); err != nil {
		_ = n.Close()
		return nil, fmt.Errorf("bootstrapping BitBook DHT: %w", err)
	}
	for _, bootstrapPeer := range cfg.BootstrapPeers {
		if err := h.Connect(ctx, bootstrapPeer); err != nil {
			_ = n.Close()
			return nil, fmt.Errorf("connecting bootstrap peer %s: %w", bootstrapPeer.ID, err)
		}
	}

	return n, nil
}

// ID returns the stable libp2p identity of this node.
func (n *Node) ID() peer.ID { return n.Host.ID() }

// Addrs returns dialable addresses including this node's peer ID.
func (n *Node) Addrs() []ma.Multiaddr {
	peerComponent, err := ma.NewMultiaddr("/p2p/" + n.ID().String())
	if err != nil {
		return nil
	}
	result := make([]ma.Multiaddr, 0, len(n.Host.Addrs()))
	for _, addr := range n.Host.Addrs() {
		result = append(result, addr.Encapsulate(peerComponent))
	}
	return result
}

// Connect adds a peer to the local peerstore and opens a connection.
func (n *Node) Connect(ctx context.Context, info peer.AddrInfo) error {
	return n.Host.Connect(ctx, info)
}

// Put stores a block and announces it to connected Bitswap peers. Callers may
// separately use DHT.Provide when they want a network-wide provider record.
func (n *Node) Put(ctx context.Context, data []byte) (cid.Cid, error) {
	block := blocks.NewBlock(data)
	if err := n.Blockstore.Put(ctx, block); err != nil {
		return cid.Undef, fmt.Errorf("storing block: %w", err)
	}
	if err := n.Bitswap.NotifyNewBlocks(ctx, block); err != nil {
		return cid.Undef, fmt.Errorf("notifying Bitswap: %w", err)
	}
	return block.Cid(), nil
}

// Get retrieves a block locally or from the isolated BitBook Bitswap network.
func (n *Node) Get(ctx context.Context, id cid.Cid) ([]byte, error) {
	block, err := n.Bitswap.GetBlock(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting block %s: %w", id, err)
	}
	return slices.Clone(block.RawData()), nil
}

// PublishRoot signs and publishes a mutable pointer from this peer identity to
// an immutable content root. Sequence handling and local record persistence
// are provided by Boxo's current IPNS publisher.
func (n *Node) PublishRoot(ctx context.Context, root cid.Cid) error {
	if !root.Defined() {
		return errors.New("cannot publish an undefined root CID")
	}
	if err := n.DHT.Provide(ctx, root, true); err != nil {
		return fmt.Errorf("announcing BitBook root provider: %w", err)
	}
	if err := n.Publisher.Publish(ctx, n.PrivateKey, boxopath.FromCid(root), namesys.PublishWithEOL(time.Now().Add(ipnsRecordLifetime))); err != nil {
		return fmt.Errorf("publishing BitBook root: %w", err)
	}
	return nil
}

// ResolveRoot resolves another peer's signed mutable pointer to its immutable
// content root.
func (n *Node) ResolveRoot(ctx context.Context, owner peer.ID) (cid.Cid, error) {
	result, err := n.Resolver.Resolve(ctx, ipns.NameFromPeer(owner).AsPath(), namesys.ResolveWithDhtRecordCount(1))
	if err != nil {
		return cid.Undef, fmt.Errorf("resolving BitBook root for %s: %w", owner, err)
	}
	segments := result.Path.Segments()
	if len(segments) < 2 || segments[0] != boxopath.IPFSNamespace {
		return cid.Undef, fmt.Errorf("resolved BitBook root is not an IPFS path: %s", result.Path)
	}
	root, err := cid.Decode(segments[1])
	if err != nil {
		return cid.Undef, fmt.Errorf("decoding resolved BitBook root: %w", err)
	}
	return root, nil
}

// Protocols reports the protocols currently registered on the host. It is
// primarily useful for diagnostics and network-isolation assertions.
func (n *Node) Protocols() []protocol.ID { return n.Host.Mux().Protocols() }

// Close stops network services in dependency order.
func (n *Node) Close() error {
	if n == nil {
		return nil
	}
	n.closeOnce.Do(func() {
		n.cancel()
		n.closeErr = errors.Join(n.Bitswap.Close(), n.DHT.Close(), n.Host.Close())
	})
	return n.closeErr
}
