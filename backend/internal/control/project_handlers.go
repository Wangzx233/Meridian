package control

import (
	"net/http"
)

func (a *API) handleProjects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := a.store.ListProjects(r.Context(), r.URL.Query().Get("server_id"))
		a.respondList(w, items, err)
	case http.MethodPost:
		var in CreateProjectInput
		if !decodeJSON(w, r, &in) {
			return
		}
		item, err := a.store.CreateProject(r.Context(), in)
		a.respond(w, http.StatusCreated, item, err)
	default:
		methodNotAllowed(w)
	}
}

func (a *API) handleProjectRoutes(w http.ResponseWriter, r *http.Request) {
	rest := trimPrefix(r.URL.Path, "/api/v1/projects/")
	parts := splitPath(rest)
	if len(parts) == 1 {
		projectID := parts[0]
		switch r.Method {
		case http.MethodGet:
			item, err := a.store.GetProject(r.Context(), projectID)
			a.respond(w, http.StatusOK, item, err)
		case http.MethodPatch:
			var in PatchProjectInput
			if !decodeJSON(w, r, &in) {
				return
			}
			item, err := a.store.PatchProject(r.Context(), projectID, in)
			a.respond(w, http.StatusOK, item, err)
		default:
			methodNotAllowed(w)
		}
		return
	}
	if len(parts) == 2 && parts[1] == "files" {
		a.handleProjectFiles(w, r, parts[0])
		return
	}
	if len(parts) >= 2 && parts[1] == "files" {
		a.handleProjectFileRoutes(w, r, parts[0], parts[2:])
		return
	}
	if len(parts) == 2 && parts[1] == "command" {
		a.handleProjectCommand(w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "terminal" {
		a.handleProjectTerminal(w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "tasks" {
		projectID := parts[0]
		switch r.Method {
		case http.MethodGet:
			statuses := splitCSV(r.URL.Query().Get("status"))
			items, err := a.store.ListTasks(r.Context(), projectID, statuses)
			a.respondList(w, items, err)
		case http.MethodPost:
			var in CreateTaskInput
			if !decodeJSON(w, r, &in) {
				return
			}
			item, err := a.store.CreateTask(r.Context(), projectID, in)
			a.respond(w, http.StatusCreated, item, err)
		default:
			methodNotAllowed(w)
		}
		return
	}
	if len(parts) == 2 && parts[1] == "context-items" {
		projectID := parts[0]
		switch r.Method {
		case http.MethodGet:
			items, err := a.store.ListContextItems(r.Context(), projectID, r.URL.Query().Get("task_id"))
			a.respondList(w, items, err)
		case http.MethodPost:
			var in CreateContextInput
			if !decodeJSON(w, r, &in) {
				return
			}
			item, err := a.store.CreateContextItem(r.Context(), projectID, in)
			a.respond(w, http.StatusCreated, item, err)
		default:
			methodNotAllowed(w)
		}
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "Resource not found.", nil)
}
