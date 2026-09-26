-- Migration: 157_invite_growth_tier_rewards
-- Adds tracking for second-level invite growth rewards.

ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS invite_indirect_reward_count INT NOT NULL DEFAULT 0;
