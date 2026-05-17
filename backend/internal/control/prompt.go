package control

import (
	"strings"
)

type contextSnapshot struct {
	ID      string
	Type    string
	Title   string
	Content string
}

func buildPrompt(mode string, task Task, message string, items []contextSnapshot) string {
	var b strings.Builder
	b.WriteString("Current task:\n")
	b.WriteString(task.Title)
	b.WriteString("\n\nDescription:\n")
	description := strings.TrimSpace(task.Description)
	if description == "" {
		description = "(no description provided)"
	}
	b.WriteString(description)
	b.WriteString("\n\n")

	if mode == RunModeResume {
		b.WriteString("Continue the current Codex session for this task.\n\n")
	}

	b.WriteString("User instruction for this turn:\n")
	b.WriteString(message)
	b.WriteString("\n\n")

	if mode == RunModeResume {
		b.WriteString("Additional context selected for this turn:\n")
	} else {
		b.WriteString("Selected context:\n")
	}

	if len(items) == 0 {
		b.WriteString("(none)\n\n")
	} else {
		for i, item := range items {
			b.WriteString("Context item ")
			b.WriteString(intString(i + 1))
			b.WriteString(" [")
			b.WriteString(item.Type)
			b.WriteString("]: ")
			b.WriteString(item.Title)
			b.WriteString("\n")
			b.WriteString(item.Content)
			b.WriteString("\n\n")
		}
	}

	b.WriteString("Instructions:\n")
	if mode == RunModeResume {
		b.WriteString("- Continue from the existing Codex session.\n")
		b.WriteString("- Do not repeat already completed work unless needed.\n")
		b.WriteString("- Current repository code is authoritative.\n")
		b.WriteString("- Use the current task title and description above as the scope for this turn.\n")
	} else {
		b.WriteString("- First inspect the current repository before deciding.\n")
		b.WriteString("- Historical context is background only.\n")
		b.WriteString("- Current repository code is authoritative if it conflicts with context.\n")
		b.WriteString("- Complete this turn and explain changes, verification, and next steps.\n")
	}
	return b.String()
}

func buildRunPrompt(mode string, task Task, message string, items []contextSnapshot, rawCommand bool) (string, error) {
	if !rawCommand {
		return buildPrompt(mode, task, message, items), nil
	}
	command := strings.TrimSpace(message)
	if command != "/compact" || mode != RunModeResume || len(items) > 0 {
		return "", ErrValidation
	}
	return command, nil
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

func buildArgv(codexPath, workdir, mode string, sessionID, model, reasoningEffort, serviceTier *string) []string {
	if codexPath == "" {
		codexPath = "codex"
	}
	args := []string{codexPath, "--cd", workdir}
	if model != nil && strings.TrimSpace(*model) != "" {
		args = append(args, "--model", strings.TrimSpace(*model))
	}
	if reasoningEffort != nil && strings.TrimSpace(*reasoningEffort) != "" {
		args = append(args, "--config", `model_reasoning_effort="`+strings.TrimSpace(*reasoningEffort)+`"`)
	}
	if serviceTier != nil && strings.TrimSpace(*serviceTier) != "" {
		args = append(args, "--config", `service_tier="`+strings.TrimSpace(*serviceTier)+`"`)
	}
	if mode == RunModeResume {
		execArgs := []string{"exec", "resume"}
		if codexBypassApprovalsAndSandboxEnabled() {
			execArgs = append(execArgs, "--dangerously-bypass-approvals-and-sandbox")
		}
		execArgs = append(execArgs, "--skip-git-repo-check", "--json", valueOrEmpty(sessionID), "-")
		return append(args, execArgs...)
	}
	execArgs := []string{"exec"}
	if codexBypassApprovalsAndSandboxEnabled() {
		execArgs = append(execArgs, "--dangerously-bypass-approvals-and-sandbox")
	}
	execArgs = append(execArgs, "--skip-git-repo-check", "--json", "-")
	return append(args, execArgs...)
}

func codexBypassApprovalsAndSandboxEnabled() bool {
	return parseBoolEnv("CODEX_BYPASS_APPROVALS_AND_SANDBOX", true)
}

func codexBypassApprovalsAndSandboxValue() string {
	if codexBypassApprovalsAndSandboxEnabled() {
		return "true"
	}
	return "false"
}

func valueOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
