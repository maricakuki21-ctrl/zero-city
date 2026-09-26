-- Allow daily fortune milestone gifts to be claimed once per 7-day cycle.
ALTER TABLE checkin_milestone_claims
    ADD COLUMN IF NOT EXISTS cycle_no INT NOT NULL DEFAULT 1;

ALTER TABLE checkin_milestone_claims
    DROP CONSTRAINT IF EXISTS checkin_milestone_claims_cycle_no_check;

ALTER TABLE checkin_milestone_claims
    ADD CONSTRAINT checkin_milestone_claims_cycle_no_check
    CHECK (cycle_no > 0);

ALTER TABLE checkin_milestone_claims
    DROP CONSTRAINT IF EXISTS checkin_milestone_claims_user_days_unique;

ALTER TABLE checkin_milestone_claims
    DROP CONSTRAINT IF EXISTS checkin_milestone_claims_user_cycle_days_unique;

ALTER TABLE checkin_milestone_claims
    ADD CONSTRAINT checkin_milestone_claims_user_cycle_days_unique
    UNIQUE (user_id, cycle_no, milestone_days);

CREATE INDEX IF NOT EXISTS idx_checkin_milestone_claims_user_cycle
    ON checkin_milestone_claims(user_id, cycle_no, milestone_days);
