CREATE TABLE node_index_versions (
  id text PRIMARY KEY,
  kb_id text NOT NULL,
  node_id text NOT NULL,
  node_release_id text NOT NULL,
  dataset_id text NOT NULL,
  doc_id text NOT NULL DEFAULT '',
  task_id text NOT NULL DEFAULT '',
  build_id text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT 'building',
  message text NOT NULL DEFAULT '',
  parsed_content text,
  parsed_meta jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX node_index_versions_doc ON node_index_versions(doc_id) WHERE doc_id <> '';
CREATE UNIQUE INDEX node_index_versions_active ON node_index_versions(kb_id,node_id) WHERE status = 'active';
CREATE INDEX node_index_versions_pending ON node_index_versions(status, updated_at);
CREATE INDEX node_index_versions_node ON node_index_versions(kb_id, node_id);
CREATE INDEX node_index_versions_build ON node_index_versions(build_id);
CREATE TABLE kb_index_builds (
  id text PRIMARY KEY,
  kb_id text NOT NULL,
  source_dataset_id text NOT NULL,
  dataset_id text NOT NULL,
  config_key text NOT NULL,
  status text NOT NULL DEFAULT 'building',
  message text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX kb_index_builds_pending ON kb_index_builds(kb_id) WHERE status = 'building';
CREATE INDEX kb_index_builds_history ON kb_index_builds(kb_id, created_at DESC);
ALTER TABLE node_learning_tasks ADD COLUMN dataset_id text NOT NULL DEFAULT '';
ALTER TABLE node_learning_tasks ADD COLUMN index_build_id text NOT NULL DEFAULT '';
