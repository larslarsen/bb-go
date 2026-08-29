package factory

import (
	"time"

	"github.com/larslarsen/bb-go/repo"
)

func NewAPITime(t time.Time) *repo.APITime {
	return repo.NewAPITime(t)
}
