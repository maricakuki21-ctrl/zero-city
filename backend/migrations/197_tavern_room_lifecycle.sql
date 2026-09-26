-- 197_tavern_room_lifecycle.sql
-- Minimal tavern room lifecycle and credit entry ledger support.

ALTER TABLE credit_ledger
  DROP CONSTRAINT IF EXISTS credit_ledger_source_type_check;

ALTER TABLE credit_ledger
  ADD CONSTRAINT credit_ledger_source_type_check
  CHECK (source_type IN (
    'starter', 'purchase', 'contribution', 'operator_reward', 'admin_adjustment',
    'invite_reward', 'promo_bonus', 'pool_seat_fee', 'pool_owner_payout',
    'share_pool_usage', 'share_pool_payout', 'daily_checkin', 'checkin_jackpot_share',
    'checkin_milestone', 'shared_pool_stability_reward', 'recharge_credit_reward',
    'checkin_card_shop', 'tavern_room_entry'
  ));

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'tavern_room_players_status_check'
          AND conrelid = 'tavern_room_players'::regclass
    ) THEN
        ALTER TABLE tavern_room_players
            ADD CONSTRAINT tavern_room_players_status_check
            CHECK (status IN ('joined', 'left', 'removed'));
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_tavern_room_players_room_status
    ON tavern_room_players(room_id, status, joined_at DESC);

COMMENT ON COLUMN tavern_room_players.status IS 'joined / left / removed';
