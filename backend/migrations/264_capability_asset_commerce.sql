ALTER TABLE capability_assets ADD COLUMN commerce_price NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (commerce_price >= 0);
CREATE TABLE capability_asset_commerce_policy (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK(id),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO capability_asset_commerce_policy(id) VALUES(TRUE);
CREATE TABLE capability_asset_purchases (
    id BIGSERIAL PRIMARY KEY,
    asset_id BIGINT NOT NULL REFERENCES capability_assets(id) ON DELETE RESTRICT,
    buyer_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    operation_id VARCHAR(100) NOT NULL,
    asset_title TEXT NOT NULL,
    amount NUMERIC(20,8) NOT NULL CHECK(amount > 0),
    platform_fee NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK(platform_fee = 0),
    creator_amount NUMERIC(20,8) NOT NULL CHECK(creator_amount > 0),
    status VARCHAR(12) NOT NULL DEFAULT 'active' CHECK(status IN ('active','refunded')),
    refund_operation_id VARCHAR(100),
    refund_reason VARCHAR(1000) NOT NULL DEFAULT '',
    refunded_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    refunded_at TIMESTAMPTZ,
    UNIQUE(buyer_user_id,operation_id),
    CHECK(buyer_user_id <> owner_user_id),
    CHECK(amount=creator_amount+platform_fee),
    CHECK((status='active' AND refunded_at IS NULL AND refunded_by IS NULL)
       OR (status='refunded' AND refunded_at IS NOT NULL AND refunded_by IS NOT NULL AND refund_operation_id IS NOT NULL AND length(btrim(refund_reason))>0))
);
CREATE UNIQUE INDEX capability_asset_active_purchase ON capability_asset_purchases(asset_id,buyer_user_id) WHERE status='active';
CREATE UNIQUE INDEX capability_asset_refund_operation ON capability_asset_purchases(refunded_by,refund_operation_id) WHERE refund_operation_id IS NOT NULL;
CREATE INDEX capability_asset_purchase_page ON capability_asset_purchases(asset_id,id DESC);
CREATE TABLE capability_asset_commerce_audit (
    id BIGSERIAL PRIMARY KEY,
    actor_user_id BIGINT NOT NULL REFERENCES users(id),
    asset_id BIGINT REFERENCES capability_assets(id),
    action VARCHAR(20) NOT NULL CHECK(action IN ('policy','pricing')),
    detail JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- user_balance_ledger source_type expansion is owned by migration 265.
