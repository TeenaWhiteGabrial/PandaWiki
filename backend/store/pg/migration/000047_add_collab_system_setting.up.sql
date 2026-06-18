INSERT INTO system_settings (key, value, description)
SELECT 'collab', '{"mode":"disabled","block_min_bytes":204800,"block_min_count":120,"block_window":12}', 'Collaboration setting'
WHERE NOT EXISTS (
    SELECT 1 FROM system_settings WHERE key = 'collab'
);
