-- Zero City social profile and resource-linked community posts.

ALTER TABLE biz_profiles
  ADD COLUMN IF NOT EXISTS profile_visibility VARCHAR(20) NOT NULL DEFAULT 'public',
  ADD COLUMN IF NOT EXISTS profile_theme VARCHAR(40) NOT NULL DEFAULT 'zero-city',
  ADD COLUMN IF NOT EXISTS background_card_key VARCHAR(80) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS background_card_rarity VARCHAR(24) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS background_card_serial_no BIGINT,
  ADD COLUMN IF NOT EXISTS background_card_edition_no BIGINT,
  ADD COLUMN IF NOT EXISTS background_card_edition_supply BIGINT;

ALTER TABLE biz_profiles
  DROP CONSTRAINT IF EXISTS biz_profiles_profile_visibility_check;

ALTER TABLE biz_profiles
  ADD CONSTRAINT biz_profiles_profile_visibility_check
  CHECK (profile_visibility IN ('public', 'private'));

ALTER TABLE community_posts
  ADD COLUMN IF NOT EXISTS source_type VARCHAR(32) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS source_id VARCHAR(120) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_community_posts_source
  ON community_posts(source_type, source_id, created_at DESC)
  WHERE source_type <> '' AND source_id <> '';

CREATE TABLE IF NOT EXISTS zero_city_follows (
  follower_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  following_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (follower_user_id, following_user_id),
  CONSTRAINT zero_city_follows_no_self_check CHECK (follower_user_id <> following_user_id)
);

CREATE INDEX IF NOT EXISTS idx_zero_city_follows_following
  ON zero_city_follows(following_user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_zero_city_follows_follower
  ON zero_city_follows(follower_user_id, created_at DESC);
