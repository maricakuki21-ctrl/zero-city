ALTER TABLE credit_lottery_sessions
  ADD COLUMN IF NOT EXISTS mode VARCHAR(24) NOT NULL DEFAULT 'three_round',
  ADD COLUMN IF NOT EXISTS start_idempotency_key VARCHAR(255) NOT NULL DEFAULT '';

ALTER TABLE credit_lottery_operation_keys
  ALTER COLUMN idempotency_key TYPE VARCHAR(255);

CREATE UNIQUE INDEX IF NOT EXISTS credit_lottery_sessions_start_idempotency_unique
  ON credit_lottery_sessions(user_id, start_idempotency_key)
  WHERE start_idempotency_key <> '';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'credit_lottery_sessions_mode_check'
      AND conrelid = 'credit_lottery_sessions'::regclass
  ) THEN
    ALTER TABLE credit_lottery_sessions
      ADD CONSTRAINT credit_lottery_sessions_mode_check
      CHECK (mode IN ('single', 'three_round'));
  END IF;

  IF EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'credit_lottery_sessions_reason_check'
      AND conrelid = 'credit_lottery_sessions'::regclass
  ) THEN
    ALTER TABLE credit_lottery_sessions
      DROP CONSTRAINT credit_lottery_sessions_reason_check;
  END IF;

  ALTER TABLE credit_lottery_sessions
    ADD CONSTRAINT credit_lottery_sessions_reason_check
    CHECK (settlement_reason IS NULL OR settlement_reason IN ('user_stop', 'third_round', 'timeout', 'single_draw'));
END
$$;
