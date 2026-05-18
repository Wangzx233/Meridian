package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"codex-task-workbench/backend/internal/migrations"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] != "up" {
		fmt.Fprintf(os.Stderr, "usage: %s [up]\n", os.Args[0])
		os.Exit(2)
	}
	if err := migrations.Up(context.Background(), os.Getenv("DATABASE_URL"), os.Getenv("MIGRATIONS_DIR"), slog.Default()); err != nil {
		slog.Error("migrate up", "error", err)
		os.Exit(1)
	}
}
