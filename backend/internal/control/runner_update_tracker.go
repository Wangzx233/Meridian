package control

import (
	"sort"
	"strings"
	"sync"
	"time"
)

const runnerUpdateTimeout = 5 * time.Minute

type RunnerUpdateTracker struct {
	mu     sync.Mutex
	latest *RunnerUpdateProgress
}

func NewRunnerUpdateTracker() *RunnerUpdateTracker {
	return &RunnerUpdateTracker{}
}

func (t *RunnerUpdateTracker) Begin(targetVersion string, now time.Time) RunnerUpdateProgress {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	progress := RunnerUpdateProgress{
		UpdateID:      randomID("upd"),
		RequestedAt:   now,
		DeadlineAt:    now.Add(runnerUpdateTimeout),
		TargetVersion: strings.TrimSpace(targetVersion),
		Active:        true,
		Results:       []RunnerUpdateProgressResult{},
	}

	t.mu.Lock()
	t.latest = &progress
	t.recalculateLocked(now)
	out := cloneRunnerUpdateProgress(*t.latest)
	t.mu.Unlock()
	return out
}

func (t *RunnerUpdateTracker) Upsert(result RunnerUpdateProgressResult, now time.Time) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.latest == nil || result.RunnerID == "" {
		return
	}
	t.upsertLocked(result, now)
	t.recalculateLocked(now)
}

func (t *RunnerUpdateTracker) MarkRunnerStatus(payload RunnerUpdateStatusPayload, now time.Time) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if !payload.OccurredAt.IsZero() {
		now = payload.OccurredAt.UTC()
	}
	status := strings.TrimSpace(payload.Status)
	if status == "" || strings.TrimSpace(payload.RunnerID) == "" || strings.TrimSpace(payload.UpdateID) == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.latest == nil || payload.UpdateID != t.latest.UpdateID {
		return
	}

	result := RunnerUpdateProgressResult{
		RunnerID:    payload.RunnerID,
		Status:      status,
		Message:     strings.TrimSpace(payload.Message),
		Error:       cloneStringPtr(payload.Error),
		UpdatedAt:   now,
		CompletedAt: nil,
	}
	if strings.TrimSpace(payload.Version) != "" {
		result.CurrentVersion = stringPtr(strings.TrimSpace(payload.Version))
	}
	if result.Message == "" && result.Error != nil && strings.TrimSpace(*result.Error) != "" {
		result.Message = *result.Error
	}
	if runnerUpdateStatusTerminal(status) {
		completedAt := now
		result.CompletedAt = &completedAt
	}
	t.upsertLocked(result, now)
	t.recalculateLocked(now)
}

func (t *RunnerUpdateTracker) MarkRegistered(runnerID string, info RunnerInfo, now time.Time) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	runnerID = strings.TrimSpace(runnerID)
	if runnerID == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.latest == nil {
		return
	}
	idx := t.findRunnerLocked(runnerID)
	if idx < 0 {
		return
	}
	if t.latest.Results[idx].Status == "skipped" {
		return
	}
	if runnerUpdateStatusFailed(t.latest.Results[idx].Status) {
		return
	}

	status := "succeeded"
	message := "Runner reconnected after update."
	var updateErr *string
	targetVersion := strings.TrimSpace(t.latest.TargetVersion)
	currentVersion := strings.TrimSpace(info.Version)
	if targetVersion != "" && targetVersion != "dev" {
		if currentVersion == targetVersion {
			message = "Runner reconnected with the target version."
		} else {
			status = "version_mismatch"
			msg := "Runner reconnected, but it is not running the target version."
			message = msg
			updateErr = &msg
		}
	}
	completedAt := now
	result := RunnerUpdateProgressResult{
		RunnerID:       runnerID,
		CurrentVersion: nil,
		Status:         status,
		Message:        message,
		Error:          updateErr,
		UpdatedAt:      now,
		CompletedAt:    &completedAt,
	}
	if currentVersion != "" {
		result.CurrentVersion = &currentVersion
	}
	t.upsertLocked(result, now)
	t.recalculateLocked(now)
}

func (t *RunnerUpdateTracker) MarkDisconnected(runnerID string, now time.Time) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	runnerID = strings.TrimSpace(runnerID)
	if runnerID == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.latest == nil {
		return
	}
	idx := t.findRunnerLocked(runnerID)
	if idx < 0 || !runnerUpdateStatusInProgress(t.latest.Results[idx].Status) {
		return
	}
	t.latest.Results[idx].Status = "waiting_reconnect"
	t.latest.Results[idx].Message = "Runner websocket disconnected; waiting for it to reconnect with the updated binary."
	t.latest.Results[idx].UpdatedAt = now
	t.recalculateLocked(now)
}

func (t *RunnerUpdateTracker) Latest(now time.Time) RunnerUpdateProgress {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.latest == nil {
		return RunnerUpdateProgress{Results: []RunnerUpdateProgressResult{}}
	}
	t.recalculateLocked(now)
	return cloneRunnerUpdateProgress(*t.latest)
}

func (t *RunnerUpdateTracker) upsertLocked(result RunnerUpdateProgressResult, now time.Time) {
	if result.UpdatedAt.IsZero() {
		result.UpdatedAt = now
	}
	if result.Status == "" {
		result.Status = "requested"
	}
	if result.Message == "" {
		result.Message = runnerUpdateDefaultMessage(result.Status)
	}
	if runnerUpdateStatusTerminal(result.Status) && result.CompletedAt == nil {
		completedAt := result.UpdatedAt
		result.CompletedAt = &completedAt
	}
	idx := t.findRunnerLocked(result.RunnerID)
	if idx < 0 {
		if result.ServerName == "" {
			result.ServerName = result.RunnerID
		}
		t.latest.Results = append(t.latest.Results, result)
		return
	}

	current := &t.latest.Results[idx]
	if result.ServerID != "" {
		current.ServerID = result.ServerID
	}
	if result.ServerName != "" {
		current.ServerName = result.ServerName
	}
	if result.RunnerID != "" {
		current.RunnerID = result.RunnerID
	}
	if result.PreviousVersion != nil {
		current.PreviousVersion = cloneStringPtr(result.PreviousVersion)
	}
	if result.CurrentVersion != nil {
		current.CurrentVersion = cloneStringPtr(result.CurrentVersion)
	}
	current.Status = result.Status
	current.Message = result.Message
	current.Error = cloneStringPtr(result.Error)
	current.UpdatedAt = result.UpdatedAt
	current.CompletedAt = cloneTimePtr(result.CompletedAt)
	if current.ServerName == "" {
		current.ServerName = current.RunnerID
	}
}

func (t *RunnerUpdateTracker) findRunnerLocked(runnerID string) int {
	for i := range t.latest.Results {
		if t.latest.Results[i].RunnerID == runnerID {
			return i
		}
	}
	return -1
}

func (t *RunnerUpdateTracker) recalculateLocked(now time.Time) {
	if t.latest == nil {
		return
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	t.latest.Succeeded = 0
	t.latest.InProgress = 0
	t.latest.Skipped = 0
	t.latest.Failed = 0
	for i := range t.latest.Results {
		result := &t.latest.Results[i]
		if result.UpdatedAt.IsZero() {
			result.UpdatedAt = t.latest.RequestedAt
		}
		if result.Status == "" {
			result.Status = "requested"
		}
		if result.Message == "" {
			result.Message = runnerUpdateDefaultMessage(result.Status)
		}
		if runnerUpdateStatusInProgress(result.Status) && !t.latest.DeadlineAt.IsZero() && now.After(t.latest.DeadlineAt) {
			msg := "Runner update did not finish before the progress timeout."
			result.Status = "timed_out"
			result.Message = msg
			result.Error = &msg
			result.UpdatedAt = now
			result.CompletedAt = &now
		}

		switch {
		case runnerUpdateStatusSucceeded(result.Status):
			t.latest.Succeeded++
		case result.Status == "skipped":
			t.latest.Skipped++
		case runnerUpdateStatusFailed(result.Status):
			t.latest.Failed++
		case runnerUpdateStatusInProgress(result.Status):
			t.latest.InProgress++
		default:
			t.latest.InProgress++
		}
	}
	sort.SliceStable(t.latest.Results, func(i, j int) bool {
		left := t.latest.Results[i]
		right := t.latest.Results[j]
		if runnerUpdateStatusRank(left.Status) != runnerUpdateStatusRank(right.Status) {
			return runnerUpdateStatusRank(left.Status) < runnerUpdateStatusRank(right.Status)
		}
		return strings.ToLower(left.ServerName) < strings.ToLower(right.ServerName)
	})
	t.latest.Total = len(t.latest.Results)
	t.latest.Active = t.latest.InProgress > 0
}

func runnerUpdateStatusRank(status string) int {
	if runnerUpdateStatusFailed(status) {
		return 0
	}
	if runnerUpdateStatusInProgress(status) {
		return 1
	}
	if status == "skipped" {
		return 2
	}
	if runnerUpdateStatusSucceeded(status) {
		return 3
	}
	return 4
}

func runnerUpdateStatusSucceeded(status string) bool {
	switch status {
	case "succeeded", "up_to_date":
		return true
	default:
		return false
	}
}

func runnerUpdateStatusFailed(status string) bool {
	switch status {
	case "failed", "timed_out", "version_mismatch":
		return true
	default:
		return false
	}
}

func runnerUpdateStatusInProgress(status string) bool {
	switch status {
	case "requested", "accepted", "downloading", "replacing", "restarting", "waiting_reconnect":
		return true
	default:
		return false
	}
}

func runnerUpdateStatusTerminal(status string) bool {
	return runnerUpdateStatusSucceeded(status) || runnerUpdateStatusFailed(status) || status == "skipped"
}

func runnerUpdateDefaultMessage(status string) string {
	switch status {
	case "requested":
		return "Update request is being sent to the runner."
	case "accepted":
		return "Runner accepted the update request."
	case "downloading":
		return "Runner is downloading the updated binary."
	case "replacing":
		return "Runner is replacing its local binary."
	case "restarting":
		return "Runner is restarting with the updated binary."
	case "waiting_reconnect":
		return "Waiting for the runner to reconnect."
	case "succeeded":
		return "Runner update completed."
	case "up_to_date":
		return "Runner is already on the target version."
	case "skipped":
		return "Runner update was skipped."
	case "version_mismatch":
		return "Runner reconnected with a different version than expected."
	case "timed_out":
		return "Runner update timed out."
	case "failed":
		return "Runner update failed."
	default:
		return "Runner update status is unknown."
	}
}

func cloneRunnerUpdateProgress(progress RunnerUpdateProgress) RunnerUpdateProgress {
	out := progress
	out.Results = make([]RunnerUpdateProgressResult, len(progress.Results))
	for i := range progress.Results {
		out.Results[i] = cloneRunnerUpdateProgressResult(progress.Results[i])
	}
	return out
}

func cloneRunnerUpdateProgressResult(result RunnerUpdateProgressResult) RunnerUpdateProgressResult {
	out := result
	out.PreviousVersion = cloneStringPtr(result.PreviousVersion)
	out.CurrentVersion = cloneStringPtr(result.CurrentVersion)
	out.Error = cloneStringPtr(result.Error)
	out.CompletedAt = cloneTimePtr(result.CompletedAt)
	return out
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func stringPtr(value string) *string {
	return &value
}
