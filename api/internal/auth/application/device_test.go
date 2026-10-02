package application

import "testing"

func TestDeviceSecretIsRandomAndVerifiable(t *testing.T) {
	plain1, hash1, err := newDeviceSecret()
	if err != nil {
		t.Fatal(err)
	}
	plain2, hash2, _ := newDeviceSecret()
	if plain1 == plain2 || string(hash1) == string(hash2) {
		t.Fatal("rahasia harus acak")
	}
	if len(plain1) < 40 || len(hash1) != 32 {
		t.Fatalf("panjang: %d %d", len(plain1), len(hash1))
	}
	if !secretMatches(plain1, hash1) || secretMatches(plain2, hash1) || secretMatches("", hash1) || secretMatches(plain1, nil) {
		t.Fatal("pencocokan rahasia salah")
	}
}
