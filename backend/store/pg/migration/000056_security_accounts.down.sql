-- Hashes deliberately cannot be converted back into reusable credentials.
-- Restore a pre-upgrade backup or rotate credentials before a binary downgrade.
DO $$ BEGIN
  RAISE EXCEPTION 'Security migration is irreversible: API token plaintext cannot be recovered. Restore a pre-upgrade backup before downgrading.';
END $$;
