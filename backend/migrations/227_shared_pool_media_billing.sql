-- 227: Money-safe shared-pool media endpoints and asynchronous video ownership.
--
-- This migration is additive. It does not delete or rewrite any pool, seat,
-- key, ledger, reservation, balance, backup, or historical usage row.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE shared_pool_usage_reservations
    DROP CONSTRAINT IF EXISTS shared_pool_usage_reservation_endpoint_check,
    ADD CONSTRAINT shared_pool_usage_reservation_endpoint_check CHECK (
        endpoint_type IN ('chat', 'responses', 'image_generation', 'image_edit', 'video')
    ) NOT VALID;

ALTER TABLE shared_pool_usage_reservations
    VALIDATE CONSTRAINT shared_pool_usage_reservation_endpoint_check;

ALTER TABLE shared_pool_usage_traces
    ADD COLUMN IF NOT EXISTS image_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS image_size VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS video_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS video_resolution VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS video_duration_seconds INT NOT NULL DEFAULT 0;

ALTER TABLE shared_pool_usage_traces
    DROP CONSTRAINT IF EXISTS shared_pool_usage_traces_media_check,
    ADD CONSTRAINT shared_pool_usage_traces_media_check CHECK (
        image_count >= 0 AND video_count >= 0 AND video_duration_seconds >= 0
    ) NOT VALID;

ALTER TABLE shared_pool_usage_traces
    VALIDATE CONSTRAINT shared_pool_usage_traces_media_check;

-- Media evidence is fenced at three layers. Pool config_version already
-- covers pool/model/account route changes (migration 216); the account-local
-- version additionally prevents a result captured for one credential snapshot
-- from being applied to a later snapshot of the same account row.
ALTER TABLE shared_pool_accounts
    ADD COLUMN IF NOT EXISTS config_version BIGINT NOT NULL DEFAULT 1;

ALTER TABLE shared_pool_accounts
    DROP CONSTRAINT IF EXISTS shared_pool_accounts_config_version_check,
    ADD CONSTRAINT shared_pool_accounts_config_version_check
        CHECK (config_version > 0) NOT VALID;

ALTER TABLE shared_pool_accounts
    VALIDATE CONSTRAINT shared_pool_accounts_config_version_check;

CREATE UNIQUE INDEX IF NOT EXISTS shared_pool_accounts_id_pool_unique
    ON shared_pool_accounts(id, pool_id);

CREATE OR REPLACE FUNCTION bump_shared_pool_account_local_config_version()
RETURNS TRIGGER AS $$
BEGIN
    IF ROW(
        NEW.provider, NEW.auth_type, NEW.upstream_base_url,
        NEW.upstream_api_key, NEW.credentials_encrypted, NEW.expires_at,
        NEW.auto_pause_on_expired, NEW.schedulable, NEW.status,
        NEW.status_note, NEW.disabled_reason, NEW.proxy_id, NEW.proxy_url,
        NEW.proxy_region, NEW.proxy_status, NEW.account_weight, NEW.priority,
        NEW.rpm_limit, NEW.account_concurrency, NEW.user_concurrency,
        NEW.tls_profile_id, NEW.ttl_seconds, NEW.cache_policy,
        NEW.routing_policy, NEW.model_configs, NEW.gate_required, NEW.deleted_at
    ) IS DISTINCT FROM ROW(
        OLD.provider, OLD.auth_type, OLD.upstream_base_url,
        OLD.upstream_api_key, OLD.credentials_encrypted, OLD.expires_at,
        OLD.auto_pause_on_expired, OLD.schedulable, OLD.status,
        OLD.status_note, OLD.disabled_reason, OLD.proxy_id, OLD.proxy_url,
        OLD.proxy_region, OLD.proxy_status, OLD.account_weight, OLD.priority,
        OLD.rpm_limit, OLD.account_concurrency, OLD.user_concurrency,
        OLD.tls_profile_id, OLD.ttl_seconds, OLD.cache_policy,
        OLD.routing_policy, OLD.model_configs, OLD.gate_required, OLD.deleted_at
    ) THEN
        NEW.config_version := GREATEST(NEW.config_version, OLD.config_version + 1);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_shared_pool_account_local_config_version
    ON shared_pool_accounts;
CREATE TRIGGER trg_shared_pool_account_local_config_version
BEFORE UPDATE ON shared_pool_accounts
FOR EACH ROW EXECUTE FUNCTION bump_shared_pool_account_local_config_version();

-- Media gates are independent from the ordinary chat/full-check probe. A
-- media endpoint can become passed only through a durable endpoint-specific
-- probe row containing observed output evidence.
CREATE TABLE IF NOT EXISTS shared_pool_media_endpoint_probes (
    id BIGSERIAL PRIMARY KEY,
    endpoint_id BIGINT NOT NULL REFERENCES shared_pool_model_endpoints(id) ON DELETE RESTRICT,
    pool_id BIGINT NOT NULL REFERENCES shared_pools(id) ON DELETE RESTRICT,
    account_id BIGINT NULL REFERENCES shared_pool_accounts(id) ON DELETE RESTRICT,
    endpoint_type VARCHAR(24) NOT NULL,
    operation_id VARCHAR(160) NOT NULL,
    pool_config_version BIGINT NOT NULL,
    account_config_version BIGINT NULL,
    probe_plan_version BIGINT NOT NULL,
    endpoint_config_version BIGINT NOT NULL,
    result_status VARCHAR(24) NOT NULL,
    upstream_http_status INT NULL,
    output_observed BOOLEAN NOT NULL DEFAULT FALSE,
    async_terminal_observed BOOLEAN NOT NULL DEFAULT FALSE,
    error_type VARCHAR(80) NOT NULL DEFAULT '',
    error_message VARCHAR(500) NOT NULL DEFAULT '',
    checked_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shared_pool_media_endpoint_probes_endpoint_check
        CHECK (endpoint_type IN ('image_generation', 'image_edit', 'video')),
    CONSTRAINT shared_pool_media_endpoint_probes_result_check
        CHECK (result_status IN ('passed', 'failed')),
    CONSTRAINT shared_pool_media_endpoint_probes_http_check
        CHECK (upstream_http_status IS NULL OR upstream_http_status BETWEEN 100 AND 599),
    CONSTRAINT shared_pool_media_endpoint_probes_expiry_check
        CHECK (expires_at > checked_at),
    CONSTRAINT shared_pool_media_endpoint_probes_versions_check
        CHECK (
            pool_config_version > 0
            AND probe_plan_version > 0
            AND endpoint_config_version > 0
            AND ((account_id IS NULL AND account_config_version IS NULL)
                 OR (account_id IS NOT NULL AND account_config_version > 0))
        ),
    CONSTRAINT shared_pool_media_endpoint_probes_passed_evidence_check
        CHECK (
            result_status <> 'passed'
            OR (output_observed AND (endpoint_type <> 'video' OR async_terminal_observed))
        ),
    CONSTRAINT shared_pool_media_endpoint_probes_operation_unique
        UNIQUE (endpoint_id, operation_id)
);

-- Make a partially applied development migration safe to rerun.
ALTER TABLE shared_pool_media_endpoint_probes
    ADD COLUMN IF NOT EXISTS pool_config_version BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS account_config_version BIGINT NULL;

UPDATE shared_pool_media_endpoint_probes probe
SET pool_config_version = pool.config_version,
    account_config_version = (
        SELECT account.config_version
        FROM shared_pool_accounts account
        WHERE account.id = probe.account_id AND account.pool_id = probe.pool_id
    )
FROM shared_pools pool
WHERE pool.id = probe.pool_id
  AND (probe.pool_config_version IS DISTINCT FROM pool.config_version
       OR probe.account_config_version IS DISTINCT FROM (
           SELECT account.config_version
           FROM shared_pool_accounts account
           WHERE account.id = probe.account_id AND account.pool_id = probe.pool_id
       ));

ALTER TABLE shared_pool_media_endpoint_probes
    ALTER COLUMN pool_config_version DROP DEFAULT,
    DROP CONSTRAINT IF EXISTS shared_pool_media_endpoint_probes_versions_check,
    ADD CONSTRAINT shared_pool_media_endpoint_probes_versions_check
        CHECK (
            pool_config_version > 0
            AND probe_plan_version > 0
            AND endpoint_config_version > 0
            AND ((account_id IS NULL AND account_config_version IS NULL)
                 OR (account_id IS NOT NULL AND account_config_version > 0))
        ) NOT VALID,
    DROP CONSTRAINT IF EXISTS shared_pool_media_endpoint_probes_passed_evidence_check,
    ADD CONSTRAINT shared_pool_media_endpoint_probes_passed_evidence_check
        CHECK (
            result_status <> 'passed'
            OR (output_observed AND (endpoint_type <> 'video' OR async_terminal_observed))
        ) NOT VALID,
    DROP CONSTRAINT IF EXISTS shared_pool_media_endpoint_probes_account_pool_fkey,
    ADD CONSTRAINT shared_pool_media_endpoint_probes_account_pool_fkey
        FOREIGN KEY (account_id, pool_id)
        REFERENCES shared_pool_accounts(id, pool_id)
        ON DELETE RESTRICT
        NOT VALID;

ALTER TABLE shared_pool_media_endpoint_probes
    VALIDATE CONSTRAINT shared_pool_media_endpoint_probes_versions_check,
    VALIDATE CONSTRAINT shared_pool_media_endpoint_probes_passed_evidence_check,
    VALIDATE CONSTRAINT shared_pool_media_endpoint_probes_account_pool_fkey;

ALTER TABLE shared_pool_model_endpoints
    ADD COLUMN IF NOT EXISTS last_media_probe_id BIGINT NULL,
    ADD COLUMN IF NOT EXISTS last_media_probe_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS media_probe_expires_at TIMESTAMPTZ NULL;

ALTER TABLE shared_pool_model_endpoints
    DROP CONSTRAINT IF EXISTS shared_pool_model_endpoints_last_media_probe_fkey,
    ADD CONSTRAINT shared_pool_model_endpoints_last_media_probe_fkey
        FOREIGN KEY (last_media_probe_id)
        REFERENCES shared_pool_media_endpoint_probes(id)
        ON DELETE RESTRICT
        NOT VALID;

ALTER TABLE shared_pool_model_endpoints
    VALIDATE CONSTRAINT shared_pool_model_endpoints_last_media_probe_fkey;

CREATE OR REPLACE FUNCTION validate_shared_pool_media_probe_scope()
RETURNS TRIGGER AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM shared_pool_model_endpoints endpoint
        JOIN shared_pool_models model ON model.id = endpoint.pool_model_id
        JOIN shared_pools pool ON pool.id = model.pool_id
        WHERE endpoint.id = NEW.endpoint_id
          AND endpoint.endpoint_type = NEW.endpoint_type
          AND pool.id = NEW.pool_id
          AND pool.config_version = NEW.pool_config_version
          AND endpoint.probe_plan_version = NEW.probe_plan_version
          AND endpoint.config_version = NEW.endpoint_config_version
    ) THEN
        RAISE EXCEPTION 'media probe endpoint/pool/config scope is stale or invalid'
            USING ERRCODE = '23514';
    END IF;
    IF NEW.account_id IS NULL THEN
        IF NEW.account_config_version IS NOT NULL THEN
            RAISE EXCEPTION 'pool-level media probe cannot carry an account version'
                USING ERRCODE = '23514';
        END IF;
    ELSIF NOT EXISTS (
        SELECT 1
        FROM shared_pool_accounts account
        WHERE account.id = NEW.account_id
          AND account.pool_id = NEW.pool_id
          AND account.config_version = NEW.account_config_version
          AND account.deleted_at IS NULL
    ) THEN
        RAISE EXCEPTION 'media probe account/pool/config scope is stale or invalid'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_validate_shared_pool_media_probe_scope
    ON shared_pool_media_endpoint_probes;
CREATE TRIGGER trg_validate_shared_pool_media_probe_scope
BEFORE INSERT OR UPDATE ON shared_pool_media_endpoint_probes
FOR EACH ROW EXECUTE FUNCTION validate_shared_pool_media_probe_scope();

CREATE OR REPLACE FUNCTION validate_shared_pool_endpoint_last_media_probe()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.last_media_probe_id IS NOT NULL AND NOT EXISTS (
        SELECT 1
        FROM shared_pool_media_endpoint_probes probe
        WHERE probe.id = NEW.last_media_probe_id
          AND probe.endpoint_id = NEW.id
          AND probe.endpoint_type = NEW.endpoint_type
    ) THEN
        RAISE EXCEPTION 'last media probe does not belong to this endpoint'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_validate_shared_pool_endpoint_last_media_probe
    ON shared_pool_model_endpoints;
CREATE TRIGGER trg_validate_shared_pool_endpoint_last_media_probe
BEFORE INSERT OR UPDATE OF last_media_probe_id, endpoint_type
ON shared_pool_model_endpoints
FOR EACH ROW EXECUTE FUNCTION validate_shared_pool_endpoint_last_media_probe();

-- Any route-affecting endpoint edit invalidates its old evidence. Probe result
-- projection itself only updates gate/evidence columns and does not retrigger
-- this configuration fence.
CREATE OR REPLACE FUNCTION invalidate_shared_pool_media_endpoint_on_config_change()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.endpoint_type IN ('image_generation', 'image_edit', 'video')
       OR NEW.endpoint_type IN ('image_generation', 'image_edit', 'video') THEN
        NEW.config_version := GREATEST(NEW.config_version, OLD.config_version + 1);
        NEW.gate_status := CASE
            WHEN OLD.gate_status = 'unverified' THEN 'unverified'
            ELSE 'stale'
        END;
        NEW.media_probe_expires_at := NULL;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_invalidate_shared_pool_media_endpoint_config
    ON shared_pool_model_endpoints;
CREATE TRIGGER trg_invalidate_shared_pool_media_endpoint_config
BEFORE UPDATE OF pool_model_id, endpoint_type, upstream_path, adapter,
                 async_mode, enabled, probe_plan_version
ON shared_pool_model_endpoints
FOR EACH ROW EXECUTE FUNCTION invalidate_shared_pool_media_endpoint_on_config_change();

-- Pool config_version changes include pool URL/key/proxy edits and every
-- route-affecting account/model edit from migration 216. Fence all media
-- endpoints atomically so an old successful probe can never survive them.
CREATE OR REPLACE FUNCTION invalidate_shared_pool_media_endpoints_from_pool()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.config_version IS DISTINCT FROM OLD.config_version THEN
        UPDATE shared_pool_model_endpoints endpoint
        SET config_version = endpoint.config_version + 1,
            gate_status = CASE
                WHEN endpoint.gate_status = 'unverified' THEN 'unverified'
                ELSE 'stale'
            END,
            media_probe_expires_at = NULL,
            updated_at = NOW()
        FROM shared_pool_models model
        WHERE model.id = endpoint.pool_model_id
          AND model.pool_id = NEW.id
          AND endpoint.endpoint_type IN ('image_generation', 'image_edit', 'video');
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_invalidate_shared_pool_media_endpoints_from_pool
    ON shared_pools;
CREATE TRIGGER trg_invalidate_shared_pool_media_endpoints_from_pool
AFTER UPDATE OF config_version ON shared_pools
FOR EACH ROW EXECUTE FUNCTION invalidate_shared_pool_media_endpoints_from_pool();

CREATE INDEX IF NOT EXISTS idx_shared_pool_media_endpoint_probes_history
    ON shared_pool_media_endpoint_probes(endpoint_id, checked_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_shared_pool_media_endpoint_gate_expiry
    ON shared_pool_model_endpoints(media_probe_expires_at, id)
    WHERE gate_status = 'passed';

-- Existing media-looking models receive endpoint rows, but remain unverified.
-- A full media probe must move gate_status to passed before routing is allowed.
INSERT INTO shared_pool_model_endpoints (
    pool_model_id, endpoint_type, upstream_path, adapter, async_mode,
    enabled, gate_status, pricing_status
)
SELECT spm.id, endpoint.endpoint_type, endpoint.upstream_path,
       'openai_compatible', FALSE, spm.enabled, 'unverified', 'pending'
FROM shared_pool_models spm
CROSS JOIN (
    VALUES
        ('image_generation'::VARCHAR, '/v1/images/generations'::TEXT),
        ('image_edit'::VARCHAR, '/v1/images/edits'::TEXT)
) AS endpoint(endpoint_type, upstream_path)
WHERE LOWER(spm.model_name) ~ '(image|imagen|dall-e|grok-imagine)'
  AND LOWER(spm.model_name) !~ '(video|sora|veo)'
ON CONFLICT (pool_model_id, endpoint_type) DO NOTHING;

INSERT INTO shared_pool_model_endpoints (
    pool_model_id, endpoint_type, upstream_path, adapter, async_mode,
    enabled, gate_status, pricing_status
)
SELECT spm.id, 'video', '/v1/videos/generations',
       'grok_media', TRUE, spm.enabled, 'unverified', 'pending'
FROM shared_pool_models spm
WHERE LOWER(spm.model_name) ~ '(video|sora|veo)'
ON CONFLICT (pool_model_id, endpoint_type) DO NOTHING;

CREATE OR REPLACE FUNCTION ensure_shared_pool_media_endpoints()
RETURNS TRIGGER AS $$
BEGIN
    IF LOWER(NEW.model_name) ~ '(image|imagen|dall-e|grok-imagine)'
       AND LOWER(NEW.model_name) !~ '(video|sora|veo)' THEN
        INSERT INTO shared_pool_model_endpoints (
            pool_model_id, endpoint_type, upstream_path, adapter, async_mode,
            enabled, gate_status, pricing_status
        ) VALUES
            (NEW.id, 'image_generation', '/v1/images/generations', 'openai_compatible', FALSE, NEW.enabled, 'unverified', 'pending'),
            (NEW.id, 'image_edit', '/v1/images/edits', 'openai_compatible', FALSE, NEW.enabled, 'unverified', 'pending')
        ON CONFLICT (pool_model_id, endpoint_type)
        DO UPDATE SET enabled = EXCLUDED.enabled, updated_at = NOW();
    END IF;
    IF LOWER(NEW.model_name) ~ '(video|sora|veo)' THEN
        INSERT INTO shared_pool_model_endpoints (
            pool_model_id, endpoint_type, upstream_path, adapter, async_mode,
            enabled, gate_status, pricing_status
        ) VALUES
            (NEW.id, 'video', '/v1/videos/generations', 'grok_media', TRUE, NEW.enabled, 'unverified', 'pending')
        ON CONFLICT (pool_model_id, endpoint_type)
        DO UPDATE SET enabled = EXCLUDED.enabled, updated_at = NOW();
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_shared_pool_media_endpoints ON shared_pool_models;
CREATE TRIGGER trg_shared_pool_media_endpoints
AFTER INSERT OR UPDATE OF model_name, enabled ON shared_pool_models
FOR EACH ROW EXECUTE FUNCTION ensure_shared_pool_media_endpoints();

CREATE TABLE IF NOT EXISTS shared_pool_media_tasks (
    id BIGSERIAL PRIMARY KEY,
    reservation_id BIGINT NOT NULL REFERENCES shared_pool_usage_reservations(id) ON DELETE RESTRICT,
    access_key_id BIGINT NOT NULL REFERENCES shared_pool_access_keys(id) ON DELETE RESTRICT,
    pool_id BIGINT NOT NULL REFERENCES shared_pools(id) ON DELETE RESTRICT,
    account_id BIGINT NULL REFERENCES shared_pool_accounts(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    price_version_id BIGINT NOT NULL REFERENCES shared_pool_price_versions(id) ON DELETE RESTRICT,
    endpoint_type VARCHAR(24) NOT NULL DEFAULT 'video',
    provider VARCHAR(32) NOT NULL,
    model_snapshot VARCHAR(200) NOT NULL,
    upstream_model_snapshot VARCHAR(200) NOT NULL,
    upstream_request_id VARCHAR(200) NOT NULL,
    reservation_request_id VARCHAR(160) NOT NULL,
    requested_resolution VARCHAR(32) NOT NULL,
    requested_duration_seconds INT NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'submitted',
    last_upstream_status VARCHAR(64) NOT NULL DEFAULT '',
    last_http_status INT NULL,
    last_polled_at TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shared_pool_media_tasks_reservation_unique UNIQUE (reservation_id),
    CONSTRAINT shared_pool_media_tasks_upstream_unique UNIQUE (pool_id, upstream_request_id),
    CONSTRAINT shared_pool_media_tasks_endpoint_check CHECK (endpoint_type = 'video'),
    CONSTRAINT shared_pool_media_tasks_duration_check CHECK (
        requested_duration_seconds BETWEEN 1 AND 15
    ),
    CONSTRAINT shared_pool_media_tasks_status_check CHECK (
        status IN ('submitted', 'processing', 'succeeded', 'failed', 'review_required')
    ),
    CONSTRAINT shared_pool_media_tasks_http_status_check CHECK (
        last_http_status IS NULL OR (last_http_status >= 100 AND last_http_status <= 599)
    )
);

CREATE INDEX IF NOT EXISTS idx_shared_pool_media_tasks_owner_poll
    ON shared_pool_media_tasks(access_key_id, upstream_request_id);

CREATE INDEX IF NOT EXISTS idx_shared_pool_media_tasks_recovery
    ON shared_pool_media_tasks(expires_at, id)
    WHERE status IN ('submitted', 'processing');

COMMENT ON TABLE shared_pool_media_tasks IS
    'Credential-free ownership and settlement state for asynchronous shared-pool video requests.';
