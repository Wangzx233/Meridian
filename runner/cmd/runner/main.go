package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"codex-task-workbench/runner/internal/agent"
)

var RunnerVersion = "0.5.0"

func main() {
	hostname, _ := os.Hostname()
	cfg := agent.Config{
		ControlURL:        env("CONTROL_URL", "http://localhost:8080"),
		RunnerID:          env("RUNNER_ID", hostname),
		Hostname:          hostname,
		Version:           RunnerVersion,
		CodexPath:         env("CODEX_PATH", "codex"),
		RunnerToken:       os.Getenv("RUNNER_TOKEN"),
		UseCodexSandbox:   !boolEnv("CODEX_BYPASS_APPROVALS_AND_SANDBOX", true),
		CompactTimeout:    durationEnv("COMPACT_TIMEOUT", 5*time.Minute),
		HeartbeatInterval: durationEnv("HEARTBEAT_INTERVAL", 10*time.Second),
		Env:               splitEnv(os.Getenv("RUNNER_ENV")),
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := agent.New(cfg, slog.Default()).Run(ctx); err != nil && ctx.Err() == nil {
		slog.Error("runner failed", "error", err)
		os.Exit(1)
	}
}

func env(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func boolEnv(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func splitEnv(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
