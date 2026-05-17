CREATE TABLE workbench_notifications (
  id text PRIMARY KEY DEFAULT ('ntf_' || replace(gen_random_uuid()::text, '-', '')),
  type text NOT NULL CHECK (type IN ('task_done', 'run_finished')),
  server_id text NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  project_id text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  task_id text NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  run_id text REFERENCES runs(id) ON DELETE CASCADE,
  run_status text CHECK (run_status IS NULL OR run_status IN ('succeeded', 'failed', 'canceled')),
  title text NOT NULL,
  message text NOT NULL,
  acknowledged_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX workbench_notifications_pending_idx
  ON workbench_notifications (created_at DESC)
  WHERE acknowledged_at IS NULL;

CREATE INDEX workbench_notifications_task_id_idx
  ON workbench_notifications (task_id);

CREATE INDEX workbench_notifications_run_id_idx
  ON workbench_notifications (run_id)
  WHERE run_id IS NOT NULL;
