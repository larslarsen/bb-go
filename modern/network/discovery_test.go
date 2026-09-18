package network

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ipfs/boxo/bitswap"
	bsnet "github.com/ipfs/boxo/bitswap/network/bsnet"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/namesys"
	datastore "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	lp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	routingdiscovery "github.com/libp2p/go-libp2p/p2p/discovery/routing"
)

const fixturePeerHello = "BBGO001\n"

type upstreamFixture struct {
	host      host.Host
	dht       *dht.IpfsDHT
	datastore datastore.Batching
	blocks    blockstore.Blockstore
	bitswap   *bitswap.Bitswap
	publisher *namesys.IPNSPublisher
	resolver  *namesys.IPNSResolver
}

func newUpstreamFixture(t testing.TB, ctx context.Context) *upstreamFixture {
	t.Helper()
	store := dsync.MutexWrap(datastore.NewMapDatastore())
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatalf("create independent host: %v", err)
	}
	kad, err := dht.New(h,
		dht.Datastore(store),
		dht.Mode(dht.ModeServer),
		dht.ProtocolPrefix(protocol.ID("/ipfs")),
		dht.BootstrapPeers(),
		dht.AddressFilter(nil),
		dht.QueryFilter(func(_ any, _ peer.AddrInfo) bool { return true }),
		dht.RoutingTableFilter(func(_ any, _ peer.ID) bool { return true }),
	)
	if err != nil {
		_ = h.Close()
		t.Fatalf("create independent public DHT: %v", err)
	}
	if err := kad.Bootstrap(ctx); err != nil {
		_ = kad.Close()
		_ = h.Close()
		t.Fatalf("bootstrap independent public DHT: %v", err)
	}
	blocks := blockstore.NewIdStore(blockstore.NewBlockstore(store))
	exchange := bitswap.New(ctx, bsnet.NewFromIpfsHost(h, bsnet.Prefix(protocol.ID(""))), kad, blocks)
	fixture := &upstreamFixture{
		host:      h,
		dht:       kad,
		datastore: store,
		blocks:    blocks,
		bitswap:   exchange,
		publisher: namesys.NewIPNSPublisher(kad, store),
		resolver:  namesys.NewIPNSResolver(kad),
	}
	t.Cleanup(func() {
		_ = fixture.bitswap.Close()
		_ = fixture.dht.Close()
		_ = fixture.host.Close()
	})
	return fixture
}

func (u *upstreamFixture) addrInfo() peer.AddrInfo {
	return peer.AddrInfo{ID: u.host.ID(), Addrs: slices.Clone(u.host.Addrs())}
}

func connectUpstreamAndNode(t *testing.T, ctx context.Context, upstream *upstreamFixture, node *Node) {
	t.Helper()
	if err := node.Host.Connect(ctx, upstream.addrInfo()); err != nil {
		t.Fatalf("node connect upstream: %v", err)
	}
	if err := upstream.host.Connect(ctx, peer.AddrInfo{ID: node.ID(), Addrs: slices.Clone(node.Host.Addrs())}); err != nil {
		t.Fatalf("upstream connect node: %v", err)
	}
	if err := node.DHT.Bootstrap(ctx); err != nil {
		t.Fatalf("bootstrap node DHT: %v", err)
	}
	if err := upstream.dht.Bootstrap(ctx); err != nil {
		t.Fatalf("bootstrap upstream DHT: %v", err)
	}
	waitForRoutingPeer(t, ctx, node)
	waitForUpstreamRoutingPeer(t, ctx, upstream.dht)
}

func TestNET001PeerHelloLanguage(t *testing.T) {
	errNotEOF := errors.New("fixture did not produce EOF")
	cases := []struct {
		name string
		r    io.Reader
		ok   bool
	}{
		{name: "exact", r: bytes.NewBufferString(fixturePeerHello), ok: true},
		{name: "empty", r: bytes.NewReader(nil)},
		{name: "seven", r: bytes.NewBufferString("BBGO001")},
		{name: "wrong eight", r: bytes.NewBufferString("BBGO002\n")},
		{name: "nine", r: bytes.NewBufferString(fixturePeerHello + "X")},
		{name: "large", r: bytes.NewReader(bytes.Repeat([]byte{'X'}, 1<<20))},
		{name: "missing EOF", r: io.MultiReader(bytes.NewBufferString(fixturePeerHello), errorReader{errNotEOF})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := readPeerHello(tc.r)
			if tc.ok && err != nil {
				t.Fatalf("valid hello: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("invalid hello accepted")
			}
		})
	}
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

func TestNET001DiscoveryLifecycleAndHandshake(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	seed := newUpstreamFixture(t, ctx)
	config := Config{
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		BootstrapPeers:        []peer.AddrInfo{seed.addrInfo()},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	}
	a, err := New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	waitForTransportConnection(t, ctx, a, seed.host.ID())
	waitForTransportConnection(t, ctx, b, seed.host.ID())
	waitForRoutingPeer(t, ctx, a)
	waitForRoutingPeer(t, ctx, b)

	if err := a.StartDiscovery(nil); err == nil {
		t.Fatal("nil discovery context accepted")
	}
	cancelled, cancelNow := context.WithCancel(ctx)
	cancelNow()
	if err := a.StartDiscovery(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled StartDiscovery = %v", err)
	}
	if err := a.StartDiscovery(ctx); err != nil {
		t.Fatalf("start A discovery: %v", err)
	}
	if err := a.StartDiscovery(ctx); err != nil {
		t.Fatalf("repeat A discovery: %v", err)
	}
	seedDiscovery := routingdiscovery.NewRoutingDiscovery(seed.dht)
	waitForUpstreamAdvertisement(t, ctx, seedDiscovery, a.ID())
	if err := b.StartDiscovery(ctx); err != nil {
		t.Fatalf("start B discovery: %v", err)
	}
	waitForBitBookPeer(t, ctx, b, a.ID())
	waitForUpstreamAdvertisement(t, ctx, seedDiscovery, b.ID())
	if err := a.runDiscoveryRound(ctx); err != nil {
		t.Fatalf("A later discovery round: %v", err)
	}
	waitForBitBookPeer(t, ctx, a, b.ID())
	if len(a.BitBookPeers()) != 1 || len(b.BitBookPeers()) != 1 {
		t.Fatalf("unexpected discovered peers: A=%v B=%v", a.BitBookPeers(), b.BitBookPeers())
	}
	restartNode, err := New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer restartNode.Close()
	discoveryCtx, stopDiscovery := context.WithCancel(ctx)
	if err := restartNode.StartDiscovery(discoveryCtx); err != nil {
		t.Fatal(err)
	}
	stopDiscovery()
	waitForDiscoveryRestartRejection(t, ctx, restartNode)

	if err := a.Host.Network().ClosePeer(b.ID()); err != nil {
		t.Fatalf("disconnect discovered peer: %v", err)
	}
	waitForNoBitBookPeer(t, ctx, a, b.ID())
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if err := a.StartDiscovery(ctx); err == nil {
		t.Fatal("closed node restarted discovery")
	}
}

func TestNET001InboundHelloValidationDisconnectAndCloseCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	node, err := New(ctx, Config{
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer node.Close()
	client, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		_ = node.Close()
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.Connect(ctx, peer.AddrInfo{ID: node.ID(), Addrs: slices.Clone(node.Host.Addrs())}); err != nil {
		_ = node.Close()
		t.Fatal(err)
	}

	stream, err := client.NewStream(ctx, node.ID(), DiscoveryProtocolCurrent)
	if err != nil {
		_ = node.Close()
		t.Fatal(err)
	}
	_ = stream.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.WriteString(stream, fixturePeerHello); err != nil {
		_ = node.Close()
		t.Fatal(err)
	}
	if err := stream.CloseWrite(); err != nil {
		_ = node.Close()
		t.Fatal(err)
	}
	if err := readPeerHello(stream); err != nil {
		_ = node.Close()
		t.Fatalf("valid discovery response: %v", err)
	}
	_ = stream.Close()
	waitForBitBookPeer(t, ctx, node, client.ID())

	connections := client.Network().ConnsToPeer(node.ID())
	if len(connections) != 1 {
		_ = node.Close()
		t.Fatalf("initial client connection count = %d, want 1", len(connections))
	}
	clientDisconnected := make(chan struct{}, 1)
	clientObserver := &lp2pnet.NotifyBundle{
		DisconnectedF: func(_ lp2pnet.Network, connection lp2pnet.Conn) {
			if connection.RemotePeer() == node.ID() {
				select {
				case clientDisconnected <- struct{}{}:
				default:
				}
			}
		},
	}
	client.Network().Notify(clientObserver)
	defer client.Network().StopNotify(clientObserver)

	if err := node.Host.Network().ClosePeer(client.ID()); err != nil {
		_ = node.Close()
		t.Fatal(err)
	}
	waitForNoBitBookPeer(t, ctx, node, client.ID())
	waitSignal(t, ctx, clientDisconnected, "client-side disconnect")
	if err := client.Connect(ctx, peer.AddrInfo{ID: node.ID(), Addrs: slices.Clone(node.Host.Addrs())}); err != nil {
		_ = node.Close()
		t.Fatal(err)
	}
	reconnected := client.Network().ConnsToPeer(node.ID())
	if client.Network().Connectedness(node.ID()) != lp2pnet.Connected || len(reconnected) == 0 || slices.ContainsFunc(reconnected, func(connection lp2pnet.Conn) bool {
		return connection.ID() == connections[0].ID()
	}) {
		_ = node.Close()
		t.Fatalf("reconnect did not establish a fresh live client connection: %v", reconnected)
	}
	malformed, err := client.NewStream(ctx, node.ID(), DiscoveryProtocolCurrent)
	if err != nil {
		_ = node.Close()
		t.Fatal(err)
	}
	_ = malformed.SetDeadline(time.Now().Add(2 * time.Second))
	_, _ = io.WriteString(malformed, fixturePeerHello+"X")
	_ = malformed.CloseWrite()
	var one [1]byte
	if _, err := malformed.Read(one[:]); err == nil {
		_ = node.Close()
		t.Fatal("malformed hello stream was not reset")
	} else if isTimeoutError(err) {
		_ = node.Close()
		t.Fatalf("malformed hello reached local deadline instead of prompt reset: %v", err)
	} else if !errors.Is(err, lp2pnet.ErrReset) {
		_ = node.Close()
		t.Fatalf("malformed hello closed without a stream reset: %v", err)
	}
	if slices.Contains(node.BitBookPeers(), client.ID()) {
		_ = node.Close()
		t.Fatal("malformed reconnect restored confirmation")
	}
	if err := node.waitForDiscoveryHandlers(ctx, 0); err != nil {
		_ = node.Close()
		t.Fatalf("malformed hello handler did not finish: %v", err)
	}

	stalled, err := client.NewStream(ctx, node.ID(), DiscoveryProtocolCurrent)
	if err != nil {
		_ = node.Close()
		t.Fatal(err)
	}
	if _, err := stalled.Write([]byte{'B'}); err != nil {
		_ = node.Close()
		t.Fatalf("force stalled hello negotiation: %v", err)
	}
	if err := node.waitForDiscoveryHandlers(ctx, 1); err != nil {
		_ = stalled.Reset()
		_ = node.Close()
		t.Fatalf("stalled hello did not enter production I/O: %v", err)
	}
	closeDone := make(chan error, 1)
	go func() { closeDone <- node.Close() }()
	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("Close with stalled hello: %v", err)
		}
	case <-time.After(5 * time.Second):
		_ = stalled.Reset()
		t.Fatal("Close did not cancel stalled hello")
	}
	if _, err := stalled.Read(one[:]); err == nil {
		t.Fatal("stalled stream remained usable after Close")
	} else if !errors.Is(err, lp2pnet.ErrReset) {
		t.Fatalf("Close ended stalled discovery I/O without reset: %v", err)
	}
	if err := node.waitForDiscoveryHandlers(ctx, 0); err != nil {
		t.Fatalf("Close returned before discovery handlers joined: %v", err)
	}
}

func TestNET001ConfirmationDoesNotSurviveReconnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	subject, err := New(ctx, Config{
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer subject.Close()
	client, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	subjectInfo := peer.AddrInfo{ID: subject.ID(), Addrs: slices.Clone(subject.Host.Addrs())}
	if err := client.Connect(ctx, subjectInfo); err != nil {
		t.Fatal(err)
	}
	completeDiscoveryHello(t, ctx, client, subject.ID())
	waitForBitBookPeer(t, ctx, subject, client.ID())

	connections := subject.Host.Network().ConnsToPeer(client.ID())
	if len(connections) != 1 {
		t.Fatalf("initial connection count = %d, want 1", len(connections))
	}
	clientDisconnected := make(chan struct{}, 1)
	clientObserver := &lp2pnet.NotifyBundle{
		DisconnectedF: func(_ lp2pnet.Network, connection lp2pnet.Conn) {
			if connection.RemotePeer() == subject.ID() {
				select {
				case clientDisconnected <- struct{}{}:
				default:
				}
			}
		},
	}
	client.Network().Notify(clientObserver)
	defer client.Network().StopNotify(clientObserver)
	original := subject.discovery.notifiee
	entered := make(chan struct{})
	release := make(chan struct{})
	delivered := make(chan struct{})
	delayed := &delayedDiscoveryDisconnectNotifiee{
		Notifiee:  original,
		connID:    connections[0].ID(),
		entered:   entered,
		release:   release,
		delivered: delivered,
	}
	subject.Host.Network().StopNotify(original)
	subject.Host.Network().Notify(delayed)
	var releaseOnce sync.Once
	releaseDisconnect := func() { releaseOnce.Do(func() { close(release) }) }
	callbackEntered := false
	defer func() {
		releaseDisconnect()
		if callbackEntered {
			select {
			case <-delivered:
			case <-time.After(5 * time.Second):
				t.Errorf("delayed disconnect callback cleanup did not finish")
			}
		}
		subject.Host.Network().StopNotify(delayed)
		subject.Host.Network().Notify(original)
	}()

	if err := subject.Host.Network().ClosePeer(client.ID()); err != nil {
		t.Fatal(err)
	}
	waitSignal(t, ctx, entered, "production disconnect callback")
	callbackEntered = true
	waitSignal(t, ctx, clientDisconnected, "client-side disconnect")
	if err := client.Connect(ctx, subjectInfo); err != nil {
		t.Fatal(err)
	}
	reconnected := subject.Host.Network().ConnsToPeer(client.ID())
	if len(reconnected) == 0 || slices.ContainsFunc(reconnected, func(connection lp2pnet.Conn) bool {
		return connection.ID() == connections[0].ID()
	}) {
		t.Fatalf("reconnect did not establish a fresh subject connection: %v", reconnected)
	}
	if slices.Contains(subject.BitBookPeers(), client.ID()) {
		t.Errorf("confirmation survived the last-connection gap before the delayed callback")
	}

	releaseDisconnect()
	waitSignal(t, ctx, delivered, "delayed production disconnect delivery")
	if slices.Contains(subject.BitBookPeers(), client.ID()) {
		t.Errorf("confirmation survived the last-connection gap after the delayed callback")
	}

	completeDiscoveryHello(t, ctx, client, subject.ID())
	waitForBitBookPeer(t, ctx, subject, client.ID())
}

type delayedDiscoveryDisconnectNotifiee struct {
	lp2pnet.Notifiee
	connID    string
	entered   chan<- struct{}
	release   <-chan struct{}
	delivered chan<- struct{}
}

func (notifiee *delayedDiscoveryDisconnectNotifiee) Disconnected(network lp2pnet.Network, connection lp2pnet.Conn) {
	if connection.ID() != notifiee.connID {
		notifiee.Notifiee.Disconnected(network, connection)
		return
	}
	close(notifiee.entered)
	<-notifiee.release
	notifiee.Notifiee.Disconnected(network, connection)
	close(notifiee.delivered)
}

func TestNET001InboundHelloConcurrencyBound(t *testing.T) {
	for _, count := range []int{15, 16, 17} {
		t.Run(fmt.Sprintf("%d streams", count), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			node, err := New(ctx, Config{
				ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
				DHTMode:               dht.ModeServer,
				AllowPrivateAddresses: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer node.Close()
			client, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			if err := client.Connect(ctx, peer.AddrInfo{ID: node.ID(), Addrs: slices.Clone(node.Host.Addrs())}); err != nil {
				t.Fatal(err)
			}

			streams := make([]lp2pnet.Stream, 0, count)
			defer func() {
				for _, stream := range streams {
					_ = stream.Reset()
				}
			}()
			admitted := count
			if admitted > 16 {
				admitted = 16
			}
			for i := 0; i < admitted; i++ {
				stream, err := client.NewStream(ctx, node.ID(), DiscoveryProtocolCurrent)
				if err != nil {
					t.Fatalf("open admitted inbound stream %d: %v", i, err)
				}
				streams = append(streams, stream)
				if _, err := stream.Write([]byte{'B'}); err != nil {
					t.Fatalf("force handler %d into I/O: %v", i, err)
				}
				if err := node.waitForDiscoveryHandlers(ctx, i+1); err != nil {
					t.Fatalf("observe %d admitted handlers: %v", i+1, err)
				}
			}
			if count > 16 {
				excess, err := client.NewStream(ctx, node.ID(), DiscoveryProtocolCurrent)
				if err != nil {
					t.Fatalf("negotiate excess stream: %v", err)
				}
				defer excess.Reset()
				_, _ = excess.Write([]byte{'B'})
				_ = excess.SetReadDeadline(time.Now().Add(2 * time.Second))
				var one [1]byte
				if _, err := excess.Read(one[:]); err == nil {
					t.Fatal("17th inbound handler was queued instead of reset")
				} else if isTimeoutError(err) {
					t.Fatalf("17th stream reached its deadline instead of remote admission reset: %v", err)
				} else if !errors.Is(err, lp2pnet.ErrReset) {
					t.Fatalf("17th stream was closed without an admission reset: %v", err)
				}
			}

			first := streams[0]
			if _, err := io.WriteString(first, fixturePeerHello[1:]); err != nil {
				t.Fatalf("complete admitted hello: %v", err)
			}
			if err := first.CloseWrite(); err != nil {
				t.Fatalf("close admitted request: %v", err)
			}
			if err := readPeerHello(first); err != nil {
				t.Fatalf("admitted hello response: %v", err)
			}
			_ = first.Close()
			if err := node.waitForDiscoveryHandlers(ctx, admitted-1); err != nil {
				t.Fatalf("released slot was not observed: %v", err)
			}
			fresh, err := client.NewStream(ctx, node.ID(), DiscoveryProtocolCurrent)
			if err != nil {
				t.Fatalf("open stream after release: %v", err)
			}
			defer fresh.Reset()
			_ = fresh.SetDeadline(time.Now().Add(2 * time.Second))
			if _, err := io.WriteString(fresh, fixturePeerHello); err != nil {
				t.Fatal(err)
			}
			if err := fresh.CloseWrite(); err != nil {
				t.Fatal(err)
			}
			if err := readPeerHello(fresh); err != nil {
				t.Fatalf("valid exchange after slot release: %v", err)
			}
		})
	}
}

func TestNET001DiscoveryRetryAfterInvalidHello(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	seed := newUpstreamFixture(t, ctx)
	candidate := newUpstreamFixture(t, ctx)
	if err := candidate.host.Connect(ctx, seed.addrInfo()); err != nil {
		t.Fatal(err)
	}
	if err := candidate.dht.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	waitForUpstreamRoutingPeer(t, ctx, candidate.dht)
	router := routingdiscovery.NewRoutingDiscovery(candidate.dht)
	if _, err := router.Advertise(ctx, DiscoveryNamespace); err != nil {
		t.Fatal(err)
	}
	waitForUpstreamAdvertisement(t, ctx, routingdiscovery.NewRoutingDiscovery(seed.dht), candidate.host.ID())

	candidate.host.SetStreamHandler(DiscoveryProtocolCurrent, func(stream lp2pnet.Stream) {
		defer stream.Close()
		_ = stream.SetDeadline(time.Now().Add(5 * time.Second))
		if readPeerHello(stream) == nil {
			if _, err := io.WriteString(stream, "INVALID\n"); err != nil {
				return
			}
			if err := stream.CloseWrite(); err != nil {
				return
			}
		}
	})

	node, err := New(ctx, Config{
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		BootstrapPeers:        []peer.AddrInfo{seed.addrInfo()},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer node.Close()
	waitForTransportConnection(t, ctx, node, seed.host.ID())
	waitForRoutingPeer(t, ctx, node)
	if err := node.runDiscoveryRound(ctx); err != nil {
		t.Fatalf("invalid discovery round: %v", err)
	}
	if slices.Contains(node.BitBookPeers(), candidate.host.ID()) {
		t.Fatal("invalid responder was confirmed after its discovery round completed")
	}

	candidate.host.SetStreamHandler(DiscoveryProtocolCurrent, validFixtureHelloHandler)
	if err := node.runDiscoveryRound(ctx); err != nil {
		t.Fatalf("retry discovery round: %v", err)
	}
	waitForBitBookPeer(t, ctx, node, candidate.host.ID())
}

func TestNET001OutboundHelloRequiresValidation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	node, err := New(ctx, Config{
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer node.Close()
	controlled, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatal(err)
	}
	defer controlled.Close()
	if err := node.Host.Connect(ctx, peer.AddrInfo{ID: controlled.ID(), Addrs: slices.Clone(controlled.Addrs())}); err != nil {
		t.Fatal(err)
	}

	invalidWritten := make(chan struct{}, 1)
	controlled.SetStreamHandler(DiscoveryProtocolCurrent, func(stream lp2pnet.Stream) {
		defer stream.Close()
		_ = stream.SetDeadline(time.Now().Add(5 * time.Second))
		if readPeerHello(stream) != nil {
			_ = stream.Reset()
			return
		}
		if _, err := io.WriteString(stream, "INVALID\n"); err != nil {
			return
		}
		if err := stream.CloseWrite(); err != nil {
			return
		}
		invalidWritten <- struct{}{}
	})
	if err := node.probeBitBookPeer(ctx, controlled.ID()); err == nil {
		t.Fatal("invalid hello response was accepted")
	}
	select {
	case <-invalidWritten:
	case <-ctx.Done():
		t.Fatalf("controlled peer did not write its invalid response: %v", ctx.Err())
	}
	if slices.Contains(node.BitBookPeers(), controlled.ID()) {
		t.Fatal("peer was confirmed after completed invalid probe")
	}

	controlled.SetStreamHandler(DiscoveryProtocolCurrent, validFixtureHelloHandler)
	if err := node.probeBitBookPeer(ctx, controlled.ID()); err != nil {
		t.Fatalf("valid hello probe: %v", err)
	}
	if !slices.Contains(node.BitBookPeers(), controlled.ID()) {
		t.Fatal("same controlled peer was not confirmed after a valid completed probe")
	}
}

func TestNET001DiscoveryLoopDoesNotOverlapAndRetries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ticks := make(chan time.Time, 4)
	type invocation struct {
		call    int
		release chan struct{}
	}
	entered := make(chan invocation, 4)
	var calls atomic.Int32
	var active atomic.Int32
	var maximum atomic.Int32
	round := func(roundCtx context.Context) error {
		call := int(calls.Add(1))
		current := active.Add(1)
		defer active.Add(-1)
		for {
			old := maximum.Load()
			if current <= old || maximum.CompareAndSwap(old, current) {
				break
			}
		}
		release := make(chan struct{})
		select {
		case entered <- invocation{call: call, release: release}:
		case <-roundCtx.Done():
			return roundCtx.Err()
		}
		select {
		case <-release:
			return errors.New("controlled round failure")
		case <-roundCtx.Done():
			return roundCtx.Err()
		}
	}
	done := make(chan struct{})
	go func() {
		runDiscoveryLoop(ctx, ticks, round)
		close(done)
	}()
	var releases []chan struct{}
	defer func() {
		cancel()
		for _, release := range releases {
			if release == nil {
				continue
			}
			select {
			case <-release:
			default:
				close(release)
			}
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Errorf("discovery loop cleanup did not finish")
		}
	}()
	receive := func() invocation {
		select {
		case got := <-entered:
			return got
		case <-time.After(5 * time.Second):
			t.Fatal("discovery round did not start")
			return invocation{}
		}
	}
	first := receive()
	releases = append(releases, first.release)
	if first.call != 1 {
		t.Fatalf("initial round = %d", first.call)
	}
	for range 3 {
		ticks <- time.Now()
	}
	select {
	case invocation := <-entered:
		releases = append(releases, invocation.release)
		t.Fatalf("overlapping round %d started", invocation.call)
	case <-time.After(150 * time.Millisecond):
	}
	close(first.release)
	second := receive()
	releases = append(releases, second.release)
	if second.call != 2 {
		t.Fatalf("retry round = %d", second.call)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("discovery loop did not stop")
	}
	if maximum.Load() != 1 || active.Load() != 0 {
		t.Fatalf("round concurrency maximum=%d active=%d", maximum.Load(), active.Load())
	}
}

func TestNET001DiscoveryTimingAndIndependentRoundTasks(t *testing.T) {
	if discoveryRoundInterval != time.Minute || discoveryRoundTimeout != 30*time.Second || discoveryProbeTimeout != 5*time.Second {
		t.Fatalf("discovery timing = interval %s round %s probe %s", discoveryRoundInterval, discoveryRoundTimeout, discoveryProbeTimeout)
	}
	lastSuccess := time.Unix(10_000, 0)
	ttl := 2 * time.Hour
	if discoveryAdvertisementDue(lastSuccess.Add(ttl/2-time.Nanosecond), lastSuccess, ttl) {
		t.Fatal("advertisement renewed before half its successful TTL")
	}
	if !discoveryAdvertisementDue(lastSuccess.Add(ttl/2), lastSuccess, ttl) {
		t.Fatal("advertisement was not renewed at half its successful TTL")
	}
	if !discoveryAdvertisementDue(lastSuccess, time.Time{}, 0) {
		t.Fatal("initial or failed advertisement was not immediately due")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	advertiseErr := errors.New("controlled advertise failure")
	advertiseStarted := make(chan struct{}, 1)
	lookupStarted := make(chan struct{}, 1)
	releaseAdvertise := make(chan struct{})
	var releaseOnce sync.Once
	result := make(chan error, 1)
	tasksDone := make(chan struct{})
	defer func() {
		cancel()
		releaseOnce.Do(func() { close(releaseAdvertise) })
		select {
		case <-tasksDone:
		case <-time.After(5 * time.Second):
			t.Errorf("round task cleanup did not finish")
		}
	}()
	go func() {
		defer close(tasksDone)
		result <- runDiscoveryTasks(ctx,
			func(taskCtx context.Context) error {
				advertiseStarted <- struct{}{}
				select {
				case <-releaseAdvertise:
					return advertiseErr
				case <-taskCtx.Done():
					return taskCtx.Err()
				}
			},
			func(context.Context) error {
				lookupStarted <- struct{}{}
				return nil
			},
		)
	}()
	waitSignal(t, ctx, advertiseStarted, "advertisement task start")
	waitSignal(t, ctx, lookupStarted, "lookup task start while advertisement is blocked")
	releaseOnce.Do(func() { close(releaseAdvertise) })
	select {
	case err := <-result:
		if !errors.Is(err, advertiseErr) {
			t.Fatalf("round task error = %v, want advertise failure", err)
		}
	case <-ctx.Done():
		t.Fatalf("independent round tasks did not join: %v", ctx.Err())
	}
}

func TestNET001DiscoveryCandidateAndProbeBounds(t *testing.T) {
	self := fixturePeerID(t)
	invalidID := peer.ID("not-a-valid-multihash")
	valid := fixturePeerIDs(t, 2)
	small, _ := selectDiscoveryCandidates(self,
		[]peer.ID{"", invalidID, self, valid[0], valid[0]},
		[]peer.ID{"", invalidID, self, valid[0], valid[1], valid[1]}, 0)
	if !slices.Equal(small, valid) {
		t.Fatalf("invalid/self/duplicate filtering before capacity = %v, want %v", small, valid)
	}

	for _, count := range []int{31, 32, 33} {
		providers := fixturePeerIDs(t, count)
		connected := fixturePeerIDs(t, count)
		got, _ := selectDiscoveryCandidates(self, providers, connected, 0)
		wantPerSource := count
		if wantPerSource > 32 {
			wantPerSource = 32
		}
		if len(got) != 2*wantPerSource {
			t.Fatalf("%d/%d source candidates produced %d, want %d", count, count, len(got), 2*wantPerSource)
		}
		for _, id := range providers[:wantPerSource] {
			if !slices.Contains(got, id) {
				t.Fatalf("provider allowance %d omitted %s", count, id)
			}
		}
		connectedCount := 0
		for _, id := range got {
			if slices.Contains(connected, id) {
				connectedCount++
			}
		}
		if connectedCount != wantPerSource {
			t.Fatalf("connected allowance %d selected %d, want %d", count, connectedCount, wantPerSource)
		}
	}

	providers := fixturePeerIDs(t, 33)
	connected := fixturePeerIDs(t, 40)
	first, rotation := selectDiscoveryCandidates(self, providers, connected, 0)
	second, nextRotation := selectDiscoveryCandidates(self, providers, connected, rotation)
	if rotation == nextRotation || slices.Equal(first[32:], second[32:]) {
		t.Fatal("connected candidate selection did not rotate")
	}
	if len(uniquePeerIDs(first)) != len(first) {
		t.Fatalf("combined candidates contain duplicates: %v", first)
	}

	var active atomic.Int32
	var maximum atomic.Int32
	var attempted atomic.Int32
	entered := make(chan struct{}, len(first))
	release := make(chan struct{})
	probeCtx, cancelProbes := context.WithCancel(context.Background())
	var releaseOnce sync.Once
	probesFinished := make(chan struct{})
	defer func() {
		cancelProbes()
		releaseOnce.Do(func() { close(release) })
		select {
		case <-probesFinished:
		case <-time.After(5 * time.Second):
			t.Errorf("probe cleanup did not finish")
		}
	}()
	probeDone := make(chan error, 1)
	go func() {
		defer close(probesFinished)
		probeDone <- runDiscoveryProbes(probeCtx, first, func(probeCtx context.Context, _ peer.ID) error {
			current := active.Add(1)
			defer active.Add(-1)
			attempted.Add(1)
			for {
				old := maximum.Load()
				if current <= old || maximum.CompareAndSwap(old, current) {
					break
				}
			}
			entered <- struct{}{}
			select {
			case <-release:
				return errors.New("unavailable fixture peer")
			case <-probeCtx.Done():
				return probeCtx.Err()
			}
		})
	}()
	for range 4 {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("four probes did not start")
		}
	}
	select {
	case <-entered:
		t.Fatal("more than four probes ran concurrently")
	case <-time.After(150 * time.Millisecond):
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case err := <-probeDone:
		if err != nil {
			t.Fatalf("bounded probe runner: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("bounded probes did not join")
	}
	if maximum.Load() > 4 || attempted.Load() != int32(len(first)) {
		t.Fatalf("probe bounds maximum=%d attempted=%d want<=4/%d", maximum.Load(), attempted.Load(), len(first))
	}

	activeCtx, cancelActive := context.WithCancel(context.Background())
	defer cancelActive()
	activeStarted := make(chan struct{}, 1)
	activeDone := make(chan error, 1)
	activeFinished := make(chan struct{})
	defer func() {
		cancelActive()
		select {
		case <-activeFinished:
		case <-time.After(5 * time.Second):
			t.Errorf("active probe cleanup did not finish")
		}
	}()
	go func() {
		defer close(activeFinished)
		activeDone <- runDiscoveryProbes(activeCtx, first[:1], func(ioCtx context.Context, _ peer.ID) error {
			activeStarted <- struct{}{}
			<-ioCtx.Done()
			return ioCtx.Err()
		})
	}()
	select {
	case <-activeStarted:
	case <-time.After(5 * time.Second):
		cancelActive()
		t.Fatal("active outbound probe did not start")
	}
	cancelActive()
	select {
	case err := <-activeDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("active probe cancellation = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("active outbound I/O did not stop after cancellation")
	}
}

func TestNET001ConfirmedPeerCapacityOwnershipAndOrder(t *testing.T) {
	self := fixturePeerID(t)
	connected := func(peer.ID) bool { return true }
	for _, count := range []int{127, 128, 129} {
		confirmed := newConfirmedPeers(self)
		ids := fixturePeerIDs(t, count)
		base := time.Unix(1_000, 0)
		confirmed.confirm("", base)
		confirmed.confirm(peer.ID("not-a-valid-multihash"), base)
		confirmed.confirm(self, base)
		confirmed.confirm(ids[0], base.Add(time.Second/2))
		for i, id := range ids {
			confirmed.confirm(id, base.Add(time.Duration(i+1)*time.Second))
		}
		confirmed.confirm(ids[count-1], base.Add(time.Hour))
		got := confirmed.snapshot(connected)
		want := count
		if want > 128 {
			want = 128
		}
		if len(got) != want {
			t.Fatalf("%d confirmations produced %d entries, want %d", count, len(got), want)
		}
		if slices.Contains(got, "") || slices.Contains(got, peer.ID("not-a-valid-multihash")) || slices.Contains(got, self) {
			t.Fatalf("invalid or self confirmation survived before capacity handling: %v", got)
		}
		if len(uniquePeerIDs(got)) != len(got) {
			t.Fatalf("duplicate confirmation survived: %v", got)
		}
		if count == 129 && slices.Contains(got, ids[0]) {
			t.Fatal("least recently confirmed peer was not evicted at 129")
		}
		if !slices.IsSorted(got) {
			t.Fatal("confirmed snapshot is not sorted")
		}
		wantFirst := got[0]
		got[0] = self
		again := confirmed.snapshot(connected)
		if len(again) != want || again[0] != wantFirst {
			t.Fatal("caller mutation changed confirmed state")
		}
		forgotten := ids[count-1]
		confirmed.forget(forgotten)
		if slices.Contains(confirmed.snapshot(connected), forgotten) {
			t.Fatal("forgotten peer remained confirmed")
		}
		kept := ids[count-2]
		if filtered := confirmed.snapshot(func(id peer.ID) bool { return id == kept }); len(filtered) != 1 || filtered[0] != kept {
			t.Fatalf("connected filtering = %v", filtered)
		}
	}
}

func validFixtureHelloHandler(stream lp2pnet.Stream) {
	defer stream.Close()
	_ = stream.SetDeadline(time.Now().Add(5 * time.Second))
	if readPeerHello(stream) != nil {
		_ = stream.Reset()
		return
	}
	if _, err := io.WriteString(stream, fixturePeerHello); err != nil {
		_ = stream.Reset()
		return
	}
	_ = stream.CloseWrite()
}

func completeDiscoveryHello(t testing.TB, ctx context.Context, client host.Host, remote peer.ID) {
	t.Helper()
	stream, err := client.NewStream(ctx, remote, DiscoveryProtocolCurrent)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if err := stream.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(stream, fixturePeerHello); err != nil {
		t.Fatal(err)
	}
	if err := stream.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	if err := readPeerHello(stream); err != nil {
		t.Fatalf("valid discovery response: %v", err)
	}
}

func waitForBitBookPeer(t testing.TB, ctx context.Context, node *Node, id peer.ID) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if slices.Contains(node.BitBookPeers(), id) {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for BitBook peer %s: %v (got %v)", id, ctx.Err(), node.BitBookPeers())
		case <-ticker.C:
		}
	}
}

func waitForNoBitBookPeer(t testing.TB, ctx context.Context, node *Node, id peer.ID) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if !slices.Contains(node.BitBookPeers(), id) {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting to forget BitBook peer %s: %v", id, ctx.Err())
		case <-ticker.C:
		}
	}
}

func waitForDiscoveryRestartRejection(t testing.TB, ctx context.Context, node *Node) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := node.StartDiscovery(ctx); err != nil {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("discovery restarted after its parent stopped: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

func fixturePeerID(t testing.TB) peer.ID {
	t.Helper()
	key, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id, err := peer.IDFromPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func fixturePeerIDs(t testing.TB, count int) []peer.ID {
	t.Helper()
	ids := make([]peer.ID, count)
	for i := range ids {
		ids[i] = fixturePeerID(t)
	}
	return ids
}

func uniquePeerIDs(ids []peer.ID) map[peer.ID]struct{} {
	unique := make(map[peer.ID]struct{}, len(ids))
	for _, id := range ids {
		unique[id] = struct{}{}
	}
	return unique
}

func isTimeoutError(err error) bool {
	var timeout net.Error
	return errors.As(err, &timeout) && timeout.Timeout()
}

func waitSignal(t testing.TB, ctx context.Context, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-ctx.Done():
		t.Fatalf("waiting for %s: %v", description, ctx.Err())
	}
}

func waitForUpstreamAdvertisement(t testing.TB, ctx context.Context, discovery *routingdiscovery.RoutingDiscovery, want peer.ID) {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		findCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		found, err := discovery.FindPeers(findCtx, DiscoveryNamespace)
		if err == nil {
			for info := range found {
				if info.ID == want {
					cancel()
					return
				}
			}
		}
		cancel()
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for advertisement from %s: %v", want, ctx.Err())
		case <-ticker.C:
		}
	}
}

func waitForUpstreamRoutingPeer(t testing.TB, ctx context.Context, kad *dht.IpfsDHT) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for kad.RoutingTable().Size() == 0 {
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for independent DHT routing peer: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}
