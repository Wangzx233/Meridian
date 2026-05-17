package control

import (
	"net/http"
	"strings"
)

var BuildCommit = "dev"

func buildCommit() string {
	commit := strings.TrimSpace(BuildCommit)
	if commit == "" {
		return "dev"
	}
	return commit
}

func (a *API) handleBuildInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"commit": buildCommit()})
}
