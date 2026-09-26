-- 217: Versioned shared-pool endpoint and pricing contract.
--
-- Official catalog models continue to resolve their base price from the
-- platform pricing resolver. Owner-defined models must publish a complete,
-- append-only price version before they can be routed.

ALTER TABLE shared_pool_models
    ADD COLUMN IF NOT EXISTS pricing_source VARCHAR(24) NOT NULL DEFAULT 'official_catalog',
    ADD COLUMN IF NOT EXISTS pricing_status VARCHAR(24) NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS pricing_config_version BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS custom_pricing JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS pricing_updated_at TIMESTAMPTZ NULL;

ALTER TABLE shared_pool_models
    DROP CONSTRAINT IF EXISTS shared_pool_models_pricing_source_check,
    ADD CONSTRAINT shared_pool_models_pricing_source_check
        CHECK (pricing_source IN ('official_catalog', 'owner_custom')),
    DROP CONSTRAINT IF EXISTS shared_pool_models_pricing_status_check,
    ADD CONSTRAINT shared_pool_models_pricing_status_check
        CHECK (pricing_status IN ('pending', 'ready', 'invalid'));

CREATE TABLE IF NOT EXISTS shared_pool_model_endpoints (
    id BIGSERIAL PRIMARY KEY,
    pool_model_id BIGINT NOT NULL REFERENCES shared_pool_models(id) ON DELETE RESTRICT,
    endpoint_type VARCHAR(32) NOT NULL,
    upstream_path TEXT NOT NULL DEFAULT '',
    adapter VARCHAR(80) NOT NULL DEFAULT 'openai_compatible',
    async_mode BOOLEAN NOT NULL DEFAULT FALSE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    gate_status VARCHAR(24) NOT NULL DEFAULT 'unverified',
    pricing_status VARCHAR(24) NOT NULL DEFAULT 'pending',
    probe_plan_version BIGINT NOT NULL DEFAULT 1,
    config_version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shared_pool_model_endpoints_type_check
        CHECK (endpoint_type IN (
            'chat', 'responses', 'image_generation', 'image_edit',
            'audio_transcription', 'audio_speech', 'video'
        )),
    CONSTRAINT shared_pool_model_endpoints_gate_check
        CHECK (gate_status IN ('unverified', 'passed', 'failed', 'stale')),
    CONSTRAINT shared_pool_model_endpoints_pricing_check
        CHECK (pricing_status IN ('pending', 'ready', 'invalid'))
);

CREATE UNIQUE INDEX IF NOT EXISTS shared_pool_model_endpoints_unique
    ON shared_pool_model_endpoints(pool_model_id, endpoint_type);

CREATE INDEX IF NOT EXISTS idx_shared_pool_model_endpoints_routing
    ON shared_pool_model_endpoints(endpoint_type, enabled, gate_status, pricing_status);

CREATE TABLE IF NOT EXISTS shared_pool_price_versions (
    id BIGSERIAL PRIMARY KEY,
    pool_id BIGINT NOT NULL REFERENCES shared_pools(id) ON DELETE RESTRICT,
    pool_model_id BIGINT NOT NULL REFERENCES shared_pool_models(id) ON DELETE RESTRICT,
    endpoint_id BIGINT NOT NULL REFERENCES shared_pool_model_endpoints(id) ON DELETE RESTRICT,
    version_no BIGINT NOT NULL,
    config_version BIGINT NOT NULL,
    operation_id VARCHAR(128) NOT NULL,
    source VARCHAR(24) NOT NULL,
    billing_mode VARCHAR(24) NOT NULL,
    currency VARCHAR(8) NOT NULL DEFAULT 'USD',
    input_price NUMERIC(24, 12) NULL,
    output_price NUMERIC(24, 12) NULL,
    cache_read_price NUMERIC(24, 12) NULL,
    cache_write_price NUMERIC(24, 12) NULL,
    image_item_price NUMERIC(24, 12) NULL,
    video_second_price NUMERIC(24, 12) NULL,
    audio_minute_price NUMERIC(24, 12) NULL,
    per_request_price NUMERIC(24, 12) NULL,
    multiplier NUMERIC(12, 6) NOT NULL DEFAULT 1,
    minimum_charge NUMERIC(24, 12) NULL,
    maximum_charge NUMERIC(24, 12) NULL,
    effective_from TIMESTAMPTZ NOT NULL,
    price_hash VARCHAR(128) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by BIGINT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shared_pool_price_versions_source_check
        CHECK (source IN ('official_catalog', 'owner_custom')),
    CONSTRAINT shared_pool_price_versions_mode_check
        CHECK (billing_mode IN ('token', 'per_request', 'image', 'video', 'audio')),
    CONSTRAINT shared_pool_price_versions_currency_check
        CHECK (currency IN ('USD')),
    CONSTRAINT shared_pool_price_versions_multiplier_check
        CHECK (multiplier > 0),
    CONSTRAINT shared_pool_price_versions_nonnegative_check
        CHECK (
            COALESCE(input_price, 0) >= 0
            AND COALESCE(output_price, 0) >= 0
            AND COALESCE(cache_read_price, 0) >= 0
            AND COALESCE(cache_write_price, 0) >= 0
            AND COALESCE(image_item_price, 0) >= 0
            AND COALESCE(video_second_price, 0) >= 0
            AND COALESCE(audio_minute_price, 0) >= 0
            AND COALESCE(per_request_price, 0) >= 0
            AND COALESCE(minimum_charge, 0) >= 0
            AND COALESCE(maximum_charge, 0) >= 0
        ),
    CONSTRAINT shared_pool_price_versions_bounds_check
        CHECK (
            maximum_charge IS NULL
            OR minimum_charge IS NULL
            OR maximum_charge >= minimum_charge
        ),
    CONSTRAINT shared_pool_price_versions_complete_check
        CHECK (
            (billing_mode = 'token' AND input_price IS NOT NULL AND output_price IS NOT NULL)
            OR (billing_mode = 'per_request' AND per_request_price IS NOT NULL)
            OR (billing_mode = 'image' AND (image_item_price IS NOT NULL OR per_request_price IS NOT NULL))
            OR (billing_mode = 'video' AND video_second_price IS NOT NULL)
            OR (billing_mode = 'audio' AND (audio_minute_price IS NOT NULL OR per_request_price IS NOT NULL))
        )
);

-- Make a partially applied earlier skeleton safe to upgrade and safe to rerun.
ALTER TABLE shared_pool_price_versions
    ADD COLUMN IF NOT EXISTS config_version BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS operation_id VARCHAR(128) NOT NULL DEFAULT '';

UPDATE shared_pool_price_versions
SET operation_id = 'legacy:' || id::TEXT
WHERE operation_id = '';

ALTER TABLE shared_pool_price_versions
    ALTER COLUMN operation_id DROP DEFAULT,
    DROP CONSTRAINT IF EXISTS shared_pool_price_versions_complete_check,
    ADD CONSTRAINT shared_pool_price_versions_complete_check
        CHECK (
            (billing_mode = 'token' AND input_price IS NOT NULL AND output_price IS NOT NULL)
            OR (billing_mode = 'per_request' AND per_request_price IS NOT NULL)
            OR (billing_mode = 'image' AND (image_item_price IS NOT NULL OR per_request_price IS NOT NULL))
            OR (billing_mode = 'video' AND video_second_price IS NOT NULL)
            OR (billing_mode = 'audio' AND (audio_minute_price IS NOT NULL OR per_request_price IS NOT NULL))
        ) NOT VALID;

-- Append-only rows cannot coexist with an ON DELETE SET NULL author FK: a
-- user deletion would otherwise issue an implicit UPDATE and trip the
-- immutability trigger. Keep the historical author id protected instead.
ALTER TABLE shared_pool_price_versions
    DROP CONSTRAINT IF EXISTS shared_pool_price_versions_created_by_fkey,
    ADD CONSTRAINT shared_pool_price_versions_created_by_fkey
        FOREIGN KEY (created_by)
        REFERENCES users(id)
        ON DELETE RESTRICT
        NOT VALID;

ALTER TABLE shared_pool_price_versions
    VALIDATE CONSTRAINT shared_pool_price_versions_complete_check,
    VALIDATE CONSTRAINT shared_pool_price_versions_created_by_fkey;

CREATE OR REPLACE FUNCTION reject_shared_pool_price_version_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'shared_pool_price_versions is append-only: % is forbidden', TG_OP
        USING ERRCODE = '55000';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_reject_shared_pool_price_version_mutation
    ON shared_pool_price_versions;
CREATE TRIGGER trg_reject_shared_pool_price_version_mutation
BEFORE UPDATE OR DELETE ON shared_pool_price_versions
FOR EACH ROW EXECUTE FUNCTION reject_shared_pool_price_version_mutation();

CREATE UNIQUE INDEX IF NOT EXISTS shared_pool_price_versions_number_unique
    ON shared_pool_price_versions(endpoint_id, version_no);

CREATE UNIQUE INDEX IF NOT EXISTS shared_pool_price_versions_operation_unique
    ON shared_pool_price_versions(endpoint_id, operation_id);

DROP INDEX IF EXISTS shared_pool_price_versions_hash_unique;

CREATE INDEX IF NOT EXISTS idx_shared_pool_price_versions_hash
    ON shared_pool_price_versions(endpoint_id, price_hash);

CREATE INDEX IF NOT EXISTS idx_shared_pool_price_versions_effective
    ON shared_pool_price_versions(endpoint_id, effective_from DESC, version_no DESC);

-- Existing text-model pools have two implemented gateway surfaces. They remain
-- unverified/pending until the application resolves an official price and a
-- fresh probe; this migration does not pretend that image/video support exists.
INSERT INTO shared_pool_model_endpoints (
    pool_model_id,
    endpoint_type,
    upstream_path,
    adapter,
    async_mode,
    enabled,
    gate_status,
    pricing_status
)
SELECT spm.id, endpoint.endpoint_type, endpoint.upstream_path,
       'openai_compatible', FALSE, spm.enabled, 'unverified', 'pending'
FROM shared_pool_models spm
CROSS JOIN (
    VALUES
        ('chat'::VARCHAR, '/v1/chat/completions'::TEXT),
        ('responses'::VARCHAR, '/v1/responses'::TEXT)
) AS endpoint(endpoint_type, upstream_path)
ON CONFLICT (pool_model_id, endpoint_type) DO NOTHING;

CREATE OR REPLACE FUNCTION ensure_shared_pool_text_endpoints()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO shared_pool_model_endpoints (
        pool_model_id, endpoint_type, upstream_path, adapter, async_mode,
        enabled, gate_status, pricing_status
    ) VALUES
        (NEW.id, 'chat', '/v1/chat/completions', 'openai_compatible', FALSE, NEW.enabled, 'unverified', 'pending'),
        (NEW.id, 'responses', '/v1/responses', 'openai_compatible', FALSE, NEW.enabled, 'unverified', 'pending')
    ON CONFLICT (pool_model_id, endpoint_type)
    DO UPDATE SET enabled = EXCLUDED.enabled, updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_shared_pool_text_endpoints ON shared_pool_models;
CREATE TRIGGER trg_shared_pool_text_endpoints
AFTER INSERT OR UPDATE OF enabled ON shared_pool_models
FOR EACH ROW EXECUTE FUNCTION ensure_shared_pool_text_endpoints();

COMMENT ON TABLE shared_pool_price_versions IS
    'Append-only price snapshots. Accepted requests must retain the selected version id.';
COMMENT ON COLUMN shared_pool_models.pricing_status IS
    'pending/invalid models are not eligible for owner_custom routing.';
