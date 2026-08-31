package payment

func CheckRequestReplay(first PaymentRequestV1, firstDigest string, second PaymentRequestV1, secondDigest string) error {
	if firstDigest == secondDigest {
		return nil
	}
	if first.RequestID == second.RequestID || first.Nonce == second.Nonce {
		return paymentError(CodeReplay, "request ID or nonce was reused with different signed bytes")
	}
	return nil
}

func CheckStatusReplay(first PaymentStatusEventV1, firstDigest string, second PaymentStatusEventV1, secondDigest string) error {
	if firstDigest == secondDigest {
		return nil
	}
	if first.EventID == second.EventID || first.Nonce == second.Nonce {
		return paymentError(CodeReplay, "status event ID or nonce was reused with different signed bytes")
	}
	return nil
}
