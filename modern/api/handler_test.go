package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/larslarsen/bb-go/modern/network"
	"github.com/larslarsen/bb-go/modern/social"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
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
	if got := request(t, handler, http.MethodGet, "/ob/followers", "").Code; got != http.StatusNotImplemented {
		t.Fatalf("followers status = %d, want %d", got, http.StatusNotImplemented)
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
	handler, err := NewHandler(node, store)
	if err != nil {
		node.Close()
		t.Fatal(err)
	}
	return handler, node
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
