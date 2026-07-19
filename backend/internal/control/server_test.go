package control

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
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
		`RUN_AS='user'`,
		`stat -c '%U' "$CODEX_PATH"`,
		`RUNNER_TOKEN=$RUNNER_TOKEN`,
		`CODEX_BYPASS_APPROVALS_AND_SANDBOX=$CODEX_BYPASS_APPROVALS_AND_SANDBOX`,
		`RUNNER_RUN_AS=$RUN_AS`,
		`PATH=$RUNNER_PATH`,
		`HOME=$RUNNER_HOME`,
		`USER=$RUNNER_USER`,
		`EnvironmentFile=$ENV_FILE`,
		`User=$RUNNER_USER`,
		`WorkingDirectory=$RUNNER_HOME`,
		`Restart=on-failure`,
		`$SUDO rm -f "$INSTALL_DIR/runner.disabled"`,
		`$SUDO chown -R "$RUNNER_USER" "$INSTALL_DIR"`,
		`user_writable_linux_update()`,
		`[ -n "$RUNNER_ID" ] || return 1`,
		`Updating existing user-writable runner binary`,
		`kill -KILL "$PID"`,
		`runuser -u "$RUNNER_USER" -- sh -c "$START_CMD"`,
		`linux_systemd_available()`,
		`[ -d /run/systemd/system ] || return 1`,
		`[ "$(ps -p 1 -o comm= 2>/dev/null | tr -d '[:space:]')" = "systemd" ] || return 1`,
		`systemctl show --property=Version --value >/dev/null 2>&1 || return 1`,
		`start_standalone_runner`,
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

func TestDecodePatchServerInputCanClearAlias(t *testing.T) {
	rec := httptest.NewRecorder()
	in, ok := decodePatchServerInput(rec, map[string]json.RawMessage{
		"alias": json.RawMessage("null"),
	})
	if !ok {
		t.Fatalf("decode patch server input failed with status %d: %s", rec.Code, rec.Body.String())
	}
	if in.Alias == nil {
		t.Fatalf("alias pointer is nil, want empty string pointer for clear")
	}
	if *in.Alias != "" {
		t.Fatalf("alias = %q, want empty string", *in.Alias)
	}
}

func TestDeleteServerRequestsRunnerShutdown(t *testing.T) {
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
	api := NewAPI(store, nil, AuthConfig{})
	httpServer := httptest.NewServer(api.Handler())
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/v1/runner/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial runner websocket: %v", err)
	}
	defer conn.Close()
	if err := conn.WriteJSON(RunnerEnvelope{
		Type:      "runner.register",
		MessageID: "msg_register",
		SentAt:    time.Now().UTC(),
		Payload: mustJSON(t, RunnerRegisterPayload{
			RunnerID: "runner_desktop",
			Hostname: "desktop",
			Version:  "test",
			Capabilities: map[string]any{
				"shutdown": true,
			},
		}),
	}); err != nil {
		t.Fatalf("write register: %v", err)
	}
	waitForRunnerConnected(t, api, "runner_desktop")

	shutdownSeen := make(chan RunnerEnvelope, 1)
	go func() {
		var env RunnerEnvelope
		if err := conn.ReadJSON(&env); err == nil {
			shutdownSeen <- env
		}
	}()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/servers/"+server.ID, nil)
	deleteDone := make(chan struct{})
	go func() {
		defer close(deleteDone)
		api.handleServerByID(rec, req)
	}()

	var shutdownEnv RunnerEnvelope
	select {
	case env := <-shutdownSeen:
		shutdownEnv = env
		if env.Type != "runner.shutdown" {
			t.Fatalf("runner message = %q, want runner.shutdown", env.Type)
		}
		var payload RunnerShutdownRequestPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			t.Fatalf("decode shutdown payload: %v", err)
		}
		if payload.Reason != "server_deleted" {
			t.Fatalf("shutdown reason = %q, want server_deleted", payload.Reason)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runner shutdown request was not sent")
	}

	if err := conn.WriteJSON(RunnerEnvelope{
		Type:      "runner.shutdown.response",
		MessageID: shutdownEnv.MessageID,
		SentAt:    time.Now().UTC(),
		Payload: mustJSON(t, RunnerShutdownResponsePayload{
			Accepted: true,
			Message:  "ok",
		}),
	}); err != nil {
		t.Fatalf("write shutdown response: %v", err)
	}
	select {
	case <-deleteDone:
	case <-time.After(2 * time.Second):
		t.Fatal("delete did not finish after shutdown response")
	}

	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if _, err := store.GetServer(ctx, server.ID); err == nil {
		t.Fatalf("server still exists after delete")
	}
}

func waitForRunnerConnected(t *testing.T, api *API, runnerID string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if api.runners.Connected(runnerID) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("runner %s did not connect", runnerID)
}

func TestTusPatchRejectsNonFinalEmptyChunk(t *testing.T) {
	token, err := encodeProjectFileTusToken(projectFileTusToken{
		ProjectID:  "project_1",
		Path:       "upload.bin",
		UploadID:   "upload_1",
		TotalSize:  16,
		CreateDirs: true,
	})
	if err != nil {
		t.Fatalf("encode tus token: %v", err)
	}

	api := NewAPI(nil, nil, AuthConfig{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/projects/project_1/files/upload/tus/"+token, http.NoBody)
	req.Header.Set("Content-Type", "application/offset+octet-stream")
	req.Header.Set("Upload-Offset", "0")
	api.handleProjectFileTusResource(rec, req, "project_1", token)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Upload chunk must not be empty") {
		t.Fatalf("body missing empty chunk error: %s", rec.Body.String())
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return raw
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

func TestAcknowledgeAllNotificationsRoute(t *testing.T) {
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
	task, err := store.CreateTask(ctx, project.ID, CreateTaskInput{Title: "Ack all route"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	created, err := store.CreateRun(ctx, CreateRunInput{TaskID: task.ID, Message: "run", Mode: "new"})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if _, err := store.MarkRunStarted(ctx, created.Run.ID, server.RunnerID, time.Now().UTC()); err != nil {
		t.Fatalf("mark run started: %v", err)
	}
	complete, err := store.CompleteRun(ctx, CompleteRunInput{
		RunID:   created.Run.ID,
		Status:  RunStatusSucceeded,
		EndedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("complete run: %v", err)
	}
	if _, err := store.CreateRunFinishedNotification(ctx, complete.Run); err != nil {
		t.Fatalf("create notification: %v", err)
	}

	api := NewAPI(store, nil, AuthConfig{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/ack-all", http.NoBody)
	api.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Acknowledged int64 `json:"acknowledged"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Acknowledged != 1 {
		t.Fatalf("acknowledged = %d, want 1", response.Acknowledged)
	}
	items, err := store.ListWorkbenchNotifications(ctx, true)
	if err != nil {
		t.Fatalf("list pending notifications: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("pending notifications = %#v, want none", items)
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
