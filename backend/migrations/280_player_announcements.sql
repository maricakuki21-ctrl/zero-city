-- Player announcements remain projections of canonical, immutable polls.
ALTER TABLE community_polls
    ADD COLUMN IF NOT EXISTS proposal_kind TEXT NOT NULL DEFAULT 'general'
        CHECK (proposal_kind IN ('general', 'announcement', 'activity', 'improvement', 'rule')),
    ADD COLUMN IF NOT EXISTS minimum_votes INTEGER NOT NULL DEFAULT 3 CHECK (minimum_votes > 0),
    ADD COLUMN IF NOT EXISTS support_percent INTEGER NOT NULL DEFAULT 60 CHECK (support_percent BETWEEN 51 AND 100),
    ADD COLUMN IF NOT EXISTS display_days INTEGER NOT NULL DEFAULT 7 CHECK (display_days BETWEEN 1 AND 30);

CREATE INDEX IF NOT EXISTS community_player_announcements_deadline
    ON community_polls(closes_at DESC) WHERE proposal_kind = 'announcement' AND closed_at IS NULL;
