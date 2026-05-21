package control

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const maxProjectFileUploadBytes int64 = 10 * 1024 * 1024

func (a *API) handleProjectFiles(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	project, server, ok := a.projectAndServerForRunnerRequest(w, r, projectID, "project_files")
	if !ok {
		return
	}
	requestPath := strings.TrimSpace(r.URL.Query().Get("path"))
	env, err := a.runners.Request(server.RunnerID, "project.files", ProjectFileListRequestPayload{
		Workdir: project.Workdir,
		Path:    requestPath,
	}, 10*time.Second)
	if err != nil {
		a.respondRunnerRequestError(w, server.RunnerID, "project file request", err)
		return
	}
	var listing ProjectFileListing
	if !decodeEnvelopePayload(env.Payload, &listing, a, "project.files.response") {
		writeError(w, http.StatusInternalServerError, "internal_error", "Invalid runner response.", nil)
		return
	}
	if listing.Error != nil {
		writeError(w, http.StatusBadRequest, "validation_error", *listing.Error, nil)
		return
	}
	writeJSON(w, http.StatusOK, listing)
}

func (a *API) handleProjectFileRoutes(w http.ResponseWriter, r *http.Request, projectID string, parts []string) {
	if len(parts) != 1 {
		writeError(w, http.StatusNotFound, "not_found", "Resource not found.", nil)
		return
	}
	switch parts[0] {
	case "content":
		a.handleProjectFileContent(w, r, projectID)
	case "upload":
		a.handleProjectFileUpload(w, r, projectID)
	case "actions":
		a.handleProjectFileAction(w, r, projectID)
	default:
		writeError(w, http.StatusNotFound, "not_found", "Resource not found.", nil)
	}
}

func (a *API) handleProjectFileUpload(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxProjectFileUploadBytes+1024*1024)
	if err := r.ParseMultipartForm(maxProjectFileUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid multipart upload.", nil)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	targetDir := strings.TrimSpace(r.FormValue("path"))
	createDirs := strings.EqualFold(strings.TrimSpace(r.FormValue("create_dirs")), "true")
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "File is required.", nil)
		return
	}
	defer file.Close()
	if header == nil || strings.TrimSpace(header.Filename) == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Filename is required.", nil)
		return
	}
	if header.Size > maxProjectFileUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "validation_error", "File is too large.", nil)
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxProjectFileUploadBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Unable to read uploaded file.", nil)
		return
	}
	if int64(len(data)) > maxProjectFileUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "validation_error", "File is too large.", nil)
		return
	}
	filename := path.Base(path.Clean(strings.ReplaceAll(header.Filename, "\\", "/")))
	if filename == "." || filename == "/" || strings.TrimSpace(filename) == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Filename is required.", nil)
		return
	}
	targetPath := filename
	if targetDir != "" && targetDir != "." {
		targetPath = strings.Trim(strings.ReplaceAll(targetDir, "\\", "/"), "/") + "/" + filename
	}

	project, server, ok := a.projectAndServerForRunnerRequest(w, r, projectID, "project_file_upload")
	if !ok {
		return
	}
	env, err := a.runners.Request(server.RunnerID, "project.file.upload", ProjectFileUploadRequestPayload{
		Workdir:       project.Workdir,
		Path:          targetPath,
		ContentBase64: base64.StdEncoding.EncodeToString(data),
		CreateDirs:    createDirs,
	}, 30*time.Second)
	if err != nil {
		a.respondRunnerRequestError(w, server.RunnerID, "project file upload request", err)
		return
	}
	var result ProjectFileActionResult
	if !decodeEnvelopePayload(env.Payload, &result, a, "project.file.upload.response") {
		writeError(w, http.StatusInternalServerError, "internal_error", "Invalid runner response.", nil)
		return
	}
	if result.Error != nil {
		writeError(w, http.StatusBadRequest, "validation_error", *result.Error, nil)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) handleProjectFileContent(w http.ResponseWriter, r *http.Request, projectID string) {
	switch r.Method {
	case http.MethodGet:
		project, server, ok := a.projectAndServerForRunnerRequest(w, r, projectID, "project_file_io")
		if !ok {
			return
		}
		requestPath := strings.TrimSpace(r.URL.Query().Get("path"))
		env, err := a.runners.Request(server.RunnerID, "project.file.read", ProjectFileReadRequestPayload{
			Workdir:  project.Workdir,
			Path:     requestPath,
			MaxBytes: 2 * 1024 * 1024,
		}, 10*time.Second)
		if err != nil {
			a.respondRunnerRequestError(w, server.RunnerID, "project file read request", err)
			return
		}
		var content ProjectFileContent
		if !decodeEnvelopePayload(env.Payload, &content, a, "project.file.read.response") {
			writeError(w, http.StatusInternalServerError, "internal_error", "Invalid runner response.", nil)
			return
		}
		if content.Error != nil {
			writeError(w, http.StatusBadRequest, "validation_error", *content.Error, nil)
			return
		}
		writeJSON(w, http.StatusOK, content)
	case http.MethodPut:
		var in struct {
			Path       string `json:"path"`
			Content    string `json:"content"`
			CreateDirs bool   `json:"create_dirs"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		if strings.TrimSpace(in.Path) == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "Path is required.", nil)
			return
		}
		project, server, ok := a.projectAndServerForRunnerRequest(w, r, projectID, "project_file_io")
		if !ok {
			return
		}
		env, err := a.runners.Request(server.RunnerID, "project.file.write", ProjectFileWriteRequestPayload{
			Workdir:    project.Workdir,
			Path:       in.Path,
			Content:    in.Content,
			CreateDirs: in.CreateDirs,
		}, 10*time.Second)
		if err != nil {
			a.respondRunnerRequestError(w, server.RunnerID, "project file write request", err)
			return
		}
		var result ProjectFileActionResult
		if !decodeEnvelopePayload(env.Payload, &result, a, "project.file.write.response") {
			writeError(w, http.StatusInternalServerError, "internal_error", "Invalid runner response.", nil)
			return
		}
		if result.Error != nil {
			writeError(w, http.StatusBadRequest, "validation_error", *result.Error, nil)
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		methodNotAllowed(w)
	}
}

func (a *API) handleProjectFileAction(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var in struct {
		Action     string `json:"action"`
		Path       string `json:"path"`
		TargetPath string `json:"target_path"`
		IsDir      bool   `json:"is_dir"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	action := strings.TrimSpace(in.Action)
	if action == "" || strings.TrimSpace(in.Path) == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Action and path are required.", nil)
		return
	}
	if action == "rename" && strings.TrimSpace(in.TargetPath) == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "target_path is required for rename.", nil)
		return
	}
	project, server, ok := a.projectAndServerForRunnerRequest(w, r, projectID, "project_file_io")
	if !ok {
		return
	}
	env, err := a.runners.Request(server.RunnerID, "project.file.action", ProjectFileActionRequestPayload{
		Workdir:    project.Workdir,
		Action:     action,
		Path:       in.Path,
		TargetPath: in.TargetPath,
		IsDir:      in.IsDir,
	}, 10*time.Second)
	if err != nil {
		a.respondRunnerRequestError(w, server.RunnerID, "project file action request", err)
		return
	}
	var result ProjectFileActionResult
	if !decodeEnvelopePayload(env.Payload, &result, a, "project.file.action.response") {
		writeError(w, http.StatusInternalServerError, "internal_error", "Invalid runner response.", nil)
		return
	}
	if result.Error != nil {
		writeError(w, http.StatusBadRequest, "validation_error", *result.Error, nil)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) handleProjectCommand(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var in struct {
		Command     string `json:"command"`
		TimeoutSecs int    `json:"timeout_secs"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Command) == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Command is required.", nil)
		return
	}
	project, server, ok := a.projectAndServerForRunnerRequest(w, r, projectID, "project_command")
	if !ok {
		return
	}
	env, err := a.runners.Request(server.RunnerID, "project.command", ProjectCommandRequestPayload{
		Workdir:     project.Workdir,
		Command:     in.Command,
		TimeoutSecs: in.TimeoutSecs,
	}, 130*time.Second)
	if err != nil {
		a.respondRunnerRequestError(w, server.RunnerID, "project command request", err)
		return
	}
	var result ProjectCommandResult
	if !decodeEnvelopePayload(env.Payload, &result, a, "project.command.response") {
		writeError(w, http.StatusInternalServerError, "internal_error", "Invalid runner response.", nil)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) handleProjectTerminal(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	conn, err := browserUpgrader.Upgrade(w, r, nil)
	if err != nil {
		a.logger.Warn("terminal websocket upgrade failed", "project_id", projectID, "error", err)
		return
	}
	defer conn.Close()

	project, err := a.store.GetProject(r.Context(), projectID)
	if err != nil {
		_ = conn.WriteJSON(map[string]any{"type": "error", "message": "Project was not found."})
		return
	}
	server, err := a.store.GetServer(r.Context(), project.ServerID)
	if err != nil {
		_ = conn.WriteJSON(map[string]any{"type": "error", "message": "Server was not found."})
		return
	}
	if !a.runners.Connected(server.RunnerID) {
		_ = conn.WriteJSON(map[string]any{"type": "error", "message": "No runner is connected for this server."})
		return
	}
	if !a.runners.Supports(server.RunnerID, "project_terminal") {
		_ = conn.WriteJSON(map[string]any{"type": "error", "message": "The connected runner does not support terminal sessions."})
		return
	}

	terminalID := randomID("term")
	sub := a.terminalHub.Subscribe(r.Context(), terminalID)
	writeMu := &sync.Mutex{}
	opened := false
	closed := false

	closeRunnerTerminal := func() {
		if closed {
			return
		}
		closed = true
		_ = a.runners.Send(server.RunnerID, "project.terminal.close", ProjectTerminalClosePayload{TerminalID: terminalID})
	}
	defer closeRunnerTerminal()

	for {
		var raw map[string]json.RawMessage
		if err := conn.ReadJSON(&raw); err != nil {
			return
		}
		typ := rawString(raw["type"])
		switch typ {
		case "open":
			if opened {
				continue
			}
			var payload struct {
				Cols int `json:"cols"`
				Rows int `json:"rows"`
			}
			_ = json.Unmarshal(raw["payload"], &payload)
			env, err := a.runners.Request(server.RunnerID, "project.terminal.open", ProjectTerminalOpenRequestPayload{
				TerminalID: terminalID,
				Workdir:    project.Workdir,
				Cols:       payload.Cols,
				Rows:       payload.Rows,
			}, 10*time.Second)
			if err != nil {
				writeTerminalJSON(writeMu, conn, map[string]any{"type": "error", "message": "Unable to open terminal."})
				a.respondRunnerRequestErrorWebsocket(server.RunnerID, "project terminal open request", err)
				return
			}
			var result ProjectTerminalOpenResponse
			if !decodeEnvelopePayload(env.Payload, &result, a, "project.terminal.open.response") {
				writeTerminalJSON(writeMu, conn, map[string]any{"type": "error", "message": "Invalid runner response."})
				return
			}
			if result.Error != nil {
				writeTerminalJSON(writeMu, conn, map[string]any{"type": "error", "message": *result.Error})
				return
			}
			opened = true
			if err := writeTerminalJSON(writeMu, conn, map[string]any{"type": "ready", "terminal_id": terminalID, "workdir": result.Workdir}); err != nil {
				return
			}
			go a.forwardTerminalEvents(r.Context(), writeMu, conn, sub, terminalID)
		case "input":
			if !opened {
				continue
			}
			var payload struct {
				Data string `json:"data"`
			}
			_ = json.Unmarshal(raw["payload"], &payload)
			_ = a.runners.Send(server.RunnerID, "project.terminal.input", ProjectTerminalInputPayload{
				TerminalID: terminalID,
				Data:       payload.Data,
			})
		case "resize":
			if !opened {
				continue
			}
			var payload struct {
				Cols int `json:"cols"`
				Rows int `json:"rows"`
			}
			_ = json.Unmarshal(raw["payload"], &payload)
			_ = a.runners.Send(server.RunnerID, "project.terminal.resize", ProjectTerminalResizePayload{
				TerminalID: terminalID,
				Cols:       payload.Cols,
				Rows:       payload.Rows,
			})
		case "close":
			return
		default:
			writeTerminalJSON(writeMu, conn, map[string]any{"type": "error", "message": "Unknown terminal message."})
		}
	}
}

func (a *API) forwardTerminalEvents(ctx context.Context, writeMu *sync.Mutex, conn *websocket.Conn, events <-chan RunnerEnvelope, terminalID string) {
	for {
		select {
		case <-ctx.Done():
			return
		case env, ok := <-events:
			if !ok {
				return
			}
			switch env.Type {
			case "project.terminal.output":
				var payload ProjectTerminalOutputPayload
				if !decodeEnvelopePayload(env.Payload, &payload, a, env.Type) || payload.TerminalID != terminalID {
					continue
				}
				if err := writeTerminalJSON(writeMu, conn, map[string]any{"type": "output", "data": payload.Data}); err != nil {
					return
				}
			case "project.terminal.exit":
				var payload ProjectTerminalExitPayload
				if !decodeEnvelopePayload(env.Payload, &payload, a, env.Type) || payload.TerminalID != terminalID {
					continue
				}
				if err := writeTerminalJSON(writeMu, conn, map[string]any{"type": "exit", "exit_code": payload.ExitCode, "error": payload.Error}); err != nil {
					return
				}
				return
			}
		}
	}
}

func writeTerminalJSON(mu *sync.Mutex, conn *websocket.Conn, value any) error {
	mu.Lock()
	defer mu.Unlock()
	return conn.WriteJSON(value)
}

func (a *API) projectAndServerForRunnerRequest(w http.ResponseWriter, r *http.Request, projectID, capability string) (Project, Server, bool) {
	project, err := a.store.GetProject(r.Context(), projectID)
	if err != nil {
		a.respond(w, http.StatusOK, nil, err)
		return Project{}, Server{}, false
	}
	server, err := a.store.GetServer(r.Context(), project.ServerID)
	if err != nil {
		a.respond(w, http.StatusOK, nil, err)
		return Project{}, Server{}, false
	}
	if !a.runners.Connected(server.RunnerID) {
		a.respond(w, http.StatusOK, nil, ErrRunnerUnavailable)
		return Project{}, Server{}, false
	}
	if !a.runners.Supports(server.RunnerID, capability) {
		a.respond(w, http.StatusOK, nil, ErrRunnerUnsupported)
		return Project{}, Server{}, false
	}
	return project, server, true
}

func (a *API) respondRunnerRequestError(w http.ResponseWriter, runnerID, operation string, err error) {
	if errors.Is(err, ErrRunnerRequestTimeout) {
		a.respond(w, http.StatusOK, nil, err)
		return
	}
	a.logger.Warn("runner request failed", "runner_id", runnerID, "operation", operation, "error", err)
	a.respond(w, http.StatusOK, nil, ErrRunnerUnavailable)
}

func (a *API) respondRunnerRequestErrorWebsocket(runnerID, operation string, err error) {
	a.logger.Warn("runner websocket request failed", "runner_id", runnerID, "operation", operation, "error", err)
}
