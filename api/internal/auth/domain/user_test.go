package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestNormalizeEmail(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"  Owner@Kedai.ID ", "owner@kedai.id", true},
		{"a@b", "", false},
		{"", "", false},
		{"tanpa-at.com", "", false},
		{"Nama <a@b.co>", "", false},
		{strings.Repeat("a", 250) + "@b.co", "", false},
	}
	for _, c := range cases {
		got, ok := NormalizeEmail(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("NormalizeEmail(%q) = %q,%v want %q,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestValidatePasswordBoundaries(t *testing.T) {
	if _, ok := ValidatePassword(strings.Repeat("a", 9)); ok {
		t.Error("9 karakter harus ditolak")
	}
	if _, ok := ValidatePassword(strings.Repeat("a", 10)); !ok {
		t.Error("10 karakter harus diterima")
	}
	if _, ok := ValidatePassword(strings.Repeat("a", 128)); !ok {
		t.Error("128 karakter harus diterima")
	}
	if _, ok := ValidatePassword(strings.Repeat("a", 129)); ok {
		t.Error("129 karakter harus ditolak")
	}
}

func TestRegistrationValidateCollectsAllIssues(t *testing.T) {
	_, err := Registration{}.Validate()
	ve, ok := err.(*ValidationError)
	if !ok || len(ve.Issues) != 4 {
		t.Fatalf("harus 4 issue, dapat %v", err)
	}
}

func TestRegistrationValidateNormalizes(t *testing.T) {
	r, err := Registration{BusinessName: " Kopi ", OwnerName: " Sari ", Email: "S@X.ID", Password: "password-aman"}.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if r.BusinessName != "Kopi" || r.OwnerName != "Sari" || r.Email != "s@x.id" {
		t.Fatalf("tidak ternormalisasi: %+v", r)
	}
}

func TestRefreshTokenRoundTrip(t *testing.T) {
	tenant := uuid.New()
	plain, hash, err := NewRefreshToken(tenant)
	if err != nil {
		t.Fatal(err)
	}
	gotTenant, gotHash, err := ParseRefreshToken(plain)
	if err != nil || gotTenant != tenant || string(gotHash) != string(hash) {
		t.Fatalf("round trip gagal: %v", err)
	}
}

func TestParseRefreshTokenRejectsMalformed(t *testing.T) {
	for _, in := range []string{"", "abc", "x.y", uuid.NewString() + ".pendek", uuid.NewString() + ".!!!"} {
		if _, _, err := ParseRefreshToken(in); err == nil {
			t.Errorf("%q harus ditolak", in)
		}
	}
}
