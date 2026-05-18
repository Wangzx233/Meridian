package control

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func scanServer(row pgx.Row) (Server, error) {
	var s Server
	var alias pgtype.Text
	err := row.Scan(&s.ID, &s.Name, &alias, &s.RunnerID, &s.Status, &s.LastHeartbeatAt, &s.CreatedAt, &s.UpdatedAt)
	if alias.Valid {
		value := alias.String
		s.Alias = &value
	}
	return s, dbErr(err)
}

func scanServers(rows pgx.Rows) ([]Server, error) {
	var out []Server
	for rows.Next() {
		v, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func scanAuthUser(row pgx.Row) (AuthUser, error) {
	var item AuthUser
	err := row.Scan(&item.ID, &item.Username, &item.PasswordHash, &item.CreatedAt, &item.UpdatedAt)
	return item, dbErr(err)
}

func scanProject(row pgx.Row) (Project, error) {
	var p Project
	err := row.Scan(&p.ID, &p.ServerID, &p.Name, &p.Workdir, &p.DefaultBranch, &p.RulesPath, &p.CreatedAt, &p.UpdatedAt)
	return p, dbErr(err)
}

func scanProjects(rows pgx.Rows) ([]Project, error) {
	var out []Project
	for rows.Next() {
		v, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func scanTask(row pgx.Row) (Task, error) {
	var t Task
	err := row.Scan(&t.ID, &t.ProjectID, &t.Title, &t.Description, &t.Status, &t.CodexSessionID, &t.ActiveRunID, &t.CreatedAt, &t.UpdatedAt, &t.CompletedAt, &t.ArchivedAt)
	return t, dbErr(err)
}

func scanTasks(rows pgx.Rows) ([]Task, error) {
	var out []Task
	for rows.Next() {
		v, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func scanRun(row pgx.Row) (Run, error) {
	var r Run
	err := row.Scan(&r.ID, &r.TaskID, &r.Mode, &r.Status, &r.UserMessage, &r.GeneratedPrompt, &r.CodexModel, &r.ReasoningEffort, &r.ServiceTier, &r.RawCommand, &r.FinalMessage, &r.CodexSessionID,
		&r.AssignedRunnerID, &r.ExitCode, &r.ErrorMessage, &r.CancelRequestedAt, &r.RunnerStartedAt, &r.StartedAt, &r.EndedAt, &r.CreatedAt)
	return r, dbErr(err)
}

func scanRuns(rows pgx.Rows) ([]Run, error) {
	var out []Run
	for rows.Next() {
		v, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func scanContextItem(row pgx.Row) (ContextItem, error) {
	var c ContextItem
	err := row.Scan(&c.ID, &c.ServerID, &c.ProjectID, &c.TaskID, &c.Scope, &c.Type, &c.Title, &c.Content, &c.Tags, &c.CreatedAt, &c.UpdatedAt)
	return c, dbErr(err)
}

func scanContextItems(rows pgx.Rows) ([]ContextItem, error) {
	var out []ContextItem
	for rows.Next() {
		v, err := scanContextItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func scanRunEvent(row pgx.Row) (RunEvent, error) {
	var e RunEvent
	err := row.Scan(&e.ID, &e.RunID, &e.TaskID, &e.Seq, &e.EventType, &e.Stream, &e.Payload, &e.OccurredAt, &e.CreatedAt)
	return e, dbErr(err)
}

func scanRunEvents(rows pgx.Rows) ([]RunEvent, error) {
	var out []RunEvent
	for rows.Next() {
		v, err := scanRunEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
