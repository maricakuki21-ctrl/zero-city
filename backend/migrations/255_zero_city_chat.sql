-- 255_zero_city_chat.sql
-- Site-wide zero-city chat: public channels plus persisted, idempotent messages.
-- PostgreSQL is the authoritative history; broadcasts may later be layered on top.

CREATE TABLE IF NOT EXISTS biz_chat_channels (
    id BIGSERIAL PRIMARY KEY,
    slug VARCHAR(64) NOT NULL UNIQUE,
    title VARCHAR(120) NOT NULL,
    description VARCHAR(300) NOT NULL DEFAULT '',
    kind VARCHAR(20) NOT NULL DEFAULT 'public'
        CHECK (kind IN ('public', 'private')),
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS biz_chat_messages (
    id BIGSERIAL PRIMARY KEY,
    channel_id BIGINT NOT NULL REFERENCES biz_chat_channels(id) ON DELETE CASCADE,
    sender_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_message_id VARCHAR(100) NOT NULL,
    body TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (channel_id, sender_user_id, client_message_id)
);

CREATE INDEX IF NOT EXISTS idx_biz_chat_messages_channel
    ON biz_chat_messages (channel_id, id DESC);

INSERT INTO biz_chat_channels (slug, title, description, sort_order)
VALUES
    ('lobby', '零号城大厅', '全站公共大厅', 10),
    ('tavern', '酒馆', '酒馆话题', 20)
ON CONFLICT (slug) DO NOTHING;
