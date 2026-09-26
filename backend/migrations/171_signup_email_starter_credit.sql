-- Enable 1.00 credit balance for email signup.
-- This updates the credit/quota track (`users.credit_balance` via AuthService), not the cash/account balance (`users.balance`).
UPDATE settings
SET value = '1', updated_at = CURRENT_TIMESTAMP
WHERE key = 'auth_source_default_email_balance';

INSERT INTO settings (key, value)
SELECT 'auth_source_default_email_balance', '1'
WHERE NOT EXISTS (SELECT 1 FROM settings WHERE key = 'auth_source_default_email_balance');

UPDATE settings
SET value = 'true', updated_at = CURRENT_TIMESTAMP
WHERE key = 'auth_source_default_email_grant_on_signup';

INSERT INTO settings (key, value)
SELECT 'auth_source_default_email_grant_on_signup', 'true'
WHERE NOT EXISTS (SELECT 1 FROM settings WHERE key = 'auth_source_default_email_grant_on_signup');
