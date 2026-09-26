-- Migration: 205_shared_pool_likes
-- Real pool endorsement (like) loop for marketplace trust signals.
-- Mirrors shared_pool_complaints: one like per user per pool, aggregate count on pool row.

ALTER TABLE shared_pools
    ADD COLUMN IF NOT EXISTS like_count INT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS shared_pool_likes (
    id          BIGSERIAL PRIMARY KEY,
    pool_id     BIGINT NOT NULL REFERENCES shared_pools(id) ON DELETE CASCADE,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS shared_pool_likes_pool_user_unique
    ON shared_pool_likes(pool_id, user_id);

CREATE INDEX IF NOT EXISTS idx_shared_pool_likes_pool
    ON shared_pool_likes(pool_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_shared_pool_likes_user
    ON shared_pool_likes(user_id, created_at DESC);

-- Keep like_count consistent with existing rows if any were seeded later.
UPDATE shared_pools sp
SET like_count = COALESCE((
    SELECT COUNT(*)::int FROM shared_pool_likes l WHERE l.pool_id = sp.id
), 0)
WHERE TRUE;
