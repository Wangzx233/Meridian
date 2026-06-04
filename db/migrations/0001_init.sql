CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS schema_migrations (
  version text PRIMARY KEY,
  applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE servers (
  id text PRIMARY KEY DEFAULT ('srv_' || replace(gen_random_uuid()::text, '-', '')),
  name text NOT NULL,
  runner_id text NOT NULL UNIQUE,
  status text NOT NULL DEFAULT 'offline' CHECK (status IN ('online', 'offline')),
  last_heartbeat_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE projects (
  id text PRIMARY KEY DEFAULT ('prj_' || replace(gen_random_uuid()::text, '-', '')),
  server_id text NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  name text NOT NULL,
  workdir text NOT NULL,
  default_branch text,
  rules_path text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tasks (
  id text PRIMARY KEY DEFAULT ('tsk_' || replace(gen_random_uuid()::text, '-', '')),
  project_id text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  title text NOT NULL,
  description text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'running', 'waiting_user', 'done', 'archived')),
  codex_session_id text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz,
  archived_at timestamptz
);

CREATE TABLE runs (
  id text PRIMARY KEY DEFAULT ('run_' || replace(gen_random_uuid()::text, '-', '')),
  task_id text NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  mode text NOT NULL CHECK (mode IN ('new', 'resume')),
  status text NOT NULL CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'canceled')),
  user_message text NOT NULL,
  generated_prompt text NOT NULL,
  codex_model text,
  codex_reasoning_effort text CHECK (codex_reasoning_effort IS NULL OR codex_reasoning_effort IN ('low', 'medium', 'high', 'xhigh')),
  codex_service_tier text CHECK (codex_service_tier IS NULL OR codex_service_tier IN ('fast')),
  raw_command boolean NOT NULL DEFAULT false,
  idempotency_key text,
  final_message text,
  codex_session_id text,
  assigned_runner_id text,
  exit_code integer,
  error_message text,
  cancel_requested_at timestamptz,
  runner_started_at timestamptz,
  started_at timestamptz NOT NULL DEFAULT now(),
  ended_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX runs_one_active_per_task_idx
  ON runs(task_id)
  WHERE status IN ('queued', 'running');

CREATE UNIQUE INDEX runs_task_idempotency_key_idx
  ON runs(task_id, idempotency_key)
  WHERE idempotency_key IS NOT NULL;

CREATE TABLE context_items (
  id text PRIMARY KEY DEFAULT ('ctx_' || replace(gen_random_uuid()::text, '-', '')),
  server_id text REFERENCES servers(id) ON DELETE CASCADE,
  project_id text REFERENCES projects(id) ON DELETE CASCADE,
  task_id text REFERENCES tasks(id) ON DELETE CASCADE,
  scope text NOT NULL CHECK (scope IN ('global', 'server', 'project', 'task')),
  type text NOT NULL CHECK (type IN ('project_rule', 'task_summary', 'decision', 'log_snippet', 'verify_command', 'file_hint', 'note')),
  title text NOT NULL,
  content text NOT NULL,
  tags text[] NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT context_scope_owner_consistency CHECK (
    (scope = 'global' AND server_id IS NULL AND project_id IS NULL AND task_id IS NULL) OR
    (scope = 'server' AND server_id IS NOT NULL AND project_id IS NULL AND task_id IS NULL) OR
    (scope = 'project' AND server_id IS NOT NULL AND project_id IS NOT NULL AND task_id IS NULL) OR
    (scope = 'task' AND server_id IS NOT NULL AND project_id IS NOT NULL AND task_id IS NOT NULL)
  )
);

CREATE INDEX context_items_server_id_idx ON context_items(server_id);
CREATE INDEX context_items_project_id_idx ON context_items(project_id);
CREATE INDEX context_items_task_id_idx ON context_items(task_id);

CREATE TABLE run_context_items (
  run_id text NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  context_item_id text NOT NULL REFERENCES context_items(id) ON DELETE RESTRICT,
  order_index integer NOT NULL,
  type_snapshot text NOT NULL,
  title_snapshot text NOT NULL,
  content_snapshot text NOT NULL,
  PRIMARY KEY (run_id, order_index)
);

CREATE TABLE run_input_images (
  id text PRIMARY KEY DEFAULT ('img_' || replace(gen_random_uuid()::text, '-', '')),
  run_id text NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  filename text NOT NULL,
  mime_type text NOT NULL CHECK (mime_type IN ('image/png', 'image/jpeg', 'image/gif', 'image/webp')),
  size_bytes bigint NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 8388608),
  content bytea NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX run_input_images_run_id_idx
  ON run_input_images (run_id, created_at);

CREATE TABLE run_events (
  id text PRIMARY KEY DEFAULT ('evt_' || replace(gen_random_uuid()::text, '-', '')),
  run_id text NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  seq bigint NOT NULL,
  event_type text NOT NULL CHECK (event_type IN ('run.state', 'codex.event', 'process.output', 'runner.error', 'run.final')),
  stream text NOT NULL CHECK (stream IN ('jsonl', 'stdout', 'stderr', 'system')),
  payload jsonb NOT NULL,
  occurred_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (run_id, seq)
);

CREATE TABLE task_memories (
  id text PRIMARY KEY DEFAULT ('mem_' || replace(gen_random_uuid()::text, '-', '')),
  task_id text NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  project_id text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  problem text NOT NULL DEFAULT '',
  root_cause text NOT NULL DEFAULT '',
  changes text NOT NULL DEFAULT '',
  files text NOT NULL DEFAULT '',
  decisions text NOT NULL DEFAULT '',
  verification text NOT NULL DEFAULT '',
  related_tasks text NOT NULL DEFAULT '',
  source_commit text NOT NULL DEFAULT '',
  stale_conditions text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
