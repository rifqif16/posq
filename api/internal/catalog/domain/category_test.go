package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateCategory(t *testing.T) {
	name, err := ValidateCategory("  Minuman  ", 0)
	if err != nil || name != "Minuman" {
		t.Fatalf("valid: %q %v", name, err)
	}
	if _, err := ValidateCategory(strings.Repeat("a", 100), MaxSortOrder); err != nil {
		t.Fatalf("batas atas harus valid: %v", err)
	}

	for _, c := range []struct {
		name string
		sort int
		n    int
	}{
		{"", 0, 1}, {"   ", 0, 1}, {strings.Repeat("a", 101), 0, 1},
		{"ok", -1, 1}, {"ok", MaxSortOrder + 1, 1}, {"", -1, 2},
	} {
		_, err := ValidateCategory(c.name, c.sort)
		var ve *ValidationError
		if !errors.As(err, &ve) || len(ve.Issues) != c.n {
			t.Errorf("(%q,%d): harus %d issue, dapat %v", c.name, c.sort, c.n, err)
		}
	}
}
