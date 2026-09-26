-- Migration: 163_disable_seeded_register_promo
-- Prevent new accounts from receiving both default signup balance and the seeded register promo.
-- Existing users, balances, promo claims, and ledger entries are preserved.

UPDATE promo_campaigns
SET enabled = FALSE,
    updated_at = NOW()
WHERE name = '三天注册赠送活动'
  AND type = 'register_bonus'
  AND credit_amount = 30
  AND target = 'new_users'
  AND enabled = TRUE;
