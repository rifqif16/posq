package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	MaxNameLen = 100
	MaxSortOrder = 10000
)

type Category struct {
	ID uuid.UUID
	ParentID *uuid.UUID
	Name string
	SortOrder int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Issue struct {
	Field string
	Message string
}

type ValidationError struct {Issues []Issue}

func (e *ValidationError) Error() string {return "validasi gagal"}

func ValidateCategory(name string, sortOrder int) (string, error) {
	var issues []Issue
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > MaxNameLen {
		issues = append(issues, Issue{"name", "Nama kategori wajib diisi (maks 100 karakter)"})
	}
	if sortOrder < 0 || sortOrder > MaxSortOrder {
		issues = append(issues, Issue{"sort_order", "Urutan harus antara 0 dan 10000"})
	}
	if len(issues) > 0 {
		return name, &ValidationError{Issues: issues}
	}
	return name, nil
}
