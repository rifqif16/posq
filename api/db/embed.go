// Package db menyimpan file migrasi SQL yang di-embed ke dalam binary.
package db

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrations mengembalikan FS berisi file migrasi goose.
func Migrations() (fs.FS, error) {
	return fs.Sub(migrationsFS, "migrations")
}
