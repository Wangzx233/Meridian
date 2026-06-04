CREATE TABLE IF NOT EXISTS run_input_images (
  id text PRIMARY KEY DEFAULT ('img_' || replace(gen_random_uuid()::text, '-', '')),
  run_id text NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  filename text NOT NULL,
  mime_type text NOT NULL CHECK (mime_type IN ('image/png', 'image/jpeg', 'image/gif', 'image/webp')),
  size_bytes bigint NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 8388608),
  content bytea NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS run_input_images_run_id_idx
  ON run_input_images (run_id, created_at);
