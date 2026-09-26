-- Migration: 156_invite_reward_support
-- Support "invite one person, get $30 credits" feature.
-- Adds 'invite_reward' to credit_ledger source_type CHECK (for existing DBs),
-- and adds invite_reward_count to user_affiliates for idempotency tracking.

-- 1. Update credit_ledger CHECK constraint to include 'invite_reward'
-- (Fresh installs from migration 151 already have it; this handles existing DBs.)
DO $$
BEGIN
    -- Drop old constraint if it exists (without 'invite_reward')
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'credit_ledger_source_type_check'
          AND conrelid = 'credit_ledger'::regclass
    ) THEN
        -- Check if the existing constraint already includes 'invite_reward'
        IF NOT EXISTS (
            SELECT 1 FROM pg_constraint
            WHERE conname = 'credit_ledger_source_type_check'
              AND conrelid = 'credit_ledger'::regclass
              AND pg_get_constraintdef(oid) LIKE '%invite_reward%'
        ) THEN
            ALTER TABLE credit_ledger DROP CONSTRAINT credit_ledger_source_type_check;
            ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_source_type_check
                CHECK (source_type IN ('starter', 'purchase', 'contribution', 'operator_reward', 'admin_adjustment', 'invite_reward'));
        END IF;
    ELSE
        -- Constraint doesn't exist at all (unlikely but safe)
        ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_source_type_check
            CHECK (source_type IN ('starter', 'purchase', 'contribution', 'operator_reward', 'admin_adjustment', 'invite_reward'));
    END IF;
END
$$;

-- 2. Add invite_reward_count to user_affiliates for tracking how many
-- invite rewards have been granted to this user (as inviter).
ALTER TABLE user_affiliates ADD COLUMN IF NOT EXISTS invite_reward_count INT NOT NULL DEFAULT 0;
