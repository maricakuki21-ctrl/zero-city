-- Only rows present during migration retain the historical non-payment flow.
ALTER TABLE marketplace_orders
    ADD COLUMN legacy_cooperation BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE marketplace_orders ALTER COLUMN legacy_cooperation SET DEFAULT FALSE;

CREATE TABLE marketplace_funds_policy (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO marketplace_funds_policy(id) VALUES(TRUE);

-- No amount is inferred from historical free-text quotes.
CREATE TABLE marketplace_order_funds (
    order_id BIGINT PRIMARY KEY REFERENCES marketplace_orders(id),
    amount NUMERIC(20,8) NOT NULL CHECK(amount > 0),
    currency VARCHAR(3) NOT NULL DEFAULT 'USD' CHECK(currency='USD'),
    status VARCHAR(12) NOT NULL DEFAULT 'unpaid' CHECK(status IN ('unpaid','held','released','refunded')),
    operation_id VARCHAR(100),
    paid_at TIMESTAMPTZ,
    released_at TIMESTAMPTZ,
    refunded_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK((status='unpaid' AND operation_id IS NULL AND paid_at IS NULL)
       OR (status<>'unpaid' AND operation_id IS NOT NULL AND paid_at IS NOT NULL))
);
CREATE UNIQUE INDEX marketplace_order_funds_operation ON marketplace_order_funds(operation_id) WHERE operation_id IS NOT NULL;
CREATE TABLE marketplace_funds_policy_audit (
    id BIGSERIAL PRIMARY KEY,
    actor_user_id BIGINT NOT NULL REFERENCES users(id),
    enabled BOOLEAN NOT NULL,
    reason TEXT NOT NULL CHECK(length(reason)>0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
