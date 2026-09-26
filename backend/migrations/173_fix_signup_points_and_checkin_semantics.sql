-- Fix BizDecipher asset semantics after points/quota split.
-- Points use users.balance; credits/quota use users.credit_balance.
-- Registration gift is 30 points, not quota.

UPDATE settings
SET value = '30', updated_at = CURRENT_TIMESTAMP
WHERE key = 'signup_point_balance';

INSERT INTO settings (key, value)
SELECT 'signup_point_balance', '30'
WHERE NOT EXISTS (SELECT 1 FROM settings WHERE key = 'signup_point_balance');

-- default_balance is a quota fallback in AuthService; it must not carry the 30 signup points.
UPDATE settings
SET value = '0', updated_at = CURRENT_TIMESTAMP
WHERE key = 'default_balance';

INSERT INTO settings (key, value)
SELECT 'default_balance', '0'
WHERE NOT EXISTS (SELECT 1 FROM settings WHERE key = 'default_balance');

-- OIDC signup also receives points via signup_point_balance. Avoid double-granting 30 as quota.
UPDATE settings
SET value = '0', updated_at = CURRENT_TIMESTAMP
WHERE key = 'auth_source_default_oidc_balance';

INSERT INTO settings (key, value)
SELECT 'auth_source_default_oidc_balance', '0'
WHERE NOT EXISTS (SELECT 1 FROM settings WHERE key = 'auth_source_default_oidc_balance');
