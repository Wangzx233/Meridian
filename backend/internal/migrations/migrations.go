package migrations

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

func Up(ctx context.Context, dsn, migrationsDir string, logger *slog.Logger) error {
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}
	if migrationsDir == "" {
		migrationsDir = "db/migrations"
	}
	if logger == nil {
		logger = slog.Default()
	}

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer conn.Close(ctx)

	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	sort.Strings(files)
	for _, file := range files {
		if strings.HasSuffix(file, ".down.sql") {
			continue
		}
		version := strings.TrimSuffix(filepath.Base(file), ".sql")
		applied, err := applied(ctx, conn, version)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", version, err)
		}
		if applied {
			logger.Info("migration already applied", "version", version)
			continue
		}
		if err := applyFile(ctx, conn, file, version); err != nil {
			return err
		}
		logger.Info("migration applied", "version", version)
	}
	return nil
}

func applyFile(ctx context.Context, conn *pgx.Conn, file, version string) error {
	sqlBytes, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", file, err)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", version, err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
		return fmt.Errorf("apply migration %s: %w", version, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT DO NOTHING`, version); err != nil {
		return fmt.Errorf("record migration %s: %w", version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %s: %w", version, err)
	}
	return nil
}

func applied(ctx context.Context, conn *pgx.Conn, version string) (bool, error) {
	var exists bool
	err := conn.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM information_schema.tables
		WHERE table_schema='public' AND table_name='schema_migrations'
	)`).Scan(&exists)
	if err != nil || !exists {
		return false, err
	}
	err = conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return exists, err
}
