-- Migration: 211_shared_pool_oauth_credentials
-- Secure OAuth credential storage and owner-controlled scheduling for shared-pool accounts.

ALTER TABLE shared_pool_accounts
    ADD COLUMN IF NOT EXISTS credentials_encrypted TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS credential_fingerprint VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS auto_pause_on_expired BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS schedulable BOOLEAN NOT NULL DEFAULT TRUE;

CREATE UNIQUE INDEX IF NOT EXISTS idx_shared_pool_accounts_credential_identity
    ON shared_pool_accounts(pool_id, credential_fingerprint)
    WHERE deleted_at IS NULL AND credential_fingerprint <> '';

DROP INDEX IF EXISTS idx_shared_pool_accounts_routing;
CREATE INDEX IF NOT EXISTS idx_shared_pool_accounts_routing
    ON shared_pool_accounts(pool_id, schedulable, status, priority, total_calls, last_used_at)
    WHERE deleted_at IS NULL
      AND (
        (auth_type IN ('apikey', 'api_key') AND upstream_base_url <> '' AND upstream_api_key <> '')
        OR (auth_type = 'oauth' AND credentials_encrypted <> '')
      );
