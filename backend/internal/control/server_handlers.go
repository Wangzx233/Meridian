package control

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

func (a *API) handleServers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := a.store.ListServers(r.Context())
		a.hydrateServers(items)
		a.respondList(w, items, err)
	case http.MethodPost:
		var in CreateServerInput
		if !decodeJSON(w, r, &in) {
			return
		}
		item, err := a.store.CreateServer(r.Context(), in)
		a.hydrateServer(&item)
		a.respond(w, http.StatusCreated, item, err)
	default:
		methodNotAllowed(w)
	}
}

func (a *API) handleServerByID(w http.ResponseWriter, r *http.Request) {
	rest := trimPrefix(r.URL.Path, "/api/v1/servers/")
	parts := splitPath(rest)
	if len(parts) == 2 && parts[1] == "directories" {
		a.handleServerDirectories(w, r, parts[0])
		return
	}
	if len(parts) != 1 {
		writeError(w, http.StatusNotFound, "not_found", "Resource not found.", nil)
		return
	}
	id := parts[0]
	switch r.Method {
	case http.MethodGet:
		item, err := a.store.GetServer(r.Context(), id)
		a.hydrateServer(&item)
		a.respond(w, http.StatusOK, item, err)
	case http.MethodPatch:
		var raw map[string]json.RawMessage
		if !decodeJSON(w, r, &raw) {
			return
		}
		in, ok := decodePatchServerInput(w, raw)
		if !ok {
			return
		}
		item, err := a.store.PatchServer(r.Context(), id, in)
		a.hydrateServer(&item)
		a.respond(w, http.StatusOK, item, err)
	case http.MethodDelete:
		err := a.store.DeleteServer(r.Context(), id)
		if err != nil {
			a.respond(w, http.StatusOK, nil, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w)
	}
}

func decodePatchServerInput(w http.ResponseWriter, raw map[string]json.RawMessage) (PatchServerInput, bool) {
	var in PatchServerInput
	for key, value := range raw {
		switch key {
		case "name":
			s, ok := decodePatchString(w, value, "name", false)
			if !ok {
				return PatchServerInput{}, false
			}
			in.Name = s
		case "alias":
			s, ok := decodePatchString(w, value, "alias", true)
			if !ok {
				return PatchServerInput{}, false
			}
			in.Alias = s
		case "runner_id":
			s, ok := decodePatchString(w, value, "runner_id", false)
			if !ok {
				return PatchServerInput{}, false
			}
			in.RunnerID = s
		case "status":
			s, ok := decodePatchString(w, value, "status", false)
			if !ok {
				return PatchServerInput{}, false
			}
			in.Status = s
		default:
			writeError(w, http.StatusBadRequest, "validation_error", "Invalid JSON request body.", nil)
			return PatchServerInput{}, false
		}
	}
	return in, true
}

func decodePatchString(w http.ResponseWriter, raw json.RawMessage, field string, nullClears bool) (*string, bool) {
	if string(raw) == "null" {
		if !nullClears {
			return nil, true
		}
		empty := ""
		return &empty, true
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid "+field+".", nil)
		return nil, false
	}
	return &value, true
}

func (a *API) handleServerDirectories(w http.ResponseWriter, r *http.Request, serverID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	server, err := a.store.GetServer(r.Context(), serverID)
	if err != nil {
		a.respond(w, http.StatusOK, nil, err)
		return
	}
	requestPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if !a.runners.Connected(server.RunnerID) {
		a.respond(w, http.StatusOK, nil, ErrRunnerUnavailable)
		return
	}
	if !a.runners.Supports(server.RunnerID, "fs_list") {
		a.respond(w, http.StatusOK, nil, ErrRunnerUnsupported)
		return
	}
	env, err := a.runners.Request(server.RunnerID, "fs.list", DirectoryListRequestPayload{Path: requestPath}, 10*time.Second)
	if err != nil {
		if errors.Is(err, ErrRunnerRequestTimeout) {
			a.respond(w, http.StatusOK, nil, err)
			return
		}
		a.logger.Warn("runner directory request failed", "runner_id", server.RunnerID, "error", err)
		a.respond(w, http.StatusOK, nil, ErrRunnerUnavailable)
		return
	}
	var listing DirectoryListing
	if !decodeEnvelopePayload(env.Payload, &listing, a, "fs.list.response") {
		writeError(w, http.StatusInternalServerError, "internal_error", "Invalid runner response.", nil)
		return
	}
	if listing.Error != nil {
		writeError(w, http.StatusBadRequest, "validation_error", *listing.Error, nil)
		return
	}
	writeJSON(w, http.StatusOK, listing)
}
