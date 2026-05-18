package control

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	"github.com/jackc/pgx/v5"
)

const defaultEmailNotificationTLSMode = "starttls"

type CreateEmailNotificationConfigInput struct {
	Name          string   `json:"name"`
	Enabled       *bool    `json:"enabled"`
	SMTPHost      string   `json:"smtp_host"`
	SMTPPort      int      `json:"smtp_port"`
	SMTPUsername  string   `json:"smtp_username"`
	SMTPPassword  string   `json:"smtp_password"`
	FromAddress   string   `json:"from_address"`
	ToAddresses   []string `json:"to_addresses"`
	TLSMode       string   `json:"tls_mode"`
	SubjectPrefix string   `json:"subject_prefix"`
}

type PatchEmailNotificationConfigInput struct {
	Name           *string  `json:"name"`
	Enabled        *bool    `json:"enabled"`
	SMTPHost       *string  `json:"smtp_host"`
	SMTPPort       *int     `json:"smtp_port"`
	SMTPUsername   *string  `json:"smtp_username"`
	SMTPPassword   *string  `json:"smtp_password"`
	FromAddress    *string  `json:"from_address"`
	ToAddresses    []string `json:"to_addresses"`
	ToAddressesSet bool     `json:"-"`
	TLSMode        *string  `json:"tls_mode"`
	SubjectPrefix  *string  `json:"subject_prefix"`
}

func (s *Store) ListEmailNotificationConfigs(ctx context.Context, includePassword bool) ([]EmailNotificationConfig, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, name, enabled, smtp_host, smtp_port, smtp_username, smtp_password, from_address, to_addresses,
		       tls_mode, subject_prefix, created_at, updated_at
		FROM email_notification_configs
		ORDER BY updated_at DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items, err := scanEmailNotificationConfigs(rows)
	if err != nil {
		return nil, err
	}
	if !includePassword {
		for i := range items {
			items[i].SMTPPassword = ""
		}
	}
	return items, nil
}

func (s *Store) GetEmailNotificationConfig(ctx context.Context, id string, includePassword bool) (EmailNotificationConfig, error) {
	row := s.db.QueryRow(ctx, `
		SELECT id, name, enabled, smtp_host, smtp_port, smtp_username, smtp_password, from_address, to_addresses,
		       tls_mode, subject_prefix, created_at, updated_at
		FROM email_notification_configs
		WHERE id=$1`, id)
	item, err := scanEmailNotificationConfig(row)
	if err != nil {
		return EmailNotificationConfig{}, err
	}
	if !includePassword {
		item.SMTPPassword = ""
	}
	return item, nil
}

func (s *Store) CreateEmailNotificationConfig(ctx context.Context, in CreateEmailNotificationConfigInput) (EmailNotificationConfig, error) {
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	normalized, err := normalizeEmailNotificationConfig(EmailNotificationConfig{
		Name:          in.Name,
		Enabled:       enabled,
		SMTPHost:      in.SMTPHost,
		SMTPPort:      in.SMTPPort,
		SMTPUsername:  in.SMTPUsername,
		SMTPPassword:  in.SMTPPassword,
		FromAddress:   in.FromAddress,
		ToAddresses:   in.ToAddresses,
		TLSMode:       in.TLSMode,
		SubjectPrefix: in.SubjectPrefix,
	})
	if err != nil {
		return EmailNotificationConfig{}, err
	}
	row := s.db.QueryRow(ctx, `
		INSERT INTO email_notification_configs
			(name, enabled, smtp_host, smtp_port, smtp_username, smtp_password, from_address, to_addresses, tls_mode, subject_prefix)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, name, enabled, smtp_host, smtp_port, smtp_username, smtp_password, from_address, to_addresses,
		          tls_mode, subject_prefix, created_at, updated_at`,
		normalized.Name, normalized.Enabled, normalized.SMTPHost, normalized.SMTPPort, normalized.SMTPUsername,
		normalized.SMTPPassword, normalized.FromAddress, normalized.ToAddresses, normalized.TLSMode, normalized.SubjectPrefix)
	created, err := scanEmailNotificationConfig(row)
	if err != nil {
		return EmailNotificationConfig{}, err
	}
	created.SMTPPassword = ""
	return created, nil
}

func (s *Store) PatchEmailNotificationConfig(ctx context.Context, id string, in PatchEmailNotificationConfigInput) (EmailNotificationConfig, error) {
	current, err := s.GetEmailNotificationConfig(ctx, id, true)
	if err != nil {
		return EmailNotificationConfig{}, err
	}
	if in.Name != nil {
		current.Name = *in.Name
	}
	if in.Enabled != nil {
		current.Enabled = *in.Enabled
	}
	if in.SMTPHost != nil {
		current.SMTPHost = *in.SMTPHost
	}
	if in.SMTPPort != nil {
		current.SMTPPort = *in.SMTPPort
	}
	if in.SMTPUsername != nil {
		current.SMTPUsername = *in.SMTPUsername
	}
	if in.SMTPPassword != nil {
		current.SMTPPassword = *in.SMTPPassword
	}
	if in.FromAddress != nil {
		current.FromAddress = *in.FromAddress
	}
	if in.ToAddressesSet {
		current.ToAddresses = in.ToAddresses
	}
	if in.TLSMode != nil {
		current.TLSMode = *in.TLSMode
	}
	if in.SubjectPrefix != nil {
		current.SubjectPrefix = *in.SubjectPrefix
	}
	normalized, err := normalizeEmailNotificationConfig(current)
	if err != nil {
		return EmailNotificationConfig{}, err
	}
	row := s.db.QueryRow(ctx, `
		UPDATE email_notification_configs
		SET name=$2, enabled=$3, smtp_host=$4, smtp_port=$5, smtp_username=$6, smtp_password=$7,
		    from_address=$8, to_addresses=$9, tls_mode=$10, subject_prefix=$11, updated_at=now()
		WHERE id=$1
		RETURNING id, name, enabled, smtp_host, smtp_port, smtp_username, smtp_password, from_address, to_addresses,
		          tls_mode, subject_prefix, created_at, updated_at`,
		id, normalized.Name, normalized.Enabled, normalized.SMTPHost, normalized.SMTPPort, normalized.SMTPUsername,
		normalized.SMTPPassword, normalized.FromAddress, normalized.ToAddresses, normalized.TLSMode, normalized.SubjectPrefix)
	updated, err := scanEmailNotificationConfig(row)
	if err != nil {
		return EmailNotificationConfig{}, err
	}
	updated.SMTPPassword = ""
	return updated, nil
}

func (s *Store) DeleteEmailNotificationConfig(ctx context.Context, id string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM email_notification_configs WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListWorkbenchNotifications(ctx context.Context, pendingOnly bool) ([]WorkbenchNotification, error) {
	query := `
		SELECT n.id, n.type, n.server_id, COALESCE(NULLIF(s.alias, ''), s.name), n.project_id, p.name, n.task_id, t.title,
		       n.run_id, n.run_status, n.title, n.message, n.acknowledged_at, n.created_at
		FROM workbench_notifications n
		JOIN servers s ON s.id=n.server_id
		JOIN projects p ON p.id=n.project_id
		JOIN tasks t ON t.id=n.task_id`
	if pendingOnly {
		query += ` WHERE n.acknowledged_at IS NULL AND n.type <> 'task_done'`
	}
	query += ` ORDER BY n.created_at DESC LIMIT 100`
	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanWorkbenchNotifications(rows)
}

func (s *Store) CreateTaskDoneNotification(ctx context.Context, task Task) (WorkbenchNotification, error) {
	project, err := s.GetProject(ctx, task.ProjectID)
	if err != nil {
		return WorkbenchNotification{}, err
	}
	server, err := s.GetServer(ctx, project.ServerID)
	if err != nil {
		return WorkbenchNotification{}, err
	}
	title := fmt.Sprintf("Task done: %s", strings.TrimSpace(task.Title))
	message := fmt.Sprintf("%s / %s", strings.TrimSpace(project.Name), strings.TrimSpace(task.Title))
	row := s.db.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO workbench_notifications (type, server_id, project_id, task_id, title, message)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id, type, server_id, project_id, task_id, run_id, run_status, title, message, acknowledged_at, created_at
		)
		SELECT n.id, n.type, n.server_id, $7::text AS server_name, n.project_id, $8::text AS project_name,
		       n.task_id, $9::text AS task_title, n.run_id, n.run_status, n.title, n.message, n.acknowledged_at, n.created_at
		FROM inserted n`,
		NotificationTypeTaskDone, server.ID, project.ID, task.ID, title, message, serverDisplayName(server), project.Name, task.Title)
	return scanWorkbenchNotification(row)
}

func (s *Store) CreateRunFinishedNotification(ctx context.Context, run Run) (WorkbenchNotification, error) {
	task, err := s.GetTask(ctx, run.TaskID)
	if err != nil {
		return WorkbenchNotification{}, err
	}
	project, err := s.GetProject(ctx, task.ProjectID)
	if err != nil {
		return WorkbenchNotification{}, err
	}
	server, err := s.GetServer(ctx, project.ServerID)
	if err != nil {
		return WorkbenchNotification{}, err
	}
	statusLabel := run.Status
	switch run.Status {
	case RunStatusSucceeded:
		statusLabel = "succeeded"
	case RunStatusFailed:
		statusLabel = "failed"
	case RunStatusCanceled:
		statusLabel = "canceled"
	}
	title := fmt.Sprintf("Run %s: %s", statusLabel, strings.TrimSpace(task.Title))
	message := fmt.Sprintf("%s / %s", strings.TrimSpace(project.Name), strings.TrimSpace(task.Title))
	row := s.db.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO workbench_notifications (type, server_id, project_id, task_id, run_id, run_status, title, message)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id, type, server_id, project_id, task_id, run_id, run_status, title, message, acknowledged_at, created_at
		)
		SELECT n.id, n.type, n.server_id, $9::text AS server_name, n.project_id, $10::text AS project_name,
		       n.task_id, $11::text AS task_title, n.run_id, n.run_status, n.title, n.message, n.acknowledged_at, n.created_at
		FROM inserted n`,
		NotificationTypeRunFinished, server.ID, project.ID, task.ID, run.ID, run.Status, title, message, serverDisplayName(server), project.Name, task.Title)
	return scanWorkbenchNotification(row)
}

func (s *Store) AcknowledgeWorkbenchNotification(ctx context.Context, id string) (WorkbenchNotification, error) {
	row := s.db.QueryRow(ctx, `
		WITH updated AS (
			UPDATE workbench_notifications
			SET acknowledged_at=COALESCE(acknowledged_at, now())
			WHERE id=$1
			RETURNING id, type, server_id, project_id, task_id, run_id, run_status, title, message, acknowledged_at, created_at
		)
		SELECT n.id, n.type, n.server_id, COALESCE(NULLIF(s.alias, ''), s.name), n.project_id, p.name, n.task_id, t.title,
		       n.run_id, n.run_status, n.title, n.message, n.acknowledged_at, n.created_at
		FROM updated n
		JOIN servers s ON s.id=n.server_id
		JOIN projects p ON p.id=n.project_id
		JOIN tasks t ON t.id=n.task_id`, id)
	return scanWorkbenchNotification(row)
}

func normalizeEmailNotificationConfig(in EmailNotificationConfig) (EmailNotificationConfig, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.SMTPHost = strings.TrimSpace(in.SMTPHost)
	in.SMTPUsername = strings.TrimSpace(in.SMTPUsername)
	in.SMTPPassword = strings.TrimSpace(in.SMTPPassword)
	in.FromAddress = strings.TrimSpace(in.FromAddress)
	in.TLSMode = strings.TrimSpace(in.TLSMode)
	in.SubjectPrefix = strings.TrimSpace(in.SubjectPrefix)
	if in.TLSMode == "" {
		in.TLSMode = defaultEmailNotificationTLSMode
	}
	if in.SubjectPrefix == "" {
		in.SubjectPrefix = "[Meridian]"
	}
	in.ToAddresses = normalizeEmailAddresses(in.ToAddresses)
	if in.Name == "" || in.SMTPHost == "" || in.SMTPPort < 1 || in.SMTPPort > 65535 ||
		in.FromAddress == "" || len(in.ToAddresses) == 0 {
		return EmailNotificationConfig{}, ErrValidation
	}
	if containsLineBreak(in.Name) || containsLineBreak(in.SMTPHost) || containsLineBreak(in.SMTPUsername) ||
		containsLineBreak(in.SMTPPassword) || containsLineBreak(in.FromAddress) || containsLineBreak(in.SubjectPrefix) {
		return EmailNotificationConfig{}, ErrValidation
	}
	if len(in.Name) > 120 || len(in.SMTPHost) > 255 || len(in.SMTPUsername) > 255 ||
		len(in.SMTPPassword) > 1024 || len(in.FromAddress) > 320 || len(in.SubjectPrefix) > 120 {
		return EmailNotificationConfig{}, ErrValidation
	}
	switch in.TLSMode {
	case "none", "starttls", "tls":
	default:
		return EmailNotificationConfig{}, ErrValidation
	}
	if _, err := mail.ParseAddress(in.FromAddress); err != nil {
		return EmailNotificationConfig{}, ErrValidation
	}
	for _, address := range in.ToAddresses {
		if containsLineBreak(address) || len(address) > 320 {
			return EmailNotificationConfig{}, ErrValidation
		}
		if _, err := mail.ParseAddress(address); err != nil {
			return EmailNotificationConfig{}, ErrValidation
		}
	}
	return in, nil
}

func normalizeEmailAddresses(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func containsLineBreak(value string) bool {
	return strings.ContainsAny(value, "\r\n")
}

func scanEmailNotificationConfig(row pgx.Row) (EmailNotificationConfig, error) {
	var item EmailNotificationConfig
	err := row.Scan(&item.ID, &item.Name, &item.Enabled, &item.SMTPHost, &item.SMTPPort, &item.SMTPUsername,
		&item.SMTPPassword, &item.FromAddress, &item.ToAddresses, &item.TLSMode, &item.SubjectPrefix,
		&item.CreatedAt, &item.UpdatedAt)
	return item, dbErr(err)
}

func scanEmailNotificationConfigs(rows pgx.Rows) ([]EmailNotificationConfig, error) {
	var out []EmailNotificationConfig
	for rows.Next() {
		item, err := scanEmailNotificationConfig(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func scanWorkbenchNotification(row pgx.Row) (WorkbenchNotification, error) {
	var item WorkbenchNotification
	err := row.Scan(&item.ID, &item.Type, &item.ServerID, &item.ServerName, &item.ProjectID, &item.ProjectName,
		&item.TaskID, &item.TaskTitle, &item.RunID, &item.RunStatus, &item.Title, &item.Message, &item.AcknowledgedAt, &item.CreatedAt)
	return item, dbErr(err)
}

func scanWorkbenchNotifications(rows pgx.Rows) ([]WorkbenchNotification, error) {
	var out []WorkbenchNotification
	for rows.Next() {
		item, err := scanWorkbenchNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
