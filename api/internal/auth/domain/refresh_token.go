package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const refreshSecretBytes = 32 // 256-bit (11.1)

var ErrMalformedRefreshToken = errors.New("refresh token tidak valid")

// NewRefreshToken membuat token berformat "<tenant_id>.<secret>". Awalan tenant_id
// memungkinkan lookup ber-RLS tanpa fungsi lintas-tenant. Hanya hash SHA-256 yang disimpan.
func NewRefreshToken(tenantID uuid.UUID) (plain string, hash []byte, err error) {
	secret := make([]byte, refreshSecretBytes)
	if _, err := rand.Read(secret); err != nil {
		return "", nil, fmt.Errorf("baca random: %w", err)
	}
	plain = tenantID.String() + "." + base64.RawURLEncoding.EncodeToString(secret)
	return plain, HashRefreshToken(plain), nil
}

func HashRefreshToken(plain string) []byte {
	sum := sha256.Sum256([]byte(plain))
	return sum[:]
}

// ParseRefreshToken memvalidasi bentuk token dan mengembalikan tenant serta hash-nya.
func ParseRefreshToken(plain string) (uuid.UUID, []byte, error) {
	tenantPart, secretPart, found := strings.Cut(plain, ".")
	if !found {
		return uuid.Nil, nil, ErrMalformedRefreshToken
	}
	tenantID, err := uuid.Parse(tenantPart)
	if err != nil {
		return uuid.Nil, nil, ErrMalformedRefreshToken
	}
	secret, err := base64.RawURLEncoding.DecodeString(secretPart)
	if err != nil || len(secret) != refreshSecretBytes {
		return uuid.Nil, nil, ErrMalformedRefreshToken
	}
	return tenantID, HashRefreshToken(plain), nil
}
