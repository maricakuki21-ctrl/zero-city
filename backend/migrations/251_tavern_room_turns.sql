-- 251_tavern_room_turns.sql
-- Persisted room text turns. This is the transport and audit trail only:
-- it does not grant an AI host, charge money, or settle creator revenue.

CREATE TABLE IF NOT EXISTS tavern_room_turns (
    id BIGSERIAL PRIMARY KEY,
    room_id BIGINT NOT NULL REFERENCES tavern_rooms(id) ON DELETE CASCADE,
    author_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    author_name VARCHAR(120) NOT NULL DEFAULT '',
    author_role VARCHAR(16) NOT NULL DEFAULT 'player',
    turn_index INTEGER NOT NULL,
    kind VARCHAR(24) NOT NULL DEFAULT 'player',
    client_message_id VARCHAR(96) NOT NULL,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT tavern_room_turns_index_positive CHECK (turn_index > 0),
    CONSTRAINT tavern_room_turns_body_length CHECK (
        char_length(btrim(body)) > 0 AND char_length(body) <= 4000
    ),
    CONSTRAINT tavern_room_turns_author_role_check CHECK (
        author_role IN ('owner', 'player', 'system')
    ),
    CONSTRAINT tavern_room_turns_kind_check CHECK (
        kind IN ('player', 'host', 'system')
    ),
    UNIQUE(room_id, turn_index),
    UNIQUE(room_id, client_message_id)
);

CREATE INDEX IF NOT EXISTS idx_tavern_room_turns_room_index
    ON tavern_room_turns(room_id, turn_index ASC, id ASC);

COMMENT ON TABLE tavern_room_turns IS
    'Persisted Zero City Tavern room turns. AI host execution and billing remain separate capabilities.';
COMMENT ON COLUMN tavern_room_turns.client_message_id IS
    'Client-generated idempotency key scoped to one room.';
