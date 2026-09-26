-- Drop the legacy one-claim-per-day trigger so the daily fortune gift can
-- support independent free, credit, and balance draw limits.

DROP TRIGGER IF EXISTS trg_prevent_duplicate_daily_checkin ON daily_checkins;
DROP FUNCTION IF EXISTS prevent_duplicate_daily_checkin();
