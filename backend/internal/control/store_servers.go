package control

import (
	"context"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

func (s *Store) ListServers(ctx context.Context) ([]Server, error) {
	rows, err := s.db.Query(ctx, `SELECT id, name, alias, runner_id, status, last_heartbeat_at, created_at, updated_at FROM servers ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanServers(rows)
}

func (s *Store) CreateServer(ctx context.Context, in CreateServerInput) (Server, error) {
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.RunnerID) == "" {
		return Server{}, ErrValidation
	}
	alias := normalizeServerAlias(in.Alias)
	row := s.db.QueryRow(ctx, `
		INSERT INTO servers (name, alias, runner_id, status)
		VALUES ($1, $2, $3, 'offline')
		RETURNING id, name, alias, runner_id, status, last_heartbeat_at, created_at, updated_at`,
		in.Name, serverAliasValue(alias), in.RunnerID)
	return scanServer(row)
}

func (s *Store) GetServer(ctx context.Context, id string) (Server, error) {
	row := s.db.QueryRow(ctx, `SELECT id, name, alias, runner_id, status, last_heartbeat_at, created_at, updated_at FROM servers WHERE id=$1`, id)
	return scanServer(row)
}

func (s *Store) PatchServer(ctx context.Context, id string, in PatchServerInput) (Server, error) {
	current, err := s.GetServer(ctx, id)
	if err != nil {
		return Server{}, err
	}
	name := current.Name
	alias := current.Alias
	runnerID := current.RunnerID
	status := current.Status
	if in.Name != nil {
		if strings.TrimSpace(*in.Name) == "" {
			return Server{}, ErrValidation
		}
		name = *in.Name
	}
	if in.Alias != nil {
		alias = normalizeServerAlias(in.Alias)
	}
	if in.RunnerID != nil {
		if strings.TrimSpace(*in.RunnerID) == "" {
			return Server{}, ErrValidation
		}
		runnerID = *in.RunnerID
	}
	if in.Status != nil {
		if *in.Status != "online" && *in.Status != "offline" {
			return Server{}, ErrValidation
		}
		status = *in.Status
	}
	row := s.db.QueryRow(ctx, `
		UPDATE servers SET name=$2, alias=$3, runner_id=$4, status=$5, updated_at=now()
		WHERE id=$1
		RETURNING id, name, alias, runner_id, status, last_heartbeat_at, created_at, updated_at`,
		id, name, serverAliasValue(alias), runnerID, status)
	return scanServer(row)
}

func (s *Store) DeleteServer(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)

	_, err = tx.Exec(ctx, `
		DELETE FROM run_context_items rci
		USING runs r, tasks t, projects p
		WHERE rci.run_id=r.id
		  AND r.task_id=t.id
		  AND t.project_id=p.id
		  AND p.server_id=$1`, id)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		DELETE FROM run_context_items rci
		USING context_items ci
		WHERE rci.context_item_id=ci.id
		  AND ci.server_id=$1`, id)
	if err != nil {
		return err
	}

	tag, err := tx.Exec(ctx, `DELETE FROM servers WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

func (s *Store) UpsertRunnerHeartbeat(ctx context.Context, runnerID, fallbackName string) error {
	if strings.TrimSpace(runnerID) == "" {
		return ErrValidation
	}
	if strings.TrimSpace(fallbackName) == "" {
		fallbackName = runnerID
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO servers (name, runner_id, status, last_heartbeat_at)
		VALUES ($1, $2, 'online', now())
		ON CONFLICT (runner_id) DO UPDATE
		SET status='online', last_heartbeat_at=now(), updated_at=now()`,
		fallbackName, runnerID)
	return err
}

func normalizeServerAlias(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func serverAliasValue(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func (s *Store) MarkRunnerOffline(ctx context.Context, runnerID string) error {
	if strings.TrimSpace(runnerID) == "" {
		return ErrValidation
	}
	_, err := s.db.Exec(ctx, `
		UPDATE servers SET status='offline', updated_at=now()
		WHERE runner_id=$1`,
		runnerID)
	return err
}

func (s *Store) MarkRunnerOfflineIfStale(ctx context.Context, runnerID string, staleBefore time.Time) error {
	if strings.TrimSpace(runnerID) == "" {
		return ErrValidation
	}
	_, err := s.db.Exec(ctx, `
		UPDATE servers SET status='offline', updated_at=now()
		WHERE runner_id=$1
		  AND (last_heartbeat_at IS NULL OR last_heartbeat_at < $2)`,
		runnerID, staleBefore)
	return err
}
