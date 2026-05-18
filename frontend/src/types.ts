export type Timestamp = string;

export type ListResponse<T> = {
  items: T[];
  next_cursor: string | null;
};

export type ApiErrorResponse = {
  error: {
    code:
      | "validation_error"
      | "not_found"
      | "invalid_state"
      | "active_run_exists"
      | "active_run_missing"
      | "missing_codex_session"
      | "unauthorized"
      | "runner_unavailable"
      | "runner_unsupported"
      | "runner_request_timeout"
      | "internal_error"
      | string;
    message: string;
    details?: Record<string, unknown>;
  };
};

export type ServerStatus = "online" | "offline";

export type AuthSession = {
  authenticated: boolean;
  username: string;
  setup_required?: boolean;
  runner_token?: string;
};

export type Server = {
  id: string;
  name: string;
  alias: string | null;
  runner_id: string;
  status: ServerStatus;
  runner_connected: boolean;
  runner_connection?: {
    hostname?: string;
    version?: string;
    codex_path?: string;
    connected_at: Timestamp;
  };
  runner_capabilities?: Record<string, unknown>;
  last_heartbeat_at: Timestamp;
  created_at: Timestamp;
  updated_at: Timestamp;
};

export type RunnerUpdateStatus = "accepted" | "skipped" | "failed";

export type RunnerUpdateServerResult = {
  server_id: string;
  server_name: string;
  runner_id: string;
  previous_version?: string;
  status: RunnerUpdateStatus;
  message: string;
};

export type RunnerUpdateAllResponse = {
  requested_at: Timestamp;
  accepted: number;
  skipped: number;
  failed: number;
  results: RunnerUpdateServerResult[];
};

export type Project = {
  id: string;
  server_id: string;
  name: string;
  workdir: string;
  default_branch: string;
  rules_path: string;
  created_at: Timestamp;
  updated_at: Timestamp;
};

export type DirectoryEntry = {
  name: string;
  path: string;
  is_dir: boolean;
  markers?: string[];
};

export type DirectoryListing = {
  path: string;
  parent: string | null;
  entries: DirectoryEntry[];
  error?: string | null;
};

export type ProjectFileEntry = {
  name: string;
  path: string;
  is_dir: boolean;
  size: number;
  modified_at?: Timestamp | null;
  markers?: string[];
};

export type ProjectFileListing = {
  root: string;
  path: string;
  parent: string | null;
  entries: ProjectFileEntry[];
  error?: string | null;
};

export type ProjectFileContent = {
  root: string;
  path: string;
  name: string;
  size: number;
  modified_at?: Timestamp | null;
  content: string;
  encoding: string;
  error?: string | null;
};

export type ProjectFileActionResult = {
  root: string;
  path: string;
  target_path?: string;
  is_dir?: boolean;
  size?: number;
  modified_at?: Timestamp | null;
  error?: string | null;
};

export type ProjectCommandResult = {
  command: string;
  workdir: string;
  exit_code: number;
  stdout: string;
  stderr: string;
  duration_ms: number;
  error?: string | null;
};

export type TaskStatus = "open" | "running" | "waiting_user" | "done" | "archived";

export type Task = {
  id: string;
  project_id: string;
  title: string;
  description: string;
  status: TaskStatus;
  codex_session_id: string | null;
  active_run_id: string | null;
  created_at: Timestamp;
  updated_at: Timestamp;
  completed_at: Timestamp | null;
  archived_at: Timestamp | null;
};

export type RunMode = "new" | "resume";
export type CreateRunMode = "auto" | "new" | "resume";
export type CodexReasoningEffort = "" | "low" | "medium" | "high" | "xhigh";
export type CodexServiceTier = "" | "fast";
export type RunStatus = "queued" | "running" | "succeeded" | "failed" | "canceled";

export type Run = {
  id: string;
  task_id: string;
  mode: RunMode;
  status: RunStatus;
  user_message: string;
  generated_prompt: string;
  codex_model: string | null;
  codex_reasoning_effort: Exclude<CodexReasoningEffort, ""> | null;
  codex_service_tier: Exclude<CodexServiceTier, ""> | null;
  raw_command: boolean;
  final_message: string | null;
  codex_session_id: string | null;
  assigned_runner_id: string | null;
  exit_code: number | null;
  error_message: string | null;
  cancel_requested_at: Timestamp | null;
  runner_started_at: Timestamp | null;
  started_at: Timestamp | null;
  ended_at: Timestamp | null;
  created_at: Timestamp;
};

export type ContextScope = "global" | "server" | "project" | "task";

export type ContextType =
  | "project_rule"
  | "task_summary"
  | "decision"
  | "log_snippet"
  | "verify_command"
  | "file_hint"
  | "note";

export type ContextItem = {
  id: string;
  server_id: string | null;
  project_id: string | null;
  task_id: string | null;
  scope: ContextScope;
  type: ContextType;
  title: string;
  content: string;
  tags: string[];
  created_at: Timestamp;
  updated_at: Timestamp;
};

export type EmailTLSMode = "none" | "starttls" | "tls";

export type EmailNotificationConfig = {
  id: string;
  name: string;
  enabled: boolean;
  smtp_host: string;
  smtp_port: number;
  smtp_username: string;
  smtp_password?: string;
  from_address: string;
  to_addresses: string[];
  tls_mode: EmailTLSMode;
  subject_prefix: string;
  created_at: Timestamp;
  updated_at: Timestamp;
};

export type WorkbenchNotificationType = "run_finished" | "task_done";

export type WorkbenchNotification = {
  id: string;
  type: WorkbenchNotificationType;
  server_id: string;
  server_name: string;
  project_id: string;
  project_name: string;
  task_id: string;
  task_title: string;
  run_id: string | null;
  run_status: RunStatus | null;
  title: string;
  message: string;
  acknowledged_at: Timestamp | null;
  created_at: Timestamp;
};

export type RunEventType =
  | "run.state"
  | "codex.event"
  | "process.output"
  | "runner.error"
  | "run.final";

export type EventStream = "jsonl" | "stdout" | "stderr" | "system";

export type RunEventPayload = {
  raw?: unknown;
  text?: string;
  session_id?: string;
  status?: RunStatus;
  previous_status?: RunStatus;
  message?: string;
  code?: string;
  exit_code?: number | null;
  final_message?: string | null;
  error_message?: string | null;
  codex_session_id?: string | null;
  [key: string]: unknown;
};

export type RunEvent = {
  id?: string;
  run_id: string;
  task_id: string;
  seq: number;
  event_type: RunEventType;
  stream: EventStream;
  payload: RunEventPayload;
  occurred_at: Timestamp;
  created_at?: Timestamp;
};

export type CreateServerRequest = {
  name: string;
  alias?: string;
  runner_id: string;
};

export type CreateProjectRequest = {
  server_id: string;
  name: string;
  workdir: string;
  default_branch: string;
  rules_path: string;
};

export type CreateTaskRequest = {
  title: string;
  description: string;
};

export type TaskMemoryDraft = {
  problem: string;
  root_cause?: string;
  changes: string;
  files: string;
  decisions?: string;
  verification: string;
  related_tasks?: string;
  source_commit?: string;
  stale_conditions: string;
};

export type MarkDoneRequest = {
  summary: string;
  memory?: TaskMemoryDraft;
};

export type CreateContextItemRequest = {
  server_id?: string | null;
  project_id?: string | null;
  scope: ContextScope;
  task_id: string | null;
  type: ContextType;
  title: string;
  content: string;
  tags: string[];
};

export type EmailNotificationConfigRequest = {
  name: string;
  enabled: boolean;
  smtp_host: string;
  smtp_port: number;
  smtp_username: string;
  smtp_password: string;
  from_address: string;
  to_addresses: string[];
  tls_mode: EmailTLSMode;
  subject_prefix: string;
};

export type CreateRunRequest = {
  message: string;
  mode: CreateRunMode;
  codex_model: string;
  codex_reasoning_effort: CodexReasoningEffort;
  codex_service_tier: CodexServiceTier;
  raw_command?: boolean;
  context_item_ids: string[];
};

export type CreateRunResponse = {
  run: Pick<Run, "id" | "task_id" | "mode" | "status">;
  task: Pick<Task, "id" | "status" | "active_run_id">;
};
