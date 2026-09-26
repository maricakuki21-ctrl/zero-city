-- Ensure each user receives at most one BizDecipher starter credit ledger entry.
-- The starter balance itself is already initialized by AuthService grantPlan.Balance;
-- this ledger guard prevents duplicate accounting when multiple signup/OAuth paths race.
CREATE UNIQUE INDEX IF NOT EXISTS credit_ledger_user_starter_unique
ON credit_ledger(user_id)
WHERE source_type = 'starter';
