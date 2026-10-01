// Perintah: migrate [up|down|status]. Memakai MIGRATE_DATABASE_URL (role owner tabel).
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/rifqif16/posq/api/db"
	"github.com/rifqif16/posq/api/internal/platform/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run() error {
	if _, err := config.LoadDotEnv("."); err != nil {
		return err
	}
	url := os.Getenv("MIGRATE_DATABASE_URL")
	if url == "" {
		return fmt.Errorf("MIGRATE_DATABASE_URL wajib diisi")
	}
	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	conn, err := sql.Open("pgx", url)
	if err != nil {
		return err
	}
	defer conn.Close()

	migrations, err := db.Migrations()
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, conn, migrations)
	if err != nil {
		return err
	}
	ctx := context.Background()

	switch cmd {
	case "up":
		results, err := provider.Up(ctx)
		printResults(results)
		return err
	case "down": // hanya satu langkah, sengaja tidak ada down-to-zero
		result, err := provider.Down(ctx)
		if result != nil {
			printResults([]*goose.MigrationResult{result})
		}
		return err
	case "status":
		statuses, err := provider.Status(ctx)
		for _, s := range statuses {
			fmt.Printf("%-30s %s\n", s.Source.Path, s.State)
		}
		return err
	}
	return fmt.Errorf("perintah tidak dikenal: %s (up|down|status)", cmd)
}

func printResults(results []*goose.MigrationResult) {
	for _, r := range results {
		fmt.Println(r.String())
	}
}
