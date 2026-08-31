package payment

import (
	"bytes"
	"encoding/json"
	"unicode/utf8"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

func SignRequest(key crypto.PrivKey, request PaymentRequestV1) (SignedObject, error) {
	if key == nil {
		return SignedObject{}, paymentError(CodeSignature, "missing signing key")
	}
	if err := validateRequestUTF8(request); err != nil {
		return SignedObject{}, err
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return SignedObject{}, schemaError("encoding payment request")
	}
	decoded, canonical, _, err := DecodePaymentRequest(raw)
	if err != nil {
		return SignedObject{}, err
	}
	signer, err := peer.IDFromPrivateKey(key)
	if err != nil {
		return SignedObject{}, paymentError(CodeSignature, "deriving signing identity")
	}
	if decoded.PayeePeerID != signer.String() {
		return SignedObject{}, paymentError(CodePayee, "signer is not the request payee")
	}
	return signCanonical(key, KindRequest, DomainSeparatorRequest, canonical)
}

func SignStatus(key crypto.PrivKey, status PaymentStatusEventV1) (SignedObject, error) {
	if key == nil {
		return SignedObject{}, paymentError(CodeSignature, "missing signing key")
	}
	if err := validateStatusUTF8(status); err != nil {
		return SignedObject{}, err
	}
	raw, err := json.Marshal(status)
	if err != nil {
		return SignedObject{}, schemaError("encoding payment status")
	}
	_, canonical, _, err := DecodePaymentStatus(raw)
	if err != nil {
		return SignedObject{}, err
	}
	return signCanonical(key, KindStatus, DomainSeparatorStatus, canonical)
}

func validateRequestUTF8(request PaymentRequestV1) error {
	values := []string{
		request.RequestID, request.PayerPeerID, request.PayeePeerID, request.Asset,
		request.Network, request.AmountAtomic, request.Receiver, request.ReceiverKind,
		request.Memo, request.Nonce, request.CreatedAt, request.ExpiresAt,
	}
	for _, value := range values {
		if !utf8.ValidString(value) {
			return schemaError("payment request contains invalid UTF-8")
		}
	}
	return nil
}

func validateStatusUTF8(status PaymentStatusEventV1) error {
	values := []string{status.RequestID, status.EventID, status.Nonce, status.Status, status.At, status.TxRef}
	for _, value := range values {
		if !utf8.ValidString(value) {
			return schemaError("payment status contains invalid UTF-8")
		}
	}
	return nil
}

func VerifySignedObject(signed SignedObject, remote, recipient peer.ID) (VerifiedObject, error) {
	if signed.Version != 1 || (signed.Kind != KindRequest && signed.Kind != KindStatus) {
		return VerifiedObject{}, schemaError("unsupported signed envelope version or kind")
	}
	canonicalCopy := []byte(signed.Canonical)
	verified := VerifiedObject{}
	var domain string
	switch signed.Kind {
	case KindRequest:
		request, canonical, digest, err := DecodePaymentRequest(canonicalCopy)
		if err != nil {
			return VerifiedObject{}, err
		}
		if err := CheckCanonicalCopy(canonical, canonicalCopy); err != nil {
			return VerifiedObject{}, err
		}
		verified.Request = &request
		verified.Digest = digest
		domain = DomainSeparatorRequest
	case KindStatus:
		status, canonical, digest, err := DecodePaymentStatus(canonicalCopy)
		if err != nil {
			return VerifiedObject{}, err
		}
		if err := CheckCanonicalCopy(canonical, canonicalCopy); err != nil {
			return VerifiedObject{}, err
		}
		verified.Status = &status
		verified.Digest = digest
		domain = DomainSeparatorStatus
	}
	publicKey, err := crypto.UnmarshalPublicKey(signed.PublicKey)
	if err != nil {
		return VerifiedObject{}, paymentError(CodeSignature, "malformed public key")
	}
	signer, err := peer.IDFromPublicKey(publicKey)
	if err != nil {
		return VerifiedObject{}, paymentError(CodeSignature, "invalid public key identity")
	}
	if signer != remote {
		return VerifiedObject{}, paymentError(CodeRemote, "public key does not match live remote peer")
	}
	message := make([]byte, 0, len(domain)+len(canonicalCopy))
	message = append(message, domain...)
	message = append(message, canonicalCopy...)
	valid, err := publicKey.Verify(message, signed.Signature)
	if err != nil || !valid {
		return VerifiedObject{}, paymentError(CodeSignature, "invalid payment signature")
	}
	verified.PeerID = signer
	if verified.Request != nil {
		if verified.Request.PayeePeerID != signer.String() {
			return VerifiedObject{}, paymentError(CodePayee, "request payee does not match signer")
		}
		if verified.Request.PayerPeerID != recipient.String() {
			return VerifiedObject{}, paymentError(CodePayer, "request payer does not match recipient")
		}
	}
	return verified, nil
}

func signCanonical(key crypto.PrivKey, kind Kind, domain string, canonical []byte) (SignedObject, error) {
	publicKey, err := crypto.MarshalPublicKey(key.GetPublic())
	if err != nil {
		return SignedObject{}, paymentError(CodeSignature, "marshalling public key")
	}
	message := make([]byte, 0, len(domain)+len(canonical))
	message = append(message, domain...)
	message = append(message, canonical...)
	signature, err := key.Sign(message)
	if err != nil {
		return SignedObject{}, paymentError(CodeSignature, "signing payment object")
	}
	return SignedObject{
		Version: 1, Kind: kind, Canonical: string(bytes.Clone(canonical)),
		PublicKey: publicKey, Signature: signature,
	}, nil
}
