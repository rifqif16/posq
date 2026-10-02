package domain

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

const MaxStaffStores = 20

var pinPattern = regexp.MustCompile(`^\d{6}$`)

var weakPINs = map[string]bool{"123456": true, "654321": true, "012345": true, "123123": true}

func AssignableRoles(actor Role) []Role {
	switch actor {
	case RoleOwner:
		return []Role{RoleAdmin, RoleCashier, RoleKitchen}
	case RoleAdmin:
		return []Role{RoleCashier}
	}
	return nil
}

func CanAssign(actor, target Role) bool {
	for _, r := range AssignableRoles(actor) {
		if r == target {
			return true
		}
	}
	return false
}

func CanManage(actor, target Role) bool {
	return target != RoleOwner && CanAssign(actor, target)
}

func ValidatePIN(pin string) (string, bool) {
	if !pinPattern.MatchString(pin) {
		return "PIN harus 6 digit angka", false
	}
	if weakPINs[pin] || strings.Count(pin, pin[:1]) == len(pin) {
		return "PIN terlalu mudah ditebak", false
	}
	return "", true
}

func validStaffRole(r Role) bool {
	return r == RoleAdmin || r == RoleCashier || r == RoleKitchen
}

func validateStoreIDs(ids []uuid.UUID) []Issue {
	if len(ids) < 1 || len(ids) > MaxStaffStores {
		return []Issue{{Field: "store_ids", Message: fmt.Sprintf("Pilih 1 sampai %d outlet", MaxStaffStores)}}
	}
	var issues []Issue
	seen := make(map[uuid.UUID]bool, len(ids))
	for i, id := range ids {
		if id == uuid.Nil || seen[id] {
			issues = append(issues, Issue{Field: fmt.Sprintf("store_ids[%d]", i), Message: "Outlet tidak valid atau dipilih lebih dari sekali"})
		}
		seen[id] = true
	}
	return issues
}

type StaffInput struct {
	Name     string
	Email    string
	Password string
	Role     Role
	StoreIDs []uuid.UUID
	PIN      string
}

func (in StaffInput) Validate() (StaffInput, error) {
	var issues []Issue
	name, ok := normalizeText(in.Name, MaxNameLen)
	if !ok {
		issues = append(issues, Issue{Field: "name", Message: "Nama wajib diisi (maks 100 karakter)"})
	}
	in.Name = name
	email, ok := NormalizeEmail(in.Email)
	if !ok {
		issues = append(issues, Issue{Field: "email", Message: "Format email tidak valid"})
	}
	in.Email = email
	if msg, ok := ValidatePassword(in.Password); !ok {
		issues = append(issues, Issue{Field: "password", Message: msg})
	}
	if !validStaffRole(in.Role) {
		issues = append(issues, Issue{Field: "role", Message: "Role harus admin, cashier, atau kitchen"})
	}
	issues = append(issues, validateStoreIDs(in.StoreIDs)...)
	if in.PIN != "" {
		if msg, ok := ValidatePIN(in.PIN); !ok {
			issues = append(issues, Issue{Field: "pin", Message: msg})
		}
	}
	if len(issues) > 0 {
		return in, &ValidationError{Issues: issues}
	}
	return in, nil
}

type StaffChange struct {
	Name     string
	Role     Role
	StoreIDs []uuid.UUID
	Status   UserStatus
}

func (c StaffChange) Validate() (StaffChange, error) {
	var issues []Issue
	name, ok := normalizeText(c.Name, MaxNameLen)
	if !ok {
		issues = append(issues, Issue{Field: "name", Message: "Nama wajib diisi (maks 100 karakter)"})
	}
	c.Name = name
	if !validStaffRole(c.Role) {
		issues = append(issues, Issue{Field: "role", Message: "Role harus admin, cashier, atau kitchen"})
	}
	if c.Status != UserActive && c.Status != UserDisabled {
		issues = append(issues, Issue{Field: "status", Message: "Status harus active atau disabled"})
	}
	issues = append(issues, validateStoreIDs(c.StoreIDs)...)
	if len(issues) > 0 {
		return c, &ValidationError{Issues: issues}
	}
	return c, nil
}
