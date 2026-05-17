ALTER TABLE workbench_notifications
  DROP CONSTRAINT IF EXISTS workbench_notifications_type_check;

ALTER TABLE workbench_notifications
  ADD CONSTRAINT workbench_notifications_type_check
  CHECK (type IN ('task_done', 'run_finished'));

ALTER TABLE workbench_notifications
  ADD COLUMN IF NOT EXISTS run_id text REFERENCES runs(id) ON DELETE CASCADE;

ALTER TABLE workbench_notifications
  ADD COLUMN IF NOT EXISTS run_status text;

ALTER TABLE workbench_notifications
  DROP CONSTRAINT IF EXISTS workbench_notifications_run_status_check;

ALTER TABLE workbench_notifications
  ADD CONSTRAINT workbench_notifications_run_status_check
  CHECK (run_status IS NULL OR run_status IN ('succeeded', 'failed', 'canceled'));

CREATE INDEX IF NOT EXISTS workbench_notifications_run_id_idx
  ON workbench_notifications (run_id)
  WHERE run_id IS NOT NULL;
