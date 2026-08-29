package factory

import (
	"time"

	"github.com/larslarsen/bb-go/pb"
	"github.com/larslarsen/bb-go/repo"
)

func NewDisputeCaseRecord() *repo.DisputeCaseRecord {
	dispute := &repo.DisputeCaseRecord{
		BuyerContract:  NewDisputeableContract(),
		VendorContract: NewDisputeableContract(),
		Timestamp:      time.Now(),
		OrderState:     pb.OrderState_DISPUTED,
	}
	return dispute
}

func NewExpiredDisputeCaseRecord() *repo.DisputeCaseRecord {
	dispute := NewDisputeCaseRecord()
	dispute.Timestamp = time.Now().Add(-repo.ModeratorDisputeExpiry_lastInterval)
	return dispute
}
