package infrastructure

import (
	"crypto/ed25519"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/auth/application"
	"github.com/rifqif16/posq/api/internal/auth/domain"
)

var fastParams = Argon2Params{MemoryKiB: 8, Time: 1, Threads: 1, SaltLen: 16, KeyLen: 32}

func TestArgon2HashVerify(t *testing.T) {
	h := NewArgon2Hasher(fastParams)
	enc, err := h.Hash("rahasia-panjang")
	if err != nil || !strings.HasPrefix(enc, "$argon2id$v=19$m=8,t=1,p=1$") {
		t.Fatalf("hash: %q %v", enc, err)
	}
	if ok, err := h.Verify("rahasia-panjang", enc); !ok || err != nil {
		t.Fatalf("password benar harus lolos: %v %v", ok, err)
	}
	if ok, _ := h.Verify("salah-salah!", enc); ok {
		t.Fatal("password salah harus gagal")
	}
	enc2, _ := h.Hash("rahasia-panjang")
	if enc == enc2 {
		t.Fatal("salt harus unik per hash")
	}
}

func TestArgon2VerifyRejectsMalformed(t *testing.T) {
	h := NewArgon2Hasher(fastParams)
	for _, in := range []string{"", "plain", "$argon2i$v=19$m=8,t=1,p=1$YQ$YQ", "$argon2id$v=19$m=8,t=1,p=1$!!$YQ"} {
		if ok, err := h.Verify("x", in); ok || err == nil {
			t.Errorf("%q harus error", in)
		}
	}
}

func newIssuer(t *testing.T) (*JWTIssuer, ed25519.PrivateKey) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	return NewJWTIssuer(priv), priv
}

func TestJWTRoundTripAndExpiry(t *testing.T) {
	j, _ := newIssuer(t)
	now := time.Now()
	in := application.AccessClaims{
		UserID: uuid.New(), TenantID: uuid.New(), Role: domain.RoleOwner, StoreIDs: []uuid.UUID{uuid.New()},
	}
	tok, err := j.Issue(in, now, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	out, err := j.Parse(tok, now.Add(time.Minute))
	if err != nil || out.UserID != in.UserID || out.TenantID != in.TenantID || out.Role != in.Role || len(out.StoreIDs) != 1 {
		t.Fatalf("round trip gagal: %+v %v", out, err)
	}
	if _, err := j.Parse(tok, now.Add(16*time.Minute)); err == nil {
		t.Fatal("token kedaluwarsa harus ditolak")
	}
}

func TestJWTRejectsForeignKeyAndTampering(t *testing.T) {
	a, _ := newIssuer(t)
	b, _ := newIssuer(t)
	now := time.Now()
	tok, _ := a.Issue(application.AccessClaims{UserID: uuid.New(), TenantID: uuid.New(), Role: domain.RoleAdmin}, now, time.Minute)
	if _, err := b.Parse(tok, now); err == nil {
		t.Fatal("token dari kunci lain harus ditolak")
	}
	if _, err := a.Parse(tok+"x", now); err == nil {
		t.Fatal("token yang diubah harus ditolak")
	}
	if _, err := a.Parse("bukan.jwt.sama-sekali", now); err == nil {
		t.Fatal("token sampah harus ditolak")
	}
}
