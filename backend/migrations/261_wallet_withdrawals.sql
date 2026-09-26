-- Manual payouts only; amounts retain the owner wallet's USD accounting unit.
CREATE TABLE shared_pool_withdrawal_policy (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    channels JSONB NOT NULL DEFAULT '[]',
    instructions VARCHAR(2000) NOT NULL DEFAULT '',
    updated_by BIGINT REFERENCES users(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO shared_pool_withdrawal_policy(id) VALUES(TRUE);

CREATE TABLE shared_pool_withdrawals (
    id BIGSERIAL PRIMARY KEY,
    owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    operation_id VARCHAR(100) NOT NULL,
    amount NUMERIC(24,12) NOT NULL CHECK(amount > 0),
    channel VARCHAR(80) NOT NULL,
    recipient VARCHAR(500) NOT NULL,
    processing_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK(status IN ('pending','processing','paid','rejected','cancelled')),
    reference VARCHAR(200) NOT NULL DEFAULT '',
    reason VARCHAR(1000) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(owner_id, operation_id),
    CHECK(status <> 'paid' OR length(trim(reference)) > 0)
);
CREATE INDEX shared_pool_withdrawals_queue ON shared_pool_withdrawals(status,id);
CREATE TABLE shared_pool_withdrawal_audit (
    id BIGSERIAL PRIMARY KEY,
    withdrawal_id BIGINT REFERENCES shared_pool_withdrawals(id) ON DELETE RESTRICT,
    actor_id BIGINT NOT NULL REFERENCES users(id),
    action VARCHAR(30) NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    reference VARCHAR(200) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE shared_pool_owner_earnings_ledger DROP CONSTRAINT shared_pool_owner_earnings_event_check;
ALTER TABLE shared_pool_owner_earnings_ledger ADD CONSTRAINT shared_pool_owner_earnings_event_check
    CHECK(event_type IN ('earning','transfer_to_balance','reversal','adjustment','withdrawal','withdrawal_return'));
