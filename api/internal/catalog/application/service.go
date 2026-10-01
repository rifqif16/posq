package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/catalog/domain"
)

var (
	ErrNotFound          = errors.New("data tidak ditemukan")
	ErrCategoryNameTaken = errors.New("nama kategori sudah dipakai")
	ErrCategoryInUse     = errors.New("kategori masih dipakai sub-kategori atau produk")
	ErrInvalidParent     = errors.New("induk kategori tidak valid")
)

type Actor struct{ TenantID, UserID uuid.UUID }

type NewCategory struct {
	ID        uuid.UUID
	ParentID  *uuid.UUID
	Name      string
	SortOrder int
}

type Repository interface {
	ListCategories(ctx context.Context, tenantID uuid.UUID) ([]domain.Category, error)
	CreateCategory(ctx context.Context, a Actor, c NewCategory, now time.Time) (domain.Category, error)
	UpdateCategory(ctx context.Context, a Actor, id uuid.UUID, name string, sortOrder int, now time.Time) (domain.Category, error)
	DeleteCategory(ctx context.Context, a Actor, id uuid.UUID, now time.Time) error
}

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service { return &Service{repo: repo, now: time.Now} }

func (s *Service) ListCategories(ctx context.Context, a Actor) ([]domain.Category, error) {
	return s.repo.ListCategories(ctx, a.TenantID)
}

func (s *Service) CreateCategory(ctx context.Context, a Actor, parentID *uuid.UUID, name string, sortOrder int) (domain.Category, error) {
	name, err := domain.ValidateCategory(name, sortOrder)
	if err != nil {
		return domain.Category{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return domain.Category{}, fmt.Errorf("buat id: %w", err)
	}
	return s.repo.CreateCategory(ctx, a, NewCategory{ID: id, ParentID: parentID, Name: name, SortOrder: sortOrder}, s.now())
}

func (s *Service) UpdateCategory(ctx context.Context, a Actor, id uuid.UUID, name string, sortOrder int) (domain.Category, error) {
	name, err := domain.ValidateCategory(name, sortOrder)
	if err != nil {
		return domain.Category{}, err
	}
	return s.repo.UpdateCategory(ctx, a, id, name, sortOrder, s.now())
}

func (s *Service) DeleteCategory(ctx context.Context, a Actor, id uuid.UUID) error {
	return s.repo.DeleteCategory(ctx, a, id, s.now())
}
