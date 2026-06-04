package control

import (
	"encoding/json"
	"time"
)

const (
	TaskStatusOpen        = "open"
	TaskStatusRunning     = "running"
	TaskStatusWaitingUser = "waiting_user"
	TaskStatusDone        = "done"
	TaskStatusArchived    = "archived"

	RunModeNew    = "new"
	RunModeResume = "resume"

	RunStatusQueued    = "queued"
	RunStatusRunning   = "running"
	RunStatusSucceeded = "succeeded"
	RunStatusFailed    = "failed"
	RunStatusCanceled  = "canceled"

	NotificationTypeTaskDone      = "task_done"
	NotificationTypeRunFinished   = "run_finished"
	NotificationTypeCodexReminder = "codex_reminder"

	EventRunState    = "run.state"
	EventCodexEvent  = "codex.event"
	EventProcessOut  = "process.output"
	EventRunnerError = "runner.error"
	EventRunFinal    = "run.final"
	StreamJSONL      = "jsonl"
	StreamStdout     = "stdout"
	StreamStderr     = "stderr"
	StreamSystem     = "system"
)

type Server struct {
	ID                 string         `json:"id"`
	Name               string         `json:"name"`
	Alias              *string        `json:"alias"`
	RunnerID           string         `json:"runner_id"`
	Status             string         `json:"status"`
	RunnerConnected    bool           `json:"runner_connected"`
	RunnerConnection   *RunnerInfo    `json:"runner_connection,omitempty"`
	RunnerCapabilities map[string]any `json:"runner_capabilities,omitempty"`
	LastHeartbeatAt    *time.Time     `json:"last_heartbeat_at"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type RunnerInfo struct {
	Hostname    string    `json:"hostname,omitempty"`
	Version     string    `json:"version,omitempty"`
	CodexPath   string    `json:"codex_path,omitempty"`
	ConnectedAt time.Time `json:"connected_at"`
}

type Project struct {
	ID            string    `json:"id"`
	ServerID      string    `json:"server_id"`
	Name          string    `json:"name"`
	Workdir       string    `json:"workdir"`
	DefaultBranch *string   `json:"default_branch"`
	RulesPath     *string   `json:"rules_path"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type DirectoryEntry struct {
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	IsDir   bool     `json:"is_dir"`
	Markers []string `json:"markers,omitempty"`
}

type DirectoryListing struct {
	Path    string           `json:"path"`
	Parent  *string          `json:"parent"`
	Entries []DirectoryEntry `json:"entries"`
	Error   *string          `json:"error,omitempty"`
}

type DirectoryListRequestPayload struct {
	Path string `json:"path"`
}

type ProjectFileEntry struct {
	Name       string     `json:"name"`
	Path       string     `json:"path"`
	IsDir      bool       `json:"is_dir"`
	Size       int64      `json:"size"`
	ModifiedAt *time.Time `json:"modified_at,omitempty"`
	Markers    []string   `json:"markers,omitempty"`
}

type ProjectFileListing struct {
	Root    string             `json:"root"`
	Path    string             `json:"path"`
	Parent  *string            `json:"parent"`
	Entries []ProjectFileEntry `json:"entries"`
	Error   *string            `json:"error,omitempty"`
}

type ProjectFileListRequestPayload struct {
	Workdir string `json:"workdir"`
	Path    string `json:"path"`
}

type ProjectFileReadRequestPayload struct {
	Workdir  string `json:"workdir"`
	Path     string `json:"path"`
	MaxBytes int64  `json:"max_bytes"`
}

type ProjectFileContent struct {
	Root       string     `json:"root"`
	Path       string     `json:"path"`
	Name       string     `json:"name"`
	Size       int64      `json:"size"`
	ModifiedAt *time.Time `json:"modified_at,omitempty"`
	Content    string     `json:"content"`
	Encoding   string     `json:"encoding"`
	Error      *string    `json:"error,omitempty"`
}

type ProjectFileWriteRequestPayload struct {
	Workdir    string `json:"workdir"`
	Path       string `json:"path"`
	Content    string `json:"content"`
	CreateDirs bool   `json:"create_dirs"`
}

type ProjectFileUploadRequestPayload struct {
	Workdir       string `json:"workdir"`
	Path          string `json:"path"`
	ContentBase64 string `json:"content_base64"`
	CreateDirs    bool   `json:"create_dirs"`
}

type ProjectFileUploadStatusRequestPayload struct {
	Workdir   string `json:"workdir"`
	Path      string `json:"path"`
	UploadID  string `json:"upload_id"`
	TotalSize int64  `json:"total_size"`
}

type ProjectFileUploadChunkRequestPayload struct {
	Workdir       string `json:"workdir"`
	Path          string `json:"path"`
	UploadID      string `json:"upload_id"`
	Offset        int64  `json:"offset"`
	TotalSize     int64  `json:"total_size"`
	ContentBase64 string `json:"content_base64"`
	CreateDirs    bool   `json:"create_dirs"`
	Final         bool   `json:"final"`
}

type ProjectFileUploadStreamRequestPayload struct {
	Workdir    string `json:"workdir"`
	Path       string `json:"path"`
	UploadID   string `json:"upload_id"`
	Offset     int64  `json:"offset"`
	TotalSize  int64  `json:"total_size"`
	ChunkBytes int64  `json:"chunk_bytes"`
	CreateDirs bool   `json:"create_dirs"`
	Final      bool   `json:"final"`
}

type ProjectFileActionRequestPayload struct {
	Workdir    string `json:"workdir"`
	Action     string `json:"action"`
	Path       string `json:"path"`
	TargetPath string `json:"target_path,omitempty"`
	IsDir      bool   `json:"is_dir,omitempty"`
}

type ProjectFileActionResult struct {
	Root          string     `json:"root"`
	Path          string     `json:"path"`
	TargetPath    string     `json:"target_path,omitempty"`
	IsDir         bool       `json:"is_dir,omitempty"`
	Size          int64      `json:"size,omitempty"`
	UploadedBytes int64      `json:"uploaded_bytes,omitempty"`
	TotalSize     int64      `json:"total_size,omitempty"`
	Complete      bool       `json:"complete,omitempty"`
	ResumeOffset  int64      `json:"resume_offset,omitempty"`
	ModifiedAt    *time.Time `json:"modified_at,omitempty"`
	Error         *string    `json:"error,omitempty"`
}

type ProjectCommandRequestPayload struct {
	Workdir     string `json:"workdir"`
	Command     string `json:"command"`
	TimeoutSecs int    `json:"timeout_secs"`
}

type ProjectCommandResult struct {
	Command    string  `json:"command"`
	Workdir    string  `json:"workdir"`
	ExitCode   int     `json:"exit_code"`
	Stdout     string  `json:"stdout"`
	Stderr     string  `json:"stderr"`
	DurationMS int64   `json:"duration_ms"`
	Error      *string `json:"error,omitempty"`
}

type ProjectTerminalOpenRequestPayload struct {
	TerminalID string `json:"terminal_id"`
	Workdir    string `json:"workdir"`
	Cols       int    `json:"cols"`
	Rows       int    `json:"rows"`
}

type ProjectTerminalOpenResponse struct {
	TerminalID string  `json:"terminal_id"`
	Workdir    string  `json:"workdir"`
	Error      *string `json:"error,omitempty"`
}

type ProjectTerminalInputPayload struct {
	TerminalID string `json:"terminal_id"`
	Data       string `json:"data"`
}

type ProjectTerminalResizePayload struct {
	TerminalID string `json:"terminal_id"`
	Cols       int    `json:"cols"`
	Rows       int    `json:"rows"`
}

type ProjectTerminalClosePayload struct {
	TerminalID string `json:"terminal_id"`
}

type ProjectTerminalOutputPayload struct {
	TerminalID string `json:"terminal_id"`
	Data       string `json:"data"`
}

type ProjectTerminalExitPayload struct {
	TerminalID string  `json:"terminal_id"`
	ExitCode   int     `json:"exit_code"`
	Error      *string `json:"error,omitempty"`
}

type Task struct {
	ID             string     `json:"id"`
	ProjectID      string     `json:"project_id"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Status         string     `json:"status"`
	CodexSessionID *string    `json:"codex_session_id"`
	ActiveRunID    *string    `json:"active_run_id"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	CompletedAt    *time.Time `json:"completed_at"`
	ArchivedAt     *time.Time `json:"archived_at"`
}

type Run struct {
	ID                      string          `json:"id"`
	TaskID                  string          `json:"task_id"`
	Mode                    string          `json:"mode"`
	Status                  string          `json:"status"`
	UserMessage             string          `json:"user_message"`
	GeneratedPrompt         string          `json:"generated_prompt"`
	InputImages             []RunInputImage `json:"input_images,omitempty"`
	CodexModel              *string         `json:"codex_model"`
	ReasoningEffort         *string         `json:"codex_reasoning_effort"`
	ServiceTier             *string         `json:"codex_service_tier"`
	RawCommand              bool            `json:"raw_command"`
	ReminderCallbackEnabled bool            `json:"reminder_callback_enabled"`
	FinalMessage            *string         `json:"final_message"`
	CodexSessionID          *string         `json:"codex_session_id"`
	AssignedRunnerID        *string         `json:"assigned_runner_id"`
	ExitCode                *int            `json:"exit_code"`
	ErrorMessage            *string         `json:"error_message"`
	CancelRequestedAt       *time.Time      `json:"cancel_requested_at"`
	RunnerStartedAt         *time.Time      `json:"runner_started_at"`
	StartedAt               time.Time       `json:"started_at"`
	EndedAt                 *time.Time      `json:"ended_at"`
	CreatedAt               time.Time       `json:"created_at"`
}

type RunInputImage struct {
	ID        string    `json:"id"`
	RunID     string    `json:"run_id,omitempty"`
	Filename  string    `json:"filename"`
	MimeType  string    `json:"mime_type"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

type RunInputImageAttachment struct {
	ID            string `json:"id"`
	Filename      string `json:"filename"`
	MimeType      string `json:"mime_type"`
	ContentBase64 string `json:"content_base64"`
}

type ContextItem struct {
	ID        string    `json:"id"`
	ServerID  *string   `json:"server_id"`
	ProjectID *string   `json:"project_id"`
	TaskID    *string   `json:"task_id"`
	Scope     string    `json:"scope"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type EmailNotificationConfig struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Enabled       bool      `json:"enabled"`
	SMTPHost      string    `json:"smtp_host"`
	SMTPPort      int       `json:"smtp_port"`
	SMTPUsername  string    `json:"smtp_username"`
	SMTPPassword  string    `json:"smtp_password,omitempty"`
	FromAddress   string    `json:"from_address"`
	ToAddresses   []string  `json:"to_addresses"`
	TLSMode       string    `json:"tls_mode"`
	SubjectPrefix string    `json:"subject_prefix"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type WorkbenchNotification struct {
	ID             string     `json:"id"`
	Type           string     `json:"type"`
	ServerID       string     `json:"server_id"`
	ServerName     string     `json:"server_name"`
	ProjectID      string     `json:"project_id"`
	ProjectName    string     `json:"project_name"`
	TaskID         string     `json:"task_id"`
	TaskTitle      string     `json:"task_title"`
	RunID          *string    `json:"run_id"`
	RunStatus      *string    `json:"run_status"`
	Title          string     `json:"title"`
	Message        string     `json:"message"`
	AcknowledgedAt *time.Time `json:"acknowledged_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

type RunEvent struct {
	ID         string          `json:"id"`
	RunID      string          `json:"run_id"`
	TaskID     string          `json:"task_id,omitempty"`
	Seq        int64           `json:"seq"`
	EventType  string          `json:"event_type"`
	Stream     string          `json:"stream"`
	Payload    json.RawMessage `json:"payload"`
	OccurredAt time.Time       `json:"occurred_at"`
	CreatedAt  time.Time       `json:"created_at,omitempty"`
}

type TaskMemory struct {
	ID              string    `json:"id"`
	TaskID          string    `json:"task_id"`
	ProjectID       string    `json:"project_id"`
	Problem         string    `json:"problem"`
	RootCause       string    `json:"root_cause"`
	Changes         string    `json:"changes"`
	Files           string    `json:"files"`
	Decisions       string    `json:"decisions"`
	Verification    string    `json:"verification"`
	RelatedTasks    string    `json:"related_tasks"`
	SourceCommit    string    `json:"source_commit"`
	StaleConditions string    `json:"stale_conditions"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type RunnerEnvelope struct {
	Type      string          `json:"type"`
	MessageID string          `json:"message_id"`
	SentAt    time.Time       `json:"sent_at"`
	Payload   json.RawMessage `json:"payload"`
}

type RunnerRegisterPayload struct {
	RunnerID     string         `json:"runner_id"`
	Hostname     string         `json:"hostname"`
	Version      string         `json:"version"`
	CodexPath    string         `json:"codex_path"`
	Capabilities map[string]any `json:"capabilities"`
	ActiveRunIDs []string       `json:"active_run_ids"`
}

type RunnerFileTransferRegisterPayload struct {
	RunnerID string `json:"runner_id"`
	Version  string `json:"version"`
}

type RunnerHeartbeatPayload struct {
	RunnerID     string   `json:"runner_id"`
	ActiveRunIDs []string `json:"active_run_ids"`
}

type RunAssignPayload struct {
	RunID                   string                    `json:"run_id"`
	TaskID                  string                    `json:"task_id"`
	ProjectID               string                    `json:"project_id"`
	Workdir                 string                    `json:"workdir"`
	Mode                    string                    `json:"mode"`
	CodexSessionID          *string                   `json:"codex_session_id"`
	CodexModel              *string                   `json:"codex_model,omitempty"`
	ReasoningEffort         *string                   `json:"codex_reasoning_effort,omitempty"`
	ServiceTier             *string                   `json:"codex_service_tier,omitempty"`
	ReminderCallbackEnabled bool                      `json:"reminder_callback_enabled,omitempty"`
	Prompt                  string                    `json:"prompt"`
	Argv                    []string                  `json:"argv"`
	InputImages             []RunInputImageAttachment `json:"input_images,omitempty"`
	AssignedRunner          string                    `json:"-"`
	TargetRunnerID          string                    `json:"-"`
	ProjectServerID         string                    `json:"-"`
}

type RunStartedPayload struct {
	RunID     string    `json:"run_id"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
}

type RunEventPayload struct {
	RunID        string          `json:"run_id"`
	SourceSeq    int64           `json:"source_seq"`
	EventType    string          `json:"event_type"`
	Stream       string          `json:"stream"`
	EventPayload json.RawMessage `json:"event_payload"`
	OccurredAt   time.Time       `json:"occurred_at"`
}

type RunCompletedPayload struct {
	RunID          string     `json:"run_id"`
	Status         string     `json:"status"`
	ExitCode       *int       `json:"exit_code"`
	ErrorMessage   *string    `json:"error_message"`
	FinalMessage   *string    `json:"final_message"`
	CodexSessionID *string    `json:"codex_session_id"`
	EndedAt        *time.Time `json:"ended_at"`
}

type RunReminderPayload struct {
	RunID   string     `json:"run_id"`
	Title   string     `json:"title"`
	Message string     `json:"message"`
	SentAt  *time.Time `json:"sent_at"`
}

type RunCancelPayload struct {
	RunID       string    `json:"run_id"`
	Reason      string    `json:"reason"`
	RequestedAt time.Time `json:"requested_at"`
}

type RunCancelAckPayload struct {
	RunID    string `json:"run_id"`
	Accepted bool   `json:"accepted"`
}

type RunnerUpdateRequestPayload struct {
	UpdateID      string `json:"update_id,omitempty"`
	TargetVersion string `json:"target_version,omitempty"`
}

type RunnerControlResponsePayload struct {
	Accepted bool    `json:"accepted"`
	Message  string  `json:"message"`
	Error    *string `json:"error,omitempty"`
}

type RunnerUpdateResponsePayload = RunnerControlResponsePayload

type RunnerUpdateStatusPayload struct {
	UpdateID      string    `json:"update_id"`
	RunnerID      string    `json:"runner_id,omitempty"`
	Status        string    `json:"status"`
	Message       string    `json:"message,omitempty"`
	Version       string    `json:"version,omitempty"`
	TargetVersion string    `json:"target_version,omitempty"`
	Error         *string   `json:"error,omitempty"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type RunnerShutdownRequestPayload struct {
	Reason string `json:"reason,omitempty"`
}

type RunnerShutdownResponsePayload = RunnerControlResponsePayload

type RunnerUpdateServerResult struct {
	ServerID        string  `json:"server_id"`
	ServerName      string  `json:"server_name"`
	RunnerID        string  `json:"runner_id"`
	PreviousVersion *string `json:"previous_version,omitempty"`
	Status          string  `json:"status"`
	Message         string  `json:"message"`
}

type RunnerUpdateAllResponse struct {
	RequestedAt   time.Time                  `json:"requested_at"`
	UpdateID      string                     `json:"update_id"`
	TargetVersion string                     `json:"target_version"`
	DeadlineAt    time.Time                  `json:"deadline_at"`
	Accepted      int                        `json:"accepted"`
	Skipped       int                        `json:"skipped"`
	Failed        int                        `json:"failed"`
	Results       []RunnerUpdateServerResult `json:"results"`
}

type RunnerUpdateProgressResult struct {
	ServerID        string     `json:"server_id"`
	ServerName      string     `json:"server_name"`
	RunnerID        string     `json:"runner_id"`
	PreviousVersion *string    `json:"previous_version,omitempty"`
	CurrentVersion  *string    `json:"current_version,omitempty"`
	Status          string     `json:"status"`
	Message         string     `json:"message"`
	Error           *string    `json:"error,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

type RunnerUpdateProgress struct {
	UpdateID      string                       `json:"update_id"`
	RequestedAt   time.Time                    `json:"requested_at"`
	DeadlineAt    time.Time                    `json:"deadline_at"`
	TargetVersion string                       `json:"target_version"`
	Active        bool                         `json:"active"`
	Total         int                          `json:"total"`
	Succeeded     int                          `json:"succeeded"`
	InProgress    int                          `json:"in_progress"`
	Skipped       int                          `json:"skipped"`
	Failed        int                          `json:"failed"`
	Results       []RunnerUpdateProgressResult `json:"results"`
}
