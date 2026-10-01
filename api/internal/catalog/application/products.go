package application

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/catalog/domain"
)

var (
	ErrSKUTaken        = errors.New("SKU sudah dipakai")
	ErrBarcodeTaken    = errors.New("barcode sudah dipakai")
	ErrInvalidCategory = errors.New("kategori tidak valid")
	ErrVersionConflict = errors.New("data sudah diubah pihak lain")
	ErrInvalidCursor   = errors.New("cursor tidak valid")
)

type InvalidVariantError struct{ Index int }

func (e *InvalidVariantError) Error() string { return "varian tidak dikenal pada produk ini" }

const (
	defaultPageSize = 50
	maxPageSize     = 100
	maxQueryLen     = 100
)

type NewProduct struct {
	ID    uuid.UUID
	Input domain.ProductInput
}

type ProductFilter struct {
	Query      string
	CategoryID *uuid.UUID
	Limit      int
	AfterKey   *string
	AfterID    *uuid.UUID
}

type ProductPage struct {
	Items   []domain.Product
	LastKey string // lower(name) item terakhir, untuk cursor halaman berikutnya
	HasMore bool
}

type ProductList struct {
	Items      []domain.Product
	NextCursor string
}

type ProductRepository interface {
	CreateProduct(ctx context.Context, a Actor, p NewProduct, now time.Time) (domain.Product, error)
	GetProduct(ctx context.Context, tenantID, id uuid.UUID) (domain.Product, error)
	ListProducts(ctx context.Context, tenantID uuid.UUID, f ProductFilter) (ProductPage, error)
	UpdateProduct(ctx context.Context, a Actor, id uuid.UUID, version int, in domain.ProductInput, now time.Time) (domain.Product, error)
	DeleteProduct(ctx context.Context, a Actor, id uuid.UUID, now time.Time) error
}

type ProductService struct {
	repo ProductRepository
	now  func() time.Time
}

func NewProductService(repo ProductRepository) *ProductService {
	return &ProductService{repo: repo, now: time.Now}
}

func (s *ProductService) Create(ctx context.Context, a Actor, in domain.ProductInput) (domain.Product, error) {
	for i := range in.Variants {
		in.Variants[i].ID = nil // pada create semua varian baru; id dari klien diabaikan
	}
	in, err := in.Validate()
	if err != nil {
		return domain.Product{}, err
	}
	productID, err := uuid.NewV7()
	if err != nil {
		return domain.Product{}, fmt.Errorf("buat id: %w", err)
	}
	if err := assignNewVariants(in.Variants); err != nil {
		return domain.Product{}, err
	}
	return s.repo.CreateProduct(ctx, a, NewProduct{ID: productID, Input: in}, s.now())
}

func assignNewVariants(variants []domain.VariantInput) error {
	for i := range variants {
		v := &variants[i]
		if v.ID != nil {
			continue
		}
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("buat id varian: %w", err)
		}
		v.NewID = id
		if v.SKU == "" {
			v.SKU = autoSKU(id)
		}
	}
	return nil
}

func autoSKU(id uuid.UUID) string {
	return "SKU-" + strings.ToUpper(id.String()[28:])
}

func (s *ProductService) Get(ctx context.Context, a Actor, id uuid.UUID) (domain.Product, error) {
	return s.repo.GetProduct(ctx, a.TenantID, id)
}

func (s *ProductService) List(ctx context.Context, a Actor, query string, categoryID *uuid.UUID, limit int, cursor string) (ProductList, error) {
	f := ProductFilter{Query: clampQuery(query), CategoryID: categoryID, Limit: clampLimit(limit)}
	if cursor != "" {
		key, id, err := DecodeCursor(cursor)
		if err != nil {
			return ProductList{}, err
		}
		f.AfterKey, f.AfterID = &key, &id
	}
	page, err := s.repo.ListProducts(ctx, a.TenantID, f)
	if err != nil {
		return ProductList{}, err
	}
	out := ProductList{Items: page.Items}
	if page.HasMore && len(page.Items) > 0 {
		out.NextCursor = EncodeCursor(page.LastKey, page.Items[len(page.Items)-1].ID)
	}
	return out, nil
}

func (s *ProductService) Update(ctx context.Context, a Actor, id uuid.UUID, version int, in domain.ProductInput) (domain.Product, error) {
	in, err := in.Validate()
	if err != nil {
		return domain.Product{}, err
	}
	if err := assignNewVariants(in.Variants); err != nil {
		return domain.Product{}, err
	}
	return s.repo.UpdateProduct(ctx, a, id, version, in, s.now())
}

func (s *ProductService) Delete(ctx context.Context, a Actor, id uuid.UUID) error {
	return s.repo.DeleteProduct(ctx, a, id, s.now())
}

func clampLimit(n int) int {
	switch {
	case n <= 0:
		return defaultPageSize
	case n > maxPageSize:
		return maxPageSize
	}
	return n
}

func clampQuery(q string) string {
	q = strings.TrimSpace(q)
	if utf8.RuneCountInString(q) > maxQueryLen {
		q = string([]rune(q)[:maxQueryLen])
	}
	return q
}

func EncodeCursor(key string, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(key + "\x00" + id.String()))
}

func DecodeCursor(c string) (string, uuid.UUID, error) {
	raw, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return "", uuid.Nil, ErrInvalidCursor
	}
	i := strings.LastIndexByte(string(raw), 0)
	if i < 0 {
		return "", uuid.Nil, ErrInvalidCursor
	}
	id, err := uuid.Parse(string(raw[i+1:]))
	if err != nil {
		return "", uuid.Nil, ErrInvalidCursor
	}
	return string(raw[:i]), id, nil
}
