package social

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/larslarsen/bb-go/modern/network"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	maxProfileBytes  = 1 << 20
	maxStatusRunes   = 280
	maxLongFormRunes = 50_000
)

var (
	profileKey   = datastore.NewKey("/bitbook/social/profile")
	postsKey     = datastore.NewKey("/bitbook/social/posts")
	followingKey = datastore.NewKey("/bitbook/social/following")
	rootKey      = datastore.NewKey("/bitbook/social/root")
)

// Store persists local social state and publishes immutable snapshots through
// the maintained BitBook network core.
type Store struct {
	node      *network.Node
	now       func() time.Time
	mu        sync.Mutex
	publishMu sync.Mutex
}

func NewStore(node *network.Node) (*Store, error) {
	if node == nil {
		return nil, errors.New("nil network node")
	}
	if node.Datastore == nil {
		return nil, errors.New("network node has no datastore")
	}
	return &Store{node: node, now: time.Now}, nil
}

// SetProfile validates and persists an OpenBazaar-compatible JSON profile. It
// owns peerID, lastModified, and the following count; unknown presentation
// fields are retained so the existing desktop can migrate incrementally.
func (s *Store) SetProfile(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(raw) > maxProfileBytes {
		return nil, fmt.Errorf("profile exceeds %d bytes", maxProfileBytes)
	}
	var profile map[string]any
	if err := json.Unmarshal(raw, &profile); err != nil {
		return nil, fmt.Errorf("decoding profile: %w", err)
	}
	if profile == nil {
		return nil, errors.New("profile must be a JSON object")
	}
	profile["peerID"] = s.node.ID().String()
	profile["lastModified"] = s.now().UTC().Format(time.RFC3339Nano)

	following, err := s.loadFollowing(ctx)
	if err != nil {
		return nil, err
	}
	stats, _ := profile["stats"].(map[string]any)
	if stats == nil {
		stats = make(map[string]any)
	}
	stats["followingCount"] = len(following)
	profile["stats"] = stats

	normalized, err := json.Marshal(profile)
	if err != nil {
		return nil, fmt.Errorf("encoding profile: %w", err)
	}
	if err := s.node.Datastore.Put(ctx, profileKey, normalized); err != nil {
		return nil, fmt.Errorf("storing profile: %w", err)
	}
	return slices.Clone(normalized), nil
}

func (s *Store) LocalProfile(ctx context.Context) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadProfile(ctx)
}

// AddPost creates and signs an immutable post. The accepted JSON fields match
// the old post API; identity, timestamp, slug, and signature are server-owned.
func (s *Store) AddPost(ctx context.Context, raw json.RawMessage) (SignedPost, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var post map[string]any
	if err := json.Unmarshal(raw, &post); err != nil {
		return SignedPost{}, fmt.Errorf("decoding post: %w", err)
	}
	if post == nil {
		return SignedPost{}, errors.New("post must be a JSON object")
	}
	if err := validatePost(post); err != nil {
		return SignedPost{}, err
	}
	posts, err := s.loadPosts(ctx)
	if err != nil {
		return SignedPost{}, err
	}

	slug, _ := post["slug"].(string)
	if slug == "" {
		status, _ := post["status"].(string)
		slug = uniqueSlug(slugify(status), posts)
	} else if postSlugExists(slug, posts) {
		return SignedPost{}, fmt.Errorf("post %q already exists", slug)
	}
	post["slug"] = slug
	post["vendorID"] = map[string]any{"peerID": s.node.ID().String()}
	post["timestamp"] = s.now().UTC().Format(time.RFC3339Nano)
	if _, ok := post["postType"]; !ok {
		post["postType"] = "POST"
	}

	canonical, err := json.Marshal(post)
	if err != nil {
		return SignedPost{}, fmt.Errorf("encoding post: %w", err)
	}
	signature, err := s.node.PrivateKey.Sign(canonical)
	if err != nil {
		return SignedPost{}, fmt.Errorf("signing post: %w", err)
	}
	publicKey, err := crypto.MarshalPublicKey(s.node.PrivateKey.GetPublic())
	if err != nil {
		return SignedPost{}, fmt.Errorf("encoding post public key: %w", err)
	}
	envelope := SignedPost{Post: canonical, Signature: signature, PublicKey: publicKey}
	blockBytes, err := json.Marshal(envelope)
	if err != nil {
		return SignedPost{}, fmt.Errorf("encoding signed post: %w", err)
	}
	postCID, err := s.node.Put(ctx, blockBytes)
	if err != nil {
		return SignedPost{}, err
	}
	envelope.CID = postCID.String()
	posts = append([]SignedPost{envelope}, posts...)
	if err := s.saveJSON(ctx, postsKey, posts); err != nil {
		return SignedPost{}, err
	}
	return cloneSignedPost(envelope), nil
}

func (s *Store) DeletePost(ctx context.Context, slug string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	posts, err := s.loadPosts(ctx)
	if err != nil {
		return err
	}
	filtered := posts[:0]
	for _, post := range posts {
		if postSlug(post) != slug && post.CID != slug {
			filtered = append(filtered, post)
		}
	}
	if len(filtered) == len(posts) {
		return datastore.ErrNotFound
	}
	return s.saveJSON(ctx, postsKey, filtered)
}

func (s *Store) LocalPosts(ctx context.Context) ([]SignedPost, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	posts, err := s.loadPosts(ctx)
	return clonePosts(posts), err
}

func (s *Store) Follow(ctx context.Context, id peer.ID) error {
	if id == s.node.ID() {
		return errors.New("cannot follow self")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	following, err := s.loadFollowing(ctx)
	if err != nil {
		return err
	}
	encoded := id.String()
	if !slices.Contains(following, encoded) {
		following = append(following, encoded)
		sort.Strings(following)
		if err := s.saveJSON(ctx, followingKey, following); err != nil {
			return err
		}
	}
	return s.updateProfileFollowingCount(ctx, len(following))
}

func (s *Store) Unfollow(ctx context.Context, id peer.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	following, err := s.loadFollowing(ctx)
	if err != nil {
		return err
	}
	following = slices.DeleteFunc(following, func(candidate string) bool { return candidate == id.String() })
	if err := s.saveJSON(ctx, followingKey, following); err != nil {
		return err
	}
	return s.updateProfileFollowingCount(ctx, len(following))
}

func (s *Store) LocalFollowing(ctx context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	following, err := s.loadFollowing(ctx)
	return slices.Clone(following), err
}

// Commit writes the current sections and manifest as immutable blocks without
// requiring network connectivity.
func (s *Store) Commit(ctx context.Context) (cid.Cid, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.commitLocked(ctx)
}

// Publish commits and advertises the current root through provider records and
// signed IPNS.
func (s *Store) Publish(ctx context.Context) (cid.Cid, error) {
	s.mu.Lock()
	root, err := s.commitLocked(ctx)
	s.mu.Unlock()
	if err != nil {
		return cid.Undef, err
	}
	if err := s.PublishRoot(ctx, root); err != nil {
		return cid.Undef, err
	}
	return root, nil
}

// PublishRoot advertises an already committed root. Publication is serialized
// so periodic refreshes cannot race an API-triggered IPNS sequence update.
func (s *Store) PublishRoot(ctx context.Context, root cid.Cid) error {
	s.publishMu.Lock()
	defer s.publishMu.Unlock()
	return s.node.PublishRoot(ctx, root)
}

// Fetch resolves and validates another peer's published state.
func (s *Store) Fetch(ctx context.Context, owner peer.ID) (State, error) {
	root, err := s.node.ResolveRoot(ctx, owner)
	if err != nil {
		return State{}, err
	}
	manifestBytes, err := s.node.Get(ctx, root)
	if err != nil {
		return State{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return State{}, fmt.Errorf("decoding social manifest: %w", err)
	}
	if manifest.Version != ManifestVersion || manifest.Author != owner.String() {
		return State{}, errors.New("social manifest identity or version mismatch")
	}

	profile, err := s.fetchSection(ctx, manifest.Profile)
	if err != nil {
		return State{}, fmt.Errorf("fetching profile: %w", err)
	}
	postsBytes, err := s.fetchSection(ctx, manifest.Posts)
	if err != nil {
		return State{}, fmt.Errorf("fetching posts: %w", err)
	}
	followingBytes, err := s.fetchSection(ctx, manifest.Following)
	if err != nil {
		return State{}, fmt.Errorf("fetching following: %w", err)
	}
	var posts []SignedPost
	if err := json.Unmarshal(postsBytes, &posts); err != nil {
		return State{}, fmt.Errorf("decoding posts: %w", err)
	}
	for _, post := range posts {
		if err := VerifyPost(owner, post); err != nil {
			return State{}, err
		}
	}
	var following []string
	if err := json.Unmarshal(followingBytes, &following); err != nil {
		return State{}, fmt.Errorf("decoding following: %w", err)
	}
	return State{Manifest: manifest, Profile: profile, Posts: posts, Following: following}, nil
}

func VerifyPost(author peer.ID, post SignedPost) error {
	publicKey, err := crypto.UnmarshalPublicKey(post.PublicKey)
	if err != nil {
		return fmt.Errorf("decoding post author key: %w", err)
	}
	claimedAuthor, err := peer.IDFromPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("deriving post author identity: %w", err)
	}
	if claimedAuthor != author {
		return fmt.Errorf("post public key belongs to %s, not %s", claimedAuthor, author)
	}
	valid, err := publicKey.Verify(post.Post, post.Signature)
	if err != nil {
		return fmt.Errorf("verifying post signature: %w", err)
	}
	if !valid {
		return errors.New("invalid post signature")
	}
	return nil
}

func (s *Store) commitLocked(ctx context.Context) (cid.Cid, error) {
	profile, err := s.loadProfile(ctx)
	if err != nil {
		return cid.Undef, err
	}
	posts, err := s.loadPosts(ctx)
	if err != nil {
		return cid.Undef, err
	}
	following, err := s.loadFollowing(ctx)
	if err != nil {
		return cid.Undef, err
	}
	postsBytes, err := json.Marshal(posts)
	if err != nil {
		return cid.Undef, err
	}
	followingBytes, err := json.Marshal(following)
	if err != nil {
		return cid.Undef, err
	}
	profileCID, err := s.node.Put(ctx, profile)
	if err != nil {
		return cid.Undef, err
	}
	postsCID, err := s.node.Put(ctx, postsBytes)
	if err != nil {
		return cid.Undef, err
	}
	followingCID, err := s.node.Put(ctx, followingBytes)
	if err != nil {
		return cid.Undef, err
	}
	manifest := Manifest{
		Version: ManifestVersion, Author: s.node.ID().String(),
		Profile: profileCID.String(), Posts: postsCID.String(), Following: followingCID.String(),
		UpdatedAt: s.now().UTC(),
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return cid.Undef, err
	}
	root, err := s.node.Put(ctx, manifestBytes)
	if err != nil {
		return cid.Undef, err
	}
	if err := s.node.Datastore.Put(ctx, rootKey, []byte(root.String())); err != nil {
		return cid.Undef, fmt.Errorf("storing social root: %w", err)
	}
	return root, nil
}

func (s *Store) fetchSection(ctx context.Context, encoded string) ([]byte, error) {
	id, err := cid.Decode(encoded)
	if err != nil {
		return nil, err
	}
	return s.node.Get(ctx, id)
}

func (s *Store) loadProfile(ctx context.Context) ([]byte, error) {
	profile, err := s.node.Datastore.Get(ctx, profileKey)
	if errors.Is(err, datastore.ErrNotFound) {
		return []byte(fmt.Sprintf(`{"peerID":%q,"stats":{"followingCount":0}}`, s.node.ID())), nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading profile: %w", err)
	}
	return slices.Clone(profile), nil
}

func (s *Store) loadPosts(ctx context.Context) ([]SignedPost, error) {
	var posts []SignedPost
	if err := s.loadJSON(ctx, postsKey, &posts); err != nil {
		return nil, err
	}
	return posts, nil
}

func (s *Store) loadFollowing(ctx context.Context) ([]string, error) {
	var following []string
	if err := s.loadJSON(ctx, followingKey, &following); err != nil {
		return nil, err
	}
	return following, nil
}

func (s *Store) loadJSON(ctx context.Context, key datastore.Key, target any) error {
	raw, err := s.node.Datastore.Get(ctx, key)
	if errors.Is(err, datastore.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func (s *Store) saveJSON(ctx context.Context, key datastore.Key, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.node.Datastore.Put(ctx, key, raw)
}

func (s *Store) updateProfileFollowingCount(ctx context.Context, count int) error {
	profile, err := s.loadProfile(ctx)
	if err != nil {
		return err
	}
	var decoded map[string]any
	if err := json.Unmarshal(profile, &decoded); err != nil {
		return err
	}
	stats, _ := decoded["stats"].(map[string]any)
	if stats == nil {
		stats = make(map[string]any)
	}
	stats["followingCount"] = count
	decoded["stats"] = stats
	decoded["lastModified"] = s.now().UTC().Format(time.RFC3339Nano)
	return s.saveJSON(ctx, profileKey, decoded)
}

func validatePost(post map[string]any) error {
	status, _ := post["status"].(string)
	longForm, _ := post["longForm"].(string)
	if status == "" && longForm == "" {
		return errors.New("post requires status or longForm")
	}
	if len([]rune(status)) > maxStatusRunes {
		return fmt.Errorf("status exceeds %d characters", maxStatusRunes)
	}
	if len([]rune(longForm)) > maxLongFormRunes {
		return fmt.Errorf("longForm exceeds %d characters", maxLongFormRunes)
	}
	return nil
}

func slugify(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "post"
	}
	if len(result) > 80 {
		result = strings.TrimRight(result[:80], "-")
	}
	return result
}

func uniqueSlug(base string, posts []SignedPost) string {
	if !postSlugExists(base, posts) {
		return base
	}
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s-%d", base, suffix)
		if !postSlugExists(candidate, posts) {
			return candidate
		}
	}
}

func postSlugExists(slug string, posts []SignedPost) bool {
	for _, post := range posts {
		if postSlug(post) == slug {
			return true
		}
	}
	return false
}

func postSlug(post SignedPost) string {
	var decoded struct {
		Slug string `json:"slug"`
	}
	_ = json.Unmarshal(post.Post, &decoded)
	return decoded.Slug
}

func cloneSignedPost(post SignedPost) SignedPost {
	post.Post = slices.Clone(post.Post)
	post.Signature = slices.Clone(post.Signature)
	post.PublicKey = slices.Clone(post.PublicKey)
	return post
}

func clonePosts(posts []SignedPost) []SignedPost {
	result := make([]SignedPost, len(posts))
	for i, post := range posts {
		result[i] = cloneSignedPost(post)
	}
	return result
}
