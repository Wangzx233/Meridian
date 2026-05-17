package control

import "strings"

func resolveCreateRunMode(requested string, task Task) (string, error) {
	if requested == "" {
		requested = "auto"
	}
	if requested != "auto" && requested != RunModeNew && requested != RunModeResume {
		return "", ErrValidation
	}
	if requested == "auto" {
		if task.CodexSessionID != nil && *task.CodexSessionID != "" {
			return RunModeResume, nil
		}
		return RunModeNew, nil
	}
	if requested == RunModeResume && (task.CodexSessionID == nil || *task.CodexSessionID == "") {
		return "", ErrMissingCodexSession
	}
	return requested, nil
}

func normalizeCodexRunOptions(model, reasoningEffort, serviceTier string) (*string, *string, *string, error) {
	model = strings.TrimSpace(model)
	reasoningEffort = strings.TrimSpace(reasoningEffort)
	serviceTier = strings.TrimSpace(serviceTier)
	if model != "" {
		if len(model) > 120 || strings.ContainsAny(model, "\r\n\t") {
			return nil, nil, nil, ErrValidation
		}
	}
	if reasoningEffort != "" {
		switch reasoningEffort {
		case "low", "medium", "high", "xhigh":
		default:
			return nil, nil, nil, ErrValidation
		}
	}
	if serviceTier != "" {
		switch serviceTier {
		case "fast":
		default:
			return nil, nil, nil, ErrValidation
		}
	}
	return optionalString(model), optionalString(reasoningEffort), optionalString(serviceTier), nil
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func validateCreateRunState(task Task) error {
	if task.Status == TaskStatusDone || task.Status == TaskStatusArchived {
		return ErrInvalidState
	}
	if task.ActiveRunID != nil && *task.ActiveRunID != "" {
		return ErrActiveRunExists
	}
	return nil
}

func isActiveRunStatus(status string) bool {
	return status == RunStatusQueued || status == RunStatusRunning
}

func isTerminalRunStatus(status string) bool {
	return status == RunStatusSucceeded || status == RunStatusFailed || status == RunStatusCanceled
}

func taskStatusAfterTerminalRun(current string) string {
	if current == TaskStatusRunning {
		return TaskStatusWaitingUser
	}
	return current
}
