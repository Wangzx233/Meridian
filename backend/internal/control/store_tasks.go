package control

import (
	"context"
	"github.com/jackc/pgx/v5"
	"strings"
)

func (s *Store) ListTasks(ctx context.Context, projectID string, statuses []string) ([]Task, error) {
	if err := s.reconcileTaskStatuses(ctx, projectID, ""); err != nil {
		return nil, err
	}
	args := []any{projectID}
	query := taskSelectBase() + ` WHERE t.project_id=$1`
	if len(statuses) > 0 {
		args = append(args, statuses)
		query += ` AND t.status = ANY($2)`
	}
	query += ` ORDER BY t.created_at DESC`
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

func (s *Store) CreateTask(ctx context.Context, projectID string, in CreateTaskInput) (Task, error) {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(in.Title) == "" {
		return Task{}, ErrValidation
	}
	row := s.db.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO tasks (project_id, title, description, status)
			VALUES ($1, $2, $3, 'open')
			RETURNING id, project_id, title, description, status, codex_session_id, created_at, updated_at, completed_at, archived_at
		)
		SELECT i.id, i.project_id, i.title, i.description, i.status, i.codex_session_id,
		       ar.id AS active_run_id, i.created_at, i.updated_at, i.completed_at, i.archived_at
		FROM inserted i
		LEFT JOIN runs ar ON ar.task_id=i.id AND ar.status IN ('queued','running')`, projectID, in.Title, in.Description)
	return scanTask(row)
}

func (s *Store) GetTask(ctx context.Context, id string) (Task, error) {
	if err := s.reconcileTaskStatuses(ctx, "", id); err != nil {
		return Task{}, err
	}
	row := s.db.QueryRow(ctx, taskSelectBase()+` WHERE t.id=$1`, id)
	return scanTask(row)
}

func (s *Store) PatchTask(ctx context.Context, id string, in PatchTaskInput) (Task, error) {
	current, err := s.GetTask(ctx, id)
	if err != nil {
		return Task{}, err
	}
	if current.Status == TaskStatusArchived || current.Status == TaskStatusDone {
		return Task{}, ErrInvalidState
	}
	title := current.Title
	description := current.Description
	status := current.Status
	if in.Title != nil {
		title = *in.Title
	}
	if in.Description != nil {
		description = *in.Description
	}
	if in.Status != nil {
		status = *in.Status
	}
	if status == TaskStatusRunning || status == TaskStatusDone || status == TaskStatusArchived {
		return Task{}, ErrInvalidState
	}
	row := s.db.QueryRow(ctx, `
		WITH updated AS (
			UPDATE tasks SET title=$2, description=$3, status=$4, updated_at=now()
			WHERE id=$1
			RETURNING id, project_id, title, description, status, codex_session_id, created_at, updated_at, completed_at, archived_at
		)
		SELECT u.id, u.project_id, u.title, u.description, u.status, u.codex_session_id,
		       ar.id AS active_run_id, u.created_at, u.updated_at, u.completed_at, u.archived_at
		FROM updated u
		LEFT JOIN runs ar ON ar.task_id=u.id AND ar.status IN ('queued','running')`, id, title, description, status)
	return scanTask(row)
}

func (s *Store) MarkTaskDone(ctx context.Context, id string, in MarkTaskDoneInput) (Task, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Task{}, err
	}
	defer rollback(ctx, tx)

	task, err := scanTask(tx.QueryRow(ctx, taskSelectBase()+` WHERE t.id=$1 FOR UPDATE OF t`, id))
	if err != nil {
		return Task{}, err
	}
	if task.Status != TaskStatusOpen && task.Status != TaskStatusWaitingUser {
		return Task{}, ErrInvalidState
	}
	task, err = scanTask(tx.QueryRow(ctx, `
		WITH updated AS (
			UPDATE tasks SET status='done', completed_at=now(), updated_at=now()
			WHERE id=$1
			RETURNING id, project_id, title, description, status, codex_session_id, created_at, updated_at, completed_at, archived_at
		)
		SELECT u.id, u.project_id, u.title, u.description, u.status, u.codex_session_id,
		       ar.id AS active_run_id, u.created_at, u.updated_at, u.completed_at, u.archived_at
		FROM updated u
		LEFT JOIN runs ar ON ar.task_id=u.id AND ar.status IN ('queued','running')`, id))
	if err != nil {
		return Task{}, err
	}
	memory := normalizeTaskMemoryInput(in)
	if taskMemoryHasContent(memory) {
		_, err = tx.Exec(ctx, `
			INSERT INTO task_memories (
				task_id, project_id, problem, root_cause, changes, files, decisions,
				verification, related_tasks, source_commit, stale_conditions
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			task.ID, task.ProjectID, memory.Problem, memory.RootCause, memory.Changes, memory.Files,
			memory.Decisions, memory.Verification, memory.RelatedTasks, memory.SourceCommit, memory.StaleConditions)
		if err != nil {
			return Task{}, err
		}
	}
	return task, tx.Commit(ctx)
}

func normalizeTaskMemoryInput(in MarkTaskDoneInput) TaskMemoryInput {
	memory := TaskMemoryInput{}
	if in.Memory != nil {
		memory = *in.Memory
	}
	if strings.TrimSpace(memory.Problem) == "" {
		memory.Problem = in.Summary
	}
	memory.Problem = strings.TrimSpace(memory.Problem)
	memory.RootCause = strings.TrimSpace(memory.RootCause)
	memory.Changes = strings.TrimSpace(memory.Changes)
	memory.Files = strings.TrimSpace(memory.Files)
	memory.Decisions = strings.TrimSpace(memory.Decisions)
	memory.Verification = strings.TrimSpace(memory.Verification)
	memory.RelatedTasks = strings.TrimSpace(memory.RelatedTasks)
	memory.SourceCommit = strings.TrimSpace(memory.SourceCommit)
	memory.StaleConditions = strings.TrimSpace(memory.StaleConditions)
	return memory
}

func taskMemoryHasContent(memory TaskMemoryInput) bool {
	return memory.Problem != "" ||
		memory.RootCause != "" ||
		memory.Changes != "" ||
		memory.Files != "" ||
		memory.Decisions != "" ||
		memory.Verification != "" ||
		memory.RelatedTasks != "" ||
		memory.SourceCommit != "" ||
		memory.StaleConditions != ""
}

func formatTaskMemorySummary(memory TaskMemoryInput) string {
	var b strings.Builder
	writeMemorySection := func(label, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(label)
		b.WriteString(":\n")
		b.WriteString(value)
	}
	writeMemorySection("Problem", memory.Problem)
	writeMemorySection("Root cause", memory.RootCause)
	writeMemorySection("Changes", memory.Changes)
	writeMemorySection("Files", memory.Files)
	writeMemorySection("Decisions", memory.Decisions)
	writeMemorySection("Verification", memory.Verification)
	writeMemorySection("Related tasks", memory.RelatedTasks)
	writeMemorySection("Source commit", memory.SourceCommit)
	writeMemorySection("Stale conditions", memory.StaleConditions)
	return b.String()
}

func (s *Store) ArchiveTask(ctx context.Context, id string) (Task, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Task{}, err
	}
	defer rollback(ctx, tx)
	task, err := scanTask(tx.QueryRow(ctx, taskSelectBase()+` WHERE t.id=$1 FOR UPDATE OF t`, id))
	if err != nil {
		return Task{}, err
	}
	if task.Status == TaskStatusRunning {
		return Task{}, ErrInvalidState
	}
	task, err = scanTask(tx.QueryRow(ctx, `
		WITH updated AS (
			UPDATE tasks SET status='archived', archived_at=now(), updated_at=now()
			WHERE id=$1
			RETURNING id, project_id, title, description, status, codex_session_id, created_at, updated_at, completed_at, archived_at
		)
		SELECT u.id, u.project_id, u.title, u.description, u.status, u.codex_session_id,
		       ar.id AS active_run_id, u.created_at, u.updated_at, u.completed_at, u.archived_at
		FROM updated u
		LEFT JOIN runs ar ON ar.task_id=u.id AND ar.status IN ('queued','running')`, id))
	if err != nil {
		return Task{}, err
	}
	return task, tx.Commit(ctx)
}

func (s *Store) taskInProject(ctx context.Context, taskID, projectID string) (Task, error) {
	task, err := s.GetTask(ctx, taskID)
	if err != nil {
		return Task{}, err
	}
	if task.ProjectID != projectID {
		return Task{}, ErrValidation
	}
	return task, nil
}

func (s *Store) reconcileTaskStatuses(ctx context.Context, projectID, taskID string) error {
	if strings.TrimSpace(projectID) == "" && strings.TrimSpace(taskID) == "" {
		return nil
	}
	if strings.TrimSpace(taskID) != "" {
		_, err := s.db.Exec(ctx, `
			UPDATE tasks t
			SET status='waiting_user', updated_at=now()
			WHERE t.id=$1
			  AND t.status='running'
			  AND NOT EXISTS (
			    SELECT 1 FROM runs r
			    WHERE r.task_id=t.id AND r.status IN ('queued','running')
			  )`, taskID)
		return err
	}
	_, err := s.db.Exec(ctx, `
		UPDATE tasks t
		SET status='waiting_user', updated_at=now()
		WHERE t.project_id=$1
		  AND t.status='running'
		  AND NOT EXISTS (
		    SELECT 1 FROM runs r
		    WHERE r.task_id=t.id AND r.status IN ('queued','running')
		  )`, projectID)
	return err
}

func taskSelectBase() string {
	return `
		SELECT t.id, t.project_id, t.title, t.description, t.status, t.codex_session_id,
		       ar.id AS active_run_id, t.created_at, t.updated_at, t.completed_at, t.archived_at
		FROM tasks t
		LEFT JOIN runs ar ON ar.task_id=t.id AND ar.status IN ('queued','running')`
}
