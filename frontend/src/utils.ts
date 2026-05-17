import type { ContextType, Run, RunEvent, RunStatus, ServerStatus, TaskStatus } from "./types";

export function formatDateTime(value: string | null | undefined) {
  if (!value) {
    return "Not recorded";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export function shortId(value: string | null | undefined, size = 10) {
  if (!value) {
    return "none";
  }
  return value.length > size ? `${value.slice(0, size)}...` : value;
}

export function isActiveRunStatus(status: RunStatus) {
  return status === "queued" || status === "running";
}

export function isActiveTaskStatus(status: TaskStatus) {
  return status === "running";
}

export function statusTone(status: TaskStatus | RunStatus | ServerStatus) {
  if (status === "running" || status === "queued") {
    return "attention";
  }
  if (status === "succeeded" || status === "waiting_user" || status === "online") {
    return "success";
  }
  if (status === "failed" || status === "offline") {
    return "danger";
  }
  if (status === "canceled" || status === "archived") {
    return "muted";
  }
  if (status === "done") {
    return "done";
  }
  return "neutral";
}

export function eventSummary(event: RunEvent) {
  if (typeof event.payload.text === "string" && event.payload.text.trim()) {
    return event.payload.text;
  }

  if (event.event_type === "runner.error") {
    return event.payload.message ? String(event.payload.message) : "Runner error";
  }

  if (event.event_type === "run.state") {
    return `Run state changed to ${event.payload.status ?? "unknown"}`;
  }

  if (event.event_type === "run.final") {
    return String(event.payload.final_message ?? event.payload.error_message ?? `Run ${event.payload.status ?? "finished"}`);
  }

  return JSON.stringify(event.payload.raw ?? event.payload, null, 2);
}

export function contextTypeLabel(type: ContextType) {
  return type.replace(/_/g, " ");
}

export function runDuration(run: Run) {
  const start = run.runner_started_at ?? run.started_at ?? run.created_at;
  const end = run.ended_at;
  if (!start || !end) {
    return "In progress";
  }
  const ms = new Date(end).getTime() - new Date(start).getTime();
  if (Number.isNaN(ms) || ms < 0) {
    return "Not recorded";
  }
  const seconds = Math.round(ms / 1000);
  if (seconds < 60) {
    return `${seconds}s`;
  }
  return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
}
