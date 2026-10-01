package application

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/catalog/domain"
)

type PriceHistoryFilter struct {
	ProductID uuid.UUID
	Limit     int
	AfterAt   *time.Time
	AfterID   *uuid.UUID
}

type PriceHistoryPage struct {
	Items   []domain.PriceChange
	HasMore bool
}

type PriceHistoryList struct {
	Items      []domain.PriceChange
	NextCursor string
}

type PriceHistoryRepository interface {
	ListPriceHistory(ctx context.Context, tenantID uuid.UUID, f PriceHistoryFilter) (PriceHistoryPage, error)
}

type PriceHistoryService struct {
	repo PriceHistoryRepository
}

func NewPriceHistoryService(repo PriceHistoryRepository) *PriceHistoryService {
	return &PriceHistoryService{repo: repo}
}

func (s *PriceHistoryService) List(ctx context.Context, a Actor, productID uuid.UUID, limit int, cursor string) (PriceHistoryList, error) {
	f := PriceHistoryFilter{ProductID: productID, Limit: clampLimit(limit)}
	if cursor != "" {
		at, id, err := decodeTimeCursor(cursor)
		if err != nil {
			return PriceHistoryList{}, err
		}
		f.AfterAt, f.AfterID = &at, &id
	}
	page, err := s.repo.ListPriceHistory(ctx, a.TenantID, f)
	if err != nil {
		return PriceHistoryList{}, err
	}
	out := PriceHistoryList{Items: page.Items}
	if page.HasMore && len(page.Items) > 0 {
		last := page.Items[len(page.Items)-1]
		out.NextCursor = encodeTimeCursor(last.At, last.ID)
	}
	return out, nil
}

func encodeTimeCursor(at time.Time, id uuid.UUID) string {
	return EncodeCursor(at.UTC().Format(time.RFC3339Nano), id)
}

func decodeTimeCursor(cursor string) (time.Time, uuid.UUID, error) {
	key, id, err := DecodeCursor(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	at, err := time.Parse(time.RFC3339Nano, key)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	return at, id, nil
}
