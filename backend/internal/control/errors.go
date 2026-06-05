package control

import (
	"encoding/json"
	"errors"
	"net/http"
)

var (
	ErrNotFound             = errors.New("not_found")
	ErrValidation           = errors.New("validation_error")
	ErrInvalidState         = errors.New("invalid_state")
	ErrActiveRunExists      = errors.New("active_run_exists")
	ErrActiveRunMissing     = errors.New("active_run_missing")
	ErrMissingCodexSession  = errors.New("missing_codex_session")
	ErrUnauthorized         = errors.New("unauthorized")
	ErrRunnerUnavailable    = errors.New("runner_unavailable")
	ErrRunnerUnsupported    = errors.New("runner_unsupported")
	ErrRunnerRequestTimeout = errors.New("runner_request_timeout")
)

type validationMessageError struct {
	message string
}

func (e validationMessageError) Error() string {
	return e.message
}

func (e validationMessageError) Is(target error) bool {
	return target == ErrValidation
}

func validationError(message string) error {
	if message == "" {
		return ErrValidation
	}
	return validationMessageError{message: message}
}

type APIError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	writeJSON(w, status, map[string]any{
		"error": APIError{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

func statusForError(err error) (int, string, string) {
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound, "not_found", "Resource not found."
	case errors.Is(err, ErrValidation):
		var validation validationMessageError
		if errors.As(err, &validation) && validation.message != "" {
			return http.StatusBadRequest, "validation_error", validation.message
		}
		return http.StatusBadRequest, "validation_error", "Request validation failed."
	case errors.Is(err, ErrInvalidState):
		return http.StatusConflict, "invalid_state", "Resource is not in a valid state for this operation."
	case errors.Is(err, ErrActiveRunExists):
		return http.StatusConflict, "active_run_exists", "Task already has an active run."
	case errors.Is(err, ErrActiveRunMissing):
		return http.StatusConflict, "active_run_missing", "Task does not have an active run to interrupt."
	case errors.Is(err, ErrMissingCodexSession):
		return http.StatusConflict, "missing_codex_session", "Task does not have a Codex session id."
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized, "unauthorized", "Login required."
	case errors.Is(err, ErrRunnerUnavailable):
		return http.StatusConflict, "runner_unavailable", "No runner is connected for this server."
	case errors.Is(err, ErrRunnerUnsupported):
		return http.StatusConflict, "runner_unsupported", "The connected runner does not support this operation. Update the runner and try again."
	case errors.Is(err, ErrRunnerRequestTimeout):
		return http.StatusGatewayTimeout, "runner_request_timeout", "The runner did not respond in time. Check that the runner is up to date and healthy."
	default:
		return http.StatusInternalServerError, "internal_error", "Internal server error."
	}
}
