-- 252_zero_city_governance_rules_and_badges.sql
-- Adopted governance rules and granted city badges.
--
-- Votes and contributions are records, never permissions. Adopting a rule and
-- granting a badge each store the acting user, and both can be revoked without
-- deleting history. A revoked rule or badge may be re-adopted or re-granted.

CREATE TABLE IF NOT EXISTS community_governance_rules (
    id BIGSERIAL PRIMARY KEY,
    source_poll_id BIGINT REFERENCES community_polls(id) ON DELETE SET NULL,
    title VARCHAR(200) NOT NULL CHECK (BTRIM(title) <> ''),
    body TEXT NOT NULL CHECK (BTRIM(body) <> ''),
    status VARCHAR(16) NOT NULL DEFAULT 'adopted' CHECK (status IN ('adopted', 'revoked')),
    tally_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    adopted_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    adopted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    revoked_at TIMESTAMPTZ,
    revoked_reason VARCHAR(300) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status <> 'revoked' OR revoked_at IS NOT NULL),
    CHECK (revoked_at IS NULL OR revoked_at >= adopted_at)
);

-- One adopted rule per source poll; a revoked rule frees the slot so the same
-- poll can be re-adopted deliberately instead of silently duplicating.
CREATE UNIQUE INDEX IF NOT EXISTS uq_community_governance_rules_adopted_poll
    ON community_governance_rules(source_poll_id)
    WHERE source_poll_id IS NOT NULL AND status = 'adopted';

CREATE INDEX IF NOT EXISTS idx_community_governance_rules_status
    ON community_governance_rules(status, adopted_at DESC);

CREATE TABLE IF NOT EXISTS community_badges (
    id BIGSERIAL PRIMARY KEY,
    badge_key VARCHAR(64) NOT NULL UNIQUE CHECK (badge_key ~ '^[a-z][a-z0-9_]{1,63}$'),
    name VARCHAR(80) NOT NULL CHECK (BTRIM(name) <> ''),
    description VARCHAR(300) NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS community_user_badges (
    id BIGSERIAL PRIMARY KEY,
    badge_id BIGINT NOT NULL REFERENCES community_badges(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason VARCHAR(300) NOT NULL DEFAULT '',
    granted_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    revoked_at TIMESTAMPTZ,
    revoked_reason VARCHAR(300) NOT NULL DEFAULT '',
    CHECK (revoked_at IS NULL OR revoked_at >= granted_at)
);

-- A user holds a badge at most once. Revoking keeps the row so the grant and
-- the revocation stay auditable, and re-granting creates a new active row.
CREATE UNIQUE INDEX IF NOT EXISTS uq_community_user_badges_active
    ON community_user_badges(badge_id, user_id)
    WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_community_user_badges_user
    ON community_user_badges(user_id)
    WHERE revoked_at IS NULL;

INSERT INTO community_badges (badge_key, name, description, sort_order) VALUES
    ('founder', '建城者', '在零号城早期参与建设并留下可查的贡献记录。', 10),
    ('rule_keeper', '守约人', '持续遵守并维护城市公约。', 20),
    ('contributor', '贡献者', '贡献被采纳的规则、剧本或工具。', 30),
    ('host', '房主', '在酒馆长期组织并完成公开房间。', 40)
ON CONFLICT (badge_key) DO NOTHING;
