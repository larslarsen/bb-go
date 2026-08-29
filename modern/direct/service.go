package direct

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ipfs/go-datastore"
	"github.com/larslarsen/bb-go/modern/network"
	"github.com/larslarsen/bb-go/modern/social"
	"github.com/libp2p/go-libp2p/core/crypto"
	lp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	maxFrameBytes       = 128 << 10
	maxSubjectRunes     = 500
	maxChatMessageRunes = 20_000
	deliveryTimeout     = 10 * time.Second
	maxFutureSkew       = 5 * time.Minute
)

var (
	messagesKey = datastore.NewKey("/bitbook/direct/messages")
	outboxKey   = datastore.NewKey("/bitbook/direct/outbox")
	seenPrefix  = datastore.NewKey("/bitbook/direct/seen")
)

type Service struct {
	node   *network.Node
	social *social.Store
	now    func() time.Time

	messagesMu sync.Mutex
	outboxMu   sync.Mutex
	eventsMu   sync.RWMutex
	subs       map[chan []byte]struct{}
	closeOnce  sync.Once
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

func NewService(node *network.Node, socialStore *social.Store) (*Service, error) {
	if node == nil {
		return nil, errors.New("nil network node")
	}
	if socialStore == nil {
		return nil, errors.New("nil social store")
	}
	serviceCtx, cancel := context.WithCancel(context.Background())
	service := &Service{
		node: node, social: socialStore, now: time.Now,
		subs: make(map[chan []byte]struct{}),
		ctx:  serviceCtx, cancel: cancel,
	}
	node.Host.SetStreamHandler(network.DirectProtocolCurrent, service.handleStream)
	return service, nil
}

func (s *Service) Close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		s.node.Host.RemoveStreamHandler(network.DirectProtocolCurrent)
		s.cancel()
		s.wg.Wait()
		s.eventsMu.Lock()
		for subscriber := range s.subs {
			close(subscriber)
		}
		s.subs = nil
		s.eventsMu.Unlock()
	})
}

// Subscribe returns desktop-compatible websocket payloads.
func (s *Service) Subscribe(buffer int) (<-chan []byte, func()) {
	if buffer < 1 {
		buffer = 1
	}
	channel := make(chan []byte, buffer)
	s.eventsMu.Lock()
	if s.subs != nil {
		s.subs[channel] = struct{}{}
	}
	s.eventsMu.Unlock()
	var once sync.Once
	return channel, func() {
		once.Do(func() {
			s.eventsMu.Lock()
			if _, exists := s.subs[channel]; exists {
				delete(s.subs, channel)
				close(channel)
			}
			s.eventsMu.Unlock()
		})
	}
}

func (s *Service) SendFollow(ctx context.Context, recipient peer.ID, following bool) (Delivery, error) {
	kind := KindFollow
	if !following {
		kind = KindUnfollow
	}
	envelope, err := s.newEnvelope(kind, recipient, "", "", "")
	if err != nil {
		return Delivery{}, err
	}
	return s.sendDurable(ctx, envelope)
}

func (s *Service) SendChat(ctx context.Context, recipient peer.ID, subject, message string) (ChatMessage, Delivery, error) {
	if recipient == s.node.ID() {
		return ChatMessage{}, Delivery{}, errors.New("cannot chat with self")
	}
	if utf8.RuneCountInString(subject) > maxSubjectRunes {
		return ChatMessage{}, Delivery{}, fmt.Errorf("subject exceeds %d characters", maxSubjectRunes)
	}
	if utf8.RuneCountInString(message) > maxChatMessageRunes {
		return ChatMessage{}, Delivery{}, fmt.Errorf("message exceeds %d characters", maxChatMessageRunes)
	}
	kind := KindChat
	if message == "" {
		kind = KindTyping
	}
	envelope, err := s.newEnvelope(kind, recipient, subject, message, "")
	if err != nil {
		return ChatMessage{}, Delivery{}, err
	}
	chat := ChatMessage{
		MessageID: envelope.ID, PeerID: recipient.String(), Subject: subject,
		Message: message, Read: false, Outgoing: true, Timestamp: envelope.Timestamp,
	}
	if kind == KindTyping {
		err := s.deliver(ctx, envelope)
		if err != nil {
			return chat, Delivery{Warning: err.Error()}, nil
		}
		return chat, Delivery{Delivered: true}, nil
	}
	if err := s.putMessage(ctx, chat); err != nil {
		return ChatMessage{}, Delivery{}, err
	}
	delivery, err := s.sendDurable(ctx, envelope)
	return chat, delivery, err
}

// MarkAsRead updates incoming messages and sends a durable read receipt.
func (s *Service) MarkAsRead(ctx context.Context, recipient peer.ID, subject string) (Delivery, error) {
	lastID, updated, err := s.markIncomingRead(ctx, recipient, subject)
	if err != nil || !updated {
		return Delivery{}, err
	}
	envelope, err := s.newEnvelope(KindRead, recipient, subject, "", lastID)
	if err != nil {
		return Delivery{}, err
	}
	return s.sendDurable(ctx, envelope)
}

func (s *Service) Messages(ctx context.Context, peerID, subject, offsetID string, limit int) ([]ChatMessage, error) {
	s.messagesMu.Lock()
	defer s.messagesMu.Unlock()
	messages, err := s.loadMessages(ctx)
	if err != nil {
		return nil, err
	}
	var before time.Time
	if offsetID != "" {
		for _, message := range messages {
			if message.MessageID == offsetID {
				before = message.Timestamp
				break
			}
		}
	}
	result := make([]ChatMessage, 0, len(messages))
	for _, message := range messages {
		if peerID != "" && message.PeerID != peerID {
			continue
		}
		if message.Subject != subject {
			continue
		}
		if !before.IsZero() && !message.Timestamp.Before(before) {
			continue
		}
		result = append(result, message)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Timestamp.After(result[j].Timestamp) })
	if limit >= 0 && len(result) > limit {
		result = result[:limit]
	}
	if result == nil {
		result = []ChatMessage{}
	}
	return result, nil
}

func (s *Service) Conversations(ctx context.Context) ([]Conversation, error) {
	s.messagesMu.Lock()
	defer s.messagesMu.Unlock()
	messages, err := s.loadMessages(ctx)
	if err != nil {
		return nil, err
	}
	byPeer := make(map[string]*Conversation)
	for _, message := range messages {
		if message.Subject != "" {
			continue
		}
		conversation := byPeer[message.PeerID]
		if conversation == nil {
			conversation = &Conversation{PeerID: message.PeerID}
			byPeer[message.PeerID] = conversation
		}
		if !message.Outgoing && !message.Read {
			conversation.Unread++
		}
		if conversation.Timestamp.IsZero() || message.Timestamp.After(conversation.Timestamp) {
			conversation.LastMessage = message.Message
			conversation.Timestamp = message.Timestamp
			conversation.Outgoing = message.Outgoing
		}
	}
	result := make([]Conversation, 0, len(byPeer))
	for _, conversation := range byPeer {
		result = append(result, *conversation)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Timestamp.After(result[j].Timestamp) })
	return result, nil
}

func (s *Service) DeleteMessage(ctx context.Context, messageID string) error {
	s.messagesMu.Lock()
	defer s.messagesMu.Unlock()
	messages, err := s.loadMessages(ctx)
	if err != nil {
		return err
	}
	filtered := messages[:0]
	for _, message := range messages {
		if message.MessageID != messageID {
			filtered = append(filtered, message)
		}
	}
	if len(filtered) == len(messages) {
		return datastore.ErrNotFound
	}
	return s.saveJSON(ctx, messagesKey, filtered)
}

func (s *Service) DeleteConversation(ctx context.Context, peerID string) error {
	s.messagesMu.Lock()
	defer s.messagesMu.Unlock()
	messages, err := s.loadMessages(ctx)
	if err != nil {
		return err
	}
	filtered := messages[:0]
	for _, message := range messages {
		if message.PeerID != peerID || message.Subject != "" {
			filtered = append(filtered, message)
		}
	}
	return s.saveJSON(ctx, messagesKey, filtered)
}

func (s *Service) Pending(ctx context.Context) (int, error) {
	s.outboxMu.Lock()
	defer s.outboxMu.Unlock()
	outbox, err := s.loadOutbox(ctx)
	return len(outbox), err
}

// RetryPending attempts every queued durable envelope once.
func (s *Service) RetryPending(ctx context.Context) (int, error) {
	s.outboxMu.Lock()
	defer s.outboxMu.Unlock()
	outbox, err := s.loadOutbox(ctx)
	if err != nil {
		return 0, err
	}
	remaining := make([]Envelope, 0, len(outbox))
	delivered := 0
	for _, envelope := range outbox {
		if err := s.deliver(ctx, envelope); err != nil {
			remaining = append(remaining, envelope)
			continue
		}
		delivered++
	}
	if err := s.saveJSON(ctx, outboxKey, remaining); err != nil {
		return delivered, err
	}
	return delivered, nil
}

func (s *Service) sendDurable(ctx context.Context, envelope Envelope) (Delivery, error) {
	if err := s.deliver(ctx, envelope); err == nil {
		return Delivery{Delivered: true}, nil
	} else {
		queueCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), deliveryTimeout)
		defer cancel()
		if queueErr := s.queue(queueCtx, envelope); queueErr != nil {
			return Delivery{}, errors.Join(err, queueErr)
		}
		return Delivery{Queued: true, Warning: err.Error()}, nil
	}
}

func (s *Service) queue(ctx context.Context, envelope Envelope) error {
	s.outboxMu.Lock()
	defer s.outboxMu.Unlock()
	outbox, err := s.loadOutbox(ctx)
	if err != nil {
		return err
	}
	for _, queued := range outbox {
		if queued.ID == envelope.ID {
			return nil
		}
	}
	outbox = append(outbox, envelope)
	return s.saveJSON(ctx, outboxKey, outbox)
}

func (s *Service) deliver(parent context.Context, envelope Envelope) error {
	ctx, cancel := context.WithTimeout(parent, deliveryTimeout)
	defer cancel()
	recipient, err := peer.Decode(envelope.Recipient)
	if err != nil {
		return err
	}
	if s.node.Host.Network().Connectedness(recipient) != lp2pnet.Connected {
		info, err := s.node.DHT.FindPeer(ctx, recipient)
		if err != nil {
			return fmt.Errorf("finding peer %s: %w", recipient, err)
		}
		if err := s.node.Connect(ctx, info); err != nil {
			return fmt.Errorf("connecting peer %s: %w", recipient, err)
		}
	}
	stream, err := s.node.Host.NewStream(ctx, recipient, network.DirectProtocolCurrent)
	if err != nil {
		return fmt.Errorf("opening direct stream: %w", err)
	}
	deadline := s.now().Add(deliveryTimeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	_ = stream.SetDeadline(deadline)
	if err := writeFrame(stream, envelope); err != nil {
		_ = stream.Reset()
		return fmt.Errorf("writing direct envelope: %w", err)
	}
	var ack acknowledgement
	if err := readFrame(stream, &ack); err != nil {
		_ = stream.Reset()
		return fmt.Errorf("reading direct acknowledgement: %w", err)
	}
	_ = stream.Close()
	if ack.ID != envelope.ID {
		return errors.New("direct acknowledgement ID mismatch")
	}
	if !ack.Accepted {
		return fmt.Errorf("direct message rejected: %s", ack.Error)
	}
	return nil
}

func (s *Service) handleStream(stream lp2pnet.Stream) {
	defer stream.Close()
	_ = stream.SetDeadline(s.now().Add(deliveryTimeout))
	var envelope Envelope
	if err := readFrame(stream, &envelope); err != nil {
		_ = writeFrame(stream, acknowledgement{Error: err.Error()})
		return
	}
	ack := acknowledgement{ID: envelope.ID}
	if err := s.accept(stream.Conn().RemotePeer(), envelope); err != nil {
		ack.Error = err.Error()
	} else {
		ack.Accepted = true
	}
	_ = writeFrame(stream, ack)
}

func (s *Service) accept(remote peer.ID, envelope Envelope) error {
	if err := s.verify(remote, envelope); err != nil {
		return err
	}
	if envelope.Kind != KindTyping {
		seen, err := s.seen(context.Background(), envelope.Sender, envelope.ID)
		if err != nil {
			return err
		}
		if seen {
			return nil
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), deliveryTimeout)
	defer cancel()
	sender, _ := peer.Decode(envelope.Sender)
	switch envelope.Kind {
	case KindFollow, KindUnfollow:
		changed, err := s.social.ApplyFollower(ctx, sender, envelope.Kind == KindFollow, envelope.Timestamp)
		if err != nil {
			return err
		}
		if changed {
			s.emitFollow(envelope)
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				s.publishFollowerState()
			}()
		}
	case KindChat:
		message := ChatMessage{
			MessageID: envelope.ID, PeerID: envelope.Sender, Subject: envelope.Subject,
			Message: envelope.Message, Timestamp: envelope.Timestamp,
		}
		if err := s.putMessage(ctx, message); err != nil {
			return err
		}
		s.emit(map[string]any{"message": message})
	case KindTyping:
		s.emit(map[string]any{"messageTyping": map[string]any{
			"messageId": envelope.ID, "peerId": envelope.Sender, "subject": envelope.Subject,
		}})
	case KindRead:
		if err := s.markOutgoingRead(ctx, sender, envelope.Subject, envelope.ReplyTo); err != nil {
			return err
		}
		s.emit(map[string]any{"messageRead": map[string]any{
			"messageId": envelope.ReplyTo, "peerId": envelope.Sender, "subject": envelope.Subject,
		}})
	default:
		return fmt.Errorf("unsupported direct message kind %q", envelope.Kind)
	}
	if envelope.Kind != KindTyping {
		return s.markSeen(ctx, envelope.Sender, envelope.ID)
	}
	return nil
}

func (s *Service) verify(remote peer.ID, envelope Envelope) error {
	if envelope.Version != EnvelopeVersion {
		return fmt.Errorf("unsupported direct envelope version %d", envelope.Version)
	}
	if envelope.ID == "" || len(envelope.ID) > 128 {
		return errors.New("invalid direct envelope ID")
	}
	if envelope.Recipient != s.node.ID().String() {
		return errors.New("direct envelope recipient mismatch")
	}
	if envelope.Timestamp.IsZero() || envelope.Timestamp.After(s.now().Add(maxFutureSkew)) {
		return errors.New("invalid direct envelope timestamp")
	}
	if utf8.RuneCountInString(envelope.Subject) > maxSubjectRunes {
		return errors.New("direct envelope subject is too long")
	}
	if utf8.RuneCountInString(envelope.Message) > maxChatMessageRunes {
		return errors.New("direct envelope message is too long")
	}
	publicKey, err := crypto.UnmarshalPublicKey(envelope.PublicKey)
	if err != nil {
		return fmt.Errorf("decoding direct public key: %w", err)
	}
	sender, err := peer.IDFromPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("deriving direct sender: %w", err)
	}
	if sender != remote || sender.String() != envelope.Sender {
		return errors.New("direct envelope sender mismatch")
	}
	canonical, err := json.Marshal(fieldsOf(envelope))
	if err != nil {
		return err
	}
	valid, err := publicKey.Verify(canonical, envelope.Signature)
	if err != nil {
		return err
	}
	if !valid {
		return errors.New("invalid direct envelope signature")
	}
	switch envelope.Kind {
	case KindFollow, KindUnfollow:
		if envelope.Subject != "" || envelope.Message != "" || envelope.ReplyTo != "" {
			return errors.New("follow envelope contains chat fields")
		}
	case KindChat:
		if envelope.Message == "" {
			return errors.New("chat envelope has no message")
		}
	case KindTyping:
		if envelope.Message != "" || envelope.ReplyTo != "" {
			return errors.New("typing envelope contains message data")
		}
	case KindRead:
		if envelope.ReplyTo == "" || envelope.Message != "" {
			return errors.New("read envelope is missing its message ID")
		}
	default:
		return fmt.Errorf("unsupported direct message kind %q", envelope.Kind)
	}
	return nil
}

func (s *Service) newEnvelope(kind Kind, recipient peer.ID, subject, message, replyTo string) (Envelope, error) {
	idBytes := make([]byte, 20)
	if _, err := rand.Read(idBytes); err != nil {
		return Envelope{}, err
	}
	publicKey, err := crypto.MarshalPublicKey(s.node.PrivateKey.GetPublic())
	if err != nil {
		return Envelope{}, err
	}
	envelope := Envelope{
		Version: EnvelopeVersion, Kind: kind, ID: hex.EncodeToString(idBytes),
		Sender: s.node.ID().String(), Recipient: recipient.String(),
		Subject: subject, Message: message, ReplyTo: replyTo,
		Timestamp: s.now().UTC(), PublicKey: publicKey,
	}
	canonical, err := json.Marshal(fieldsOf(envelope))
	if err != nil {
		return Envelope{}, err
	}
	envelope.Signature, err = s.node.PrivateKey.Sign(canonical)
	return envelope, err
}

func fieldsOf(envelope Envelope) signedFields {
	return signedFields{
		Version: envelope.Version, Kind: envelope.Kind, ID: envelope.ID,
		Sender: envelope.Sender, Recipient: envelope.Recipient,
		Subject: envelope.Subject, Message: envelope.Message, ReplyTo: envelope.ReplyTo,
		Timestamp: envelope.Timestamp,
	}
}

func (s *Service) putMessage(ctx context.Context, message ChatMessage) error {
	s.messagesMu.Lock()
	defer s.messagesMu.Unlock()
	messages, err := s.loadMessages(ctx)
	if err != nil {
		return err
	}
	for _, existing := range messages {
		if existing.MessageID == message.MessageID {
			return nil
		}
	}
	messages = append(messages, message)
	return s.saveJSON(ctx, messagesKey, messages)
}

func (s *Service) markIncomingRead(ctx context.Context, recipient peer.ID, subject string) (string, bool, error) {
	s.messagesMu.Lock()
	defer s.messagesMu.Unlock()
	messages, err := s.loadMessages(ctx)
	if err != nil {
		return "", false, err
	}
	updated := false
	var last ChatMessage
	for index := range messages {
		message := &messages[index]
		if message.PeerID != recipient.String() || message.Subject != subject || message.Outgoing {
			continue
		}
		if !message.Read {
			message.Read = true
			updated = true
		}
		if last.Timestamp.IsZero() || message.Timestamp.After(last.Timestamp) {
			last = *message
		}
	}
	if !updated {
		return "", false, nil
	}
	if err := s.saveJSON(ctx, messagesKey, messages); err != nil {
		return "", false, err
	}
	return last.MessageID, true, nil
}

func (s *Service) markOutgoingRead(ctx context.Context, sender peer.ID, subject, messageID string) error {
	s.messagesMu.Lock()
	defer s.messagesMu.Unlock()
	messages, err := s.loadMessages(ctx)
	if err != nil {
		return err
	}
	var through time.Time
	for _, message := range messages {
		if message.MessageID == messageID && message.PeerID == sender.String() && message.Outgoing {
			through = message.Timestamp
			break
		}
	}
	if through.IsZero() {
		return nil
	}
	for index := range messages {
		message := &messages[index]
		if message.PeerID == sender.String() && message.Subject == subject && message.Outgoing &&
			!message.Timestamp.After(through) {
			message.Read = true
		}
	}
	return s.saveJSON(ctx, messagesKey, messages)
}

func (s *Service) loadMessages(ctx context.Context) ([]ChatMessage, error) {
	var messages []ChatMessage
	if err := s.loadJSON(ctx, messagesKey, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func (s *Service) loadOutbox(ctx context.Context) ([]Envelope, error) {
	var outbox []Envelope
	if err := s.loadJSON(ctx, outboxKey, &outbox); err != nil {
		return nil, err
	}
	return outbox, nil
}

func (s *Service) loadJSON(ctx context.Context, key datastore.Key, target any) error {
	raw, err := s.node.Datastore.Get(ctx, key)
	if errors.Is(err, datastore.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func (s *Service) saveJSON(ctx context.Context, key datastore.Key, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.node.Datastore.Put(ctx, key, raw)
}

func (s *Service) seen(ctx context.Context, sender, id string) (bool, error) {
	return s.node.Datastore.Has(ctx, seenPrefix.ChildString(sender).ChildString(id))
}

func (s *Service) markSeen(ctx context.Context, sender, id string) error {
	return s.node.Datastore.Put(ctx, seenPrefix.ChildString(sender).ChildString(id), []byte{1})
}

func (s *Service) publishFollowerState() {
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()
	root, err := s.social.Commit(ctx)
	if err != nil || len(s.node.Host.Network().Peers()) == 0 {
		return
	}
	_ = s.social.PublishRoot(ctx, root)
}

func (s *Service) emitFollow(envelope Envelope) {
	s.emit(map[string]any{"notification": map[string]any{
		"notificationId": envelope.ID,
		"type":           string(envelope.Kind),
		"peerId":         envelope.Sender,
		"timestamp":      envelope.Timestamp,
	}})
}

func (s *Service) emit(value any) {
	raw, err := json.Marshal(value)
	if err != nil {
		return
	}
	s.eventsMu.RLock()
	defer s.eventsMu.RUnlock()
	for subscriber := range s.subs {
		select {
		case subscriber <- raw:
		default:
		}
	}
}

func writeFrame(writer io.Writer, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(raw) > maxFrameBytes {
		return fmt.Errorf("direct frame exceeds %d bytes", maxFrameBytes)
	}
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(raw)))
	if err := writeAll(writer, length[:]); err != nil {
		return err
	}
	return writeAll(writer, raw)
}

func writeAll(writer io.Writer, raw []byte) error {
	for len(raw) > 0 {
		written, err := writer.Write(raw)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		raw = raw[written:]
	}
	return nil
}

func readFrame(reader io.Reader, target any) error {
	var length [4]byte
	if _, err := io.ReadFull(reader, length[:]); err != nil {
		return err
	}
	size := binary.BigEndian.Uint32(length[:])
	if size == 0 || size > maxFrameBytes {
		return fmt.Errorf("invalid direct frame size %d", size)
	}
	raw := make([]byte, size)
	if _, err := io.ReadFull(reader, raw); err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return err
	}
	return nil
}
