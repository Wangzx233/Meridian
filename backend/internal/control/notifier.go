package control

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

type TaskCompletionNotification struct {
	Task      Task
	Project   Project
	Server    Server
	Summary   string
	Completed time.Time
}

type WorkbenchEmailNotification struct {
	Notification WorkbenchNotification
	CreatedAt    time.Time
}

func (a *API) notifyTaskDoneAsync(task Task, summary string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := a.notifyTaskDone(ctx, task, summary); err != nil {
			a.logger.Warn("task completion notification failed", "task_id", task.ID, "error", err)
		}
	}()
}

func (a *API) notifyTaskDone(ctx context.Context, task Task, summary string) error {
	configs, err := a.store.ListEmailNotificationConfigs(ctx, true)
	if err != nil {
		return err
	}
	if len(configs) == 0 {
		return nil
	}
	project, err := a.store.GetProject(ctx, task.ProjectID)
	if err != nil {
		return err
	}
	server, err := a.store.GetServer(ctx, project.ServerID)
	if err != nil {
		return err
	}
	completed := time.Now().UTC()
	if task.CompletedAt != nil && !task.CompletedAt.IsZero() {
		completed = *task.CompletedAt
	}
	notification := TaskCompletionNotification{
		Task:      task,
		Project:   project,
		Server:    server,
		Summary:   summary,
		Completed: completed,
	}
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		if err := sendTaskCompletionEmail(ctx, cfg, notification); err != nil {
			a.logger.Warn("send completion email failed", "task_id", task.ID, "config_id", cfg.ID, "error", err)
		}
	}
	return nil
}

func (a *API) notifyWorkbenchNotificationAsync(notification WorkbenchNotification) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := a.notifyWorkbenchNotification(ctx, notification); err != nil {
			a.logger.Warn("workbench email notification failed", "notification_id", notification.ID, "error", err)
		}
	}()
}

func (a *API) notifyWorkbenchNotification(ctx context.Context, notification WorkbenchNotification) error {
	configs, err := a.store.ListEmailNotificationConfigs(ctx, true)
	if err != nil {
		return err
	}
	if len(configs) == 0 {
		return nil
	}
	payload := WorkbenchEmailNotification{
		Notification: notification,
		CreatedAt:    notification.CreatedAt,
	}
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		if err := sendWorkbenchEmail(ctx, cfg, payload); err != nil {
			a.logger.Warn("send workbench email failed", "notification_id", notification.ID, "config_id", cfg.ID, "error", err)
		}
	}
	return nil
}

func sendTaskCompletionEmail(ctx context.Context, cfg EmailNotificationConfig, notification TaskCompletionNotification) error {
	return sendEmail(ctx, cfg, buildTaskCompletionEmail(cfg, notification))
}

func sendWorkbenchEmail(ctx context.Context, cfg EmailNotificationConfig, notification WorkbenchEmailNotification) error {
	return sendEmail(ctx, cfg, buildWorkbenchEmail(cfg, notification))
}

func sendEmail(ctx context.Context, cfg EmailNotificationConfig, message []byte) error {
	address := net.JoinHostPort(cfg.SMTPHost, fmt.Sprintf("%d", cfg.SMTPPort))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var conn net.Conn
	var err error
	if cfg.TLSMode == "tls" {
		conn, err = tls.DialWithDialer(dialer, "tcp", address, tlsConfigForHost(cfg.SMTPHost))
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	}

	client, err := smtp.NewClient(conn, cfg.SMTPHost)
	if err != nil {
		return err
	}
	defer client.Close()

	if cfg.TLSMode == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsConfigForHost(cfg.SMTPHost)); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("smtp server does not advertise STARTTLS")
		}
	}
	if cfg.SMTPUsername != "" || cfg.SMTPPassword != "" {
		auth := smtp.PlainAuth("", cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPHost)
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	from, err := mail.ParseAddress(cfg.FromAddress)
	if err != nil {
		return err
	}
	if err := client.Mail(from.Address); err != nil {
		return err
	}
	for _, raw := range cfg.ToAddresses {
		to, err := mail.ParseAddress(raw)
		if err != nil {
			return err
		}
		if err := client.Rcpt(to.Address); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func tlsConfigForHost(host string) *tls.Config {
	return &tls.Config{
		ServerName:         host,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: false,
	}
}

func buildTaskCompletionEmail(cfg EmailNotificationConfig, notification TaskCompletionNotification) []byte {
	subject := strings.TrimSpace(fmt.Sprintf("%s Task completed: %s", cfg.SubjectPrefix, notification.Task.Title))
	body := buildTaskCompletionEmailBody(notification)
	return buildPlainTextEmail(cfg, subject, notification.Completed, body)
}

func buildWorkbenchEmail(cfg EmailNotificationConfig, notification WorkbenchEmailNotification) []byte {
	subject := strings.TrimSpace(fmt.Sprintf("%s %s", cfg.SubjectPrefix, notification.Notification.Title))
	body := buildWorkbenchEmailBody(notification)
	return buildPlainTextEmail(cfg, subject, notification.CreatedAt, body)
}

func buildPlainTextEmail(cfg EmailNotificationConfig, subject string, date time.Time, body string) []byte {
	var b bytes.Buffer
	writeHeader(&b, "From", cfg.FromAddress)
	writeHeader(&b, "To", strings.Join(emailRecipients(cfg), ", "))
	writeHeader(&b, "Subject", subject)
	writeHeader(&b, "Date", date.Format(time.RFC1123Z))
	writeHeader(&b, "MIME-Version", "1.0")
	writeHeader(&b, "Content-Type", `text/plain; charset="UTF-8"`)
	writeHeader(&b, "Content-Transfer-Encoding", "quoted-printable")
	b.WriteString("\r\n")
	qp := quotedprintable.NewWriter(&b)
	_, _ = qp.Write([]byte(body))
	_ = qp.Close()
	return b.Bytes()
}

func emailRecipients(cfg EmailNotificationConfig) []string {
	recipients := make([]string, 0, len(cfg.ToAddresses))
	for _, raw := range cfg.ToAddresses {
		to, err := mail.ParseAddress(raw)
		if err != nil {
			recipients = append(recipients, raw)
			continue
		}
		recipients = append(recipients, to.Address)
	}
	return recipients
}

func writeHeader(b *bytes.Buffer, key, value string) {
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	b.WriteString(key)
	b.WriteString(": ")
	if key == "Subject" {
		b.WriteString(mime.QEncoding.Encode("UTF-8", value))
	} else {
		b.WriteString(value)
	}
	b.WriteString("\r\n")
}

func buildTaskCompletionEmailBody(notification TaskCompletionNotification) string {
	summary := strings.TrimSpace(notification.Summary)
	if summary == "" {
		summary = "No completion summary was provided."
	}
	return fmt.Sprintf(`A Codex task was marked done.

Task: %s
Task ID: %s
Project: %s
Server: %s
Completed at: %s

Summary:
%s
`, notification.Task.Title, notification.Task.ID, notification.Project.Name, notification.Server.Name, notification.Completed.Format(time.RFC3339), summary)
}

func buildWorkbenchEmailBody(notification WorkbenchEmailNotification) string {
	n := notification.Notification
	status := ""
	if n.RunStatus != nil && *n.RunStatus != "" {
		status = "\nRun status: " + *n.RunStatus
	}
	runID := ""
	if n.RunID != nil && *n.RunID != "" {
		runID = "\nRun ID: " + *n.RunID
	}
	return fmt.Sprintf(`A workbench notice needs attention.

Title: %s
Message: %s
Task: %s
Project: %s
Server: %s%s%s
Created at: %s
`, n.Title, n.Message, n.TaskTitle, n.ProjectName, n.ServerName, status, runID, notification.CreatedAt.Format(time.RFC3339))
}
