-- Manual operating intent is separate from probe health and governance.
ALTER TABLE shared_pools ADD COLUMN IF NOT EXISTS owner_paused BOOLEAN NOT NULL DEFAULT FALSE;

CREATE OR REPLACE FUNCTION preserve_shared_pool_owner_pause()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.owner_paused AND NEW.lifecycle_state <> 'archived' THEN
        NEW.listed := FALSE;
        NEW.status := 'maintenance';
        NEW.lifecycle_state := 'suspended';
    END IF;
    RETURN NEW;
END;
$$;
-- Runs after existing config-version / activation triggers.
DROP TRIGGER IF EXISTS zz_preserve_shared_pool_owner_pause ON shared_pools;
CREATE TRIGGER zz_preserve_shared_pool_owner_pause
BEFORE INSERT OR UPDATE ON shared_pools
FOR EACH ROW EXECUTE FUNCTION preserve_shared_pool_owner_pause();
