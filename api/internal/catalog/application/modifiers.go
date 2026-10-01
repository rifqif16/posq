package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/catalog/domain"
)

var ErrModifierGroupNameTaken = errors.New("nama grup modifier sudah dipakai")

type InvalidModifierError struct{ Index int }

func (e *InvalidModifierError) Error() string { return "opsi tidak dikenal pada grup ini" }

type NewModifierGroup struct {
	ID    uuid.UUID
	Input domain.ModifierGroupInput
}

type ModifierRepository interface {
	ListModifierGroups(ctx context.Context, tenantID uuid.UUID) ([]domain.ModifierGroup, error)
	GetModifierGroup(ctx context.Context, tenantID, id uuid.UUID) (domain.ModifierGroup, error)
	CreateModifierGroup(ctx context.Context, a Actor, g NewModifierGroup, now time.Time) (domain.ModifierGroup, error)
	UpdateModifierGroup(ctx context.Context, a Actor, id uuid.UUID, version int, in domain.ModifierGroupInput, now time.Time) (domain.ModifierGroup, error)
	DeleteModifierGroup(ctx context.Context, a Actor, id uuid.UUID, now time.Time) error
}

type ModifierService struct {
	repo ModifierRepository
	now  func() time.Time
}

func NewModifierService(repo ModifierRepository) *ModifierService {
	return &ModifierService{repo: repo, now: time.Now}
}

func (s *ModifierService) List(ctx context.Context, a Actor) ([]domain.ModifierGroup, error) {
	return s.repo.ListModifierGroups(ctx, a.TenantID)
}

func (s *ModifierService) Get(ctx context.Context, a Actor, id uuid.UUID) (domain.ModifierGroup, error) {
	return s.repo.GetModifierGroup(ctx, a.TenantID, id)
}

func (s *ModifierService) Create(ctx context.Context, a Actor, in domain.ModifierGroupInput) (domain.ModifierGroup, error) {
	for i := range in.Modifiers {
		in.Modifiers[i].ID = nil // pada create semua opsi baru; id dari klien diabaikan
	}
	in, err := in.Validate()
	if err != nil {
		return domain.ModifierGroup{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return domain.ModifierGroup{}, fmt.Errorf("buat id: %w", err)
	}
	if err := assignNewModifiers(in.Modifiers); err != nil {
		return domain.ModifierGroup{}, err
	}
	return s.repo.CreateModifierGroup(ctx, a, NewModifierGroup{ID: id, Input: in}, s.now())
}

func (s *ModifierService) Update(ctx context.Context, a Actor, id uuid.UUID, version int, in domain.ModifierGroupInput) (domain.ModifierGroup, error) {
	in, err := in.Validate()
	if err != nil {
		return domain.ModifierGroup{}, err
	}
	if err := assignNewModifiers(in.Modifiers); err != nil {
		return domain.ModifierGroup{}, err
	}
	return s.repo.UpdateModifierGroup(ctx, a, id, version, in, s.now())
}

func (s *ModifierService) Delete(ctx context.Context, a Actor, id uuid.UUID) error {
	return s.repo.DeleteModifierGroup(ctx, a, id, s.now())
}

func assignNewModifiers(mods []domain.ModifierInput) error {
	for i := range mods {
		if mods[i].ID != nil {
			continue
		}
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("buat id opsi: %w", err)
		}
		mods[i].NewID = id
	}
	return nil
}
