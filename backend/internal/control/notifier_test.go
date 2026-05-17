package control

import (
	"strings"
	"testing"
	"time"
)

func TestBuildTaskCompletionEmail(t *testing.T) {
	completed := time.Date(2026, 5, 13, 10, 30, 0, 0, time.UTC)
	msg := string(buildTaskCompletionEmail(
		EmailNotificationConfig{
			FromAddress:   "workbench@example.com",
			SubjectPrefix: "[CTW]",
		},
		[]string{"dev@example.com"},
		TaskCompletionNotification{
			Task:      Task{ID: "tsk_123", Title: "Finish mail"},
			Project:   Project{Name: "Workbench"},
			Server:    Server{Name: "Desktop"},
			Summary:   "Done and verified.",
			Completed: completed,
		},
	))
	for _, want := range []string{
		"From: workbench@example.com",
		"To: dev@example.com",
		"Subject:",
		"Task ID: tsk_123",
		"Project: Workbench",
		"Done and verified.",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message missing %q:\n%s", want, msg)
		}
	}
}
