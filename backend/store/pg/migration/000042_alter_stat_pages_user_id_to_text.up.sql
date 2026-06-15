ALTER TABLE stat_pages
ALTER COLUMN user_id TYPE text USING user_id::text;
