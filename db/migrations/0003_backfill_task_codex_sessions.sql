UPDATE tasks t
SET codex_session_id = (
      SELECT r.codex_session_id
      FROM runs r
      WHERE r.task_id = t.id
        AND r.codex_session_id IS NOT NULL
        AND r.codex_session_id <> ''
      ORDER BY r.created_at DESC
      LIMIT 1
    ),
    updated_at = now()
WHERE (t.codex_session_id IS NULL OR t.codex_session_id = '')
  AND EXISTS (
    SELECT 1
    FROM runs r
    WHERE r.task_id = t.id
      AND r.codex_session_id IS NOT NULL
      AND r.codex_session_id <> ''
  );
