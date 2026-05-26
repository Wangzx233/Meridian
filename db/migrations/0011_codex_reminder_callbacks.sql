ALTER TABLE runs
  ADD COLUMN IF NOT EXISTS reminder_callback_enabled boolean NOT NULL DEFAULT false;

ALTER TABLE workbench_notifications
  DROP CONSTRAINT IF EXISTS workbench_notifications_type_check;

ALTER TABLE workbench_notifications
  ADD CONSTRAINT workbench_notifications_type_check
  CHECK (type IN ('task_done', 'run_finished', 'codex_reminder'));
