package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/catalog/domain"
)

type fakeModifierRepo struct {
	created NewModifierGroup
	updated domain.ModifierGroupInput
	err     error
}

func (f *fakeModifierRepo) ListModifierGroups(context.Context, uuid.UUID) ([]domain.ModifierGroup, error) {
	return nil, f.err
}
func (f *fakeModifierRepo) GetModifierGroup(context.Context, uuid.UUID, uuid.UUID) (domain.ModifierGroup, error) {
	return domain.ModifierGroup{}, f.err
}
func (f *fakeModifierRepo) CreateModifierGroup(_ context.Context, _ Actor, g NewModifierGroup, _ time.Time) (domain.ModifierGroup, error) {
	f.created = g
	return domain.ModifierGroup{ID: g.ID}, f.err
}
func (f *fakeModifierRepo) UpdateModifierGroup(_ context.Context, _ Actor, _ uuid.UUID, _ int, in domain.ModifierGroupInput, _ time.Time) (domain.ModifierGroup, error) {
	f.updated = in
	return domain.ModifierGroup{}, f.err
}
func (f *fakeModifierRepo) DeleteModifierGroup(context.Context, Actor, uuid.UUID, time.Time) error {
	return f.err
}

func group(mods ...domain.ModifierInput) domain.ModifierGroupInput {
	return domain.ModifierGroupInput{Name: "Topping", MinSelect: 0, MaxSelect: 2, Modifiers: mods}
}

func TestModifierCreateAssignsIDsAndIgnoresClientIDs(t *testing.T) {
	repo := &fakeModifierRepo{}
	svc := NewModifierService(repo)
	stale := uuid.New()
	in := group(
		domain.ModifierInput{ID: &stale, Name: "Boba", IsActive: true}, // id klien harus diabaikan
		domain.ModifierInput{Name: "Jelly", IsActive: true},
	)
	if _, err := svc.Create(context.Background(), Actor{}, in); err != nil {
		t.Fatal(err)
	}
	ms := repo.created.Input.Modifiers
	if repo.created.ID.Version() != 7 || ms[0].ID != nil || ms[1].ID != nil {
		t.Fatalf("id salah: %+v", repo.created)
	}
	if ms[0].NewID.Version() != 7 || ms[1].NewID.Version() != 7 || ms[0].NewID == ms[1].NewID {
		t.Fatal("NewID harus UUIDv7 dan unik")
	}
}

func TestModifierUpdateOnlyAssignsNewOptions(t *testing.T) {
	repo := &fakeModifierRepo{}
	svc := NewModifierService(repo)
	existing := uuid.New()
	in := group(
		domain.ModifierInput{ID: &existing, Name: "Boba", IsActive: true},
		domain.ModifierInput{Name: "Jelly", IsActive: true},
	)
	if _, err := svc.Update(context.Background(), Actor{}, uuid.New(), 1, in); err != nil {
		t.Fatal(err)
	}
	ms := repo.updated.Modifiers
	if ms[0].ID == nil || *ms[0].ID != existing || ms[0].NewID != uuid.Nil {
		t.Fatalf("opsi lama tidak boleh disentuh: %+v", ms[0])
	}
	if ms[1].ID != nil || ms[1].NewID == uuid.Nil {
		t.Fatalf("opsi baru harus mendapat id: %+v", ms[1])
	}
}

func TestModifierServiceRejectsInvalidAndPropagatesErrors(t *testing.T) {
	repo := &fakeModifierRepo{}
	svc := NewModifierService(repo)
	var ve *domain.ValidationError
	if _, err := svc.Create(context.Background(), Actor{}, domain.ModifierGroupInput{}); !errors.As(err, &ve) {
		t.Fatalf("harus ValidationError, dapat %v", err)
	}
	if repo.created.ID != uuid.Nil {
		t.Fatal("repository tidak boleh dipanggil untuk input invalid")
	}
	repo.err = ErrModifierGroupNameTaken
	ok := group(domain.ModifierInput{Name: "A", IsActive: true})
	if _, err := svc.Create(context.Background(), Actor{}, ok); !errors.Is(err, ErrModifierGroupNameTaken) {
		t.Fatalf("error repo harus diteruskan: %v", err)
	}
}
