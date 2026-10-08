DROP INDEX IF EXISTS idx_node_learning_tasks_dispatch;

ALTER TABLE node_learning_tasks
  DROP COLUMN IF EXISTS priority;
