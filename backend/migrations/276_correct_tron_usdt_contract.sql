-- Correct only the known bad installation default. Preserve order snapshots,
-- recipient confirmation, payment enablement, balances and custom configuration.
UPDATE settings
SET value = 'TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t', updated_at = NOW()
WHERE key = 'PAYMENT_CRYPTO_CONTRACT_ADDRESS'
  AND value = 'TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj';
