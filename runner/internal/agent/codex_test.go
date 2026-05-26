package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var testUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func TestScanJSONLPreservesRawAndExtractsFields(t *testing.T) {
	input := strings.NewReader(`{"type":"message","text":"hello","session_id":"sess_123","unknown":{"kept":true}}` + "\n")
	var events []CodexEvent
	scanJSONL(input, func(ev CodexEvent) {
		events = append(events, ev)
	})
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if events[0].EventType != "codex.event" || events[0].Stream != "jsonl" {
		t.Fatalf("event type/stream = %s/%s", events[0].EventType, events[0].Stream)
	}

	var payload struct {
		Raw       map[string]any `json:"raw"`
		Text      string         `json:"text"`
		SessionID string         `json:"session_id"`
	}
	if err := json.Unmarshal(events[0].Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Text != "hello" || payload.SessionID != "sess_123" {
		t.Fatalf("text/session = %q/%q", payload.Text, payload.SessionID)
	}
	if _, ok := payload.Raw["unknown"].(map[string]any); !ok {
		t.Fatalf("raw unknown fields not preserved: %#v", payload.Raw)
	}
}

func TestScanJSONLFallsBackForNonJSONOutput(t *testing.T) {
	input := strings.NewReader("plain output\n")
	var events []CodexEvent
	scanJSONL(input, func(ev CodexEvent) {
		events = append(events, ev)
	})
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if events[0].EventType != "process.output" || events[0].Stream != "stdout" {
		t.Fatalf("event type/stream = %s/%s", events[0].EventType, events[0].Stream)
	}
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(events[0].Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Text != "plain output" {
		t.Fatalf("text = %q", payload.Text)
	}
}

func TestExtractCodexFieldsFromNormalizedPayload(t *testing.T) {
	normalized := normalizeCodexPayload(json.RawMessage(`{"message":{"content":"nested"},"session":{"id":"sess_nested"}}`))
	fields := extractCodexFieldsFromNormalized(normalized)
	if fields.Text != "nested" || fields.SessionID != "sess_nested" {
		t.Fatalf("fields = %#v", fields)
	}
}

func TestExtractCodexFieldsFromCurrentExecEvents(t *testing.T) {
	started := normalizeCodexPayload(json.RawMessage(`{"type":"thread.started","thread_id":"019e1bf8-6869-7ea3-b068-e360f929ebd2"}`))
	startedFields := extractCodexFieldsFromNormalized(started)
	if startedFields.SessionID != "019e1bf8-6869-7ea3-b068-e360f929ebd2" {
		t.Fatalf("thread session = %#v", startedFields)
	}

	completed := normalizeCodexPayload(json.RawMessage(`{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"done"}}`))
	completedFields := extractCodexFieldsFromNormalized(completed)
	if completedFields.Text != "done" {
		t.Fatalf("item text = %#v", completedFields)
	}
}

func TestCodexRunnerUsesArgvExecution(t *testing.T) {
	argv := []string{os.Args[0], "-test.run=TestHelperCodexProcess", "--"}
	var pid int
	runner := CodexRunner{
		Env:       []string{"CTW_HELPER_CODEX=1"},
		OnStarted: func(startedPID int) { pid = startedPID },
	}
	result := runner.Run(context.Background(), Assignment{
		RunID:   "run_test",
		Workdir: mustGetwd(t),
		Prompt:  "prompt",
		Argv:    argv,
	}, func(CodexEvent) {})
	if result.Status != "succeeded" {
		t.Fatalf("status = %q, error = %v", result.Status, result.ErrorMessage)
	}
	if pid <= 0 {
		t.Fatalf("OnStarted pid = %d, want child process pid", pid)
	}
	if result.FinalMessage == nil || *result.FinalMessage != "ok" {
		t.Fatalf("final message = %v", result.FinalMessage)
	}
	if result.CodexSessionID == nil || *result.CodexSessionID != "sess" {
		t.Fatalf("session id = %v", result.CodexSessionID)
	}
}

func TestCodexRunnerDoesNotUsePlainStdoutAsFinalMessage(t *testing.T) {
	argv := []string{os.Args[0], "-test.run=TestHelperCodexProcess", "--"}
	runner := CodexRunner{Env: []string{"CTW_HELPER_CODEX=plain_after_json"}}
	result := runner.Run(context.Background(), Assignment{
		RunID:   "run_test",
		Workdir: mustGetwd(t),
		Prompt:  "prompt",
		Argv:    argv,
	}, func(CodexEvent) {})
	if result.Status != "succeeded" {
		t.Fatalf("status = %q, error = %v", result.Status, result.ErrorMessage)
	}
	if result.FinalMessage == nil || *result.FinalMessage != "ok" {
		t.Fatalf("final message = %v, want ok", result.FinalMessage)
	}
}

func TestCodexRunnerCancelsHangingProcess(t *testing.T) {
	argv := []string{os.Args[0], "-test.run=TestHelperCodexProcess", "--"}
	runner := CodexRunner{Env: []string{"CTW_HELPER_CODEX=hang"}}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	result := runner.Run(ctx, Assignment{
		RunID:   "run_test",
		Workdir: mustGetwd(t),
		Prompt:  "/compact",
		Argv:    argv,
	}, func(CodexEvent) {})
	if result.Status != "canceled" {
		t.Fatalf("status = %q, want canceled", result.Status)
	}
	if result.ErrorMessage == nil || !strings.Contains(*result.ErrorMessage, context.DeadlineExceeded.Error()) {
		t.Fatalf("error message = %v, want deadline exceeded", result.ErrorMessage)
	}
}

func TestCodexRunnerReportsMissingCodexExecutable(t *testing.T) {
	var events []CodexEvent
	runner := CodexRunner{Env: []string{"PATH="}}
	result := runner.Run(context.Background(), Assignment{
		RunID:   "run_test",
		Workdir: mustGetwd(t),
		Prompt:  "prompt",
		Argv:    []string{"codex", "exec", "--json", "-"},
	}, func(ev CodexEvent) {
		events = append(events, ev)
	})
	if result.Status != "failed" {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if result.ErrorMessage == nil || !strings.Contains(*result.ErrorMessage, "CODEX_PATH") {
		t.Fatalf("error message = %v, want CODEX_PATH guidance", result.ErrorMessage)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1 runner error", len(events))
	}
	if events[0].EventType != "runner.error" || events[0].Stream != "system" {
		t.Fatalf("event type/stream = %s/%s", events[0].EventType, events[0].Stream)
	}
	var payload struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(events[0].Payload, &payload); err != nil {
		t.Fatalf("unmarshal runner error: %v", err)
	}
	if payload.Code != "codex_not_found" {
		t.Fatalf("runner error code = %q, want codex_not_found", payload.Code)
	}
	if !strings.Contains(payload.Message, "PATH=") {
		t.Fatalf("runner error message = %q, want PATH details", payload.Message)
	}
}

func TestResolveExecutableUsesMergedRunnerEnvPath(t *testing.T) {
	dir := t.TempDir()
	name := "codex"
	if os.PathSeparator == '\\' {
		name = "codex.exe"
	}
	exe := filepath.Join(dir, name)
	if err := os.WriteFile(exe, []byte("placeholder"), 0o755); err != nil {
		t.Fatalf("write fake executable: %v", err)
	}
	env := []string{"PATH=" + dir}
	if os.PathSeparator == '\\' {
		env = append(env, "PATHEXT=.EXE")
	}
	got, err := resolveExecutable("codex", env)
	if err != nil {
		t.Fatalf("resolve executable: %v", err)
	}
	if os.PathSeparator == '\\' {
		if !strings.EqualFold(got, exe) {
			t.Fatalf("resolved executable = %q, want %q", got, exe)
		}
		return
	}
	if got != exe {
		t.Fatalf("resolved executable = %q, want %q", got, exe)
	}
}

func TestAgentNormalizeAssignmentBuildsCodexOptions(t *testing.T) {
	model := "gpt-5.5"
	effort := "xhigh"
	agent := New(Config{CodexPath: "codex"}, nil)
	assign := Assignment{
		Workdir:         mustGetwd(t),
		Mode:            "resume",
		CodexSessionID:  stringPtr("sess"),
		CodexModel:      &model,
		ReasoningEffort: &effort,
	}
	agent.normalizeAssignment(&assign)
	got := strings.Join(assign.Argv, "\n")
	for _, want := range []string{"--model\ngpt-5.5", "--config\nmodel_reasoning_effort=\"xhigh\"", "exec\nresume\n--dangerously-bypass-approvals-and-sandbox\n--skip-git-repo-check\n--json\nsess\n-"} {
		if !strings.Contains(got, want) {
			t.Fatalf("argv %q missing %q", assign.Argv, want)
		}
	}
}

func TestCompactAssignmentTimeoutOnlyAppliesToCompactResume(t *testing.T) {
	timeout := 2 * time.Minute
	got := compactAssignmentTimeout(Assignment{Mode: "resume", Prompt: " /compact "}, timeout)
	if got != timeout {
		t.Fatalf("compact timeout = %v, want %v", got, timeout)
	}
	if got := compactAssignmentTimeout(Assignment{Mode: "new", Prompt: "/compact"}, timeout); got != 0 {
		t.Fatalf("new compact timeout = %v, want 0", got)
	}
	if got := compactAssignmentTimeout(Assignment{Mode: "resume", Prompt: "continue"}, timeout); got != 0 {
		t.Fatalf("normal resume timeout = %v, want 0", got)
	}
}

func TestAgentNormalizeAssignmentAddsSkipGitRepoCheckToExistingArgv(t *testing.T) {
	agent := New(Config{CodexPath: "codex-custom"}, nil)
	assign := Assignment{
		Workdir: mustGetwd(t),
		Argv:    []string{"codex", "--cd", mustGetwd(t), "exec", "resume", "--json", "sess", "-"},
	}
	agent.normalizeAssignment(&assign)
	got := strings.Join(assign.Argv, "\n")
	if assign.Argv[0] != "codex-custom" {
		t.Fatalf("argv[0] = %q, want configured codex path", assign.Argv[0])
	}
	if !strings.Contains(got, "exec\nresume\n--skip-git-repo-check\n--dangerously-bypass-approvals-and-sandbox\n--json\nsess\n-") &&
		!strings.Contains(got, "exec\nresume\n--dangerously-bypass-approvals-and-sandbox\n--skip-git-repo-check\n--json\nsess\n-") {
		t.Fatalf("argv %q missing required exec flags in resume position", assign.Argv)
	}
}

func TestWithSkipGitRepoCheckDoesNotDuplicateFlag(t *testing.T) {
	argv := []string{"codex", "exec", "--skip-git-repo-check", "--json", "-"}
	got := withSkipGitRepoCheck(argv)
	count := 0
	for _, arg := range got {
		if arg == "--skip-git-repo-check" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("skip git repo check count = %d, want 1 in %q", count, got)
	}
}

func TestAgentNormalizeAssignmentCanPreserveCodexSandbox(t *testing.T) {
	agent := New(Config{CodexPath: "codex", UseCodexSandbox: true}, nil)
	assign := Assignment{
		Workdir: mustGetwd(t),
		Argv: []string{
			"codex", "--cd", mustGetwd(t), "exec", "resume",
			"--dangerously-bypass-approvals-and-sandbox", "--json", "sess", "-",
		},
	}
	agent.normalizeAssignment(&assign)
	got := strings.Join(assign.Argv, "\n")
	if strings.Contains(got, "--dangerously-bypass-approvals-and-sandbox") {
		t.Fatalf("argv %q should preserve Codex sandbox", assign.Argv)
	}
	if !strings.Contains(got, "exec\nresume\n--skip-git-repo-check\n--json\nsess\n-") {
		t.Fatalf("argv %q missing normal resume flags", assign.Argv)
	}
}

func TestAgentRunEnvAddsReminderHelperForEnabledAssignment(t *testing.T) {
	agent := New(Config{Env: []string{"PATH=base-path"}}, nil)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = agent.reminders.shutdown(ctx)
	})
	env := agent.runEnv(Assignment{RunID: "run_reminder", ReminderCallbackEnabled: true})
	joined := strings.Join(env, "\n")
	for _, want := range []string{"MERIDIAN_NOTIFY_URL=http://127.0.0.1:", "MERIDIAN_NOTIFY_TOKEN=", "PATH="} {
		if !strings.Contains(joined, want) {
			t.Fatalf("run env missing %q:\n%s", want, joined)
		}
	}
	pathValue := envValue(mergedEnv(env), "PATH")
	if !strings.Contains(pathValue, "meridian-runner-tools-") || !strings.Contains(pathValue, "base-path") {
		t.Fatalf("PATH = %q, want helper dir plus existing PATH", pathValue)
	}
}

func TestReminderCallbackSendsRunReminder(t *testing.T) {
	type sentMessage struct {
		typ     string
		payload map[string]any
	}
	sent := make(chan sentMessage, 1)
	server := newReminderCallbackServer(nil, func(typ string, payload any) error {
		m, ok := payload.(map[string]any)
		if !ok {
			t.Fatalf("payload type = %T, want map", payload)
		}
		sent <- sentMessage{typ: typ, payload: m}
		return nil
	})
	reg, err := server.register("run_123")
	if err != nil {
		t.Fatalf("register reminder callback: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.shutdown(ctx)
	})
	body := strings.NewReader(`{"title":"Build finished","message":"Review the output"}`)
	req, err := http.NewRequest(http.MethodPost, reg.URL, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+reg.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post reminder: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}
	select {
	case msg := <-sent:
		if msg.typ != "run.reminder" || msg.payload["run_id"] != "run_123" || msg.payload["title"] != "Build finished" {
			t.Fatalf("sent = %#v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("callback did not send reminder")
	}
}

func TestParseNotifyHelperArgs(t *testing.T) {
	title, message, err := parseNotifyHelperArgs([]string{"--title", "Build", "--message", "Review output"})
	if err != nil {
		t.Fatalf("parse title/message: %v", err)
	}
	if title != "Build" || message != "Review output" {
		t.Fatalf("title/message = %q/%q", title, message)
	}

	title, message, err = parseNotifyHelperArgs([]string{"Long", "wait", "finished"})
	if err != nil {
		t.Fatalf("parse positional message: %v", err)
	}
	if title != "" || message != "Long wait finished" {
		t.Fatalf("positional title/message = %q/%q", title, message)
	}

	if _, _, err = parseNotifyHelperArgs([]string{"--title"}); err == nil {
		t.Fatal("missing title value should fail")
	}
}

func TestListDirectoriesReturnsProjectMarkers(t *testing.T) {
	root := t.TempDir()
	projectDir := root + string(os.PathSeparator) + "project"
	if err := os.Mkdir(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(projectDir+string(os.PathSeparator)+"go.mod", []byte("module example\n"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	if err := os.WriteFile(root+string(os.PathSeparator)+"file.txt", []byte("ignored"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	listing := listDirectories(root)
	if listing.Error != nil {
		t.Fatalf("list error = %v", *listing.Error)
	}
	if listing.Path == "" {
		t.Fatalf("listing path is empty")
	}
	if len(listing.Entries) != 1 {
		t.Fatalf("entries = %#v, want one directory", listing.Entries)
	}
	if listing.Entries[0].Name != "project" {
		t.Fatalf("entry name = %q, want project", listing.Entries[0].Name)
	}
	if len(listing.Entries[0].Markers) != 1 || listing.Entries[0].Markers[0] != "go.mod" {
		t.Fatalf("markers = %#v, want go.mod", listing.Entries[0].Markers)
	}
}

func TestListProjectFilesStaysInsideWorkdir(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("readme"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	listing := listProjectFiles(root, "")
	if listing.Error != nil {
		t.Fatalf("list project files error = %v", *listing.Error)
	}
	if listing.Path != "" {
		t.Fatalf("path = %q, want root-relative empty path", listing.Path)
	}
	if len(listing.Entries) != 2 {
		t.Fatalf("entries = %#v, want src and README.md", listing.Entries)
	}

	outside := listProjectFiles(root, "..")
	if outside.Error == nil {
		t.Fatalf("expected error for path outside workdir")
	}
}

func TestReadAndWriteProjectFile(t *testing.T) {
	root := t.TempDir()
	write := writeProjectFile(root, "src/main.go", "package main\n", true)
	if write.Error != nil {
		t.Fatalf("write error = %v", *write.Error)
	}
	if write.Path != "src/main.go" {
		t.Fatalf("write path = %q, want src/main.go", write.Path)
	}

	read := readProjectFile(root, "src/main.go", 1024)
	if read.Error != nil {
		t.Fatalf("read error = %v", *read.Error)
	}
	if read.Content != "package main\n" {
		t.Fatalf("content = %q", read.Content)
	}

	outside := writeProjectFile(root, "../outside.txt", "nope", true)
	if outside.Error == nil {
		t.Fatalf("expected write outside root to fail")
	}
}

func TestWriteProjectFileBytesPreservesBinaryContent(t *testing.T) {
	root := t.TempDir()
	content := []byte{0x00, 0xff, 0x41, 0x0a, 0x80}
	write := writeProjectFileBytes(root, "assets/blob.bin", content, true)
	if write.Error != nil {
		t.Fatalf("write binary error = %v", *write.Error)
	}
	if write.Size != int64(len(content)) {
		t.Fatalf("write size = %d, want %d", write.Size, len(content))
	}

	data, err := os.ReadFile(filepath.Join(root, "assets", "blob.bin"))
	if err != nil {
		t.Fatalf("read written binary: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Fatalf("binary content = %#v, want %#v", data, content)
	}

	outside := writeProjectFileBytes(root, "../outside.bin", content, true)
	if outside.Error == nil {
		t.Fatalf("expected binary write outside root to fail")
	}
}

func TestWriteProjectFileUploadChunkResumesAndCompletes(t *testing.T) {
	root := t.TempDir()
	first := []byte("hello ")
	second := []byte("world")
	totalSize := int64(len(first) + len(second))

	started := writeProjectFileUploadChunk(root, "assets/blob.txt", "upload-1", 0, totalSize, first, true, false)
	if started.Error != nil {
		t.Fatalf("write first chunk error = %v", *started.Error)
	}
	if started.UploadedBytes != int64(len(first)) || started.ResumeOffset != int64(len(first)) || started.Complete {
		t.Fatalf("first chunk result = %#v", started)
	}
	if _, err := os.Stat(filepath.Join(root, "assets", "blob.txt")); !os.IsNotExist(err) {
		t.Fatalf("target should not exist before final chunk, stat error = %v", err)
	}

	status := projectFileUploadStatus(root, "assets/blob.txt", "upload-1", totalSize)
	if status.Error != nil {
		t.Fatalf("upload status error = %v", *status.Error)
	}
	if status.ResumeOffset != int64(len(first)) || status.Complete {
		t.Fatalf("status = %#v", status)
	}

	completed := writeProjectFileUploadChunk(root, "assets/blob.txt", "upload-1", int64(len(first)), totalSize, second, true, true)
	if completed.Error != nil {
		t.Fatalf("write final chunk error = %v", *completed.Error)
	}
	if !completed.Complete || completed.Size != totalSize || completed.UploadedBytes != totalSize {
		t.Fatalf("completed result = %#v", completed)
	}
	data, err := os.ReadFile(filepath.Join(root, "assets", "blob.txt"))
	if err != nil {
		t.Fatalf("read completed upload: %v", err)
	}
	if !bytes.Equal(data, append(first, second...)) {
		t.Fatalf("uploaded content = %q, want %q", data, append(first, second...))
	}
}

func TestWriteProjectFileUploadChunkRejectsWrongOffset(t *testing.T) {
	root := t.TempDir()
	first := []byte("abc")
	started := writeProjectFileUploadChunk(root, "upload.bin", "upload-2", 0, 6, first, true, false)
	if started.Error != nil {
		t.Fatalf("write first chunk error = %v", *started.Error)
	}

	result := writeProjectFileUploadChunk(root, "upload.bin", "upload-2", 1, 6, []byte("de"), true, false)
	if result.Error == nil {
		t.Fatalf("expected wrong offset error")
	}
	if result.ResumeOffset != int64(len(first)) {
		t.Fatalf("resume offset = %d, want %d", result.ResumeOffset, len(first))
	}
	if _, err := os.Stat(filepath.Join(root, "upload.bin")); !os.IsNotExist(err) {
		t.Fatalf("target should not exist after rejected offset, stat error = %v", err)
	}
}

func TestWriteProjectFileUploadChunkSupportsEmptyFile(t *testing.T) {
	root := t.TempDir()
	result := writeProjectFileUploadChunk(root, "empty.bin", "empty", 0, 0, nil, true, true)
	if result.Error != nil {
		t.Fatalf("empty upload error = %v", *result.Error)
	}
	if !result.Complete || result.Size != 0 {
		t.Fatalf("empty upload result = %#v", result)
	}
	if data, err := os.ReadFile(filepath.Join(root, "empty.bin")); err != nil {
		t.Fatalf("read empty upload: %v", err)
	} else if len(data) != 0 {
		t.Fatalf("empty upload content = %q", data)
	}
}

func TestProjectFileUploadStatusDetectsCompletedTarget(t *testing.T) {
	root := t.TempDir()
	content := []byte("already complete")
	if err := os.WriteFile(filepath.Join(root, "done.bin"), content, 0o644); err != nil {
		t.Fatalf("write completed target: %v", err)
	}

	status := projectFileUploadStatus(root, "done.bin", "upload-done", int64(len(content)))
	if status.Error != nil {
		t.Fatalf("upload status error = %v", *status.Error)
	}
	if !status.Complete || status.ResumeOffset != int64(len(content)) || status.UploadedBytes != int64(len(content)) {
		t.Fatalf("status = %#v, want completed target offset", status)
	}
}

func TestProjectFileActionsStayInsideWorkdir(t *testing.T) {
	root := t.TempDir()
	created := createProjectFileEntry(root, "notes/todo.txt", false)
	if created.Error != nil {
		t.Fatalf("create error = %v", *created.Error)
	}
	renamed := renameProjectFileEntry(root, "notes/todo.txt", "notes/done.txt")
	if renamed.Error != nil {
		t.Fatalf("rename error = %v", *renamed.Error)
	}
	if renamed.Path != "notes/done.txt" {
		t.Fatalf("renamed path = %q", renamed.Path)
	}
	deleted := deleteProjectFileEntry(root, "notes/done.txt")
	if deleted.Error != nil {
		t.Fatalf("delete error = %v", *deleted.Error)
	}
	if _, err := os.Stat(filepath.Join(root, "notes", "done.txt")); !os.IsNotExist(err) {
		t.Fatalf("deleted file still exists or stat error = %v", err)
	}
	outside := renameProjectFileEntry(root, "notes", "../moved")
	if outside.Error == nil {
		t.Fatalf("expected rename outside root to fail")
	}
}

func TestRunProjectCommandExecutesInWorkdir(t *testing.T) {
	root := t.TempDir()
	command := "pwd"
	if os.PathSeparator == '\\' {
		command = "(Get-Location).Path"
	}
	result := runProjectCommand(context.Background(), root, command, 10*time.Second, nil)
	if result.Error != nil {
		t.Fatalf("run command error = %v", *result.Error)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if strings.TrimSpace(result.Stdout) == "" {
		t.Fatalf("stdout is empty")
	}
}

func TestWindowsPowerShellExecutableIsAbsoluteOnWindows(t *testing.T) {
	if os.PathSeparator != '\\' {
		t.Skip("windows only")
	}
	path := windowsPowerShellExecutable()
	if !filepath.IsAbs(path) {
		t.Fatalf("powershell path = %q, want absolute path", path)
	}
}

func TestSelfUpdateCommandIncludesRunnerIdentity(t *testing.T) {
	if os.PathSeparator != '\\' {
		t.Skip("install-script self update is windows only")
	}
	agent := New(Config{
		ControlURL:  "http://control.local",
		RunnerID:    "runner_desktop",
		CodexPath:   "/usr/local/bin/codex",
		RunnerToken: "runner-token",
	}, nil)
	command, args, err := agent.selfUpdateCommand()
	if err != nil {
		t.Fatalf("self update command: %v", err)
	}
	joined := command + " " + strings.Join(args, " ")
	for _, want := range []string{
		"runner_id=runner_desktop",
		"codex_path=%2Fusr%2Flocal%2Fbin%2Fcodex",
		"RUNNER_UPDATE_TOKEN",
		"Authorization",
		"Bearer",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("self update command missing %q: %s", want, joined)
		}
	}
	for _, leaked := range []string{"runner_token=", "runner-token"} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("self update command leaked %q: %s", leaked, joined)
		}
	}
}

func TestSelfUpdateArtifactName(t *testing.T) {
	tests := []struct {
		goos   string
		goarch string
		want   string
	}{
		{goos: "linux", goarch: "amd64", want: "runner-linux-amd64"},
		{goos: "linux", goarch: "arm64", want: "runner-linux-arm64"},
		{goos: "darwin", goarch: "amd64", want: "runner-darwin-amd64"},
		{goos: "darwin", goarch: "arm64", want: "runner-darwin-arm64"},
		{goos: "windows", goarch: "amd64", want: "runner-windows-amd64.exe"},
	}
	for _, tt := range tests {
		got, err := selfUpdateArtifactName(tt.goos, tt.goarch)
		if err != nil {
			t.Fatalf("selfUpdateArtifactName(%q, %q): %v", tt.goos, tt.goarch, err)
		}
		if got != tt.want {
			t.Fatalf("selfUpdateArtifactName(%q, %q) = %q, want %q", tt.goos, tt.goarch, got, tt.want)
		}
	}
	if _, err := selfUpdateArtifactName("linux", "386"); err == nil {
		t.Fatalf("unsupported artifact lookup should fail")
	}
}

func TestSelfUpdateArtifactURLUsesArtifactEndpoint(t *testing.T) {
	artifact, err := selfUpdateArtifactName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skip(err)
	}
	tests := []struct {
		controlURL string
		wantBase   string
	}{
		{controlURL: "http://control.local/", wantBase: "http://control.local"},
		{controlURL: "https://control.local/base", wantBase: "https://control.local/base"},
		{controlURL: "ws://control.local/api/v1/runner/ws", wantBase: "http://control.local"},
		{controlURL: "wss://control.local", wantBase: "https://control.local"},
	}
	for _, tt := range tests {
		agent := New(Config{
			ControlURL:  tt.controlURL,
			RunnerID:    "runner_desktop",
			CodexPath:   "/usr/local/bin/codex",
			RunnerToken: "runner-token",
		}, nil)
		got, err := agent.selfUpdateArtifactURL(time.Unix(123, 0).UTC())
		if err != nil {
			t.Fatalf("self update artifact URL for %q: %v", tt.controlURL, err)
		}
		want := tt.wantBase + "/api/v1/runner/artifacts/" + artifact + "?t=123"
		if got != want {
			t.Fatalf("artifact URL for %q = %q, want %q", tt.controlURL, got, want)
		}
		if strings.Contains(got, "runner_token=") || strings.Contains(got, "runner-token") {
			t.Fatalf("artifact URL leaked runner token: %s", got)
		}
	}
}

func TestRunnerShutdownMessageStopsReconnect(t *testing.T) {
	shutdownCalled := make(chan struct{}, 1)
	registered := make(chan struct{}, 1)
	accepted := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/runner/ws" {
			http.NotFound(w, r)
			return
		}
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()

		var env Envelope
		if err := conn.ReadJSON(&env); err != nil {
			t.Errorf("read register: %v", err)
			return
		}
		if env.Type != "runner.register" {
			t.Errorf("first message = %q, want runner.register", env.Type)
			return
		}
		registered <- struct{}{}
		if err := conn.WriteJSON(Envelope{
			Type:      "runner.shutdown",
			MessageID: "msg_shutdown",
			SentAt:    time.Now().UTC(),
			Payload:   json.RawMessage(`{"reason":"server_deleted"}`),
		}); err != nil {
			t.Errorf("write shutdown: %v", err)
			return
		}
		if err := conn.ReadJSON(&env); err != nil {
			t.Errorf("read shutdown response: %v", err)
			return
		}
		if env.Type != "runner.shutdown.response" || env.MessageID != "msg_shutdown" {
			t.Errorf("shutdown response envelope = %s/%s", env.Type, env.MessageID)
			return
		}
		var payload RunnerControlResult
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			t.Errorf("decode shutdown response: %v", err)
			return
		}
		if !payload.Accepted {
			t.Errorf("shutdown accepted = false: %#v", payload)
			return
		}
		accepted <- struct{}{}
	}))
	defer server.Close()

	agent := New(Config{
		ControlURL:        strings.Replace(server.URL, "http://", "ws://", 1),
		RunnerID:          "runner_desktop",
		Hostname:          "desktop",
		HeartbeatInterval: time.Hour,
	}, nil)
	agent.SetShutdownFunc(func(context.Context) error {
		shutdownCalled <- struct{}{}
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- agent.Run(ctx)
	}()

	select {
	case <-registered:
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not register")
	}
	select {
	case <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not accept shutdown")
	}
	select {
	case <-shutdownCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown function was not called")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("agent run error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not stop after shutdown")
	}
}

func TestWindowsRunnerRunAsDetectsSystemProfile(t *testing.T) {
	t.Setenv("RUNNER_RUN_AS", "")
	t.Setenv("USERNAME", "SYSTEM")
	t.Setenv("USERPROFILE", `C:\Windows\System32\config\systemprofile`)
	if got := windowsRunnerRunAs(); got != "system" {
		t.Fatalf("run_as = %q, want system", got)
	}

	t.Setenv("RUNNER_RUN_AS", "user")
	if got := windowsRunnerRunAs(); got != "user" {
		t.Fatalf("explicit run_as = %q, want user", got)
	}
}

func TestHelperCodexProcess(t *testing.T) {
	mode := os.Getenv("CTW_HELPER_CODEX")
	if mode == "" {
		return
	}
	if mode == "hang" {
		time.Sleep(10 * time.Second)
		os.Exit(0)
	}
	fmt.Fprintln(os.Stdout, `{"text":"ok","session_id":"sess"}`)
	if mode == "plain_after_json" {
		fmt.Fprintln(os.Stdout, "plain shutdown output")
	}
	os.Exit(0)
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return wd
}

func stringPtr(value string) *string {
	return &value
}
