package control

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStoreRunStateTransitionsIntegration(t *testing.T) {
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
	task, err := store.CreateTask(ctx, project.ID, CreateTaskInput{Title: "Implement feature", Description: "Do the work"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	result, err := store.CreateRun(ctx, CreateRunInput{TaskID: task.ID, Message: "first turn", Mode: "auto", CodexModel: "gpt-5.5", ReasoningEffort: "high"})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if result.Run.Status != RunStatusQueued {
		t.Fatalf("run status = %q, want queued", result.Run.Status)
	}
	if result.Run.Mode != RunModeNew {
		t.Fatalf("run mode = %q, want new", result.Run.Mode)
	}
	if result.Run.CodexModel == nil || *result.Run.CodexModel != "gpt-5.5" {
		t.Fatalf("run model = %v, want gpt-5.5", result.Run.CodexModel)
	}
	if result.Run.ReasoningEffort == nil || *result.Run.ReasoningEffort != "high" {
		t.Fatalf("run reasoning = %v, want high", result.Run.ReasoningEffort)
	}
	if result.Task.Status != TaskStatusRunning {
		t.Fatalf("task status after run create = %q, want running", result.Task.Status)
	}
	if result.Task.ActiveRunID == nil || *result.Task.ActiveRunID != result.Run.ID {
		t.Fatalf("active_run_id = %v, want %s", result.Task.ActiveRunID, result.Run.ID)
	}
	if result.Assign == nil || result.Assign.RunID != result.Run.ID || len(result.Assign.Argv) == 0 {
		t.Fatalf("assignment not built: %#v", result.Assign)
	}
	argv := strings.Join(result.Assign.Argv, "\n")
	if !strings.Contains(argv, "--model\ngpt-5.5") || !strings.Contains(argv, "model_reasoning_effort=\"high\"") {
		t.Fatalf("assignment argv missing codex options: %#v", result.Assign.Argv)
	}

	_, err = store.CreateRun(ctx, CreateRunInput{TaskID: task.ID, Message: "duplicate", Mode: "new"})
	if !errors.Is(err, ErrActiveRunExists) {
		t.Fatalf("duplicate active run error = %v, want %v", err, ErrActiveRunExists)
	}

	if _, err = store.MarkRunStarted(ctx, result.Run.ID, "runner_desktop", time.Now().UTC()); err != nil {
		t.Fatalf("mark run started: %v", err)
	}
	eventPayload := []byte(`{"raw":{"type":"message","text":"hello"},"text":"hello","session_id":"codex_session_123"}`)
	event, err := store.InsertRunnerEvent(ctx, RunnerEventInput{
		RunID:      result.Run.ID,
		EventType:  EventCodexEvent,
		Stream:     StreamJSONL,
		Payload:    eventPayload,
		OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("insert runner event: %v", err)
	}
	if event.TaskID != task.ID {
		t.Fatalf("persisted event task_id = %q, want %q", event.TaskID, task.ID)
	}
	task, err = store.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("get task after session event: %v", err)
	}
	if task.CodexSessionID == nil || *task.CodexSessionID != "codex_session_123" {
		t.Fatalf("task session after event = %v, want codex_session_123", task.CodexSessionID)
	}
	exitCode := 0
	finalMessage := "done"
	sessionID := "codex_session_123"
	complete, err := store.CompleteRun(ctx, CompleteRunInput{
		RunID:          result.Run.ID,
		Status:         RunStatusSucceeded,
		ExitCode:       &exitCode,
		FinalMessage:   &finalMessage,
		CodexSessionID: &sessionID,
		EndedAt:        time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("complete run: %v", err)
	}
	if complete.Run.ID != result.Run.ID || complete.Run.Status != RunStatusSucceeded {
		t.Fatalf("complete run result = %#v, want terminal run", complete.Run)
	}
	task, err = store.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Status != TaskStatusWaitingUser {
		t.Fatalf("task status after terminal run = %q, want waiting_user", task.Status)
	}
	if task.CodexSessionID == nil || *task.CodexSessionID != sessionID {
		t.Fatalf("task session = %v, want %q", task.CodexSessionID, sessionID)
	}
	if task.ActiveRunID != nil {
		t.Fatalf("active_run_id after terminal run = %v, want nil", task.ActiveRunID)
	}
	events, err := store.ListEvents(ctx, result.Run.ID, 0)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) < 4 {
		t.Fatalf("persisted events = %d, want at least queued/running/codex/final", len(events))
	}
	for i, event := range events {
		if event.Seq != int64(i+1) {
			t.Fatalf("event[%d] seq = %d, want %d", i, event.Seq, i+1)
		}
		if event.TaskID != task.ID {
			t.Fatalf("event[%d] task_id = %q, want %q", i, event.TaskID, task.ID)
		}
	}

	result, err = store.CreateRun(ctx, CreateRunInput{TaskID: task.ID, Message: "second turn", Mode: "auto"})
	if err != nil {
		t.Fatalf("create resume run: %v", err)
	}
	if result.Run.Mode != RunModeResume {
		t.Fatalf("auto after session = %q, want resume", result.Run.Mode)
	}
}

func TestStoreServerAliasIntegration(t *testing.T) {
	dsn := testDatabaseURL(t)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	defer pool.Close()
	resetIntegrationDB(t, pool)

	store := NewStore(pool)
	alias := "Oracle 424"
	server, err := store.CreateServer(ctx, CreateServerInput{Name: "oracle424-host", Alias: &alias, RunnerID: "runner_oracle424"})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	if server.Alias == nil || *server.Alias != alias {
		t.Fatalf("server alias = %v, want %q", server.Alias, alias)
	}
	if serverDisplayName(server) != alias {
		t.Fatalf("display name = %q, want alias %q", serverDisplayName(server), alias)
	}

	updatedAlias := "Backup node"
	updated, err := store.PatchServer(ctx, server.ID, PatchServerInput{Alias: &updatedAlias})
	if err != nil {
		t.Fatalf("patch server alias: %v", err)
	}
	if updated.Alias == nil || *updated.Alias != updatedAlias {
		t.Fatalf("updated alias = %v, want %q", updated.Alias, updatedAlias)
	}

	if err := store.UpsertRunnerHeartbeat(ctx, server.RunnerID, "heartbeat-hostname"); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	afterHeartbeat, err := store.GetServer(ctx, server.ID)
	if err != nil {
		t.Fatalf("get after heartbeat: %v", err)
	}
	if afterHeartbeat.Alias == nil || *afterHeartbeat.Alias != updatedAlias {
		t.Fatalf("alias after heartbeat = %v, want %q", afterHeartbeat.Alias, updatedAlias)
	}
	if afterHeartbeat.Name != server.Name {
		t.Fatalf("name after heartbeat = %q, want %q", afterHeartbeat.Name, server.Name)
	}

	clear := " "
	cleared, err := store.PatchServer(ctx, server.ID, PatchServerInput{Alias: &clear})
	if err != nil {
		t.Fatalf("clear alias: %v", err)
	}
	if cleared.Alias != nil {
		t.Fatalf("cleared alias = %v, want nil", *cleared.Alias)
	}
	if serverDisplayName(cleared) != server.Name {
		t.Fatalf("display name after clear = %q, want %q", serverDisplayName(cleared), server.Name)
	}
}

func TestStoreDeleteProjectIntegration(t *testing.T) {
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
	task, err := store.CreateTask(ctx, project.ID, CreateTaskInput{Title: "Delete project"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	contextItem, err := store.CreateContextItem(ctx, project.ID, CreateContextInput{
		Scope:   "task",
		TaskID:  &task.ID,
		Type:    "note",
		Title:   "delete context",
		Content: "context",
	})
	if err != nil {
		t.Fatalf("create context item: %v", err)
	}
	createdRun, err := store.CreateRun(ctx, CreateRunInput{
		TaskID:         task.ID,
		Message:        "run",
		Mode:           "new",
		ContextItemIDs: []string{contextItem.ID},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if _, err := store.MarkRunStarted(ctx, createdRun.Run.ID, server.RunnerID, time.Now().UTC()); err != nil {
		t.Fatalf("mark run started: %v", err)
	}
	complete, err := store.CompleteRun(ctx, CompleteRunInput{
		RunID:   createdRun.Run.ID,
		Status:  RunStatusSucceeded,
		EndedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("complete run: %v", err)
	}
	if _, err := store.CreateRunFinishedNotification(ctx, complete.Run); err != nil {
		t.Fatalf("create notification: %v", err)
	}

	if err := store.DeleteProject(ctx, project.ID); err != nil {
		t.Fatalf("delete project: %v", err)
	}
	assertTableCount(t, pool, "projects", 0)
	assertTableCount(t, pool, "tasks", 0)
	assertTableCount(t, pool, "runs", 0)
	assertTableCount(t, pool, "run_events", 0)
	assertTableCount(t, pool, "context_items", 0)
	assertTableCount(t, pool, "run_context_items", 0)
	assertTableCount(t, pool, "run_input_images", 0)
	assertTableCount(t, pool, "workbench_notifications", 0)
	assertTableCount(t, pool, "servers", 1)
}

func TestCreateRunRecoversSessionFromHistoricalRunIntegration(t *testing.T) {
	dsn := os.Getenv("CTW_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("CTW_TEST_DATABASE_URL is not set")
	}
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
	task, err := store.CreateTask(ctx, project.ID, CreateTaskInput{Title: "Recover session"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	sessionID := "codex_session_recovered"
	first, err := store.CreateRun(ctx, CreateRunInput{TaskID: task.ID, Message: "old turn", Mode: "new"})
	if err != nil {
		t.Fatalf("create historical run: %v", err)
	}
	_, err = pool.Exec(ctx, `
		UPDATE runs
		SET status='failed', codex_session_id=$2, ended_at=now()
		WHERE id=$1`, first.Run.ID, sessionID)
	if err != nil {
		t.Fatalf("patch historical run: %v", err)
	}
	_, err = pool.Exec(ctx, `
		UPDATE tasks
		SET status='waiting_user', codex_session_id=NULL
		WHERE id=$1`, task.ID)
	if err != nil {
		t.Fatalf("clear task session: %v", err)
	}

	result, err := store.CreateRun(ctx, CreateRunInput{TaskID: task.ID, Message: "continue", Mode: "auto"})
	if err != nil {
		t.Fatalf("create run after recovery: %v", err)
	}
	if result.Run.Mode != RunModeResume {
		t.Fatalf("recovered run mode = %q, want resume", result.Run.Mode)
	}
	if result.Task.CodexSessionID == nil || *result.Task.CodexSessionID != sessionID {
		t.Fatalf("recovered task session = %v, want %q", result.Task.CodexSessionID, sessionID)
	}
	if result.Run.CodexSessionID == nil || *result.Run.CodexSessionID != sessionID {
		t.Fatalf("recovered run session = %v, want %q", result.Run.CodexSessionID, sessionID)
	}
}

func TestReconcileRunnerActiveRunsIntegration(t *testing.T) {
	dsn := os.Getenv("CTW_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("CTW_TEST_DATABASE_URL is not set")
	}
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
	task, err := store.CreateTask(ctx, project.ID, CreateTaskInput{Title: "Stale run"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	result, err := store.CreateRun(ctx, CreateRunInput{TaskID: task.ID, Message: "run", Mode: "new"})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if _, err = store.MarkRunStarted(ctx, result.Run.ID, "runner_desktop", time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatalf("mark run started: %v", err)
	}
	events, err := store.ReconcileRunnerActiveRuns(ctx, "runner_desktop", nil, time.Now().UTC())
	if err != nil {
		t.Fatalf("reconcile active runs: %v", err)
	}
	if len(events) != 1 || events[0].EventType != EventRunFinal {
		t.Fatalf("events = %#v, want one final event", events)
	}
	run, err := store.GetRun(ctx, result.Run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if run.Status != RunStatusFailed {
		t.Fatalf("run status = %q, want failed", run.Status)
	}
	task, err = store.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Status != TaskStatusWaitingUser || task.ActiveRunID != nil {
		t.Fatalf("task after reconcile status=%q active=%v, want waiting_user nil", task.Status, task.ActiveRunID)
	}
}

func TestStoreCreateRunIdempotencyIntegration(t *testing.T) {
	dsn := os.Getenv("CTW_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("CTW_TEST_DATABASE_URL is not set")
	}
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
	task, err := store.CreateTask(ctx, project.ID, CreateTaskInput{Title: "Idempotent run"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	first, err := store.CreateRun(ctx, CreateRunInput{TaskID: task.ID, Message: "first", Mode: "new", IdempotencyKey: "idem-1"})
	if err != nil {
		t.Fatalf("first create run: %v", err)
	}
	second, err := store.CreateRun(ctx, CreateRunInput{TaskID: task.ID, Message: "first retry", Mode: "new", IdempotencyKey: "idem-1"})
	if err != nil {
		t.Fatalf("second create run with same key: %v", err)
	}
	if second.Run.ID != first.Run.ID {
		t.Fatalf("idempotent retry run id = %q, want %q", second.Run.ID, first.Run.ID)
	}
}

func TestCreateRunStoresInputImagesIntegration(t *testing.T) {
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
	task, err := store.CreateTask(ctx, project.ID, CreateTaskInput{Title: "Image input"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	created, err := store.CreateRun(ctx, CreateRunInput{
		TaskID:  task.ID,
		Message: "Inspect this screenshot",
		Mode:    "new",
		InputImages: []RunInputImageInput{{
			Filename:      "screen.png",
			MimeType:      "image/png",
			ContentBase64: "iVBORw0KGgo=",
		}},
	})
	if err != nil {
		t.Fatalf("create run with image: %v", err)
	}
	if len(created.Run.InputImages) != 1 || created.Run.InputImages[0].Filename != "screen.png" || created.Run.InputImages[0].SizeBytes != 8 {
		t.Fatalf("run images = %#v, want screen.png metadata", created.Run.InputImages)
	}
	if created.Assign == nil || len(created.Assign.InputImages) != 1 || created.Assign.InputImages[0].ContentBase64 != "iVBORw0KGgo=" {
		t.Fatalf("assignment images = %#v, want encoded content", created.Assign)
	}
	if got := strings.Join(created.Assign.Argv, "\n"); !strings.Contains(got, "--image\nscreen.png") {
		t.Fatalf("assignment argv missing image flag: %#v", created.Assign.Argv)
	}

	runs, err := store.ListRuns(ctx, task.ID)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 || len(runs[0].InputImages) != 1 || runs[0].InputImages[0].ID == "" {
		t.Fatalf("listed run images = %#v, want stored metadata", runs)
	}
}

func TestTaskDoneNotificationsAreHiddenFromPendingIntegration(t *testing.T) {
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
	task, err := store.CreateTask(ctx, project.ID, CreateTaskInput{Title: "Ship notification"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	task, err = store.MarkTaskDone(ctx, task.ID, MarkTaskDoneInput{})
	if err != nil {
		t.Fatalf("mark task done: %v", err)
	}
	notification, err := store.CreateTaskDoneNotification(ctx, task)
	if err != nil {
		t.Fatalf("create notification: %v", err)
	}
	if notification.Type != "task_done" || notification.TaskID != task.ID || notification.ProjectName != project.Name || notification.ServerName != serverDisplayName(server) {
		t.Fatalf("notification = %#v, want task/project/server details", notification)
	}
	items, err := store.ListWorkbenchNotifications(ctx, true)
	if err != nil {
		t.Fatalf("list pending notifications: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("pending notifications = %#v, want task_done hidden", items)
	}
	items, err = store.ListWorkbenchNotifications(ctx, false)
	if err != nil {
		t.Fatalf("list all notifications: %v", err)
	}
	if len(items) != 1 || items[0].ID != notification.ID {
		t.Fatalf("all notifications = %#v, want created task_done notification", items)
	}
}

func TestRunFinishedNotificationIntegration(t *testing.T) {
	dsn := os.Getenv("CTW_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("CTW_TEST_DATABASE_URL is not set")
	}
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
	task, err := store.CreateTask(ctx, project.ID, CreateTaskInput{Title: "Ship run notice"})
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
		Status:  RunStatusFailed,
		EndedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("complete run: %v", err)
	}
	notification, err := store.CreateRunFinishedNotification(ctx, complete.Run)
	if err != nil {
		t.Fatalf("create run finished notification: %v", err)
	}
	if notification.Type != NotificationTypeRunFinished || notification.RunID == nil || *notification.RunID != created.Run.ID || notification.RunStatus == nil || *notification.RunStatus != RunStatusFailed {
		t.Fatalf("notification = %#v, want run finished details", notification)
	}
	items, err := store.ListWorkbenchNotifications(ctx, true)
	if err != nil {
		t.Fatalf("list pending notifications: %v", err)
	}
	if len(items) != 1 || items[0].ID != notification.ID {
		t.Fatalf("pending notifications = %#v, want run notification", items)
	}
}

func TestCodexReminderNotificationIntegration(t *testing.T) {
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
	task, err := store.CreateTask(ctx, project.ID, CreateTaskInput{Title: "Long checks"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	created, err := store.CreateRun(ctx, CreateRunInput{TaskID: task.ID, Message: "run slow checks", Mode: "new", ReminderCallbackEnabled: true})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	notification, err := store.CreateCodexReminderNotification(ctx, created.Run.ID, "Checks need review", "Open the output and inspect failures.")
	if err != nil {
		t.Fatalf("create codex reminder notification: %v", err)
	}
	if notification.Type != NotificationTypeCodexReminder || notification.RunID == nil || *notification.RunID != created.Run.ID || notification.Message != "Open the output and inspect failures." {
		t.Fatalf("notification = %#v, want codex reminder details", notification)
	}
	items, err := store.ListWorkbenchNotifications(ctx, true)
	if err != nil {
		t.Fatalf("list pending notifications: %v", err)
	}
	if len(items) != 1 || items[0].ID != notification.ID {
		t.Fatalf("pending notifications = %#v, want reminder notification", items)
	}
}

func TestAcknowledgePendingWorkbenchNotificationsIntegration(t *testing.T) {
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
	for _, title := range []string{"First run notice", "Second run notice"} {
		task, err := store.CreateTask(ctx, project.ID, CreateTaskInput{Title: title})
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
			t.Fatalf("create run finished notification: %v", err)
		}
	}

	taskDone, err := store.CreateTask(ctx, project.ID, CreateTaskInput{Title: "Historical done notice"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	taskDone, err = store.MarkTaskDone(ctx, taskDone.ID, MarkTaskDoneInput{})
	if err != nil {
		t.Fatalf("mark task done: %v", err)
	}
	if _, err := store.CreateTaskDoneNotification(ctx, taskDone); err != nil {
		t.Fatalf("create task done notification: %v", err)
	}

	count, err := store.AcknowledgePendingWorkbenchNotifications(ctx)
	if err != nil {
		t.Fatalf("acknowledge pending notifications: %v", err)
	}
	if count != 2 {
		t.Fatalf("acknowledged = %d, want 2", count)
	}
	pending, err := store.ListWorkbenchNotifications(ctx, true)
	if err != nil {
		t.Fatalf("list pending notifications: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending notifications = %#v, want none", pending)
	}

	var acknowledgedRunNotices, untouchedTaskDone int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM workbench_notifications WHERE type <> 'task_done' AND acknowledged_at IS NOT NULL`).Scan(&acknowledgedRunNotices); err != nil {
		t.Fatalf("count acknowledged run notices: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM workbench_notifications WHERE type='task_done' AND acknowledged_at IS NULL`).Scan(&untouchedTaskDone); err != nil {
		t.Fatalf("count untouched task_done notices: %v", err)
	}
	if acknowledgedRunNotices != 2 || untouchedTaskDone != 1 {
		t.Fatalf("acknowledged run notices = %d, untouched task_done = %d; want 2 and 1", acknowledgedRunNotices, untouchedTaskDone)
	}
}

func TestDatabaseEnforcesOneActiveRunPerTaskIntegration(t *testing.T) {
	dsn := os.Getenv("CTW_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("CTW_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	defer pool.Close()
	resetIntegrationDB(t, pool)

	var serverID string
	if err := pool.QueryRow(ctx, `INSERT INTO servers (name, runner_id) VALUES ('desktop', 'runner_desktop') RETURNING id`).Scan(&serverID); err != nil {
		t.Fatalf("insert server: %v", err)
	}
	var projectID string
	if err := pool.QueryRow(ctx, `INSERT INTO projects (server_id, name, workdir) VALUES ($1, 'workbench', 'D:\go\workplace') RETURNING id`, serverID).Scan(&projectID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	var taskID string
	if err := pool.QueryRow(ctx, `INSERT INTO tasks (project_id, title) VALUES ($1, 'task') RETURNING id`, projectID).Scan(&taskID); err != nil {
		t.Fatalf("insert task: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO runs (task_id, mode, status, user_message, generated_prompt) VALUES ($1, 'new', 'queued', 'one', 'prompt')`, taskID); err != nil {
		t.Fatalf("insert first active run: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO runs (task_id, mode, status, user_message, generated_prompt) VALUES ($1, 'new', 'running', 'two', 'prompt')`, taskID)
	if err == nil {
		t.Fatalf("expected database unique index to reject second active run")
	}
	if !isUniqueActiveRunViolation(err) {
		t.Fatalf("second active run error = %v, want runs_one_active_per_task_idx violation", err)
	}
}

func resetIntegrationDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		TRUNCATE run_events, run_input_images, run_context_items, context_items, task_memories, email_notification_configs, workbench_notifications, runs, tasks, projects, servers, auth_users, auth_settings
		RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("reset integration database: %v", err)
	}
}

func testDatabaseURL(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("CTW_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("CTW_TEST_DATABASE_URL is not set")
	}
	return dsn
}

func assertTableCount(t *testing.T, pool *pgxpool.Pool, table string, want int) {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM `+table).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if count != want {
		t.Fatalf("%s count = %d, want %d", table, count, want)
	}
}
