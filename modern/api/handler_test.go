package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	datastore "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	"github.com/larslarsen/bb-go/modern/direct"
	"github.com/larslarsen/bb-go/modern/network"
	"github.com/larslarsen/bb-go/modern/social"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	lp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	routingdiscovery "github.com/libp2p/go-libp2p/p2p/discovery/routing"
)

func TestSocialAPIWorksOffline(t *testing.T) {
	handler, node := newTestHandler(t)
	defer node.Close()

	profileResponse := request(t, handler, http.MethodPost, "/ob/profile", `{"name":"Ada","about":"p2p"}`)
	if profileResponse.Code != http.StatusOK {
		t.Fatalf("setting profile: %d %s", profileResponse.Code, profileResponse.Body)
	}
	if profileResponse.Header().Get("X-BitBook-Published") != "false" {
		t.Fatalf("unexpected publication status: %q", profileResponse.Header().Get("X-BitBook-Published"))
	}
	var profile map[string]any
	decodeResponse(t, profileResponse, &profile)
	if profile["name"] != "Ada" || profile["peerID"] != node.ID().String() {
		t.Fatalf("unexpected profile: %v", profile)
	}
	if _, exists := profile["publication"]; exists {
		t.Fatal("publication metadata leaked into the compatibility profile")
	}
	selfProfile := request(t, handler, http.MethodGet, "/ob/profile/"+node.ID().String(), "")
	var selfProfileValue map[string]any
	decodeResponse(t, selfProfile, &selfProfileValue)
	if selfProfileValue["peerID"] != node.ID().String() {
		t.Fatalf("self profile through explicit peer endpoint: %v", selfProfileValue)
	}

	patchResponse := request(t, handler, http.MethodPatch, "/ob/profile", `{"about":"distributed social","name":null}`)
	if patchResponse.Code != http.StatusOK {
		t.Fatalf("patching profile: %d %s", patchResponse.Code, patchResponse.Body)
	}
	getProfile := request(t, handler, http.MethodGet, "/ob/profile", "")
	profile = nil
	decodeResponse(t, getProfile, &profile)
	if profile["about"] != "distributed social" {
		t.Fatalf("patch was not retained: %v", profile)
	}
	if _, exists := profile["name"]; exists {
		t.Fatalf("null merge-patch did not delete name: %v", profile)
	}

	postResponse := request(t, handler, http.MethodPost, "/ob/post", `{"status":"Hello BitBook","longForm":"full body","tags":["p2p"]}`)
	if postResponse.Code != http.StatusOK {
		t.Fatalf("adding post: %d %s", postResponse.Code, postResponse.Body)
	}
	var created map[string]any
	decodeResponse(t, postResponse, &created)
	if created["slug"] != "hello-bitbook" || created["hash"] == "" {
		t.Fatalf("unexpected post response: %v", created)
	}

	postsResponse := request(t, handler, http.MethodGet, "/ob/posts", "")
	var posts []map[string]any
	decodeResponse(t, postsResponse, &posts)
	if len(posts) != 1 || posts[0]["status"] != "Hello BitBook" {
		t.Fatalf("unexpected post index: %v", posts)
	}
	if _, exists := posts[0]["longForm"]; exists {
		t.Fatalf("post index contains long form body: %v", posts[0])
	}

	postDetail := request(t, handler, http.MethodGet, "/ob/post/hello-bitbook", "")
	var signed social.SignedPost
	decodeResponse(t, postDetail, &signed)
	if err := social.VerifyPost(node.ID(), signed); err != nil {
		t.Fatalf("API returned an invalid post: %v", err)
	}

	deleteResponse := request(t, handler, http.MethodDelete, "/ob/post/hello-bitbook", "")
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("deleting post: %d %s", deleteResponse.Code, deleteResponse.Body)
	}
	postsResponse = request(t, handler, http.MethodGet, "/ob/posts", "")
	decodeResponse(t, postsResponse, &posts)
	if len(posts) != 0 {
		t.Fatalf("post remained after deletion: %v", posts)
	}
}

func TestFollowAndSocialOnlyPolicy(t *testing.T) {
	handler, node := newTestHandler(t)
	defer node.Close()
	key, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	target, err := peer.IDFromPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}

	followResponse := request(t, handler, http.MethodPost, "/ob/follow", `{"id":"`+target.String()+`"}`)
	if followResponse.Code != http.StatusOK {
		t.Fatalf("following: %d %s", followResponse.Code, followResponse.Body)
	}
	followingResponse := request(t, handler, http.MethodGet, "/ob/following", "")
	var following []string
	decodeResponse(t, followingResponse, &following)
	if len(following) != 1 || following[0] != target.String() {
		t.Fatalf("unexpected following list: %v", following)
	}

	unfollowResponse := request(t, handler, http.MethodPost, "/ob/unfollow", `{"id":"`+target.String()+`"}`)
	if unfollowResponse.Code != http.StatusOK {
		t.Fatalf("unfollowing: %d %s", unfollowResponse.Code, unfollowResponse.Body)
	}
	if got := request(t, handler, http.MethodGet, "/ob/followers", "").Code; got != http.StatusOK {
		t.Fatalf("followers status = %d, want %d", got, http.StatusOK)
	}
	if got := request(t, handler, http.MethodGet, "/wallet/currencies", "").Code; got != http.StatusNotFound {
		t.Fatalf("wallet route status = %d, want %d", got, http.StatusNotFound)
	}
	if got := request(t, handler, http.MethodGet, "/ob/listings", "").Code; got != http.StatusNotFound {
		t.Fatalf("marketplace route status = %d, want %d", got, http.StatusNotFound)
	}
}

func TestConfigAndValidation(t *testing.T) {
	handler, node := newTestHandler(t)
	defer node.Close()
	response := request(t, handler, http.MethodGet, "/ob/config", "")
	var config map[string]any
	decodeResponse(t, response, &config)
	if config["peerID"] != node.ID().String() || config["network"] != "bitbook-v2" {
		t.Fatalf("unexpected config: %v", config)
	}
	badPost := request(t, handler, http.MethodPost, "/ob/post", `{"status":""}`)
	if badPost.Code != http.StatusBadRequest {
		t.Fatalf("empty post status = %d, want %d", badPost.Code, http.StatusBadRequest)
	}
}

func TestChatAPIQueuesAndIndexesOfflineMessage(t *testing.T) {
	handler, node := newTestHandler(t)
	defer node.Close()
	key, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	target, err := peer.IDFromPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	chatResponse := request(t, handler, http.MethodPost, "/ob/chat", `{"peerId":"`+target.String()+`","message":"queued hello"}`)
	if chatResponse.Code != http.StatusOK {
		t.Fatalf("sending chat: %d %s", chatResponse.Code, chatResponse.Body)
	}
	var sent map[string]any
	decodeResponse(t, chatResponse, &sent)
	if sent["queued"] != true || sent["messageId"] == "" {
		t.Fatalf("unexpected queued chat response: %v", sent)
	}
	messagesResponse := request(t, handler, http.MethodGet, "/ob/chatmessages/"+target.String(), "")
	var messages []direct.ChatMessage
	decodeResponse(t, messagesResponse, &messages)
	if len(messages) != 1 || messages[0].Message != "queued hello" || !messages[0].Outgoing {
		t.Fatalf("unexpected chat messages: %+v", messages)
	}
	conversationsResponse := request(t, handler, http.MethodGet, "/ob/chatconversations", "")
	var conversations []direct.Conversation
	decodeResponse(t, conversationsResponse, &conversations)
	if len(conversations) != 1 || conversations[0].PeerID != target.String() {
		t.Fatalf("unexpected conversations: %+v", conversations)
	}
	deleteResponse := request(t, handler, http.MethodDelete, "/ob/chatmessage/"+messages[0].MessageID, "")
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("deleting chat: %d %s", deleteResponse.Code, deleteResponse.Body)
	}
}

func TestWebSocketReceivesDirectChat(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	aNode, aDirect, _ := newAPIStack(t, ctx)
	defer aNode.Close()
	defer aDirect.Close()
	bNode, bDirect, bHandler := newAPIStack(t, ctx)
	defer bNode.Close()
	defer bDirect.Close()
	if err := aNode.Connect(ctx, peer.AddrInfo{ID: bNode.ID(), Addrs: bNode.Host.Addrs()}); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(bHandler)
	defer server.Close()
	socketURL := strings.Replace(server.URL, "http://", "ws://", 1) + "/ws"
	socket, _, err := websocket.DefaultDialer.DialContext(ctx, socketURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()
	if delivery, err := aDirect.SendFollow(ctx, bNode.ID(), true); err != nil {
		t.Fatal(err)
	} else if !delivery.Delivered {
		t.Fatalf("follow was not delivered: %+v", delivery)
	}
	_ = socket.SetReadDeadline(time.Now().Add(5 * time.Second))
	var notificationEvent map[string]any
	if err := socket.ReadJSON(&notificationEvent); err != nil {
		t.Fatal(err)
	}
	if _, ok := notificationEvent["notification"]; !ok {
		t.Fatalf("unexpected follow websocket event: %+v", notificationEvent)
	}
	followersResponse := request(t, bHandler, http.MethodGet, "/ob/followers", "")
	var followers []string
	decodeResponse(t, followersResponse, &followers)
	if len(followers) != 1 || followers[0] != aNode.ID().String() {
		t.Fatalf("unexpected API followers: %v", followers)
	}

	if _, delivery, err := aDirect.SendChat(ctx, bNode.ID(), "", "live hello"); err != nil {
		t.Fatal(err)
	} else if !delivery.Delivered {
		t.Fatalf("chat was not delivered: %+v", delivery)
	}
	_ = socket.SetReadDeadline(time.Now().Add(5 * time.Second))
	var event struct {
		Message direct.ChatMessage `json:"message"`
	}
	if err := socket.ReadJSON(&event); err != nil {
		t.Fatal(err)
	}
	if event.Message.Message != "live hello" || event.Message.PeerID != aNode.ID().String() {
		t.Fatalf("unexpected websocket event: %+v", event)
	}
}

func TestNET001PeerAPIRequiresHandshake(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	config := network.Config{
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	}
	seedStore := dsync.MutexWrap(datastore.NewMapDatastore())
	seedHost, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatal(err)
	}
	defer seedHost.Close()
	seedDHT, err := dht.New(seedHost,
		dht.Datastore(seedStore),
		dht.Mode(dht.ModeServer),
		dht.ProtocolPrefix(protocol.ID("/ipfs")),
		dht.BootstrapPeers(),
		dht.AddressFilter(nil),
		dht.QueryFilter(func(_ any, _ peer.AddrInfo) bool { return true }),
		dht.RoutingTableFilter(func(_ any, _ peer.ID) bool { return true }),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer seedDHT.Close()
	if err := seedDHT.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	config.BootstrapPeers = []peer.AddrInfo{{ID: seedHost.ID(), Addrs: slices.Clone(seedHost.Addrs())}}
	subject, err := network.New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer subject.Close()
	realPeer, err := network.New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer realPeer.Close()
	falsePeer, err := network.New(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer falsePeer.Close()
	waitForAPITransport(t, ctx, subject, seedHost.ID())
	waitForAPITransport(t, ctx, realPeer, seedHost.ID())
	waitForAPITransport(t, ctx, falsePeer, seedHost.ID())
	waitForAPIRoutingPeer(t, ctx, subject.DHT)
	waitForAPIRoutingPeer(t, ctx, realPeer.DHT)
	waitForAPIRoutingPeer(t, ctx, falsePeer.DHT)

	invalidWritten := make(chan peer.ID, 16)
	falsePeer.Host.SetStreamHandler(network.DiscoveryProtocolCurrent, func(stream lp2pnet.Stream) {
		defer stream.Close()
		_ = stream.SetDeadline(time.Now().Add(5 * time.Second))
		raw, _ := io.ReadAll(io.LimitReader(stream, 9))
		if string(raw) != "BBGO001\n" {
			_ = stream.Reset()
			return
		}
		if _, err := io.WriteString(stream, "INVALID\n"); err != nil {
			return
		}
		if err := stream.CloseWrite(); err != nil {
			return
		}
		invalidWritten <- stream.Conn().RemotePeer()
	})
	falseDiscovery := routingdiscovery.NewRoutingDiscovery(falsePeer.DHT)
	if _, err := falseDiscovery.Advertise(ctx, network.DiscoveryNamespace); err != nil {
		t.Fatal(err)
	}
	realDiscoveryCtx, stopRealDiscovery := context.WithCancel(ctx)
	defer stopRealDiscovery()
	if err := realPeer.StartDiscovery(realDiscoveryCtx); err != nil {
		t.Fatal(err)
	}
	seedDiscovery := routingdiscovery.NewRoutingDiscovery(seedDHT)
	waitForAdvertisedPeer(t, ctx, seedDiscovery, realPeer.ID())
	waitForAdvertisedPeer(t, ctx, seedDiscovery, falsePeer.ID())

	ordinary, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatal(err)
	}
	defer ordinary.Close()
	if err := ordinary.Connect(ctx, peer.AddrInfo{ID: subject.ID(), Addrs: slices.Clone(subject.Host.Addrs())}); err != nil {
		t.Fatal(err)
	}
	subjectDiscoveryCtx, stopSubjectDiscovery := context.WithCancel(ctx)
	defer stopSubjectDiscovery()
	if err := subject.StartDiscovery(subjectDiscoveryCtx); err != nil {
		t.Fatal(err)
	}
	responseWrittenForSubject := false
	for !responseWrittenForSubject {
		select {
		case observer := <-invalidWritten:
			if observer == subject.ID() {
				responseWrittenForSubject = true
			}
		case <-ctx.Done():
			t.Fatalf("false advertiser did not write its invalid response to the API node: %v", ctx.Err())
		}
	}
	waitForAPIBitBookPeer(t, ctx, subject, realPeer.ID())
	stopSubjectDiscovery()
	stopRealDiscovery()
	waitForAPIDiscoveryStopped(t, ctx, subject)
	waitForAPIDiscoveryStopped(t, ctx, realPeer)

	store, err := social.NewStore(subject)
	if err != nil {
		t.Fatal(err)
	}
	directService, err := direct.NewService(subject, store)
	if err != nil {
		t.Fatal(err)
	}
	defer directService.Close()
	handler, err := NewHandler(subject, store, directService)
	if err != nil {
		t.Fatal(err)
	}
	assertPeerAPI := func(wantReal bool) {
		t.Helper()
		response := request(t, handler, http.MethodGet, "/ob/peers", "")
		var peers []string
		decodeResponse(t, response, &peers)
		want := []string{}
		if wantReal {
			want = []string{realPeer.ID().String()}
		}
		if !slices.Equal(peers, want) {
			t.Fatalf("/ob/peers = %v, want %v", peers, want)
		}
		realStatus := request(t, handler, http.MethodGet, "/ob/status/"+realPeer.ID().String(), "")
		var realBody map[string]string
		decodeResponse(t, realStatus, &realBody)
		wantStatus := "not connected"
		if wantReal {
			wantStatus = "connected"
		}
		if realBody["status"] != wantStatus {
			t.Fatalf("real peer status = %q, want %q", realBody["status"], wantStatus)
		}
		for _, rejected := range []peer.ID{ordinary.ID(), falsePeer.ID()} {
			status := request(t, handler, http.MethodGet, "/ob/status/"+rejected.String(), "")
			var body map[string]string
			decodeResponse(t, status, &body)
			if body["status"] != "not connected" {
				t.Fatalf("unconfirmed peer %s status = %q", rejected, body["status"])
			}
		}
	}
	assertPeerAPI(true)
	if response := request(t, handler, http.MethodGet, "/ob/status/not-a-peer-id", ""); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid peer status = %d, want 400", response.Code)
	}

	connections := realPeer.Host.Network().ConnsToPeer(subject.ID())
	if len(connections) == 0 {
		t.Fatal("real peer has no initial connection to subject")
	}
	oldConnectionIDs := make(map[string]struct{}, len(connections))
	for _, connection := range connections {
		oldConnectionIDs[connection.ID()] = struct{}{}
	}
	if err := subject.Host.Network().ClosePeer(realPeer.ID()); err != nil {
		t.Fatal(err)
	}
	if err := realPeer.Host.Network().ClosePeer(subject.ID()); err != nil {
		t.Fatal(err)
	}
	waitForAPITransportGap(t, ctx, subject, realPeer)
	waitForAPINoBitBookPeer(t, ctx, subject, realPeer.ID())
	assertPeerAPI(false)
	if err := realPeer.Host.Connect(ctx, peer.AddrInfo{ID: subject.ID(), Addrs: slices.Clone(subject.Host.Addrs())}); err != nil {
		t.Fatal(err)
	}
	reconnected := realPeer.Host.Network().ConnsToPeer(subject.ID())
	if realPeer.Host.Network().Connectedness(subject.ID()) != lp2pnet.Connected || len(reconnected) == 0 || slices.ContainsFunc(reconnected, func(connection lp2pnet.Conn) bool {
		_, old := oldConnectionIDs[connection.ID()]
		return old
	}) {
		t.Fatalf("reconnect did not establish a fresh live real-peer connection: %v", reconnected)
	}
	stream, err := realPeer.Host.NewStream(ctx, subject.ID(), network.DiscoveryProtocolCurrent)
	if err != nil {
		t.Fatal(err)
	}
	_ = stream.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.WriteString(stream, "BBGO001\n"); err != nil {
		t.Fatal(err)
	}
	if err := stream.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	responseBytes, err := io.ReadAll(io.LimitReader(stream, 9))
	_ = stream.Close()
	if err != nil || string(responseBytes) != "BBGO001\n" {
		t.Fatalf("fresh discovery response = %q, %v", responseBytes, err)
	}
	waitForAPIBitBookPeer(t, ctx, subject, realPeer.ID())
	assertPeerAPI(true)
}

func waitForAdvertisedPeer(t testing.TB, ctx context.Context, discovery *routingdiscovery.RoutingDiscovery, want peer.ID) {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		findCtx, cancel := context.WithTimeout(ctx, time.Second)
		found, err := discovery.FindPeers(findCtx, network.DiscoveryNamespace)
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

func waitForAPIBitBookPeer(t testing.TB, ctx context.Context, node *network.Node, want peer.ID) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if slices.Contains(node.BitBookPeers(), want) {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for confirmed peer %s: %v", want, ctx.Err())
		case <-ticker.C:
		}
	}
}

func waitForAPINoBitBookPeer(t testing.TB, ctx context.Context, node *network.Node, want peer.ID) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if !slices.Contains(node.BitBookPeers(), want) {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting to remove confirmed peer %s: %v", want, ctx.Err())
		case <-ticker.C:
		}
	}
}

func waitForAPIDiscoveryStopped(t testing.TB, ctx context.Context, node *network.Node) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		err := node.StartDiscovery(ctx)
		if err != nil {
			if ctx.Err() != nil {
				t.Fatalf("waiting for discovery to stop: %v", ctx.Err())
			}
			if err.Error() != "discovery cannot be restarted" {
				t.Fatalf("waiting for discovery to stop: %v", err)
			}
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for discovery to stop: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

func waitForAPITransportGap(t testing.TB, ctx context.Context, a, b *network.Node) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		aConnections := a.Host.Network().ConnsToPeer(b.ID())
		bConnections := b.Host.Network().ConnsToPeer(a.ID())
		if len(aConnections) == 0 && len(bConnections) == 0 &&
			a.Host.Network().Connectedness(b.ID()) != lp2pnet.Connected &&
			b.Host.Network().Connectedness(a.ID()) != lp2pnet.Connected {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for transport gap between %s and %s: a=%v b=%v: %v", a.ID(), b.ID(), aConnections, bConnections, ctx.Err())
		case <-ticker.C:
		}
	}
}

func waitForAPITransport(t testing.TB, ctx context.Context, node *network.Node, want peer.ID) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if node.Host.Network().Connectedness(want) == lp2pnet.Connected {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for transport peer %s: %v", want, ctx.Err())
		case <-ticker.C:
		}
	}
}

func waitForAPIRoutingPeer(t testing.TB, ctx context.Context, kad *dht.IpfsDHT) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for kad.RoutingTable().Size() == 0 {
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for DHT routing peer: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

func newTestHandler(t *testing.T) (*Handler, *network.Node) {
	t.Helper()
	node, err := network.New(context.Background(), network.Config{
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	store, err := social.NewStore(node)
	if err != nil {
		node.Close()
		t.Fatal(err)
	}
	directService, err := direct.NewService(node, store)
	if err != nil {
		node.Close()
		t.Fatal(err)
	}
	handler, err := NewHandler(node, store, directService)
	if err != nil {
		directService.Close()
		node.Close()
		t.Fatal(err)
	}
	return handler, node
}

func newAPIStack(t *testing.T, ctx context.Context) (*network.Node, *direct.Service, *Handler) {
	t.Helper()
	node, err := network.New(ctx, network.Config{
		ListenAddrs:           []string{"/ip4/127.0.0.1/tcp/0"},
		DHTMode:               dht.ModeServer,
		AllowPrivateAddresses: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	store, err := social.NewStore(node)
	if err != nil {
		node.Close()
		t.Fatal(err)
	}
	directService, err := direct.NewService(node, store)
	if err != nil {
		node.Close()
		t.Fatal(err)
	}
	handler, err := NewHandler(node, store, directService)
	if err != nil {
		directService.Close()
		node.Close()
		t.Fatal(err)
	}
	return node, directService, handler
}

func request(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	handler.ServeHTTP(recorder, request)
	return recorder
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("response: %d %s", response.Code, response.Body)
	}
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decoding %s: %v", response.Body, err)
	}
}
