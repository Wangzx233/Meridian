package control

import (
	"testing"
	"time"
)

func TestRunnerUpdateTrackerTracksReconnectSuccess(t *testing.T) {
	tracker := NewRunnerUpdateTracker()
	now := time.Date(2026, 5, 22, 8, 0, 0, 0, time.UTC)
	progress := tracker.Begin("abc123", now)

	tracker.Upsert(RunnerUpdateProgressResult{
		ServerID:        "srv_1",
		ServerName:      "desktop",
		RunnerID:        "runner_desktop",
		PreviousVersion: stringPtr("old"),
		Status:          "accepted",
		Message:         "accepted",
		UpdatedAt:       now,
	}, now)
	tracker.MarkDisconnected("runner_desktop", now.Add(time.Second))

	latest := tracker.Latest(now.Add(2 * time.Second))
	if !latest.Active || latest.InProgress != 1 || latest.Results[0].Status != "waiting_reconnect" {
		t.Fatalf("waiting progress = %#v", latest)
	}

	tracker.MarkRegistered("runner_desktop", RunnerInfo{Version: "abc123"}, now.Add(3*time.Second))
	latest = tracker.Latest(now.Add(4 * time.Second))
	if latest.UpdateID != progress.UpdateID {
		t.Fatalf("update id changed from %q to %q", progress.UpdateID, latest.UpdateID)
	}
	if latest.Active || latest.Succeeded != 1 || latest.Failed != 0 {
		t.Fatalf("final counts = %#v", latest)
	}
	if latest.Results[0].Status != "succeeded" || latest.Results[0].CurrentVersion == nil || *latest.Results[0].CurrentVersion != "abc123" {
		t.Fatalf("result = %#v", latest.Results[0])
	}
}

func TestRunnerUpdateTrackerMarksVersionMismatch(t *testing.T) {
	tracker := NewRunnerUpdateTracker()
	now := time.Date(2026, 5, 22, 8, 0, 0, 0, time.UTC)
	tracker.Begin("target", now)
	tracker.Upsert(RunnerUpdateProgressResult{ServerName: "desktop", RunnerID: "runner_desktop", Status: "accepted", UpdatedAt: now}, now)
	tracker.MarkRegistered("runner_desktop", RunnerInfo{Version: "other"}, now.Add(time.Second))

	latest := tracker.Latest(now.Add(2 * time.Second))
	if latest.Active || latest.Failed != 1 {
		t.Fatalf("counts = %#v", latest)
	}
	if latest.Results[0].Status != "version_mismatch" || latest.Results[0].Error == nil {
		t.Fatalf("result = %#v", latest.Results[0])
	}
}

func TestRunnerUpdateTrackerTimesOutOldRunnerProgress(t *testing.T) {
	tracker := NewRunnerUpdateTracker()
	now := time.Date(2026, 5, 22, 8, 0, 0, 0, time.UTC)
	tracker.Begin("target", now)
	tracker.Upsert(RunnerUpdateProgressResult{ServerName: "desktop", RunnerID: "runner_desktop", Status: "accepted", UpdatedAt: now}, now)

	latest := tracker.Latest(now.Add(runnerUpdateTimeout + time.Second))
	if latest.Active || latest.Failed != 1 || latest.Results[0].Status != "timed_out" {
		t.Fatalf("timeout progress = %#v", latest)
	}
}
