package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/inventory/domain"
)

type fakeRepo struct {
	movement domain.ValidatedMovement
	movID    uuid.UUID
	counts   domain.ValidatedCount
	countIDs []uuid.UUID
	levels   LevelFilter
	levelPg  LevelPage
	moves    MovementFilter
	movePg   MovementPage
	err      error
}

func (f *fakeRepo) RecordMovement(_ context.Context, _ Actor, id uuid.UUID, m domain.ValidatedMovement, _ time.Time) (MovementResult, error) {
	f.movID, f.movement = id, m
	return MovementResult{}, f.err
}
func (f *fakeRepo) ApplyCounts(_ context.Context, _ Actor, c domain.ValidatedCount, ids []uuid.UUID, _ time.Time) ([]CountResult, error) {
	f.counts, f.countIDs = c, ids
	return nil, f.err
}
func (f *fakeRepo) ListLevels(_ context.Context, _ Actor, fl LevelFilter) (LevelPage, error) {
	f.levels = fl
	return f.levelPg, f.err
}
func (f *fakeRepo) ListMovements(_ context.Context, _ Actor, fl MovementFilter) (MovementPage, error) {
	f.moves = fl
	return f.movePg, f.err
}

func TestRecordMovementValidatesBeforeRepositoryAndAssignsV7ID(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	var ve *domain.ValidationError
	if _, err := svc.RecordMovement(context.Background(), Actor{}, domain.MovementInput{}); !errors.As(err, &ve) {
		t.Fatalf("harus ValidationError: %v", err)
	}
	if repo.movID != uuid.Nil {
		t.Fatal("repository tidak boleh dipanggil")
	}
	in := domain.MovementInput{StoreID: uuid.New(), VariantID: uuid.New(), Type: domain.TypePurchaseReceive, QtyDelta: "2.5"}
	if _, err := svc.RecordMovement(context.Background(), Actor{}, in); err != nil {
		t.Fatal(err)
	}
	if repo.movID.Version() != 7 || repo.movement.QtyDelta != 2500 {
		t.Fatalf("id/qty: %v %+v", repo.movID, repo.movement)
	}
}

func TestApplyCountsAssignsOneIDPerItem(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	in := domain.CountInput{StoreID: uuid.New(), Items: []domain.CountItemInput{{VariantID: uuid.New(), CountedQty: "1"}, {VariantID: uuid.New(), CountedQty: "2"}}}
	if _, err := svc.ApplyCounts(context.Background(), Actor{}, in); err != nil {
		t.Fatal(err)
	}
	if len(repo.countIDs) != 2 || repo.countIDs[0] == repo.countIDs[1] || repo.countIDs[0].Version() != 7 {
		t.Fatalf("ids: %v", repo.countIDs)
	}
	if _, err := svc.ApplyCounts(context.Background(), Actor{}, domain.CountInput{}); err == nil {
		t.Fatal("input kosong harus ditolak")
	}
}

func TestLevelsCursorRoundTripAndClamp(t *testing.T) {
	last := uuid.New()
	repo := &fakeRepo{levelPg: LevelPage{Items: []domain.Level{{VariantID: uuid.New()}, {VariantID: last}}, LastProduct: "kopi", LastVariant: "large", HasMore: true}}
	svc := NewService(repo)
	store := uuid.New()

	res, err := svc.Levels(context.Background(), Actor{}, store, "  kopi ", 9999, "")
	if err != nil || repo.levels.Limit != maxPageSize || repo.levels.Query != "kopi" || repo.levels.AfterID != nil {
		t.Fatalf("filter: %+v %v", repo.levels, err)
	}
	if _, err := svc.Levels(context.Background(), Actor{}, store, "", 0, res.NextCursor); err != nil {
		t.Fatal(err)
	}
	f := repo.levels
	if f.AfterProduct == nil || *f.AfterProduct != "kopi" || *f.AfterVariant != "large" || *f.AfterID != last || f.Limit != defaultPageSize {
		t.Fatalf("cursor diteruskan: %+v", f)
	}
	if _, err := svc.Levels(context.Background(), Actor{}, store, "", 0, "rusak!"); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("cursor rusak: %v", err)
	}
	if _, err := svc.Levels(context.Background(), Actor{}, store, "", 0, "YWJj"); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("cursor tanpa pemisah: %v", err)
	}
	repo.levelPg.HasMore = false
	if res, _ = svc.Levels(context.Background(), Actor{}, store, "", 0, ""); res.NextCursor != "" {
		t.Fatal("halaman terakhir tanpa cursor")
	}
}

func TestMovementsCursorRoundTrip(t *testing.T) {
	last := domain.Movement{ID: uuid.New(), CreatedAt: time.Now().UTC()}
	repo := &fakeRepo{movePg: MovementPage{Items: []domain.Movement{{ID: uuid.New(), CreatedAt: time.Now()}, last}, HasMore: true}}
	svc := NewService(repo)
	store, variant := uuid.New(), uuid.New()

	res, err := svc.Movements(context.Background(), Actor{}, store, &variant, "waste", 5, "")
	if err != nil || repo.moves.Type != "waste" || *repo.moves.VariantID != variant || repo.moves.Limit != 5 {
		t.Fatalf("filter: %+v %v", repo.moves, err)
	}
	if _, err := svc.Movements(context.Background(), Actor{}, store, nil, "", 0, res.NextCursor); err != nil || *repo.moves.AfterID != last.ID || !repo.moves.AfterAt.Equal(last.CreatedAt) {
		t.Fatalf("cursor: %+v %v", repo.moves, err)
	}
	if _, err := svc.Movements(context.Background(), Actor{}, store, nil, "", 0, "rusak!"); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("cursor rusak: %v", err)
	}
}
