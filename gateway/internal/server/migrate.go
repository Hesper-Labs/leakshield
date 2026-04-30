package server

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/Hesper-Labs/leakshield/gateway/internal/config"
)

//go:embed all:../migrations
var migrationsFS embed.FS

// RunMigrate executes a goose migration operation against the configured database.
func RunMigrate(ctx context.Context, cfg *config.Config, op string) error {
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	switch op {
	case "up":
		return goose.UpContext(ctx, db, "../migrations")
	case "down":
		return goose.DownContext(ctx, db, "../migrations")
	case "status":
		return goose.StatusContext(ctx, db, "../migrations")
	case "reset":
		return goose.ResetContext(ctx, db, "../migrations")
	default:
		return fmt.Errorf("unknown migrate op %q (expected up|down|status|reset)", op)
	}
}
