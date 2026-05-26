package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const reminderRegistrationTTL = 24 * time.Hour

type reminderCallbackServer struct {
	logger *slog.Logger
	send   func(string, any) error

	mu      sync.Mutex
	server  *http.Server
	address string
	tokens  map[string]reminderRegistration
}

type reminderRegistration struct {
	RunID     string
	Token     string
	URL       string
	ExpiresAt time.Time
}

func newReminderCallbackServer(logger *slog.Logger, send func(string, any) error) *reminderCallbackServer {
	if logger == nil {
		logger = slog.Default()
	}
	return &reminderCallbackServer{
		logger: logger,
		send:   send,
		tokens: map[string]reminderRegistration{},
	}
}

func (s *reminderCallbackServer) register(runID string) (reminderRegistration, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return reminderRegistration{}, errors.New("run id is required")
	}
	if err := s.ensureStarted(); err != nil {
		return reminderRegistration{}, err
	}
	token, err := randomToken()
	if err != nil {
		return reminderRegistration{}, err
	}
	now := time.Now().UTC()
	reg := reminderRegistration{
		RunID:     runID,
		Token:     token,
		URL:       "http://" + s.address + "/notify",
		ExpiresAt: now.Add(reminderRegistrationTTL),
	}
	s.mu.Lock()
	s.cleanupLocked(now)
	s.tokens[token] = reg
	s.mu.Unlock()
	return reg, nil
}

func (s *reminderCallbackServer) ensureStarted() error {
	s.mu.Lock()
	if s.server != nil {
		s.mu.Unlock()
		return nil
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		s.mu.Unlock()
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/notify", s.handleNotify)
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	s.server = server
	s.address = listener.Addr().String()
	s.mu.Unlock()

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Warn("reminder callback server stopped", "error", err)
		}
	}()
	return nil
}

func (s *reminderCallbackServer) handleNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	now := time.Now().UTC()
	s.mu.Lock()
	s.cleanupLocked(now)
	reg, ok := s.tokens[token]
	s.mu.Unlock()
	if !ok || now.After(reg.ExpiresAt) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	defer r.Body.Close()
	var payload struct {
		Title   string `json:"title"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8192)).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.Title) == "" && strings.TrimSpace(payload.Message) == "" {
		http.Error(w, "title or message is required", http.StatusBadRequest)
		return
	}
	err := s.send("run.reminder", map[string]any{
		"run_id":  reg.RunID,
		"title":   strings.TrimSpace(payload.Title),
		"message": strings.TrimSpace(payload.Message),
		"sent_at": now,
	})
	if err != nil {
		http.Error(w, "runner is not connected", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
}

func (s *reminderCallbackServer) cleanupLocked(now time.Time) {
	for token, reg := range s.tokens {
		if now.After(reg.ExpiresAt) {
			delete(s.tokens, token)
		}
	}
}

func (s *reminderCallbackServer) shutdown(ctx context.Context) error {
	s.mu.Lock()
	server := s.server
	s.server = nil
	s.address = ""
	s.tokens = map[string]reminderRegistration{}
	s.mu.Unlock()
	if server == nil {
		return nil
	}
	return server.Shutdown(ctx)
}

func randomToken() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
