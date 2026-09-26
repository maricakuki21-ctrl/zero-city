-- 206: Version shared-pool settlement rules by effective billing hour.
--
-- Pool owners can change the displayed seat fee and waiver threshold at any time,
-- but the new terms apply only from the next UTC billing hour. Platform fee
-- changes are administrator-governed and follow the same boundary, so an
-- already-started seat or API settlement window never changes mid-hour.

CREATE TABLE IF NOT EXISTS shared_pool_settlement_rule_versions (
    id BIGSERIAL PRIMARY KEY,
    pool_id BIGINT NOT NULL REFERENCES shared_pools(id) ON DELETE CASCADE,
    hourly_seat_fee DECIMAL(20, 8) NOT NULL DEFAULT 0,
    hourly_min_usage_waiver DECIMAL(20, 8) NOT NULL DEFAULT 0,
    platform_fee_percent DECIMAL(5, 2) NOT NULL DEFAULT 10,
    effective_from TIMESTAMPTZ NOT NULL,
    source VARCHAR(24) NOT NULL DEFAULT 'system',
    created_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shared_pool_settlement_rule_versions_fee_check
        CHECK (hourly_seat_fee >= 0 AND hourly_min_usage_waiver >= 0),
    CONSTRAINT shared_pool_settlement_rule_versions_platform_fee_check
        CHECK (platform_fee_percent >= 0 AND platform_fee_percent <= 100),
    CONSTRAINT shared_pool_settlement_rule_versions_source_check
        CHECK (source IN ('bootstrap', 'owner_update', 'admin_governance', 'system'))
);

CREATE UNIQUE INDEX IF NOT EXISTS shared_pool_settlement_rule_versions_pool_effective_unique
    ON shared_pool_settlement_rule_versions(pool_id, effective_from);

CREATE INDEX IF NOT EXISTS idx_shared_pool_settlement_rule_versions_lookup
    ON shared_pool_settlement_rule_versions(pool_id, effective_from DESC, id DESC);

INSERT INTO shared_pool_settlement_rule_versions (
    pool_id,
    hourly_seat_fee,
    hourly_min_usage_waiver,
    platform_fee_percent,
    effective_from,
    source
)
SELECT
    sp.id,
    sp.hourly_seat_fee,
    sp.hourly_min_usage_waiver,
    sp.platform_fee_percent,
    date_trunc('hour', NOW()),
    'bootstrap'
FROM shared_pools sp
WHERE NOT EXISTS (
    SELECT 1
    FROM shared_pool_settlement_rule_versions v
    WHERE v.pool_id = sp.id
);
