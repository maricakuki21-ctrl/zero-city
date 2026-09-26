-- Migration: 212_shared_pool_daily_stability_owner_tier
-- Daily shared-pool stability rewards:
--   * settlement unit is one complete UTC day (reward_hour stores day start)
--   * owner excellence tiers: none / quality (+80/day) / certified (+150/day)
-- Existing unique key (pool_id, owner_id, reward_hour) remains the pool-level
-- idempotency guard. Owner excellence is granted once per owner per day via
-- credit_ledger source_id uniqueness.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS pool_owner_reward_tier VARCHAR(32) NOT NULL DEFAULT 'none';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'users_pool_owner_reward_tier_check'
          AND conrelid = 'users'::regclass
    ) THEN
        ALTER TABLE users
            ADD CONSTRAINT users_pool_owner_reward_tier_check
            CHECK (pool_owner_reward_tier IN ('none', 'quality', 'certified'));
    END IF;
END
$$;

COMMENT ON COLUMN users.pool_owner_reward_tier IS
    'Shared pool owner excellence tier for daily stability credits: none|quality|certified';
