package control

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

func (s *Store) CreateRun(ctx context.Context, in CreateRunInput) (CreateRunResult, error) {
	if strings.TrimSpace(in.TaskID) == "" || strings.TrimSpace(in.Message) == "" {
		return CreateRunResult{}, ErrValidation
	}
	if in.Mode == "" {
		in.Mode = "auto"
	}
	if in.RawCommand {
		if in.Mode == "auto" {
			in.Mode = RunModeResume
		}
		if in.Mode != RunModeResume {
			return CreateRunResult{}, ErrValidation
		}
	}
	if in.Mode != "auto" && in.Mode != RunModeNew && in.Mode != RunModeResume {
		return CreateRunResult{}, ErrValidation
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CreateRunResult{}, err
	}
	defer rollback(ctx, tx)

	task, err := scanTask(tx.QueryRow(ctx, taskSelectBase()+` WHERE t.id=$1 FOR UPDATE OF t`, in.TaskID))
	if err != nil {
		return CreateRunResult{}, err
	}
	if task.CodexSessionID == nil || strings.TrimSpace(*task.CodexSessionID) == "" {
		var recoveredSession string
		err = tx.QueryRow(ctx, `
			SELECT codex_session_id
			FROM runs
			WHERE task_id=$1 AND codex_session_id IS NOT NULL AND codex_session_id <> ''
			ORDER BY created_at DESC
			LIMIT 1`, task.ID).Scan(&recoveredSession)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE tasks SET codex_session_id=$2, updated_at=now() WHERE id=$1`, task.ID, recoveredSession)
			if err != nil {
				return CreateRunResult{}, err
			}
			task.CodexSessionID = &recoveredSession
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return CreateRunResult{}, err
		}
	}
	if strings.TrimSpace(in.IdempotencyKey) != "" {
		existing, found, err := s.findIdempotentRun(ctx, tx, task.ID, strings.TrimSpace(in.IdempotencyKey))
		if err != nil {
			return CreateRunResult{}, err
		}
		if found {
			return CreateRunResult{Run: existing, Task: task}, tx.Commit(ctx)
		}
	}
	if err := validateCreateRunState(task); err != nil {
		return CreateRunResult{}, err
	}

	var activeID string
	err = tx.QueryRow(ctx, `SELECT id FROM runs WHERE task_id=$1 AND status IN ('queued','running') LIMIT 1`, in.TaskID).Scan(&activeID)
	if err == nil {
		return CreateRunResult{}, ErrActiveRunExists
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return CreateRunResult{}, err
	}

	mode, err := resolveCreateRunMode(in.Mode, task)
	if err != nil {
		return CreateRunResult{}, err
	}
	codexModel, reasoningEffort, serviceTier, err := normalizeCodexRunOptions(in.CodexModel, in.ReasoningEffort, in.ServiceTier)
	if err != nil {
		return CreateRunResult{}, err
	}
	if in.RawCommand && (strings.TrimSpace(in.Message) != "/compact" || len(in.ContextItemIDs) > 0) {
		return CreateRunResult{}, ErrValidation
	}

	snapshots, err := s.loadContextSnapshots(ctx, tx, task, in.ContextItemIDs)
	if err != nil {
		return CreateRunResult{}, err
	}
	prompt, err := buildRunPrompt(mode, task, in.Message, snapshots, in.RawCommand)
	if err != nil {
		return CreateRunResult{}, err
	}
	userMessage := in.Message
	if in.RawCommand {
		userMessage = strings.TrimSpace(in.Message)
	}

	run, err := scanRun(tx.QueryRow(ctx, `
		INSERT INTO runs (task_id, mode, status, user_message, generated_prompt, codex_model, codex_reasoning_effort, codex_service_tier, raw_command, codex_session_id, started_at, idempotency_key)
		VALUES ($1, $2, 'queued', $3, $4, $5, $6, $7, $8, CASE WHEN $2='resume' THEN $9 ELSE NULL END, now(), NULLIF($10, ''))
		RETURNING id, task_id, mode, status, user_message, generated_prompt, codex_model, codex_reasoning_effort, codex_service_tier, raw_command, final_message, codex_session_id,
		          assigned_runner_id, exit_code, error_message, cancel_requested_at, runner_started_at,
		          started_at, ended_at, created_at`,
		task.ID, mode, userMessage, prompt, codexModel, reasoningEffort, serviceTier, in.RawCommand, task.CodexSessionID, strings.TrimSpace(in.IdempotencyKey)))
	if err != nil {
		if isUniqueActiveRunViolation(err) {
			return CreateRunResult{}, ErrActiveRunExists
		}
		return CreateRunResult{}, err
	}
	for i, item := range snapshots {
		_, err = tx.Exec(ctx, `
			INSERT INTO run_context_items (run_id, context_item_id, order_index, type_snapshot, title_snapshot, content_snapshot)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			run.ID, item.ID, i, item.Type, item.Title, item.Content)
		if err != nil {
			return CreateRunResult{}, err
		}
	}
	task, err = scanTask(tx.QueryRow(ctx, `
		WITH updated AS (
			UPDATE tasks SET status='running', updated_at=now()
			WHERE id=$1
			RETURNING id, project_id, title, description, status, codex_session_id, created_at, updated_at, completed_at, archived_at
		)
		SELECT u.id, u.project_id, u.title, u.description, u.status, u.codex_session_id,
		       ar.id AS active_run_id, u.created_at, u.updated_at, u.completed_at, u.archived_at
		FROM updated u
		LEFT JOIN runs ar ON ar.task_id=u.id AND ar.status IN ('queued','running')`, task.ID))
	if err != nil {
		return CreateRunResult{}, err
	}
	if _, err = insertRunEventTx(ctx, tx, run.ID, EventRunState, StreamSystem, map[string]any{
		"status":          RunStatusQueued,
		"previous_status": nil,
	}, time.Now().UTC()); err != nil {
		return CreateRunResult{}, err
	}

	assignment, err := s.assignmentForRun(ctx, tx, run.ID, "")
	if err != nil {
		return CreateRunResult{}, err
	}
	if assignment != nil && assignment.TargetRunnerID != "" {
		_, err = tx.Exec(ctx, `UPDATE runs SET assigned_runner_id=$2 WHERE id=$1 AND assigned_runner_id IS NULL`, run.ID, assignment.TargetRunnerID)
		if err != nil {
			return CreateRunResult{}, err
		}
		run.AssignedRunnerID = &assignment.TargetRunnerID
	}
	if err := tx.Commit(ctx); err != nil {
		if isUniqueActiveRunViolation(err) {
			return CreateRunResult{}, ErrActiveRunExists
		}
		return CreateRunResult{}, err
	}
	return CreateRunResult{Run: run, Task: task, Assign: assignment}, nil
}

func (s *Store) InterruptRun(ctx context.Context, in CreateRunInput, reason string) (InterruptRunResult, error) {
	if strings.TrimSpace(in.TaskID) == "" || strings.TrimSpace(in.Message) == "" {
		return InterruptRunResult{}, ErrValidation
	}
	if in.Mode == "" {
		in.Mode = "auto"
	}
	if in.RawCommand {
		if in.Mode == "auto" {
			in.Mode = RunModeResume
		}
		if in.Mode != RunModeResume {
			return InterruptRunResult{}, ErrValidation
		}
	}
	if in.Mode != "auto" && in.Mode != RunModeNew && in.Mode != RunModeResume {
		return InterruptRunResult{}, ErrValidation
	}
	if strings.TrimSpace(reason) == "" {
		reason = "Interrupted by a newer user instruction."
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return InterruptRunResult{}, err
	}
	defer rollback(ctx, tx)

	task, err := scanTask(tx.QueryRow(ctx, taskSelectBase()+` WHERE t.id=$1 FOR UPDATE OF t`, in.TaskID))
	if err != nil {
		return InterruptRunResult{}, err
	}
	if task.Status == TaskStatusDone || task.Status == TaskStatusArchived {
		return InterruptRunResult{}, ErrInvalidState
	}
	if task.CodexSessionID == nil || strings.TrimSpace(*task.CodexSessionID) == "" {
		var recoveredSession string
		err = tx.QueryRow(ctx, `
			SELECT codex_session_id
			FROM runs
			WHERE task_id=$1 AND codex_session_id IS NOT NULL AND codex_session_id <> ''
			ORDER BY created_at DESC
			LIMIT 1`, task.ID).Scan(&recoveredSession)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE tasks SET codex_session_id=$2, updated_at=now() WHERE id=$1`, task.ID, recoveredSession)
			if err != nil {
				return InterruptRunResult{}, err
			}
			task.CodexSessionID = &recoveredSession
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return InterruptRunResult{}, err
		}
	}

	activeRun, err := s.activeRunForTask(ctx, tx, task.ID)
	if err != nil {
		return InterruptRunResult{}, err
	}
	now := time.Now().UTC()
	cancelQueuedUnassigned := activeRun.Status == RunStatusQueued && activeRun.AssignedRunnerID == nil
	activeRun.Status = RunStatusCanceled
	activeRun.CancelRequestedAt = &now
	activeRun.EndedAt = &now
	activeRun.ErrorMessage = &reason
	_, err = tx.Exec(ctx, `
		UPDATE runs
		SET status='canceled', cancel_requested_at=$2, ended_at=$2, error_message=$3
		WHERE id=$1 AND status IN ('queued','running')`, activeRun.ID, now, reason)
	if err != nil {
		return InterruptRunResult{}, err
	}
	cancelEvent, err := insertRunEventTx(ctx, tx, activeRun.ID, EventRunFinal, StreamSystem, map[string]any{
		"status":           RunStatusCanceled,
		"exit_code":        nil,
		"final_message":    nil,
		"error_message":    reason,
		"codex_session_id": activeRun.CodexSessionID,
	}, now)
	if err != nil {
		return InterruptRunResult{}, err
	}

	mode, err := resolveCreateRunMode(in.Mode, task)
	if err != nil {
		return InterruptRunResult{}, err
	}
	codexModel, reasoningEffort, serviceTier, err := normalizeCodexRunOptions(in.CodexModel, in.ReasoningEffort, in.ServiceTier)
	if err != nil {
		return InterruptRunResult{}, err
	}
	if in.RawCommand && (strings.TrimSpace(in.Message) != "/compact" || len(in.ContextItemIDs) > 0) {
		return InterruptRunResult{}, ErrValidation
	}
	snapshots, err := s.loadContextSnapshots(ctx, tx, task, in.ContextItemIDs)
	if err != nil {
		return InterruptRunResult{}, err
	}
	prompt, err := buildRunPrompt(mode, task, in.Message, snapshots, in.RawCommand)
	if err != nil {
		return InterruptRunResult{}, err
	}
	userMessage := in.Message
	if in.RawCommand {
		userMessage = strings.TrimSpace(in.Message)
	}
	run, err := scanRun(tx.QueryRow(ctx, `
		INSERT INTO runs (task_id, mode, status, user_message, generated_prompt, codex_model, codex_reasoning_effort, codex_service_tier, raw_command, codex_session_id, started_at, idempotency_key)
		VALUES ($1, $2, 'queued', $3, $4, $5, $6, $7, $8, CASE WHEN $2='resume' THEN $9 ELSE NULL END, $10, NULLIF($11, ''))
		RETURNING id, task_id, mode, status, user_message, generated_prompt, codex_model, codex_reasoning_effort, codex_service_tier, raw_command, final_message, codex_session_id,
		          assigned_runner_id, exit_code, error_message, cancel_requested_at, runner_started_at,
		          started_at, ended_at, created_at`,
		task.ID, mode, userMessage, prompt, codexModel, reasoningEffort, serviceTier, in.RawCommand, task.CodexSessionID, now, strings.TrimSpace(in.IdempotencyKey)))
	if err != nil {
		if isUniqueActiveRunViolation(err) {
			return InterruptRunResult{}, ErrActiveRunExists
		}
		return InterruptRunResult{}, err
	}
	for i, item := range snapshots {
		_, err = tx.Exec(ctx, `
			INSERT INTO run_context_items (run_id, context_item_id, order_index, type_snapshot, title_snapshot, content_snapshot)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			run.ID, item.ID, i, item.Type, item.Title, item.Content)
		if err != nil {
			return InterruptRunResult{}, err
		}
	}
	task, err = scanTask(tx.QueryRow(ctx, `
		WITH updated AS (
			UPDATE tasks SET status='running', updated_at=now()
			WHERE id=$1
			RETURNING id, project_id, title, description, status, codex_session_id, created_at, updated_at, completed_at, archived_at
		)
		SELECT u.id, u.project_id, u.title, u.description, u.status, u.codex_session_id,
		       ar.id AS active_run_id, u.created_at, u.updated_at, u.completed_at, u.archived_at
		FROM updated u
		LEFT JOIN runs ar ON ar.task_id=u.id AND ar.status IN ('queued','running')`, task.ID))
	if err != nil {
		return InterruptRunResult{}, err
	}
	if _, err = insertRunEventTx(ctx, tx, run.ID, EventRunState, StreamSystem, map[string]any{
		"status":          RunStatusQueued,
		"previous_status": nil,
	}, now); err != nil {
		return InterruptRunResult{}, err
	}
	assignment, err := s.assignmentForRun(ctx, tx, run.ID, "")
	if err != nil {
		return InterruptRunResult{}, err
	}
	if assignment != nil && assignment.TargetRunnerID != "" {
		_, err = tx.Exec(ctx, `UPDATE runs SET assigned_runner_id=$2 WHERE id=$1 AND assigned_runner_id IS NULL`, run.ID, assignment.TargetRunnerID)
		if err != nil {
			return InterruptRunResult{}, err
		}
		run.AssignedRunnerID = &assignment.TargetRunnerID
	}
	if err := tx.Commit(ctx); err != nil {
		if isUniqueActiveRunViolation(err) {
			return InterruptRunResult{}, ErrActiveRunExists
		}
		return InterruptRunResult{}, err
	}

	cancel := &RunCancelPayload{RunID: activeRun.ID, Reason: reason, RequestedAt: now}
	if cancelQueuedUnassigned {
		cancel = nil
	}
	return InterruptRunResult{
		Run:         run,
		Task:        task,
		Assign:      assignment,
		CanceledRun: activeRun,
		Cancel:      cancel,
		CancelEvent: cancelEvent,
	}, nil
}

func (s *Store) findIdempotentRun(ctx context.Context, tx pgx.Tx, taskID, key string) (Run, bool, error) {
	row := tx.QueryRow(ctx, `
		SELECT id, task_id, mode, status, user_message, generated_prompt, codex_model, codex_reasoning_effort, codex_service_tier, raw_command, final_message, codex_session_id,
		       assigned_runner_id, exit_code, error_message, cancel_requested_at, runner_started_at,
		       started_at, ended_at, created_at
		FROM runs
		WHERE task_id=$1 AND idempotency_key=$2`, taskID, key)
	run, err := scanRun(row)
	if errors.Is(err, ErrNotFound) {
		return Run{}, false, nil
	}
	if err != nil {
		return Run{}, false, err
	}
	return run, true, nil
}

func (s *Store) activeRunForTask(ctx context.Context, tx pgx.Tx, taskID string) (Run, error) {
	row := tx.QueryRow(ctx, `
		SELECT id, task_id, mode, status, user_message, generated_prompt, codex_model, codex_reasoning_effort, codex_service_tier, raw_command, final_message, codex_session_id,
		       assigned_runner_id, exit_code, error_message, cancel_requested_at, runner_started_at,
		       started_at, ended_at, created_at
		FROM runs
		WHERE task_id=$1 AND status IN ('queued','running')
		ORDER BY created_at DESC
		LIMIT 1
		FOR UPDATE`, taskID)
	run, err := scanRun(row)
	if errors.Is(err, ErrNotFound) {
		return Run{}, ErrActiveRunMissing
	}
	return run, err
}

func (s *Store) loadContextSnapshots(ctx context.Context, tx pgx.Tx, task Task, ids []string) ([]contextSnapshot, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var taskServerID string
	if err := tx.QueryRow(ctx, `SELECT p.server_id FROM projects p WHERE p.id=$1`, task.ProjectID).Scan(&taskServerID); err != nil {
		return nil, dbErr(err)
	}
	rows, err := tx.Query(ctx, `
		SELECT id, type, title, content, server_id, project_id, task_id, scope
		FROM context_items
		WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	found := map[string]struct {
		snapshot contextSnapshot
		server   *string
		project  *string
		taskID   *string
		scope    string
	}{}
	for rows.Next() {
		var rec struct {
			snapshot contextSnapshot
			server   *string
			project  *string
			taskID   *string
			scope    string
		}
		if err := rows.Scan(&rec.snapshot.ID, &rec.snapshot.Type, &rec.snapshot.Title, &rec.snapshot.Content, &rec.server, &rec.project, &rec.taskID, &rec.scope); err != nil {
			return nil, err
		}
		found[rec.snapshot.ID] = rec
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	out := make([]contextSnapshot, 0, len(ids))
	for _, id := range ids {
		rec, ok := found[id]
		if !ok {
			return nil, ErrNotFound
		}
		if !contextVisibleToTask(rec.scope, rec.server, rec.project, rec.taskID, taskServerID, task.ProjectID, task.ID) {
			return nil, ErrValidation
		}
		out = append(out, rec.snapshot)
	}
	return out, nil
}
