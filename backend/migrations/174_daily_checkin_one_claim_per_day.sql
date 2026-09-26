-- Enforce one daily checkin claim per user per day going forward.
-- Existing historical duplicates are preserved to avoid retroactive balance changes.

CREATE OR REPLACE FUNCTION prevent_duplicate_daily_checkin()
RETURNS trigger AS $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM daily_checkins
        WHERE user_id = NEW.user_id
          AND checkin_date = NEW.checkin_date
          AND id <> COALESCE(NEW.id, -1)
    ) THEN
        RAISE EXCEPTION 'daily checkin already claimed for user % on %', NEW.user_id, NEW.checkin_date
            USING ERRCODE = '23505';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_prevent_duplicate_daily_checkin ON daily_checkins;
CREATE TRIGGER trg_prevent_duplicate_daily_checkin
BEFORE INSERT OR UPDATE OF user_id, checkin_date ON daily_checkins
FOR EACH ROW
EXECUTE FUNCTION prevent_duplicate_daily_checkin();
