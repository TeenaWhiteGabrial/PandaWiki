CREATE TABLE IF NOT EXISTS node_learning_tasks (
  id text PRIMARY KEY,
  kb_id text NOT NULL,
  node_id text NOT NULL,
  node_release_id text NOT NULL,
  action text NOT NULL,
  status text NOT NULL,
  doc_id text NOT NULL DEFAULT '',
  error_message text NOT NULL DEFAULT '',
  triggered_by text NOT NULL DEFAULT '',
  started_at timestamptz,
  finished_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_node_learning_tasks_kb_node
  ON node_learning_tasks (kb_id, node_id);

CREATE INDEX IF NOT EXISTS idx_node_learning_tasks_release
  ON node_learning_tasks (node_release_id);

CREATE INDEX IF NOT EXISTS idx_node_learning_tasks_doc
  ON node_learning_tasks (doc_id);

CREATE INDEX IF NOT EXISTS idx_node_learning_tasks_status
  ON node_learning_tasks (status);

CREATE UNIQUE INDEX IF NOT EXISTS idx_node_learning_tasks_active_node
  ON node_learning_tasks (kb_id, node_id)
  WHERE status IN ('waiting', 'running', 'cancel_requested');
