// Package api exposes the social-only HTTP surface used by BitBook clients.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/larslarsen/bb-go/modern/direct"
	"github.com/larslarsen/bb-go/modern/network"
	"github.com/larslarsen/bb-go/modern/social"
	lp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

const maxRequestBytes = 1 << 20

// Handler implements the maintained, social-only subset of the historical
// /ob API. Marketplace and wallet routes are intentionally absent.
type Handler struct {
	node   *network.Node
	store  *social.Store
	direct *direct.Service
}

func NewHandler(node *network.Node, store *social.Store, directService *direct.Service) (*Handler, error) {
	if node == nil {
		return nil, errors.New("nil network node")
	}
	if store == nil {
		return nil, errors.New("nil social store")
	}
	if directService == nil {
		return nil, errors.New("nil direct service")
	}
	return &Handler{node: node, store: store, direct: directService}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSuffix(r.URL.Path, "/") == "/ws" && r.Method == http.MethodGet {
		h.serveWebSocket(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == "" {
		path = "/"
	}
	switch {
	case path == "/ob/healthcheck" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case path == "/ob/config" && r.Method == http.MethodGet:
		h.getConfig(w)
	case path == "/ob/peers" && r.Method == http.MethodGet:
		h.getPeers(w)
	case strings.HasPrefix(path, "/ob/status/") && r.Method == http.MethodGet:
		h.getStatus(w, strings.TrimPrefix(path, "/ob/status/"))
	case path == "/ob/profile" && r.Method == http.MethodGet:
		h.getLocalProfile(w, r)
	case strings.HasPrefix(path, "/ob/profile/") && r.Method == http.MethodGet:
		h.getRemoteProfile(w, r, strings.TrimPrefix(path, "/ob/profile/"))
	case path == "/ob/profile" && (r.Method == http.MethodPost || r.Method == http.MethodPut):
		h.setProfile(w, r)
	case path == "/ob/profile" && r.Method == http.MethodPatch:
		h.patchProfile(w, r)
	case path == "/ob/follow" && r.Method == http.MethodPost:
		h.follow(w, r, true)
	case path == "/ob/unfollow" && r.Method == http.MethodPost:
		h.follow(w, r, false)
	case path == "/ob/following" && r.Method == http.MethodGet:
		h.getLocalFollowing(w, r)
	case strings.HasPrefix(path, "/ob/following/") && r.Method == http.MethodGet:
		h.getRemoteFollowing(w, r, strings.TrimPrefix(path, "/ob/following/"))
	case path == "/ob/followers" && r.Method == http.MethodGet:
		h.getLocalFollowers(w, r)
	case strings.HasPrefix(path, "/ob/followers/") && r.Method == http.MethodGet:
		h.getRemoteFollowers(w, r, strings.TrimPrefix(path, "/ob/followers/"))
	case strings.HasPrefix(path, "/ob/followsme/") && r.Method == http.MethodGet:
		h.getFollowsMe(w, r, strings.TrimPrefix(path, "/ob/followsme/"))
	case strings.HasPrefix(path, "/ob/isfollowing/") && r.Method == http.MethodGet:
		h.getIsFollowing(w, r, strings.TrimPrefix(path, "/ob/isfollowing/"))
	case path == "/ob/post" && r.Method == http.MethodPost:
		h.addPost(w, r)
	case path == "/ob/posts" && r.Method == http.MethodGet:
		h.getLocalPosts(w, r)
	case strings.HasPrefix(path, "/ob/posts/") && r.Method == http.MethodGet:
		h.getRemotePosts(w, r, strings.TrimPrefix(path, "/ob/posts/"))
	case strings.HasPrefix(path, "/ob/post/") && r.Method == http.MethodGet:
		h.getPost(w, r, strings.TrimPrefix(path, "/ob/post/"))
	case strings.HasPrefix(path, "/ob/post/") && r.Method == http.MethodDelete:
		h.deletePost(w, r, strings.TrimPrefix(path, "/ob/post/"))
	case path == "/ob/chat" && r.Method == http.MethodPost:
		h.sendChat(w, r)
	case path == "/ob/chatmessages" && r.Method == http.MethodGet:
		h.getChatMessages(w, r, "")
	case strings.HasPrefix(path, "/ob/chatmessages/") && r.Method == http.MethodGet:
		h.getChatMessages(w, r, strings.TrimPrefix(path, "/ob/chatmessages/"))
	case path == "/ob/chatconversations" && r.Method == http.MethodGet:
		h.getChatConversations(w, r)
	case (path == "/ob/markchatasread" || strings.HasPrefix(path, "/ob/markchatasread/")) && r.Method == http.MethodPost:
		h.markChatAsRead(w, r, strings.TrimPrefix(path, "/ob/markchatasread/"))
	case strings.HasPrefix(path, "/ob/chatmessage/") && r.Method == http.MethodDelete:
		h.deleteChatMessage(w, r, strings.TrimPrefix(path, "/ob/chatmessage/"))
	case strings.HasPrefix(path, "/ob/chatconversation/") && r.Method == http.MethodDelete:
		h.deleteChatConversation(w, r, strings.TrimPrefix(path, "/ob/chatconversation/"))
	case path == "/ob/publish" && r.Method == http.MethodPost:
		h.publish(w, r)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (h *Handler) getConfig(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, map[string]any{
		"peerID":  h.node.ID().String(),
		"testnet": false,
		"tor":     false,
		"wallets": []string{},
		"network": "bitbook-v2",
	})
}

func (h *Handler) getPeers(w http.ResponseWriter) {
	peers := h.node.Host.Network().Peers()
	encoded := make([]string, len(peers))
	for i, id := range peers {
		encoded[i] = id.String()
	}
	slices.Sort(encoded)
	writeJSON(w, http.StatusOK, encoded)
}

func (h *Handler) getStatus(w http.ResponseWriter, encoded string) {
	id, err := peer.Decode(encoded)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	status := "not connected"
	if h.node.Host.Network().Connectedness(id) == lp2pnet.Connected {
		status = "connected"
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

func (h *Handler) getLocalProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := h.store.LocalProfile(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeRawJSON(w, http.StatusOK, profile)
}

func (h *Handler) getRemoteProfile(w http.ResponseWriter, r *http.Request, encoded string) {
	state, ok := h.fetchRemote(w, r, encoded)
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	writeRawJSON(w, http.StatusOK, state.Profile)
}

func (h *Handler) setProfile(w http.ResponseWriter, r *http.Request) {
	raw, ok := readBody(w, r)
	if !ok {
		return
	}
	profile, err := h.store.SetProfile(r.Context(), raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	publication := h.commitAndPublish(r.Context())
	writePublicationHeaders(w, publication)
	writeRawJSON(w, http.StatusOK, profile)
}

func (h *Handler) patchProfile(w http.ResponseWriter, r *http.Request) {
	patchBytes, ok := readBody(w, r)
	if !ok {
		return
	}
	current, err := h.store.LocalProfile(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var document, patch map[string]any
	if err := json.Unmarshal(current, &document); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := json.Unmarshal(patchBytes, &patch); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	mergeObject(document, patch)
	merged, err := json.Marshal(document)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.store.SetProfile(r.Context(), merged); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, h.commitAndPublish(r.Context()))
}

func (h *Handler) follow(w http.ResponseWriter, r *http.Request, following bool) {
	var request struct {
		ID string `json:"id"`
	}
	if !decodeBody(w, r, &request) {
		return
	}
	id, err := peer.Decode(request.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if following {
		err = h.store.Follow(r.Context(), id)
	} else {
		err = h.store.Unfollow(r.Context(), id)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	publication := h.commitAndPublish(r.Context())
	delivery, err := h.direct.SendFollow(r.Context(), id, following)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for key, value := range deliveryResponse(delivery) {
		if key == "warning" {
			publication["deliveryWarning"] = value
			continue
		}
		publication[key] = value
	}
	writeJSON(w, http.StatusOK, publication)
}

func (h *Handler) getLocalFollowing(w http.ResponseWriter, r *http.Request) {
	following, err := h.store.LocalFollowing(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, following)
}

func (h *Handler) getRemoteFollowing(w http.ResponseWriter, r *http.Request, encoded string) {
	state, ok := h.fetchRemote(w, r, encoded)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, state.Following)
}

func (h *Handler) getLocalFollowers(w http.ResponseWriter, r *http.Request) {
	followers, err := h.store.LocalFollowers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, followers)
}

func (h *Handler) getRemoteFollowers(w http.ResponseWriter, r *http.Request, encoded string) {
	state, ok := h.fetchRemote(w, r, encoded)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, state.Followers)
}

func (h *Handler) getFollowsMe(w http.ResponseWriter, r *http.Request, encoded string) {
	followers, err := h.store.LocalFollowers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"followsMe": slices.Contains(followers, encoded)})
}

func (h *Handler) getIsFollowing(w http.ResponseWriter, r *http.Request, encoded string) {
	following, err := h.store.LocalFollowing(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"isFollowing": slices.Contains(following, encoded)})
}

func (h *Handler) addPost(w http.ResponseWriter, r *http.Request) {
	raw, ok := readBody(w, r)
	if !ok {
		return
	}
	post, err := h.store.AddPost(r.Context(), raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var decoded struct {
		Slug string `json:"slug"`
	}
	_ = json.Unmarshal(post.Post, &decoded)
	response := h.commitAndPublish(r.Context())
	response["slug"] = decoded.Slug
	response["hash"] = post.CID
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) getLocalPosts(w http.ResponseWriter, r *http.Request) {
	posts, err := h.store.LocalPosts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, postSummaries(posts))
}

func (h *Handler) getRemotePosts(w http.ResponseWriter, r *http.Request, encoded string) {
	state, ok := h.fetchRemote(w, r, encoded)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, postSummaries(state.Posts))
}

func (h *Handler) getPost(w http.ResponseWriter, r *http.Request, remainder string) {
	parts := strings.Split(remainder, "/")
	var posts []social.SignedPost
	var lookup string
	if len(parts) == 1 {
		local, err := h.store.LocalPosts(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		posts, lookup = local, parts[0]
	} else if len(parts) == 2 {
		state, ok := h.fetchRemote(w, r, parts[0])
		if !ok {
			return
		}
		posts, lookup = state.Posts, parts[1]
	} else {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	for _, post := range posts {
		if post.CID == lookup || postSlug(post) == lookup {
			writeJSON(w, http.StatusOK, post)
			return
		}
	}
	writeError(w, http.StatusNotFound, "post not found")
}

func (h *Handler) deletePost(w http.ResponseWriter, r *http.Request, lookup string) {
	if strings.Contains(lookup, "/") || lookup == "" {
		writeError(w, http.StatusBadRequest, "invalid post identifier")
		return
	}
	if err := h.store.DeletePost(r.Context(), lookup); err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			writeError(w, http.StatusNotFound, "post not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, h.commitAndPublish(r.Context()))
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	root, err := h.store.Publish(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"root": root.String(), "published": true})
}

func (h *Handler) sendChat(w http.ResponseWriter, r *http.Request) {
	var request struct {
		PeerID  string `json:"peerId"`
		Subject string `json:"subject"`
		Message string `json:"message"`
	}
	if !decodeBody(w, r, &request) {
		return
	}
	recipient, err := peer.Decode(request.PeerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	message, delivery, err := h.direct.SendChat(r.Context(), recipient, request.Subject, request.Message)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	response := deliveryResponse(delivery)
	response["messageId"] = message.MessageID
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) getChatMessages(w http.ResponseWriter, r *http.Request, peerID string) {
	limit := -1
	if encoded := r.URL.Query().Get("limit"); encoded != "" {
		parsed, err := strconv.Atoi(encoded)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		limit = parsed
	}
	messages, err := h.direct.Messages(
		r.Context(), peerID, r.URL.Query().Get("subject"), r.URL.Query().Get("offsetId"), limit,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, messages)
}

func (h *Handler) getChatConversations(w http.ResponseWriter, r *http.Request) {
	conversations, err := h.direct.Conversations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, conversations)
}

func (h *Handler) markChatAsRead(w http.ResponseWriter, r *http.Request, encoded string) {
	if encoded == "" || encoded == "/ob/markchatasread" {
		writeError(w, http.StatusBadRequest, "peer ID is required")
		return
	}
	recipient, err := peer.Decode(encoded)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	delivery, err := h.direct.MarkAsRead(r.Context(), recipient, r.URL.Query().Get("subject"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, deliveryResponse(delivery))
}

func (h *Handler) deleteChatMessage(w http.ResponseWriter, r *http.Request, messageID string) {
	if err := h.direct.DeleteMessage(r.Context(), messageID); err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			writeError(w, http.StatusNotFound, "chat message not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (h *Handler) deleteChatConversation(w http.ResponseWriter, r *http.Request, peerID string) {
	if err := h.direct.DeleteConversation(r.Context(), peerID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (h *Handler) fetchRemote(w http.ResponseWriter, r *http.Request, encoded string) (social.State, bool) {
	id, err := peer.Decode(encoded)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return social.State{}, false
	}
	if id == h.node.ID() {
		profile, err := h.store.LocalProfile(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return social.State{}, false
		}
		posts, err := h.store.LocalPosts(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return social.State{}, false
		}
		following, err := h.store.LocalFollowing(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return social.State{}, false
		}
		followers, err := h.store.LocalFollowers(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return social.State{}, false
		}
		return social.State{Profile: profile, Posts: posts, Following: following, Followers: followers}, true
	}
	state, err := h.store.Fetch(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return social.State{}, false
	}
	return state, true
}

func (h *Handler) commitAndPublish(ctx context.Context) map[string]any {
	root, err := h.store.Commit(ctx)
	if err != nil {
		return map[string]any{"published": false, "warning": err.Error()}
	}
	result := map[string]any{"root": root.String(), "published": false}
	if len(h.node.Host.Network().Peers()) == 0 {
		result["warning"] = "saved locally; no peers are connected"
		return result
	}
	publishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	if err := h.store.PublishRoot(publishCtx, root); err != nil {
		result["warning"] = "saved locally; publication failed: " + err.Error()
		return result
	}
	result["published"] = true
	return result
}

func postSummaries(posts []social.SignedPost) []map[string]any {
	result := make([]map[string]any, 0, len(posts))
	for _, signed := range posts {
		var post map[string]any
		if json.Unmarshal(signed.Post, &post) != nil {
			continue
		}
		delete(post, "longForm")
		delete(post, "vendorID")
		post["hash"] = signed.CID
		result = append(result, post)
	}
	return result
}

func deliveryResponse(delivery direct.Delivery) map[string]any {
	result := map[string]any{"delivered": delivery.Delivered, "queued": delivery.Queued}
	if delivery.Warning != "" {
		result["warning"] = delivery.Warning
	}
	return result
}

func postSlug(post social.SignedPost) string {
	var value struct {
		Slug string `json:"slug"`
	}
	_ = json.Unmarshal(post.Post, &value)
	return value.Slug
}

func mergeObject(document, patch map[string]any) {
	for key, patchValue := range patch {
		if patchValue == nil {
			delete(document, key)
			continue
		}
		patchMap, patchIsMap := patchValue.(map[string]any)
		documentMap, documentIsMap := document[key].(map[string]any)
		if patchIsMap && documentIsMap {
			mergeObject(documentMap, patchMap)
			continue
		}
		document[key] = patchValue
	}
}

func decodeBody(w http.ResponseWriter, r *http.Request, target any) bool {
	raw, ok := readBody(w, r)
	if !ok {
		return false
	}
	if err := json.Unmarshal(raw, target); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return false
	}
	return true
}

func readBody(w http.ResponseWriter, r *http.Request) (json.RawMessage, bool) {
	defer r.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return nil, false
	}
	if len(raw) > maxRequestBytes {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("request exceeds %d bytes", maxRequestBytes))
		return nil, false
	}
	return raw, true
}

func writePublicationHeaders(w http.ResponseWriter, publication map[string]any) {
	if root, ok := publication["root"].(string); ok {
		w.Header().Set("X-BitBook-Root", root)
	}
	if published, ok := publication["published"].(bool); ok {
		w.Header().Set("X-BitBook-Published", fmt.Sprint(published))
	}
	if warning, ok := publication["warning"].(string); ok {
		w.Header().Set("Warning", `199 BitBook "`+strings.ReplaceAll(warning, `"`, `'`)+`"`)
	}
}

func writeRawJSON(w http.ResponseWriter, status int, raw json.RawMessage) {
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

func writeError(w http.ResponseWriter, status int, reason string) {
	writeJSON(w, status, map[string]string{"reason": reason})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
