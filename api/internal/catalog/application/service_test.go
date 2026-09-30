package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/catalog/domain"
)

type fakeRepo struct {
	created NewCategory
	actor   Actor
	err     error
}

func (f *fakeRepo) ListCategories(context.Context, uuid.UUID) ([]domain.Category, error) {
	return nil, f.err
}
func (f *fakeRepo) CreateCategory(_ context.Context, a Actor, c NewCategory, _ time.Time) (domain.Category, error) {
	f.created, f.actor = c, a
	return domain.Category{ID: c.ID, Name: c.Name}, f.err
}
func (f *fakeRepo) UpdateCategory(_ context.Context, _ Actor, id uuid.UUID, name string, _ int, _ time.Time) (domain.Category, error) {
	return domain.Category{ID: id, Name: name}, f.err
}
func (f *fakeRepo) DeleteCategory(context.Context, Actor, uuid.UUID, time.Time) error { return f.err }

func TestCreateCategoryNormalizesAndAssignsV7ID(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	actor := Actor{TenantID: uuid.New(), UserID: uuid.New()}

	cat, err := svc.CreateCategory(context.Background(), actor, nil, "  Kopi ", 3)
	if err != nil {
		t.Fatal(err)
	}
	if cat.Name != "Kopi" || repo.created.SortOrder != 3 || repo.actor != actor {
		t.Fatalf("tidak sesuai: %+v %+v", repo.created, repo.actor)
	}
	if repo.created.ID.Version() != 7 {
		t.Fatalf("id harus UUIDv7, dapat v%d", repo.created.ID.Version())
	}
}

func TestServiceRejectsInvalidInputBeforeRepository(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	var ve *domain.ValidationError
	if _, err := svc.CreateCategory(context.Background(), Actor{}, nil, "", 0); !errors.As(err, &ve) {
		t.Fatalf("create: harus ValidationError, dapat %v", err)
	}
	if _, err := svc.UpdateCategory(context.Background(), Actor{}, uuid.New(), " ", 0); !errors.As(err, &ve) {
		t.Fatalf("update: harus ValidationError, dapat %v", err)
	}
	if repo.created.ID != uuid.Nil {
		t.Fatal("repository tidak boleh dipanggil untuk input invalid")
	}
}

func TestServicePropagatesRepositoryErrors(t *testing.T) {
	repo := &fakeRepo{err: ErrCategoryInUse}
	svc := NewService(repo)
	if err := svc.DeleteCategory(context.Background(), Actor{}, uuid.New()); !errors.Is(err, ErrCategoryInUse) {
		t.Fatalf("harus ErrCategoryInUse, dapat %v", err)
	}
}
