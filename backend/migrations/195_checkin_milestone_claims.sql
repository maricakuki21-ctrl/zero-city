-- Store claimed daily fortune milestone gifts.
CREATE TABLE IF NOT EXISTS checkin_milestone_claims (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    milestone_days INT NOT NULL,
    credit_reward NUMERIC(12, 4) NOT NULL DEFAULT 0,
    balance_reward NUMERIC(12, 4) NOT NULL DEFAULT 0,
    balance_after NUMERIC(12, 4) NOT NULL DEFAULT 0,
    credit_balance_after NUMERIC(12, 4) NOT NULL DEFAULT 0,
    claimed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT checkin_milestone_claims_days_check CHECK (milestone_days IN (3, 5, 7)),
    CONSTRAINT checkin_milestone_claims_reward_check CHECK (credit_reward >= 0 AND balance_reward >= 0),
    CONSTRAINT checkin_milestone_claims_user_days_unique UNIQUE (user_id, milestone_days)
);

CREATE INDEX IF NOT EXISTS idx_checkin_milestone_claims_user_claimed
    ON checkin_milestone_claims(user_id, claimed_at DESC);
