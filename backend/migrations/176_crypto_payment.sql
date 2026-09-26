-- Migration: 176_crypto_payment
-- Add USDT-TRC20 crypto payment support.
-- The pay_amount precision is widened so order fingerprints such as 30.000217 USDT are preserved.

ALTER TABLE payment_orders
    ALTER COLUMN pay_amount TYPE DECIMAL(20,6);

CREATE UNIQUE INDEX IF NOT EXISTS payment_orders_crypto_tx_unique
    ON payment_orders(payment_trade_no)
    WHERE payment_type = 'crypto'
      AND payment_trade_no <> '';

INSERT INTO settings (key, value, updated_at)
VALUES
    ('PAYMENT_CRYPTO_ENABLED', 'true', NOW()),
    ('PAYMENT_CRYPTO_WALLET_ADDRESS', 'TJC3ZHLWuXqGcqJSxXZt2uVWAUtXniwmBr', NOW()),
    ('PAYMENT_CRYPTO_NETWORK', 'TRC20', NOW()),
    ('PAYMENT_CRYPTO_TOKEN', 'USDT', NOW()),
    ('PAYMENT_CRYPTO_CHAIN', 'tron', NOW()),
    ('PAYMENT_CRYPTO_CONTRACT_ADDRESS', 'TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj', NOW()),
    ('PAYMENT_CRYPTO_MIN_AMOUNT', '0.01', NOW()),
    ('PAYMENT_CRYPTO_USDT_BALANCE_RATE', '7', NOW()),
    ('PAYMENT_CRYPTO_EXPLORER_BASE_URL', 'https://tronscan.org/#/transaction/', NOW()),
    ('PAYMENT_CRYPTO_TRONGRID_BASE_URL', 'https://api.trongrid.io', NOW()),
    ('PAYMENT_CRYPTO_SWEEP_LOOKBACK_MINUTES', '180', NOW())
ON CONFLICT (key) DO NOTHING;
