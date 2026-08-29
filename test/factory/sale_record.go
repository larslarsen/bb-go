package factory

import (
	"github.com/larslarsen/bb-go/repo"
)

func NewSaleRecord() *repo.SaleRecord {
	contract := NewContract()
	return &repo.SaleRecord{
		Contract: contract,
		OrderID:  "anOrderIDforaSaleRecord",
	}
}
