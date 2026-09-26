CREATE TABLE IF NOT EXISTS payhip_webhook_orders (
    id BIGSERIAL PRIMARY KEY,
    transaction_id VARCHAR(64) NOT NULL UNIQUE,
    buyer_email VARCHAR(255) NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'USD',
    price_cents BIGINT NOT NULL,
    credit_amount DECIMAL(20,2) NOT NULL,
    product_key VARCHAR(64) NOT NULL,
    product_name VARCHAR(255) NOT NULL DEFAULT '',
    product_link TEXT NOT NULL DEFAULT '',
    payment_type VARCHAR(40) NOT NULL DEFAULT '',
    status VARCHAR(30) NOT NULL DEFAULT 'paid',
    redeem_code VARCHAR(64) NOT NULL UNIQUE,
    claimed_by BIGINT,
    claimed_at TIMESTAMPTZ,
    refunded_at TIMESTAMPTZ,
    amount_refunded BIGINT NOT NULL DEFAULT 0,
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT payhip_webhook_orders_status_check CHECK (status IN ('paid', 'claimed', 'refunded')),
    CONSTRAINT payhip_webhook_orders_price_positive CHECK (price_cents > 0),
    CONSTRAINT payhip_webhook_orders_credit_positive CHECK (credit_amount > 0)
);

CREATE INDEX IF NOT EXISTS idx_payhip_webhook_orders_buyer_email ON payhip_webhook_orders(buyer_email);
CREATE INDEX IF NOT EXISTS idx_payhip_webhook_orders_status ON payhip_webhook_orders(status);
CREATE INDEX IF NOT EXISTS idx_payhip_webhook_orders_claimed_by ON payhip_webhook_orders(claimed_by);
CREATE INDEX IF NOT EXISTS idx_payhip_webhook_orders_created_at ON payhip_webhook_orders(created_at);
