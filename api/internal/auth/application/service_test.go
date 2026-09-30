package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/auth/domain"
)

// fakeRepo adalah repository in-memory untuk menguji orkestrasi use case.
type fakeRepo struct {
	users    map[string]LoginUser // by email
	accounts []NewAccount
	saved    []NewRefreshToken
	rotate   RotateResult
	revoked  int
}

func (f *fakeRepo) CreateAccount(_ context.Context, a NewAccount) error {
	if _, dup := f.users[a.Email]; dup {
		return ErrEmailTaken
	}
	f.accounts = append(f.accounts, a)
	f.users[a.Email] = LoginUser{ID: a.UserID, TenantID: a.TenantID, PasswordHash: a.PasswordHash, Status: domain.UserActive}
	return nil
}
func (f *fakeRepo) FindLoginUser(_ context.Context, email string) (LoginUser, error) {
	u, ok := f.users[email]
	if !ok {
		return LoginUser{}, ErrUserNotFound
	}
	return u, nil
}
func (f *fakeRepo) GetPrincipal(_ context.Context, t, u uuid.UUID) (Principal, error) {
	return Principal{UserID: u, TenantID: t, Role: domain.RoleOwner, StoreIDs: []uuid.UUID{uuid.New()}}, nil
}
func (f *fakeRepo) SaveRefreshToken(_ context.Context, t NewRefreshToken) error {
	f.saved = append(f.saved, t)
	return nil
}
func (f *fakeRepo) RotateRefreshToken(context.Context, RotateInput) (RotateResult, error) {
	return f.rotate, nil
}
func (f *fakeRepo) RevokeRefreshFamily(context.Context, uuid.UUID, []byte, time.Time) error {
	f.revoked++
	return nil
}

// plainHasher: hasher deterministik agar test cepat.
type plainHasher struct{}

func (plainHasher) Hash(p string) (string, error)      { return "h:" + p, nil }
func (plainHasher) Verify(p, enc string) (bool, error) { return enc == "h:"+p, nil }

type stubTokens struct{ fail bool }

func (s stubTokens) Issue(AccessClaims, time.Time, time.Duration) (string, error) {
	return "access", nil
}
func (s stubTokens) Parse(string, time.Time) (AccessClaims, error) {
	if s.fail {
		return AccessClaims{}, errors.New("bad")
	}
	return AccessClaims{}, nil
}

func newTestService(t *testing.T) (*Service, *fakeRepo) {
	t.Helper()
	repo := &fakeRepo{users: map[string]LoginUser{}}
	svc, err := NewService(repo, plainHasher{}, stubTokens{}, Options{AccessTTL: time.Minute, RefreshTTL: time.Hour, TrialDays: 14})
	if err != nil {
		t.Fatal(err)
	}
	return svc, repo
}

var validReg = domain.Registration{BusinessName: "Kedai", OwnerName: "Sari", Email: "Sari@Kedai.ID", Password: "password-aman"}

func TestRegisterCreatesTrialAccountAndSession(t *testing.T) {
	svc, repo := newTestService(t)
	now := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	sess, err := svc.Register(context.Background(), validReg)
	if err != nil {
		t.Fatal(err)
	}
	acc := repo.accounts[0]
	if acc.Email != "sari@kedai.id" || acc.PasswordHash != "h:password-aman" {
		t.Fatalf("akun tidak sesuai: %+v", acc)
	}
	if !acc.TrialEndsAt.Equal(now.AddDate(0, 0, 14)) {
		t.Fatalf("trial salah: %v", acc.TrialEndsAt)
	}
	if sess.AccessToken == "" || !strings.HasPrefix(sess.RefreshToken, acc.TenantID.String()+".") || len(repo.saved) != 1 {
		t.Fatalf("sesi tidak lengkap: %+v", sess)
	}
}

func TestRegisterRejectsInvalidAndDuplicate(t *testing.T) {
	svc, _ := newTestService(t)
	var ve *domain.ValidationError
	if _, err := svc.Register(context.Background(), domain.Registration{}); !errors.As(err, &ve) {
		t.Fatalf("harus ValidationError, dapat %v", err)
	}
	if _, err := svc.Register(context.Background(), validReg); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register(context.Background(), validReg); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("harus ErrEmailTaken, dapat %v", err)
	}
}

func TestLoginOutcomes(t *testing.T) {
	svc, repo := newTestService(t)
	if _, err := svc.Register(context.Background(), validReg); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if _, err := svc.Login(ctx, "SARI@kedai.id", "password-aman"); err != nil {
		t.Fatalf("login valid gagal: %v", err)
	}
	for name, c := range map[string][2]string{
		"password salah":   {"sari@kedai.id", "salah-salah-salah"},
		"user tak ada":     {"nobody@kedai.id", "password-aman"},
		"email invalid":    {"bukan-email", "password-aman"},
		"password raksasa": {"sari@kedai.id", strings.Repeat("a", 500)},
	} {
		if _, err := svc.Login(ctx, c[0], c[1]); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("%s: harus ErrInvalidCredentials, dapat %v", name, err)
		}
	}

	u := repo.users["sari@kedai.id"]
	u.Status = domain.UserDisabled
	repo.users["sari@kedai.id"] = u
	if _, err := svc.Login(ctx, "sari@kedai.id", "password-aman"); !errors.Is(err, ErrAccountDisabled) {
		t.Fatalf("akun nonaktif harus ErrAccountDisabled, dapat %v", err)
	}
}

func TestRefreshMapsOutcomes(t *testing.T) {
	svc, repo := newTestService(t)
	tenant := uuid.New()
	plain, _, _ := domain.NewRefreshToken(tenant)

	cases := map[RotateOutcome]error{
		RotateInvalid:      ErrInvalidRefreshToken,
		RotateReused:       ErrRefreshTokenReuse,
		RotateUserDisabled: ErrAccountDisabled,
	}
	for outcome, want := range cases {
		repo.rotate = RotateResult{Outcome: outcome}
		if _, err := svc.Refresh(context.Background(), plain); !errors.Is(err, want) {
			t.Errorf("outcome %d: harus %v, dapat %v", outcome, want, err)
		}
	}

	repo.rotate = RotateResult{Outcome: RotateOK, UserID: uuid.New()}
	sess, err := svc.Refresh(context.Background(), plain)
	if err != nil || sess.RefreshToken == plain || !strings.HasPrefix(sess.RefreshToken, tenant.String()+".") {
		t.Fatalf("rotasi sukses harus menghasilkan token baru: %v", err)
	}

	if _, err := svc.Refresh(context.Background(), "sampah"); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("token cacat harus ErrInvalidRefreshToken, dapat %v", err)
	}
}

func TestLogoutIsIdempotentForMalformedToken(t *testing.T) {
	svc, repo := newTestService(t)
	if err := svc.Logout(context.Background(), "sampah"); err != nil || repo.revoked != 0 {
		t.Fatalf("token cacat: err=%v revoked=%d", err, repo.revoked)
	}
	plain, _, _ := domain.NewRefreshToken(uuid.New())
	if err := svc.Logout(context.Background(), plain); err != nil || repo.revoked != 1 {
		t.Fatalf("token valid: err=%v revoked=%d", err, repo.revoked)
	}
}

func TestAuthenticateMapsToUnauthorized(t *testing.T) {
	svc, _ := newTestService(t)
	svc.tokens = stubTokens{fail: true}
	if _, err := svc.Authenticate("x"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("harus ErrUnauthorized, dapat %v", err)
	}
}
