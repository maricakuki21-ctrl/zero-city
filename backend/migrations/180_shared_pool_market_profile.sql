-- Shared pool market profile fields.
-- Adds provider-controlled branding and status metadata for the share market.

ALTER TABLE shared_pools
    ADD COLUMN IF NOT EXISTS avatar_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS status_note TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS disabled_reason TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS featured_score DECIMAL(8, 4) NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_shared_pools_featured_score
    ON shared_pools(featured_score DESC, rank_weight DESC, quality_score DESC);
