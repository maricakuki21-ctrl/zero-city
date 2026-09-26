-- BizDecipher launch defaults.
-- Seed only missing settings so existing/operator-edited installs are never overwritten.
-- Secrets such as OIDC client_secret, JWT secret, upstream keys, and passwords must stay in config/env.

INSERT INTO settings (key, value)
VALUES
    ('site_name', 'BizDecipher'),
    ('default_balance', '30'),
    ('oidc_connect_enabled', 'true'),
    ('oidc_connect_provider_name', 'dc.hhhl.cc'),
    ('oidc_connect_redirect_url', 'https://bizdecipher.com/api/v1/auth/oauth/oidc/callback'),
    ('oidc_connect_frontend_redirect_url', '/auth/oidc/callback'),
    ('oidc_connect_token_auth_method', 'client_secret_post'),
    ('oidc_connect_use_pkce', 'false'),
    ('oidc_connect_validate_id_token', 'false'),
    ('auth_source_default_oidc_balance', '30'),
    ('auth_source_default_oidc_grant_on_signup', 'true'),
    ('affiliate_enabled', 'true'),
    ('affiliate_rebate_rate', '20'),
    ('affiliate_rebate_freeze_hours', '0'),
    ('affiliate_rebate_duration_days', '0'),
    ('affiliate_rebate_per_invitee_cap', '30')
ON CONFLICT (key) DO NOTHING;
