-- Support multiple credit/balance checkins per day by replacing the old
-- per-type unique index with a partial unique index that only enforces
-- one-per-day for free/paid types.

DROP INDEX IF EXISTS daily_checkins_user_date_type_unique;

CREATE UNIQUE INDEX IF NOT EXISTS daily_checkins_user_date_once_unique
    ON daily_checkins(user_id, checkin_date, checkin_type)
    WHERE checkin_type IN ('free', 'paid');

CREATE INDEX IF NOT EXISTS idx_daily_checkins_user_date_type
    ON daily_checkins(user_id, checkin_date, checkin_type);
