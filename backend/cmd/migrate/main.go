package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] != "up" {
		fmt.Fprintf(os.Stderr, "usage: %s [up]\n", os.Args[0])
		os.Exit(2)
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = "db/migrations"
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		slog.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)

	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		slog.Error("read migrations", "error", err)
		os.Exit(1)
	}
	sort.Strings(files)
	for _, file := range files {
		if strings.HasSuffix(file, ".down.sql") {
			continue
		}
		version := strings.TrimSuffix(filepath.Base(file), ".sql")
		if applied(ctx, conn, version) {
			slog.Info("migration already applied", "version", version)
			continue
		}
		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			slog.Error("read migration", "file", file, "error", err)
			os.Exit(1)
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			slog.Error("begin migration", "version", version, "error", err)
			os.Exit(1)
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			slog.Error("apply migration", "version", version, "error", err)
			os.Exit(1)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT DO NOTHING`, version); err != nil {
			_ = tx.Rollback(ctx)
			slog.Error("record migration", "version", version, "error", err)
			os.Exit(1)
		}
		if err := tx.Commit(ctx); err != nil {
			slog.Error("commit migration", "version", version, "error", err)
			os.Exit(1)
		}
		slog.Info("migration applied", "version", version)
	}
}

func applied(ctx context.Context, conn *pgx.Conn, version string) bool {
	var exists bool
	err := conn.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM information_schema.tables
		WHERE table_schema='public' AND table_name='schema_migrations'
	)`).Scan(&exists)
	if err != nil || !exists {
		return false
	}
	err = conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	return err == nil && exists
}
