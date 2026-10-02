package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestRoleManagementRules(t *testing.T) {
	cases := []struct {
		actor, target  Role
		assign, manage bool
	}{
		{RoleOwner, RoleAdmin, true, true}, {RoleOwner, RoleCashier, true, true}, {RoleOwner, RoleKitchen, true, true},
		{RoleOwner, RoleOwner, false, false},
		{RoleAdmin, RoleCashier, true, true}, {RoleAdmin, RoleAdmin, false, false}, {RoleAdmin, RoleKitchen, false, false},
		{RoleAdmin, RoleOwner, false, false}, {RoleCashier, RoleCashier, false, false}, {RoleKitchen, RoleCashier, false, false},
	}
	for _, c := range cases {
		if CanAssign(c.actor, c.target) != c.assign || CanManage(c.actor, c.target) != c.manage {
			t.Errorf("%s -> %s: assign=%v manage=%v", c.actor, c.target, CanAssign(c.actor, c.target), CanManage(c.actor, c.target))
		}
	}
}

func TestValidatePIN(t *testing.T) {
	for _, ok := range []string{"135790", "482915", "100200"} {
		if _, valid := ValidatePIN(ok); !valid {
			t.Errorf("%s harus valid", ok)
		}
	}
	for _, bad := range []string{"", "12345", "1234567", "12345a", "000000", "111111", "123456", "654321", " 12345", "١٢٣٤٥٦"} {
		if _, valid := ValidatePIN(bad); valid {
			t.Errorf("%q harus ditolak", bad)
		}
	}
}

func validStaff() StaffInput {
	return StaffInput{Name: " Budi ", Email: " Budi@Kedai.ID ", Password: "password-aman-1", Role: RoleCashier, StoreIDs: []uuid.UUID{uuid.New()}}
}

func staffFields(err error) []string {
	var ve *ValidationError
	if !errors.As(err, &ve) {
		return nil
	}
	var out []string
	for _, i := range ve.Issues {
		out = append(out, i.Field)
	}
	return out
}

func TestStaffInputValidation(t *testing.T) {
	in, err := validStaff().Validate()
	if err != nil || in.Name != "Budi" || in.Email != "budi@kedai.id" {
		t.Fatalf("valid: %+v %v", in, err)
	}
	withPIN := validStaff()
	withPIN.PIN = "135790"
	if _, err := withPIN.Validate(); err != nil {
		t.Fatalf("pin valid: %v", err)
	}
	mutations := map[string]func(*StaffInput){
		"name":      func(s *StaffInput) { s.Name = " " },
		"email":     func(s *StaffInput) { s.Email = "bukan-email" },
		"password":  func(s *StaffInput) { s.Password = "pendek" },
		"role":      func(s *StaffInput) { s.Role = RoleOwner },
		"store_ids": func(s *StaffInput) { s.StoreIDs = nil },
		"pin":       func(s *StaffInput) { s.PIN = "111111" },
	}
	for field, mutate := range mutations {
		s := validStaff()
		mutate(&s)
		_, err := s.Validate()
		if got := staffFields(err); len(got) != 1 || got[0] != field {
			t.Errorf("%s: %v", field, got)
		}
	}
	dup := validStaff()
	id := uuid.New()
	dup.StoreIDs = []uuid.UUID{id, id}
	if _, err := dup.Validate(); len(staffFields(err)) != 1 || staffFields(err)[0] != "store_ids[1]" {
		t.Errorf("outlet duplikat: %v", staffFields(err))
	}
	many := validStaff()
	for i := 0; i <= MaxStaffStores; i++ {
		many.StoreIDs = append(many.StoreIDs, uuid.New())
	}
	if _, err := many.Validate(); len(staffFields(err)) != 1 || staffFields(err)[0] != "store_ids" {
		t.Errorf("terlalu banyak outlet: %v", staffFields(err))
	}
}

func TestStaffChangeValidation(t *testing.T) {
	ok := StaffChange{Name: " Budi ", Role: RoleKitchen, StoreIDs: []uuid.UUID{uuid.New()}, Status: UserDisabled}
	if c, err := ok.Validate(); err != nil || c.Name != "Budi" {
		t.Fatalf("valid: %+v %v", c, err)
	}
	bad := map[string]StaffChange{
		"name":   {Name: "", Role: RoleCashier, StoreIDs: []uuid.UUID{uuid.New()}, Status: UserActive},
		"role":   {Name: "x", Role: "owner", StoreIDs: []uuid.UUID{uuid.New()}, Status: UserActive},
		"status": {Name: "x", Role: RoleCashier, StoreIDs: []uuid.UUID{uuid.New()}, Status: "hapus"},
	}
	for field, c := range bad {
		if _, err := c.Validate(); len(staffFields(err)) != 1 || staffFields(err)[0] != field {
			t.Errorf("%s: %v", field, staffFields(err))
		}
	}
}
