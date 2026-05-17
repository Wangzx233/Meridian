ALTER TABLE runs
  ADD COLUMN IF NOT EXISTS codex_model text,
  ADD COLUMN IF NOT EXISTS codex_reasoning_effort text;

ALTER TABLE runs
  DROP CONSTRAINT IF EXISTS runs_codex_reasoning_effort_check;

ALTER TABLE runs
  ADD CONSTRAINT runs_codex_reasoning_effort_check
  CHECK (codex_reasoning_effort IS NULL OR codex_reasoning_effort IN ('low', 'medium', 'high', 'xhigh'));
