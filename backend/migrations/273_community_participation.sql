-- Participation is independent of badges, money and moderation roles.
CREATE TABLE community_participation (
    user_id BIGINT PRIMARY KEY REFERENCES users(id),
    level SMALLINT NOT NULL DEFAULT 0 CHECK (level BETWEEN 0 AND 1),
    reason VARCHAR(300) NOT NULL,
    updated_by BIGINT NOT NULL REFERENCES users(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE community_participation_history (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    level SMALLINT NOT NULL CHECK (level BETWEEN 0 AND 1),
    reason VARCHAR(300) NOT NULL,
    actor_id BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX community_participation_history_user ON community_participation_history(user_id, id DESC);
