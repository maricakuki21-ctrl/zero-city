-- Persist operation identity for free and balance daily-fortune draws. A
-- committed daily_checkins row is the settled result and can be queried after
-- a client timeout. Empty operation IDs preserve compatibility for old rows.

ALTER TABLE daily_checkins
    ADD COLUMN IF NOT EXISTS operation_id VARCHAR(128) NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS daily_checkins_user_operation_unique
    ON daily_checkins(user_id, operation_id)
    WHERE operation_id <> '';
