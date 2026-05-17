package agent

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	pty "github.com/aymanbagabas/go-pty"
)

type TerminalOpenResult struct {
	TerminalID string  `json:"terminal_id"`
	Workdir    string  `json:"workdir"`
	Error      *string `json:"error,omitempty"`
}

type TerminalOutput struct {
	TerminalID string `json:"terminal_id"`
	Data       string `json:"data"`
}

type TerminalExit struct {
	TerminalID string  `json:"terminal_id"`
	ExitCode   int     `json:"exit_code"`
	Error      *string `json:"error,omitempty"`
}

type terminalSession struct {
	id     string
	pty    pty.Pty
	cmd    *pty.Cmd
	cancel context.CancelFunc
	mu     sync.Mutex
	closed bool
}

type terminalManager struct {
	mu       sync.Mutex
	sessions map[string]*terminalSession
	send     func(typ string, payload any) error
	env      []string
}

func newTerminalManager(env []string, send func(typ string, payload any) error) *terminalManager {
	return &terminalManager{
		sessions: map[string]*terminalSession{},
		send:     send,
		env:      env,
	}
}

func (m *terminalManager) open(parent context.Context, terminalID, workdir string, cols, rows int) TerminalOpenResult {
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		terminalID = "term_" + time.Now().UTC().Format("20060102150405.000000000")
	}
	cleanRoot, _, _, err := resolveProjectPath(workdir, "")
	if err != nil {
		msg := err.Error()
		return TerminalOpenResult{TerminalID: terminalID, Workdir: workdir, Error: &msg}
	}
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}

	m.close(terminalID)

	pt, err := pty.New()
	if err != nil {
		msg := err.Error()
		return TerminalOpenResult{TerminalID: terminalID, Workdir: cleanRoot, Error: &msg}
	}
	_ = pt.Resize(cols, rows)

	shell, args := terminalShell()
	ctx, cancel := context.WithCancel(parent)
	cmd := pt.CommandContext(ctx, shell, args...)
	cmd.Dir = cleanRoot
	cmd.Env = mergedEnv(m.env)
	if err := cmd.Start(); err != nil {
		cancel()
		_ = pt.Close()
		msg := err.Error()
		return TerminalOpenResult{TerminalID: terminalID, Workdir: cleanRoot, Error: &msg}
	}

	session := &terminalSession{id: terminalID, pty: pt, cmd: cmd, cancel: cancel}
	m.mu.Lock()
	m.sessions[terminalID] = session
	m.mu.Unlock()

	go m.readLoop(session)
	go m.waitLoop(session)

	return TerminalOpenResult{TerminalID: terminalID, Workdir: cleanRoot}
}

func (m *terminalManager) input(terminalID, data string) {
	session := m.session(terminalID)
	if session == nil || data == "" {
		return
	}
	_, _ = session.pty.Write([]byte(data))
}

func (m *terminalManager) resize(terminalID string, cols, rows int) {
	session := m.session(terminalID)
	if session == nil || cols <= 0 || rows <= 0 {
		return
	}
	_ = session.pty.Resize(cols, rows)
}

func (m *terminalManager) close(terminalID string) {
	session := m.remove(terminalID)
	if session == nil {
		return
	}
	session.close()
}

func (m *terminalManager) closeAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.close(id)
	}
}

func (m *terminalManager) session(terminalID string) *terminalSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[terminalID]
}

func (m *terminalManager) remove(terminalID string) *terminalSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	session := m.sessions[terminalID]
	delete(m.sessions, terminalID)
	return session
}

func (m *terminalManager) readLoop(session *terminalSession) {
	buf := make([]byte, 8192)
	for {
		n, err := session.pty.Read(buf)
		if n > 0 {
			_ = m.send("project.terminal.output", TerminalOutput{
				TerminalID: session.id,
				Data:       string(buf[:n]),
			})
		}
		if err != nil {
			if !errors.Is(err, io.EOF) && !session.isClosed() {
				msg := err.Error()
				_ = m.send("project.terminal.exit", TerminalExit{TerminalID: session.id, ExitCode: -1, Error: &msg})
			}
			return
		}
	}
}

func (m *terminalManager) waitLoop(session *terminalSession) {
	err := session.cmd.Wait()
	exitCode := 0
	var message *string
	if session.cmd.ProcessState != nil {
		exitCode = session.cmd.ProcessState.ExitCode()
	}
	if err != nil && !session.isClosed() {
		msg := err.Error()
		message = &msg
		if exitCode == 0 {
			exitCode = -1
		}
	}
	_ = m.remove(session.id)
	session.close()
	_ = m.send("project.terminal.exit", TerminalExit{TerminalID: session.id, ExitCode: exitCode, Error: message})
}

func (s *terminalSession) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()
	s.cancel()
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	_ = s.pty.Close()
}

func (s *terminalSession) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func terminalShell() (string, []string) {
	if runtime.GOOS == "windows" {
		return windowsPowerShellExecutable(), []string{"-NoProfile", "-ExecutionPolicy", "Bypass"}
	}
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		shell = "/bin/sh"
	}
	return shell, nil
}

func windowsPowerShellExecutable() string {
	if path, err := exec.LookPath("powershell.exe"); err == nil && filepath.IsAbs(path) {
		return path
	}

	for _, root := range []string{os.Getenv("SystemRoot"), os.Getenv("WINDIR"), `C:\Windows`} {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		candidate := filepath.Join(root, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return "powershell.exe"
}
