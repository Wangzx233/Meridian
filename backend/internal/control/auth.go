package control

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const authCookieName = "ctw_session"

type AuthConfig struct {
	Users        map[string]string
	SessionKey   []byte
	RunnerToken  string
	CookieSecure bool
}

func LoadAuthConfigFromEnv() (AuthConfig, error) {
	users, err := parseAuthUsers(os.Getenv("WORKBENCH_AUTH_USERS"))
	if err != nil {
		return AuthConfig{}, err
	}
	secret := strings.TrimSpace(os.Getenv("WORKBENCH_AUTH_SESSION_SECRET"))
	if len(users) > 0 && secret == "" {
		return AuthConfig{}, errors.New("WORKBENCH_AUTH_SESSION_SECRET is required when WORKBENCH_AUTH_USERS is set")
	}
	runnerToken := strings.TrimSpace(os.Getenv("WORKBENCH_RUNNER_TOKEN"))
	if len(users) > 0 && runnerToken == "" {
		return AuthConfig{}, errors.New("WORKBENCH_RUNNER_TOKEN is required when WORKBENCH_AUTH_USERS is set")
	}
	if len(users) == 0 {
		secret = ""
		runnerToken = ""
	}
	cfg := AuthConfig{
		Users:       users,
		SessionKey:  []byte(secret),
		RunnerToken: runnerToken,
	}
	cfg.CookieSecure = parseBoolEnv("WORKBENCH_AUTH_COOKIE_SECURE", true)
	return cfg, nil
}

func parseAuthUsers(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	users := map[string]string{}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		username, password, ok := strings.Cut(entry, ":")
		username = strings.TrimSpace(username)
		password = strings.TrimSpace(password)
		if !ok || username == "" || password == "" {
			return nil, errors.New("WORKBENCH_AUTH_USERS entries must use username:password")
		}
		users[username] = password
	}
	return users, nil
}

func parseBoolEnv(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func (a *API) authEnabled() bool {
	return a.authConfigured(context.Background())
}

func (a *API) authDisabledForLocalDev() bool {
	return len(a.auth.Users) == 0 && a.store == nil
}

func (a *API) authConfigured(ctx context.Context) bool {
	if len(a.auth.Users) > 0 {
		return true
	}
	if a.store == nil {
		return false
	}
	ok, err := a.store.HasAuthUsers(ctx)
	if err != nil {
		a.logger.Error("check auth users", "error", err)
		return false
	}
	return ok
}

func (a *API) setupRequired(ctx context.Context) bool {
	if len(a.auth.Users) > 0 || a.store == nil {
		return false
	}
	ok, err := a.store.HasAuthUsers(ctx)
	if err != nil {
		a.logger.Error("check auth setup state", "error", err)
		return false
	}
	return !ok
}

func (a *API) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if a.setupRequired(r.Context()) {
		writeError(w, http.StatusConflict, "invalid_state", "Create the first access account before signing in.", nil)
		return
	}
	if a.authDisabledForLocalDev() {
		a.writeAuthSession(w, "", "")
		return
	}
	if !a.authConfigured(r.Context()) {
		writeError(w, http.StatusInternalServerError, "internal_error", "Unable to determine authentication state.", nil)
		return
	}
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	username := strings.TrimSpace(in.Username)
	user, ok := a.verifyLogin(r.Context(), username, in.Password)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid username or password.", nil)
		return
	}
	a.setSessionCookie(r.Context(), w, username)
	a.writeAuthSession(w, username, user.runnerToken)
}

func (a *API) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	a.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
}

func (a *API) handleAuthSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	if a.setupRequired(r.Context()) {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false, "setup_required": true})
		return
	}
	if a.authDisabledForLocalDev() {
		a.writeAuthSession(w, "", "")
		return
	}
	if !a.authConfigured(r.Context()) {
		writeError(w, http.StatusInternalServerError, "internal_error", "Unable to determine authentication state.", nil)
		return
	}
	username, runnerToken, ok := a.sessionUsername(r)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	a.writeAuthSession(w, username, runnerToken)
}

func (a *API) handleAuthSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if !a.setupRequired(r.Context()) {
		writeError(w, http.StatusConflict, "invalid_state", "Workbench authentication is already configured.", nil)
		return
	}
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	username := strings.TrimSpace(in.Username)
	if username == "" || len([]rune(username)) > 80 || strings.ContainsAny(username, " \t\r\n,:") {
		writeError(w, http.StatusBadRequest, "validation_error", "Username is required and cannot contain whitespace, comma, or colon.", nil)
		return
	}
	if len(in.Password) < 8 {
		writeError(w, http.StatusBadRequest, "validation_error", "Password must be at least 8 characters.", nil)
		return
	}
	if a.store == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Setup requires a database store.", nil)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		a.logger.Error("hash setup password", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error.", nil)
		return
	}
	sessionSecret := randomSecretHex(32)
	runnerToken := randomSecretHex(32)
	if err := a.store.InitializeAuth(r.Context(), username, string(hash), sessionSecret, runnerToken); err != nil {
		a.respond(w, http.StatusCreated, nil, err)
		return
	}
	a.setSessionCookie(r.Context(), w, username)
	a.writeAuthSession(w, username, runnerToken)
}

func (a *API) writeAuthSession(w http.ResponseWriter, username, runnerToken string) {
	payload := map[string]any{
		"authenticated": true,
		"username":      username,
	}
	if runnerToken == "" {
		runnerToken = a.runnerToken(contextOrBackground(nil))
	}
	if runnerToken != "" {
		payload["runner_token"] = runnerToken
	}
	writeJSON(w, http.StatusOK, payload)
}

func (a *API) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.isPublicAuthRoute(r.URL.Path) || a.isAuthorizedRunnerRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		if a.authDisabledForLocalDev() {
			next.ServeHTTP(w, r)
			return
		}
		if !a.authConfigured(r.Context()) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Login required.", nil)
			return
		}
		if _, _, ok := a.sessionUsername(r); ok {
			next.ServeHTTP(w, r)
			return
		}
		writeError(w, http.StatusUnauthorized, "unauthorized", "Login required.", nil)
	})
}

func (a *API) isPublicAuthRoute(path string) bool {
	switch path {
	case "/api/v1/build", "/api/v1/auth/login", "/api/v1/auth/logout", "/api/v1/auth/session", "/api/v1/auth/setup":
		return true
	default:
		return false
	}
}

func (a *API) isAuthorizedRunnerRequest(r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, "/api/v1/runner/") {
		return false
	}
	runnerToken := a.runnerToken(r.Context())
	if runnerToken == "" {
		return a.authDisabledForLocalDev()
	}
	token := runnerTokenFromRequest(r)
	return subtle.ConstantTimeCompare([]byte(token), []byte(runnerToken)) == 1
}

func runnerTokenFromRequest(r *http.Request) string {
	if token := strings.TrimSpace(r.URL.Query().Get("runner_token")); token != "" {
		return token
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "Bearer "
	if len(auth) > len(prefix) && strings.EqualFold(auth[:len(prefix)], prefix) {
		return strings.TrimSpace(auth[len(prefix):])
	}
	return strings.TrimSpace(r.Header.Get("X-Runner-Token"))
}

func (a *API) sessionUsername(r *http.Request) (string, string, bool) {
	if a.authDisabledForLocalDev() {
		return "", "", true
	}
	if !a.authConfigured(r.Context()) {
		return "", "", false
	}
	cookie, err := r.Cookie(authCookieName)
	if err != nil {
		return "", "", false
	}
	username, runnerToken, ok := a.verifySession(r.Context(), cookie.Value)
	if !ok {
		return "", "", false
	}
	return username, runnerToken, true
}

func (a *API) setSessionCookie(ctx context.Context, w http.ResponseWriter, username string) {
	value := a.signSession(ctx, username)
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.auth.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((14 * 24 * time.Hour).Seconds()),
	})
}

func (a *API) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.auth.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (a *API) signSession(ctx context.Context, username string) string {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		nonce = []byte(time.Now().UTC().Format(time.RFC3339Nano))
	}
	payload := username + "|" + base64.RawURLEncoding.EncodeToString(nonce)
	mac := hmac.New(sha256.New, a.sessionSigningKey(ctx, username))
	_, _ = mac.Write([]byte(payload))
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func (a *API) verifySession(ctx context.Context, value string) (string, string, bool) {
	payloadPart, sigPart, ok := strings.Cut(value, ".")
	if !ok {
		return "", "", false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil {
		return "", "", false
	}
	sig, err := base64.RawURLEncoding.DecodeString(sigPart)
	if err != nil {
		return "", "", false
	}
	username, _, ok := strings.Cut(string(payloadBytes), "|")
	if !ok || username == "" {
		return "", "", false
	}
	mac := hmac.New(sha256.New, a.sessionSigningKey(ctx, username))
	_, _ = mac.Write(payloadBytes)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return "", "", false
	}
	user, ok := a.lookupUser(ctx, username)
	if !ok {
		return "", "", false
	}
	return username, user.RunnerToken, true
}

func (a *API) sessionSigningKey(ctx context.Context, username string) []byte {
	key := a.sessionSecret(ctx)
	if len(key) == 0 {
		key = []byte("codex-task-workbench-dev-session-key")
	}
	if user, ok := a.lookupUser(ctx, username); ok {
		key = append(key, 0)
		if user.Password != "" {
			key = append(key, []byte(user.Password)...)
		} else {
			key = append(key, []byte(user.PasswordHash)...)
		}
	}
	return key
}

func (a *API) verifyLogin(ctx context.Context, username, password string) (struct{ runnerToken string }, bool) {
	record, ok := a.lookupUser(ctx, username)
	if !ok {
		return struct{ runnerToken string }{}, false
	}
	if record.Password != "" {
		if subtle.ConstantTimeCompare([]byte(record.Password), []byte(password)) != 1 {
			return struct{ runnerToken string }{}, false
		}
		return struct{ runnerToken string }{runnerToken: record.RunnerToken}, true
	}
	if bcrypt.CompareHashAndPassword([]byte(record.PasswordHash), []byte(password)) != nil {
		return struct{ runnerToken string }{}, false
	}
	return struct{ runnerToken string }{runnerToken: record.RunnerToken}, true
}

type authLookupRecord struct {
	Password     string
	PasswordHash string
	RunnerToken  string
}

func (a *API) lookupUser(ctx context.Context, username string) (authLookupRecord, bool) {
	if password, exists := a.auth.Users[username]; exists {
		return authLookupRecord{Password: password, RunnerToken: a.auth.RunnerToken}, true
	}
	if a.store == nil {
		return authLookupRecord{}, false
	}
	user, err := a.store.GetAuthUser(ctx, username)
	if err != nil {
		return authLookupRecord{}, false
	}
	return authLookupRecord{PasswordHash: user.PasswordHash, RunnerToken: a.runnerToken(ctx)}, true
}

func (a *API) sessionSecret(ctx context.Context) []byte {
	if len(a.auth.SessionKey) > 0 {
		return a.auth.SessionKey
	}
	if a.store == nil {
		return nil
	}
	value, err := a.store.GetAuthSetting(ctx, "session_secret")
	if err != nil {
		return nil
	}
	return []byte(value)
}

func (a *API) runnerToken(ctx context.Context) string {
	if a.auth.RunnerToken != "" {
		return a.auth.RunnerToken
	}
	if a.store == nil {
		return ""
	}
	value, err := a.store.GetAuthSetting(ctx, "runner_token")
	if err != nil {
		return ""
	}
	return value
}

func randomSecretHex(bytes int) string {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buf)
}

func contextOrBackground(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}
