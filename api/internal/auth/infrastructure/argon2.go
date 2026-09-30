// Package infrastructure (auth): implementasi port hashing dan token.
package infrastructure

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2Params adalah parameter hashing; default sesuai 11.1 (64 MiB, t=3, p=2); disetel ulang lewat benchmark server.
type Argon2Params struct {
	MemoryKiB uint32
	Time      uint32
	Threads   uint8
	SaltLen   uint32
	KeyLen    uint32
}

var DefaultArgon2Params = Argon2Params{MemoryKiB: 64 * 1024, Time: 3, Threads: 2, SaltLen: 16, KeyLen: 32}

type Argon2Hasher struct{ p Argon2Params }

func NewArgon2Hasher(p Argon2Params) *Argon2Hasher { return &Argon2Hasher{p: p} }

// Hash menghasilkan format PHC: $argon2id$v=19$m=..,t=..,p=..$salt$hash
func (h *Argon2Hasher) Hash(password string) (string, error) {
	salt := make([]byte, h.p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("baca salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, h.p.Time, h.p.MemoryKiB, h.p.Threads, h.p.KeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version,
		h.p.MemoryKiB, h.p.Time, h.p.Threads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

var errBadHash = errors.New("format hash tidak dikenal")

// Verify memakai parameter yang tertanam di hash, sehingga hash lama tetap valid
// setelah parameter dinaikkan.
func (h *Argon2Hasher) Verify(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errBadHash
	}
	var version int
	var p Argon2Params
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errBadHash
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.MemoryKiB, &p.Time, &p.Threads); err != nil {
		return false, errBadHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, errBadHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, errBadHash
	}
	got := argon2.IDKey([]byte(password), salt, p.Time, p.MemoryKiB, p.Threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
