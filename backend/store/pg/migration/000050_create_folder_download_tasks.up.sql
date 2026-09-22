CREATE TABLE IF NOT EXISTS folder_download_tasks (
  id varchar(64) PRIMARY KEY,
  kb_id varchar(64) NOT NULL,
  node_id varchar(64) NOT NULL,
  created_by varchar(64) NOT NULL,
  status varchar(32) NOT NULL DEFAULT 'waiting',
  total_files integer NOT NULL DEFAULT 0,
  processed_files integer NOT NULL DEFAULT 0,
  total_bytes bigint NOT NULL DEFAULT 0,
  processed_bytes bigint NOT NULL DEFAULT 0,
  retry_count integer NOT NULL DEFAULT 0,
  object_key text NOT NULL DEFAULT '',
  error_message text NOT NULL DEFAULT '',
  expires_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_folder_download_tasks_owner
  ON folder_download_tasks(kb_id, created_by, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_folder_download_tasks_expiry
  ON folder_download_tasks(expires_at);
