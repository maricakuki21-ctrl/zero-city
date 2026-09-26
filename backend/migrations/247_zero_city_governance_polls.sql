-- 247_zero_city_governance_polls.sql
-- Single-choice governance polls attached to the canonical community post.

CREATE TABLE IF NOT EXISTS community_polls (
    id BIGSERIAL PRIMARY KEY,
    post_id BIGINT NOT NULL UNIQUE REFERENCES community_posts(id) ON DELETE CASCADE,
    closes_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    closed_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (closed_at IS NULL OR closed_at >= created_at)
);

CREATE INDEX IF NOT EXISTS idx_community_polls_created_at ON community_polls(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_community_polls_open ON community_polls(id DESC) WHERE closed_at IS NULL;

CREATE TABLE IF NOT EXISTS community_poll_options (
    id BIGSERIAL PRIMARY KEY,
    poll_id BIGINT NOT NULL REFERENCES community_polls(id) ON DELETE CASCADE,
    position SMALLINT NOT NULL CHECK (position > 0),
    label VARCHAR(180) NOT NULL CHECK (BTRIM(label) <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (id, poll_id),
    UNIQUE (poll_id, position),
    UNIQUE (poll_id, label)
);

CREATE TABLE IF NOT EXISTS community_poll_ballots (
    poll_id BIGINT NOT NULL REFERENCES community_polls(id) ON DELETE CASCADE,
    option_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (poll_id, user_id),
    UNIQUE (poll_id, user_id),
    FOREIGN KEY (option_id, poll_id)
        REFERENCES community_poll_options(id, poll_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_community_poll_ballots_option ON community_poll_ballots(option_id);
