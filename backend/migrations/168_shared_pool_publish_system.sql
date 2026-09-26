-- Shared account publish system foundation.
-- Adds provider-side controls needed for an account-hosting style shared pool:
-- proxy binding, account/per-user concurrency, and account-mode routing marker.

ALTER TABLE shared_pools
    ADD COLUMN IF NOT EXISTS proxy_id BIGINT REFERENCES proxies(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS proxy_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS proxy_region VARCHAR(120) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS proxy_status VARCHAR(30) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS account_concurrency INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS user_concurrency INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS account_mode_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS oauth_provider VARCHAR(40) NOT NULL DEFAULT 'openai',
    ADD COLUMN IF NOT EXISTS oauth_subject TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_shared_pools_proxy_id ON shared_pools(proxy_id);
CREATE INDEX IF NOT EXISTS idx_shared_pools_account_mode ON shared_pools(account_mode_enabled, listed, status);

ALTER TABLE shared_pool_models
    ADD COLUMN IF NOT EXISTS daily_protection_percent NUMERIC(5, 2) NOT NULL DEFAULT 100,
    ADD COLUMN IF NOT EXISTS model_open BOOLEAN NOT NULL DEFAULT TRUE;

CREATE INDEX IF NOT EXISTS idx_shared_pool_models_open_model ON shared_pool_models(model_open, model_name);

ALTER TABLE shared_pool_access_keys
    ADD COLUMN IF NOT EXISTS account_mode BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_shared_pool_access_keys_account_mode
    ON shared_pool_access_keys(user_id, account_mode, status);
