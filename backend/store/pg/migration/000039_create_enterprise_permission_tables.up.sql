CREATE TABLE IF NOT EXISTS departments (
  id varchar(64) PRIMARY KEY,
  name varchar(255) NOT NULL,
  parent_id varchar(64),
  path text NOT NULL DEFAULT '',
  sort integer NOT NULL DEFAULT 0,
  status varchar(32) NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_departments_parent_id ON departments(parent_id);

CREATE TABLE IF NOT EXISTS permission_levels (
  id varchar(64) PRIMARY KEY,
  name varchar(255) NOT NULL,
  code varchar(128) NOT NULL UNIQUE,
  rank integer NOT NULL,
  description text NOT NULL DEFAULT '',
  is_default boolean NOT NULL DEFAULT false,
  status varchar(32) NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_departments (
  id bigserial PRIMARY KEY,
  user_id varchar(64) NOT NULL,
  department_id varchar(64) NOT NULL,
  is_primary boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_departments_user_department ON user_departments(user_id, department_id);
CREATE INDEX IF NOT EXISTS idx_user_departments_user_id ON user_departments(user_id);
CREATE INDEX IF NOT EXISTS idx_user_departments_department_id ON user_departments(department_id);

ALTER TABLE users ADD COLUMN IF NOT EXISTS permission_level_id varchar(64);

