package control

import (
	"encoding/json"
	"time"
)

type CreateRunInput struct {
	TaskID          string
	Message         string
	Mode            string
	CodexModel      string
	ReasoningEffort string
	ServiceTier     string
	RawCommand      bool
	ContextItemIDs  []string
	IdempotencyKey  string
}

type CreateRunResult struct {
	Run    Run  `json:"run"`
	Task   Task `json:"task"`
	Assign *RunAssignPayload
}

type InterruptRunResult struct {
	Run         Run  `json:"run"`
	Task        Task `json:"task"`
	Assign      *RunAssignPayload
	CanceledRun Run
	Cancel      *RunCancelPayload
	CancelEvent RunEvent
}

type CreateServerInput struct {
	Name     string  `json:"name"`
	Alias    *string `json:"alias"`
	RunnerID string  `json:"runner_id"`
}

type PatchServerInput struct {
	Name     *string `json:"name"`
	Alias    *string `json:"alias"`
	RunnerID *string `json:"runner_id"`
	Status   *string `json:"status"`
}

type CreateProjectInput struct {
	ServerID      string  `json:"server_id"`
	Name          string  `json:"name"`
	Workdir       string  `json:"workdir"`
	DefaultBranch *string `json:"default_branch"`
	RulesPath     *string `json:"rules_path"`
}

type PatchProjectInput struct {
	ServerID      *string `json:"server_id"`
	Name          *string `json:"name"`
	Workdir       *string `json:"workdir"`
	DefaultBranch *string `json:"default_branch"`
	RulesPath     *string `json:"rules_path"`
}

type CreateTaskInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type PatchTaskInput struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
}

type MarkTaskDoneInput struct {
	Summary string           `json:"summary"`
	Memory  *TaskMemoryInput `json:"memory"`
}

type TaskMemoryInput struct {
	Problem         string `json:"problem"`
	RootCause       string `json:"root_cause"`
	Changes         string `json:"changes"`
	Files           string `json:"files"`
	Decisions       string `json:"decisions"`
	Verification    string `json:"verification"`
	RelatedTasks    string `json:"related_tasks"`
	SourceCommit    string `json:"source_commit"`
	StaleConditions string `json:"stale_conditions"`
}

type CreateContextInput struct {
	ServerID  *string  `json:"server_id"`
	ProjectID *string  `json:"project_id"`
	Scope     string   `json:"scope"`
	TaskID    *string  `json:"task_id"`
	Type      string   `json:"type"`
	Title     string   `json:"title"`
	Content   string   `json:"content"`
	Tags      []string `json:"tags"`
}

type PatchContextInput struct {
	ServerID  *string  `json:"server_id"`
	ProjectID *string  `json:"project_id"`
	Scope     *string  `json:"scope"`
	TaskID    *string  `json:"task_id"`
	Type      *string  `json:"type"`
	Title     *string  `json:"title"`
	Content   *string  `json:"content"`
	Tags      []string `json:"tags"`
	TagsSet   bool     `json:"-"`
}

type CompleteRunInput struct {
	RunID          string
	Status         string
	ExitCode       *int
	ErrorMessage   *string
	FinalMessage   *string
	CodexSessionID *string
	EndedAt        time.Time
}

type CompleteRunResult struct {
	Run   Run
	Event RunEvent
}

type RunnerEventInput struct {
	RunID      string
	EventType  string
	Stream     string
	Payload    json.RawMessage
	OccurredAt time.Time
}

type queuedRunRecord struct {
	RunID           string
	TaskID          string
	ProjectID       string
	Workdir         string
	Mode            string
	CodexSessionID  *string
	CodexModel      *string
	ReasoningEffort *string
	ServiceTier     *string
	Prompt          string
	RunnerID        string
}
