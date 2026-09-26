-- Migration: 204_shared_pool_unified_api_key
-- Allow one active shared-pool API key to bind to multiple pools.

-- Build the replacement uniqueness guard before removing the older single-key guard.
CREATE UNIQUE INDEX IF NOT EXISTS shared_pool_access_keys_api_key_pool_active_unique
    ON shared_pool_access_keys(api_key_id, pool_id)
    WHERE status = 'active';

-- Legacy active bindings were created before unified shared-pool API keys used
-- account-mode runtime routing. Backfill only active rows so existing keys keep
-- working after this migration without reviving disabled bindings.
UPDATE shared_pool_access_keys
SET account_mode = TRUE,
    updated_at = NOW()
WHERE status = 'active'
  AND account_mode = FALSE;

DROP INDEX IF EXISTS shared_pool_access_keys_api_key_unique;

CREATE INDEX IF NOT EXISTS idx_api_keys_shared_pool_user_active
    ON api_keys(user_id, created_at, id)
    WHERE deleted_at IS NULL
      AND status = 'active'
      AND key LIKE 'sk-share-%';
