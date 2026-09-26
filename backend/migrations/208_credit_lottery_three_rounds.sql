-- Session-based three-round credit lottery and jackpot ledger.

ALTER TABLE credit_ledger
  DROP CONSTRAINT IF EXISTS credit_ledger_source_type_check;

ALTER TABLE credit_ledger
  ADD CONSTRAINT credit_ledger_source_type_check
  CHECK (source_type IN (
    'starter', 'purchase', 'contribution', 'operator_reward', 'admin_adjustment',
    'invite_reward', 'promo_bonus', 'pool_seat_fee', 'pool_owner_payout',
    'share_pool_usage', 'share_pool_payout', 'daily_checkin', 'checkin_jackpot_share',
    'checkin_milestone', 'shared_pool_stability_reward', 'recharge_credit_reward',
    'checkin_card_shop', 'tavern_room_entry',
    'credit_lottery_entry', 'credit_lottery_reward',
    'credit_lottery_jackpot_win', 'credit_lottery_jackpot_share'
  ));

CREATE TABLE IF NOT EXISTS jackpot_ledger (
  id BIGSERIAL PRIMARY KEY,
  pool_type VARCHAR(20) NOT NULL,
  source_type VARCHAR(40) NOT NULL,
  source_id VARCHAR(120) NOT NULL,
  entry_type VARCHAR(24) NOT NULL,
  amount NUMERIC(18,8) NOT NULL,
  user_id BIGINT NOT NULL DEFAULT 0,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT jackpot_ledger_pool_type_check CHECK (pool_type IN ('credit', 'balance')),
  CONSTRAINT jackpot_ledger_entry_type_check CHECK (entry_type IN ('contribution', 'payout', 'celebration')),
  CONSTRAINT jackpot_ledger_amount_check CHECK (amount <> 0)
);

ALTER TABLE jackpot_ledger
  DROP CONSTRAINT IF EXISTS jackpot_ledger_source_type_source_id_entry_type_user_id_key;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'jackpot_ledger_pool_source_entry_user_unique'
      AND conrelid = 'jackpot_ledger'::regclass
  ) THEN
    ALTER TABLE jackpot_ledger
      ADD CONSTRAINT jackpot_ledger_pool_source_entry_user_unique
      UNIQUE (pool_type, source_type, source_id, entry_type, user_id);
  END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_jackpot_ledger_pool_created
  ON jackpot_ledger(pool_type, created_at DESC);

CREATE TABLE IF NOT EXISTS user_balance_ledger (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  source_type VARCHAR(50) NOT NULL,
  source_id VARCHAR(120) NOT NULL DEFAULT '',
  amount DECIMAL(20, 8) NOT NULL,
  balance_after DECIMAL(20, 8) NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'posted',
  note TEXT NOT NULL DEFAULT '',
  created_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  posted_at TIMESTAMPTZ NULL,
  CONSTRAINT user_balance_ledger_source_type_check CHECK (source_type IN (
    'daily_checkin',
    'checkin_jackpot_share',
    'credit_lottery_jackpot_win',
    'credit_lottery_jackpot_share'
  )),
  CONSTRAINT user_balance_ledger_status_check CHECK (status IN ('pending', 'posted', 'reversed'))
);

CREATE INDEX IF NOT EXISTS idx_user_balance_ledger_user_created
  ON user_balance_ledger(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_user_balance_ledger_source
  ON user_balance_ledger(source_type, source_id);

CREATE UNIQUE INDEX IF NOT EXISTS user_balance_ledger_source_unique
  ON user_balance_ledger(user_id, source_type, source_id)
  WHERE status <> 'reversed';

ALTER TABLE daily_checkins
  ADD COLUMN IF NOT EXISTS jackpot_payouts JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE TABLE IF NOT EXISTS credit_lottery_sessions (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  session_date DATE NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'active',
  cost_credit NUMERIC(12,4) NOT NULL DEFAULT 20,
  max_rounds INT NOT NULL DEFAULT 3,
  current_round INT NOT NULL DEFAULT 1,
  current_round_id BIGINT,
  final_round_id BIGINT,
  settlement_reason VARCHAR(24),
  balance_before NUMERIC(18,8) NOT NULL DEFAULT 0,
  credit_balance_before NUMERIC(18,8) NOT NULL DEFAULT 0,
  credit_balance_after_cost NUMERIC(18,8) NOT NULL DEFAULT 0,
  balance_after_settlement NUMERIC(18,8) NOT NULL DEFAULT 0,
  credit_balance_after_settlement NUMERIC(18,8) NOT NULL DEFAULT 0,
  jackpot_hit BOOLEAN NOT NULL DEFAULT FALSE,
  jackpot_pool VARCHAR(20),
  jackpot_winner_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
  jackpot_celebration_amount NUMERIC(18,8) NOT NULL DEFAULT 0,
  jackpot_celebration_user_count INT NOT NULL DEFAULT 0,
  jackpot_payouts JSONB NOT NULL DEFAULT '[]'::jsonb,
  collectible_card_id BIGINT,
  expires_at TIMESTAMPTZ NOT NULL,
  settled_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT credit_lottery_sessions_status_check CHECK (status IN ('active', 'settled', 'failed')),
  CONSTRAINT credit_lottery_sessions_round_check CHECK (current_round BETWEEN 1 AND 3),
  CONSTRAINT credit_lottery_sessions_reason_check CHECK (settlement_reason IS NULL OR settlement_reason IN ('user_stop', 'third_round', 'timeout'))
);

CREATE UNIQUE INDEX IF NOT EXISTS credit_lottery_sessions_one_active_user
  ON credit_lottery_sessions(user_id)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_credit_lottery_sessions_user_date
  ON credit_lottery_sessions(user_id, session_date);

CREATE INDEX IF NOT EXISTS idx_credit_lottery_sessions_expires
  ON credit_lottery_sessions(expires_at)
  WHERE status = 'active';

ALTER TABLE credit_lottery_sessions
  ADD COLUMN IF NOT EXISTS balance_after_settlement NUMERIC(18,8) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS jackpot_payouts JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE TABLE IF NOT EXISTS credit_lottery_rounds (
  id BIGSERIAL PRIMARY KEY,
  session_id BIGINT NOT NULL REFERENCES credit_lottery_sessions(id) ON DELETE CASCADE,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  round_no INT NOT NULL,
  reward_asset VARCHAR(20) NOT NULL DEFAULT 'credit',
  reward_amount NUMERIC(12,4) NOT NULL,
  reward_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  collectible_candidate_payload JSONB,
  is_current BOOLEAN NOT NULL DEFAULT TRUE,
  superseded_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT credit_lottery_rounds_round_check CHECK (round_no BETWEEN 1 AND 3),
  UNIQUE (session_id, round_no)
);

CREATE UNIQUE INDEX IF NOT EXISTS credit_lottery_rounds_one_current
  ON credit_lottery_rounds(session_id)
  WHERE is_current = TRUE;

CREATE INDEX IF NOT EXISTS idx_credit_lottery_rounds_session
  ON credit_lottery_rounds(session_id, round_no);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'credit_lottery_sessions_current_round_fk'
      AND conrelid = 'credit_lottery_sessions'::regclass
  ) THEN
    ALTER TABLE credit_lottery_sessions
      ADD CONSTRAINT credit_lottery_sessions_current_round_fk
      FOREIGN KEY (current_round_id) REFERENCES credit_lottery_rounds(id) DEFERRABLE INITIALLY DEFERRED;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'credit_lottery_sessions_final_round_fk'
      AND conrelid = 'credit_lottery_sessions'::regclass
  ) THEN
    ALTER TABLE credit_lottery_sessions
      ADD CONSTRAINT credit_lottery_sessions_final_round_fk
      FOREIGN KEY (final_round_id) REFERENCES credit_lottery_rounds(id) DEFERRABLE INITIALLY DEFERRED;
  END IF;
END
$$;

CREATE TABLE IF NOT EXISTS credit_lottery_operation_keys (
  id BIGSERIAL PRIMARY KEY,
  session_id BIGINT NOT NULL REFERENCES credit_lottery_sessions(id) ON DELETE CASCADE,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  operation VARCHAR(24) NOT NULL,
  idempotency_key VARCHAR(120) NOT NULL,
  expected_round INT,
  result_status VARCHAR(24) NOT NULL DEFAULT 'completed',
  response_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT credit_lottery_operation_keys_operation_check CHECK (operation IN ('continue', 'settle')),
  UNIQUE (session_id, operation, idempotency_key)
);

CREATE TABLE IF NOT EXISTS credit_lottery_jackpot_shares (
  id BIGSERIAL PRIMARY KEY,
  session_id BIGINT NOT NULL REFERENCES credit_lottery_sessions(id) ON DELETE CASCADE,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  pool VARCHAR(20) NOT NULL,
  amount NUMERIC(18,8) NOT NULL DEFAULT 0,
  balance_after NUMERIC(18,8) NOT NULL DEFAULT 0,
  credit_balance_after NUMERIC(18,8) NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT credit_lottery_jackpot_shares_session_user_pool_unique UNIQUE (session_id, user_id, pool)
);

CREATE INDEX IF NOT EXISTS idx_credit_lottery_jackpot_shares_user_created
  ON credit_lottery_jackpot_shares(user_id, created_at DESC);

ALTER TABLE credit_lottery_jackpot_shares
  DROP CONSTRAINT IF EXISTS credit_lottery_jackpot_shares_session_id_user_id_key;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'credit_lottery_jackpot_shares_session_user_pool_unique'
      AND conrelid = 'credit_lottery_jackpot_shares'::regclass
  ) THEN
    ALTER TABLE credit_lottery_jackpot_shares
      ADD CONSTRAINT credit_lottery_jackpot_shares_session_user_pool_unique
      UNIQUE (session_id, user_id, pool);
  END IF;
END
$$;

ALTER TABLE checkin_collectible_cards
  ADD COLUMN IF NOT EXISTS credit_lottery_session_id BIGINT REFERENCES credit_lottery_sessions(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS credit_lottery_round_id BIGINT REFERENCES credit_lottery_rounds(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_checkin_collectible_cards_credit_lottery
  ON checkin_collectible_cards(credit_lottery_session_id, credit_lottery_round_id)
  WHERE credit_lottery_session_id IS NOT NULL;

ALTER TABLE checkin_jackpot_shares
  DROP CONSTRAINT IF EXISTS checkin_jackpot_shares_checkin_id_user_id_key;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'checkin_jackpot_shares_checkin_user_pool_unique'
      AND conrelid = 'checkin_jackpot_shares'::regclass
  ) THEN
    ALTER TABLE checkin_jackpot_shares
      ADD CONSTRAINT checkin_jackpot_shares_checkin_user_pool_unique
      UNIQUE (checkin_id, user_id, pool);
  END IF;
END
$$;

INSERT INTO jackpot_ledger (pool_type, source_type, source_id, entry_type, amount, user_id, metadata, created_at)
SELECT
  CASE WHEN checkin_type = 'balance' THEN 'balance' ELSE 'credit' END,
  'daily_checkin',
  'daily_checkin:' || id,
  'contribution',
  CASE WHEN checkin_type = 'balance' THEN point_cost * 0.10 ELSE point_cost * 0.20 END,
  user_id,
  jsonb_build_object('checkin_type', checkin_type),
  created_at
FROM daily_checkins
WHERE checkin_type IN ('paid', 'credit', 'balance')
  AND point_cost > 0
ON CONFLICT DO NOTHING;

INSERT INTO jackpot_ledger (pool_type, source_type, source_id, entry_type, amount, user_id, metadata, created_at)
SELECT
  jackpot_pool,
  'daily_checkin',
  'daily_checkin:' || id,
  'payout',
  -jackpot_winner_amount,
  user_id,
  jsonb_build_object('checkin_type', checkin_type),
  created_at
FROM daily_checkins
WHERE jackpot_hit = TRUE
  AND jackpot_pool IN ('credit', 'balance')
  AND jackpot_winner_amount > 0
ON CONFLICT DO NOTHING;

INSERT INTO jackpot_ledger (pool_type, source_type, source_id, entry_type, amount, user_id, metadata, created_at)
SELECT
  jackpot_pool,
  'daily_checkin',
  'daily_checkin:' || id,
  'celebration',
  -jackpot_celebration_amount,
  user_id,
  jsonb_build_object('checkin_type', checkin_type, 'celebration_user_count', jackpot_celebration_user_count),
  created_at
FROM daily_checkins
WHERE jackpot_hit = TRUE
  AND jackpot_pool IN ('credit', 'balance')
  AND jackpot_celebration_amount > 0
ON CONFLICT DO NOTHING;
