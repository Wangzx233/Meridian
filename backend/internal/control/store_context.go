package control

import (
	"context"
	"strings"
)

func (s *Store) ListContextItems(ctx context.Context, projectID, taskID string) ([]ContextItem, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, ErrValidation
	}
	project, err := s.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT id, server_id, project_id, task_id, scope, type, title, content, tags, created_at, updated_at
		FROM context_items
		WHERE scope='global'
		   OR (scope='server' AND server_id=$1)
		   OR (scope='project' AND project_id=$2)`
	args := []any{project.ServerID, project.ID}
	if taskID != "" {
		query += ` OR (scope='task' AND project_id=$2 AND task_id=$3)`
		args = append(args, taskID)
	}
	query += `
		ORDER BY CASE scope
		  WHEN 'global' THEN 0
		  WHEN 'server' THEN 1
		  WHEN 'project' THEN 2
		  WHEN 'task' THEN 3
		  ELSE 4
		END, created_at DESC`
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanContextItems(rows)
}

func (s *Store) CreateContextItem(ctx context.Context, projectID string, in CreateContextInput) (ContextItem, error) {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(in.Scope) == "" || strings.TrimSpace(in.Type) == "" ||
		strings.TrimSpace(in.Title) == "" {
		return ContextItem{}, ErrValidation
	}
	serverID, ownerProjectID, taskID, err := s.contextOwnersForProjectScope(ctx, projectID, in.Scope, in.TaskID)
	if err != nil {
		return ContextItem{}, err
	}
	row := s.db.QueryRow(ctx, `
		INSERT INTO context_items (server_id, project_id, task_id, scope, type, title, content, tags)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, server_id, project_id, task_id, scope, type, title, content, tags, created_at, updated_at`,
		serverID, ownerProjectID, taskID, in.Scope, in.Type, in.Title, in.Content, in.Tags)
	return scanContextItem(row)
}

func (s *Store) GetContextItem(ctx context.Context, id string) (ContextItem, error) {
	row := s.db.QueryRow(ctx, `
		SELECT id, server_id, project_id, task_id, scope, type, title, content, tags, created_at, updated_at
		FROM context_items WHERE id=$1`, id)
	return scanContextItem(row)
}

func (s *Store) PatchContextItem(ctx context.Context, id string, in PatchContextInput) (ContextItem, error) {
	current, err := s.GetContextItem(ctx, id)
	if err != nil {
		return ContextItem{}, err
	}
	scope := current.Scope
	serverID := current.ServerID
	projectID := current.ProjectID
	taskID := current.TaskID
	typ := current.Type
	title := current.Title
	content := current.Content
	tags := current.Tags
	if in.Scope != nil {
		scope = *in.Scope
	}
	if in.Scope != nil || in.ServerID != nil || in.ProjectID != nil || in.TaskID != nil {
		var err error
		serverID, projectID, taskID, err = s.contextOwnersForPatch(ctx, scope, current, in)
		if err != nil {
			return ContextItem{}, err
		}
	}
	if in.Type != nil {
		typ = *in.Type
	}
	if in.Title != nil {
		title = *in.Title
	}
	if in.Content != nil {
		content = *in.Content
	}
	if in.TagsSet {
		tags = in.Tags
	}
	row := s.db.QueryRow(ctx, `
		UPDATE context_items SET server_id=$2, project_id=$3, task_id=$4, scope=$5, type=$6, title=$7, content=$8, tags=$9, updated_at=now()
		WHERE id=$1
		RETURNING id, server_id, project_id, task_id, scope, type, title, content, tags, created_at, updated_at`,
		id, serverID, projectID, taskID, scope, typ, title, content, tags)
	return scanContextItem(row)
}

func (s *Store) DeleteContextItem(ctx context.Context, id string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM context_items WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) contextOwnersForProjectScope(ctx context.Context, projectID, scope string, taskID *string) (*string, *string, *string, error) {
	project, err := s.GetProject(ctx, projectID)
	if err != nil {
		return nil, nil, nil, err
	}
	serverID := project.ServerID
	switch scope {
	case "global":
		if taskID != nil {
			return nil, nil, nil, ErrValidation
		}
		return nil, nil, nil, nil
	case "server":
		if taskID != nil {
			return nil, nil, nil, ErrValidation
		}
		return &serverID, nil, nil, nil
	case "project":
		if taskID != nil {
			return nil, nil, nil, ErrValidation
		}
		return &serverID, &project.ID, nil, nil
	case "task":
		if taskID == nil || strings.TrimSpace(*taskID) == "" {
			return nil, nil, nil, ErrValidation
		}
		if _, err := s.taskInProject(ctx, *taskID, project.ID); err != nil {
			return nil, nil, nil, err
		}
		return &serverID, &project.ID, taskID, nil
	default:
		return nil, nil, nil, ErrValidation
	}
}

func (s *Store) contextOwnersForPatch(ctx context.Context, scope string, current ContextItem, in PatchContextInput) (*string, *string, *string, error) {
	taskID := current.TaskID
	if in.TaskID != nil {
		taskID = in.TaskID
	}

	switch scope {
	case "global":
		return nil, nil, nil, nil
	case "server":
		serverID := firstNonNilString(in.ServerID, current.ServerID)
		if serverID == nil || strings.TrimSpace(*serverID) == "" {
			return nil, nil, nil, ErrValidation
		}
		if _, err := s.GetServer(ctx, *serverID); err != nil {
			return nil, nil, nil, err
		}
		return serverID, nil, nil, nil
	case "project":
		projectID := firstNonNilString(in.ProjectID, current.ProjectID)
		if projectID == nil || strings.TrimSpace(*projectID) == "" {
			return nil, nil, nil, ErrValidation
		}
		project, err := s.GetProject(ctx, *projectID)
		if err != nil {
			return nil, nil, nil, err
		}
		return &project.ServerID, &project.ID, nil, nil
	case "task":
		projectID := firstNonNilString(in.ProjectID, current.ProjectID)
		if projectID == nil || taskID == nil || strings.TrimSpace(*projectID) == "" || strings.TrimSpace(*taskID) == "" {
			return nil, nil, nil, ErrValidation
		}
		project, err := s.GetProject(ctx, *projectID)
		if err != nil {
			return nil, nil, nil, err
		}
		if _, err := s.taskInProject(ctx, *taskID, project.ID); err != nil {
			return nil, nil, nil, err
		}
		return &project.ServerID, &project.ID, taskID, nil
	default:
		return nil, nil, nil, ErrValidation
	}
}

func contextVisibleToTask(scope string, serverID, projectID, taskID *string, taskServerID, taskProjectID, currentTaskID string) bool {
	switch scope {
	case "global":
		return true
	case "server":
		return serverID != nil && *serverID == taskServerID
	case "project":
		return projectID != nil && *projectID == taskProjectID
	case "task":
		return projectID != nil && *projectID == taskProjectID && taskID != nil && *taskID == currentTaskID
	default:
		return false
	}
}
