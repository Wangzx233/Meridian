package control

import (
	"net/http"
	"strings"
)

func (a *API) handleWorkbenchNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	pendingOnly, ok := parsePendingNotificationFilter(w, r.URL.Query().Get("pending"))
	if !ok {
		return
	}
	items, err := a.store.ListWorkbenchNotifications(r.Context(), pendingOnly)
	a.respondList(w, items, err)
}

func (a *API) handleWorkbenchNotificationByID(w http.ResponseWriter, r *http.Request) {
	rest := trimPrefix(r.URL.Path, "/api/v1/notifications/")
	parts := splitPath(rest)
	if len(parts) != 2 || parts[1] != "ack" {
		writeError(w, http.StatusNotFound, "not_found", "Resource not found.", nil)
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	item, err := a.store.AcknowledgeWorkbenchNotification(r.Context(), parts[0])
	a.respond(w, http.StatusOK, item, err)
}

func parsePendingNotificationFilter(w http.ResponseWriter, raw string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "true", "1", "yes":
		return true, true
	case "false", "0", "no":
		return false, true
	default:
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid pending filter.", nil)
		return false, false
	}
}
