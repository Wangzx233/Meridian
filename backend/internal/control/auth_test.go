package control

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAuthMiddlewareRequiresLoginWhenConfigured(t *testing.T) {
	api := NewAPI(nil, nil, AuthConfig{
		Users:        map[string]string{"admin": "secret"},
		SessionKey:   []byte("test-session-secret"),
		RunnerToken:  "runner-secret",
		CookieSecure: false,
	})
	handler := api.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"unauthorized"`) {
		t.Fatalf("body missing unauthorized code: %s", rec.Body.String())
	}
}

func TestAuthLoginSetsSessionCookie(t *testing.T) {
	api := NewAPI(nil, nil, AuthConfig{
		Users:        map[string]string{"admin": "secret"},
		SessionKey:   []byte("test-session-secret"),
		RunnerToken:  "runner-secret",
		CookieSecure: false,
	})

	login := httptest.NewRecorder()
	api.handleAuthLogin(login, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"admin","password":"secret"}`)))
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != authCookieName {
		t.Fatalf("login cookies = %#v, want %s", cookies, authCookieName)
	}

	handler := api.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	req.AddCookie(cookies[0])
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("authenticated status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAuthMiddlewareAllowsRunnerToken(t *testing.T) {
	api := NewAPI(nil, nil, AuthConfig{
		Users:        map[string]string{"admin": "secret"},
		SessionKey:   []byte("test-session-secret"),
		RunnerToken:  "runner-secret",
		CookieSecure: false,
	})
	handler := api.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/runner/ws", nil)
	req.Header.Set("Authorization", "Bearer runner-secret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestAuthSessionInvalidatesWhenPasswordChanges(t *testing.T) {
	api := NewAPI(nil, nil, AuthConfig{
		Users:        map[string]string{"admin": "secret"},
		SessionKey:   []byte("test-session-secret"),
		RunnerToken:  "runner-secret",
		CookieSecure: false,
	})
	login := httptest.NewRecorder()
	api.handleAuthLogin(login, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"admin","password":"secret"}`)))
	cookies := login.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("login did not set a cookie")
	}

	api.auth.Users["admin"] = "changed"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	req.AddCookie(cookies[0])
	rec := httptest.NewRecorder()
	api.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status after password change = %d, want 401", rec.Code)
	}
}

func TestAuthSetupIntegration(t *testing.T) {
	dsn := authSetupTestDatabaseURL(t)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	defer pool.Close()
	resetAuthSetupIntegrationDB(t, pool)

	api := NewAPI(NewStore(pool), nil, AuthConfig{CookieSecure: false})

	session := httptest.NewRecorder()
	api.Handler().ServeHTTP(session, httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil))
	if session.Code != http.StatusOK {
		t.Fatalf("session status = %d, body = %s", session.Code, session.Body.String())
	}
	if !strings.Contains(session.Body.String(), `"setup_required":true`) {
		t.Fatalf("session did not require setup: %s", session.Body.String())
	}

	beforeSetup := httptest.NewRecorder()
	api.Handler().ServeHTTP(beforeSetup, httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil))
	if beforeSetup.Code != http.StatusUnauthorized {
		t.Fatalf("before setup status = %d, want 401", beforeSetup.Code)
	}

	setup := httptest.NewRecorder()
	api.Handler().ServeHTTP(setup, httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", strings.NewReader(`{"username":"admin","password":"supersecret"}`)))
	if setup.Code != http.StatusOK {
		t.Fatalf("setup status = %d, body = %s", setup.Code, setup.Body.String())
	}
	if !strings.Contains(setup.Body.String(), `"authenticated":true`) || !strings.Contains(setup.Body.String(), `"runner_token"`) {
		t.Fatalf("setup response missing session fields: %s", setup.Body.String())
	}
	cookies := setup.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != authCookieName {
		t.Fatalf("setup cookies = %#v, want %s", cookies, authCookieName)
	}

	unauthenticated := httptest.NewRecorder()
	api.Handler().ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", unauthenticated.Code)
	}

	authenticatedReq := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	authenticatedReq.AddCookie(cookies[0])
	authenticated := httptest.NewRecorder()
	api.Handler().ServeHTTP(authenticated, authenticatedReq)
	if authenticated.Code != http.StatusOK {
		t.Fatalf("authenticated status = %d, body = %s", authenticated.Code, authenticated.Body.String())
	}

	second := httptest.NewRecorder()
	api.Handler().ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", strings.NewReader(`{"username":"ops","password":"supersecret"}`)))
	if second.Code != http.StatusConflict {
		t.Fatalf("second setup status = %d, want 409", second.Code)
	}
}

func authSetupTestDatabaseURL(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("CTW_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("CTW_TEST_DATABASE_URL is not set")
	}
	return dsn
}

func resetAuthSetupIntegrationDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `TRUNCATE auth_users, auth_settings RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("reset auth setup integration database: %v", err)
	}
}

func TestParseAuthUsersRejectsInvalidEntries(t *testing.T) {
	if _, err := parseAuthUsers("admin"); err == nil {
		t.Fatalf("expected invalid auth user entry to fail")
	}
}
