CREATE TABLE email_notification_configs (
  id text PRIMARY KEY DEFAULT ('eml_' || replace(gen_random_uuid()::text, '-', '')),
  name text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  smtp_host text NOT NULL,
  smtp_port integer NOT NULL CHECK (smtp_port >= 1 AND smtp_port <= 65535),
  smtp_username text NOT NULL DEFAULT '',
  smtp_password text NOT NULL DEFAULT '',
  from_address text NOT NULL,
  to_addresses text[] NOT NULL DEFAULT '{}',
  tls_mode text NOT NULL DEFAULT 'starttls' CHECK (tls_mode IN ('none', 'starttls', 'tls')),
  subject_prefix text NOT NULL DEFAULT '[Codex Task Workbench]',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
