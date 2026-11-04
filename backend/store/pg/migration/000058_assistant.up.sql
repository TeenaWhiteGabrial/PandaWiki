ALTER TABLE conversation_messages ADD COLUMN assistant_sources jsonb NOT NULL DEFAULT '{}';
CREATE TABLE assistant_uploads (
 id text PRIMARY KEY, kb_id text NOT NULL, user_id text NOT NULL, name text NOT NULL,
 format text NOT NULL, size bigint NOT NULL, hash text NOT NULL, object_key text NOT NULL,
 status text NOT NULL DEFAULT 'queued', error text NOT NULL DEFAULT '', report jsonb NOT NULL DEFAULT '{}',
 text text NOT NULL DEFAULT '', template_key text NOT NULL DEFAULT '', preview_key text NOT NULL DEFAULT '',
 lease_until timestamptz, lease_epoch bigint NOT NULL DEFAULT 0,
 expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX assistant_uploads_owner ON assistant_uploads(kb_id,user_id,created_at DESC);
CREATE INDEX assistant_uploads_queue ON assistant_uploads(status,lease_until);
CREATE TABLE assistant_templates (
 id text NOT NULL, version integer NOT NULL, kb_id text NOT NULL, name text NOT NULL, upload_id text NOT NULL REFERENCES assistant_uploads(id),
 state text NOT NULL DEFAULT 'draft', is_default boolean NOT NULL DEFAULT false, permissions jsonb NOT NULL DEFAULT '{}',
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(id,version)
);
CREATE UNIQUE INDEX assistant_template_default ON assistant_templates(kb_id) WHERE is_default AND state='published';
CREATE TABLE assistant_preferences (
 conversation_id text PRIMARY KEY, user_id text NOT NULL, kb_id text NOT NULL, settings jsonb NOT NULL DEFAULT '{}', updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE assistant_runs (
 id text PRIMARY KEY, kb_id text NOT NULL, user_id text NOT NULL, conversation_id text NOT NULL,
 user_message_id text NOT NULL, assistant_message_id text NOT NULL, request_id text NOT NULL, request_hash text NOT NULL,
 request jsonb NOT NULL, state text NOT NULL DEFAULT 'queued', stage text NOT NULL DEFAULT '等待处理', error text NOT NULL DEFAULT '',
 answer text NOT NULL DEFAULT '', format text NOT NULL DEFAULT '', checkpoint jsonb NOT NULL DEFAULT '{}', sources jsonb NOT NULL DEFAULT '{}', steps jsonb NOT NULL DEFAULT '{}',
 attempt integer NOT NULL DEFAULT 0, lease_epoch bigint NOT NULL DEFAULT 0, lease_until timestamptz, artifact_id text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), stage_started_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(kb_id,user_id,request_id)
);
CREATE INDEX assistant_runs_queue ON assistant_runs(state,lease_until,created_at);
CREATE INDEX assistant_runs_history ON assistant_runs(kb_id,user_id,conversation_id,created_at);
CREATE UNIQUE INDEX assistant_run_one_active ON assistant_runs(conversation_id) WHERE state IN ('queued','running');
CREATE TABLE assistant_run_attempts (
 run_id text NOT NULL REFERENCES assistant_runs(id), lease_epoch bigint NOT NULL, state text NOT NULL,
 error text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(run_id,lease_epoch)
);
CREATE TABLE assistant_artifacts (
 id text PRIMARY KEY, run_id text NOT NULL UNIQUE REFERENCES assistant_runs(id), kb_id text NOT NULL, user_id text NOT NULL,
 conversation_id text NOT NULL, family_id text NOT NULL, revision integer NOT NULL, base_id text NOT NULL DEFAULT '',
 name text NOT NULL, format text NOT NULL, size bigint NOT NULL, hash text NOT NULL, object_key text NOT NULL,
 preview_key text NOT NULL DEFAULT '', preview_state text NOT NULL DEFAULT 'unavailable', spec jsonb NOT NULL, sources jsonb NOT NULL,
 expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(family_id,revision)
);
CREATE INDEX assistant_artifacts_owner ON assistant_artifacts(kb_id,user_id,conversation_id,created_at);

CREATE TABLE assistant_objects (
 object_key text PRIMARY KEY, run_id text NOT NULL DEFAULT '', lease_epoch bigint NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX assistant_objects_gc ON assistant_objects(created_at);
