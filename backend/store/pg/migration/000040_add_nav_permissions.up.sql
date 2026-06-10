ALTER TABLE navs
  ADD COLUMN IF NOT EXISTS permissions jsonb NOT NULL DEFAULT '{"answerable":"open","visitable":"open","visible":"open"}'::jsonb;

ALTER TABLE nav_releases
  ADD COLUMN IF NOT EXISTS permissions jsonb NOT NULL DEFAULT '{"answerable":"open","visitable":"open","visible":"open"}'::jsonb;

UPDATE navs
SET permissions = '{"answerable":"open","visitable":"open","visible":"open"}'::jsonb
WHERE permissions IS NULL;

UPDATE nav_releases
SET permissions = '{"answerable":"open","visitable":"open","visible":"open"}'::jsonb
WHERE permissions IS NULL;
