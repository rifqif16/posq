package domain

import (
	"time"

	"github.com/google/uuid"
)

type PriceChange struct {
	ID            uuid.UUID
	VariantID     uuid.UUID
	VariantName   string
	Field         string
	OldValue      int64
	NewValue      int64
	ChangedByID   uuid.UUID
	ChangedByName string
	At            time.Time
}
