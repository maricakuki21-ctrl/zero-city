-- Enable point-shop purchases for low-tier Zero City collectible cards.
ALTER TABLE checkin_collectible_cards
  ALTER COLUMN checkin_id DROP NOT NULL;

ALTER TABLE credit_ledger
  DROP CONSTRAINT IF EXISTS credit_ledger_source_type_check;

ALTER TABLE credit_ledger
  ADD CONSTRAINT credit_ledger_source_type_check
  CHECK (source_type IN ('starter', 'purchase', 'contribution', 'operator_reward', 'admin_adjustment', 'invite_reward', 'promo_bonus', 'pool_seat_fee', 'pool_owner_payout', 'share_pool_usage', 'share_pool_payout', 'daily_checkin', 'checkin_jackpot_share', 'checkin_milestone', 'shared_pool_stability_reward', 'recharge_credit_reward', 'checkin_card_shop'));
