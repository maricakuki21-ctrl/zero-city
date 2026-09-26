-- 216: Durable shared-pool probe jobs.
--
-- Full capability checks can outlive an HTTP request.  Persisting the work and
-- fencing the result with shared_pools.config_version prevents a disconnected
-- browser (or an old result racing a configuration edit) from losing or
-- incorrectly publishing the probe outcome.

ALTER TABLE shared_pools
    ADD COLUMN IF NOT EXISTS config_version BIGINT NOT NULL DEFAULT 1;

ALTER TABLE shared_pools
    DROP CONSTRAINT IF EXISTS shared_pools_config_version_check,
    ADD CONSTRAINT shared_pools_config_version_check CHECK (config_version > 0);

CREATE TABLE IF NOT EXISTS shared_pool_probe_jobs (
    id UUID PRIMARY KEY,
    operation_id VARCHAR(128) NOT NULL,
    pool_id BIGINT NOT NULL REFERENCES shared_pools(id) ON DELETE CASCADE,
    account_id BIGINT NULL REFERENCES shared_pool_accounts(id) ON DELETE SET NULL,
    owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    model_name VARCHAR(255) NOT NULL DEFAULT '',
    upstream_model_name VARCHAR(255) NOT NULL DEFAULT '',
    probe_type VARCHAR(32) NOT NULL DEFAULT 'manual',
    check_level VARCHAR(16) NOT NULL DEFAULT 'full',
    config_version BIGINT NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'queued',
    attempt INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    lease_owner VARCHAR(160) NOT NULL DEFAULT '',
    lease_expires_at TIMESTAMPTZ NULL,
    heartbeat_at TIMESTAMPTZ NULL,
    started_at TIMESTAMPTZ NULL,
    finished_at TIMESTAMPTZ NULL,
    result_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_type VARCHAR(80) NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shared_pool_probe_jobs_operation_unique UNIQUE (owner_id, operation_id),
    CONSTRAINT shared_pool_probe_jobs_probe_type_check
        CHECK (probe_type IN ('manual', 'publish_gate', 'scheduled', 'scheduled_full')),
    CONSTRAINT shared_pool_probe_jobs_check_level_check
        CHECK (check_level IN ('basic', 'full', 'oauth')),
    CONSTRAINT shared_pool_probe_jobs_status_check
        CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'timed_out', 'stale', 'cancelled')),
    CONSTRAINT shared_pool_probe_jobs_attempt_check
        CHECK (attempt >= 0 AND max_attempts > 0 AND attempt <= max_attempts),
    CONSTRAINT shared_pool_probe_jobs_config_version_check CHECK (config_version > 0)
);

CREATE INDEX IF NOT EXISTS idx_shared_pool_probe_jobs_claim
    ON shared_pool_probe_jobs(status, available_at, created_at)
    WHERE status IN ('queued', 'running');

CREATE INDEX IF NOT EXISTS idx_shared_pool_probe_jobs_owner
    ON shared_pool_probe_jobs(owner_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_shared_pool_probe_jobs_pool
    ON shared_pool_probe_jobs(pool_id, created_at DESC);

CREATE TABLE IF NOT EXISTS shared_pool_probe_job_items (
    id BIGSERIAL PRIMARY KEY,
    job_id UUID NOT NULL REFERENCES shared_pool_probe_jobs(id) ON DELETE CASCADE,
    item_index INT NOT NULL,
    check_id VARCHAR(120) NOT NULL DEFAULT '',
    title VARCHAR(255) NOT NULL DEFAULT '',
    category VARCHAR(80) NOT NULL DEFAULT '',
    required BOOLEAN NOT NULL DEFAULT FALSE,
    success BOOLEAN NOT NULL DEFAULT FALSE,
    http_status INT NOT NULL DEFAULT 0,
    latency_ms INT NOT NULL DEFAULT 0,
    error_type VARCHAR(80) NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    evidence TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shared_pool_probe_job_items_unique UNIQUE (job_id, item_index)
);

CREATE INDEX IF NOT EXISTS idx_shared_pool_probe_job_items_job
    ON shared_pool_probe_job_items(job_id, item_index);

ALTER TABLE shared_pool_probe_histories
    ADD COLUMN IF NOT EXISTS job_id UUID NULL REFERENCES shared_pool_probe_jobs(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS config_version BIGINT NULL;

CREATE INDEX IF NOT EXISTS idx_shared_pool_probe_histories_job
    ON shared_pool_probe_histories(job_id)
    WHERE job_id IS NOT NULL;

-- Pool-level edits increment the fence when probe configuration or explicit
-- operating intent changes. A result may not undo a concurrent manual
-- maintenance, unlisting, governance, or archive action.
CREATE OR REPLACE FUNCTION bump_shared_pool_config_version()
RETURNS TRIGGER AS $$
BEGIN
    IF ROW(
        NEW.upstream_base_url,
        NEW.upstream_api_key,
        NEW.proxy_id,
        NEW.proxy_url,
        NEW.proxy_region,
        NEW.proxy_status,
        NEW.account_concurrency,
        NEW.user_concurrency,
        NEW.account_mode_enabled,
        NEW.oauth_provider,
        NEW.verification_mode,
        NEW.verification_exemption_reason,
        NEW.status,
        NEW.listed,
        NEW.status_note,
        NEW.disabled_reason,
        NEW.governance_status,
        NEW.governance_note,
        NEW.admin_note,
        NEW.lifecycle_state,
        NEW.archived_at
    ) IS DISTINCT FROM ROW(
        OLD.upstream_base_url,
        OLD.upstream_api_key,
        OLD.proxy_id,
        OLD.proxy_url,
        OLD.proxy_region,
        OLD.proxy_status,
        OLD.account_concurrency,
        OLD.user_concurrency,
        OLD.account_mode_enabled,
        OLD.oauth_provider,
        OLD.verification_mode,
        OLD.verification_exemption_reason,
        OLD.status,
        OLD.listed,
        OLD.status_note,
        OLD.disabled_reason,
        OLD.governance_status,
        OLD.governance_note,
        OLD.admin_note,
        OLD.lifecycle_state,
        OLD.archived_at
    ) THEN
        NEW.config_version := GREATEST(NEW.config_version, OLD.config_version + 1);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_shared_pools_config_version ON shared_pools;
CREATE TRIGGER trg_shared_pools_config_version
    BEFORE UPDATE ON shared_pools
    FOR EACH ROW EXECUTE FUNCTION bump_shared_pool_config_version();

-- Account and model changes also invalidate an in-flight pool probe.  Probe
-- result fields are deliberately excluded from the account update trigger.
CREATE OR REPLACE FUNCTION bump_shared_pool_config_version_from_account()
RETURNS TRIGGER AS $$
DECLARE
    target_pool_id BIGINT;
BEGIN
    IF TG_OP = 'INSERT' THEN
        target_pool_id := NEW.pool_id;
        UPDATE shared_pools
        SET config_version = config_version + 1
        WHERE id = target_pool_id;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        target_pool_id := OLD.pool_id;
        UPDATE shared_pools
        SET config_version = config_version + 1
        WHERE id = target_pool_id;
        RETURN OLD;
    END IF;

    target_pool_id := NEW.pool_id;
    IF ROW(
        NEW.provider,
        NEW.auth_type,
        NEW.upstream_base_url,
        NEW.upstream_api_key,
        NEW.credentials_encrypted,
        NEW.expires_at,
        NEW.auto_pause_on_expired,
        NEW.schedulable,
        NEW.status,
        NEW.status_note,
        NEW.disabled_reason,
        NEW.proxy_id,
        NEW.proxy_url,
        NEW.proxy_region,
        NEW.proxy_status,
        NEW.account_weight,
        NEW.priority,
        NEW.rpm_limit,
        NEW.account_concurrency,
        NEW.user_concurrency,
        NEW.tls_profile_id,
        NEW.ttl_seconds,
        NEW.cache_policy,
        NEW.routing_policy,
        NEW.model_configs,
        NEW.gate_required,
        NEW.deleted_at
    ) IS DISTINCT FROM ROW(
        OLD.provider,
        OLD.auth_type,
        OLD.upstream_base_url,
        OLD.upstream_api_key,
        OLD.credentials_encrypted,
        OLD.expires_at,
        OLD.auto_pause_on_expired,
        OLD.schedulable,
        OLD.status,
        OLD.status_note,
        OLD.disabled_reason,
        OLD.proxy_id,
        OLD.proxy_url,
        OLD.proxy_region,
        OLD.proxy_status,
        OLD.account_weight,
        OLD.priority,
        OLD.rpm_limit,
        OLD.account_concurrency,
        OLD.user_concurrency,
        OLD.tls_profile_id,
        OLD.ttl_seconds,
        OLD.cache_policy,
        OLD.routing_policy,
        OLD.model_configs,
        OLD.gate_required,
        OLD.deleted_at
    ) THEN
        UPDATE shared_pools
        SET config_version = config_version + 1
        WHERE id = target_pool_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_shared_pool_accounts_config_version ON shared_pool_accounts;
CREATE TRIGGER trg_shared_pool_accounts_config_version
    AFTER INSERT OR UPDATE OR DELETE ON shared_pool_accounts
    FOR EACH ROW EXECUTE FUNCTION bump_shared_pool_config_version_from_account();

CREATE OR REPLACE FUNCTION bump_shared_pool_config_version_from_model()
RETURNS TRIGGER AS $$
DECLARE
    target_pool_id BIGINT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        target_pool_id := OLD.pool_id;
    ELSIF TG_OP = 'UPDATE' THEN
        -- Only routing/capability changes invalidate queued probe results.
        -- Pricing and owner-facing presentation updates are deliberately
        -- excluded so publishing a price version cannot stale a passed gate.
        IF NOT (
            NEW.pool_id IS DISTINCT FROM OLD.pool_id
            OR NEW.provider IS DISTINCT FROM OLD.provider
            OR NEW.model_name IS DISTINCT FROM OLD.model_name
            OR NEW.upstream_model_name IS DISTINCT FROM OLD.upstream_model_name
            OR NEW.model_aliases IS DISTINCT FROM OLD.model_aliases
            OR NEW.enabled IS DISTINCT FROM OLD.enabled
            OR NEW.model_open IS DISTINCT FROM OLD.model_open
            OR NEW.max_concurrency IS DISTINCT FROM OLD.max_concurrency
        ) THEN
            RETURN NEW;
        END IF;

        -- A model moved between pools changes both routing graphs.
        IF NEW.pool_id IS DISTINCT FROM OLD.pool_id THEN
            UPDATE shared_pools
            SET config_version = config_version + 1
            WHERE id = OLD.pool_id;
        END IF;
        target_pool_id := NEW.pool_id;
    ELSE
        target_pool_id := NEW.pool_id;
    END IF;
    UPDATE shared_pools
    SET config_version = config_version + 1
    WHERE id = target_pool_id;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_shared_pool_models_config_version ON shared_pool_models;
CREATE TRIGGER trg_shared_pool_models_config_version
    AFTER INSERT OR UPDATE OR DELETE ON shared_pool_models
    FOR EACH ROW EXECUTE FUNCTION bump_shared_pool_config_version_from_model();
