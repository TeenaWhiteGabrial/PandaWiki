ALTER TABLE node_learning_tasks
  ADD COLUMN IF NOT EXISTS priority integer NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_node_learning_tasks_dispatch
  ON node_learning_tasks (kb_id, status, priority DESC, created_at ASC);
