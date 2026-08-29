// Package direct implements BitBook's signed peer-to-peer social messages.
package direct

import "time"

const EnvelopeVersion = 1

type Kind string

const (
	KindFollow   Kind = "follow"
	KindUnfollow Kind = "unfollow"
	KindChat     Kind = "chat"
	KindTyping   Kind = "typing"
	KindRead     Kind = "read"
)

// Envelope is the versioned, signed unit carried by the BitBook direct stream.
// The libp2p connection authenticates the live peer while the signature keeps
// queued messages independently verifiable.
type Envelope struct {
	Version   int       `json:"version"`
	Kind      Kind      `json:"kind"`
	ID        string    `json:"id"`
	Sender    string    `json:"sender"`
	Recipient string    `json:"recipient"`
	Subject   string    `json:"subject,omitempty"`
	Message   string    `json:"message,omitempty"`
	ReplyTo   string    `json:"replyTo,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	PublicKey []byte    `json:"publicKey"`
	Signature []byte    `json:"signature"`
}

type signedFields struct {
	Version   int       `json:"version"`
	Kind      Kind      `json:"kind"`
	ID        string    `json:"id"`
	Sender    string    `json:"sender"`
	Recipient string    `json:"recipient"`
	Subject   string    `json:"subject,omitempty"`
	Message   string    `json:"message,omitempty"`
	ReplyTo   string    `json:"replyTo,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type acknowledgement struct {
	ID       string `json:"id"`
	Accepted bool   `json:"accepted"`
	Error    string `json:"error,omitempty"`
}

// ChatMessage retains the historical desktop JSON shape.
type ChatMessage struct {
	MessageID string    `json:"messageId"`
	PeerID    string    `json:"peerId"`
	Subject   string    `json:"subject"`
	Message   string    `json:"message"`
	Read      bool      `json:"read"`
	Outgoing  bool      `json:"outgoing"`
	Timestamp time.Time `json:"timestamp"`
}

type Conversation struct {
	PeerID      string    `json:"peerId"`
	Unread      int       `json:"unread"`
	LastMessage string    `json:"lastMessage"`
	Timestamp   time.Time `json:"timestamp"`
	Outgoing    bool      `json:"outgoing"`
}

// Delivery distinguishes an acknowledged live delivery from a durable queued
// message. Queued follows and chats are retried by the daemon.
type Delivery struct {
	Delivered bool   `json:"delivered"`
	Queued    bool   `json:"queued"`
	Warning   string `json:"warning,omitempty"`
}
