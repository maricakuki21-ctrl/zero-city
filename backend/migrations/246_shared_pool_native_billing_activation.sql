-- Native pool billing activation. Additive only; no native pool is auto-activated.

ALTER TABLE shared_pools
    ADD COLUMN IF NOT EXISTS native_activation_operation_id VARCHAR(160),
    ADD COLUMN IF NOT EXISTS native_activation_fingerprint CHAR(64),
    ADD COLUMN IF NOT EXISTS native_activated_config_version BIGINT,
    ADD COLUMN IF NOT EXISTS native_activated_at TIMESTAMPTZ;

ALTER TABLE shared_pools
    DROP CONSTRAINT IF EXISTS shared_pools_native_onboarding_state_check,
    ADD CONSTRAINT shared_pools_native_onboarding_state_check CHECK (
        native_onboarding_state IN (
            'draft','supply_configuring','supply_needs_attention',
            'supply_ready_billing_blocked','billing_active','legacy_existing'
        )
    ) NOT VALID;
ALTER TABLE shared_pools VALIDATE CONSTRAINT shared_pools_native_onboarding_state_check;

CREATE OR REPLACE FUNCTION bump_shared_pool_config_from_native_progress()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' OR (
        OLD.native_binding_state,
        OLD.native_models,
        OLD.native_connection_status,
        OLD.native_account_observed_updated_at
    ) IS DISTINCT FROM (
        NEW.native_binding_state,
        NEW.native_models,
        NEW.native_connection_status,
        NEW.native_account_observed_updated_at
    ) THEN
        UPDATE shared_pools SET config_version = config_version + 1, updated_at = NOW()
        WHERE id = NEW.pool_id;
    END IF;
    RETURN NEW;
END;
$$;

-- Runtime disablement during an activation revocation must not recursively
-- fence the same pool through migration 216's shared-pool account trigger.
CREATE OR REPLACE FUNCTION bump_shared_pool_config_version_from_account()
RETURNS TRIGGER AS $$
DECLARE
    target_pool_id BIGINT;
BEGIN
    IF current_setting('bizdecipher.native_activation_revocation', TRUE) = '1' THEN
        RETURN NEW;
    END IF;

    IF TG_OP = 'INSERT' THEN
        target_pool_id := NEW.pool_id;
        UPDATE shared_pools SET config_version = config_version + 1 WHERE id = target_pool_id;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        target_pool_id := OLD.pool_id;
        UPDATE shared_pools SET config_version = config_version + 1 WHERE id = target_pool_id;
        RETURN OLD;
    END IF;

    target_pool_id := NEW.pool_id;
    IF ROW(
        NEW.provider, NEW.auth_type, NEW.upstream_base_url, NEW.upstream_api_key,
        NEW.credentials_encrypted, NEW.expires_at, NEW.auto_pause_on_expired,
        NEW.schedulable, NEW.status, NEW.status_note, NEW.disabled_reason,
        NEW.proxy_id, NEW.proxy_url, NEW.proxy_region, NEW.proxy_status,
        NEW.account_weight, NEW.priority, NEW.rpm_limit, NEW.account_concurrency,
        NEW.user_concurrency, NEW.tls_profile_id, NEW.ttl_seconds,
        NEW.cache_policy, NEW.routing_policy, NEW.model_configs,
        NEW.gate_required, NEW.deleted_at
    ) IS DISTINCT FROM ROW(
        OLD.provider, OLD.auth_type, OLD.upstream_base_url, OLD.upstream_api_key,
        OLD.credentials_encrypted, OLD.expires_at, OLD.auto_pause_on_expired,
        OLD.schedulable, OLD.status, OLD.status_note, OLD.disabled_reason,
        OLD.proxy_id, OLD.proxy_url, OLD.proxy_region, OLD.proxy_status,
        OLD.account_weight, OLD.priority, OLD.rpm_limit, OLD.account_concurrency,
        OLD.user_concurrency, OLD.tls_profile_id, OLD.ttl_seconds,
        OLD.cache_policy, OLD.routing_policy, OLD.model_configs,
        OLD.gate_required, OLD.deleted_at
    ) THEN
        UPDATE shared_pools SET config_version = config_version + 1 WHERE id = target_pool_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION revoke_native_pool_activation_on_config_change()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE canonical_group BIGINT;
BEGIN
    IF OLD.native_onboarding_state = 'billing_active'
       AND NEW.config_version IS DISTINCT FROM OLD.config_version
       AND NEW.config_version IS DISTINCT FROM NEW.native_activated_config_version THEN
        NEW.native_onboarding_state := 'supply_ready_billing_blocked';
        NEW.listed := FALSE;
        NEW.status := 'offline';
        NEW.lifecycle_state := 'draft';
        SELECT canonical_group_id INTO canonical_group
        FROM shared_pool_sub2_bindings WHERE pool_id = NEW.id;
        PERFORM set_config('bizdecipher.native_activation_revocation', '1', TRUE);
        UPDATE shared_pool_accounts SET schedulable = FALSE, updated_at = NOW()
        WHERE pool_id = NEW.id AND deleted_at IS NULL;
        UPDATE accounts SET schedulable = FALSE, updated_at = NOW()
        WHERE id IN (
            SELECT canonical_account_id FROM shared_pool_supply_dispositions
            WHERE pool_id = NEW.id AND source_kind = 'pool_account' AND disposition = 'mapped'
        );
        UPDATE groups SET status = 'disabled', updated_at = NOW()
        WHERE id = canonical_group;
        PERFORM set_config('bizdecipher.native_activation_revocation', '0', TRUE);
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_revoke_native_pool_activation ON shared_pools;
CREATE TRIGGER trg_revoke_native_pool_activation
BEFORE UPDATE OF config_version ON shared_pools
FOR EACH ROW EXECUTE FUNCTION revoke_native_pool_activation_on_config_change();
