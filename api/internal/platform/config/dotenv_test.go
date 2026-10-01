package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDotEnvParsesAndKeepsExistingEnv(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"), `# komentar
POSQ_T_PLAIN=satu
export POSQ_T_EXPORT=dua
POSQ_T_DQUOTE="tiga empat"
POSQ_T_SQUOTE='lima'
POSQ_T_EMPTY=
POSQ_T_EQ=a=b==
POSQ_T_EXISTING=dari-file

`)
	t.Setenv("POSQ_T_EXISTING", "dari-env")
	for _, k := range []string{"POSQ_T_PLAIN", "POSQ_T_EXPORT", "POSQ_T_DQUOTE", "POSQ_T_SQUOTE", "POSQ_T_EMPTY", "POSQ_T_EQ"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}

	path, err := LoadDotEnv(dir)
	if err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}
	if path != filepath.Join(dir, ".env") {
		t.Fatalf("path = %q", path)
	}
	want := map[string]string{
		"POSQ_T_PLAIN":    "satu",
		"POSQ_T_EXPORT":   "dua",
		"POSQ_T_DQUOTE":   "tiga empat",
		"POSQ_T_SQUOTE":   "lima",
		"POSQ_T_EQ":       "a=b==",
		"POSQ_T_EXISTING": "dari-env",
	}
	for k, v := range want {
		if got := os.Getenv(k); got != v {
			t.Errorf("%s = %q, mau %q", k, got, v)
		}
	}
	if v, ok := os.LookupEnv("POSQ_T_EMPTY"); !ok || v != "" {
		t.Errorf("POSQ_T_EMPTY harus ada dan kosong, dapat %q ok=%v", v, ok)
	}
}

func TestLoadDotEnvSearchesUpToRepoRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "api", "cmd")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, ".env"), "POSQ_T_UP=ketemu\n")
	t.Setenv("POSQ_T_UP", "")
	os.Unsetenv("POSQ_T_UP")

	path, err := LoadDotEnv(sub)
	if err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}
	if path != filepath.Join(root, ".env") || os.Getenv("POSQ_T_UP") != "ketemu" {
		t.Fatalf("path=%q POSQ_T_UP=%q", path, os.Getenv("POSQ_T_UP"))
	}
}

func TestLoadDotEnvStopsAtRepoRoot(t *testing.T) {
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, ".env"), "POSQ_T_LEAK=bocor\n")
	repo := filepath.Join(outside, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("POSQ_T_LEAK", "")
	os.Unsetenv("POSQ_T_LEAK")

	path, err := LoadDotEnv(repo)
	if err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}
	if path != "" || os.Getenv("POSQ_T_LEAK") != "" {
		t.Fatalf(".env di luar repo tidak boleh dimuat: path=%q", path)
	}
}

func TestLoadDotEnvRejectsMalformedLine(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"), "POSQ_T_OK=1\nbaris tanpa sama dengan\n")
	if _, err := LoadDotEnv(dir); err == nil {
		t.Fatal("harus error untuk baris tanpa '='")
	}
}
