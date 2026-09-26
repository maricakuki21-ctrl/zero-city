-- Daily fortune jackpot hit and celebration records.
ALTER TABLE daily_checkins
  ADD COLUMN IF NOT EXISTS jackpot_hit BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS jackpot_pool VARCHAR(20),
  ADD COLUMN IF NOT EXISTS jackpot_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS jackpot_winner_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS jackpot_celebration_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS jackpot_celebration_user_count INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS checkin_jackpot_shares (
  id BIGSERIAL PRIMARY KEY,
  checkin_id BIGINT NOT NULL REFERENCES daily_checkins(id) ON DELETE CASCADE,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  pool VARCHAR(20) NOT NULL,
  amount NUMERIC(18,8) NOT NULL DEFAULT 0,
  balance_after NUMERIC(18,8) NOT NULL DEFAULT 0,
  credit_balance_after NUMERIC(18,8) NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (checkin_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_checkin_jackpot_shares_user_created ON checkin_jackpot_shares(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_daily_checkins_jackpot_pool ON daily_checkins(jackpot_pool) WHERE jackpot_hit = TRUE;
