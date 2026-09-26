-- Migration: 161_pool_seats
-- Shared-pool seat billing + owner settlement (MONEY-SENSITIVE workstream).
--
-- This is the deliberately-deferred phase from migration 160: joining a pool,
-- holding a seat, hourly seat-fee charging, and owner payout settlement.
-- All money movement is double-entry through credit_ledger:
--   * pool_seat_fee     : DEBIT  the seat holder (negative amount)
--   * pool_owner_payout : CREDIT the pool owner   (positive amount)
-- Every charge is idempotent per (seat, billing hour) via a unique index so a
-- worker re-run or overlapping leader never double-charges.

-- 1. Seat bindings: one active seat per (pool, user). Holding a seat is what
--    accrues hourly fees. last_charged_at advances one whole hour at a time.
CREATE TABLE IF NOT EXISTS pool_seat_bindings (
    id                BIGSERIAL PRIMARY KEY,
    pool_id           BIGINT NOT NULL REFERENCES shared_pools(id) ON DELETE CASCADE,
    user_id           BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status            VARCHAR(20) NOT NULL DEFAULT 'active',
    hourly_seat_fee   DECIMAL(20, 8) NOT NULL DEFAULT 0,  -- snapshot of pool fee at join time
    joined_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_charged_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),  -- next charge covers [last_charged_at, +1h)
    released_at       TIMESTAMPTZ NULL,
    total_charged     DECIMAL(20, 8) NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT pool_seat_bindings_status_check CHECK (status IN ('active', 'released'))
);

-- At most one ACTIVE seat per (pool, user). Released seats may accumulate as history.
CREATE UNIQUE INDEX IF NOT EXISTS pool_seat_bindings_active_unique
    ON pool_seat_bindings(pool_id, user_id) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_pool_seat_bindings_user ON pool_seat_bindings(user_id, status);
CREATE INDEX IF NOT EXISTS idx_pool_seat_bindings_due ON pool_seat_bindings(status, last_charged_at);

-- 2. Per-hour charge records. Idempotency key = (seat_id, billing_hour).
--    billing_hour is the truncated hour that the charge covers, so the worker
--    can charge any number of overdue whole hours without ever duplicating one.
CREATE TABLE IF NOT EXISTS pool_seat_charges (
    id                BIGSERIAL PRIMARY KEY,
    seat_id           BIGINT NOT NULL REFERENCES pool_seat_bindings(id) ON DELETE CASCADE,
    pool_id           BIGINT NOT NULL REFERENCES shared_pools(id) ON DELETE CASCADE,
    user_id           BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    owner_id          BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    billing_hour      TIMESTAMPTZ NOT NULL,
    amount            DECIMAL(20, 8) NOT NULL,  -- positive magnitude charged
    fee_ledger_id     BIGINT NULL,              -- credit_ledger row debiting the holder
    payout_ledger_id  BIGINT NULL,              -- credit_ledger row crediting the owner (NULL if no owner)
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS pool_seat_charges_seat_hour_unique
    ON pool_seat_charges(seat_id, billing_hour);
CREATE INDEX IF NOT EXISTS idx_pool_seat_charges_user ON pool_seat_charges(user_id, created_at DESC);

-- 3. Extend credit_ledger source_type CHECK to include the two seat-billing kinds.
--    Same safe DROP/ADD pattern as migration 159 (pg_constraint guarded).
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'credit_ledger_source_type_check'
          AND conrelid = 'credit_ledger'::regclass
    ) THEN
        IF NOT EXISTS (
            SELECT 1 FROM pg_constraint
            WHERE conname = 'credit_ledger_source_type_check'
              AND conrelid = 'credit_ledger'::regclass
              AND pg_get_constraintdef(oid) LIKE '%pool_seat_fee%'
        ) THEN
            ALTER TABLE credit_ledger DROP CONSTRAINT credit_ledger_source_type_check;
            ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_source_type_check
                CHECK (source_type IN ('starter', 'purchase', 'contribution', 'operator_reward', 'admin_adjustment', 'invite_reward', 'promo_bonus', 'pool_seat_fee', 'pool_owner_payout'));
        END IF;
    ELSE
        ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_source_type_check
            CHECK (source_type IN ('starter', 'purchase', 'contribution', 'operator_reward', 'admin_adjustment', 'invite_reward', 'promo_bonus', 'pool_seat_fee', 'pool_owner_payout'));
    END IF;
END
$$;
