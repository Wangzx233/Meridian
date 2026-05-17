package control

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (a *API) handleEmailNotificationConfigs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := a.store.ListEmailNotificationConfigs(r.Context(), false)
		a.respondList(w, items, err)
	case http.MethodPost:
		var in CreateEmailNotificationConfigInput
		if !decodeJSON(w, r, &in) {
			return
		}
		item, err := a.store.CreateEmailNotificationConfig(r.Context(), in)
		a.respond(w, http.StatusCreated, item, err)
	default:
		methodNotAllowed(w)
	}
}

func (a *API) handleEmailNotificationConfigByID(w http.ResponseWriter, r *http.Request) {
	id := trimPrefix(r.URL.Path, "/api/v1/settings/email-notifications/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "not_found", "Resource not found.", nil)
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, err := a.store.GetEmailNotificationConfig(r.Context(), id, false)
		a.respond(w, http.StatusOK, item, err)
	case http.MethodPatch:
		var raw map[string]json.RawMessage
		if !decodeJSON(w, r, &raw) {
			return
		}
		in, ok := parsePatchEmailNotificationConfig(w, raw)
		if !ok {
			return
		}
		item, err := a.store.PatchEmailNotificationConfig(r.Context(), id, in)
		a.respond(w, http.StatusOK, item, err)
	case http.MethodDelete:
		err := a.store.DeleteEmailNotificationConfig(r.Context(), id)
		if err != nil {
			a.respond(w, http.StatusOK, nil, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w)
	}
}

func parsePatchEmailNotificationConfig(w http.ResponseWriter, raw map[string]json.RawMessage) (PatchEmailNotificationConfigInput, bool) {
	in := PatchEmailNotificationConfigInput{}
	if v, ok := raw["name"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "Invalid name.", nil)
			return in, false
		}
		in.Name = &s
	}
	if v, ok := raw["enabled"]; ok {
		var b bool
		if err := json.Unmarshal(v, &b); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "Invalid enabled.", nil)
			return in, false
		}
		in.Enabled = &b
	}
	if v, ok := raw["smtp_host"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "Invalid smtp_host.", nil)
			return in, false
		}
		in.SMTPHost = &s
	}
	if v, ok := raw["smtp_port"]; ok {
		var n int
		if err := json.Unmarshal(v, &n); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "Invalid smtp_port.", nil)
			return in, false
		}
		in.SMTPPort = &n
	}
	if v, ok := raw["smtp_username"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "Invalid smtp_username.", nil)
			return in, false
		}
		in.SMTPUsername = &s
	}
	if v, ok := raw["smtp_password"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "Invalid smtp_password.", nil)
			return in, false
		}
		in.SMTPPassword = &s
	}
	if v, ok := raw["from_address"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "Invalid from_address.", nil)
			return in, false
		}
		in.FromAddress = &s
	}
	if v, ok := raw["to_addresses"]; ok {
		if err := json.Unmarshal(v, &in.ToAddresses); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "Invalid to_addresses.", nil)
			return in, false
		}
		in.ToAddressesSet = true
	}
	if v, ok := raw["tls_mode"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "Invalid tls_mode.", nil)
			return in, false
		}
		in.TLSMode = &s
	}
	if v, ok := raw["subject_prefix"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "Invalid subject_prefix.", nil)
			return in, false
		}
		in.SubjectPrefix = &s
	}
	return in, true
}
