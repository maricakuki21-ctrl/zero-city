-- 207_community_district_channels.sql
-- Persist Zero City community district/channel routing used by the public UI.

ALTER TABLE community_posts
  ADD COLUMN IF NOT EXISTS district VARCHAR(48) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS channel VARCHAR(64) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_community_posts_district_channel
  ON community_posts(district, channel, created_at DESC)
  WHERE deleted_at IS NULL;
