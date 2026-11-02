-- Downgrading removes staging metadata only; it cannot restore a deleted index.
ALTER TABLE node_learning_tasks DROP COLUMN index_build_id;
ALTER TABLE node_learning_tasks DROP COLUMN dataset_id;
DROP TABLE kb_index_builds;
DROP TABLE node_index_versions;
