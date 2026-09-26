DROP INDEX CONCURRENTLY IF EXISTS credit_lottery_sessions_start_idempotency_unique;

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS credit_lottery_sessions_start_idempotency_unique
  ON credit_lottery_sessions(user_id, session_date, start_idempotency_key)
  WHERE start_idempotency_key <> '';

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS credit_ledger_credit_lottery_source_unique
  ON credit_ledger(user_id, source_type, source_id)
  WHERE status <> 'reversed'
    AND source_type IN (
      'credit_lottery_entry',
      'credit_lottery_reward',
      'credit_lottery_jackpot_win',
      'credit_lottery_jackpot_share'
    );
