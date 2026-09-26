-- Migration: 158_invite_reward_idempotency_guard
-- Adds an idempotency guard for invite reward ledger entries.
-- This protects direct and indirect invite rewards from accidental duplicate grants.

CREATE UNIQUE INDEX IF NOT EXISTS credit_ledger_invite_reward_once_unique
    ON credit_ledger(user_id, source_type, source_id)
    WHERE source_type = 'invite_reward';
