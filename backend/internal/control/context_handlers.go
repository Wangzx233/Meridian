package control

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (a *API) handleContextItemByID(w http.ResponseWriter, r *http.Request) {
	id := trimPrefix(r.URL.Path, "/api/v1/context-items/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "not_found", "Resource not found.", nil)
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, err := a.store.GetContextItem(r.Context(), id)
		a.respond(w, http.StatusOK, item, err)
	case http.MethodPatch:
		var raw map[string]json.RawMessage
		if !decodeJSON(w, r, &raw) {
			return
		}
		in := PatchContextInput{}
		if v, ok := raw["scope"]; ok {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", "Invalid scope.", nil)
				return
			}
			in.Scope = &s
		}
		if v, ok := raw["server_id"]; ok {
			if string(v) == "null" {
				in.ServerID = nil
			} else {
				var s string
				if err := json.Unmarshal(v, &s); err != nil {
					writeError(w, http.StatusBadRequest, "validation_error", "Invalid server_id.", nil)
					return
				}
				in.ServerID = &s
			}
		}
		if v, ok := raw["project_id"]; ok {
			if string(v) == "null" {
				in.ProjectID = nil
			} else {
				var s string
				if err := json.Unmarshal(v, &s); err != nil {
					writeError(w, http.StatusBadRequest, "validation_error", "Invalid project_id.", nil)
					return
				}
				in.ProjectID = &s
			}
		}
		if v, ok := raw["task_id"]; ok {
			if string(v) == "null" {
				in.TaskID = nil
			} else {
				var s string
				if err := json.Unmarshal(v, &s); err != nil {
					writeError(w, http.StatusBadRequest, "validation_error", "Invalid task_id.", nil)
					return
				}
				in.TaskID = &s
			}
		}
		if v, ok := raw["type"]; ok {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", "Invalid type.", nil)
				return
			}
			in.Type = &s
		}
		if v, ok := raw["title"]; ok {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", "Invalid title.", nil)
				return
			}
			in.Title = &s
		}
		if v, ok := raw["content"]; ok {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", "Invalid content.", nil)
				return
			}
			in.Content = &s
		}
		if v, ok := raw["tags"]; ok {
			if err := json.Unmarshal(v, &in.Tags); err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", "Invalid tags.", nil)
				return
			}
			in.TagsSet = true
		}
		item, err := a.store.PatchContextItem(r.Context(), id, in)
		a.respond(w, http.StatusOK, item, err)
	case http.MethodDelete:
		err := a.store.DeleteContextItem(r.Context(), id)
		if err != nil {
			a.respond(w, http.StatusOK, nil, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w)
	}
}
