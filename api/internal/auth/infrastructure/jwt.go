package infrastructure

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/auth/application"
	"github.com/rifqif16/posq/api/internal/auth/domain"
)

const (
	jwtIssuer = "posq"
	jwtKeyID  = "v1" // siap rotasi kunci (11.1)
)

type accessClaims struct {
	Tenant string   `json:"tenant"`
	Role   string   `json:"role"`
	Stores []string `json:"stores"`
	jwt.RegisteredClaims
}

type JWTIssuer struct {
	priv ed25519.PrivateKey
	pub  ed25519.PublicKey
}

func NewJWTIssuer(priv ed25519.PrivateKey) *JWTIssuer {
	return &JWTIssuer{priv: priv, pub: priv.Public().(ed25519.PublicKey)}
}

func (j *JWTIssuer) Issue(c application.AccessClaims, now time.Time, ttl time.Duration) (string, error) {
	stores := make([]string, len(c.StoreIDs))
	for i, id := range c.StoreIDs {
		stores[i] = id.String()
	}
	claims := accessClaims{
		Tenant: c.TenantID.String(), Role: string(c.Role), Stores: stores,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   c.UserID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = jwtKeyID
	signed, err := tok.SignedString(j.priv)
	if err != nil {
		return "", fmt.Errorf("tanda tangan jwt: %w", err)
	}
	return signed, nil
}

func (j *JWTIssuer) Parse(token string, now time.Time) (application.AccessClaims, error) {
	var claims accessClaims
	_, err := jwt.ParseWithClaims(token, &claims,
		func(*jwt.Token) (any, error) { return j.pub, nil },
		jwt.WithValidMethods([]string{"EdDSA"}),
		jwt.WithIssuer(jwtIssuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	if err != nil {
		return application.AccessClaims{}, err
	}
	return toAccessClaims(claims)
}

func toAccessClaims(c accessClaims) (application.AccessClaims, error) {
	uid, err := uuid.Parse(c.Subject)
	if err != nil {
		return application.AccessClaims{}, errors.New("sub tidak valid")
	}
	tid, err := uuid.Parse(c.Tenant)
	if err != nil {
		return application.AccessClaims{}, errors.New("tenant tidak valid")
	}
	stores := make([]uuid.UUID, 0, len(c.Stores))
	for _, s := range c.Stores {
		id, err := uuid.Parse(s)
		if err != nil {
			return application.AccessClaims{}, errors.New("store tidak valid")
		}
		stores = append(stores, id)
	}
	return application.AccessClaims{UserID: uid, TenantID: tid, Role: domain.Role(c.Role), StoreIDs: stores}, nil
}
