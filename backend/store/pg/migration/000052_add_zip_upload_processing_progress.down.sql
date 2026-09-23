ALTER TABLE zip_upload_tasks
  DROP COLUMN IF EXISTS total_files,
  DROP COLUMN IF EXISTS processed_files,
  DROP COLUMN IF EXISTS total_bytes,
  DROP COLUMN IF EXISTS processed_bytes;
