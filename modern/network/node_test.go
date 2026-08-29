package network

import (
	"bytes"
	"context"
	"slices"
	"testing"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TestBitBookProtocolsAreIsolated(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	n, err := New(ctx, Config{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:     dht.ModeServer,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()

	protocols := n.Protocols()
	if !slices.Contains(protocols, BitswapProtocolCurrent) {
		t.Fatalf("BitBook Bitswap protocol missing; protocols: %v", protocols)
	}
	if !slices.Contains(protocols, DHTProtocolCurrent) {
		t.Fatalf("BitBook DHT protocol missing; protocols: %v", protocols)
	}
	for _, id := range protocols {
		if id == "/ipfs/bitswap/1.2.0" || id == "/ipfs/kad/1.0.0" {
			t.Fatalf("public IPFS protocol unexpectedly registered: %s", id)
		}
	}
}

func TestNodeCloseIsIdempotent(t *testing.T) {
	n, err := New(context.Background(), Config{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := n.Close(); err != nil {
		t.Fatal(err)
	}
	if err := n.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTwoNodesTransferBlockOverBitBookBitswap(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	config := Config{
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	}
	server, err := New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	client, err := New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if err := client.Connect(ctx, peer.AddrInfo{ID: server.ID()}); err == nil {
		t.Fatal("connect unexpectedly succeeded without server addresses")
	}
	serverInfo := server.Host.Peerstore().PeerInfo(server.ID())
	serverInfo.Addrs = server.Host.Addrs()
	if err := client.Connect(ctx, serverInfo); err != nil {
		t.Fatalf("connect: %v", err)
	}

	want := []byte("BitBook modern network block")
	id, err := server.Put(ctx, want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("wrong block: got %q want %q", got, want)
	}
}

func TestTwoNodesPublishAndResolveBitBookRoot(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config := Config{
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	}
	author, err := New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer author.Close()
	reader, err := New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	authorInfo := peer.AddrInfo{ID: author.ID(), Addrs: author.Host.Addrs()}
	readerInfo := peer.AddrInfo{ID: reader.ID(), Addrs: reader.Host.Addrs()}
	if err := reader.Connect(ctx, authorInfo); err != nil {
		t.Fatal(err)
	}
	if err := author.Connect(ctx, readerInfo); err != nil {
		t.Fatal(err)
	}
	if err := author.DHT.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	if err := reader.DHT.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	waitForRoutingPeer(t, ctx, author)
	waitForRoutingPeer(t, ctx, reader)

	want := []byte("signed BitBook social root")
	root, err := author.Put(ctx, want)
	if err != nil {
		t.Fatal(err)
	}
	if err := author.PublishRoot(ctx, root); err != nil {
		t.Fatal(err)
	}
	providers := reader.DHT.FindProvidersAsync(ctx, root, 1)
	provider, ok := <-providers
	if !ok {
		t.Fatal("published root has no DHT provider")
	}
	if provider.ID != author.ID() {
		t.Fatalf("root provider is %s, want author %s", provider.ID, author.ID())
	}
	resolved, err := reader.ResolveRoot(ctx, author.ID())
	if err != nil {
		t.Fatal(err)
	}
	if resolved != root {
		t.Fatalf("resolved %s, want %s", resolved, root)
	}
	got, err := reader.Get(ctx, resolved)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("wrong resolved content: got %q want %q", got, want)
	}
}

func waitForRoutingPeer(t *testing.T, ctx context.Context, n *Node) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for n.DHT.RoutingTable().Size() == 0 {
		select {
		case <-ctx.Done():
			t.Fatalf("DHT routing table remained empty: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}
