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
	updated domain.ProductInput
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
func (f *fakeProductRepo) UpdateProduct(_ context.Context, _ Actor, _ uuid.UUID, _ int, in domain.ProductInput, _ time.Time) (domain.Product, error) {
	f.updated = in
	return domain.Product{}, f.err
}
func (f *fakeProductRepo) DeleteProduct(context.Context, Actor, uuid.UUID, time.Time) error {
	return f.err
}

var autoSKUPattern = regexp.MustCompile(`^SKU-[0-9A-F]{8}$`)

func TestCreateAssignsIDsAndSKUsForEveryVariant(t *testing.T) {
	repo := &fakeProductRepo{}
	svc := NewProductService(repo)
	stale := uuid.New()
	in := domain.ProductInput{Name: "Kopi", IsActive: true, Variants: []domain.VariantInput{
		{ID: &stale, Name: "Small", SellPrice: 1, IsActive: true}, // id klien harus diabaikan
		{Name: "Large", SKU: "K-L", SellPrice: 2, IsActive: true},
	}}
	if _, err := svc.Create(context.Background(), Actor{}, in); err != nil {
		t.Fatal(err)
	}
	vs := repo.created.Input.Variants
	if repo.created.ID.Version() != 7 || vs[0].ID != nil || vs[1].ID != nil {
		t.Fatalf("id produk/varian salah: %+v", repo.created)
	}
	if vs[0].NewID.Version() != 7 || vs[1].NewID.Version() != 7 || vs[0].NewID == vs[1].NewID {
		t.Fatal("NewID harus UUIDv7 dan unik")
	}
	if !autoSKUPattern.MatchString(vs[0].SKU) || vs[1].SKU != "K-L" {
		t.Fatalf("sku: %q %q", vs[0].SKU, vs[1].SKU)
	}
}

func TestUpdateOnlyAssignsNewVariants(t *testing.T) {
	repo := &fakeProductRepo{}
	svc := NewProductService(repo)
	existing := uuid.New()
	in := domain.ProductInput{Name: "Kopi", IsActive: true, Variants: []domain.VariantInput{
		{ID: &existing, Name: "Small", IsActive: true}, // SKU kosong = dipertahankan oleh repository
		{Name: "XL", IsActive: true},
	}}
	if _, err := svc.Update(context.Background(), Actor{}, uuid.New(), 1, in); err != nil {
		t.Fatal(err)
	}
	vs := repo.updated.Variants
	if vs[0].ID == nil || *vs[0].ID != existing || vs[0].NewID != uuid.Nil || vs[0].SKU != "" {
		t.Fatalf("varian lama tidak boleh disentuh: %+v", vs[0])
	}
	if vs[1].ID != nil || vs[1].NewID == uuid.Nil || !autoSKUPattern.MatchString(vs[1].SKU) {
		t.Fatalf("varian baru harus mendapat id dan sku: %+v", vs[1])
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
