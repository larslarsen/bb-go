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
	"github.com/ipfs/go-datastore/query"
	"github.com/larslarsen/bb-go/modern/attachment"
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
	followersKey = datastore.NewKey("/bitbook/social/followers")
	rootKey      = datastore.NewKey("/bitbook/social/root")
)

// Store persists local social state and publishes immutable snapshots through
// the maintained BitBook network core.
type Store struct {
	node             *network.Node
	now              func() time.Time
	attachments      *attachment.Store
	richLimits       RichPostLimits
	richRecords      map[string]richRecord
	richBytes        int64
	recoveryRequired bool
	mu               sync.Mutex
	publishMu        sync.Mutex
}

func NewStore(node *network.Node) (*Store, error) {
	if node == nil {
		return nil, errors.New("nil network node")
	}
	if node.Datastore == nil {
		return nil, errors.New("network node has no datastore")
	}
	hasRich, err := hasRichPostRecords(context.Background(), node)
	if err != nil {
		return nil, errors.Join(ErrRichPostUnavailable, err)
	}
	if hasRich {
		return nil, ErrRichPostUnavailable
	}
	return &Store{node: node, now: time.Now}, nil
}

func hasRichPostRecords(ctx context.Context, node *network.Node) (bool, error) {
	results, err := node.Datastore.Query(ctx, query.Query{Prefix: richRecordPrefix.String(), Limit: 1, KeysOnly: true})
	if err != nil {
		return false, err
	}
	defer results.Close()
	result, ok := results.NextSync()
	if !ok {
		return false, nil
	}
	return result.Error == nil, result.Error
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
	followerStates, err := s.loadFollowerStates(ctx)
	if err != nil {
		return nil, err
	}
	stats, _ := profile["stats"].(map[string]any)
	if stats == nil {
		stats = make(map[string]any)
	}
	stats["followingCount"] = len(following)
	stats["followerCount"] = len(followerIDs(followerStates))
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
	if err := s.requirePostOperationsLocked(); err != nil {
		return SignedPost{}, err
	}

	var post map[string]any
	if err := json.Unmarshal(raw, &post); err != nil {
		return SignedPost{}, fmt.Errorf("decoding post: %w", err)
	}
	if post == nil {
		return SignedPost{}, errors.New("post must be a JSON object")
	}
	for _, reserved := range []string{"schema", "content", "requestId", "uploadReferences"} {
		if _, exists := post[reserved]; exists {
			return SignedPost{}, fmt.Errorf("legacy post field %q is reserved", reserved)
		}
	}
	if err := validatePost(post); err != nil {
		return SignedPost{}, err
	}
	posts, err := s.loadPosts(ctx)
	if err != nil {
		return SignedPost{}, err
	}

	slug, _ := post["slug"].(string)
	if strings.HasPrefix(slug, "rich-") {
		return SignedPost{}, ErrInvalidRichPost
	}
	if slug == "" {
		status, _ := post["status"].(string)
		slug = s.uniqueLocalSlugLocked(slugify(status), posts)
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
	if err := s.requirePostOperationsLocked(); err != nil {
		return err
	}
	for id, record := range s.richRecords {
		if slug == "rich-"+id || (record.Envelope.CID != "" && record.Envelope.CID == slug) {
			return ErrInvalidRichPost
		}
	}
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
	posts, err := s.loadAllPostsLocked(ctx)
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

// ApplyFollower records the latest signed follow state received directly from
// another peer. Older deliveries are ignored so an out-of-order retry cannot
// undo a newer follow or unfollow.
func (s *Store) ApplyFollower(ctx context.Context, id peer.ID, following bool, updatedAt time.Time) (bool, error) {
	if id == s.node.ID() {
		return false, errors.New("cannot follow self")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	states, err := s.loadFollowerStates(ctx)
	if err != nil {
		return false, err
	}
	encoded := id.String()
	previous, exists := states[encoded]
	if exists && !updatedAt.After(previous.UpdatedAt) {
		return false, nil
	}
	states[encoded] = followerState{Following: following, UpdatedAt: updatedAt.UTC()}
	if err := s.saveJSON(ctx, followersKey, states); err != nil {
		return false, err
	}
	followers := followerIDs(states)
	if err := s.updateProfileFollowerCount(ctx, len(followers)); err != nil {
		return false, err
	}
	return !exists || previous.Following != following, nil
}

func (s *Store) LocalFollowers(ctx context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	states, err := s.loadFollowerStates(ctx)
	if err != nil {
		return nil, err
	}
	return followerIDs(states), nil
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
	s.mu.Lock()
	err := s.requirePostOperationsLocked()
	s.mu.Unlock()
	if err != nil {
		return err
	}
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
	var followers []string
	if manifest.Followers != "" {
		followersBytes, err := s.fetchSection(ctx, manifest.Followers)
		if err != nil {
			return State{}, fmt.Errorf("fetching followers: %w", err)
		}
		if err := json.Unmarshal(followersBytes, &followers); err != nil {
			return State{}, fmt.Errorf("decoding followers: %w", err)
		}
	}
	return State{Manifest: manifest, Profile: profile, Posts: posts, Following: following, Followers: followers}, nil
}

func VerifyPost(author peer.ID, post SignedPost) error {
	isRich, classificationErr := classifyRichCandidate(post.Post)
	if classificationErr != nil {
		return errors.Join(ErrInvalidRichPost, classificationErr)
	}
	if isRich && (len(post.Post) > maxRichPayloadBytes || len(post.Signature) == 0 || len(post.Signature) > maxRichSignatureBytes || len(post.PublicKey) == 0 || len(post.PublicKey) > maxRichPublicKeyBytes) {
		return ErrInvalidRichPost
	}
	publicKey, err := crypto.UnmarshalPublicKey(post.PublicKey)
	if err != nil {
		if isRich {
			return errors.Join(ErrInvalidRichPost, err)
		}
		return fmt.Errorf("decoding post author key: %w", err)
	}
	claimedAuthor, err := peer.IDFromPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("deriving post author identity: %w", err)
	}
	if claimedAuthor != author {
		if isRich {
			return ErrInvalidRichPost
		}
		return fmt.Errorf("post public key belongs to %s, not %s", claimedAuthor, author)
	}
	valid, err := publicKey.Verify(post.Post, post.Signature)
	if err != nil {
		if isRich {
			return errors.Join(ErrInvalidRichPost, err)
		}
		return fmt.Errorf("verifying post signature: %w", err)
	}
	if !valid {
		if isRich {
			return ErrInvalidRichPost
		}
		return errors.New("invalid post signature")
	}
	if isRich {
		if _, err := validateRichEnvelope(author, post); err != nil {
			return errors.Join(ErrInvalidRichPost, err)
		}
	}
	return nil
}

func (s *Store) commitLocked(ctx context.Context) (cid.Cid, error) {
	if err := s.requirePostOperationsLocked(); err != nil {
		return cid.Undef, err
	}
	profile, err := s.loadProfile(ctx)
	if err != nil {
		return cid.Undef, err
	}
	posts, err := s.loadAllPostsLocked(ctx)
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
	followerStates, err := s.loadFollowerStates(ctx)
	if err != nil {
		return cid.Undef, err
	}
	followersBytes, err := json.Marshal(followerIDs(followerStates))
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
	followersCID, err := s.node.Put(ctx, followersBytes)
	if err != nil {
		return cid.Undef, err
	}
	manifest := Manifest{
		Version: ManifestVersion, Author: s.node.ID().String(),
		Profile: profileCID.String(), Posts: postsCID.String(), Following: followingCID.String(),
		Followers: followersCID.String(),
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
		return []byte(fmt.Sprintf(`{"peerID":%q,"stats":{"followerCount":0,"followingCount":0}}`, s.node.ID())), nil
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

func (s *Store) requirePostOperationsLocked() error {
	if s.recoveryRequired {
		return ErrRichPostRecoveryRequired
	}
	return nil
}

func (s *Store) loadAllPostsLocked(ctx context.Context) ([]SignedPost, error) {
	if err := s.requirePostOperationsLocked(); err != nil {
		return nil, err
	}
	legacy, err := s.loadPosts(ctx)
	if err != nil {
		return nil, err
	}
	rich, err := s.liveRichPostsLocked(ctx)
	if err != nil {
		return nil, err
	}
	if len(rich) == 0 {
		return legacy, nil
	}
	type orderedPost struct {
		post      SignedPost
		timestamp time.Time
		validTime bool
		rich      bool
		id        string
	}
	ordered := make([]orderedPost, 0, len(legacy)+len(rich))
	for _, post := range legacy {
		var payload struct {
			Timestamp string `json:"timestamp"`
		}
		_ = json.Unmarshal(post.Post, &payload)
		parsed, parseErr := time.Parse(time.RFC3339Nano, payload.Timestamp)
		ordered = append(ordered, orderedPost{post: post, timestamp: parsed, validTime: parseErr == nil})
	}
	for _, record := range rich {
		payload, parseErr := parseRichPayload(s.node.ID(), record.Envelope.Post)
		if parseErr != nil {
			s.recoveryRequired = true
			return nil, errors.Join(ErrCorruptRichPostState, parseErr)
		}
		ordered = append(ordered, orderedPost{post: record.Envelope, timestamp: payload.Timestamp, validTime: true, rich: true, id: record.ID})
	}
	sort.SliceStable(ordered, func(left, right int) bool {
		a, b := ordered[left], ordered[right]
		if a.validTime != b.validTime {
			return a.validTime
		}
		if a.validTime && !a.timestamp.Equal(b.timestamp) {
			return a.timestamp.After(b.timestamp)
		}
		if a.rich != b.rich {
			return !a.rich
		}
		if a.rich {
			return a.id < b.id
		}
		return false
	})
	posts := make([]SignedPost, len(ordered))
	for index := range ordered {
		posts[index] = ordered[index].post
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

func (s *Store) loadFollowerStates(ctx context.Context) (map[string]followerState, error) {
	states := make(map[string]followerState)
	if err := s.loadJSON(ctx, followersKey, &states); err != nil {
		return nil, err
	}
	return states, nil
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
	return s.updateProfileCount(ctx, "followingCount", count)
}

func (s *Store) updateProfileFollowerCount(ctx context.Context, count int) error {
	return s.updateProfileCount(ctx, "followerCount", count)
}

func (s *Store) updateProfileCount(ctx context.Context, name string, count int) error {
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
	stats[name] = count
	decoded["stats"] = stats
	decoded["lastModified"] = s.now().UTC().Format(time.RFC3339Nano)
	return s.saveJSON(ctx, profileKey, decoded)
}

func followerIDs(states map[string]followerState) []string {
	followers := make([]string, 0, len(states))
	for id, state := range states {
		if state.Following {
			followers = append(followers, id)
		}
	}
	sort.Strings(followers)
	return followers
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

func (s *Store) uniqueLocalSlugLocked(base string, posts []SignedPost) string {
	if !postSlugExists(base, posts) && !s.richSlugReservedLocked(base) {
		return base
	}
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s-%d", base, suffix)
		if !postSlugExists(candidate, posts) && !s.richSlugReservedLocked(candidate) {
			return candidate
		}
	}
}

func (s *Store) richSlugReservedLocked(slug string) bool {
	if !strings.HasPrefix(slug, "rich-") {
		return false
	}
	_, reserved := s.richRecords[strings.TrimPrefix(slug, "rich-")]
	return reserved
}

func postSlugExists(slug string, posts []SignedPost) bool {
	for _, post := range posts {
		if postSlug(post) == slug {
			return true
		}
	}
	return false
}

func legacyPostIDExists(id string, posts []SignedPost) bool {
	for _, post := range posts {
		var decoded struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(post.Post, &decoded) == nil && decoded.ID == id {
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
