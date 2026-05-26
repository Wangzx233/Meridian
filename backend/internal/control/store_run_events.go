package control

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

func (s *Store) ListRuns(ctx context.Context, taskID string) ([]Run, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, task_id, mode, status, user_message, generated_prompt, codex_model, codex_reasoning_effort, codex_service_tier, raw_command, reminder_callback_enabled, final_message, codex_session_id,
		       assigned_runner_id, exit_code, error_message, cancel_requested_at, runner_started_at,
		       started_at, ended_at, created_at
		FROM runs WHERE task_id=$1 ORDER BY created_at ASC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRuns(rows)
}

func (s *Store) GetRun(ctx context.Context, id string) (Run, error) {
	row := s.db.QueryRow(ctx, `
		SELECT id, task_id, mode, status, user_message, generated_prompt, codex_model, codex_reasoning_effort, codex_service_tier, raw_command, reminder_callback_enabled, final_message, codex_session_id,
		       assigned_runner_id, exit_code, error_message, cancel_requested_at, runner_started_at,
		       started_at, ended_at, created_at
		FROM runs WHERE id=$1`, id)
	return scanRun(row)
}

func (s *Store) ListEvents(ctx context.Context, runID string, afterSeq int64) ([]RunEvent, error) {
	rows, err := s.db.Query(ctx, `
		SELECT e.id, e.run_id, r.task_id, e.seq, e.event_type, e.stream, e.payload, e.occurred_at, e.created_at
		FROM run_events e
		JOIN runs r ON r.id=e.run_id
		WHERE e.run_id=$1 AND e.seq>$2
		ORDER BY e.seq ASC`, runID, afterSeq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRunEvents(rows)
}

func (s *Store) InsertRunnerEvent(ctx context.Context, in RunnerEventInput) (RunEvent, error) {
	if in.OccurredAt.IsZero() {
		in.OccurredAt = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return RunEvent{}, err
	}
	defer rollback(ctx, tx)
	event, err := insertRunEventTx(ctx, tx, in.RunID, in.EventType, in.Stream, json.RawMessage(in.Payload), in.OccurredAt)
	if err != nil {
		return RunEvent{}, err
	}
	if in.EventType == EventCodexEvent {
		var payload struct {
			SessionID string `json:"session_id"`
		}
		if json.Unmarshal(in.Payload, &payload) == nil && payload.SessionID != "" {
			var taskID string
			err = tx.QueryRow(ctx, `
				UPDATE runs SET codex_session_id=$2
				WHERE id=$1 AND (codex_session_id IS NULL OR codex_session_id='')
				RETURNING task_id`, in.RunID, payload.SessionID).Scan(&taskID)
			if errors.Is(err, pgx.ErrNoRows) {
				err = tx.QueryRow(ctx, `SELECT task_id FROM runs WHERE id=$1`, in.RunID).Scan(&taskID)
			}
			if err != nil {
				return RunEvent{}, err
			}
			_, err = tx.Exec(ctx, `
				UPDATE tasks SET codex_session_id=$2, updated_at=now()
				WHERE id=$1 AND (codex_session_id IS NULL OR codex_session_id='')`, taskID, payload.SessionID)
			if err != nil {
				return RunEvent{}, err
			}
		}
	}
	return event, tx.Commit(ctx)
}

func (s *Store) MarkRunStarted(ctx context.Context, runID, runnerID string, startedAt time.Time) (RunEvent, error) {
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return RunEvent{}, err
	}
	defer rollback(ctx, tx)

	var previous string
	err = tx.QueryRow(ctx, `SELECT status FROM runs WHERE id=$1 FOR UPDATE`, runID).Scan(&previous)
	if err != nil {
		return RunEvent{}, dbErr(err)
	}
	if previous != RunStatusQueued {
		return RunEvent{}, ErrInvalidState
	}
	tag, err := tx.Exec(ctx, `
		UPDATE runs SET status='running', assigned_runner_id=$2, runner_started_at=$3
		WHERE id=$1 AND status='queued'`, runID, runnerID, startedAt)
	if err != nil {
		return RunEvent{}, err
	}
	if tag.RowsAffected() == 0 {
		return RunEvent{}, ErrInvalidState
	}
	event, err := insertRunEventTx(ctx, tx, runID, EventRunState, StreamSystem, map[string]any{
		"status":          RunStatusRunning,
		"previous_status": previous,
	}, startedAt)
	if err != nil {
		return RunEvent{}, err
	}
	return event, tx.Commit(ctx)
}

func (s *Store) CompleteRun(ctx context.Context, in CompleteRunInput) (CompleteRunResult, error) {
	if in.Status != RunStatusSucceeded && in.Status != RunStatusFailed && in.Status != RunStatusCanceled {
		return CompleteRunResult{}, ErrValidation
	}
	if in.EndedAt.IsZero() {
		in.EndedAt = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CompleteRunResult{}, err
	}
	defer rollback(ctx, tx)

	var taskID, previous, mode string
	var existingRunSession *string
	err = tx.QueryRow(ctx, `SELECT task_id, status, mode, codex_session_id FROM runs WHERE id=$1 FOR UPDATE`, in.RunID).Scan(&taskID, &previous, &mode, &existingRunSession)
	if err != nil {
		return CompleteRunResult{}, dbErr(err)
	}
	if isTerminalRunStatus(previous) {
		return CompleteRunResult{}, ErrInvalidState
	}
	sessionID := in.CodexSessionID
	if sessionID == nil {
		sessionID = existingRunSession
	}
	if sessionID == nil && in.Status == RunStatusSucceeded && mode == RunModeResume {
		var recoveredSession string
		err = tx.QueryRow(ctx, `
			SELECT codex_session_id
			FROM tasks
			WHERE id=$1 AND codex_session_id IS NOT NULL AND codex_session_id <> ''`, taskID).Scan(&recoveredSession)
		if err == nil {
			sessionID = &recoveredSession
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return CompleteRunResult{}, err
		}
	}
	run, err := scanRun(tx.QueryRow(ctx, `
		UPDATE runs
		SET status=$2, exit_code=$3, error_message=$4, final_message=$5, codex_session_id=$6, ended_at=$7
		WHERE id=$1
		RETURNING id, task_id, mode, status, user_message, generated_prompt, codex_model, codex_reasoning_effort, codex_service_tier, raw_command, reminder_callback_enabled, final_message, codex_session_id,
		          assigned_runner_id, exit_code, error_message, cancel_requested_at, runner_started_at,
		          started_at, ended_at, created_at`,
		in.RunID, in.Status, in.ExitCode, in.ErrorMessage, in.FinalMessage, sessionID, in.EndedAt))
	if err != nil {
		return CompleteRunResult{}, err
	}

	if in.Status == RunStatusSucceeded && sessionID != nil && *sessionID != "" && mode == RunModeNew {
		_, err = tx.Exec(ctx, `UPDATE tasks SET codex_session_id=$2, updated_at=now() WHERE id=$1`, taskID, *sessionID)
		if err != nil {
			return CompleteRunResult{}, err
		}
	}
	_, err = tx.Exec(ctx, `
		UPDATE tasks SET status='waiting_user', updated_at=now()
		WHERE id=$1 AND status='running'`, taskID)
	if err != nil {
		return CompleteRunResult{}, err
	}
	event, err := insertRunEventTx(ctx, tx, in.RunID, EventRunFinal, StreamSystem, map[string]any{
		"status":           in.Status,
		"exit_code":        in.ExitCode,
		"final_message":    in.FinalMessage,
		"error_message":    in.ErrorMessage,
		"codex_session_id": sessionID,
	}, in.EndedAt)
	if err != nil {
		return CompleteRunResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CompleteRunResult{}, err
	}
	return CompleteRunResult{Run: run, Event: event}, nil
}

func (s *Store) ReconcileRunnerActiveRuns(ctx context.Context, runnerID string, activeRunIDs []string, observedAt time.Time) ([]RunEvent, error) {
	if strings.TrimSpace(runnerID) == "" {
		return nil, ErrValidation
	}
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	active := make(map[string]struct{}, len(activeRunIDs))
	for _, runID := range activeRunIDs {
		if strings.TrimSpace(runID) != "" {
			active[runID] = struct{}{}
		}
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer rollback(ctx, tx)

	rows, err := tx.Query(ctx, `
		SELECT id, task_id, codex_session_id
		FROM runs
		WHERE assigned_runner_id=$1
		  AND status='running'
		  AND runner_started_at IS NOT NULL
		  AND runner_started_at < $2
		FOR UPDATE`, runnerID, observedAt.Add(-15*time.Second))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type staleRun struct {
		id        string
		taskID    string
		sessionID *string
	}
	stale := []staleRun{}
	for rows.Next() {
		var run staleRun
		if err := rows.Scan(&run.id, &run.taskID, &run.sessionID); err != nil {
			return nil, err
		}
		if _, ok := active[run.id]; ok {
			continue
		}
		stale = append(stale, run)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	events := make([]RunEvent, 0, len(stale))
	for _, run := range stale {
		msg := "Runner heartbeat no longer reports this run as active before a completion event was received."
		_, err = tx.Exec(ctx, `
			UPDATE runs
			SET status='failed', error_message=$2, ended_at=$3
			WHERE id=$1 AND status='running'`, run.id, msg, observedAt)
		if err != nil {
			return nil, err
		}
		var exists bool
		err = tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM run_events
				WHERE run_id=$1 AND event_type=$2
			)`, run.id, EventRunFinal).Scan(&exists)
		if err != nil {
			return nil, err
		}
		if exists {
			continue
		}
		_, err = tx.Exec(ctx, `UPDATE tasks SET status='waiting_user', updated_at=now() WHERE id=$1 AND status='running'`, run.taskID)
		if err != nil {
			return nil, err
		}
		event, err := insertRunEventTx(ctx, tx, run.id, EventRunFinal, StreamSystem, map[string]any{
			"status":           RunStatusFailed,
			"exit_code":        nil,
			"final_message":    nil,
			"error_message":    msg,
			"codex_session_id": run.sessionID,
		}, observedAt)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, tx.Commit(ctx)
}

func (s *Store) ReconcileRunnerDisconnected(ctx context.Context, runnerID string, observedAt time.Time) ([]RunEvent, error) {
	if strings.TrimSpace(runnerID) == "" {
		return nil, ErrValidation
	}
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer rollback(ctx, tx)

	rows, err := tx.Query(ctx, `
		SELECT id, task_id, codex_session_id
		FROM runs
		WHERE assigned_runner_id=$1
		  AND status IN ('queued','running')
		FOR UPDATE`, runnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type activeRun struct {
		id        string
		taskID    string
		sessionID *string
	}
	runs := []activeRun{}
	for rows.Next() {
		var run activeRun
		if err := rows.Scan(&run.id, &run.taskID, &run.sessionID); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	events := make([]RunEvent, 0, len(runs))
	for _, run := range runs {
		msg := "Runner disconnected before a completion event was received."
		_, err = tx.Exec(ctx, `
			UPDATE runs
			SET status='failed', error_message=COALESCE(error_message, $2), ended_at=COALESCE(ended_at, $3)
			WHERE id=$1 AND status IN ('queued','running')`, run.id, msg, observedAt)
		if err != nil {
			return nil, err
		}
		var exists bool
		err = tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM run_events
				WHERE run_id=$1 AND event_type=$2
			)`, run.id, EventRunFinal).Scan(&exists)
		if err != nil {
			return nil, err
		}
		if exists {
			continue
		}
		_, err = tx.Exec(ctx, `
			UPDATE tasks SET status='waiting_user', updated_at=now()
			WHERE id=$1 AND status='running'`, run.taskID)
		if err != nil {
			return nil, err
		}
		event, err := insertRunEventTx(ctx, tx, run.id, EventRunFinal, StreamSystem, map[string]any{
			"status":           RunStatusFailed,
			"exit_code":        nil,
			"final_message":    nil,
			"error_message":    msg,
			"codex_session_id": run.sessionID,
		}, observedAt)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, tx.Commit(ctx)
}

func (s *Store) CancelRun(ctx context.Context, runID, reason string) (Run, *RunCancelPayload, RunEvent, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Run{}, nil, RunEvent{}, err
	}
	defer rollback(ctx, tx)

	run, err := scanRun(tx.QueryRow(ctx, `
		SELECT id, task_id, mode, status, user_message, generated_prompt, codex_model, codex_reasoning_effort, codex_service_tier, raw_command, reminder_callback_enabled, final_message, codex_session_id,
		       assigned_runner_id, exit_code, error_message, cancel_requested_at, runner_started_at,
		       started_at, ended_at, created_at
		FROM runs WHERE id=$1 FOR UPDATE`, runID))
	if err != nil {
		return Run{}, nil, RunEvent{}, err
	}
	if run.Status != RunStatusQueued && run.Status != RunStatusRunning {
		return Run{}, nil, RunEvent{}, ErrInvalidState
	}
	now := time.Now().UTC()
	var event RunEvent
	if run.Status == RunStatusQueued {
		msg := reason
		run.Status = RunStatusCanceled
		run.EndedAt = &now
		run.CancelRequestedAt = &now
		run.ErrorMessage = &msg
		_, err = tx.Exec(ctx, `
			UPDATE runs SET status='canceled', cancel_requested_at=$2, ended_at=$2, error_message=$3
			WHERE id=$1`, runID, now, msg)
		if err != nil {
			return Run{}, nil, RunEvent{}, err
		}
		_, err = tx.Exec(ctx, `UPDATE tasks SET status='waiting_user', updated_at=now() WHERE id=$1 AND status='running'`, run.TaskID)
		if err != nil {
			return Run{}, nil, RunEvent{}, err
		}
		event, err = insertRunEventTx(ctx, tx, runID, EventRunFinal, StreamSystem, map[string]any{
			"status":           RunStatusCanceled,
			"exit_code":        nil,
			"final_message":    nil,
			"error_message":    msg,
			"codex_session_id": run.CodexSessionID,
		}, now)
		if err != nil {
			return Run{}, nil, RunEvent{}, err
		}
	} else {
		run.CancelRequestedAt = &now
		_, err = tx.Exec(ctx, `UPDATE runs SET cancel_requested_at=$2 WHERE id=$1`, runID, now)
		if err != nil {
			return Run{}, nil, RunEvent{}, err
		}
		event, err = insertRunEventTx(ctx, tx, runID, EventRunState, StreamSystem, map[string]any{
			"status":        RunStatusRunning,
			"canceling":     true,
			"cancel_reason": reason,
		}, now)
		if err != nil {
			return Run{}, nil, RunEvent{}, err
		}
	}
	cancel := &RunCancelPayload{RunID: runID, Reason: reason, RequestedAt: now}
	if err := tx.Commit(ctx); err != nil {
		return Run{}, nil, RunEvent{}, err
	}
	if run.Status == RunStatusQueued && run.AssignedRunnerID == nil {
		cancel = nil
	}
	return run, cancel, event, nil
}

func (s *Store) NextQueuedRunsForRunner(ctx context.Context, runnerID string, limit int) ([]*RunAssignPayload, error) {
	if limit <= 0 {
		limit = 10
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer rollback(ctx, tx)

	rows, err := tx.Query(ctx, `
		SELECT r.id, r.task_id, p.id, p.workdir, r.mode, r.codex_session_id, r.codex_model, r.codex_reasoning_effort, r.codex_service_tier, r.reminder_callback_enabled, r.generated_prompt, srv.runner_id
		FROM runs r
		JOIN tasks t ON t.id=r.task_id
		JOIN projects p ON p.id=t.project_id
		JOIN servers srv ON srv.id=p.server_id
		WHERE r.status='queued' AND srv.runner_id=$1
		ORDER BY r.created_at ASC
		LIMIT $2
		FOR UPDATE OF r SKIP LOCKED`, runnerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	assignments := []*RunAssignPayload{}
	for rows.Next() {
		var rec queuedRunRecord
		if err := rows.Scan(&rec.RunID, &rec.TaskID, &rec.ProjectID, &rec.Workdir, &rec.Mode, &rec.CodexSessionID, &rec.CodexModel, &rec.ReasoningEffort, &rec.ServiceTier, &rec.ReminderCallbackEnabled, &rec.Prompt, &rec.RunnerID); err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, `UPDATE runs SET assigned_runner_id=$2 WHERE id=$1 AND assigned_runner_id IS NULL`, rec.RunID, runnerID)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, assignmentFromRecord(rec))
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return assignments, tx.Commit(ctx)
}

func (s *Store) assignmentForRun(ctx context.Context, tx pgx.Tx, runID, codexPath string) (*RunAssignPayload, error) {
	var rec queuedRunRecord
	err := tx.QueryRow(ctx, `
		SELECT r.id, r.task_id, p.id, p.workdir, r.mode, r.codex_session_id, r.codex_model, r.codex_reasoning_effort, r.codex_service_tier, r.reminder_callback_enabled, r.generated_prompt, srv.runner_id
		FROM runs r
		JOIN tasks t ON t.id=r.task_id
		JOIN projects p ON p.id=t.project_id
		JOIN servers srv ON srv.id=p.server_id
		WHERE r.id=$1`, runID).
		Scan(&rec.RunID, &rec.TaskID, &rec.ProjectID, &rec.Workdir, &rec.Mode, &rec.CodexSessionID, &rec.CodexModel, &rec.ReasoningEffort, &rec.ServiceTier, &rec.ReminderCallbackEnabled, &rec.Prompt, &rec.RunnerID)
	if err != nil {
		return nil, dbErr(err)
	}
	return assignmentFromRecord(rec), nil
}

func assignmentFromRecord(rec queuedRunRecord) *RunAssignPayload {
	return &RunAssignPayload{
		RunID:                   rec.RunID,
		TaskID:                  rec.TaskID,
		ProjectID:               rec.ProjectID,
		Workdir:                 rec.Workdir,
		Mode:                    rec.Mode,
		CodexSessionID:          rec.CodexSessionID,
		CodexModel:              rec.CodexModel,
		ReasoningEffort:         rec.ReasoningEffort,
		ServiceTier:             rec.ServiceTier,
		ReminderCallbackEnabled: rec.ReminderCallbackEnabled,
		Prompt:                  rec.Prompt,
		Argv:                    buildArgv("codex", rec.Workdir, rec.Mode, rec.CodexSessionID, rec.CodexModel, rec.ReasoningEffort, rec.ServiceTier),
		TargetRunnerID:          rec.RunnerID,
	}
}

func insertRunEventTx(ctx context.Context, tx pgx.Tx, runID, eventType, stream string, payload any, occurredAt time.Time) (RunEvent, error) {
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	var raw json.RawMessage
	switch v := payload.(type) {
	case json.RawMessage:
		raw = v
	case []byte:
		raw = append(raw, v...)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return RunEvent{}, err
		}
		raw = b
	}
	row := tx.QueryRow(ctx, `
		WITH locked_run AS (
			SELECT id, task_id FROM runs WHERE id=$1 FOR UPDATE
		),
		next_seq AS (
			SELECT COALESCE(MAX(seq), 0) + 1 AS seq
			FROM run_events
			WHERE run_id=$1
		),
		inserted AS (
			INSERT INTO run_events (run_id, seq, event_type, stream, payload, occurred_at)
			SELECT locked_run.id, next_seq.seq, $2, $3, $4, $5
			FROM locked_run CROSS JOIN next_seq
			RETURNING id, run_id, seq, event_type, stream, payload, occurred_at, created_at
		)
		SELECT inserted.id, inserted.run_id, locked_run.task_id, inserted.seq, inserted.event_type,
		       inserted.stream, inserted.payload, inserted.occurred_at, inserted.created_at
		FROM inserted JOIN locked_run ON locked_run.id=inserted.run_id`,
		runID, eventType, stream, raw, occurredAt)
	return scanRunEvent(row)
}
