package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/auth/domain"
)

const (
	MaxPinAttempts  = 5
	PinLockDuration = 15 * time.Minute
)

var ErrPinLocked = errors.New("PIN terkunci sementara")

type PinLoginInput struct {
	DeviceID     uuid.UUID
	DeviceSecret string
	UserID       uuid.UUID
	PIN          string
}

type PinUsersResult struct {
	DeviceName string
	StoreID    uuid.UUID
	Users      []PinUser
}

type PinLoginService struct {
	*Service
	devices DeviceRepository
}

func NewPinLoginService(svc *Service, devices DeviceRepository) *PinLoginService {
	return &PinLoginService{Service: svc, devices: devices}
}

func (p *PinLoginService) authDevice(ctx context.Context, id uuid.UUID, secret string) (DeviceAuth, error) {
	d, err := p.devices.FindDevice(ctx, id)
	if errors.Is(err, ErrDeviceNotFound) {
		return DeviceAuth{}, ErrInvalidCredentials
	}
	if err != nil {
		return DeviceAuth{}, err
	}
	if d.Status != DeviceActive || !secretMatches(secret, d.SecretHash) {
		return DeviceAuth{}, ErrInvalidCredentials
	}
	return d, nil
}

func (p *PinLoginService) Users(ctx context.Context, deviceID uuid.UUID, secret string) (PinUsersResult, error) {
	d, err := p.authDevice(ctx, deviceID, secret)
	if err != nil {
		return PinUsersResult{}, err
	}
	users, err := p.devices.PinUsers(ctx, d.TenantID, d.StoreID)
	if err != nil {
		return PinUsersResult{}, err
	}
	return PinUsersResult{DeviceName: d.Name, StoreID: d.StoreID, Users: users}, nil
}

func (p *PinLoginService) Login(ctx context.Context, in PinLoginInput) (Session, error) {
	d, err := p.authDevice(ctx, in.DeviceID, in.DeviceSecret)
	if err != nil {
		return Session{}, err
	}
	cand, err := p.devices.FindPinCandidate(ctx, d.TenantID, d.StoreID, in.UserID)
	if errors.Is(err, ErrUserNotFound) {
		p.spendDummyHash(in.PIN)
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}
	eligible := cand.PINHash != nil && (cand.Role == domain.RoleCashier || cand.Role == domain.RoleKitchen)
	if !eligible {
		p.spendDummyHash(in.PIN)
		return Session{}, ErrInvalidCredentials
	}
	now := p.now()
	if cand.LockedUntil != nil && now.Before(*cand.LockedUntil) {
		return Session{}, ErrPinLocked
	}
	ok, err := p.hasher.Verify(in.PIN, *cand.PINHash)
	if err != nil {
		return Session{}, fmt.Errorf("verifikasi PIN: %w", err)
	}
	if !ok {
		locked, err := p.devices.RecordPinFailure(ctx, d.TenantID, in.UserID, MaxPinAttempts, now.Add(PinLockDuration))
		if err != nil {
			return Session{}, err
		}
		if locked {
			return Session{}, ErrPinLocked
		}
		return Session{}, ErrInvalidCredentials
	}
	if cand.Status == domain.UserDisabled {
		return Session{}, ErrAccountDisabled
	}
	if err := p.devices.ResetPinFailures(ctx, d.TenantID, in.UserID); err != nil {
		return Session{}, err
	}
	if err := p.devices.TouchDevice(ctx, d.TenantID, d.ID, now); err != nil {
		return Session{}, err
	}
	return p.openSession(ctx, d.TenantID, in.UserID)
}

func (p *PinLoginService) spendDummyHash(pin string) {
	_, _ = p.hasher.Verify(pin, p.dummyHash)
}
