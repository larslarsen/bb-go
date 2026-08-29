package social

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/larslarsen/bb-go/modern/network"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TestLocalSocialStateCommitsWithoutNetwork(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	n := newTestNode(t, ctx)
	defer n.Close()
	store, err := NewStore(n)
	if err != nil {
		t.Fatal(err)
	}

	profile, err := store.SetProfile(ctx, json.RawMessage(`{"name":"Ada","about":"distributed social"}`))
	if err != nil {
		t.Fatal(err)
	}
	var decodedProfile map[string]any
	if err := json.Unmarshal(profile, &decodedProfile); err != nil {
		t.Fatal(err)
	}
	if decodedProfile["peerID"] != n.ID().String() {
		t.Fatalf("profile peerID is %v, want %s", decodedProfile["peerID"], n.ID())
	}

	post, err := store.AddPost(ctx, json.RawMessage(`{"status":"Hello BitBook"}`))
	if err != nil {
		t.Fatal(err)
	}
	if post.CID == "" {
		t.Fatal("post CID is empty")
	}
	if err := VerifyPost(n.ID(), post); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := n.Datastore.Get(ctx, rootKey); err != nil {
		t.Fatalf("local root was not persisted: %v", err)
	}
}

func TestTwoPeersPublishAndFetchSocialState(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	author := newTestNode(t, ctx)
	defer author.Close()
	reader := newTestNode(t, ctx)
	defer reader.Close()
	connectTestNodes(t, ctx, author, reader)

	authorStore, err := NewStore(author)
	if err != nil {
		t.Fatal(err)
	}
	readerStore, err := NewStore(reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authorStore.SetProfile(ctx, json.RawMessage(`{"name":"Grace","handle":"grace"}`)); err != nil {
		t.Fatal(err)
	}
	if err := authorStore.Follow(ctx, reader.ID()); err != nil {
		t.Fatal(err)
	}
	created, err := authorStore.AddPost(ctx, json.RawMessage(`{"status":"A signed social post","tags":["p2p"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authorStore.Publish(ctx); err != nil {
		t.Fatal(err)
	}

	state, err := readerStore.Fetch(ctx, author.ID())
	if err != nil {
		t.Fatal(err)
	}
	var profile map[string]any
	if err := json.Unmarshal(state.Profile, &profile); err != nil {
		t.Fatal(err)
	}
	if profile["name"] != "Grace" || profile["peerID"] != author.ID().String() {
		t.Fatalf("unexpected remote profile: %s", state.Profile)
	}
	if len(state.Posts) != 1 || state.Posts[0].CID != created.CID {
		t.Fatalf("unexpected remote posts: %+v", state.Posts)
	}
	if len(state.Following) != 1 || state.Following[0] != reader.ID().String() {
		t.Fatalf("unexpected following list: %v", state.Following)
	}
}

func TestVerifyPostRejectsTampering(t *testing.T) {
	ctx := context.Background()
	n := newTestNode(t, ctx)
	defer n.Close()
	store, err := NewStore(n)
	if err != nil {
		t.Fatal(err)
	}
	post, err := store.AddPost(ctx, json.RawMessage(`{"status":"original"}`))
	if err != nil {
		t.Fatal(err)
	}
	post.Post = json.RawMessage(`{"status":"tampered"}`)
	if err := VerifyPost(n.ID(), post); err == nil {
		t.Fatal("tampered post passed signature verification")
	}
}

func newTestNode(t *testing.T, ctx context.Context) *network.Node {
	t.Helper()
	n, err := network.New(ctx, network.Config{
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func connectTestNodes(t *testing.T, ctx context.Context, left, right *network.Node) {
	t.Helper()
	if err := left.Connect(ctx, peer.AddrInfo{ID: right.ID(), Addrs: right.Host.Addrs()}); err != nil {
		t.Fatal(err)
	}
	if err := right.Connect(ctx, peer.AddrInfo{ID: left.ID(), Addrs: left.Host.Addrs()}); err != nil {
		t.Fatal(err)
	}
	if err := left.DHT.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	if err := right.DHT.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	waitForRoutingPeer(t, ctx, left)
	waitForRoutingPeer(t, ctx, right)
}

func waitForRoutingPeer(t *testing.T, ctx context.Context, n *network.Node) {
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
