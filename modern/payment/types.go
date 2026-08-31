// Package payment implements BitBook's signed, payer-bound payment objects.
package payment

import (
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

type Kind string

const (
	KindRequest Kind = "request"
	KindStatus  Kind = "status"
)

type Direction string

const (
	DirectionInbound  Direction = "inbound"
	DirectionOutbound Direction = "outbound"
)

const (
	StatusCancelled = "cancelled"
	StatusPaid      = "paid"
	StatusExpired   = "expired"

	DomainSeparatorRequest = "bitbook-payment-request-v1\n"
	DomainSeparatorStatus  = "bitbook-payment-status-v1\n"

	MaxFrameBytes = 64 << 10
)

const (
	CodeSchema    = "SCHEMA"
	CodeFrame     = "FRAME"
	CodeSignature = "SIGNATURE"
	CodeRemote    = "REMOTE"
	CodePayer     = "PAYER"
	CodePayee     = "PAYEE"
	CodeReplay    = "REPLAY"
	CodeLinkage   = "LINKAGE"
	CodeExpired   = "EXPIRED"
	CodeStatus    = "STATUS"
	CodeStorage   = "STORAGE"
)

type PaymentRequestV1 struct {
	V            int    `json:"v"`
	RequestID    string `json:"request_id"`
	PayerPeerID  string `json:"payer_peer_id"`
	PayeePeerID  string `json:"payee_peer_id"`
	Asset        string `json:"asset"`
	Network      string `json:"network"`
	AmountAtomic string `json:"amount_atomic"`
	Receiver     string `json:"receiver"`
	ReceiverKind string `json:"receiver_kind"`
	Memo         string `json:"memo"`
	Nonce        string `json:"nonce"`
	CreatedAt    string `json:"created_at"`
	ExpiresAt    string `json:"expires_at"`
}

type PaymentStatusEventV1 struct {
	V         int    `json:"v"`
	RequestID string `json:"request_id"`
	EventID   string `json:"event_id"`
	Nonce     string `json:"nonce"`
	Status    string `json:"status"`
	At        string `json:"at"`
	TxRef     string `json:"tx_ref"`
}

type SignedObject struct {
	Version   int    `json:"version"`
	Kind      Kind   `json:"kind"`
	Canonical string `json:"canonical"`
	PublicKey []byte `json:"public_key"`
	Signature []byte `json:"signature"`
}

type VerifiedObject struct {
	PeerID  peer.ID
	Request *PaymentRequestV1
	Status  *PaymentStatusEventV1
	Digest  string
}

type Acknowledgement struct {
	Accepted bool   `json:"accepted"`
	Digest   string `json:"digest"`
	Code     string `json:"code"`
}

type RecordedObject struct {
	Signed     SignedObject `json:"signed"`
	Digest     string       `json:"digest"`
	Direction  Direction    `json:"direction"`
	ReceivedAt time.Time    `json:"received_at"`
}

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return e.Code
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func paymentError(code, message string) error {
	return &Error{Code: code, Message: message}
}
