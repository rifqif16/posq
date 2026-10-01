package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/catalog/domain"
)

type fakePriceRepo struct {
	filter PriceHistoryFilter
	page   PriceHistoryPage
	err    error
}

func (f *fakePriceRepo) ListPriceHistory(_ context.Context, _ uuid.UUID, fl PriceHistoryFilter) (PriceHistoryPage, error) {
	f.filter = fl
	return f.page, f.err
}

func TestTimeCursorRoundTripKeepsMicroseconds(t *testing.T) {
	id := uuid.New()
	at := time.Date(2026, 10, 1, 8, 15, 22, 123456000, time.FixedZone("WIB", 7*3600))
	gotAt, gotID, err := decodeTimeCursor(encodeTimeCursor(at, id))
	if err != nil || !gotAt.Equal(at) || gotID != id {
		t.Fatalf("round trip: %v %v %v", gotAt, gotID, err)
	}
}

func TestTimeCursorRejectsGarbage(t *testing.T) {
	id := uuid.New()
	for _, bad := range []string{"!!!", EncodeCursor("bukan-waktu", id), EncodeCursor("", id)} {
		if _, _, err := decodeTimeCursor(bad); !errors.Is(err, ErrInvalidCursor) {
			t.Errorf("%q harus ErrInvalidCursor, dapat %v", bad, err)
		}
	}
}

func TestPriceHistoryListBuildsNextCursorAndClampsLimit(t *testing.T) {
	last := domain.PriceChange{ID: uuid.New(), At: time.Now().UTC()}
	repo := &fakePriceRepo{page: PriceHistoryPage{Items: []domain.PriceChange{{ID: uuid.New(), At: time.Now()}, last}, HasMore: true}}
	svc := NewPriceHistoryService(repo)
	productID := uuid.New()

	res, err := svc.List(context.Background(), Actor{}, productID, 9999, "")
	if err != nil {
		t.Fatal(err)
	}
	if repo.filter.Limit != maxPageSize || repo.filter.ProductID != productID || repo.filter.AfterAt != nil {
		t.Fatalf("filter salah: %+v", repo.filter)
	}
	at, id, err := decodeTimeCursor(res.NextCursor)
	if err != nil || id != last.ID || !at.Equal(last.At) {
		t.Fatalf("next cursor salah: %v %v %v", at, id, err)
	}

	if _, err := svc.List(context.Background(), Actor{}, productID, 0, res.NextCursor); err != nil || repo.filter.AfterID == nil || *repo.filter.AfterID != last.ID {
		t.Fatalf("cursor harus diteruskan ke filter: %+v %v", repo.filter, err)
	}
	if _, err := svc.List(context.Background(), Actor{}, productID, 0, "rusak!"); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("cursor rusak: %v", err)
	}
	repo.page.HasMore = false
	if res, _ = svc.List(context.Background(), Actor{}, productID, 0, ""); res.NextCursor != "" || repo.filter.Limit != defaultPageSize {
		t.Fatalf("halaman terakhir tanpa cursor: %+v", res)
	}
}
