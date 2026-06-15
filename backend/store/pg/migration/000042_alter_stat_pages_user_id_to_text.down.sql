UPDATE stat_pages
SET user_id = NULL
WHERE user_id IS NOT NULL
  AND user_id !~ '^[0-9]+$';

ALTER TABLE stat_pages
ALTER COLUMN user_id TYPE bigint USING user_id::bigint;
