-- 218: Separate shared-pool owner earnings from the user's ordinary site
-- balance and retain a permanent, pool-independent owner ledger.
--
-- Historical payouts already credited to users.balance are intentionally not
-- copied into the new wallet. They remain visible as legacy entries and are
-- never paid a second time.

CREATE TABLE IF NOT EXISTS shared_pool_owner_wallets (
    owner_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE RESTRICT,
    available_amount NUMERIC(24, 12) NOT NULL DEFAULT 0,
    pending_amount NUMERIC(24, 12) NOT NULL DEFAULT 0,
    frozen_amount NUMERIC(24, 12) NOT NULL DEFAULT 0,
    transferred_amount NUMERIC(24, 12) NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shared_pool_owner_wallets_amount_check
        CHECK (
            available_amount >= 0
            AND pending_amount >= 0
            AND frozen_amount >= 0
            AND transferred_amount >= 0
        )
);

CREATE TABLE IF NOT EXISTS shared_pool_owner_earnings_ledger (
    id BIGSERIAL PRIMARY KEY,
    owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    pool_id BIGINT NULL REFERENCES shared_pools(id) ON DELETE SET NULL,
    account_id BIGINT NULL REFERENCES shared_pool_accounts(id) ON DELETE SET NULL,
    price_version_id BIGINT NULL REFERENCES shared_pool_price_versions(id) ON DELETE RESTRICT,
    event_type VARCHAR(32) NOT NULL,
    operation_id VARCHAR(160) NOT NULL DEFAULT '',
    request_id VARCHAR(160) NOT NULL DEFAULT '',
    pool_name_snapshot VARCHAR(160) NOT NULL DEFAULT '',
    owner_label_snapshot VARCHAR(160) NOT NULL DEFAULT '',
    model_snapshot VARCHAR(160) NOT NULL DEFAULT '',
    pricing_source_snapshot VARCHAR(24) NOT NULL DEFAULT '',
    gross_amount NUMERIC(24, 12) NOT NULL DEFAULT 0,
    platform_fee_amount NUMERIC(24, 12) NOT NULL DEFAULT 0,
    net_amount NUMERIC(24, 12) NOT NULL DEFAULT 0,
    wallet_delta NUMERIC(24, 12) NOT NULL,
    available_after NUMERIC(24, 12) NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'available',
    available_at TIMESTAMPTZ NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    posted_at TIMESTAMPTZ NULL,
    CONSTRAINT shared_pool_owner_earnings_event_check
        CHECK (event_type IN ('earning', 'transfer_to_balance', 'reversal', 'adjustment')),
    CONSTRAINT shared_pool_owner_earnings_status_check
        CHECK (status IN ('pending', 'available', 'settled', 'reversed')),
    CONSTRAINT shared_pool_owner_earnings_split_check
        CHECK (
            gross_amount >= 0
            AND platform_fee_amount >= 0
            AND net_amount >= 0
            AND platform_fee_amount + net_amount <= gross_amount + 0.000000000001
        )
);

CREATE UNIQUE INDEX IF NOT EXISTS shared_pool_owner_earnings_operation_unique
    ON shared_pool_owner_earnings_ledger(owner_id, event_type, operation_id)
    WHERE operation_id <> '';

CREATE INDEX IF NOT EXISTS idx_shared_pool_owner_earnings_owner
    ON shared_pool_owner_earnings_ledger(owner_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_shared_pool_owner_earnings_pool
    ON shared_pool_owner_earnings_ledger(pool_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_shared_pool_owner_earnings_status
    ON shared_pool_owner_earnings_ledger(owner_id, status, available_at);

ALTER TABLE shared_pool_owner_earnings_ledger
    DROP CONSTRAINT IF EXISTS shared_pool_owner_earnings_ledger_price_version_id_fkey,
    ADD CONSTRAINT shared_pool_owner_earnings_ledger_price_version_id_fkey
        FOREIGN KEY (price_version_id)
        REFERENCES shared_pool_price_versions(id)
        ON DELETE RESTRICT
        NOT VALID;

ALTER TABLE shared_pool_owner_earnings_ledger
    VALIDATE CONSTRAINT shared_pool_owner_earnings_ledger_price_version_id_fkey;

INSERT INTO shared_pool_owner_wallets (owner_id)
SELECT DISTINCT owner_id
FROM shared_pools
WHERE owner_id IS NOT NULL
ON CONFLICT (owner_id) DO NOTHING;

ALTER TABLE shared_pool_balance_ledger
    ADD COLUMN IF NOT EXISTS settlement_destination VARCHAR(32) NOT NULL DEFAULT 'legacy_balance',
    ADD COLUMN IF NOT EXISTS pool_name_snapshot VARCHAR(160) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS owner_label_snapshot VARCHAR(160) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS model_snapshot VARCHAR(160) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS price_version_id BIGINT NULL REFERENCES shared_pool_price_versions(id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS owner_earnings_ledger_id BIGINT NULL REFERENCES shared_pool_owner_earnings_ledger(id) ON DELETE SET NULL;

ALTER TABLE shared_pool_balance_ledger
    DROP CONSTRAINT IF EXISTS shared_pool_balance_ledger_price_version_id_fkey,
    ADD CONSTRAINT shared_pool_balance_ledger_price_version_id_fkey
        FOREIGN KEY (price_version_id)
        REFERENCES shared_pool_price_versions(id)
        ON DELETE RESTRICT
        NOT VALID;

ALTER TABLE shared_pool_balance_ledger
    VALIDATE CONSTRAINT shared_pool_balance_ledger_price_version_id_fkey;

-- Classify every historical row by its accounting source independently of the
-- pool relation. Deleted pools have already been detached by ON DELETE SET NULL,
-- so joining shared_pools here would leave their usage and seat-fee rows marked
-- as legacy owner payouts. The owner-ledger guard also makes a retry preserve
-- earnings that were created for the new wallet after this migration started.
UPDATE shared_pool_balance_ledger
SET settlement_destination = CASE source_type
    WHEN 'share_pool_usage' THEN 'user_balance'
    WHEN 'pool_seat_fee' THEN 'user_balance'
    WHEN 'share_pool_payout' THEN 'legacy_balance'
    WHEN 'pool_owner_payout' THEN 'legacy_balance'
    ELSE settlement_destination
END
WHERE owner_earnings_ledger_id IS NULL
  AND source_type IN (
      'share_pool_usage',
      'pool_seat_fee',
      'share_pool_payout',
      'pool_owner_payout'
  );

-- Snapshot labels can only be recovered for pools that still exist. This is
-- intentionally separate from settlement classification above.
UPDATE shared_pool_balance_ledger ledger
SET pool_name_snapshot = COALESCE(NULLIF(ledger.pool_name_snapshot, ''), pool.name, ''),
    owner_label_snapshot = COALESCE(NULLIF(ledger.owner_label_snapshot, ''), pool.owner_label, '')
FROM shared_pools pool
WHERE pool.id = ledger.pool_id
  AND (
      ledger.pool_name_snapshot = ''
      OR ledger.owner_label_snapshot = ''
  );

-- Existing rows start as legacy_balance so the backfill above can classify
-- historical payouts safely. New member usage and seat-fee debits belong to
-- the ordinary user balance even if a future insert omits the column.
ALTER TABLE shared_pool_balance_ledger
    ALTER COLUMN settlement_destination SET DEFAULT 'user_balance';

ALTER TABLE shared_pool_balance_ledger
    DROP CONSTRAINT IF EXISTS shared_pool_balance_ledger_destination_check,
    ADD CONSTRAINT shared_pool_balance_ledger_destination_check
        CHECK (settlement_destination IN ('user_balance', 'owner_wallet', 'legacy_balance'));

CREATE INDEX IF NOT EXISTS idx_shared_pool_balance_ledger_destination
    ON shared_pool_balance_ledger(user_id, settlement_destination, created_at DESC);

-- A wallet-to-site-balance transfer changes users.balance and therefore also
-- needs the platform-wide balance audit trail.
ALTER TABLE user_balance_ledger
    ALTER COLUMN source_id TYPE VARCHAR(160),
    DROP CONSTRAINT IF EXISTS user_balance_ledger_source_type_check,
    ADD CONSTRAINT user_balance_ledger_source_type_check CHECK (source_type IN (
        'daily_checkin',
        'checkin_jackpot_share',
        'credit_lottery_jackpot_win',
        'credit_lottery_jackpot_share',
        'shared_pool_owner_wallet_transfer'
    ));

COMMENT ON TABLE shared_pool_owner_wallets IS
    'Authoritative owner-earnings wallet, separate from users.balance.';
COMMENT ON TABLE shared_pool_owner_earnings_ledger IS
    'Permanent append-only owner ledger with pool and pricing snapshots.';
