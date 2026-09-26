-- 194_community_trust_layer.sql
-- Adds a reusable community trust/collaboration layer that can anchor shared pools,
-- two-sided marketplace needs, workbench tasks, and capability assets without
-- turning the community into a single-purpose forum.

ALTER TABLE community_posts
  ADD COLUMN IF NOT EXISTS scenario VARCHAR(48) NOT NULL DEFAULT 'general',
  ADD COLUMN IF NOT EXISTS subject_type VARCHAR(48) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS subject_id VARCHAR(120) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS subject_title VARCHAR(180) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS action_type VARCHAR(48) NOT NULL DEFAULT 'discuss',
  ADD COLUMN IF NOT EXISTS evidence JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS trust_signals JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE community_posts
SET subject_type = source_type,
    subject_id = source_id
WHERE subject_type = ''
  AND source_type <> ''
  AND source_id <> '';

UPDATE community_posts
SET scenario = CASE
    WHEN kind = 'pool' OR source_type = 'shared_pool' THEN 'resource_decision'
    WHEN kind = 'feedback' THEN 'feedback_triage'
    WHEN kind = 'support' THEN 'incident_support'
    WHEN kind = 'token' THEN 'usage_intel'
    WHEN kind = 'announcement' THEN 'announcement'
    ELSE scenario
  END
WHERE scenario = 'general';

UPDATE community_posts
SET action_type = CASE
    WHEN kind = 'feedback' THEN 'report'
    WHEN kind = 'support' THEN 'ask_help'
    WHEN kind = 'pool' OR source_type = 'shared_pool' THEN 'share_signal'
    WHEN kind = 'announcement' THEN 'announce'
    ELSE action_type
  END
WHERE action_type = 'discuss';

CREATE INDEX IF NOT EXISTS idx_community_posts_scenario
  ON community_posts(scenario, created_at DESC)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_community_posts_subject
  ON community_posts(subject_type, subject_id, created_at DESC)
  WHERE subject_type <> '' AND subject_id <> '';

CREATE INDEX IF NOT EXISTS idx_community_posts_action_type
  ON community_posts(action_type, created_at DESC)
  WHERE deleted_at IS NULL;
