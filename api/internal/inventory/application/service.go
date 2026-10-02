package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/inventory/domain"
	"github.com/rifqif16/posq/api/internal/platform/pagination"
)

const (
	defaultPageSize = 50
	maxPageSize     = 100
	maxQueryLen     = 100
	keySeparator    = "\x01"
)

var (
	ErrStoreNotFound = errors.New("outlet tidak ditemukan")
	ErrInvalidCursor = pagination.ErrInvalidCursor
)

type VariantProblemError struct {
	Index      int
	NotTracked bool
}

func (e *VariantProblemError) Error() string {
	if e.NotTracked {
		return "produk tidak melacak stok"
	}
	return "varian tidak ditemukan"
}

type Actor struct{ TenantID, UserID uuid.UUID }

type MovementResult struct {
	Movement  domain.Movement
	QtyOnHand int64
}

type CountResult struct {
	VariantID uuid.UUID
	Before    int64
	Counted   int64
	Delta     int64
}

type LevelFilter struct {
	StoreID      uuid.UUID
	Query        string
	Limit        int
	AfterProduct *string
	AfterVariant *string
	AfterID      *uuid.UUID
}

type LevelPage struct {
	Items       []domain.Level
	LastProduct string
	LastVariant string
	HasMore     bool
}

type MovementFilter struct {
	StoreID   uuid.UUID
	VariantID *uuid.UUID
	Type      string
	Limit     int
	AfterAt   *time.Time
	AfterID   *uuid.UUID
}

type MovementPage struct {
	Items   []domain.Movement
	HasMore bool
}

type LevelList struct {
	Items      []domain.Level
	NextCursor string
}

type MovementList struct {
	Items      []domain.Movement
	NextCursor string
}

type Repository interface {
	RecordMovement(ctx context.Context, a Actor, id uuid.UUID, m domain.ValidatedMovement, now time.Time) (MovementResult, error)
	ApplyCounts(ctx context.Context, a Actor, c domain.ValidatedCount, ids []uuid.UUID, now time.Time) ([]CountResult, error)
	ListLevels(ctx context.Context, a Actor, f LevelFilter) (LevelPage, error)
	ListMovements(ctx context.Context, a Actor, f MovementFilter) (MovementPage, error)
}

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service { return &Service{repo: repo, now: time.Now} }

func (s *Service) RecordMovement(ctx context.Context, a Actor, in domain.MovementInput) (MovementResult, error) {
	m, err := in.Validate()
	if err != nil {
		return MovementResult{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return MovementResult{}, fmt.Errorf("buat id: %w", err)
	}
	return s.repo.RecordMovement(ctx, a, id, m, s.now())
}

func (s *Service) ApplyCounts(ctx context.Context, a Actor, in domain.CountInput) ([]CountResult, error) {
	c, err := in.Validate()
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(c.Items))
	for i := range ids {
		if ids[i], err = uuid.NewV7(); err != nil {
			return nil, fmt.Errorf("buat id: %w", err)
		}
	}
	return s.repo.ApplyCounts(ctx, a, c, ids, s.now())
}

func (s *Service) Levels(ctx context.Context, a Actor, storeID uuid.UUID, query string, limit int, cursor string) (LevelList, error) {
	f := LevelFilter{StoreID: storeID, Query: clampQuery(query), Limit: pagination.Clamp(limit, defaultPageSize, maxPageSize)}
	if cursor != "" {
		key, id, err := pagination.Decode(cursor)
		if err != nil {
			return LevelList{}, err
		}
		product, variant, ok := strings.Cut(key, keySeparator)
		if !ok {
			return LevelList{}, ErrInvalidCursor
		}
		f.AfterProduct, f.AfterVariant, f.AfterID = &product, &variant, &id
	}
	page, err := s.repo.ListLevels(ctx, a, f)
	if err != nil {
		return LevelList{}, err
	}
	out := LevelList{Items: page.Items}
	if page.HasMore && len(page.Items) > 0 {
		out.NextCursor = pagination.Encode(page.LastProduct+keySeparator+page.LastVariant, page.Items[len(page.Items)-1].VariantID)
	}
	return out, nil
}

func (s *Service) Movements(ctx context.Context, a Actor, storeID uuid.UUID, variantID *uuid.UUID, typ string, limit int, cursor string) (MovementList, error) {
	f := MovementFilter{StoreID: storeID, VariantID: variantID, Type: typ, Limit: pagination.Clamp(limit, defaultPageSize, maxPageSize)}
	if cursor != "" {
		at, id, err := pagination.DecodeTime(cursor)
		if err != nil {
			return MovementList{}, err
		}
		f.AfterAt, f.AfterID = &at, &id
	}
	page, err := s.repo.ListMovements(ctx, a, f)
	if err != nil {
		return MovementList{}, err
	}
	out := MovementList{Items: page.Items}
	if page.HasMore && len(page.Items) > 0 {
		last := page.Items[len(page.Items)-1]
		out.NextCursor = pagination.EncodeTime(last.CreatedAt, last.ID)
	}
	return out, nil
}

func clampQuery(q string) string {
	q = strings.TrimSpace(q)
	if r := []rune(q); len(r) > maxQueryLen {
		q = string(r[:maxQueryLen])
	}
	return q
}
