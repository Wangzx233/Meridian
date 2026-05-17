package control

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type API struct {
	store       *Store
	hub         *EventHub
	terminalHub *TerminalHub
	runners     *RunnerManager
	logger      *slog.Logger
	auth        AuthConfig
}

func NewAPI(store *Store, logger *slog.Logger, auth AuthConfig) *API {
	if logger == nil {
		logger = slog.Default()
	}
	return &API{
		store:       store,
		hub:         NewEventHub(),
		terminalHub: NewTerminalHub(),
		runners:     NewRunnerManager(),
		logger:      logger,
		auth:        auth,
	}
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", a.handleAuthLogin)
	mux.HandleFunc("/api/v1/auth/logout", a.handleAuthLogout)
	mux.HandleFunc("/api/v1/auth/session", a.handleAuthSession)
	mux.HandleFunc("/api/v1/auth/setup", a.handleAuthSetup)
	mux.HandleFunc("/api/v1/servers", a.handleServers)
	mux.HandleFunc("/api/v1/servers/", a.handleServerByID)
	mux.HandleFunc("/api/v1/projects", a.handleProjects)
	mux.HandleFunc("/api/v1/projects/", a.handleProjectRoutes)
	mux.HandleFunc("/api/v1/tasks/", a.handleTaskRoutes)
	mux.HandleFunc("/api/v1/context-items/", a.handleContextItemByID)
	mux.HandleFunc("/api/v1/notifications", a.handleWorkbenchNotifications)
	mux.HandleFunc("/api/v1/notifications/", a.handleWorkbenchNotificationByID)
	mux.HandleFunc("/api/v1/settings/email-notifications", a.handleEmailNotificationConfigs)
	mux.HandleFunc("/api/v1/settings/email-notifications/", a.handleEmailNotificationConfigByID)
	mux.HandleFunc("/api/v1/runs/", a.handleRunRoutes)
	mux.HandleFunc("/api/v1/runners/update-all", a.handleRunnerUpdateAll)
	mux.HandleFunc("/api/v1/runner/ws", a.handleRunnerWS)
	mux.HandleFunc("/api/v1/runner/install.ps1", a.handleRunnerInstallPowerShell)
	mux.HandleFunc("/api/v1/runner/install.sh", a.handleRunnerInstallShell)
	mux.HandleFunc("/api/v1/runner/artifacts/", a.handleRunnerArtifact)
	return withRequestLogging(a.requireAuth(mux), a.logger)
}

func (a *API) respondList(w http.ResponseWriter, items any, err error) {
	if err != nil {
		a.respond(w, http.StatusOK, nil, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": nil})
}

func (a *API) respond(w http.ResponseWriter, status int, item any, err error) {
	if err != nil {
		httpStatus, code, msg := statusForError(err)
		writeError(w, httpStatus, code, msg, nil)
		return
	}
	writeJSON(w, status, item)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid JSON request body.", nil)
		return false
	}
	return true
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "validation_error", "Method not allowed.", nil)
}

func splitPath(s string) []string {
	s = strings.Trim(s, "/")
	if s == "" {
		return nil
	}
	return strings.Split(s, "/")
}

func trimPrefix(s, prefix string) string {
	return strings.TrimPrefix(s, prefix)
}

func publicControlURL(r *http.Request) string {
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		if r.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return strings.TrimRight(proto+"://"+host, "/")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseSeq(s string) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

func rawString(raw json.RawMessage) string {
	var value string
	_ = json.Unmarshal(raw, &value)
	return strings.TrimSpace(value)
}

func withRequestLogging(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Debug("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}

func (a *API) publishAndMaybeAssign(ctx context.Context, runnerID string) {
	assignments, err := a.store.NextQueuedRunsForRunner(ctx, runnerID, 10)
	if err != nil {
		a.logger.Warn("load queued runs failed", "runner_id", runnerID, "error", err)
		return
	}
	for _, assign := range assignments {
		if err := a.runners.SendAssign(assign); err != nil {
			a.logger.Warn("send queued run failed", "runner_id", runnerID, "run_id", assign.RunID, "error", err)
		}
	}
}

func (a *API) hydrateServers(servers []Server) {
	for i := range servers {
		a.hydrateServer(&servers[i])
	}
}

func (a *API) hydrateServer(server *Server) {
	if server == nil || server.RunnerID == "" {
		return
	}
	server.RunnerConnected = a.runners.Connected(server.RunnerID)
	if server.RunnerConnected {
		server.RunnerConnection = a.runners.Info(server.RunnerID)
		server.RunnerCapabilities = a.runners.Capabilities(server.RunnerID)
	}
}
