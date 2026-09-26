-- 256_marketplace_orders.sql
-- Marketplace order lifecycle: quote -> confirm -> deliver -> accept -> settle -> review.
-- This migration deliberately creates no payment, custody or frozen-balance state.
-- `amount_text` is informational until a settlement provider is explicitly wired.

CREATE TABLE IF NOT EXISTS marketplace_orders (
    id BIGSERIAL PRIMARY KEY,
    inquiry_id BIGINT NOT NULL REFERENCES marketplace_inquiries(id) ON DELETE RESTRICT,
    listing_id BIGINT NOT NULL REFERENCES marketplace_listings(id) ON DELETE RESTRICT,
    buyer_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    seller_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'quoted'
        CHECK (status IN ('quoted', 'confirmed', 'delivered', 'accepted', 'settled', 'canceled', 'disputed')),
    scope_text VARCHAR(2000) NOT NULL DEFAULT '',
    amount_text VARCHAR(120) NOT NULL DEFAULT '',
    delivery_text VARCHAR(240) NOT NULL DEFAULT '',
    revision_limit SMALLINT NOT NULL DEFAULT 0 CHECK (revision_limit BETWEEN 0 AND 10),
    delivery_note VARCHAR(2000) NOT NULL DEFAULT '',
    accept_note VARCHAR(2000) NOT NULL DEFAULT '',
    dispute_note VARCHAR(2000) NOT NULL DEFAULT '',
    cancel_reason VARCHAR(500) NOT NULL DEFAULT '',
    quoted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    confirmed_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    accepted_at TIMESTAMPTZ,
    settled_at TIMESTAMPTZ,
    canceled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (buyer_user_id <> seller_user_id)
);

CREATE INDEX IF NOT EXISTS idx_marketplace_orders_buyer
    ON marketplace_orders (buyer_user_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_marketplace_orders_seller
    ON marketplace_orders (seller_user_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_marketplace_orders_inquiry
    ON marketplace_orders (inquiry_id, id DESC);

CREATE TABLE IF NOT EXISTS marketplace_order_events (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES marketplace_orders(id) ON DELETE CASCADE,
    actor_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event VARCHAR(24) NOT NULL,
    from_status VARCHAR(20) NOT NULL,
    to_status VARCHAR(20) NOT NULL,
    note VARCHAR(1000) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_marketplace_order_events_order
    ON marketplace_order_events (order_id, id);

CREATE TABLE IF NOT EXISTS marketplace_order_reviews (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES marketplace_orders(id) ON DELETE CASCADE,
    author_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    body VARCHAR(2000) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (order_id, author_user_id)
);

CREATE INDEX IF NOT EXISTS idx_marketplace_order_reviews_order
    ON marketplace_order_reviews (order_id, id);
