// Package domain (auth): aturan identitas murni, tanpa database/HTTP.
package domain

import (
	"net/mail"
	"strings"
	"unicode/utf8"
)

type Role string

const (
	RoleOwner   Role = "owner"
	RoleAdmin   Role = "admin"
	RoleCashier Role = "cashier"
	RoleKitchen Role = "kitchen"
)

type UserStatus string

const (
	UserActive   UserStatus = "active"
	UserDisabled UserStatus = "disabled"
)

const (
	MinPasswordLen = 10 // 7.1
	// Batas atas mencegah DoS lewat hashing input raksasa.
	MaxPasswordLen = 128
	MaxNameLen     = 100
	MaxEmailLen    = 254
)

// Issue menggambarkan satu pelanggaran validasi pada satu field.
type Issue struct {
	Field   string
	Message string
}

type ValidationError struct{ Issues []Issue }

func (e *ValidationError) Error() string { return "validasi gagal" }

// NormalizeEmail memangkas spasi, mengubah ke huruf kecil, dan memvalidasi format.
func NormalizeEmail(raw string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || len(email) > MaxEmailLen {
		return "", false
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || !strings.Contains(email[strings.LastIndex(email, "@"):], ".") {
		return "", false
	}
	return email, true
}

func ValidatePassword(p string) (string, bool) {
	n := utf8.RuneCountInString(p)
	switch {
	case n < MinPasswordLen:
		return "Password minimal 10 karakter", false
	case len(p) > MaxPasswordLen:
		return "Password maksimal 128 karakter", false
	}
	return "", true
}

func normalizeText(s string, max int) (string, bool) {
	s = strings.TrimSpace(s)
	return s, s != "" && utf8.RuneCountInString(s) <= max
}

type Registration struct {
	BusinessName string
	OwnerName    string
	Email        string
	Password     string
}

// Validate menormalkan input dan mengumpulkan semua pelanggaran sekaligus.
func (r Registration) Validate() (Registration, error) {
	var issues []Issue
	var ok bool

	if r.BusinessName, ok = normalizeText(r.BusinessName, MaxNameLen); !ok {
		issues = append(issues, Issue{"business_name", "Nama usaha wajib diisi (maks 100 karakter)"})
	}
	if r.OwnerName, ok = normalizeText(r.OwnerName, MaxNameLen); !ok {
		issues = append(issues, Issue{"owner_name", "Nama pemilik wajib diisi (maks 100 karakter)"})
	}
	if r.Email, ok = NormalizeEmail(r.Email); !ok {
		issues = append(issues, Issue{"email", "Format email tidak valid"})
	}
	if msg, ok := ValidatePassword(r.Password); !ok {
		issues = append(issues, Issue{"password", msg})
	}
	if len(issues) > 0 {
		return r, &ValidationError{Issues: issues}
	}
	return r, nil
}
