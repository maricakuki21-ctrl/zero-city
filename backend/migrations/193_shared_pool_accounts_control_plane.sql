-- Migration: 193_shared_pool_accounts_control_plane
-- Shared pool control plane foundation: one pool can manage multiple upstream accounts.
-- The existing shared_pools.upstream_* columns stay as compatibility fallback.

CREATE TABLE IF NOT EXISTS shared_pool_accounts (
    id BIGSERIAL PRIMARY KEY,
    pool_id BIGINT NOT NULL REFERENCES shared_pools(id) ON DELETE CASCADE,
    owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(160) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    provider VARCHAR(80) NOT NULL DEFAULT 'openai',
    auth_type VARCHAR(40) NOT NULL DEFAULT 'api_key',
    upstream_base_url TEXT NOT NULL DEFAULT '',
    upstream_api_key TEXT NOT NULL DEFAULT '',
    key_preview VARCHAR(96) NOT NULL DEFAULT '',
    status VARCHAR(24) NOT NULL DEFAULT 'active',
    status_note TEXT NOT NULL DEFAULT '',
    disabled_reason TEXT NOT NULL DEFAULT '',
    group_name VARCHAR(120) NOT NULL DEFAULT '',
    proxy_id BIGINT NULL,
    proxy_url TEXT NOT NULL DEFAULT '',
    proxy_region VARCHAR(120) NOT NULL DEFAULT '',
    proxy_status VARCHAR(40) NOT NULL DEFAULT '',
    account_weight DECIMAL(10, 4) NOT NULL DEFAULT 1,
    priority INT NOT NULL DEFAULT 100,
    rpm_limit INT NOT NULL DEFAULT 0,
    account_concurrency INT NOT NULL DEFAULT 1,
    user_concurrency INT NOT NULL DEFAULT 1,
    tls_profile_id BIGINT NULL,
    ttl_seconds INT NOT NULL DEFAULT 0,
    cache_policy JSONB NOT NULL DEFAULT '{}'::jsonb,
    routing_policy JSONB NOT NULL DEFAULT '{}'::jsonb,
    model_configs JSONB NOT NULL DEFAULT '[]'::jsonb,
    last_probe_at TIMESTAMPTZ NULL,
    last_probe_success BOOLEAN NULL,
    last_probe_error_type VARCHAR(80) NOT NULL DEFAULT '',
    last_probe_error_message TEXT NOT NULL DEFAULT '',
    last_successful_probe_at TIMESTAMPTZ NULL,
    full_check_score DECIMAL(8, 4) NOT NULL DEFAULT 0,
    full_check_passed INT NOT NULL DEFAULT 0,
    full_check_total INT NOT NULL DEFAULT 0,
    gate_required BOOLEAN NOT NULL DEFAULT TRUE,
    gate_passed BOOLEAN NOT NULL DEFAULT FALSE,
    total_calls BIGINT NOT NULL DEFAULT 0,
    successful_calls BIGINT NOT NULL DEFAULT 0,
    failed_calls BIGINT NOT NULL DEFAULT 0,
    last_used_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL,
    CONSTRAINT shared_pool_accounts_status_check CHECK (status IN ('active', 'testing', 'limited', 'disabled', 'offline', 'expired')),
    CONSTRAINT shared_pool_accounts_weight_check CHECK (account_weight >= 0),
    CONSTRAINT shared_pool_accounts_rpm_check CHECK (rpm_limit >= 0),
    CONSTRAINT shared_pool_accounts_concurrency_check CHECK (account_concurrency >= 0 AND user_concurrency >= 0),
    CONSTRAINT shared_pool_accounts_full_check_score_check CHECK (full_check_score >= 0 AND full_check_score <= 100)
);

CREATE INDEX IF NOT EXISTS idx_shared_pool_accounts_pool_status
    ON shared_pool_accounts(pool_id, status, priority, account_weight DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_shared_pool_accounts_owner
    ON shared_pool_accounts(owner_id, pool_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_shared_pool_accounts_routing
    ON shared_pool_accounts(pool_id, status, priority, total_calls, last_used_at)
    WHERE deleted_at IS NULL AND upstream_base_url <> '' AND upstream_api_key <> '';

CREATE INDEX IF NOT EXISTS idx_shared_pool_accounts_models_gin
    ON shared_pool_accounts USING GIN(model_configs);

CREATE INDEX IF NOT EXISTS idx_shared_pool_accounts_cache_policy_gin
    ON shared_pool_accounts USING GIN(cache_policy);

ALTER TABLE shared_pool_probe_histories
    ADD COLUMN IF NOT EXISTS account_id BIGINT NULL REFERENCES shared_pool_accounts(id) ON DELETE SET NULL;

ALTER TABLE shared_pool_balance_ledger
    ADD COLUMN IF NOT EXISTS account_id BIGINT NULL REFERENCES shared_pool_accounts(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_shared_pool_probe_histories_account
    ON shared_pool_probe_histories(account_id, checked_at DESC)
    WHERE account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_shared_pool_balance_ledger_account
    ON shared_pool_balance_ledger(account_id, created_at DESC)
    WHERE account_id IS NOT NULL;

INSERT INTO shared_pool_accounts (
    pool_id,
    owner_id,
    name,
    description,
    provider,
    auth_type,
    upstream_base_url,
    upstream_api_key,
    key_preview,
    status,
    proxy_id,
    proxy_url,
    proxy_region,
    proxy_status,
    account_concurrency,
    user_concurrency,
    model_configs,
    gate_required,
    gate_passed,
    created_at,
    updated_at
)
SELECT
    sp.id,
    sp.owner_id,
    COALESCE(NULLIF(sp.name, ''), 'Shared pool') || ' default upstream',
    'Backfilled from shared_pools upstream fields for control-plane compatibility.',
    COALESCE(NULLIF(sp.oauth_provider, ''), 'openai'),
    'api_key',
    sp.upstream_base_url,
    sp.upstream_api_key,
    CASE
        WHEN LENGTH(BTRIM(sp.upstream_api_key)) <= 12 THEN BTRIM(sp.upstream_api_key)
        ELSE SUBSTRING(BTRIM(sp.upstream_api_key) FROM 1 FOR 9) || '****' || RIGHT(BTRIM(sp.upstream_api_key), 4)
    END,
    CASE WHEN sp.status IN ('healthy', 'limited') THEN 'active' ELSE 'disabled' END,
    sp.proxy_id,
    sp.proxy_url,
    sp.proxy_region,
    sp.proxy_status,
    GREATEST(sp.account_concurrency, 1),
    GREATEST(sp.user_concurrency, 1),
    COALESCE((
        SELECT jsonb_agg(jsonb_build_object(
            'provider', spm.provider,
            'model_name', spm.model_name,
            'upstream_model_name', spm.upstream_model_name,
            'rate_multiplier', spm.rate_multiplier,
            'max_concurrency', spm.max_concurrency,
            'model_open', spm.model_open
        ) ORDER BY spm.sort_order, spm.id)
        FROM shared_pool_models spm
        WHERE spm.pool_id = sp.id AND spm.enabled = TRUE
    ), '[]'::jsonb),
    TRUE,
    COALESCE(sp.last_probe_success, FALSE),
    sp.created_at,
    NOW()
FROM shared_pools sp
WHERE sp.owner_id IS NOT NULL
  AND BTRIM(sp.upstream_base_url) <> ''
  AND BTRIM(sp.upstream_api_key) <> ''
  AND NOT EXISTS (
      SELECT 1 FROM shared_pool_accounts spa
      WHERE spa.pool_id = sp.id
        AND spa.deleted_at IS NULL
        AND BTRIM(spa.upstream_base_url) = BTRIM(sp.upstream_base_url)
        AND BTRIM(spa.upstream_api_key) = BTRIM(sp.upstream_api_key)
  );
