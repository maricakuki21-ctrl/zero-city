CREATE TABLE IF NOT EXISTS shared_pool_sub2_bindings (
    pool_id BIGINT PRIMARY KEY REFERENCES shared_pools(id) ON DELETE RESTRICT,
    owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    canonical_group_id BIGINT NOT NULL UNIQUE REFERENCES groups(id) ON DELETE RESTRICT,
    lifecycle VARCHAR(24) NOT NULL DEFAULT 'quarantined',
    source_epoch BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shared_pool_sub2_bindings_owner_unique UNIQUE (pool_id, owner_id),
    CONSTRAINT shared_pool_sub2_bindings_lifecycle_check
        CHECK (lifecycle IN ('active', 'quarantined', 'archived')),
    CONSTRAINT shared_pool_sub2_bindings_source_epoch_check CHECK (source_epoch > 0)
);

CREATE TABLE IF NOT EXISTS shared_pool_supply_dispositions (
    source_kind VARCHAR(24) NOT NULL,
    source_id BIGINT NOT NULL,
    pool_id BIGINT NOT NULL REFERENCES shared_pools(id) ON DELETE RESTRICT,
    owner_id BIGINT NULL REFERENCES users(id) ON DELETE RESTRICT,
    canonical_account_id BIGINT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    disposition VARCHAR(24) NOT NULL,
    reason_code VARCHAR(48) NOT NULL,
    credential_hash CHAR(64) NOT NULL,
    source_checksum CHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (source_kind, source_id),
    CONSTRAINT shared_pool_supply_dispositions_source_kind_check
        CHECK (source_kind IN ('pool_default', 'pool_account')),
    CONSTRAINT shared_pool_supply_dispositions_disposition_check
        CHECK (disposition IN ('mapped', 'quarantined', 'invalid', 'manual-review')),
    CONSTRAINT shared_pool_supply_dispositions_hash_check
        CHECK (credential_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT shared_pool_supply_dispositions_checksum_check
        CHECK (source_checksum ~ '^[0-9a-f]{64}$'),
    CONSTRAINT shared_pool_supply_dispositions_mapping_check
        CHECK ((disposition = 'mapped') = (canonical_account_id IS NOT NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_shared_pool_supply_dispositions_account
    ON shared_pool_supply_dispositions(canonical_account_id)
    WHERE canonical_account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_shared_pool_supply_dispositions_pool
    ON shared_pool_supply_dispositions(pool_id, disposition, reason_code);

CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_bizdecipher_supply_identity
    ON accounts (
        (extra->>'bizdecipher_supply_source'),
        (extra->>'bizdecipher_supply_id')
    )
    WHERE deleted_at IS NULL
      AND extra ? 'bizdecipher_supply_source'
      AND extra ? 'bizdecipher_supply_id';

DROP TABLE IF EXISTS task6_supply_candidates;
CREATE TEMP TABLE task6_supply_candidates (
    source_kind VARCHAR(24) NOT NULL,
    source_id BIGINT NOT NULL,
    source_priority INT NOT NULL,
    pool_id BIGINT NOT NULL,
    owner_id BIGINT NULL,
    pool_owner_id BIGINT NULL,
    pool_exists BOOLEAN NOT NULL,
    owner_exists BOOLEAN NOT NULL,
    provider VARCHAR(50) NOT NULL,
    auth_type VARCHAR(24) NOT NULL,
    upstream_base_url TEXT NOT NULL,
    secret_material TEXT NOT NULL,
    credential_fingerprint VARCHAR(128) NOT NULL,
    source_status VARCHAR(24) NOT NULL,
    proxy_id BIGINT NULL,
    source_concurrency INT NOT NULL,
    canonical_priority INT NOT NULL,
    alias_collision BOOLEAN NOT NULL,
    credential_valid BOOLEAN NOT NULL,
    credential_hash CHAR(64) NOT NULL,
    source_checksum CHAR(64) NOT NULL,
    PRIMARY KEY (source_kind, source_id)
);

WITH account_sources AS (
    SELECT
        'pool_account'::varchar AS source_kind,
        spa.id AS source_id,
        1 AS source_priority,
        spa.pool_id,
        spa.owner_id,
        pool.owner_id AS pool_owner_id,
        pool.id IS NOT NULL AND pool.deleted_at IS NULL AS pool_exists,
        owner_user.id IS NOT NULL AND owner_user.deleted_at IS NULL AS owner_exists,
        LOWER(BTRIM(COALESCE(NULLIF(spa.provider, ''), 'openai'))) AS provider,
        LOWER(BTRIM(spa.auth_type)) AS auth_type,
        BTRIM(spa.upstream_base_url) AS upstream_base_url,
        CASE
            WHEN LOWER(BTRIM(spa.auth_type)) IN ('apikey', 'api_key') THEN BTRIM(spa.upstream_api_key)
            WHEN LOWER(BTRIM(spa.auth_type)) = 'oauth' THEN BTRIM(spa.credentials_encrypted)
            ELSE ''
        END AS secret_material,
        LOWER(BTRIM(spa.credential_fingerprint)) AS credential_fingerprint,
        LOWER(BTRIM(spa.status)) AS source_status,
        spa.proxy_id,
        spa.account_concurrency AS source_concurrency,
        GREATEST(spa.priority, 1) AS canonical_priority
    FROM shared_pool_accounts spa
    LEFT JOIN shared_pools pool ON pool.id = spa.pool_id
    LEFT JOIN users owner_user ON owner_user.id = spa.owner_id
    WHERE spa.deleted_at IS NULL
),
pool_sources AS (
    SELECT
        'pool_default'::varchar AS source_kind,
        pool.id AS source_id,
        2 AS source_priority,
        pool.id AS pool_id,
        pool.owner_id,
        pool.owner_id AS pool_owner_id,
        pool.deleted_at IS NULL AS pool_exists,
        owner_user.id IS NOT NULL AND owner_user.deleted_at IS NULL AS owner_exists,
        LOWER(BTRIM(COALESCE(NULLIF(pool.oauth_provider, ''), 'openai'))) AS provider,
        'api_key'::varchar AS auth_type,
        BTRIM(pool.upstream_base_url) AS upstream_base_url,
        BTRIM(pool.upstream_api_key) AS secret_material,
        ''::varchar AS credential_fingerprint,
        LOWER(BTRIM(pool.status)) AS source_status,
        pool.proxy_id,
        pool.account_concurrency AS source_concurrency,
        100 AS canonical_priority
    FROM shared_pools pool
    LEFT JOIN users owner_user ON owner_user.id = pool.owner_id
    WHERE BTRIM(pool.upstream_base_url) <> '' OR BTRIM(pool.upstream_api_key) <> ''
),
all_sources AS (
    SELECT * FROM account_sources
    UNION ALL
    SELECT * FROM pool_sources
),
normalized AS (
    SELECT
        source.*,
        EXISTS (
            SELECT 1
            FROM shared_pool_models model
            WHERE model.pool_id = source.pool_id
              AND model.enabled = TRUE
              AND LOWER(BTRIM(COALESCE(NULLIF(model.upstream_model_name, ''), model.model_name))) <> ''
            GROUP BY LOWER(BTRIM(COALESCE(NULLIF(model.upstream_model_name, ''), model.model_name)))
            HAVING COUNT(DISTINCT LOWER(BTRIM(model.model_name))) > 1
        ) AS alias_collision,
        CASE
            WHEN source.auth_type IN ('apikey', 'api_key') THEN
                source.upstream_base_url ~* '^https?://[^[:space:]]+$'
                AND LENGTH(source.secret_material) >= 8
            WHEN source.auth_type = 'oauth' THEN
                source.credential_fingerprint ~ '^[0-9a-f]{64}$'
                AND LENGTH(source.secret_material) >= 40
                AND MOD(LENGTH(source.secret_material), 4) = 0
                AND source.secret_material ~ '^[A-Za-z0-9+/]+={0,2}$'
            ELSE FALSE
        END AS credential_valid,
        ENCODE(SHA256(CONVERT_TO(CONCAT_WS(
            '|', source.provider, source.auth_type, source.upstream_base_url, source.secret_material
        ), 'UTF8')), 'hex') AS credential_hash
    FROM all_sources source
)
INSERT INTO task6_supply_candidates (
    source_kind, source_id, source_priority, pool_id, owner_id, pool_owner_id,
    pool_exists, owner_exists, provider, auth_type, upstream_base_url,
    secret_material, credential_fingerprint, source_status, proxy_id,
    source_concurrency, canonical_priority, alias_collision,
    credential_valid, credential_hash, source_checksum
)
SELECT
    source_kind, source_id, source_priority, pool_id, owner_id, pool_owner_id,
    pool_exists, owner_exists, provider, auth_type, upstream_base_url,
    secret_material, credential_fingerprint, source_status, proxy_id,
    source_concurrency, canonical_priority, alias_collision,
    credential_valid, credential_hash,
    ENCODE(SHA256(CONVERT_TO(CONCAT_WS(
        '|', source_kind, source_id::text, pool_id::text, COALESCE(owner_id::text, ''),
        provider, auth_type, upstream_base_url, credential_hash, source_status,
        COALESCE(proxy_id::text, ''), source_concurrency::text, canonical_priority::text
    ), 'UTF8')), 'hex')
FROM normalized;

INSERT INTO groups (
    name, description, rate_multiplier, is_exclusive, status,
    duplicate_operation_id, platform
)
SELECT
    LEFT('biz-pool-' || pool.id::text, 100),
    'Canonical Sub2 group for BizDecipher pool ' || pool.id::text,
    pool.rate_multiplier,
    TRUE,
    'disabled',
    'bizdecipher-pool-binding:' || pool.id::text,
    COALESCE((
        SELECT candidate.provider
        FROM task6_supply_candidates candidate
        WHERE candidate.pool_id = pool.id
        ORDER BY candidate.source_priority, candidate.source_id
        LIMIT 1
    ), 'openai')
FROM shared_pools pool
JOIN users owner_user ON owner_user.id = pool.owner_id AND owner_user.deleted_at IS NULL
WHERE pool.deleted_at IS NULL
ON CONFLICT DO NOTHING;

INSERT INTO shared_pool_sub2_bindings (
    pool_id, owner_id, canonical_group_id, lifecycle
)
SELECT
    pool.id,
    pool.owner_id,
    canonical_group.id,
    'quarantined'
FROM shared_pools pool
JOIN users owner_user ON owner_user.id = pool.owner_id AND owner_user.deleted_at IS NULL
JOIN groups canonical_group
  ON canonical_group.duplicate_operation_id = 'bizdecipher-pool-binding:' || pool.id::text
 AND canonical_group.deleted_at IS NULL
WHERE pool.deleted_at IS NULL
ON CONFLICT DO NOTHING;

WITH classified AS (
    SELECT
        candidate.*,
        COUNT(*) OVER (
            PARTITION BY candidate.pool_id, candidate.credential_hash
        ) AS same_pool_count,
        ROW_NUMBER() OVER (
            PARTITION BY candidate.pool_id, candidate.credential_hash
            ORDER BY candidate.source_priority, candidate.source_id
        ) AS same_pool_rank,
        (
            SELECT COUNT(DISTINCT sibling.pool_id)
            FROM task6_supply_candidates sibling
            WHERE sibling.credential_hash = candidate.credential_hash
              AND sibling.credential_valid = TRUE
        ) AS credential_pool_count
    FROM task6_supply_candidates candidate
),
mapped_sources AS (
    SELECT classified.*
    FROM classified
    WHERE credential_valid = TRUE
      AND pool_exists = TRUE
      AND owner_id IS NOT NULL
      AND owner_exists = TRUE
      AND pool_owner_id = owner_id
      AND alias_collision = FALSE
      AND credential_pool_count = 1
      AND (same_pool_count = 1 OR same_pool_rank = 1)
      AND auth_type IN ('apikey', 'api_key')
)
INSERT INTO accounts (
    name, platform, type, credentials, extra, proxy_id,
    concurrency, priority, status, schedulable
)
SELECT
    LEFT('biz-pool-' || source.pool_id::text || '-supply-' || source.source_id::text, 100),
    source.provider,
    'apikey',
    JSONB_BUILD_OBJECT(
        'api_key', source.secret_material,
        'base_url', source.upstream_base_url
    ),
    JSONB_BUILD_OBJECT(
        'bizdecipher_pool_id', source.pool_id,
        'bizdecipher_supply_source', source.source_kind,
        'bizdecipher_supply_id', source.source_id,
        'bizdecipher_source_checksum', source.source_checksum,
        'bizdecipher_source_concurrency', source.source_concurrency
    ),
    source.proxy_id,
    1,
    source.canonical_priority,
    CASE WHEN source.source_status IN ('active', 'healthy', 'limited', 'testing') THEN 'active' ELSE 'disabled' END,
    source.source_status IN ('active', 'healthy', 'limited', 'testing')
FROM mapped_sources source
ON CONFLICT DO NOTHING;

WITH classified AS (
    SELECT
        candidate.*,
        COUNT(*) OVER (
            PARTITION BY candidate.pool_id, candidate.credential_hash
        ) AS same_pool_count,
        ROW_NUMBER() OVER (
            PARTITION BY candidate.pool_id, candidate.credential_hash
            ORDER BY candidate.source_priority, candidate.source_id
        ) AS same_pool_rank,
        (
            SELECT COUNT(DISTINCT sibling.pool_id)
            FROM task6_supply_candidates sibling
            WHERE sibling.credential_hash = candidate.credential_hash
              AND sibling.credential_valid = TRUE
        ) AS credential_pool_count
    FROM task6_supply_candidates candidate
),
decisions AS (
    SELECT
        classified.*,
        CASE
            WHEN credential_valid = FALSE THEN 'invalid'
            WHEN pool_exists = FALSE THEN 'quarantined'
            WHEN owner_id IS NULL OR owner_exists = FALSE OR pool_owner_id IS DISTINCT FROM owner_id THEN 'quarantined'
            WHEN alias_collision = TRUE THEN 'manual-review'
            WHEN credential_pool_count > 1 THEN 'manual-review'
            WHEN same_pool_count > 1 AND same_pool_rank > 1 THEN 'quarantined'
            WHEN auth_type = 'oauth' THEN 'manual-review'
            ELSE 'mapped'
        END AS disposition,
        CASE
            WHEN credential_valid = FALSE THEN 'invalid_credential'
            WHEN pool_exists = FALSE THEN 'orphan_pool'
            WHEN owner_id IS NULL OR owner_exists = FALSE OR pool_owner_id IS DISTINCT FROM owner_id THEN 'orphan_owner'
            WHEN alias_collision = TRUE THEN 'model_alias_collision'
            WHEN credential_pool_count > 1 THEN 'cross_pool_reuse'
            WHEN same_pool_count > 1 AND same_pool_rank > 1 THEN 'duplicate_account'
            WHEN auth_type = 'oauth' THEN 'oauth_reencrypt_required'
            ELSE 'canonicalized'
        END AS reason_code
    FROM classified
)
INSERT INTO shared_pool_supply_dispositions (
    source_kind, source_id, pool_id, owner_id, canonical_account_id,
    disposition, reason_code, credential_hash, source_checksum
)
SELECT
    decision.source_kind,
    decision.source_id,
    decision.pool_id,
    CASE WHEN decision.owner_exists = TRUE THEN decision.owner_id ELSE NULL END,
    CASE WHEN decision.disposition = 'mapped' THEN canonical_account.id ELSE NULL END,
    decision.disposition,
    decision.reason_code,
    decision.credential_hash,
    decision.source_checksum
FROM decisions decision
LEFT JOIN accounts canonical_account
  ON canonical_account.extra->>'bizdecipher_supply_source' = decision.source_kind
 AND canonical_account.extra->>'bizdecipher_supply_id' = decision.source_id::text
 AND canonical_account.deleted_at IS NULL
WHERE decision.disposition <> 'mapped' OR canonical_account.id IS NOT NULL
ON CONFLICT DO NOTHING;

UPDATE shared_pool_sub2_bindings binding
SET lifecycle = 'active', updated_at = NOW()
WHERE binding.lifecycle = 'quarantined'
  AND EXISTS (
      SELECT 1
      FROM shared_pool_supply_dispositions disposition
      WHERE disposition.pool_id = binding.pool_id
        AND disposition.disposition = 'mapped'
  );

UPDATE groups canonical_group
SET status = 'active', updated_at = NOW()
FROM shared_pool_sub2_bindings binding
WHERE binding.canonical_group_id = canonical_group.id
  AND binding.lifecycle = 'active'
  AND canonical_group.status = 'disabled';

INSERT INTO account_groups (account_id, group_id, priority)
SELECT
    disposition.canonical_account_id,
    binding.canonical_group_id,
    candidate.canonical_priority
FROM shared_pool_supply_dispositions disposition
JOIN shared_pool_sub2_bindings binding ON binding.pool_id = disposition.pool_id
JOIN task6_supply_candidates candidate
  ON candidate.source_kind = disposition.source_kind
 AND candidate.source_id = disposition.source_id
WHERE disposition.disposition = 'mapped'
  AND disposition.canonical_account_id IS NOT NULL
ON CONFLICT DO NOTHING;

CREATE OR REPLACE FUNCTION reject_shared_pool_supply_disposition_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'shared_pool_supply_dispositions is append-only';
END;
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_trigger
        WHERE tgname = 'trg_shared_pool_supply_dispositions_immutable'
          AND tgrelid = 'shared_pool_supply_dispositions'::regclass
    ) THEN
        CREATE TRIGGER trg_shared_pool_supply_dispositions_immutable
            BEFORE UPDATE OR DELETE ON shared_pool_supply_dispositions
            FOR EACH ROW EXECUTE FUNCTION reject_shared_pool_supply_disposition_mutation();
    END IF;
END
$$;

CREATE OR REPLACE VIEW shared_pool_supply_quarantine AS
SELECT
    source_kind,
    source_id,
    pool_id,
    owner_id,
    disposition,
    reason_code,
    credential_hash,
    source_checksum,
    created_at
FROM shared_pool_supply_dispositions
WHERE disposition <> 'mapped';

DROP TABLE task6_supply_candidates;
