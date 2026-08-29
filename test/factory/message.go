package factory

import (
	"github.com/larslarsen/bb-go/pb"
	"github.com/larslarsen/bb-go/repo"
	"github.com/golang/protobuf/ptypes/any"
)

func NewMessageWithOrderPayload() repo.Message {
	payload := []byte("test payload")
	return repo.Message{
		Msg: pb.Message{
			MessageType: pb.Message_ORDER,
			Payload:     &any.Any{Value: payload},
		},
	}
}
