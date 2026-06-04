package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

type Assignment struct {
	RunID                   string                    `json:"run_id"`
	TaskID                  string                    `json:"task_id"`
	ProjectID               string                    `json:"project_id"`
	Workdir                 string                    `json:"workdir"`
	Mode                    string                    `json:"mode"`
	CodexSessionID          *string                   `json:"codex_session_id"`
	CodexModel              *string                   `json:"codex_model,omitempty"`
	ReasoningEffort         *string                   `json:"codex_reasoning_effort,omitempty"`
	ServiceTier             *string                   `json:"codex_service_tier,omitempty"`
	ReminderCallbackEnabled bool                      `json:"reminder_callback_enabled,omitempty"`
	Prompt                  string                    `json:"prompt"`
	Argv                    []string                  `json:"argv"`
	InputImages             []RunInputImageAttachment `json:"input_images,omitempty"`
}

type RunInputImageAttachment struct {
	ID            string `json:"id"`
	Filename      string `json:"filename"`
	MimeType      string `json:"mime_type"`
	ContentBase64 string `json:"content_base64"`
}

type CodexEvent struct {
	EventType string
	Stream    string
	Payload   json.RawMessage
}

type RunResult struct {
	Status         string
	ExitCode       *int
	ErrorMessage   *string
	FinalMessage   *string
	CodexSessionID *string
}

type RunnerControlResult struct {
	Accepted bool    `json:"accepted"`
	Message  string  `json:"message"`
	Error    *string `json:"error,omitempty"`
}

type RunnerUpdateRequest struct {
	UpdateID      string `json:"update_id,omitempty"`
	TargetVersion string `json:"target_version,omitempty"`
}

type RunnerUpdateStatus struct {
	UpdateID      string    `json:"update_id"`
	RunnerID      string    `json:"runner_id,omitempty"`
	Status        string    `json:"status"`
	Message       string    `json:"message,omitempty"`
	Version       string    `json:"version,omitempty"`
	TargetVersion string    `json:"target_version,omitempty"`
	Error         *string   `json:"error,omitempty"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type CodexRunner struct {
	Env       []string
	OnStarted func(pid int)
}

func (r *CodexRunner) Run(ctx context.Context, assign Assignment, onEvent func(CodexEvent)) RunResult {
	if len(assign.Argv) == 0 {
		msg := "missing argv"
		emitRunnerError(onEvent, "missing_argv", msg)
		return RunResult{Status: "failed", ErrorMessage: &msg}
	}
	env := mergedEnv(r.Env)
	argv := append([]string(nil), assign.Argv...)
	cleanupImages := func() {}
	if len(assign.InputImages) > 0 {
		var imageErr error
		argv, cleanupImages, imageErr = prepareInputImages(assign.RunID, argv, assign.InputImages)
		if imageErr != nil {
			msg := imageErr.Error()
			emitRunnerError(onEvent, "image_input_failed", msg)
			return RunResult{Status: "failed", ErrorMessage: &msg}
		}
		defer cleanupImages()
	}
	resolved, err := resolveExecutable(argv[0], env)
	if err != nil {
		msg := err.Error()
		emitRunnerError(onEvent, executableNotFoundCode(argv[0]), msg)
		return RunResult{Status: "failed", ErrorMessage: &msg}
	}
	argv[0] = resolved

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = assign.Workdir
	if len(r.Env) > 0 {
		cmd.Env = env
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		msg := err.Error()
		emitRunnerError(onEvent, "process_setup_failed", msg)
		return RunResult{Status: "failed", ErrorMessage: &msg}
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		msg := err.Error()
		emitRunnerError(onEvent, "process_setup_failed", msg)
		return RunResult{Status: "failed", ErrorMessage: &msg}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		msg := err.Error()
		emitRunnerError(onEvent, "process_setup_failed", msg)
		return RunResult{Status: "failed", ErrorMessage: &msg}
	}
	if err := cmd.Start(); err != nil {
		msg := startErrorMessage(argv[0], err, env)
		code := "process_start_failed"
		if os.IsNotExist(err) || errors.Is(err, exec.ErrNotFound) {
			code = executableNotFoundCode(argv[0])
		}
		emitRunnerError(onEvent, code, msg)
		return RunResult{Status: "failed", ErrorMessage: &msg}
	}
	if r.OnStarted != nil {
		r.OnStarted(cmd.Process.Pid)
	}
	go func() {
		_, _ = io.WriteString(stdin, assign.Prompt)
		if !strings.HasSuffix(assign.Prompt, "\n") {
			_, _ = io.WriteString(stdin, "\n")
		}
		_ = stdin.Close()
	}()

	var final atomic.Value
	var session atomic.Value
	done := make(chan struct{}, 2)
	go func() {
		scanJSONL(stdout, func(ev CodexEvent) {
			if ev.EventType == "codex.event" {
				extract := extractCodexFieldsFromNormalized(ev.Payload)
				if extract.Text != "" {
					final.Store(extract.Text)
				}
				if extract.SessionID != "" {
					session.Store(extract.SessionID)
				}
			}
			onEvent(ev)
		})
		done <- struct{}{}
	}()
	go func() {
		scanText(stderr, "stderr", onEvent)
		done <- struct{}{}
	}()

	waitErr := waitForProcessAndReaders(cmd, stdout, stderr, done)

	result := RunResult{}
	if s, ok := final.Load().(string); ok && s != "" {
		result.FinalMessage = &s
	}
	if s, ok := session.Load().(string); ok && s != "" {
		result.CodexSessionID = &s
	}
	if ctx.Err() != nil {
		result.Status = "canceled"
		msg := ctx.Err().Error()
		result.ErrorMessage = &msg
		return result
	}
	if waitErr == nil {
		code := 0
		result.Status = "succeeded"
		result.ExitCode = &code
		return result
	}
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		code := exitErr.ExitCode()
		result.ExitCode = &code
		result.Status = "failed"
		msg := waitErr.Error()
		result.ErrorMessage = &msg
		return result
	}
	result.Status = "failed"
	msg := waitErr.Error()
	result.ErrorMessage = &msg
	return result
}

func prepareInputImages(runID string, argv []string, images []RunInputImageAttachment) ([]string, func(), error) {
	if len(images) == 0 {
		return argv, func() {}, nil
	}
	dir, err := os.MkdirTemp("", "meridian-run-images-"+safeTempNamePart(runID)+"-")
	if err != nil {
		return nil, func() {}, fmt.Errorf("create image input temp dir: %w", err)
	}
	cleanup := func() {
		_ = os.RemoveAll(dir)
	}

	paths := make([]string, 0, len(images))
	used := map[string]int{}
	for index, image := range images {
		content, err := base64.StdEncoding.DecodeString(strings.TrimSpace(image.ContentBase64))
		if err != nil {
			cleanup()
			return nil, func() {}, fmt.Errorf("decode image %q: %w", image.Filename, err)
		}
		if len(content) == 0 {
			cleanup()
			return nil, func() {}, fmt.Errorf("image %q is empty", image.Filename)
		}
		name := safeInputImageFilename(image.Filename, index)
		key := strings.ToLower(name)
		if count := used[key]; count > 0 {
			ext := filepath.Ext(name)
			stem := strings.TrimSuffix(name, ext)
			name = stem + "-" + intString(count+1) + ext
		}
		used[key]++
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, content, 0o600); err != nil {
			cleanup()
			return nil, func() {}, fmt.Errorf("write image %q: %w", image.Filename, err)
		}
		paths = append(paths, path)
	}

	return withImageArgs(argv, paths), cleanup, nil
}

func withImageArgs(argv []string, paths []string) []string {
	if len(paths) == 0 {
		return argv
	}
	argv = withoutImageArgs(argv)
	insertAt := len(argv)
	if jsonIndex := indexArg(argv, "--json"); jsonIndex >= 0 {
		insertAt = jsonIndex
	} else if dashIndex := indexArg(argv, "-"); dashIndex >= 0 {
		insertAt = dashIndex
	}
	out := make([]string, 0, len(argv)+len(paths)*2)
	out = append(out, argv[:insertAt]...)
	for _, path := range paths {
		out = append(out, "--image", path)
	}
	out = append(out, argv[insertAt:]...)
	return out
}

func withoutImageArgs(argv []string) []string {
	out := make([]string, 0, len(argv))
	for index := 0; index < len(argv); index++ {
		arg := argv[index]
		if arg == "--image" || arg == "-i" {
			if index+1 < len(argv) {
				index++
			}
			continue
		}
		out = append(out, arg)
	}
	return out
}

func safeInputImageFilename(filename string, index int) string {
	filename = strings.TrimSpace(filepath.Base(strings.ReplaceAll(filename, "\\", "/")))
	if filename == "" || filename == "." || filename == string(filepath.Separator) {
		filename = "image-" + intString(index+1) + ".img"
	}
	filename = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == 0 || r < 32 {
			return '-'
		}
		return r
	}, filename)
	if len(filename) > 160 {
		ext := filepath.Ext(filename)
		stem := strings.TrimSuffix(filename, ext)
		if len(ext) > 20 {
			ext = ""
		}
		if len(stem) > 140 {
			stem = stem[:140]
		}
		filename = stem + ext
	}
	return filename
}

func safeTempNamePart(value string) string {
	value = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, value)
	if value == "" {
		return "run"
	}
	if len(value) > 64 {
		return value[:64]
	}
	return value
}

func intString(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func waitForProcessAndReaders(cmd *exec.Cmd, stdout, stderr io.Closer, done <-chan struct{}) error {
	waitErrCh := make(chan error, 1)
	go func() {
		waitErrCh <- cmd.Wait()
	}()

	readers := 2
	var waitErr error
	for readers > 0 {
		select {
		case <-done:
			readers--
		case waitErr = <-waitErrCh:
			timer := time.NewTimer(2 * time.Second)
			for readers > 0 {
				select {
				case <-done:
					readers--
				case <-timer.C:
					_ = stdout.Close()
					_ = stderr.Close()
					return waitErr
				}
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return waitErr
		}
	}
	return <-waitErrCh
}

func emitRunnerError(onEvent func(CodexEvent), code, message string) {
	if onEvent == nil {
		return
	}
	payload, _ := json.Marshal(map[string]any{"code": code, "message": message})
	onEvent(CodexEvent{EventType: "runner.error", Stream: "system", Payload: payload})
}

func resolveExecutable(file string, env []string) (string, error) {
	if strings.TrimSpace(file) == "" {
		return "", errors.New("missing executable path")
	}
	if hasPathSeparator(file) {
		return file, nil
	}
	if resolved, ok := lookPathEnv(file, env); ok {
		return resolved, nil
	}
	return "", fmt.Errorf("%s executable %q was not found in PATH=%q. Install it on the runner host, or set CODEX_PATH to an absolute executable path and restart the runner", executableLabel(file), file, envValue(env, "PATH"))
}

func lookPathEnv(file string, env []string) (string, bool) {
	pathValue := envValue(env, "PATH")
	for _, dir := range filepath.SplitList(pathValue) {
		for _, name := range executableNames(file, env) {
			candidate := name
			if dir != "" {
				candidate = filepath.Join(dir, name)
			}
			if isExecutableFile(candidate) {
				return candidate, true
			}
		}
	}
	return "", false
}

func executableNames(file string, env []string) []string {
	if runtime.GOOS != "windows" || filepath.Ext(file) != "" {
		return []string{file}
	}
	pathext := envValue(env, "PATHEXT")
	if strings.TrimSpace(pathext) == "" {
		pathext = ".COM;.EXE;.BAT;.CMD"
	}
	names := []string{file}
	for _, ext := range strings.Split(pathext, ";") {
		ext = strings.TrimSpace(ext)
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		names = append(names, file+ext)
	}
	return names
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

func hasPathSeparator(path string) bool {
	return strings.ContainsAny(path, `/\`)
}

func envValue(env []string, key string) string {
	for i := len(env) - 1; i >= 0; i-- {
		envKey, value, ok := strings.Cut(env[i], "=")
		if ok && envKeyEqual(envKey, key) {
			return value
		}
	}
	return ""
}

func envKeyEqual(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func executableNotFoundCode(file string) string {
	if strings.EqualFold(filepath.Base(file), "codex") || strings.EqualFold(filepath.Base(file), "codex.exe") || strings.EqualFold(filepath.Base(file), "codex.cmd") {
		return "codex_not_found"
	}
	return "executable_not_found"
}

func executableLabel(file string) string {
	if executableNotFoundCode(file) == "codex_not_found" {
		return "Codex"
	}
	return "Configured"
}

func startErrorMessage(file string, err error, env []string) string {
	if os.IsNotExist(err) || errors.Is(err, exec.ErrNotFound) {
		return fmt.Sprintf("%s executable %q could not be started: %v. PATH=%q. Set CODEX_PATH to an absolute executable path and restart the runner", executableLabel(file), file, err, envValue(env, "PATH"))
	}
	return err.Error()
}

func KillProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	time.Sleep(2 * time.Second)
	_ = cmd.Process.Kill()
}

func scanJSONL(r io.Reader, onEvent func(CodexEvent)) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var raw json.RawMessage
		if json.Valid(line) {
			raw = append(raw, line...)
			payload := normalizeCodexPayload(raw)
			onEvent(CodexEvent{EventType: "codex.event", Stream: "jsonl", Payload: payload})
		} else {
			payload, _ := json.Marshal(map[string]any{"text": string(line)})
			onEvent(CodexEvent{EventType: "process.output", Stream: "stdout", Payload: payload})
		}
	}
}

func scanText(r io.Reader, stream string, onEvent func(CodexEvent)) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 16*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		payload, _ := json.Marshal(map[string]any{"text": line})
		onEvent(CodexEvent{EventType: "process.output", Stream: stream, Payload: payload})
	}
}

type extractedFields struct {
	Text      string
	SessionID string
}

func normalizeCodexPayload(raw json.RawMessage) json.RawMessage {
	fields := extractCodexFields(raw)
	payload := map[string]any{"raw": json.RawMessage(raw)}
	if fields.Text != "" {
		payload["text"] = fields.Text
	}
	if fields.SessionID != "" {
		payload["session_id"] = fields.SessionID
	}
	out, _ := json.Marshal(payload)
	return out
}

func extractCodexFields(raw json.RawMessage) extractedFields {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return extractedFields{}
	}
	m, ok := v.(map[string]any)
	if !ok {
		return extractedFields{}
	}
	var out extractedFields
	out.SessionID = firstString(m, "session_id", "sessionId", "thread_id", "threadId", "conversation_id", "conversationId")
	out.Text = firstString(m, "text", "message", "content", "final_message", "finalMessage")
	if out.Text == "" {
		out.Text = nestedString(m, []string{"message", "content"})
	}
	if out.Text == "" {
		out.Text = nestedString(m, []string{"delta", "text"})
	}
	if out.Text == "" {
		out.Text = nestedString(m, []string{"item", "text"})
	}
	if out.Text == "" {
		out.Text = nestedString(m, []string{"output", "text"})
	}
	if out.SessionID == "" {
		out.SessionID = nestedString(m, []string{"session", "id"})
	}
	return out
}

func extractCodexFieldsFromNormalized(raw json.RawMessage) extractedFields {
	var payload struct {
		Text      string          `json:"text"`
		SessionID string          `json:"session_id"`
		Raw       json.RawMessage `json:"raw"`
	}
	if err := json.Unmarshal(raw, &payload); err == nil {
		out := extractedFields{Text: payload.Text, SessionID: payload.SessionID}
		if out.Text != "" || out.SessionID != "" {
			return out
		}
		if len(payload.Raw) > 0 {
			return extractCodexFields(payload.Raw)
		}
	}
	return extractCodexFields(raw)
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key].(string); ok {
			return v
		}
	}
	return ""
}

func nestedString(m map[string]any, path []string) string {
	var current any = m
	for _, key := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = obj[key]
	}
	if s, ok := current.(string); ok {
		return s
	}
	return ""
}
