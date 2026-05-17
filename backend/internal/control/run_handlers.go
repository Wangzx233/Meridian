package control

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

func (a *API) handleRunRoutes(w http.ResponseWriter, r *http.Request) {
	rest := trimPrefix(r.URL.Path, "/api/v1/runs/")
	parts := splitPath(rest)
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		item, err := a.store.GetRun(r.Context(), parts[0])
		a.respond(w, http.StatusOK, item, err)
		return
	}
	if len(parts) == 2 && parts[1] == "events" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		afterSeq := parseSeq(r.URL.Query().Get("after_seq"))
		items, err := a.store.ListEvents(r.Context(), parts[0], afterSeq)
		a.respondList(w, items, err)
		return
	}
	if len(parts) == 3 && parts[1] == "events" && parts[2] == "stream" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		a.streamEvents(w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "cancel" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var in struct {
			Reason string `json:"reason"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		run, cancel, event, err := a.store.CancelRun(r.Context(), parts[0], in.Reason)
		if err != nil {
			a.respond(w, http.StatusOK, nil, err)
			return
		}
		a.hub.Publish(event)
		if cancel != nil && run.AssignedRunnerID != nil {
			if err := a.runners.SendCancel(*cancel, *run.AssignedRunnerID); err != nil && !errors.Is(err, ErrRunnerUnavailable) {
				a.logger.Warn("send run cancel failed", "run_id", run.ID, "error", err)
			}
		}
		writeJSON(w, http.StatusAccepted, run)
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "Resource not found.", nil)
}

func (a *API) streamEvents(w http.ResponseWriter, r *http.Request, runID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal_error", "Streaming unsupported.", nil)
		return
	}
	afterSeq := parseSeq(r.URL.Query().Get("after_seq"))
	if hdr := r.Header.Get("Last-Event-ID"); hdr != "" {
		if v := parseSeq(hdr); v > afterSeq {
			afterSeq = v
		}
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	send := func(event RunEvent) bool {
		data, err := json.Marshal(event)
		if err != nil {
			return true
		}
		if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.Seq, event.EventType, data); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	sub := a.hub.Subscribe(r.Context(), runID)

	events, err := a.store.ListEvents(r.Context(), runID, afterSeq)
	if err != nil {
		status, code, msg := statusForError(err)
		writeError(w, status, code, msg, nil)
		return
	}
	for _, event := range events {
		if event.Seq <= afterSeq {
			continue
		}
		if !send(event) {
			return
		}
		afterSeq = event.Seq
		if event.EventType == EventRunFinal {
			return
		}
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-sub:
			if !ok {
				return
			}
			if event.Seq <= afterSeq {
				continue
			}
			if !send(event) {
				return
			}
			afterSeq = event.Seq
			if event.EventType == EventRunFinal {
				return
			}
		case <-ticker.C:
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
