-- R1 owner-native onboarding. Existing 235 identity dispositions remain immutable.

ALTER TABLE shared_pools
    ADD COLUMN IF NOT EXISTS create_operation_id VARCHAR(160) NULL,
    ADD COLUMN IF NOT EXISTS create_request_fingerprint CHAR(64) NULL,
    ADD COLUMN IF NOT EXISTS native_onboarding_state VARCHAR(48) NULL;

UPDATE shared_pools
SET native_onboarding_state = 'legacy_existing'
WHERE native_onboarding_state IS NULL;

ALTER TABLE shared_pools
    ALTER COLUMN native_onboarding_state SET DEFAULT 'draft',
    ALTER COLUMN native_onboarding_state SET NOT NULL,
    ADD CONSTRAINT shared_pools_native_onboarding_state_check CHECK (
        native_onboarding_state IN ('draft','supply_configuring','supply_needs_attention','supply_ready_billing_blocked','legacy_existing')
    ) NOT VALID;
ALTER TABLE shared_pools VALIDATE CONSTRAINT shared_pools_native_onboarding_state_check;

CREATE UNIQUE INDEX IF NOT EXISTS shared_pools_owner_create_operation_unique
    ON shared_pools(owner_id, create_operation_id)
    WHERE owner_id IS NOT NULL AND create_operation_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS shared_pool_native_onboarding_progress (
    source_kind VARCHAR(24) NOT NULL DEFAULT 'pool_account',
    source_id BIGINT NOT NULL REFERENCES shared_pool_accounts(id) ON DELETE RESTRICT,
    pool_id BIGINT NOT NULL REFERENCES shared_pools(id) ON DELETE RESTRICT,
    owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    native_binding_ref UUID NOT NULL,
    native_operation_id VARCHAR(160) NOT NULL,
    native_request_fingerprint CHAR(64) NOT NULL,
    native_binding_state VARCHAR(32) NOT NULL DEFAULT 'pending',
    native_binding_step VARCHAR(64) NOT NULL DEFAULT 'claim',
    native_binding_error_code VARCHAR(80) NOT NULL DEFAULT '',
    native_binding_error_message VARCHAR(500) NOT NULL DEFAULT '',
    native_account_observed_updated_at TIMESTAMPTZ NULL,
    native_repair_operation_id VARCHAR(160) NULL,
    native_repair_request_fingerprint CHAR(64) NULL,
    native_repair_expected_config_version BIGINT NULL,
    native_repair_state VARCHAR(24) NOT NULL DEFAULT '',
    native_repair_started_at TIMESTAMPTZ NULL,
    native_repair_completed_at TIMESTAMPTZ NULL,
    native_models JSONB NOT NULL DEFAULT '[]'::jsonb,
    native_models_verified_at TIMESTAMPTZ NULL,
    native_connection_status VARCHAR(48) NOT NULL DEFAULT 'unverified',
    native_connection_verified_at TIMESTAMPTZ NULL,
    native_verification_attempted_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (source_kind, source_id),
    CONSTRAINT shared_pool_native_progress_source_kind_check CHECK (source_kind = 'pool_account'),
    CONSTRAINT shared_pool_native_progress_state_check CHECK (native_binding_state IN ('pending','creating','attached','verifying','ready','needs_attention','detached')),
    CONSTRAINT shared_pool_native_progress_connection_check CHECK (native_connection_status IN ('unverified','verified','failed','authenticated_metadata_reachable')),
    CONSTRAINT shared_pool_native_progress_repair_state_check CHECK (native_repair_state IN ('','pending','applied')),
    CONSTRAINT shared_pool_native_progress_repair_identity_check CHECK (
        (native_repair_operation_id IS NULL AND native_repair_request_fingerprint IS NULL AND native_repair_expected_config_version IS NULL AND native_repair_state = '')
        OR
        (native_repair_operation_id IS NOT NULL AND native_repair_request_fingerprint IS NOT NULL AND native_repair_expected_config_version IS NOT NULL AND native_repair_state <> '')
    ),
    CONSTRAINT shared_pool_native_progress_source_pool_unique UNIQUE (source_id, pool_id),
    CONSTRAINT shared_pool_native_progress_owner_operation_unique UNIQUE (pool_id, native_operation_id),
    CONSTRAINT shared_pool_native_progress_binding_ref_unique UNIQUE (native_binding_ref)
);

CREATE OR REPLACE FUNCTION validate_shared_pool_native_progress_source()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM shared_pool_accounts source
        WHERE source.id = NEW.source_id
          AND source.pool_id = NEW.pool_id
          AND source.owner_id = NEW.owner_id
    ) THEN
        RAISE EXCEPTION 'native onboarding source does not match pool owner';
    END IF;
    RETURN NEW;
END;
$$;
DROP TRIGGER IF EXISTS trg_shared_pool_native_progress_source ON shared_pool_native_onboarding_progress;
CREATE CONSTRAINT TRIGGER trg_shared_pool_native_progress_source
AFTER INSERT OR UPDATE OF source_id, pool_id, owner_id ON shared_pool_native_onboarding_progress
DEFERRABLE INITIALLY IMMEDIATE
FOR EACH ROW EXECUTE FUNCTION validate_shared_pool_native_progress_source();

CREATE UNIQUE INDEX IF NOT EXISTS accounts_shared_pool_binding_ref_unique
    ON accounts((extra->>'shared_pool_binding_ref'))
    WHERE COALESCE(extra->>'shared_pool_binding_ref','') <> '';

CREATE UNIQUE INDEX IF NOT EXISTS shared_pool_native_repair_operation_unique
    ON shared_pool_native_onboarding_progress(pool_id, native_repair_operation_id)
    WHERE native_repair_operation_id IS NOT NULL;

CREATE OR REPLACE FUNCTION bump_shared_pool_config_from_native_progress()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    UPDATE shared_pools SET config_version = config_version + 1, updated_at = NOW()
    WHERE id = NEW.pool_id;
    RETURN NEW;
END;
$$;
DROP TRIGGER IF EXISTS trg_shared_pool_native_progress_config ON shared_pool_native_onboarding_progress;
CREATE TRIGGER trg_shared_pool_native_progress_config
AFTER INSERT OR UPDATE ON shared_pool_native_onboarding_progress
FOR EACH ROW EXECUTE FUNCTION bump_shared_pool_config_from_native_progress();
