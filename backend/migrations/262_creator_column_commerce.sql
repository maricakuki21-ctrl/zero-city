-- USD site-balance purchases; permanent access, no recurring billing.
ALTER TABLE creator_columns ADD COLUMN IF NOT EXISTS mode VARCHAR(8) NOT NULL DEFAULT 'free'
    CHECK (mode IN ('free', 'paid'));
ALTER TABLE creator_columns ADD COLUMN IF NOT EXISTS price NUMERIC(20,8) NOT NULL DEFAULT 0
    CHECK (price >= 0);
ALTER TABLE creator_columns ADD CONSTRAINT creator_columns_price_mode_check
    CHECK ((mode = 'free' AND price = 0) OR (mode = 'paid' AND price > 0));

CREATE TABLE creator_column_commerce_policy (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO creator_column_commerce_policy(id) VALUES(TRUE);

CREATE TABLE creator_column_purchases (
    id BIGSERIAL PRIMARY KEY,
    column_id BIGINT NOT NULL REFERENCES creator_columns(id) ON DELETE RESTRICT,
    buyer_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    operation_id VARCHAR(100) NOT NULL,
    column_title VARCHAR(120) NOT NULL,
    amount NUMERIC(20,8) NOT NULL CHECK (amount > 0),
    platform_fee NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (platform_fee = 0),
    creator_amount NUMERIC(20,8) NOT NULL CHECK (creator_amount > 0),
    status VARCHAR(12) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'refunded')),
    refund_operation_id VARCHAR(100),
    refund_reason VARCHAR(1000) NOT NULL DEFAULT '',
    refunded_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    refunded_at TIMESTAMPTZ,
    UNIQUE(buyer_user_id, operation_id),
    CHECK (buyer_user_id <> owner_user_id),
    CHECK (amount = platform_fee + creator_amount),
    CHECK ((status = 'active' AND refunded_at IS NULL AND refunded_by IS NULL)
        OR (status = 'refunded' AND refunded_at IS NOT NULL AND refunded_by IS NOT NULL
            AND refund_operation_id IS NOT NULL AND length(btrim(refund_reason)) > 0))
);
CREATE UNIQUE INDEX creator_column_active_purchase ON creator_column_purchases(column_id,buyer_user_id)
    WHERE status = 'active';
CREATE UNIQUE INDEX creator_column_refund_operation ON creator_column_purchases(refunded_by,refund_operation_id)
    WHERE refund_operation_id IS NOT NULL;
CREATE INDEX creator_column_purchase_page ON creator_column_purchases(column_id,id DESC);

CREATE TABLE creator_column_commerce_audit (
    id BIGSERIAL PRIMARY KEY,
    actor_user_id BIGINT NOT NULL REFERENCES users(id),
    column_id BIGINT REFERENCES creator_columns(id),
    action VARCHAR(20) NOT NULL CHECK (action IN ('policy','pricing')),
    detail JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE user_balance_ledger DROP CONSTRAINT user_balance_ledger_source_type_check;
ALTER TABLE user_balance_ledger ADD CONSTRAINT user_balance_ledger_source_type_check CHECK (source_type IN (
    'daily_checkin','checkin_jackpot_share','credit_lottery_jackpot_win','credit_lottery_jackpot_share',
    'shared_pool_owner_wallet_transfer','checkin_milestone','creator_column_purchase','creator_column_refund'
));
