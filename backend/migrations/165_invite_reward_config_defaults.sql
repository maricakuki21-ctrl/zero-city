-- Migration: 165_invite_reward_config_defaults
-- Make first-valid-call invite rewards configurable instead of hard-coded.

INSERT INTO settings (key, value, updated_at)
VALUES ('affiliate_enabled', 'true', NOW())
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value,
    updated_at = NOW();

INSERT INTO settings (key, value, updated_at)
VALUES
    ('invite_reward_enabled', 'true', NOW()),
    ('invite_reward_amount', '30', NOW()),
    ('invite_reward_milestone_threshold', '0', NOW()),
    ('invite_reward_milestone_amount', '0', NOW())
ON CONFLICT (key) DO NOTHING;

