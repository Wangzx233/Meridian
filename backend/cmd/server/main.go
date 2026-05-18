package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"codex-task-workbench/backend/internal/control"
	"codex-task-workbench/backend/internal/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	addr := env("BACKEND_ADDR", ":8080")
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if env("MERIDIAN_AUTO_MIGRATE", "true") != "false" {
		if err := migrations.Up(ctx, dsn, os.Getenv("MIGRATIONS_DIR"), slog.Default()); err != nil {
			slog.Error("migrate database", "error", err)
			os.Exit(1)
		}
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		slog.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("ping database", "error", err)
		os.Exit(1)
	}

	auth, err := control.LoadAuthConfigFromEnv()
	if err != nil {
		slog.Error("load auth config", "error", err)
		os.Exit(1)
	}

	api := control.NewAPI(control.NewStore(pool), slog.Default(), auth)
	server := &http.Server{
		Addr:              addr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("backend listening", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("http shutdown failed", "error", err)
	}
}

func env(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
