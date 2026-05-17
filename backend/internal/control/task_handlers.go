package control

import (
	"errors"
	"net/http"
)

func (a *API) handleTaskRoutes(w http.ResponseWriter, r *http.Request) {
	rest := trimPrefix(r.URL.Path, "/api/v1/tasks/")
	parts := splitPath(rest)
	if len(parts) == 1 {
		taskID := parts[0]
		switch r.Method {
		case http.MethodGet:
			item, err := a.store.GetTask(r.Context(), taskID)
			a.respond(w, http.StatusOK, item, err)
		case http.MethodPatch:
			var in PatchTaskInput
			if !decodeJSON(w, r, &in) {
				return
			}
			item, err := a.store.PatchTask(r.Context(), taskID, in)
			a.respond(w, http.StatusOK, item, err)
		default:
			methodNotAllowed(w)
		}
		return
	}
	if len(parts) == 2 && parts[1] == "mark-done" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var in MarkTaskDoneInput
		if !decodeJSON(w, r, &in) {
			return
		}
		item, err := a.store.MarkTaskDone(r.Context(), parts[0], in)
		if err == nil {
			a.notifyTaskDoneAsync(item, formatTaskMemorySummary(normalizeTaskMemoryInput(in)))
		}
		a.respond(w, http.StatusOK, item, err)
		return
	}
	if len(parts) == 2 && parts[1] == "archive" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		item, err := a.store.ArchiveTask(r.Context(), parts[0])
		a.respond(w, http.StatusOK, item, err)
		return
	}
	if len(parts) == 2 && parts[1] == "runs" {
		taskID := parts[0]
		switch r.Method {
		case http.MethodGet:
			items, err := a.store.ListRuns(r.Context(), taskID)
			a.respondList(w, items, err)
		case http.MethodPost:
			var in struct {
				Message         string   `json:"message"`
				Mode            string   `json:"mode"`
				CodexModel      string   `json:"codex_model"`
				ReasoningEffort string   `json:"codex_reasoning_effort"`
				ServiceTier     string   `json:"codex_service_tier"`
				RawCommand      bool     `json:"raw_command"`
				ContextItemIDs  []string `json:"context_item_ids"`
			}
			if !decodeJSON(w, r, &in) {
				return
			}
			result, err := a.store.CreateRun(r.Context(), CreateRunInput{
				TaskID:          taskID,
				Message:         in.Message,
				Mode:            in.Mode,
				CodexModel:      in.CodexModel,
				ReasoningEffort: in.ReasoningEffort,
				ServiceTier:     in.ServiceTier,
				RawCommand:      in.RawCommand,
				ContextItemIDs:  in.ContextItemIDs,
				IdempotencyKey:  r.Header.Get("Idempotency-Key"),
			})
			if err != nil {
				a.respond(w, http.StatusCreated, nil, err)
				return
			}
			if result.Assign != nil {
				if err := a.runners.SendAssign(result.Assign); err != nil && !errors.Is(err, ErrRunnerUnavailable) {
					a.logger.Warn("send run assignment failed", "run_id", result.Run.ID, "error", err)
				}
			}
			writeJSON(w, http.StatusCreated, map[string]any{"run": result.Run, "task": result.Task})
		default:
			methodNotAllowed(w)
		}
		return
	}
	if len(parts) == 3 && parts[1] == "runs" && parts[2] == "interrupt" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		taskID := parts[0]
		var in struct {
			Message         string   `json:"message"`
			Mode            string   `json:"mode"`
			CodexModel      string   `json:"codex_model"`
			ReasoningEffort string   `json:"codex_reasoning_effort"`
			ServiceTier     string   `json:"codex_service_tier"`
			RawCommand      bool     `json:"raw_command"`
			ContextItemIDs  []string `json:"context_item_ids"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		result, err := a.store.InterruptRun(r.Context(), CreateRunInput{
			TaskID:          taskID,
			Message:         in.Message,
			Mode:            in.Mode,
			CodexModel:      in.CodexModel,
			ReasoningEffort: in.ReasoningEffort,
			ServiceTier:     in.ServiceTier,
			RawCommand:      in.RawCommand,
			ContextItemIDs:  in.ContextItemIDs,
			IdempotencyKey:  r.Header.Get("Idempotency-Key"),
		}, "Interrupted by a newer user instruction.")
		if err != nil {
			a.respond(w, http.StatusCreated, nil, err)
			return
		}
		a.hub.Publish(result.CancelEvent)
		if result.Cancel != nil && result.CanceledRun.AssignedRunnerID != nil {
			if err := a.runners.SendCancel(*result.Cancel, *result.CanceledRun.AssignedRunnerID); err != nil && !errors.Is(err, ErrRunnerUnavailable) {
				a.logger.Warn("send interrupted run cancel failed", "run_id", result.CanceledRun.ID, "error", err)
			}
		}
		if result.Assign != nil {
			if err := a.runners.SendAssign(result.Assign); err != nil && !errors.Is(err, ErrRunnerUnavailable) {
				a.logger.Warn("send interrupt run assignment failed", "run_id", result.Run.ID, "error", err)
			}
		}
		writeJSON(w, http.StatusCreated, map[string]any{"run": result.Run, "task": result.Task})
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "Resource not found.", nil)
}
