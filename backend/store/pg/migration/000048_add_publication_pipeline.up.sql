-- Publication pipeline v2.
-- These tables are deliberately independent from the legacy node/RAG status
-- fields.  A document can therefore keep serving its active revision while a
-- newer revision is being parsed or indexed.

CREATE TABLE IF NOT EXISTS document_revisions (
  id varchar(64) PRIMARY KEY,
  kb_id varchar(64) NOT NULL,
  document_id varchar(64) NOT NULL,
  revision_no bigint NOT NULL,
  title text NOT NULL DEFAULT '',
  source_kind varchar(32) NOT NULL,
  source_blob_id varchar(64),
  content text NOT NULL DEFAULT '',
  editor_content jsonb,
  editor_format varchar(32) NOT NULL DEFAULT '',
  source_sha256 varchar(128) NOT NULL DEFAULT '',
  created_by varchar(64) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  restored_from_id varchar(64),
  CONSTRAINT uq_document_revisions_no UNIQUE (document_id, revision_no)
);
CREATE INDEX IF NOT EXISTS idx_document_revisions_kb_document
  ON document_revisions(kb_id, document_id, revision_no DESC);

CREATE TABLE IF NOT EXISTS parse_artifacts (
  id varchar(64) PRIMARY KEY,
  kb_id varchar(64) NOT NULL,
  revision_id varchar(64) NOT NULL,
  parser_profile varchar(128) NOT NULL,
  structure jsonb NOT NULL DEFAULT '{}'::jsonb,
  preview_blob_id varchar(64),
  content_hash varchar(128) NOT NULL DEFAULT '',
  block_count integer NOT NULL DEFAULT 0,
  char_count integer NOT NULL DEFAULT 0,
  warnings jsonb NOT NULL DEFAULT '[]'::jsonb,
  quality_score double precision NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_parse_artifacts_revision_profile UNIQUE (revision_id, parser_profile)
);
CREATE INDEX IF NOT EXISTS idx_parse_artifacts_kb_revision
  ON parse_artifacts(kb_id, revision_id);

CREATE TABLE IF NOT EXISTS chunks (
  id varchar(128) PRIMARY KEY,
  kb_id varchar(64) NOT NULL,
  document_id varchar(64) NOT NULL,
  revision_id varchar(64) NOT NULL,
  artifact_id varchar(64) NOT NULL,
  ordinal integer NOT NULL,
  text text NOT NULL,
  title_path jsonb NOT NULL DEFAULT '[]'::jsonb,
  locator jsonb NOT NULL DEFAULT '{}'::jsonb,
  token_count integer NOT NULL DEFAULT 0,
  content_hash varchar(128) NOT NULL DEFAULT '',
  lexical_tokens text[] NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_chunks_artifact_ordinal UNIQUE (artifact_id, ordinal)
);
CREATE INDEX IF NOT EXISTS idx_chunks_kb_revision ON chunks(kb_id, revision_id, ordinal);
CREATE INDEX IF NOT EXISTS idx_chunks_lexical_tokens ON chunks USING gin(lexical_tokens);

CREATE TABLE IF NOT EXISTS index_generations (
  id varchar(64) PRIMARY KEY,
  status varchar(32) NOT NULL,
  embedding_config_id varchar(64) NOT NULL,
  embedding_dimensions integer NOT NULL,
  chunker_profile_hash varchar(128) NOT NULL,
  collection_name varchar(255) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  activated_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_index_generations_active
  ON index_generations(status) WHERE status = 'active';

CREATE TABLE IF NOT EXISTS index_builds (
  id varchar(64) PRIMARY KEY,
  kb_id varchar(64) NOT NULL,
  document_id varchar(64) NOT NULL,
  revision_id varchar(64) NOT NULL,
  generation_id varchar(64) NOT NULL DEFAULT 'default',
  artifact_id varchar(64) NOT NULL,
  state varchar(32) NOT NULL,
  expected_count integer NOT NULL DEFAULT 0,
  verified_count integer NOT NULL DEFAULT 0,
  manifest_hash varchar(128) NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  verified_at timestamptz,
  CONSTRAINT uq_index_build_revision_generation UNIQUE (revision_id, generation_id)
);
CREATE INDEX IF NOT EXISTS idx_index_builds_active_lookup
  ON index_builds(kb_id, document_id, state);

CREATE TABLE IF NOT EXISTS publication_attempts (
  id varchar(64) PRIMARY KEY,
  kb_id varchar(64) NOT NULL,
  document_id varchar(64) NOT NULL,
  revision_id varchar(64) NOT NULL,
  build_id varchar(64),
  expected_epoch bigint NOT NULL,
  state varchar(32) NOT NULL,
  failure_code varchar(128) NOT NULL DEFAULT '',
  failure_message text NOT NULL DEFAULT '',
  created_by varchar(64) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  started_at timestamptz,
  finished_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_publication_attempts_document_created
  ON publication_attempts(kb_id, document_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_publication_attempts_active
  ON publication_attempts(document_id)
  WHERE state IN ('queued', 'running', 'ready');

CREATE TABLE IF NOT EXISTS document_publication_state (
  document_id varchar(64) PRIMARY KEY,
  kb_id varchar(64) NOT NULL,
  draft_revision_id varchar(64),
  active_revision_id varchar(64),
  active_build_id varchar(64),
  publication_epoch bigint NOT NULL DEFAULT 0,
  lock_version bigint NOT NULL DEFAULT 1,
  deleted_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_document_publication_state_kb
  ON document_publication_state(kb_id, deleted_at);

CREATE TABLE IF NOT EXISTS publication_jobs (
  id varchar(64) PRIMARY KEY,
  kb_id varchar(64) NOT NULL,
  kind varchar(64) NOT NULL,
  target_id varchar(64) NOT NULL,
  stage varchar(64) NOT NULL,
  state varchar(32) NOT NULL,
  attempt integer NOT NULL DEFAULT 0,
  max_attempts integer NOT NULL DEFAULT 4,
  available_at timestamptz NOT NULL DEFAULT now(),
  lease_until timestamptz,
  lease_token bigint NOT NULL DEFAULT 0,
  deadline timestamptz,
  idempotency_key varchar(255) NOT NULL,
  last_error_code varchar(128) NOT NULL DEFAULT '',
  last_error_message text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_publication_jobs_idempotency UNIQUE (idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_publication_jobs_claim
  ON publication_jobs(state, available_at);

CREATE TABLE IF NOT EXISTS publication_job_steps (
  id varchar(64) PRIMARY KEY,
  job_id varchar(64) NOT NULL,
  stage varchar(64) NOT NULL,
  attempt integer NOT NULL,
  state varchar(32) NOT NULL,
  started_at timestamptz,
  finished_at timestamptz,
  error_code varchar(128) NOT NULL DEFAULT '',
  artifact_ref varchar(255) NOT NULL DEFAULT '',
  trace_id varchar(128) NOT NULL DEFAULT '',
  details jsonb NOT NULL DEFAULT '{}'::jsonb,
  CONSTRAINT uq_publication_job_steps_attempt UNIQUE (job_id, stage, attempt)
);
CREATE INDEX IF NOT EXISTS idx_publication_job_steps_job_stage
  ON publication_job_steps(job_id, stage, attempt);

CREATE TABLE IF NOT EXISTS runtime_logs (
  id bigserial PRIMARY KEY,
  event_time timestamptz NOT NULL DEFAULT now(),
  level varchar(16) NOT NULL,
  service varchar(64) NOT NULL,
  module varchar(128) NOT NULL DEFAULT '',
  trace_id varchar(128) NOT NULL DEFAULT '',
  span_id varchar(128) NOT NULL DEFAULT '',
  request_id varchar(128) NOT NULL DEFAULT '',
  kb_id varchar(64),
  document_id varchar(64),
  revision_id varchar(64),
  job_id varchar(64),
  run_id varchar(64),
  stage varchar(64) NOT NULL DEFAULT '',
  event_code varchar(128) NOT NULL,
  duration_ms bigint,
  safe_message text NOT NULL DEFAULT '',
  safe_fields jsonb NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX IF NOT EXISTS idx_runtime_logs_event_time
  ON runtime_logs(event_time DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_runtime_logs_trace_time
  ON runtime_logs(trace_id, event_time DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_runtime_logs_kb_time
  ON runtime_logs(kb_id, event_time DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_runtime_logs_level_time
  ON runtime_logs(level, event_time DESC, id DESC);

CREATE TABLE IF NOT EXISTS retrieval_runs (
  id varchar(64) PRIMARY KEY,
  kb_id varchar(64) NOT NULL,
  query text NOT NULL,
  rewritten_query text NOT NULL DEFAULT '',
  generation_id varchar(64) NOT NULL DEFAULT '',
  mode varchar(32) NOT NULL,
  state varchar(32) NOT NULL,
  failure_code varchar(128) NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_retrieval_runs_kb_time ON retrieval_runs(kb_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS retrieval_candidates (
  id varchar(64) PRIMARY KEY,
  run_id varchar(64) NOT NULL,
  chunk_id varchar(128) NOT NULL,
  document_id varchar(64) NOT NULL,
  revision_id varchar(64) NOT NULL,
  source varchar(32) NOT NULL,
  lexical_rank integer NOT NULL DEFAULT 0,
  vector_rank integer NOT NULL DEFAULT 0,
  lexical_score double precision NOT NULL DEFAULT 0,
  vector_score double precision NOT NULL DEFAULT 0,
  fusion_score double precision NOT NULL DEFAULT 0,
  filter_reason varchar(128) NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_retrieval_candidates_run ON retrieval_candidates(run_id, fusion_score DESC);

CREATE TABLE IF NOT EXISTS chat_runs (
  id varchar(64) PRIMARY KEY,
  kb_id varchar(64) NOT NULL,
  conversation_id varchar(64) NOT NULL,
  owner_id varchar(64) NOT NULL,
  user_message_id varchar(64) NOT NULL,
  assistant_message_id varchar(64),
  allow_general_knowledge boolean NOT NULL DEFAULT false,
  chat_config_id varchar(64) NOT NULL DEFAULT '',
  generation_id varchar(64) NOT NULL DEFAULT '',
  state varchar(32) NOT NULL,
  answer_kind varchar(32) NOT NULL DEFAULT '',
  failure_code varchar(128) NOT NULL DEFAULT '',
  trace_id varchar(128) NOT NULL DEFAULT '',
  usage jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  started_at timestamptz,
  finished_at timestamptz,
  retry_of varchar(64)
);
CREATE INDEX IF NOT EXISTS idx_chat_runs_owner_time ON chat_runs(owner_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_chat_runs_kb_time ON chat_runs(kb_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS chat_citations (
  id varchar(64) PRIMARY KEY,
  run_id varchar(64) NOT NULL,
  kb_id varchar(64) NOT NULL,
  document_id varchar(64) NOT NULL,
  revision_id varchar(64) NOT NULL,
  chunk_id varchar(128) NOT NULL,
  publication_epoch bigint NOT NULL,
  locator jsonb NOT NULL DEFAULT '{}'::jsonb,
  title_snapshot text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_chat_citations_run ON chat_citations(run_id, created_at, id);

CREATE TABLE IF NOT EXISTS chat_run_events (
  run_id varchar(64) NOT NULL,
  seq bigint NOT NULL,
  event_type varchar(64) NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (run_id, seq)
);
CREATE INDEX IF NOT EXISTS idx_chat_run_events_time ON chat_run_events(run_id, seq);
