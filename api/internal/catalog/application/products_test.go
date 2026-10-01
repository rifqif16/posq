package application

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/catalog/domain"
)

type fakeProductRepo struct {
	created NewProduct
	filter  ProductFilter
	page    ProductPage
	err     error
}

func (f *fakeProductRepo) CreateProduct(_ context.Context, _ Actor, p NewProduct, _ time.Time) (domain.Product, error) {
	f.created = p
	return domain.Product{ID: p.ID}, f.err
}
func (f *fakeProductRepo) GetProduct(context.Context, uuid.UUID, uuid.UUID) (domain.Product, error) {
	return domain.Product{}, f.err
}
func (f *fakeProductRepo) ListProducts(_ context.Context, _ uuid.UUID, fl ProductFilter) (ProductPage, error) {
	f.filter = fl
	return f.page, f.err
}
func (f *fakeProductRepo) UpdateProduct(context.Context, Actor, uuid.UUID, int, domain.ProductInput, time.Time) (domain.Product, error) {
	return domain.Product{}, f.err
}
func (f *fakeProductRepo) DeleteProduct(context.Context, Actor, uuid.UUID, time.Time) error {
	return f.err
}

func input(sku string) domain.ProductInput {
	return domain.ProductInput{Name: "Kopi", IsActive: true,
		Variants: []domain.VariantInput{{SKU: sku, SellPrice: 1000}}}
}

func TestCreateGeneratesSKUWhenEmptyAndKeepsGivenSKU(t *testing.T) {
	repo := &fakeProductRepo{}
	svc := NewProductService(repo)

	if _, err := svc.Create(context.Background(), Actor{}, input("")); err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^SKU-[0-9A-F]{8}$`).MatchString(repo.created.Input.Variants[0].SKU) {
		t.Fatalf("sku otomatis salah: %q", repo.created.Input.Variants[0].SKU)
	}
	if repo.created.ID.Version() != 7 || repo.created.VariantID.Version() != 7 {
		t.Fatal("id harus UUIDv7")
	}

	if _, err := svc.Create(context.Background(), Actor{}, input("KOPI-1")); err != nil {
		t.Fatal(err)
	}
	if repo.created.Input.Variants[0].SKU != "KOPI-1" {
		t.Fatalf("sku harus dipertahankan: %q", repo.created.Input.Variants[0].SKU)
	}
}

func TestCreateRejectsInvalidBeforeRepository(t *testing.T) {
	repo := &fakeProductRepo{}
	var ve *domain.ValidationError
	if _, err := NewProductService(repo).Create(context.Background(), Actor{}, domain.ProductInput{}); !errors.As(err, &ve) {
		t.Fatalf("harus ValidationError, dapat %v", err)
	}
	if repo.created.ID != uuid.Nil {
		t.Fatal("repository tidak boleh dipanggil")
	}
}

func TestCursorRoundTripAndRejectsGarbage(t *testing.T) {
	id := uuid.New()
	key, gotID, err := DecodeCursor(EncodeCursor("kopi \x01 susu", id))
	if err != nil || key != "kopi \x01 susu" || gotID != id {
		t.Fatalf("round trip: %q %v %v", key, gotID, err)
	}
	for _, bad := range []string{"!!!", "YWJj", EncodeCursor("a", id)[:5]} {
		if _, _, err := DecodeCursor(bad); !errors.Is(err, ErrInvalidCursor) {
			t.Errorf("%q harus ErrInvalidCursor, dapat %v", bad, err)
		}
	}
}

func TestListClampsLimitAndBuildsNextCursor(t *testing.T) {
	last := uuid.New()
	repo := &fakeProductRepo{page: ProductPage{Items: []domain.Product{{ID: uuid.New()}, {ID: last}}, LastKey: "k", HasMore: true}}
	svc := NewProductService(repo)

	res, err := svc.List(context.Background(), Actor{}, "  "+strings.Repeat("x", 300), nil, 1000, "")
	if err != nil {
		t.Fatal(err)
	}
	if repo.filter.Limit != maxPageSize || len([]rune(repo.filter.Query)) != maxQueryLen {
		t.Fatalf("clamp gagal: %+v", repo.filter)
	}
	key, id, err := DecodeCursor(res.NextCursor)
	if err != nil || key != "k" || id != last {
		t.Fatalf("next cursor salah: %q %v %v", key, id, err)
	}

	if _, err := svc.List(context.Background(), Actor{}, "", nil, 0, "rusak!"); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("cursor rusak: %v", err)
	}
	repo.page.HasMore = false
	if res, _ = svc.List(context.Background(), Actor{}, "", nil, 0, ""); res.NextCursor != "" || repo.filter.Limit != defaultPageSize {
		t.Fatalf("halaman terakhir tidak boleh punya cursor: %+v", res)
	}
}
