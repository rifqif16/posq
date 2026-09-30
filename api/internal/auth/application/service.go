package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/auth/domain"
	tenantdomain "github.com/rifqif16/posq/api/internal/tenant/domain"
)

type Options struct {
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	TrialDays  int
}

type Service struct {
	repo      Repository
	hasher    PasswordHasher
	tokens    TokenIssuer
	opts      Options
	now       func() time.Time
	dummyHash string // dipakai agar login user tak dikenal memakan waktu sama
}

func NewService(repo Repository, hasher PasswordHasher, tokens TokenIssuer, opts Options) (*Service, error) {
	dummy, err := hasher.Hash("dummy-password-for-timing")
	if err != nil {
		return nil, fmt.Errorf("siapkan dummy hash: %w", err)
	}
	return &Service{repo: repo, hasher: hasher, tokens: tokens, opts: opts, now: time.Now, dummyHash: dummy}, nil
}

func newID() (uuid.UUID, error) { return uuid.NewV7() }

// Register membuat tenant (trial), outlet pertama, dan owner, lalu langsung membuka sesi.
func (s *Service) Register(ctx context.Context, in domain.Registration) (Session, error) {
	reg, err := in.Validate()
	if err != nil {
		return Session{}, err
	}
	hash, err := s.hasher.Hash(reg.Password)
	if err != nil {
		return Session{}, fmt.Errorf("hash password: %w", err)
	}
	acc, err := s.newAccount(reg, hash)
	if err != nil {
		return Session{}, err
	}
	if err := s.repo.CreateAccount(ctx, acc); err != nil {
		return Session{}, err
	}
	return s.openSession(ctx, acc.TenantID, acc.UserID)
}

func (s *Service) newAccount(reg domain.Registration, hash string) (NewAccount, error) {
	var ids [3]uuid.UUID
	for i := range ids {
		id, err := newID()
		if err != nil {
			return NewAccount{}, fmt.Errorf("buat id: %w", err)
		}
		ids[i] = id
	}
	return NewAccount{
		TenantID: ids[0], StoreID: ids[1], UserID: ids[2],
		BusinessName: reg.BusinessName, OwnerName: reg.OwnerName, Email: reg.Email,
		PasswordHash: hash,
		TrialEndsAt:  tenantdomain.TrialEndsAt(s.now(), s.opts.TrialDays),
	}, nil
}

// Login memverifikasi email+password. Semua kegagalan kredensial memberi error yang sama.
func (s *Service) Login(ctx context.Context, email, password string) (Session, error) {
	norm, ok := domain.NormalizeEmail(email)
	if !ok || len(password) > domain.MaxPasswordLen {
		return Session{}, ErrInvalidCredentials
	}
	user, err := s.repo.FindLoginUser(ctx, norm)
	if errors.Is(err, ErrUserNotFound) {
		_, _ = s.hasher.Verify(password, s.dummyHash)
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}
	match, err := s.hasher.Verify(password, user.PasswordHash)
	if err != nil {
		return Session{}, fmt.Errorf("verifikasi password: %w", err)
	}
	if !match {
		return Session{}, ErrInvalidCredentials
	}
	if user.Status != domain.UserActive {
		return Session{}, ErrAccountDisabled
	}
	return s.openSession(ctx, user.TenantID, user.ID)
}

// Refresh menukar refresh token dengan pasangan token baru (rotasi + deteksi reuse).
func (s *Service) Refresh(ctx context.Context, plain string) (Session, error) {
	tenantID, oldHash, err := domain.ParseRefreshToken(plain)
	if err != nil {
		return Session{}, ErrInvalidRefreshToken
	}
	newPlain, newHash, err := domain.NewRefreshToken(tenantID)
	if err != nil {
		return Session{}, err
	}
	newID, err := newID()
	if err != nil {
		return Session{}, fmt.Errorf("buat id: %w", err)
	}
	now := s.now()
	expiry := now.Add(s.opts.RefreshTTL)
	res, err := s.repo.RotateRefreshToken(ctx, RotateInput{
		TenantID: tenantID, OldHash: oldHash, NewID: newID, NewHash: newHash, NewExpiry: expiry, Now: now,
	})
	if err != nil {
		return Session{}, err
	}
	switch res.Outcome {
	case RotateReused:
		return Session{}, ErrRefreshTokenReuse
	case RotateUserDisabled:
		return Session{}, ErrAccountDisabled
	case RotateInvalid:
		return Session{}, ErrInvalidRefreshToken
	}
	return s.buildSession(ctx, tenantID, res.UserID, newPlain, expiry)
}

// Logout mencabut seluruh keluarga refresh token. Idempoten: token tak valid dianggap sukses.
func (s *Service) Logout(ctx context.Context, plain string) error {
	tenantID, hash, err := domain.ParseRefreshToken(plain)
	if err != nil {
		return nil
	}
	return s.repo.RevokeRefreshFamily(ctx, tenantID, hash, s.now())
}

// Authenticate memvalidasi access token.
func (s *Service) Authenticate(token string) (AccessClaims, error) {
	c, err := s.tokens.Parse(token, s.now())
	if err != nil {
		return AccessClaims{}, ErrUnauthorized
	}
	return c, nil
}

func (s *Service) Me(ctx context.Context, c AccessClaims) (Principal, error) {
	p, err := s.repo.GetPrincipal(ctx, c.TenantID, c.UserID)
	if errors.Is(err, ErrUserNotFound) {
		return Principal{}, ErrUnauthorized
	}
	return p, err
}

func (s *Service) openSession(ctx context.Context, tenantID, userID uuid.UUID) (Session, error) {
	plain, hash, err := domain.NewRefreshToken(tenantID)
	if err != nil {
		return Session{}, err
	}
	rtID, err := newID()
	if err != nil {
		return Session{}, fmt.Errorf("buat id: %w", err)
	}
	familyID, err := newID()
	if err != nil {
		return Session{}, fmt.Errorf("buat id: %w", err)
	}
	expiry := s.now().Add(s.opts.RefreshTTL)
	err = s.repo.SaveRefreshToken(ctx, NewRefreshToken{
		ID: rtID, TenantID: tenantID, UserID: userID, FamilyID: familyID, Hash: hash, ExpiresAt: expiry,
	})
	if err != nil {
		return Session{}, err
	}
	return s.buildSession(ctx, tenantID, userID, plain, expiry)
}

func (s *Service) buildSession(ctx context.Context, tenantID, userID uuid.UUID, refresh string, refreshExp time.Time) (Session, error) {
	p, err := s.repo.GetPrincipal(ctx, tenantID, userID)
	if err != nil {
		return Session{}, err
	}
	access, err := s.tokens.Issue(AccessClaims{
		UserID: p.UserID, TenantID: p.TenantID, Role: p.Role, StoreIDs: p.StoreIDs,
	}, s.now(), s.opts.AccessTTL)
	if err != nil {
		return Session{}, fmt.Errorf("terbitkan access token: %w", err)
	}
	return Session{
		AccessToken: access, AccessExpiresIn: s.opts.AccessTTL,
		RefreshToken: refresh, RefreshExpiresAt: refreshExp, Principal: p,
	}, nil
}
