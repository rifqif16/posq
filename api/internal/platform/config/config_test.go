package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_ED25519_SEED", base64.StdEncoding.EncodeToString(make([]byte, 32)))
}

func TestLoadDefaults(t *testing.T) {
	setValidEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.TrialDays != 14 || cfg.CookieSecure {
		t.Fatalf("default tidak sesuai: %+v", cfg)
	}
}

func TestLoadCollectsAllErrors(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_ED25519_SEED", "bukan-base64")
	t.Setenv("ACCESS_TTL", "abc")
	_, err := Load()
	if err == nil {
		t.Fatal("harus error")
	}
	for _, want := range []string{"DATABASE_URL", "JWT_ED25519_SEED", "ACCESS_TTL"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error harus menyebut %s: %v", want, err)
		}
	}
}
