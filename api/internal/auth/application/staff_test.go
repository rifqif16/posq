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

type fakeStaffRepo struct {
	members map[uuid.UUID]StaffMember
	mine    []uuid.UUID
	created *NewStaff
	updated *domain.StaffChange
	pwHash  string
	pinHash *string
	pinSet  bool
}

func (f *fakeStaffRepo) ListStaff(context.Context, uuid.UUID) ([]StaffMember, error) { return nil, nil }
func (f *fakeStaffRepo) ListStores(context.Context, uuid.UUID) ([]StoreInfo, error)  { return nil, nil }
func (f *fakeStaffRepo) GetStaff(_ context.Context, _, id uuid.UUID) (StaffMember, error) {
	m, ok := f.members[id]
	if !ok {
		return StaffMember{}, ErrUserNotFound
	}
	return m, nil
}
func (f *fakeStaffRepo) UserStoreIDs(context.Context, uuid.UUID, uuid.UUID) ([]uuid.UUID, error) {
	return f.mine, nil
}
func (f *fakeStaffRepo) CreateStaff(_ context.Context, _ StaffActor, n NewStaff, _ time.Time) error {
	f.created = &n
	f.members[n.ID] = StaffMember{ID: n.ID, Role: n.Input.Role, Status: domain.UserActive}
	return nil
}
func (f *fakeStaffRepo) UpdateStaff(_ context.Context, _ StaffActor, id uuid.UUID, c domain.StaffChange, _ time.Time) (StaffMember, error) {
	f.updated = &c
	return f.members[id], nil
}
func (f *fakeStaffRepo) SetPassword(_ context.Context, _, _ uuid.UUID, hash string, _ time.Time) error {
	f.pwHash = hash
	return nil
}
func (f *fakeStaffRepo) SetPIN(_ context.Context, _, _ uuid.UUID, hash *string, _ time.Time) error {
	f.pinHash, f.pinSet = hash, true
	return nil
}

type prefixHasher struct{}

func (prefixHasher) Hash(p string) (string, error)      { return "h:" + p, nil }
func (prefixHasher) Verify(p, enc string) (bool, error) { return enc == "h:"+p, nil }

func staffFixture() (*StaffService, *fakeStaffRepo, map[string]StaffActor, map[string]uuid.UUID) {
	store := uuid.New()
	ids := map[string]uuid.UUID{"owner": uuid.New(), "admin": uuid.New(), "admin2": uuid.New(), "cashier": uuid.New(), "kitchen": uuid.New(), "store": store, "otherStore": uuid.New()}
	repo := &fakeStaffRepo{mine: []uuid.UUID{store}, members: map[uuid.UUID]StaffMember{}}
	for name, role := range map[string]domain.Role{"owner": domain.RoleOwner, "admin": domain.RoleAdmin, "admin2": domain.RoleAdmin, "cashier": domain.RoleCashier, "kitchen": domain.RoleKitchen} {
		repo.members[ids[name]] = StaffMember{ID: ids[name], Role: role, Status: domain.UserActive}
	}
	tenant := uuid.New()
	actors := map[string]StaffActor{
		"owner": {TenantID: tenant, UserID: ids["owner"], Role: domain.RoleOwner},
		"admin": {TenantID: tenant, UserID: ids["admin"], Role: domain.RoleAdmin},
	}
	return NewStaffService(repo, prefixHasher{}), repo, actors, ids
}

func newStaffInput(role domain.Role, stores ...uuid.UUID) domain.StaffInput {
	return domain.StaffInput{Name: "Budi", Email: "budi@kedai.id", Password: "password-aman-1", Role: role, StoreIDs: stores, PIN: "135790"}
}

func TestCreateHashesSecretsAndEnforcesRoles(t *testing.T) {
	svc, repo, actors, ids := staffFixture()
	ctx := context.Background()

	if _, err := svc.Create(ctx, actors["owner"], newStaffInput(domain.RoleAdmin, ids["otherStore"])); err != nil {
		t.Fatalf("owner membuat admin: %v", err)
	}
	if repo.created.PasswordHash != "h:password-aman-1" || repo.created.PINHash == nil || *repo.created.PINHash != "h:135790" || repo.created.ID.Version() != 7 {
		t.Fatalf("hash/id: %+v", repo.created)
	}
	if strings.Contains(repo.created.PasswordHash, "password-aman-1") && !strings.HasPrefix(repo.created.PasswordHash, "h:") {
		t.Fatal("password tidak boleh tersimpan polos")
	}

	noPIN := newStaffInput(domain.RoleCashier, ids["store"])
	noPIN.PIN = ""
	if _, err := svc.Create(ctx, actors["admin"], noPIN); err != nil || repo.created.PINHash != nil {
		t.Fatalf("admin membuat kasir tanpa PIN: %v %+v", err, repo.created)
	}
	if _, err := svc.Create(ctx, actors["admin"], newStaffInput(domain.RoleAdmin, ids["store"])); !errors.Is(err, ErrRoleNotAllowed) {
		t.Fatalf("admin membuat admin: %v", err)
	}
	if _, err := svc.Create(ctx, actors["admin"], newStaffInput(domain.RoleKitchen, ids["store"])); !errors.Is(err, ErrRoleNotAllowed) {
		t.Fatalf("admin membuat kitchen: %v", err)
	}
	var ve *domain.ValidationError
	if _, err := svc.Create(ctx, actors["owner"], domain.StaffInput{}); !errors.As(err, &ve) {
		t.Fatalf("validasi: %v", err)
	}
}

func TestAdminStoresMustBeSubsetOfOwnStores(t *testing.T) {
	svc, _, actors, ids := staffFixture()
	var se *StoreInvalidError
	_, err := svc.Create(context.Background(), actors["admin"], newStaffInput(domain.RoleCashier, ids["store"], ids["otherStore"]))
	if !errors.As(err, &se) || se.Index != 1 {
		t.Fatalf("outlet di luar milik admin: %v", err)
	}
	change := domain.StaffChange{Name: "Kasir", Role: domain.RoleCashier, StoreIDs: []uuid.UUID{ids["otherStore"]}, Status: domain.UserActive}
	if _, err := svc.Update(context.Background(), actors["admin"], ids["cashier"], change); !errors.As(err, &se) {
		t.Fatalf("update dengan outlet asing: %v", err)
	}
	if _, err := svc.Update(context.Background(), actors["owner"], ids["cashier"], change); err != nil {
		t.Fatalf("owner bebas memilih outlet: %v", err)
	}
}

func TestUpdateAndSecretsAuthorizeTarget(t *testing.T) {
	svc, repo, actors, ids := staffFixture()
	ctx := context.Background()
	change := domain.StaffChange{Name: "X", Role: domain.RoleCashier, StoreIDs: []uuid.UUID{ids["store"]}, Status: domain.UserDisabled}

	for _, target := range []string{"owner", "admin", "admin2", "kitchen"} {
		if _, err := svc.Update(ctx, actors["admin"], ids[target], change); !errors.Is(err, ErrForbiddenTarget) {
			t.Errorf("admin mengubah %s: %v", target, err)
		}
		if err := svc.ResetPassword(ctx, actors["admin"], ids[target], "password-baru-1"); !errors.Is(err, ErrForbiddenTarget) {
			t.Errorf("admin reset password %s: %v", target, err)
		}
		if err := svc.SetPIN(ctx, actors["admin"], ids[target], "135790"); !errors.Is(err, ErrForbiddenTarget) {
			t.Errorf("admin set PIN %s: %v", target, err)
		}
	}
	if _, err := svc.Update(ctx, actors["owner"], ids["owner"], change); !errors.Is(err, ErrForbiddenTarget) {
		t.Errorf("owner mengubah dirinya: %v", err)
	}
	if _, err := svc.Update(ctx, actors["admin"], uuid.New(), change); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("tidak ada: %v", err)
	}
	if _, err := svc.Update(ctx, actors["admin"], ids["cashier"], domain.StaffChange{Name: "X", Role: domain.RoleAdmin, StoreIDs: []uuid.UUID{ids["store"]}, Status: domain.UserActive}); !errors.Is(err, ErrRoleNotAllowed) {
		t.Errorf("admin menaikkan kasir jadi admin: %v", err)
	}
	if _, err := svc.Update(ctx, actors["admin"], ids["cashier"], change); err != nil || repo.updated.Status != domain.UserDisabled {
		t.Errorf("admin menonaktifkan kasir: %v", err)
	}

	if err := svc.ResetPassword(ctx, actors["admin"], ids["cashier"], "password-baru-1"); err != nil || repo.pwHash != "h:password-baru-1" {
		t.Errorf("reset password: %v %q", err, repo.pwHash)
	}
	var ve *domain.ValidationError
	if err := svc.ResetPassword(ctx, actors["admin"], ids["cashier"], "pendek"); !errors.As(err, &ve) {
		t.Errorf("password pendek: %v", err)
	}
	if err := svc.SetPIN(ctx, actors["admin"], ids["cashier"], "135790"); err != nil || repo.pinHash == nil || *repo.pinHash != "h:135790" {
		t.Errorf("set PIN: %v", err)
	}
	if err := svc.SetPIN(ctx, actors["admin"], ids["cashier"], "111111"); !errors.As(err, &ve) {
		t.Errorf("PIN lemah: %v", err)
	}
	if err := svc.ClearPIN(ctx, actors["admin"], ids["cashier"]); err != nil || repo.pinHash != nil || !repo.pinSet {
		t.Errorf("hapus PIN: %v", err)
	}
}
