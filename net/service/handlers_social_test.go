package service

import (
	"testing"

	"github.com/larslarsen/bb-go/core"
	"github.com/larslarsen/bb-go/pb"
)

func TestSocialOnlyMessageType(t *testing.T) {
	tests := []struct {
		messageType pb.Message_MessageType
		allowed     bool
	}{
		{pb.Message_PING, true},
		{pb.Message_FOLLOW, true},
		{pb.Message_CHAT, true},
		{pb.Message_OFFLINE_RELAY, true},
		{pb.Message_ORDER, false},
		{pb.Message_DISPUTE_OPEN, false},
		{pb.Message_MODERATOR_ADD, false},
		{pb.Message_ORDER_PAYMENT, false},
	}

	for _, test := range tests {
		if got := socialOnlyMessageType(test.messageType); got != test.allowed {
			t.Errorf("socialOnlyMessageType(%s) = %t, want %t", test.messageType, got, test.allowed)
		}
	}
}

func TestSocialOnlyHandlerDoesNotDispatchMarketplaceMessages(t *testing.T) {
	service := &OpenBazaarService{
		node: &core.OpenBazaarNode{RuntimeMode: core.RuntimeModeSocial},
	}

	if handler := service.HandlerForMsgType(pb.Message_CHAT); handler == nil {
		t.Fatal("chat handler is disabled in social mode")
	}
	if handler := service.HandlerForMsgType(pb.Message_ORDER); handler != nil {
		t.Fatal("order handler is enabled in social mode")
	}
}
