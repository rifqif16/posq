package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/auth/domain"
)

var (
	ErrForbiddenTarget = errors.New("pengguna ini tidak dapat dikelola oleh Anda")
	ErrRoleNotAllowed  = errors.New("role ini tidak dapat Anda berikan")
	ErrPlanLimit       = errors.New("batas pengguna paket tercapai")
)

type StoreInvalidError struct{ Index int }

func (e *StoreInvalidError) Error() string { return "outlet tidak valid" }

type StaffActor struct {
	TenantID uuid.UUID
	UserID   uuid.UUID
	Role     domain.Role
}

type StaffMember struct {
	ID        uuid.UUID
	Name      string
	Email     string
	Role      domain.Role
	Status    domain.UserStatus
	StoreIDs  []uuid.UUID
	HasPIN    bool
	CreatedAt time.Time
}

type StoreInfo struct {
	ID   uuid.UUID
	Code string
	Name string
}

type NewStaff struct {
	ID           uuid.UUID
	Input        domain.StaffInput
	PasswordHash string
	PINHash      *string
}

type StaffRepository interface {
	ListStaff(ctx context.Context, tenantID uuid.UUID) ([]StaffMember, error)
	ListStores(ctx context.Context, tenantID uuid.UUID) ([]StoreInfo, error)
	GetStaff(ctx context.Context, tenantID, id uuid.UUID) (StaffMember, error)
	UserStoreIDs(ctx context.Context, tenantID, userID uuid.UUID) ([]uuid.UUID, error)
	CreateStaff(ctx context.Context, a StaffActor, n NewStaff, now time.Time) error
	UpdateStaff(ctx context.Context, a StaffActor, id uuid.UUID, c domain.StaffChange, now time.Time) (StaffMember, error)
	SetPassword(ctx context.Context, tenantID, id uuid.UUID, hash string, now time.Time) error
	SetPIN(ctx context.Context, tenantID, id uuid.UUID, hash *string, now time.Time) error
}

type StaffService struct {
	repo   StaffRepository
	hasher PasswordHasher
	now    func() time.Time
}

func NewStaffService(repo StaffRepository, hasher PasswordHasher) *StaffService {
	return &StaffService{repo: repo, hasher: hasher, now: time.Now}
}

func (s *StaffService) List(ctx context.Context, a StaffActor) ([]StaffMember, error) {
	return s.repo.ListStaff(ctx, a.TenantID)
}

func (s *StaffService) Stores(ctx context.Context, a StaffActor) ([]StoreInfo, error) {
	return s.repo.ListStores(ctx, a.TenantID)
}

func (s *StaffService) Create(ctx context.Context, a StaffActor, in domain.StaffInput) (StaffMember, error) {
	v, err := in.Validate()
	if err != nil {
		return StaffMember{}, err
	}
	if !domain.CanAssign(a.Role, v.Role) {
		return StaffMember{}, ErrRoleNotAllowed
	}
	if err := s.checkStores(ctx, a, v.StoreIDs); err != nil {
		return StaffMember{}, err
	}
	hash, err := s.hasher.Hash(v.Password)
	if err != nil {
		return StaffMember{}, fmt.Errorf("hash password: %w", err)
	}
	n := NewStaff{Input: v, PasswordHash: hash}
	if v.PIN != "" {
		pinHash, err := s.hasher.Hash(v.PIN)
		if err != nil {
			return StaffMember{}, fmt.Errorf("hash pin: %w", err)
		}
		n.PINHash = &pinHash
	}
	if n.ID, err = uuid.NewV7(); err != nil {
		return StaffMember{}, fmt.Errorf("buat id: %w", err)
	}
	if err := s.repo.CreateStaff(ctx, a, n, s.now()); err != nil {
		return StaffMember{}, err
	}
	return s.repo.GetStaff(ctx, a.TenantID, n.ID)
}

func (s *StaffService) Update(ctx context.Context, a StaffActor, id uuid.UUID, in domain.StaffChange) (StaffMember, error) {
	c, err := in.Validate()
	if err != nil {
		return StaffMember{}, err
	}
	if _, err := s.manageable(ctx, a, id); err != nil {
		return StaffMember{}, err
	}
	if !domain.CanAssign(a.Role, c.Role) {
		return StaffMember{}, ErrRoleNotAllowed
	}
	if err := s.checkStores(ctx, a, c.StoreIDs); err != nil {
		return StaffMember{}, err
	}
	return s.repo.UpdateStaff(ctx, a, id, c, s.now())
}

func (s *StaffService) ResetPassword(ctx context.Context, a StaffActor, id uuid.UUID, password string) error {
	if msg, ok := domain.ValidatePassword(password); !ok {
		return &domain.ValidationError{Issues: []domain.Issue{{Field: "password", Message: msg}}}
	}
	if _, err := s.manageable(ctx, a, id); err != nil {
		return err
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.repo.SetPassword(ctx, a.TenantID, id, hash, s.now())
}

func (s *StaffService) SetPIN(ctx context.Context, a StaffActor, id uuid.UUID, pin string) error {
	if msg, ok := domain.ValidatePIN(pin); !ok {
		return &domain.ValidationError{Issues: []domain.Issue{{Field: "pin", Message: msg}}}
	}
	if _, err := s.manageable(ctx, a, id); err != nil {
		return err
	}
	hash, err := s.hasher.Hash(pin)
	if err != nil {
		return fmt.Errorf("hash pin: %w", err)
	}
	return s.repo.SetPIN(ctx, a.TenantID, id, &hash, s.now())
}

func (s *StaffService) ClearPIN(ctx context.Context, a StaffActor, id uuid.UUID) error {
	if _, err := s.manageable(ctx, a, id); err != nil {
		return err
	}
	return s.repo.SetPIN(ctx, a.TenantID, id, nil, s.now())
}

func (s *StaffService) manageable(ctx context.Context, a StaffActor, id uuid.UUID) (StaffMember, error) {
	m, err := s.repo.GetStaff(ctx, a.TenantID, id)
	if err != nil {
		return StaffMember{}, err
	}
	if m.ID == a.UserID || !domain.CanManage(a.Role, m.Role) {
		return StaffMember{}, ErrForbiddenTarget
	}
	return m, nil
}

func (s *StaffService) checkStores(ctx context.Context, a StaffActor, ids []uuid.UUID) error {
	if a.Role == domain.RoleOwner {
		return nil
	}
	mine, err := s.repo.UserStoreIDs(ctx, a.TenantID, a.UserID)
	if err != nil {
		return err
	}
	allowed := make(map[uuid.UUID]bool, len(mine))
	for _, id := range mine {
		allowed[id] = true
	}
	for i, id := range ids {
		if !allowed[id] {
			return &StoreInvalidError{Index: i}
		}
	}
	return nil
}
