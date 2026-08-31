package network

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/protocol"
)

func TestPaymentProtocolCurrent(t *testing.T) {
	const want protocol.ID = "/bitbook/payment/1.0.0"
	if PaymentProtocolCurrent != want {
		t.Fatalf("PaymentProtocolCurrent = %q, want %q", PaymentProtocolCurrent, want)
	}
	existing := []protocol.ID{DHTProtocolCurrent, BitswapProtocolCurrent, DirectProtocolCurrent}
	for _, id := range existing {
		if PaymentProtocolCurrent == id {
			t.Fatalf("PaymentProtocolCurrent reused %q", id)
		}
	}
}
