ALTER TABLE zip_upload_tasks
  ADD COLUMN IF NOT EXISTS total_files integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS processed_files integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS total_bytes bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS processed_bytes bigint NOT NULL DEFAULT 0;
