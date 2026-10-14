CREATE TABLE IF NOT EXISTS chat_traces (
  trace_id varchar(64) PRIMARY KEY,
  kb_id varchar(64) NOT NULL,
  conversation_id varchar(64) NOT NULL,
  user_message_id varchar(64) NOT NULL,
  assistant_message_id varchar(64) NOT NULL,
  question text NOT NULL DEFAULT '',
  model varchar(255) NOT NULL DEFAULT '',
  status varchar(32) NOT NULL DEFAULT 'in_progress',
  started_at timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz,
  expires_at timestamptz NOT NULL DEFAULT (now() + interval '30 days'),
  steps jsonb NOT NULL DEFAULT '[]'::jsonb
);
CREATE INDEX IF NOT EXISTS idx_chat_traces_kb_conversation_started
  ON chat_traces(kb_id, conversation_id, started_at);
CREATE INDEX IF NOT EXISTS idx_chat_traces_assistant_message
  ON chat_traces(assistant_message_id);
CREATE INDEX IF NOT EXISTS idx_chat_traces_expires_at
  ON chat_traces(expires_at);
