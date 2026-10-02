package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/auth/domain"
)

const (
	DeviceActive     = "active"
	DeviceRevoked    = "revoked"
	MaxDeviceNameLen = 60
)

var (
	ErrDeviceNotFound  = errors.New("perangkat tidak ditemukan")
	ErrDevicePlanLimit = errors.New("batas perangkat paket tercapai")
)

type Device struct {
	ID           uuid.UUID
	StoreID      uuid.UUID
	Code         string
	Name         string
	Status       string
	RegisteredAt time.Time
	LastSeenAt   *time.Time
	RegisteredBy string
}

type DeviceAuth struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	StoreID    uuid.UUID
	Code       string
	Name       string
	SecretHash []byte
	Status     string
}

type NewDevice struct {
	ID         uuid.UUID
	StoreID    uuid.UUID
	Name       string
	SecretHash []byte
}

type PinUser struct {
	ID   uuid.UUID
	Name string
	Role domain.Role
}

type PinCandidate struct {
	Role        domain.Role
	Status      domain.UserStatus
	PINHash     *string
	LockedUntil *time.Time
}

type DeviceRepository interface {
	ListDevices(ctx context.Context, tenantID uuid.UUID) ([]Device, error)
	CreateDevice(ctx context.Context, a StaffActor, n NewDevice, now time.Time) (Device, error)
	RevokeDevice(ctx context.Context, tenantID, id uuid.UUID, now time.Time) error
	FindDevice(ctx context.Context, id uuid.UUID) (DeviceAuth, error)
	TouchDevice(ctx context.Context, tenantID, id uuid.UUID, now time.Time) error
	PinUsers(ctx context.Context, tenantID, storeID uuid.UUID) ([]PinUser, error)
	FindPinCandidate(ctx context.Context, tenantID, storeID, userID uuid.UUID) (PinCandidate, error)
	RecordPinFailure(ctx context.Context, tenantID, userID uuid.UUID, maxAttempts int, lockUntil time.Time) (bool, error)
	ResetPinFailures(ctx context.Context, tenantID, userID uuid.UUID) error
}

type DeviceService struct {
	repo DeviceRepository
	now  func() time.Time
}

func NewDeviceService(repo DeviceRepository) *DeviceService {
	return &DeviceService{repo: repo, now: time.Now}
}

func newDeviceSecret() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("buat rahasia perangkat: %w", err)
	}
	plain := base64.RawURLEncoding.EncodeToString(raw)
	return plain, hashSecret(plain), nil
}

func hashSecret(plain string) []byte {
	sum := sha256.Sum256([]byte(plain))
	return sum[:]
}

func secretMatches(plain string, hash []byte) bool {
	return subtle.ConstantTimeCompare(hashSecret(plain), hash) == 1
}

func (s *DeviceService) List(ctx context.Context, a StaffActor) ([]Device, error) {
	return s.repo.ListDevices(ctx, a.TenantID)
}

func (s *DeviceService) Create(ctx context.Context, a StaffActor, storeID uuid.UUID, name string) (Device, string, error) {
	var issues []domain.Issue
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > MaxDeviceNameLen {
		issues = append(issues, domain.Issue{Field: "name", Message: "Nama perangkat wajib diisi (maks 60 karakter)"})
	}
	if storeID == uuid.Nil {
		issues = append(issues, domain.Issue{Field: "store_id", Message: "Outlet wajib diisi"})
	}
	if len(issues) > 0 {
		return Device{}, "", &domain.ValidationError{Issues: issues}
	}
	secret, hash, err := newDeviceSecret()
	if err != nil {
		return Device{}, "", err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Device{}, "", fmt.Errorf("buat id: %w", err)
	}
	d, err := s.repo.CreateDevice(ctx, a, NewDevice{ID: id, StoreID: storeID, Name: name, SecretHash: hash}, s.now())
	if err != nil {
		return Device{}, "", err
	}
	return d, secret, nil
}

func (s *DeviceService) Revoke(ctx context.Context, a StaffActor, id uuid.UUID) error {
	return s.repo.RevokeDevice(ctx, a.TenantID, id, s.now())
}
