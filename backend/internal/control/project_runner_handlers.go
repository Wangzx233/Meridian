package control

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	maxProjectFileLegacyUploadBytes int64 = 5 * 1024 * 1024
	maxProjectFileUploadChunkBytes  int64 = 8 * 1024 * 1024
)

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
	if len(parts) == 0 {
		writeError(w, http.StatusNotFound, "not_found", "Resource not found.", nil)
		return
	}
	switch parts[0] {
	case "content":
		if len(parts) != 1 {
			writeError(w, http.StatusNotFound, "not_found", "Resource not found.", nil)
			return
		}
		a.handleProjectFileContent(w, r, projectID)
	case "upload":
		if len(parts) == 1 {
			a.handleProjectFileUpload(w, r, projectID)
			return
		}
		if len(parts) == 2 && parts[1] == "tus" {
			a.handleProjectFileTusCollection(w, r, projectID)
			return
		}
		if len(parts) == 3 && parts[1] == "tus" {
			a.handleProjectFileTusResource(w, r, projectID, parts[2])
			return
		}
		writeError(w, http.StatusNotFound, "not_found", "Resource not found.", nil)
	case "actions":
		if len(parts) != 1 {
			writeError(w, http.StatusNotFound, "not_found", "Resource not found.", nil)
			return
		}
		a.handleProjectFileAction(w, r, projectID)
	default:
		writeError(w, http.StatusNotFound, "not_found", "Resource not found.", nil)
	}
}

func (a *API) handleProjectFileUpload(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method == http.MethodGet {
		a.handleProjectFileUploadStatus(w, r, projectID)
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.Contains(contentType, "application/json") {
		a.handleProjectFileUploadJSON(w, r, projectID)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxProjectFileLegacyUploadBytes+1024*1024)
	if err := r.ParseMultipartForm(maxProjectFileLegacyUploadBytes); err != nil {
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
	if header.Size > maxProjectFileLegacyUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "validation_error", "File is too large.", nil)
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxProjectFileLegacyUploadBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Unable to read uploaded file.", nil)
		return
	}
	if int64(len(data)) > maxProjectFileLegacyUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "validation_error", "File is too large.", nil)
		return
	}
	filename := uploadFilename(header.Filename)
	if filename == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Filename is required.", nil)
		return
	}
	a.forwardProjectFileUpload(w, r, projectID, uploadTargetPath(targetDir, filename), data, createDirs)
}

func (a *API) handleProjectFileTusCollection(w http.ResponseWriter, r *http.Request, projectID string) {
	setTusHeaders(w)
	switch r.Method {
	case http.MethodOptions:
		w.Header().Set("Tus-Version", "1.0.0")
		w.Header().Set("Tus-Extension", "creation")
		w.Header().Set("Tus-Max-Size", strconv.FormatInt(maxProjectFileUploadChunkBytes, 10))
		w.WriteHeader(http.StatusNoContent)
	case http.MethodPost:
		_, _, ok := a.projectAndServerForRunnerRequest(w, r, projectID, "project_file_upload_chunked")
		if !ok {
			return
		}
		totalSize, ok := parseNonNegativeInt64(r.Header.Get("Upload-Length"))
		if !ok {
			writeError(w, http.StatusBadRequest, "validation_error", "Upload-Length must be a non-negative integer.", nil)
			return
		}
		metadata, err := parseTusMetadata(r.Header.Get("Upload-Metadata"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "Invalid Upload-Metadata header.", nil)
			return
		}
		filename := uploadFilename(metadata["filename"])
		if filename == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "filename metadata is required.", nil)
			return
		}
		uploadID := strings.TrimSpace(metadata["upload_id"])
		if uploadID == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "upload_id metadata is required.", nil)
			return
		}
		createDirs := !strings.EqualFold(strings.TrimSpace(metadata["create_dirs"]), "false")
		token := projectFileTusToken{
			ProjectID:  projectID,
			Path:       uploadTargetPath(metadata["path"], filename),
			UploadID:   uploadID,
			TotalSize:  totalSize,
			CreateDirs: createDirs,
		}
		if totalSize == 0 {
			result, ok := a.requestProjectFileUploadChunk(w, r, projectID, token.Path, token.UploadID, 0, 0, nil, token.CreateDirs, true)
			if !ok {
				return
			}
			w.Header().Set("Upload-Info", encodeTusUploadInfo(result))
		}
		encoded, err := encodeProjectFileTusToken(token)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Unable to create upload.", nil)
			return
		}
		w.Header().Set("Location", "/api/v1/projects/"+url.PathEscape(projectID)+"/files/upload/tus/"+encoded)
		w.Header().Set("Upload-Offset", "0")
		w.WriteHeader(http.StatusCreated)
	default:
		methodNotAllowed(w)
	}
}

func (a *API) handleProjectFileTusResource(w http.ResponseWriter, r *http.Request, projectID, rawToken string) {
	setTusHeaders(w)
	token, err := decodeProjectFileTusToken(rawToken)
	if err != nil || token.ProjectID != projectID {
		writeError(w, http.StatusNotFound, "not_found", "Upload was not found.", nil)
		return
	}
	switch r.Method {
	case http.MethodHead:
		result, ok := a.requestProjectFileUploadStatus(w, r, projectID, token.Path, token.UploadID, token.TotalSize)
		if !ok {
			return
		}
		offset := result.ResumeOffset
		if result.Complete {
			offset = token.TotalSize
		}
		w.Header().Set("Upload-Offset", strconv.FormatInt(offset, 10))
		w.Header().Set("Upload-Length", strconv.FormatInt(token.TotalSize, 10))
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	case http.MethodPatch:
		contentType := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
		if contentType != "application/offset+octet-stream" {
			writeError(w, http.StatusUnsupportedMediaType, "validation_error", "PATCH uploads must use application/offset+octet-stream.", nil)
			return
		}
		offset, ok := parseNonNegativeInt64(r.Header.Get("Upload-Offset"))
		if !ok {
			writeError(w, http.StatusBadRequest, "validation_error", "Upload-Offset must be a non-negative integer.", nil)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxProjectFileUploadChunkBytes+1)
		data, err := io.ReadAll(io.LimitReader(r.Body, maxProjectFileUploadChunkBytes+1))
		if closeErr := r.Body.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "Unable to read upload chunk.", nil)
			return
		}
		if int64(len(data)) > maxProjectFileUploadChunkBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "validation_error", "Upload chunk is too large.", nil)
			return
		}
		final := offset+int64(len(data)) == token.TotalSize
		result, ok := a.requestProjectFileUploadChunk(w, r, projectID, token.Path, token.UploadID, offset, token.TotalSize, data, token.CreateDirs, final)
		if !ok {
			return
		}
		nextOffset := result.ResumeOffset
		if nextOffset == 0 && result.UploadedBytes > 0 {
			nextOffset = result.UploadedBytes
		}
		w.Header().Set("Upload-Offset", strconv.FormatInt(nextOffset, 10))
		if result.Complete {
			w.Header().Set("Upload-Info", encodeTusUploadInfo(result))
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w)
	}
}

func (a *API) handleProjectFileUploadStatus(w http.ResponseWriter, r *http.Request, projectID string) {
	filename := uploadFilename(r.URL.Query().Get("filename"))
	if filename == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Filename is required.", nil)
		return
	}
	uploadID := strings.TrimSpace(r.URL.Query().Get("upload_id"))
	if uploadID == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "upload_id is required.", nil)
		return
	}
	totalSize, ok := parseNonNegativeInt64(r.URL.Query().Get("total_size"))
	if !ok {
		writeError(w, http.StatusBadRequest, "validation_error", "total_size must be a non-negative integer.", nil)
		return
	}
	targetPath := uploadTargetPath(r.URL.Query().Get("path"), filename)
	a.forwardProjectFileUploadStatus(w, r, projectID, targetPath, uploadID, totalSize)
}

func (a *API) handleProjectFileUploadJSON(w http.ResponseWriter, r *http.Request, projectID string) {
	r.Body = http.MaxBytesReader(w, r.Body, maxProjectFileLegacyUploadBytes*2+1024*1024)
	var in struct {
		Path          string `json:"path"`
		Filename      string `json:"filename"`
		ContentBase64 string `json:"content_base64"`
		UploadID      string `json:"upload_id"`
		Offset        int64  `json:"offset"`
		TotalSize     int64  `json:"total_size"`
		CreateDirs    bool   `json:"create_dirs"`
		Final         bool   `json:"final"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.UploadID) != "" || in.Offset > 0 || in.TotalSize > 0 || in.Final {
		a.handleProjectFileUploadChunkJSON(w, r, projectID, in.Path, in.Filename, in.UploadID, in.Offset, in.TotalSize, in.ContentBase64, in.CreateDirs, in.Final)
		return
	}
	content, err := base64.StdEncoding.DecodeString(strings.TrimSpace(in.ContentBase64))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid base64 file content.", nil)
		return
	}
	if int64(len(content)) > maxProjectFileLegacyUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "validation_error", "This request used the legacy one-shot upload path, which only accepts files up to 5 MiB. Refresh the page after deploying the latest frontend so uploads use resumable chunks.", map[string]any{"legacy_limit_bytes": maxProjectFileLegacyUploadBytes, "resumable_capability": "project_file_upload_chunked"})
		return
	}
	filename := uploadFilename(in.Filename)
	if filename == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Filename is required.", nil)
		return
	}
	targetPath := uploadTargetPath(in.Path, filename)
	a.forwardProjectFileUpload(w, r, projectID, targetPath, content, in.CreateDirs)
}

func (a *API) handleProjectFileUploadChunkJSON(w http.ResponseWriter, r *http.Request, projectID, targetDir, rawFilename, uploadID string, offset, totalSize int64, contentBase64 string, createDirs, final bool) {
	filename := uploadFilename(rawFilename)
	if filename == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Filename is required.", nil)
		return
	}
	uploadID = strings.TrimSpace(uploadID)
	if uploadID == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "upload_id is required.", nil)
		return
	}
	if offset < 0 || totalSize < 0 {
		writeError(w, http.StatusBadRequest, "validation_error", "offset and total_size must be non-negative.", nil)
		return
	}
	content, err := base64.StdEncoding.DecodeString(strings.TrimSpace(contentBase64))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid base64 file content.", nil)
		return
	}
	if int64(len(content)) > maxProjectFileUploadChunkBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "validation_error", "Upload chunk is too large.", nil)
		return
	}
	uploadedBytes := offset + int64(len(content))
	if uploadedBytes > totalSize {
		writeError(w, http.StatusBadRequest, "validation_error", "Upload chunk exceeds total_size.", nil)
		return
	}
	if final && uploadedBytes != totalSize {
		writeError(w, http.StatusBadRequest, "validation_error", "Final upload chunk must end at total_size.", nil)
		return
	}
	if totalSize == 0 && (!final || len(content) != 0 || offset != 0) {
		writeError(w, http.StatusBadRequest, "validation_error", "Empty uploads must send one final zero-byte chunk.", nil)
		return
	}
	targetPath := uploadTargetPath(targetDir, filename)
	a.forwardProjectFileUploadChunk(w, r, projectID, targetPath, uploadID, offset, totalSize, content, createDirs, final)
}

func (a *API) forwardProjectFileUpload(w http.ResponseWriter, r *http.Request, projectID, targetPath string, data []byte, createDirs bool) {
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

func (a *API) forwardProjectFileUploadStatus(w http.ResponseWriter, r *http.Request, projectID, targetPath, uploadID string, totalSize int64) {
	result, ok := a.requestProjectFileUploadStatus(w, r, projectID, targetPath, uploadID, totalSize)
	if ok {
		writeJSON(w, http.StatusOK, result)
	}
}

func (a *API) requestProjectFileUploadStatus(w http.ResponseWriter, r *http.Request, projectID, targetPath, uploadID string, totalSize int64) (ProjectFileActionResult, bool) {
	project, server, ok := a.projectAndServerForRunnerRequest(w, r, projectID, "project_file_upload_chunked")
	if !ok {
		return ProjectFileActionResult{}, false
	}
	env, err := a.runners.Request(server.RunnerID, "project.file.upload.status", ProjectFileUploadStatusRequestPayload{
		Workdir:   project.Workdir,
		Path:      targetPath,
		UploadID:  uploadID,
		TotalSize: totalSize,
	}, 10*time.Second)
	if err != nil {
		a.respondRunnerRequestError(w, server.RunnerID, "project file upload status request", err)
		return ProjectFileActionResult{}, false
	}
	var result ProjectFileActionResult
	if !decodeEnvelopePayload(env.Payload, &result, a, "project.file.upload.status.response") {
		writeError(w, http.StatusInternalServerError, "internal_error", "Invalid runner response.", nil)
		return ProjectFileActionResult{}, false
	}
	if result.Error != nil {
		writeError(w, http.StatusBadRequest, "validation_error", *result.Error, nil)
		return ProjectFileActionResult{}, false
	}
	return result, true
}

func (a *API) forwardProjectFileUploadChunk(w http.ResponseWriter, r *http.Request, projectID, targetPath, uploadID string, offset, totalSize int64, data []byte, createDirs, final bool) {
	result, ok := a.requestProjectFileUploadChunk(w, r, projectID, targetPath, uploadID, offset, totalSize, data, createDirs, final)
	if ok {
		writeJSON(w, http.StatusOK, result)
	}
}

func (a *API) requestProjectFileUploadChunk(w http.ResponseWriter, r *http.Request, projectID, targetPath, uploadID string, offset, totalSize int64, data []byte, createDirs, final bool) (ProjectFileActionResult, bool) {
	project, server, ok := a.projectAndServerForRunnerRequest(w, r, projectID, "project_file_upload_chunked")
	if !ok {
		return ProjectFileActionResult{}, false
	}
	env, err := a.runners.Request(server.RunnerID, "project.file.upload.chunk", ProjectFileUploadChunkRequestPayload{
		Workdir:       project.Workdir,
		Path:          targetPath,
		UploadID:      uploadID,
		Offset:        offset,
		TotalSize:     totalSize,
		ContentBase64: base64.StdEncoding.EncodeToString(data),
		CreateDirs:    createDirs,
		Final:         final,
	}, 30*time.Second)
	if err != nil {
		a.respondRunnerRequestError(w, server.RunnerID, "project file upload chunk request", err)
		return ProjectFileActionResult{}, false
	}
	var result ProjectFileActionResult
	if !decodeEnvelopePayload(env.Payload, &result, a, "project.file.upload.chunk.response") {
		writeError(w, http.StatusInternalServerError, "internal_error", "Invalid runner response.", nil)
		return ProjectFileActionResult{}, false
	}
	if result.Error != nil {
		status := http.StatusBadRequest
		if result.ResumeOffset != offset {
			status = http.StatusConflict
		}
		writeError(w, status, "validation_error", *result.Error, nil)
		return ProjectFileActionResult{}, false
	}
	return result, true
}

func parseNonNegativeInt64(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	out, err := strconv.ParseInt(value, 10, 64)
	return out, err == nil && out >= 0
}

type projectFileTusToken struct {
	ProjectID  string `json:"project_id"`
	Path       string `json:"path"`
	UploadID   string `json:"upload_id"`
	TotalSize  int64  `json:"total_size"`
	CreateDirs bool   `json:"create_dirs"`
}

func encodeProjectFileTusToken(token projectFileTusToken) (string, error) {
	raw, err := json.Marshal(token)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeProjectFileTusToken(value string) (projectFileTusToken, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return projectFileTusToken{}, err
	}
	var token projectFileTusToken
	if err := json.Unmarshal(raw, &token); err != nil {
		return projectFileTusToken{}, err
	}
	if strings.TrimSpace(token.ProjectID) == "" || strings.TrimSpace(token.Path) == "" || strings.TrimSpace(token.UploadID) == "" || token.TotalSize < 0 {
		return projectFileTusToken{}, fmt.Errorf("invalid upload token")
	}
	return token, nil
}

func parseTusMetadata(header string) (map[string]string, error) {
	out := map[string]string{}
	header = strings.TrimSpace(header)
	if header == "" {
		return out, nil
	}
	for _, item := range strings.Split(header, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parts := strings.SplitN(item, " ", 2)
		key := strings.TrimSpace(parts[0])
		if key == "" {
			return nil, fmt.Errorf("empty metadata key")
		}
		if len(parts) == 1 {
			out[key] = ""
			continue
		}
		value, err := base64.StdEncoding.DecodeString(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, err
		}
		out[key] = string(value)
	}
	return out, nil
}

func setTusHeaders(w http.ResponseWriter) {
	w.Header().Set("Tus-Resumable", "1.0.0")
	w.Header().Set("Access-Control-Expose-Headers", "Location, Tus-Resumable, Upload-Offset, Upload-Length, Upload-Info")
}

func encodeTusUploadInfo(result ProjectFileActionResult) string {
	raw, err := json.Marshal(result)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func uploadFilename(filename string) string {
	filename = path.Base(path.Clean(strings.ReplaceAll(filename, "\\", "/")))
	if filename == "." || filename == "/" || strings.TrimSpace(filename) == "" {
		return ""
	}
	return filename
}

func uploadTargetPath(targetDir, filename string) string {
	targetDir = strings.Trim(strings.ReplaceAll(strings.TrimSpace(targetDir), "\\", "/"), "/")
	if targetDir == "" || targetDir == "." {
		return filename
	}
	return targetDir + "/" + filename
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
