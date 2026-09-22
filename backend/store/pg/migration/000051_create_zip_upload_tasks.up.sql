CREATE TABLE IF NOT EXISTS zip_upload_tasks (
  id varchar(64) PRIMARY KEY,
  kb_id varchar(64) NOT NULL,
  nav_id varchar(64) NOT NULL,
  parent_id varchar(64) NOT NULL DEFAULT '',
  created_by varchar(64) NOT NULL,
  filename text NOT NULL,
  file_key text NOT NULL,
  size bigint NOT NULL,
  part_size bigint NOT NULL,
  total_parts integer NOT NULL,
  upload_id text NOT NULL,
  uploaded_parts jsonb NOT NULL DEFAULT '{}'::jsonb,
  uploaded_bytes bigint NOT NULL DEFAULT 0,
  status varchar(32) NOT NULL DEFAULT 'waiting',
  retry_count integer NOT NULL DEFAULT 0,
  error_message text NOT NULL DEFAULT '',
  node_id varchar(64) NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_zip_upload_tasks_owner ON zip_upload_tasks(kb_id, created_by, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_zip_upload_tasks_status ON zip_upload_tasks(status, updated_at);
