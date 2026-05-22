package control

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var runnerUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var browserUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (a *API) handleRunnerWS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	conn, err := runnerUpgrader.Upgrade(w, r, nil)
	if err != nil {
		a.logger.Warn("runner websocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	var runnerID string
	defer func() {
		if runnerID != "" {
			a.fileTransfers.Close(runnerID)
			if a.runners.Unregister(runnerID, conn) {
				a.runnerUpdates.MarkDisconnected(runnerID, time.Now().UTC())
				a.markRunnerOfflineAfterGrace(runnerID, 45*time.Second)
			}
		}
	}()

	for {
		var env RunnerEnvelope
		if err := conn.ReadJSON(&env); err != nil {
			a.logger.Info("runner websocket closed", "runner_id", runnerID, "error", err)
			return
		}
		switch env.Type {
		case "runner.register":
			var payload RunnerRegisterPayload
			if !decodeEnvelopePayload(env.Payload, &payload, a, "runner.register") {
				continue
			}
			runnerID = payload.RunnerID
			if err := a.store.UpsertRunnerHeartbeat(r.Context(), payload.RunnerID, payload.Hostname); err != nil {
				a.logger.Warn("runner register failed", "runner_id", payload.RunnerID, "error", err)
				continue
			}
			a.runners.Register(payload.RunnerID, conn, RunnerInfo{
				Hostname:    payload.Hostname,
				Version:     payload.Version,
				CodexPath:   payload.CodexPath,
				ConnectedAt: time.Now().UTC(),
			}, payload.Capabilities)
			a.runnerUpdates.MarkRegistered(payload.RunnerID, RunnerInfo{
				Hostname:    payload.Hostname,
				Version:     payload.Version,
				CodexPath:   payload.CodexPath,
				ConnectedAt: time.Now().UTC(),
			}, time.Now().UTC())
			if supportsCapability(payload.Capabilities, "active_runs") {
				events, err := a.store.ReconcileRunnerActiveRuns(r.Context(), payload.RunnerID, payload.ActiveRunIDs, time.Now().UTC())
				if err != nil {
					a.logger.Warn("runner active run reconcile failed", "runner_id", payload.RunnerID, "error", err)
				}
				for _, event := range events {
					a.hub.Publish(event)
				}
			}
			go a.publishAndMaybeAssign(context.Background(), payload.RunnerID)
		case "runner.heartbeat":
			var payload RunnerHeartbeatPayload
			if !decodeEnvelopePayload(env.Payload, &payload, a, "runner.heartbeat") {
				continue
			}
			if payload.RunnerID == "" {
				payload.RunnerID = runnerID
			}
			if payload.RunnerID == "" {
				continue
			}
			if err := a.store.UpsertRunnerHeartbeat(r.Context(), payload.RunnerID, payload.RunnerID); err != nil {
				a.logger.Warn("runner heartbeat failed", "runner_id", payload.RunnerID, "error", err)
				continue
			}
			if a.runners.Supports(payload.RunnerID, "active_runs") {
				events, err := a.store.ReconcileRunnerActiveRuns(r.Context(), payload.RunnerID, payload.ActiveRunIDs, time.Now().UTC())
				if err != nil {
					a.logger.Warn("runner active run reconcile failed", "runner_id", payload.RunnerID, "error", err)
				}
				for _, event := range events {
					a.hub.Publish(event)
				}
			}
			if runnerID == "" {
				runnerID = payload.RunnerID
				a.runners.Register(payload.RunnerID, conn, RunnerInfo{ConnectedAt: time.Now().UTC()}, nil)
			}
			go a.publishAndMaybeAssign(context.Background(), payload.RunnerID)
		case "runner.update.response", "runner.shutdown.response", "fs.list.response", "project.files.response", "project.file.read.response", "project.file.write.response", "project.file.upload.response", "project.file.upload.status.response", "project.file.upload.chunk.response", "project.file.action.response", "project.command.response", "project.terminal.open.response":
			if !a.runners.HandleResponse(runnerID, env) {
				a.logger.Warn("unmatched runner response", "type", env.Type, "message_id", env.MessageID, "runner_id", runnerID)
			}
		case "runner.update.status":
			var payload RunnerUpdateStatusPayload
			if !decodeEnvelopePayload(env.Payload, &payload, a, "runner.update.status") {
				continue
			}
			if payload.RunnerID == "" {
				payload.RunnerID = runnerID
			}
			a.runnerUpdates.MarkRunnerStatus(payload, time.Now().UTC())
		case "project.terminal.output":
			var payload ProjectTerminalOutputPayload
			if !decodeEnvelopePayload(env.Payload, &payload, a, "project.terminal.output") {
				continue
			}
			a.terminalHub.Publish(payload.TerminalID, env)
		case "project.terminal.exit":
			var payload ProjectTerminalExitPayload
			if !decodeEnvelopePayload(env.Payload, &payload, a, "project.terminal.exit") {
				continue
			}
			a.terminalHub.Publish(payload.TerminalID, env)
		case "run.started":
			var payload RunStartedPayload
			if !decodeEnvelopePayload(env.Payload, &payload, a, "run.started") {
				continue
			}
			if payload.StartedAt.IsZero() {
				payload.StartedAt = time.Now().UTC()
			}
			event, err := a.store.MarkRunStarted(r.Context(), payload.RunID, runnerID, payload.StartedAt)
			if err != nil {
				a.logger.Warn("mark run started failed", "run_id", payload.RunID, "error", err)
				continue
			}
			a.hub.Publish(event)
		case "run.event":
			var payload RunEventPayload
			if !decodeEnvelopePayload(env.Payload, &payload, a, "run.event") {
				continue
			}
			event, err := a.store.InsertRunnerEvent(r.Context(), RunnerEventInput{
				RunID:      payload.RunID,
				EventType:  payload.EventType,
				Stream:     payload.Stream,
				Payload:    payload.EventPayload,
				OccurredAt: payload.OccurredAt,
			})
			if err != nil {
				a.logger.Warn("persist runner event failed", "run_id", payload.RunID, "error", err)
				continue
			}
			a.hub.Publish(event)
		case "run.completed":
			var payload RunCompletedPayload
			if !decodeEnvelopePayload(env.Payload, &payload, a, "run.completed") {
				continue
			}
			endedAt := time.Now().UTC()
			if payload.EndedAt != nil && !payload.EndedAt.IsZero() {
				endedAt = *payload.EndedAt
			}
			result, err := a.store.CompleteRun(r.Context(), CompleteRunInput{
				RunID:          payload.RunID,
				Status:         payload.Status,
				ExitCode:       payload.ExitCode,
				ErrorMessage:   payload.ErrorMessage,
				FinalMessage:   payload.FinalMessage,
				CodexSessionID: payload.CodexSessionID,
				EndedAt:        endedAt,
			})
			if err != nil {
				a.logger.Warn("complete run failed", "run_id", payload.RunID, "error", err)
				continue
			}
			if _, notifyErr := a.store.CreateRunFinishedNotification(r.Context(), result.Run); notifyErr != nil {
				a.logger.Warn("create run finished web notification failed", "run_id", payload.RunID, "error", notifyErr)
			}
			a.hub.Publish(result.Event)
		case "run.cancel_ack":
			var payload RunCancelAckPayload
			if !decodeEnvelopePayload(env.Payload, &payload, a, "run.cancel_ack") {
				continue
			}
			a.logger.Info("runner cancel ack", "runner_id", runnerID, "run_id", payload.RunID, "accepted", payload.Accepted)
		default:
			a.logger.Warn("unknown runner message", "type", env.Type, "runner_id", runnerID)
		}
	}
}

func (a *API) handleRunnerFileTransferWS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	conn, err := runnerUpgrader.Upgrade(w, r, nil)
	if err != nil {
		a.logger.Warn("runner file transfer websocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	var runnerID string
	defer func() {
		if runnerID != "" {
			a.fileTransfers.Unregister(runnerID, conn)
		}
	}()

	for {
		var env RunnerEnvelope
		if err := conn.ReadJSON(&env); err != nil {
			a.logger.Info("runner file transfer websocket closed", "runner_id", runnerID, "error", err)
			return
		}
		switch env.Type {
		case "runner.file_transfer.register":
			var payload RunnerFileTransferRegisterPayload
			if !decodeEnvelopePayload(env.Payload, &payload, a, "runner.file_transfer.register") {
				continue
			}
			if payload.RunnerID == "" {
				continue
			}
			runnerID = payload.RunnerID
			a.fileTransfers.Register(payload.RunnerID, conn)
		case "project.file.upload.stream.response":
			if !a.fileTransfers.HandleResponse(runnerID, env) {
				a.logger.Warn("unmatched runner file transfer response", "type", env.Type, "message_id", env.MessageID, "runner_id", runnerID)
			}
		default:
			a.logger.Warn("unknown runner file transfer message", "type", env.Type, "runner_id", runnerID)
		}
	}
}

func (a *API) markRunnerOfflineAfterGrace(runnerID string, grace time.Duration) {
	go func() {
		timer := time.NewTimer(grace)
		defer timer.Stop()
		<-timer.C
		if a.runners.Connected(runnerID) {
			return
		}
		staleBefore := time.Now().UTC().Add(-grace)
		if err := a.store.MarkRunnerOfflineIfStale(context.Background(), runnerID, staleBefore); err != nil {
			a.logger.Warn("mark runner offline failed", "runner_id", runnerID, "error", err)
		}
		events, err := a.store.ReconcileRunnerDisconnected(context.Background(), runnerID, time.Now().UTC())
		if err != nil {
			a.logger.Warn("runner disconnect reconcile failed", "runner_id", runnerID, "error", err)
			return
		}
		for _, event := range events {
			a.hub.Publish(event)
		}
	}()
}

func decodeEnvelopePayload(raw json.RawMessage, dst any, a *API, typ string) bool {
	if err := json.Unmarshal(raw, dst); err != nil {
		a.logger.Warn("invalid runner payload", "type", typ, "error", err)
		return false
	}
	return true
}

func supportsCapability(capabilities map[string]any, capability string) bool {
	if capabilities == nil {
		return false
	}
	value, ok := capabilities[capability]
	if !ok {
		return false
	}
	if available, ok := value.(bool); ok {
		return available
	}
	return false
}
