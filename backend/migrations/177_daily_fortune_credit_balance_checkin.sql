-- Allow the daily fortune gift to use independent free, credit, and balance checkins.
-- Existing "paid" rows are kept for backward compatibility; new paid requests are normalized to "credit" in service code.

ALTER TABLE daily_checkins DROP CONSTRAINT IF EXISTS daily_checkins_type_check;
ALTER TABLE daily_checkins ADD CONSTRAINT daily_checkins_type_check
    CHECK (checkin_type IN ('free', 'paid', 'credit', 'balance'));
