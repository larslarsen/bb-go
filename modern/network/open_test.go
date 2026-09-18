package network

import (
	"bytes"
	"context"
	"testing"
	"time"

	blocks "github.com/ipfs/go-block-format"
	datastore "github.com/ipfs/go-datastore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
)

func TestOpenPersistsPeerIdentity(t *testing.T) {
	dataDir := t.TempDir()
	config := Config{ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"}}

	first, err := Open(context.Background(), dataDir, config)
	if err != nil {
		t.Fatal(err)
	}
	firstID := first.ID()
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := Open(context.Background(), dataDir, config)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if second.ID() != firstID {
		t.Fatalf("peer identity changed: got %s want %s", second.ID(), firstID)
	}
}

func TestNET001ReopenPreservesPublicContentAndKeepsPrivateDatastorePrivate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	dataDir := t.TempDir()
	config := Config{
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	}
	first, err := Open(ctx, dataDir, config)
	if err != nil {
		t.Fatal(err)
	}
	firstID := first.ID()
	publicBytes := []byte("NET-001 durable public block")
	publicCID, err := first.Put(ctx, publicBytes)
	if err != nil {
		t.Fatal(err)
	}
	privateBytes := []byte("synthetic private direct-message record")
	privateCID := blocks.NewBlock(privateBytes).Cid()
	if err := first.Datastore.Put(ctx, datastore.NewKey("/bitbook/private/net001"), privateBytes); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := Open(ctx, dataDir, config)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if second.ID() != firstID {
		t.Fatalf("peer identity changed: got %s want %s", second.ID(), firstID)
	}
	local, err := second.Get(ctx, publicCID)
	if err != nil || !bytes.Equal(local, publicBytes) {
		t.Fatalf("reopened public block = %q, %v", local, err)
	}

	upstream := newUpstreamFixture(t, ctx)
	connectUpstreamAndNode(t, ctx, upstream, second.Node)
	publicBlock, err := upstream.bitswap.GetBlock(ctx, publicCID)
	if err != nil {
		t.Fatalf("upstream public retrieval: %v", err)
	}
	if !bytes.Equal(publicBlock.RawData(), publicBytes) {
		t.Fatalf("upstream public retrieval = %q", publicBlock.RawData())
	}
	privateCtx, privateCancel := context.WithTimeout(ctx, 2*time.Second)
	defer privateCancel()
	if block, err := upstream.bitswap.GetBlock(privateCtx, privateCID); err == nil {
		t.Fatalf("private datastore bytes were retrievable as public block %s: %q", privateCID, block.RawData())
	}
}
