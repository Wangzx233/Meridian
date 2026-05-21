package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Config struct {
	ControlURL        string
	RunnerID          string
	Hostname          string
	Version           string
	CodexPath         string
	RunnerToken       string
	UseCodexSandbox   bool
	CompactTimeout    time.Duration
	HeartbeatInterval time.Duration
	Env               []string
}

type Envelope struct {
	Type      string          `json:"type"`
	MessageID string          `json:"message_id"`
	SentAt    time.Time       `json:"sent_at"`
	Payload   json.RawMessage `json:"payload"`
}

type Agent struct {
	cfg    Config
	logger *slog.Logger
	conn   *websocket.Conn
	mu     sync.Mutex
	active map[string]context.CancelFunc
}

var errRunnerNotConnected = errors.New("runner websocket is not connected")

func New(cfg Config, logger *slog.Logger) *Agent {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.HeartbeatInterval == 0 {
		cfg.HeartbeatInterval = 10 * time.Second
	}
	if cfg.Version == "" {
		cfg.Version = "0.5.0"
	}
	if cfg.CodexPath == "" {
		cfg.CodexPath = "codex"
	}
	if cfg.CompactTimeout == 0 {
		cfg.CompactTimeout = 5 * time.Minute
	}
	return &Agent{cfg: cfg, logger: logger, active: map[string]context.CancelFunc{}}
}

func (a *Agent) Run(ctx context.Context) error {
	retryDelay := time.Second
	for {
		connected, err := a.runOnce(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if connected {
			retryDelay = time.Second
		}
		a.logger.Warn("runner websocket disconnected", "error", err, "retry_in", retryDelay)
		if !sleepContext(ctx, retryDelay) {
			return nil
		}
		if !connected {
			retryDelay *= 2
			if retryDelay > 30*time.Second {
				retryDelay = 30 * time.Second
			}
		}
	}
}

func (a *Agent) runOnce(ctx context.Context) (bool, error) {
	u, err := url.Parse(a.cfg.ControlURL)
	if err != nil {
		return false, err
	}
	if u.Scheme == "http" {
		u.Scheme = "ws"
	}
	if u.Scheme == "https" {
		u.Scheme = "wss"
	}
	u.Path = "/api/v1/runner/ws"

	header := make(map[string][]string)
	if a.cfg.RunnerToken != "" {
		header["Authorization"] = []string{"Bearer " + a.cfg.RunnerToken}
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), header)
	if err != nil {
		return false, err
	}
	a.setConn(conn)
	connCtx, stopHeartbeat := context.WithCancel(ctx)
	terminals := newTerminalManager(a.cfg.Env, a.send)
	defer func() {
		stopHeartbeat()
		terminals.closeAll()
		a.clearConn(conn)
		_ = conn.Close()
	}()

	if err := a.register(); err != nil {
		return true, err
	}
	go a.heartbeatLoop(connCtx)
	for {
		var env Envelope
		if err := conn.ReadJSON(&env); err != nil {
			return true, err
		}
		switch env.Type {
		case "run.assign":
			var assign Assignment
			if err := json.Unmarshal(env.Payload, &assign); err != nil {
				a.logger.Warn("invalid assignment", "error", err)
				continue
			}
			a.normalizeAssignment(&assign)
			runCtx, cancel := context.WithCancel(ctx)
			if !a.addActive(assign.RunID, cancel) {
				cancel()
				a.logger.Warn("duplicate active assignment ignored", "run_id", assign.RunID)
				continue
			}
			go a.execute(ctx, runCtx, assign)
		case "run.cancel":
			var payload struct {
				RunID string `json:"run_id"`
			}
			if err := json.Unmarshal(env.Payload, &payload); err != nil {
				a.logger.Warn("invalid cancel", "error", err)
				continue
			}
			accepted := a.cancel(payload.RunID)
			_ = a.send("run.cancel_ack", map[string]any{"run_id": payload.RunID, "accepted": accepted})
		case "runner.update":
			_ = a.sendResponse("runner.update.response", env.MessageID, a.startSelfUpdate(ctx))
		case "fs.list":
			var payload struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(env.Payload, &payload); err != nil {
				a.logger.Warn("invalid fs.list", "error", err)
				continue
			}
			_ = a.sendResponse("fs.list.response", env.MessageID, listDirectories(payload.Path))
		case "project.files":
			var payload struct {
				Workdir string `json:"workdir"`
				Path    string `json:"path"`
			}
			if err := json.Unmarshal(env.Payload, &payload); err != nil {
				a.logger.Warn("invalid project.files", "error", err)
				continue
			}
			_ = a.sendResponse("project.files.response", env.MessageID, listProjectFiles(payload.Workdir, payload.Path))
		case "project.file.read":
			var payload struct {
				Workdir  string `json:"workdir"`
				Path     string `json:"path"`
				MaxBytes int64  `json:"max_bytes"`
			}
			if err := json.Unmarshal(env.Payload, &payload); err != nil {
				a.logger.Warn("invalid project.file.read", "error", err)
				continue
			}
			_ = a.sendResponse("project.file.read.response", env.MessageID, readProjectFile(payload.Workdir, payload.Path, payload.MaxBytes))
		case "project.file.write":
			var payload struct {
				Workdir    string `json:"workdir"`
				Path       string `json:"path"`
				Content    string `json:"content"`
				CreateDirs bool   `json:"create_dirs"`
			}
			if err := json.Unmarshal(env.Payload, &payload); err != nil {
				a.logger.Warn("invalid project.file.write", "error", err)
				continue
			}
			_ = a.sendResponse("project.file.write.response", env.MessageID, writeProjectFile(payload.Workdir, payload.Path, payload.Content, payload.CreateDirs))
		case "project.file.upload":
			var payload struct {
				Workdir       string `json:"workdir"`
				Path          string `json:"path"`
				ContentBase64 string `json:"content_base64"`
				CreateDirs    bool   `json:"create_dirs"`
			}
			if err := json.Unmarshal(env.Payload, &payload); err != nil {
				a.logger.Warn("invalid project.file.upload", "error", err)
				continue
			}
			content, err := base64.StdEncoding.DecodeString(payload.ContentBase64)
			if err != nil {
				msg := "invalid base64 file content"
				_ = a.sendResponse("project.file.upload.response", env.MessageID, ProjectFileActionResult{Path: payload.Path, Error: &msg})
				continue
			}
			_ = a.sendResponse("project.file.upload.response", env.MessageID, writeProjectFileBytes(payload.Workdir, payload.Path, content, payload.CreateDirs))
		case "project.file.action":
			var payload struct {
				Workdir    string `json:"workdir"`
				Action     string `json:"action"`
				Path       string `json:"path"`
				TargetPath string `json:"target_path"`
				IsDir      bool   `json:"is_dir"`
			}
			if err := json.Unmarshal(env.Payload, &payload); err != nil {
				a.logger.Warn("invalid project.file.action", "error", err)
				continue
			}
			var result ProjectFileActionResult
			switch payload.Action {
			case "create":
				result = createProjectFileEntry(payload.Workdir, payload.Path, payload.IsDir)
			case "rename":
				result = renameProjectFileEntry(payload.Workdir, payload.Path, payload.TargetPath)
			case "delete":
				result = deleteProjectFileEntry(payload.Workdir, payload.Path)
			default:
				msg := "unsupported file action"
				result = ProjectFileActionResult{Path: payload.Path, Error: &msg}
			}
			_ = a.sendResponse("project.file.action.response", env.MessageID, result)
		case "project.command":
			var payload struct {
				Workdir     string `json:"workdir"`
				Command     string `json:"command"`
				TimeoutSecs int    `json:"timeout_secs"`
			}
			if err := json.Unmarshal(env.Payload, &payload); err != nil {
				a.logger.Warn("invalid project.command", "error", err)
				continue
			}
			messageID := env.MessageID
			timeout := time.Duration(payload.TimeoutSecs) * time.Second
			go func() {
				result := runProjectCommand(ctx, payload.Workdir, payload.Command, timeout, a.cfg.Env)
				_ = a.sendResponse("project.command.response", messageID, result)
			}()
		case "project.terminal.open":
			var payload struct {
				TerminalID string `json:"terminal_id"`
				Workdir    string `json:"workdir"`
				Cols       int    `json:"cols"`
				Rows       int    `json:"rows"`
			}
			if err := json.Unmarshal(env.Payload, &payload); err != nil {
				a.logger.Warn("invalid project.terminal.open", "error", err)
				continue
			}
			_ = a.sendResponse("project.terminal.open.response", env.MessageID, terminals.open(connCtx, payload.TerminalID, payload.Workdir, payload.Cols, payload.Rows))
		case "project.terminal.input":
			var payload struct {
				TerminalID string `json:"terminal_id"`
				Data       string `json:"data"`
			}
			if err := json.Unmarshal(env.Payload, &payload); err != nil {
				a.logger.Warn("invalid project.terminal.input", "error", err)
				continue
			}
			terminals.input(payload.TerminalID, payload.Data)
		case "project.terminal.resize":
			var payload struct {
				TerminalID string `json:"terminal_id"`
				Cols       int    `json:"cols"`
				Rows       int    `json:"rows"`
			}
			if err := json.Unmarshal(env.Payload, &payload); err != nil {
				a.logger.Warn("invalid project.terminal.resize", "error", err)
				continue
			}
			terminals.resize(payload.TerminalID, payload.Cols, payload.Rows)
		case "project.terminal.close":
			var payload struct {
				TerminalID string `json:"terminal_id"`
			}
			if err := json.Unmarshal(env.Payload, &payload); err != nil {
				a.logger.Warn("invalid project.terminal.close", "error", err)
				continue
			}
			terminals.close(payload.TerminalID)
		default:
			a.logger.Warn("unknown control message", "type", env.Type)
		}
	}
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (a *Agent) setConn(conn *websocket.Conn) {
	a.mu.Lock()
	a.conn = conn
	a.mu.Unlock()
}

func (a *Agent) clearConn(conn *websocket.Conn) {
	a.mu.Lock()
	if a.conn == conn {
		a.conn = nil
	}
	a.mu.Unlock()
}

func (a *Agent) normalizeAssignment(assign *Assignment) {
	if len(assign.Argv) > 0 {
		if a.cfg.CodexPath != "" {
			assign.Argv[0] = a.cfg.CodexPath
		}
		assign.Argv = withSkipGitRepoCheck(assign.Argv)
		if a.cfg.UseCodexSandbox {
			assign.Argv = withoutArg(assign.Argv, "--dangerously-bypass-approvals-and-sandbox")
		} else {
			assign.Argv = withBypassApprovalsAndSandbox(assign.Argv)
		}
		return
	}
	args := []string{a.cfg.CodexPath, "--cd", assign.Workdir}
	if assign.CodexModel != nil && strings.TrimSpace(*assign.CodexModel) != "" {
		args = append(args, "--model", strings.TrimSpace(*assign.CodexModel))
	}
	if assign.ReasoningEffort != nil && strings.TrimSpace(*assign.ReasoningEffort) != "" {
		args = append(args, "--config", `model_reasoning_effort="`+strings.TrimSpace(*assign.ReasoningEffort)+`"`)
	}
	if assign.ServiceTier != nil && strings.TrimSpace(*assign.ServiceTier) != "" {
		args = append(args, "--config", `service_tier="`+strings.TrimSpace(*assign.ServiceTier)+`"`)
	}
	if assign.Mode == "resume" {
		session := ""
		if assign.CodexSessionID != nil {
			session = *assign.CodexSessionID
		}
		execArgs := []string{"exec", "resume"}
		if !a.cfg.UseCodexSandbox {
			execArgs = append(execArgs, "--dangerously-bypass-approvals-and-sandbox")
		}
		execArgs = append(execArgs, "--skip-git-repo-check", "--json", session, "-")
		assign.Argv = append(args, execArgs...)
		return
	}
	execArgs := []string{"exec"}
	if !a.cfg.UseCodexSandbox {
		execArgs = append(execArgs, "--dangerously-bypass-approvals-and-sandbox")
	}
	execArgs = append(execArgs, "--skip-git-repo-check", "--json", "-")
	assign.Argv = append(args, execArgs...)
}

func withBypassApprovalsAndSandbox(argv []string) []string {
	return withCodexExecFlag(argv, "--dangerously-bypass-approvals-and-sandbox")
}

func withSkipGitRepoCheck(argv []string) []string {
	return withCodexExecFlag(argv, "--skip-git-repo-check")
}

func withCodexExecFlag(argv []string, flag string) []string {
	if len(argv) == 0 || hasArg(argv, flag) {
		return argv
	}
	execIndex := indexArg(argv, "exec")
	if execIndex < 0 {
		return argv
	}
	insertAt := execIndex + 1
	if insertAt < len(argv) && argv[insertAt] == "resume" {
		insertAt++
	}
	out := make([]string, 0, len(argv)+1)
	out = append(out, argv[:insertAt]...)
	out = append(out, flag)
	out = append(out, argv[insertAt:]...)
	return out
}

func withoutArg(argv []string, value string) []string {
	out := argv[:0]
	for _, arg := range argv {
		if arg != value {
			out = append(out, arg)
		}
	}
	return out
}

func hasArg(argv []string, value string) bool {
	return indexArg(argv, value) >= 0
}

func indexArg(argv []string, value string) int {
	for i, arg := range argv {
		if arg == value {
			return i
		}
	}
	return -1
}

func (a *Agent) execute(agentCtx, runCtx context.Context, assign Assignment) {
	defer func() {
		a.mu.Lock()
		delete(a.active, assign.RunID)
		a.mu.Unlock()
	}()

	var seq int64
	execCtx := runCtx
	var timeoutCancel context.CancelFunc
	compactTimeout := compactAssignmentTimeout(assign, a.cfg.CompactTimeout)
	if compactTimeout > 0 {
		execCtx, timeoutCancel = context.WithTimeout(runCtx, compactTimeout)
		defer timeoutCancel()
	}
	runner := CodexRunner{
		Env: a.cfg.Env,
		OnStarted: func(pid int) {
			_ = a.send("run.started", map[string]any{
				"run_id":     assign.RunID,
				"pid":        pid,
				"started_at": time.Now().UTC(),
			})
		},
	}
	result := runner.Run(execCtx, assign, func(ev CodexEvent) {
		seq++
		_ = a.send("run.event", map[string]any{
			"run_id":        assign.RunID,
			"source_seq":    seq,
			"event_type":    ev.EventType,
			"stream":        ev.Stream,
			"event_payload": ev.Payload,
			"occurred_at":   time.Now().UTC(),
		})
	})
	if compactTimeout > 0 && errors.Is(execCtx.Err(), context.DeadlineExceeded) {
		msg := "Compact command timed out after " + compactTimeout.String() + ". Codex CLI did not finish, so the runner stopped the process and returned the task to the user."
		payload, _ := json.Marshal(map[string]any{"code": "compact_timeout", "message": msg})
		seq++
		_ = a.send("run.event", map[string]any{
			"run_id":        assign.RunID,
			"source_seq":    seq,
			"event_type":    "runner.error",
			"stream":        "system",
			"event_payload": payload,
			"occurred_at":   time.Now().UTC(),
		})
		result.Status = "failed"
		result.ExitCode = nil
		result.ErrorMessage = &msg
	}
	a.sendRunCompleted(agentCtx, assign.RunID, result)
}

func (a *Agent) sendRunCompleted(ctx context.Context, runID string, result RunResult) {
	payload := map[string]any{
		"run_id":           runID,
		"status":           result.Status,
		"exit_code":        result.ExitCode,
		"error_message":    result.ErrorMessage,
		"final_message":    result.FinalMessage,
		"codex_session_id": result.CodexSessionID,
		"ended_at":         time.Now().UTC(),
	}
	for {
		if err := a.send("run.completed", payload); err != nil {
			a.logger.Warn("send run completion failed; retrying", "run_id", runID, "error", err)
			if !sleepContext(ctx, 2*time.Second) {
				return
			}
			continue
		}
		return
	}
}

func compactAssignmentTimeout(assign Assignment, timeout time.Duration) time.Duration {
	if assign.Mode != "resume" || strings.TrimSpace(assign.Prompt) != "/compact" {
		return 0
	}
	return timeout
}

func (a *Agent) addActive(runID string, cancel context.CancelFunc) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, exists := a.active[runID]; exists {
		return false
	}
	a.active[runID] = cancel
	return true
}

func (a *Agent) cancel(runID string) bool {
	a.mu.Lock()
	cancel := a.active[runID]
	a.mu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

func (a *Agent) register() error {
	return a.send("runner.register", map[string]any{
		"runner_id":      a.cfg.RunnerID,
		"hostname":       a.cfg.Hostname,
		"version":        a.cfg.Version,
		"codex_path":     a.cfg.CodexPath,
		"active_run_ids": a.activeRuns(),
		"capabilities": map[string]any{
			"codex_exec":          true,
			"cancel":              true,
			"fs_list":             true,
			"project_files":       true,
			"project_file_io":     true,
			"project_file_upload": true,
			"project_command":     true,
			"project_terminal":    true,
			"codex_options":       true,
			"active_runs":         true,
			"self_update":         true,
		},
	})
}

func (a *Agent) startSelfUpdate(ctx context.Context) RunnerUpdateResult {
	command, args, err := a.selfUpdateCommand()
	if err != nil {
		msg := err.Error()
		return RunnerUpdateResult{Accepted: false, Message: "Runner update is not available on this host.", Error: &msg}
	}
	go func() {
		if err := a.runSelfUpdate(ctx, command, args); err != nil {
			a.logger.Warn("runner self-update failed", "error", err)
		}
	}()
	return RunnerUpdateResult{Accepted: true, Message: "Runner update started. The websocket may disconnect while the runner restarts."}
}

func (a *Agent) selfUpdateCommand() (string, []string, error) {
	controlURL := strings.TrimRight(a.cfg.ControlURL, "/")
	if controlURL == "" {
		return "", nil, errors.New("CONTROL_URL is not set")
	}
	runnerID := strings.TrimSpace(a.cfg.RunnerID)
	if runnerID == "" {
		return "", nil, errors.New("RUNNER_ID is not set")
	}
	query := url.Values{}
	query.Set("runner_id", runnerID)
	if a.cfg.CodexPath != "" && a.cfg.CodexPath != "codex" {
		query.Set("codex_path", a.cfg.CodexPath)
	}

	switch runtime.GOOS {
	case "windows":
		runAs := windowsRunnerRunAs()
		query.Set("run_as", runAs)
		installURL := controlURL + "/api/v1/runner/install.ps1?" + query.Encode()
		script := "$headers=@{}; if ($env:RUNNER_UPDATE_TOKEN) { $headers['Authorization']='Bearer ' + $env:RUNNER_UPDATE_TOKEN }; Start-Sleep -Seconds 2; iex ((iwr -UseBasicParsing -Uri '" + strings.ReplaceAll(installURL, "'", "''") + "' -Headers $headers).Content)"
		return windowsPowerShellExecutable(), []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script}, nil
	case "linux":
		installURL := controlURL + "/api/v1/runner/install.sh?" + query.Encode()
		installCommand := runnerInstallShellCommand(installURL)
		script := "if command -v systemd-run >/dev/null 2>&1 && systemd-run --unit codex-task-workbench-runner-update --collect /bin/sh -lc " + shellSingleQuote(installCommand) + "; then :; else (" + installCommand + ") >/tmp/codex-task-workbench-runner-update.log 2>&1 </dev/null & fi"
		return "/bin/sh", []string{"-lc", script}, nil
	case "darwin":
		installURL := controlURL + "/api/v1/runner/install.sh?" + query.Encode()
		script := "(" + runnerInstallShellCommand(installURL) + ") >/tmp/codex-task-workbench-runner-update.log 2>&1 </dev/null &"
		return "/bin/sh", []string{"-lc", script}, nil
	default:
		return "", nil, errors.New("unsupported operating system " + runtime.GOOS)
	}
}

func windowsRunnerRunAs() string {
	runAs := strings.ToLower(strings.TrimSpace(os.Getenv("RUNNER_RUN_AS")))
	if runAs == "user" || runAs == "system" {
		return runAs
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("USERNAME")), "SYSTEM") {
		return "system"
	}
	if strings.Contains(strings.ToLower(os.Getenv("USERPROFILE")), `systemprofile`) {
		return "system"
	}
	return "user"
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func runnerInstallShellCommand(installURL string) string {
	quotedURL := shellSingleQuote(installURL)
	return `sleep 2; if [ -z "${RUNNER_UPDATE_TOKEN:-}" ] && [ -r /etc/codex-task-workbench-runner.env ]; then RUNNER_UPDATE_TOKEN="$(sed -n 's/^RUNNER_TOKEN=//p' /etc/codex-task-workbench-runner.env | tail -n 1)"; fi; if [ -n "${RUNNER_UPDATE_TOKEN:-}" ]; then curl -fsSL -H "Authorization: Bearer ${RUNNER_UPDATE_TOKEN}" ` + quotedURL + ` | sh; else curl -fsSL ` + quotedURL + ` | sh; fi`
}

func (a *Agent) runSelfUpdate(ctx context.Context, command string, args []string) error {
	updateCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(updateCtx, command, args...)
	cmd.Env = mergedEnv(a.cfg.Env)
	if a.cfg.RunnerToken != "" {
		cmd.Env = mergedEnvList(cmd.Env).with("RUNNER_UPDATE_TOKEN=" + a.cfg.RunnerToken)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

func (a *Agent) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(a.cfg.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = a.send("runner.heartbeat", map[string]any{
				"runner_id":      a.cfg.RunnerID,
				"active_run_ids": a.activeRuns(),
			})
		}
	}
}

func (a *Agent) activeRuns() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]string, 0, len(a.active))
	for runID := range a.active {
		out = append(out, runID)
	}
	return out
}

func (a *Agent) send(typ string, payload any) error {
	return a.sendResponse(typ, "msg_"+time.Now().UTC().Format("20060102150405.000000000"), payload)
}

func (a *Agent) sendResponse(typ, messageID string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	env := Envelope{
		Type:      typ,
		MessageID: messageID,
		SentAt:    time.Now().UTC(),
		Payload:   raw,
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.conn == nil {
		return errRunnerNotConnected
	}
	return a.conn.WriteJSON(env)
}
