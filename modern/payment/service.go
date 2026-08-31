package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/larslarsen/bb-go/modern/network"
	"github.com/libp2p/go-libp2p/core/crypto"
	lp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

const paymentDeliveryTimeout = 10 * time.Second

var paymentRecordsKey = datastore.NewKey("/bitbook/payment/records")

type serviceConfig struct {
	now   func() time.Time
	store datastore.Batching
}

type ServiceOption func(*serviceConfig) error

func WithClock(now func() time.Time) ServiceOption {
	return func(config *serviceConfig) error {
		if now == nil {
			return errors.New("nil payment clock")
		}
		config.now = now
		return nil
	}
}

func WithDatastore(store datastore.Batching) ServiceOption {
	return func(config *serviceConfig) error {
		if store == nil {
			return errors.New("nil payment datastore")
		}
		config.store = store
		return nil
	}
}

type Service struct {
	node  *network.Node
	store datastore.Batching
	now   func() time.Time

	recordsMu sync.Mutex

	lifecycleMu sync.Mutex
	closed      bool
	handlers    sync.WaitGroup
	closeOnce   sync.Once
}

func NewService(node *network.Node, options ...ServiceOption) (*Service, error) {
	if node == nil || node.Host == nil || node.PrivateKey == nil || node.Datastore == nil {
		return nil, errors.New("payment service requires a complete network node")
	}
	config := serviceConfig{now: time.Now, store: node.Datastore}
	for _, option := range options {
		if option == nil {
			return nil, errors.New("nil payment service option")
		}
		if err := option(&config); err != nil {
			return nil, err
		}
	}
	service := &Service{node: node, store: config.store, now: config.now}
	node.Host.SetStreamHandler(network.PaymentProtocolCurrent, service.handleStream)
	return service, nil
}

func (s *Service) Close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		s.lifecycleMu.Lock()
		s.closed = true
		s.node.Host.RemoveStreamHandler(network.PaymentProtocolCurrent)
		s.lifecycleMu.Unlock()
		s.handlers.Wait()
	})
}

func (s *Service) SendRequest(ctx context.Context, request PaymentRequestV1) (Acknowledgement, error) {
	signed, err := SignRequest(s.node.PrivateKey, request)
	if err != nil {
		return Acknowledgement{}, err
	}
	recipient, err := peer.Decode(request.PayerPeerID)
	if err != nil {
		return Acknowledgement{}, paymentError(CodePayer, "request payer is not a libp2p peer ID")
	}
	return s.SendSigned(ctx, recipient, signed)
}

func (s *Service) SendStatus(ctx context.Context, status PaymentStatusEventV1) (Acknowledgement, error) {
	if err := validateStatusUTF8(status); err != nil {
		return Acknowledgement{}, err
	}
	raw, err := json.Marshal(status)
	if err != nil {
		return Acknowledgement{}, schemaError("encoding payment status")
	}
	decoded, _, _, err := DecodePaymentStatus(raw)
	if err != nil {
		return Acknowledgement{}, err
	}
	if decoded.Status != StatusCancelled || decoded.TxRef != "" {
		return Acknowledgement{}, paymentError(CodeStatus, "v1 network transport permits cancellation only")
	}
	record, err := s.GetRequest(ctx, decoded.RequestID)
	if err != nil {
		if isStorageError(err) {
			return Acknowledgement{}, err
		}
		return Acknowledgement{}, paymentError(CodeLinkage, "referenced payment request is not stored locally")
	}
	request, _, _, err := DecodePaymentRequest([]byte(record.Signed.Canonical))
	if err != nil {
		return Acknowledgement{}, paymentError(CodeStorage, "stored request is corrupt")
	}
	if decoded.Nonce == request.Nonce {
		return Acknowledgement{}, paymentError(CodeReplay, "status nonce reuses the linked request nonce")
	}
	if request.PayeePeerID != s.node.ID().String() {
		return Acknowledgement{}, paymentError(CodePayee, "local node is not the request payee")
	}
	recipient, err := peer.Decode(request.PayerPeerID)
	if err != nil {
		return Acknowledgement{}, paymentError(CodePayer, "stored request payer is not a libp2p peer ID")
	}
	signed, err := SignStatus(s.node.PrivateKey, decoded)
	if err != nil {
		return Acknowledgement{}, err
	}
	return s.SendSigned(ctx, recipient, signed)
}

func (s *Service) SendSigned(ctx context.Context, recipient peer.ID, signed SignedObject) (Acknowledgement, error) {
	if err := s.requireOpen(); err != nil {
		return Acknowledgement{}, err
	}
	verified, err := VerifySignedObject(signed, s.node.ID(), recipient)
	if err != nil {
		return Acknowledgement{}, err
	}
	if verified.Status != nil && (verified.Status.Status != StatusCancelled || verified.Status.TxRef != "") {
		return Acknowledgement{}, paymentError(CodeStatus, "v1 network transport permits cancellation only")
	}
	payload, err := json.Marshal(signed)
	if err != nil {
		return Acknowledgement{}, schemaError("encoding signed payment envelope")
	}
	stream, err := s.node.Host.NewStream(ctx, recipient, network.PaymentProtocolCurrent)
	if err != nil {
		return Acknowledgement{}, paymentError(CodeFrame, "opening payment stream")
	}
	defer stream.Close()
	_ = stream.SetDeadline(streamDeadline(ctx))
	if err := WriteFrame(stream, payload); err != nil {
		_ = stream.Reset()
		return Acknowledgement{}, err
	}
	if err := stream.CloseWrite(); err != nil {
		_ = stream.Reset()
		return Acknowledgement{}, paymentError(CodeFrame, "half-closing payment request stream")
	}
	ackPayload, err := ReadFrame(stream)
	if err != nil {
		_ = stream.Reset()
		return Acknowledgement{}, err
	}
	ack, err := decodeAcknowledgement(ackPayload)
	if err != nil {
		_ = stream.Reset()
		return Acknowledgement{}, err
	}
	if !ack.Accepted {
		if !knownPaymentCode(ack.Code) {
			return ack, paymentError(CodeFrame, "payment rejection has no recognized stable code")
		}
		return ack, paymentError(ack.Code, "remote rejected payment object")
	}
	if ack.Code != "" || ack.Digest == "" || ack.Digest != verified.Digest {
		return ack, paymentError(CodeFrame, "payment acknowledgement is inconsistent")
	}
	if err := s.storeOutbound(ctx, signed, verified); err != nil {
		return ack, err
	}
	return ack, nil
}

func (s *Service) GetRequest(ctx context.Context, requestID string) (RecordedObject, error) {
	s.recordsMu.Lock()
	defer s.recordsMu.Unlock()
	records, err := s.loadRecordsLocked(ctx)
	if err != nil {
		return RecordedObject{}, err
	}
	for _, record := range records {
		if record.Signed.Kind != KindRequest {
			continue
		}
		request, _, _, err := DecodePaymentRequest([]byte(record.Signed.Canonical))
		if err != nil {
			return RecordedObject{}, paymentError(CodeStorage, "stored request is corrupt")
		}
		if request.RequestID == requestID {
			return cloneRecord(record), nil
		}
	}
	return RecordedObject{}, datastore.ErrNotFound
}

func (s *Service) GetEvent(ctx context.Context, eventID string) (RecordedObject, error) {
	s.recordsMu.Lock()
	defer s.recordsMu.Unlock()
	records, err := s.loadRecordsLocked(ctx)
	if err != nil {
		return RecordedObject{}, err
	}
	for _, record := range records {
		if record.Signed.Kind != KindStatus {
			continue
		}
		status, _, _, err := DecodePaymentStatus([]byte(record.Signed.Canonical))
		if err != nil {
			return RecordedObject{}, paymentError(CodeStorage, "stored status is corrupt")
		}
		if status.EventID == eventID {
			return cloneRecord(record), nil
		}
	}
	return RecordedObject{}, datastore.ErrNotFound
}

func (s *Service) List(ctx context.Context) ([]RecordedObject, error) {
	s.recordsMu.Lock()
	defer s.recordsMu.Unlock()
	records, err := s.loadRecordsLocked(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]RecordedObject, len(records))
	for index := range records {
		result[index] = cloneRecord(records[index])
	}
	return result, nil
}

func (s *Service) handleStream(stream lp2pnet.Stream) {
	if !s.admitHandler() {
		_ = stream.Reset()
		return
	}
	defer s.handlers.Done()
	defer stream.Close()
	_ = stream.SetDeadline(time.Now().Add(paymentDeliveryTimeout))
	signed, err := ReceiveSignedObject(stream)
	ack := Acknowledgement{}
	if err != nil {
		ack.Code = errorCode(err, CodeFrame)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), paymentDeliveryTimeout)
		ack, err = s.acceptInbound(ctx, stream.Conn().RemotePeer(), signed)
		cancel()
		if err != nil {
			ack = Acknowledgement{Code: errorCode(err, CodeStorage)}
		}
	}
	payload, encodeErr := json.Marshal(ack)
	if encodeErr != nil || WriteFrame(stream, payload) != nil || stream.CloseWrite() != nil {
		_ = stream.Reset()
	}
}

func (s *Service) acceptInbound(ctx context.Context, remote peer.ID, signed SignedObject) (Acknowledgement, error) {
	verified, err := VerifySignedObject(signed, remote, s.node.ID())
	if err != nil {
		return Acknowledgement{}, err
	}
	s.recordsMu.Lock()
	defer s.recordsMu.Unlock()
	records, err := s.loadRecordsLocked(ctx)
	if err != nil {
		return Acknowledgement{}, err
	}
	if verified.Request != nil {
		for _, record := range records {
			if record.Signed.Kind != KindRequest {
				continue
			}
			existing, _, _, err := DecodePaymentRequest([]byte(record.Signed.Canonical))
			if err != nil {
				return Acknowledgement{}, paymentError(CodeStorage, "stored request is corrupt")
			}
			if err := CheckRequestReplay(existing, record.Digest, *verified.Request, verified.Digest); err != nil {
				return Acknowledgement{}, err
			}
			if record.Digest == verified.Digest {
				return Acknowledgement{Accepted: true, Digest: verified.Digest}, nil
			}
		}
		expires, err := parseTimestamp(verified.Request.ExpiresAt)
		if err != nil {
			return Acknowledgement{}, err
		}
		if !s.now().Before(expires) {
			return Acknowledgement{}, paymentError(CodeExpired, "payment request has expired")
		}
	} else {
		request, err := linkedRequest(records, verified.Status.RequestID)
		if err != nil {
			return Acknowledgement{}, err
		}
		if request.PayeePeerID != verified.PeerID.String() {
			return Acknowledgement{}, paymentError(CodePayee, "status signer is not the request payee")
		}
		if verified.Status.Nonce == request.Nonce {
			return Acknowledgement{}, paymentError(CodeReplay, "status nonce reuses the linked request nonce")
		}
		for _, record := range records {
			if record.Signed.Kind != KindStatus {
				continue
			}
			existing, _, _, err := DecodePaymentStatus([]byte(record.Signed.Canonical))
			if err != nil {
				return Acknowledgement{}, paymentError(CodeStorage, "stored status is corrupt")
			}
			if err := CheckStatusReplay(existing, record.Digest, *verified.Status, verified.Digest); err != nil {
				return Acknowledgement{}, err
			}
			if record.Digest == verified.Digest {
				return Acknowledgement{Accepted: true, Digest: verified.Digest}, nil
			}
		}
		if verified.Status.Status != StatusCancelled || verified.Status.TxRef != "" {
			return Acknowledgement{}, paymentError(CodeStatus, "v1 network transport permits cancellation only")
		}
		for _, record := range records {
			if record.Signed.Kind != KindStatus {
				continue
			}
			existing, _, _, err := DecodePaymentStatus([]byte(record.Signed.Canonical))
			if err != nil {
				return Acknowledgement{}, paymentError(CodeStorage, "stored status is corrupt")
			}
			if existing.RequestID == verified.Status.RequestID {
				return Acknowledgement{}, paymentError(CodeStatus, "payment request already has a terminal status")
			}
		}
	}
	record := RecordedObject{
		Signed: cloneStoredSignedObject(signed), Digest: verified.Digest,
		Direction: DirectionInbound, ReceivedAt: s.now().UTC(),
	}
	updated := append(cloneRecords(records), record)
	if err := s.saveRecordsLocked(ctx, updated); err != nil {
		return Acknowledgement{}, err
	}
	return Acknowledgement{Accepted: true, Digest: verified.Digest}, nil
}

func (s *Service) storeOutbound(ctx context.Context, signed SignedObject, verified VerifiedObject) error {
	s.recordsMu.Lock()
	defer s.recordsMu.Unlock()
	records, err := s.loadRecordsLocked(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		if record.Digest == verified.Digest {
			return nil
		}
		if verified.Status != nil && record.Signed.Kind == KindRequest {
			existing, _, _, _ := DecodePaymentRequest([]byte(record.Signed.Canonical))
			if existing.RequestID == verified.Status.RequestID && existing.Nonce == verified.Status.Nonce {
				return paymentError(CodeReplay, "status nonce reuses the linked request nonce")
			}
		}
		if verified.Request != nil && record.Signed.Kind == KindRequest {
			existing, _, _, _ := DecodePaymentRequest([]byte(record.Signed.Canonical))
			if err := CheckRequestReplay(existing, record.Digest, *verified.Request, verified.Digest); err != nil {
				return err
			}
		}
		if verified.Status != nil && record.Signed.Kind == KindStatus {
			existing, _, _, _ := DecodePaymentStatus([]byte(record.Signed.Canonical))
			if err := CheckStatusReplay(existing, record.Digest, *verified.Status, verified.Digest); err != nil {
				return err
			}
			if existing.RequestID == verified.Status.RequestID {
				return paymentError(CodeStatus, "payment request already has a terminal status")
			}
		}
	}
	record := RecordedObject{
		Signed: cloneStoredSignedObject(signed), Digest: verified.Digest,
		Direction: DirectionOutbound, ReceivedAt: s.now().UTC(),
	}
	return s.saveRecordsLocked(ctx, append(cloneRecords(records), record))
}

func (s *Service) loadRecordsLocked(ctx context.Context) ([]RecordedObject, error) {
	raw, err := s.store.Get(ctx, paymentRecordsKey)
	if errors.Is(err, datastore.ErrNotFound) {
		return []RecordedObject{}, nil
	}
	if err != nil {
		return nil, paymentError(CodeStorage, "loading payment records")
	}
	if _, err := parseStrictJSON(raw); err != nil {
		return nil, paymentError(CodeStorage, "payment record JSON is corrupt")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var records []RecordedObject
	if err := decoder.Decode(&records); err != nil || records == nil {
		return nil, paymentError(CodeStorage, "payment record collection is corrupt")
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, paymentError(CodeStorage, "payment record collection has trailing data")
	}
	if err := validateStoredRecords(records, s.node.ID()); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *Service) saveRecordsLocked(ctx context.Context, records []RecordedObject) error {
	if err := validateStoredRecords(records, s.node.ID()); err != nil {
		return err
	}
	raw, err := json.Marshal(records)
	if err != nil {
		return paymentError(CodeStorage, "encoding payment records")
	}
	if err := s.store.Put(ctx, paymentRecordsKey, raw); err != nil {
		return paymentError(CodeStorage, "persisting payment records")
	}
	return nil
}

func validateStoredRecords(records []RecordedObject, local peer.ID) error {
	type storedStatusMetadata struct {
		status    PaymentStatusEventV1
		signer    peer.ID
		direction Direction
	}
	requests := make(map[string]PaymentRequestV1, len(records))
	requestNonces := make(map[string]struct{}, len(records))
	statusEventIDs := make(map[string]struct{}, len(records))
	statusNonces := make(map[string]struct{}, len(records))
	terminalRequests := make(map[string]struct{}, len(records))
	seenDigests := make(map[string]struct{}, len(records))
	statuses := make([]storedStatusMetadata, 0, len(records))
	for _, record := range records {
		if (record.Direction != DirectionInbound && record.Direction != DirectionOutbound) || record.ReceivedAt.IsZero() {
			return paymentError(CodeStorage, "payment record metadata is corrupt")
		}
		if _, duplicate := seenDigests[record.Digest]; duplicate {
			return paymentError(CodeStorage, "payment record collection contains a duplicate digest")
		}
		seenDigests[record.Digest] = struct{}{}
		signer, domain, err := validateStoredSignedObject(record.Signed, record.Digest)
		if err != nil {
			return err
		}
		if domain == DomainSeparatorRequest {
			request, _, _, _ := DecodePaymentRequest([]byte(record.Signed.Canonical))
			if request.PayeePeerID != signer.String() {
				return paymentError(CodeStorage, "stored request signer is corrupt")
			}
			if (record.Direction == DirectionInbound && request.PayerPeerID != local.String()) ||
				(record.Direction == DirectionOutbound && request.PayeePeerID != local.String()) {
				return paymentError(CodeStorage, "stored request direction binding is corrupt")
			}
			if _, duplicate := requests[request.RequestID]; duplicate {
				return paymentError(CodeStorage, "stored request IDs conflict")
			}
			if _, duplicate := requestNonces[request.Nonce]; duplicate {
				return paymentError(CodeStorage, "stored request nonces conflict")
			}
			requests[request.RequestID] = request
			requestNonces[request.Nonce] = struct{}{}
		} else {
			status, _, _, _ := DecodePaymentStatus([]byte(record.Signed.Canonical))
			if status.Status != StatusCancelled || status.TxRef != "" {
				return paymentError(CodeStorage, "stored status violates the v1 network policy")
			}
			if _, duplicate := statusEventIDs[status.EventID]; duplicate {
				return paymentError(CodeStorage, "stored status event IDs conflict")
			}
			if _, duplicate := statusNonces[status.Nonce]; duplicate {
				return paymentError(CodeStorage, "stored status nonces conflict")
			}
			if _, duplicate := terminalRequests[status.RequestID]; duplicate {
				return paymentError(CodeStorage, "stored terminal statuses conflict")
			}
			statusEventIDs[status.EventID] = struct{}{}
			statusNonces[status.Nonce] = struct{}{}
			terminalRequests[status.RequestID] = struct{}{}
			statuses = append(statuses, storedStatusMetadata{
				status: status, signer: signer, direction: record.Direction,
			})
		}
	}
	for _, metadata := range statuses {
		request, ok := requests[metadata.status.RequestID]
		if !ok {
			if metadata.direction == DirectionInbound {
				return paymentError(CodeStorage, "stored inbound status has no linked request")
			}
		}
		if ok && request.PayeePeerID != metadata.signer.String() {
			return paymentError(CodeStorage, "stored status signer is corrupt")
		}
		if ok && metadata.status.Nonce == request.Nonce {
			return paymentError(CodeStorage, "stored status reuses its linked request nonce")
		}
		if (metadata.direction == DirectionInbound && ok && request.PayerPeerID != local.String()) ||
			(metadata.direction == DirectionOutbound && metadata.signer != local) {
			return paymentError(CodeStorage, "stored status direction binding is corrupt")
		}
	}
	return nil
}

func validateStoredSignedObject(signed SignedObject, digest string) (peer.ID, string, error) {
	if signed.Version != 1 || (signed.Kind != KindRequest && signed.Kind != KindStatus) {
		return "", "", paymentError(CodeStorage, "stored envelope metadata is corrupt")
	}
	var canonical []byte
	var expectedDigest, domain string
	switch signed.Kind {
	case KindRequest:
		_, canonical, expectedDigest, _ = DecodePaymentRequest([]byte(signed.Canonical))
		domain = DomainSeparatorRequest
	case KindStatus:
		_, canonical, expectedDigest, _ = DecodePaymentStatus([]byte(signed.Canonical))
		domain = DomainSeparatorStatus
	}
	if canonical == nil || CheckCanonicalCopy(canonical, []byte(signed.Canonical)) != nil || expectedDigest != digest {
		return "", "", paymentError(CodeStorage, "stored canonical object or digest is corrupt")
	}
	publicKey, err := crypto.UnmarshalPublicKey(signed.PublicKey)
	if err != nil {
		return "", "", paymentError(CodeStorage, "stored public key is corrupt")
	}
	signer, err := peer.IDFromPublicKey(publicKey)
	if err != nil {
		return "", "", paymentError(CodeStorage, "stored signer identity is corrupt")
	}
	message := append([]byte(domain), canonical...)
	valid, err := publicKey.Verify(message, signed.Signature)
	if err != nil || !valid {
		return "", "", paymentError(CodeStorage, "stored signature is corrupt")
	}
	return signer, domain, nil
}

func linkedRequest(records []RecordedObject, requestID string) (PaymentRequestV1, error) {
	for _, record := range records {
		if record.Signed.Kind != KindRequest {
			continue
		}
		request, _, _, err := DecodePaymentRequest([]byte(record.Signed.Canonical))
		if err != nil {
			return PaymentRequestV1{}, paymentError(CodeStorage, "stored request is corrupt")
		}
		if request.RequestID == requestID {
			return request, nil
		}
	}
	return PaymentRequestV1{}, paymentError(CodeLinkage, "status references an unknown payment request")
}

func decodeAcknowledgement(raw []byte) (Acknowledgement, error) {
	value, err := parseStrictJSONAllowBoolean(raw)
	if err != nil {
		return Acknowledgement{}, err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return Acknowledgement{}, schemaError("payment acknowledgement must be an object")
	}
	if err := requireFields(object, []string{"accepted", "digest", "code"}); err != nil {
		return Acknowledgement{}, err
	}
	accepted, ok := object["accepted"].(bool)
	if !ok {
		return Acknowledgement{}, schemaError("acknowledgement accepted must be Boolean")
	}
	digest, digestOK := object["digest"].(string)
	code, codeOK := object["code"].(string)
	if !digestOK || !codeOK {
		return Acknowledgement{}, schemaError("acknowledgement digest and code must be strings")
	}
	return Acknowledgement{Accepted: accepted, Digest: digest, Code: code}, nil
}

func (s *Service) admitHandler() bool {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.closed {
		return false
	}
	s.handlers.Add(1)
	return true
}

func (s *Service) requireOpen() error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.closed {
		return paymentError(CodeFrame, "payment service is closed")
	}
	return nil
}

func streamDeadline(ctx context.Context) time.Time {
	deadline := time.Now().Add(paymentDeliveryTimeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		return contextDeadline
	}
	return deadline
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("trailing JSON data")
	}
	return nil
}

func cloneStoredSignedObject(signed SignedObject) SignedObject {
	signed.PublicKey = bytes.Clone(signed.PublicKey)
	signed.Signature = bytes.Clone(signed.Signature)
	return signed
}

func cloneRecord(record RecordedObject) RecordedObject {
	record.Signed = cloneStoredSignedObject(record.Signed)
	return record
}

func cloneRecords(records []RecordedObject) []RecordedObject {
	result := make([]RecordedObject, len(records))
	for index := range records {
		result[index] = cloneRecord(records[index])
	}
	return result
}

func errorCode(err error, fallback string) string {
	var paymentErr *Error
	if errors.As(err, &paymentErr) && paymentErr.Code != "" {
		return paymentErr.Code
	}
	return fallback
}

func isStorageError(err error) bool {
	var paymentErr *Error
	return errors.As(err, &paymentErr) && paymentErr.Code == CodeStorage
}

func knownPaymentCode(code string) bool {
	switch code {
	case CodeSchema, CodeFrame, CodeSignature, CodeRemote, CodePayer, CodePayee,
		CodeReplay, CodeLinkage, CodeExpired, CodeStatus, CodeStorage:
		return true
	default:
		return false
	}
}
