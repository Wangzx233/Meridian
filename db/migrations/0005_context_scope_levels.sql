ALTER TABLE context_items
  ADD COLUMN IF NOT EXISTS server_id text REFERENCES servers(id) ON DELETE CASCADE;

UPDATE context_items ci
SET server_id = p.server_id
FROM projects p
WHERE ci.project_id = p.id
  AND ci.server_id IS NULL;

ALTER TABLE context_items
  ALTER COLUMN project_id DROP NOT NULL;

ALTER TABLE context_items
  DROP CONSTRAINT IF EXISTS context_items_scope_check,
  DROP CONSTRAINT IF EXISTS context_scope_task_consistency,
  DROP CONSTRAINT IF EXISTS context_scope_owner_consistency;

ALTER TABLE context_items
  ADD CONSTRAINT context_items_scope_check
  CHECK (scope IN ('global', 'server', 'project', 'task')),
  ADD CONSTRAINT context_scope_owner_consistency CHECK (
    (scope = 'global' AND server_id IS NULL AND project_id IS NULL AND task_id IS NULL) OR
    (scope = 'server' AND server_id IS NOT NULL AND project_id IS NULL AND task_id IS NULL) OR
    (scope = 'project' AND server_id IS NOT NULL AND project_id IS NOT NULL AND task_id IS NULL) OR
    (scope = 'task' AND server_id IS NOT NULL AND project_id IS NOT NULL AND task_id IS NOT NULL)
  );

CREATE INDEX IF NOT EXISTS context_items_server_id_idx ON context_items(server_id);
CREATE INDEX IF NOT EXISTS context_items_project_id_idx ON context_items(project_id);
CREATE INDEX IF NOT EXISTS context_items_task_id_idx ON context_items(task_id);
