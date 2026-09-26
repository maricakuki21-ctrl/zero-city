-- 257_marketplace_order_active_inquiry.sql
-- Keep one active/completed order per inquiry. A canceled order may be followed
-- by a fresh quote, while retries and concurrent requests cannot create
-- duplicate orders for the same conversation.

CREATE UNIQUE INDEX IF NOT EXISTS idx_marketplace_orders_active_inquiry
    ON marketplace_orders (inquiry_id)
    WHERE status <> 'canceled';
