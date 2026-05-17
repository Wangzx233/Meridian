package agent

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type CommandResult struct {
	Command    string  `json:"command"`
	Workdir    string  `json:"workdir"`
	ExitCode   int     `json:"exit_code"`
	Stdout     string  `json:"stdout"`
	Stderr     string  `json:"stderr"`
	DurationMS int64   `json:"duration_ms"`
	Error      *string `json:"error,omitempty"`
}

func runProjectCommand(ctx context.Context, workdir, command string, timeout time.Duration, env []string) CommandResult {
	command = strings.TrimSpace(command)
	result := CommandResult{Command: command, Workdir: workdir, ExitCode: -1}
	if command == "" {
		msg := "command is required"
		result.Error = &msg
		return result
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if timeout > 2*time.Minute {
		timeout = 2 * time.Minute
	}

	cleanRoot, target, _, err := resolveProjectPath(workdir, "")
	if err != nil {
		msg := err.Error()
		result.Error = &msg
		return result
	}
	result.Workdir = cleanRoot
	result.Workdir = target

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	shell, args := shellCommand(command)
	cmd := exec.CommandContext(runCtx, shell, args...)
	cmd.Dir = target
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err = cmd.Run()
	result.DurationMS = time.Since(start).Milliseconds()
	result.Stdout = truncateCommandOutput(stdout.String())
	result.Stderr = truncateCommandOutput(stderr.String())

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	if runCtx.Err() == context.DeadlineExceeded {
		msg := "command timed out"
		result.Error = &msg
		return result
	}
	if err != nil {
		msg := err.Error()
		result.Error = &msg
	}
	return result
}

func shellCommand(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		return windowsPowerShellExecutable(), []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", command}
	}
	return "/bin/sh", []string{"-lc", command}
}

func truncateCommandOutput(value string) string {
	const max = 64 * 1024
	if len(value) <= max {
		return value
	}
	return value[:max] + "\n[output truncated]"
}
