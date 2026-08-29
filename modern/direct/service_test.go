package direct

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"testing"
	"time"

	"github.com/larslarsen/bb-go/modern/network"
	"github.com/larslarsen/bb-go/modern/social"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TestTwoPeersFollowChatAndSendReadReceipt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	aNode, _, aDirect := newTestService(t, ctx)
	defer aNode.Close()
	defer aDirect.Close()
	bNode, bSocial, bDirect := newTestService(t, ctx)
	defer bNode.Close()
	defer bDirect.Close()
	connectNodes(t, ctx, aNode, bNode)

	events, unsubscribe := bDirect.Subscribe(8)
	defer unsubscribe()
	delivery, err := aDirect.SendFollow(ctx, bNode.ID(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !delivery.Delivered || delivery.Queued {
		t.Fatalf("unexpected follow delivery: %+v", delivery)
	}
	followers, err := bSocial.LocalFollowers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(followers) != 1 || followers[0] != aNode.ID().String() {
		t.Fatalf("unexpected followers: %v", followers)
	}
	assertEventKey(t, events, "notification")

	outgoing, delivery, err := aDirect.SendChat(ctx, bNode.ID(), "", "hello directly")
	if err != nil {
		t.Fatal(err)
	}
	if !delivery.Delivered {
		t.Fatalf("chat was not delivered: %+v", delivery)
	}
	incoming, err := bDirect.Messages(ctx, aNode.ID().String(), "", "", -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(incoming) != 1 || incoming[0].Message != "hello directly" || incoming[0].Outgoing {
		t.Fatalf("unexpected incoming message: %+v", incoming)
	}
	if incoming[0].MessageID != outgoing.MessageID {
		t.Fatalf("message IDs differ: %s != %s", incoming[0].MessageID, outgoing.MessageID)
	}
	assertEventKey(t, events, "message")

	readDelivery, err := bDirect.MarkAsRead(ctx, aNode.ID(), "")
	if err != nil {
		t.Fatal(err)
	}
	if !readDelivery.Delivered {
		t.Fatalf("read receipt was not delivered: %+v", readDelivery)
	}
	aMessages, err := aDirect.Messages(ctx, bNode.ID().String(), "", "", -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(aMessages) != 1 || !aMessages[0].Read {
		t.Fatalf("outgoing message was not marked read: %+v", aMessages)
	}

	if _, _, err := aDirect.SendChat(ctx, bNode.ID(), "", ""); err != nil {
		t.Fatal(err)
	}
	assertEventKey(t, events, "messageTyping")

	if _, err := aDirect.SendFollow(ctx, bNode.ID(), false); err != nil {
		t.Fatal(err)
	}
	followers, err = bSocial.LocalFollowers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(followers) != 0 {
		t.Fatalf("follower remained after unfollow: %v", followers)
	}
}

func TestOfflineChatIsPersistedAndQueued(t *testing.T) {
	ctx := context.Background()
	node, _, service := newTestService(t, ctx)
	defer node.Close()
	defer service.Close()
	key, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	recipient, err := peer.IDFromPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	sendCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	message, delivery, err := service.SendChat(sendCtx, recipient, "", "queued message")
	if err != nil {
		t.Fatal(err)
	}
	if !delivery.Queued || delivery.Delivered {
		t.Fatalf("unexpected offline delivery: %+v", delivery)
	}
	pending, err := service.Pending(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if pending != 1 {
		t.Fatalf("pending count = %d, want 1", pending)
	}
	messages, err := service.Messages(context.Background(), recipient.String(), "", "", -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].MessageID != message.MessageID {
		t.Fatalf("outgoing message was not persisted: %+v", messages)
	}
}

func TestEnvelopeSignatureRejectsTampering(t *testing.T) {
	ctx := context.Background()
	senderNode, _, sender := newTestService(t, ctx)
	defer senderNode.Close()
	defer sender.Close()
	receiverNode, _, receiver := newTestService(t, ctx)
	defer receiverNode.Close()
	defer receiver.Close()
	envelope, err := sender.newEnvelope(KindChat, receiverNode.ID(), "", "original", "")
	if err != nil {
		t.Fatal(err)
	}
	envelope.Message = "tampered"
	if err := receiver.verify(senderNode.ID(), envelope); err == nil {
		t.Fatal("tampered direct envelope passed signature verification")
	}
}

func newTestService(t *testing.T, ctx context.Context) (*network.Node, *social.Store, *Service) {
	t.Helper()
	node, err := network.New(ctx, network.Config{
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	socialStore, err := social.NewStore(node)
	if err != nil {
		node.Close()
		t.Fatal(err)
	}
	service, err := NewService(node, socialStore)
	if err != nil {
		node.Close()
		t.Fatal(err)
	}
	return node, socialStore, service
}

func connectNodes(t *testing.T, ctx context.Context, left, right *network.Node) {
	t.Helper()
	if err := left.Connect(ctx, peer.AddrInfo{ID: right.ID(), Addrs: right.Host.Addrs()}); err != nil {
		t.Fatal(err)
	}
	if err := right.Connect(ctx, peer.AddrInfo{ID: left.ID(), Addrs: left.Host.Addrs()}); err != nil {
		t.Fatal(err)
	}
}

func assertEventKey(t *testing.T, events <-chan []byte, key string) {
	t.Helper()
	select {
	case raw := <-events:
		var event map[string]any
		if err := json.Unmarshal(raw, &event); err != nil {
			t.Fatal(err)
		}
		if _, ok := event[key]; !ok {
			t.Fatalf("event %s does not contain %q", raw, key)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %q event", key)
	}
}
