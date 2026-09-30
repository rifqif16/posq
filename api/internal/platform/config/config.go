// Package config membaca konfigurasi proses dari environment.
package config

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	JWTPrivateKey   ed25519.PrivateKey
	AccessTTL       time.Duration
	RefreshTTL      time.Duration
	TrialDays       int
	CookieSecure    bool
	TrustedIPHeader string
}

// Load membaca dan memvalidasi environment. Semua kesalahan dikumpulkan.
func Load() (Config, error) {
	var errs []error
	cfg := Config{
		HTTPAddr:        getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		TrustedIPHeader: os.Getenv("TRUSTED_IP_HEADER"),
	}
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL wajib diisi"))
	}

	key, err := parseSeed(os.Getenv("JWT_ED25519_SEED"))
	if err != nil {
		errs = append(errs, err)
	}
	cfg.JWTPrivateKey = key

	cfg.AccessTTL = duration("ACCESS_TTL", 15*time.Minute, &errs)
	cfg.RefreshTTL = duration("REFRESH_TTL", 720*time.Hour, &errs)
	cfg.TrialDays = integer("TRIAL_DAYS", 14, &errs)
	cfg.CookieSecure = getenv("COOKIE_SECURE", "false") == "true"

	return cfg, errors.Join(errs...)
}

func parseSeed(raw string) (ed25519.PrivateKey, error) {
	seed, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("JWT_ED25519_SEED harus base64 dari %d byte", ed25519.SeedSize)
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func duration(key string, def time.Duration, errs *[]error) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		*errs = append(*errs, fmt.Errorf("%s tidak valid: %q", key, v))
		return def
	}
	return d
}

func integer(key string, def int, errs *[]error) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		*errs = append(*errs, fmt.Errorf("%s tidak valid: %q", key, v))
		return def
	}
	return n
}
