package network

import (
	"context"
	"io"
	"net"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	lp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

func TestNET001LibraryHasNoImplicitBootstrapPeers(t *testing.T) {
	if bootstrapDialTimeout != 10*time.Second {
		t.Fatalf("bootstrap dial timeout = %s, want 10s", bootstrapDialTimeout)
	}
	for _, tc := range []struct {
		name  string
		peers []peer.AddrInfo
	}{
		{name: "nil"},
		{name: "empty", peers: []peer.AddrInfo{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			trap := newStalledBootstrapPeer(t)
			restoreDefaultBootstrapPeers(t, []ma.Multiaddr{trap.p2pAddr})

			node, err := New(ctx, Config{
				ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
				BootstrapPeers: tc.peers,
				DHTMode:        dht.ModeServer,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer node.Close()
			if err := node.DHT.Bootstrap(ctx); err != nil {
				t.Fatalf("completed empty bootstrap configuration: %v", err)
			}
			if _, err := node.Put(ctx, []byte("offline bootstrap control")); err != nil {
				t.Fatalf("offline node unusable: %v", err)
			}
			select {
			case conn := <-trap.accepted:
				_ = conn.Close()
				t.Fatal("nil/empty library config selected the upstream default bootstrap peer")
			case <-time.After(250 * time.Millisecond):
			}
		})
	}

	t.Run("explicit positive control", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		trap := newStalledBootstrapPeer(t)
		node := newNodeWithin(t, ctx, Config{
			ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
			BootstrapPeers:        []peer.AddrInfo{trap.info},
			DHTMode:               dht.ModeServer,
			AllowPrivateAddresses: true,
		}, 2*time.Second)
		defer node.Close()
		conn := receiveBootstrapConnection(t, ctx, trap.accepted)
		_ = conn.Close()
	})
}

func TestNET001BootstrapStalledSeedDoesNotBlockLocalUse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	stalled := newStalledBootstrapPeer(t)

	stalledAddrs := []ma.Multiaddr{stalled.addr}
	input := []peer.AddrInfo{{ID: stalled.info.ID, Addrs: stalledAddrs}}
	node := newNodeWithin(t, ctx, Config{
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		BootstrapPeers:        input,
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	}, 2*time.Second)
	defer node.Close()

	// New returning is the configuration-copy boundary. Mutate both caller-owned
	// outer and inner backing arrays after it; background bootstrap must retain
	// the original ID and address without racing these writes.
	replacementAddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/1")
	if err != nil {
		t.Fatal(err)
	}
	input[0] = peer.AddrInfo{ID: fixturePeerID(t), Addrs: []ma.Multiaddr{replacementAddr}}
	stalledAddrs[0] = replacementAddr

	stalledConn := receiveBootstrapConnection(t, ctx, stalled.accepted)
	stalledDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, stalledConn)
		close(stalledDone)
	}()
	defer func() {
		_ = stalledConn.Close()
		select {
		case <-stalledDone:
		case <-time.After(time.Second):
			t.Errorf("stalled bootstrap recorder did not stop")
		}
	}()

	if _, err := node.Put(ctx, []byte("usable while a bootstrap dial is pending")); err != nil {
		t.Fatalf("local service while stalled seed is pending: %v", err)
	}
	select {
	case <-stalledDone:
		t.Fatal("stalled bootstrap attempt ended before local work completed")
	default:
	}
}

func TestNET001BootstrapFailureDoesNotBlockHealthySeed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	healthy := newUpstreamFixture(t, ctx)
	failing := newStalledBootstrapPeer(t)
	node := newNodeWithin(t, ctx, Config{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		BootstrapPeers: []peer.AddrInfo{
			failing.info,
			healthy.addrInfo(),
		},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	}, 2*time.Second)
	defer node.Close()

	failingConn := receiveBootstrapConnection(t, ctx, failing.accepted)
	if err := failingConn.Close(); err != nil {
		t.Fatalf("release controlled bootstrap failure: %v", err)
	}
	waitForTransportConnection(t, ctx, node, healthy.host.ID())
}

type stalledBootstrapPeer struct {
	listener net.Listener
	addr     ma.Multiaddr
	p2pAddr  ma.Multiaddr
	info     peer.AddrInfo
	accepted chan net.Conn
	mu       sync.Mutex
	conn     net.Conn
	done     chan struct{}
}

func newStalledBootstrapPeer(t testing.TB) *stalledBootstrapPeer {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/" + strconv.Itoa(port))
	if err != nil {
		_ = listener.Close()
		t.Fatal(err)
	}
	id := fixturePeerID(t)
	peerPart, err := ma.NewMultiaddr("/p2p/" + id.String())
	if err != nil {
		_ = listener.Close()
		t.Fatal(err)
	}
	fixture := &stalledBootstrapPeer{
		listener: listener,
		addr:     addr,
		p2pAddr:  addr.Encapsulate(peerPart),
		info:     peer.AddrInfo{ID: id, Addrs: []ma.Multiaddr{addr}},
		accepted: make(chan net.Conn, 1),
		done:     make(chan struct{}),
	}
	go func() {
		defer close(fixture.done)
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			fixture.mu.Lock()
			fixture.conn = conn
			fixture.mu.Unlock()
			fixture.accepted <- conn
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-fixture.done
		fixture.mu.Lock()
		if fixture.conn != nil {
			_ = fixture.conn.Close()
		}
		fixture.mu.Unlock()
	})
	return fixture
}

func restoreDefaultBootstrapPeers(t testing.TB, replacement []ma.Multiaddr) {
	t.Helper()
	original := slices.Clone(dht.DefaultBootstrapPeers)
	dht.DefaultBootstrapPeers = slices.Clone(replacement)
	t.Cleanup(func() { dht.DefaultBootstrapPeers = original })
}

func newNodeWithin(t testing.TB, parent context.Context, cfg Config, limit time.Duration) *Node {
	t.Helper()
	ctx, cancel := context.WithCancel(parent)
	type result struct {
		node *Node
		err  error
	}
	resultCh := make(chan result, 1)
	go func() {
		node, err := New(ctx, cfg)
		resultCh <- result{node: node, err: err}
	}()
	timer := time.NewTimer(limit)
	defer timer.Stop()
	select {
	case got := <-resultCh:
		if got.err != nil {
			cancel()
			t.Fatalf("construct node: %v", got.err)
		}
		t.Cleanup(cancel)
		return got.node
	case <-timer.C:
		cancel()
		got := <-resultCh
		if got.node != nil {
			_ = got.node.Close()
		}
		t.Fatalf("New did not return within %s while a bootstrap dial was stalled", limit)
		return nil
	}
}

func receiveBootstrapConnection(t testing.TB, ctx context.Context, accepted <-chan net.Conn) net.Conn {
	t.Helper()
	select {
	case conn := <-accepted:
		return conn
	case <-ctx.Done():
		t.Fatalf("controlled bootstrap dial was not observed: %v", ctx.Err())
		return nil
	}
}

func waitForTransportConnection(t testing.TB, ctx context.Context, node *Node, id peer.ID) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if node.Host.Network().Connectedness(id) == lp2pnet.Connected {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for bootstrap connection to %s: %v", id, ctx.Err())
		case <-ticker.C:
		}
	}
}
