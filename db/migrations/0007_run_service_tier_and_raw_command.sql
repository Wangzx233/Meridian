ALTER TABLE runs
  ADD COLUMN IF NOT EXISTS codex_service_tier text;

ALTER TABLE runs
  ADD COLUMN IF NOT EXISTS raw_command boolean NOT NULL DEFAULT false;

ALTER TABLE runs
  DROP CONSTRAINT IF EXISTS runs_codex_service_tier_check;

ALTER TABLE runs
  ADD CONSTRAINT runs_codex_service_tier_check
  CHECK (codex_service_tier IS NULL OR codex_service_tier IN ('fast'));
