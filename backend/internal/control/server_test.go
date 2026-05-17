package control

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRunnerInstallShellResolvesCodexPathForSystemd(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://control.local/api/v1/runner/install.sh", nil)
	rec := httptest.NewRecorder()

	api := &API{}
	api.handleRunnerInstallShell(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`RESOLVED_CODEX="$(command -v "$CODEX_PATH" 2>/dev/null || true)"`,
		`CODEX_PATH="$RESOLVED_CODEX"`,
		`RUNNER_TOKEN=$RUNNER_TOKEN`,
		`CODEX_BYPASS_APPROVALS_AND_SANDBOX=$CODEX_BYPASS_APPROVALS_AND_SANDBOX`,
		`PATH=$RUNNER_PATH`,
		`EnvironmentFile=$ENV_FILE`,
		`linux_systemd_available()`,
		`[ -d /run/systemd/system ] || return 1`,
		`[ "$(ps -p 1 -o comm= 2>/dev/null | tr -d '[:space:]')" = "systemd" ] || return 1`,
		`systemctl show --property=Version --value >/dev/null 2>&1 || return 1`,
		`start_standalone_runner`,
		`RUNNER_RUN_AS=standalone`,
		`nohup '$WRAPPER' >> '$RUNNER_LOG' 2>> '$RUNNER_ERR_LOG' < /dev/null & echo \$! > '$PID_FILE'`,
		`runner-darwin-amd64`,
		`runner-darwin-arm64`,
		`/Library/LaunchDaemons/com.codex-task-workbench.runner.plist`,
		`launchctl bootstrap system "$PLIST"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("install script missing %q:\n%s", want, body)
		}
	}
}

func TestBuildInfoIsPublicWhenAuthConfigured(t *testing.T) {
	old := BuildCommit
	BuildCommit = "abc123"
	defer func() { BuildCommit = old }()

	api := NewAPI(nil, nil, AuthConfig{
		Users:        map[string]string{"admin": "secret"},
		SessionKey:   []byte("test-session-secret"),
		RunnerToken:  "runner-secret",
		CookieSecure: false,
	})
	rec := httptest.NewRecorder()
	api.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/build", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"commit":"abc123"`) {
		t.Fatalf("build response missing commit: %s", rec.Body.String())
	}
}

func TestMarkDoneDoesNotCreatePendingNotice(t *testing.T) {
	dsn := testDatabaseURL(t)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	defer pool.Close()
	resetIntegrationDB(t, pool)

	store := NewStore(pool)
	server, err := store.CreateServer(ctx, CreateServerInput{Name: "desktop", RunnerID: "runner_desktop"})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	project, err := store.CreateProject(ctx, CreateProjectInput{ServerID: server.ID, Name: "workbench", Workdir: "D:\\go\\workplace"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	task, err := store.CreateTask(ctx, project.ID, CreateTaskInput{Title: "Done without pending notice"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	api := NewAPI(store, nil, AuthConfig{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+task.ID+"/mark-done", strings.NewReader(`{"summary":"done"}`))
	api.handleTaskRoutes(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var updated Task
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if updated.Status != TaskStatusDone {
		t.Fatalf("task status = %q, want done", updated.Status)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM workbench_notifications`).Scan(&count); err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if count != 0 {
		t.Fatalf("workbench notifications = %d, want 0", count)
	}
}

func TestMarkDoneStoresStructuredMemory(t *testing.T) {
	dsn := testDatabaseURL(t)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	defer pool.Close()
	resetIntegrationDB(t, pool)

	store := NewStore(pool)
	server, err := store.CreateServer(ctx, CreateServerInput{Name: "desktop", RunnerID: "runner_desktop"})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	project, err := store.CreateProject(ctx, CreateProjectInput{ServerID: server.ID, Name: "workbench", Workdir: "D:\\go\\workplace"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	task, err := store.CreateTask(ctx, project.ID, CreateTaskInput{Title: "Structured memory"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	api := NewAPI(store, nil, AuthConfig{})
	rec := httptest.NewRecorder()
	body := `{"memory":{"problem":"ship memory","changes":"added fields","verification":"go test","files":"store_tasks.go","stale_conditions":"schema changes"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+task.ID+"/mark-done", strings.NewReader(body))
	api.handleTaskRoutes(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var problem, changes, verification, files, staleConditions string
	if err := pool.QueryRow(ctx, `
		SELECT problem, changes, verification, files, stale_conditions
		FROM task_memories
		WHERE task_id=$1`, task.ID).Scan(&problem, &changes, &verification, &files, &staleConditions); err != nil {
		t.Fatalf("read task memory: %v", err)
	}
	if problem != "ship memory" || changes != "added fields" || verification != "go test" || files != "store_tasks.go" || staleConditions != "schema changes" {
		t.Fatalf("stored memory = %q/%q/%q/%q/%q", problem, changes, verification, files, staleConditions)
	}
}
