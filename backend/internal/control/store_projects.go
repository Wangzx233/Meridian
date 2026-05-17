package control

import (
	"context"
	"github.com/jackc/pgx/v5"
	"strings"
)

func (s *Store) ListProjects(ctx context.Context, serverID string) ([]Project, error) {
	var rows pgx.Rows
	var err error
	if serverID == "" {
		rows, err = s.db.Query(ctx, `SELECT id, server_id, name, workdir, default_branch, rules_path, created_at, updated_at FROM projects ORDER BY created_at DESC`)
	} else {
		rows, err = s.db.Query(ctx, `SELECT id, server_id, name, workdir, default_branch, rules_path, created_at, updated_at FROM projects WHERE server_id=$1 ORDER BY created_at DESC`, serverID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProjects(rows)
}

func (s *Store) CreateProject(ctx context.Context, in CreateProjectInput) (Project, error) {
	if strings.TrimSpace(in.ServerID) == "" || strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Workdir) == "" {
		return Project{}, ErrValidation
	}
	row := s.db.QueryRow(ctx, `
		INSERT INTO projects (server_id, name, workdir, default_branch, rules_path)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, server_id, name, workdir, default_branch, rules_path, created_at, updated_at`,
		in.ServerID, in.Name, in.Workdir, in.DefaultBranch, in.RulesPath)
	return scanProject(row)
}

func (s *Store) GetProject(ctx context.Context, id string) (Project, error) {
	row := s.db.QueryRow(ctx, `SELECT id, server_id, name, workdir, default_branch, rules_path, created_at, updated_at FROM projects WHERE id=$1`, id)
	return scanProject(row)
}

func (s *Store) PatchProject(ctx context.Context, id string, in PatchProjectInput) (Project, error) {
	current, err := s.GetProject(ctx, id)
	if err != nil {
		return Project{}, err
	}
	serverID := current.ServerID
	name := current.Name
	workdir := current.Workdir
	defaultBranch := current.DefaultBranch
	rulesPath := current.RulesPath
	if in.ServerID != nil {
		serverID = *in.ServerID
	}
	if in.Name != nil {
		name = *in.Name
	}
	if in.Workdir != nil {
		workdir = *in.Workdir
	}
	if in.DefaultBranch != nil {
		defaultBranch = in.DefaultBranch
	}
	if in.RulesPath != nil {
		rulesPath = in.RulesPath
	}
	row := s.db.QueryRow(ctx, `
		UPDATE projects SET server_id=$2, name=$3, workdir=$4, default_branch=$5, rules_path=$6, updated_at=now()
		WHERE id=$1
		RETURNING id, server_id, name, workdir, default_branch, rules_path, created_at, updated_at`,
		id, serverID, name, workdir, defaultBranch, rulesPath)
	return scanProject(row)
}
