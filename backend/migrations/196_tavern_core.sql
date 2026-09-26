-- 196_tavern_core.sql
-- Zero City Tavern core keeps the business control plane in BizDecipher.
-- SillyTavern or other AGPL runtime code must stay outside the main repository.

CREATE TABLE IF NOT EXISTS tavern_scripts (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(160) NOT NULL,
    slug VARCHAR(190) NOT NULL,
    summary VARCHAR(360) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    genre VARCHAR(40) NOT NULL DEFAULT 'mystery',
    status VARCHAR(24) NOT NULL DEFAULT 'draft',
    visibility VARCHAR(24) NOT NULL DEFAULT 'private',
    player_min INT NOT NULL DEFAULT 1,
    player_max INT NOT NULL DEFAULT 6,
    estimated_minutes INT NOT NULL DEFAULT 60,
    difficulty VARCHAR(24) NOT NULL DEFAULT 'normal',
    tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    npc_cards JSONB NOT NULL DEFAULT '[]'::jsonb,
    host_brief TEXT NOT NULL DEFAULT '',
    opening_prompt TEXT NOT NULL DEFAULT '',
    safety_notes TEXT NOT NULL DEFAULT '',
    pricing_mode VARCHAR(24) NOT NULL DEFAULT 'free',
    entry_credit_cost INT NOT NULL DEFAULT 0,
    entry_balance_cost NUMERIC(12, 4) NOT NULL DEFAULT 0,
    author_revenue_share NUMERIC(5, 2) NOT NULL DEFAULT 0,
    quality_score NUMERIC(4, 2) NOT NULL DEFAULT 0,
    review_note TEXT NOT NULL DEFAULT '',
    reviewed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT tavern_scripts_player_range CHECK (player_min >= 1 AND player_max >= player_min AND player_max <= 12),
    CONSTRAINT tavern_scripts_estimated_minutes_check CHECK (estimated_minutes >= 10 AND estimated_minutes <= 480),
    CONSTRAINT tavern_scripts_entry_credit_cost_check CHECK (entry_credit_cost >= 0),
    CONSTRAINT tavern_scripts_entry_balance_cost_check CHECK (entry_balance_cost >= 0),
    CONSTRAINT tavern_scripts_author_revenue_share_check CHECK (author_revenue_share >= 0 AND author_revenue_share <= 100),
    CONSTRAINT tavern_scripts_quality_score_check CHECK (quality_score >= 0 AND quality_score <= 100)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tavern_scripts_slug_alive
    ON tavern_scripts(slug)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_tavern_scripts_public_list
    ON tavern_scripts(status, visibility, quality_score DESC, published_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_tavern_scripts_user_status
    ON tavern_scripts(user_id, status, updated_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_tavern_scripts_genre_status
    ON tavern_scripts(genre, status, published_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_tavern_scripts_tags_gin
    ON tavern_scripts USING GIN(tags);

CREATE TABLE IF NOT EXISTS tavern_rooms (
    id BIGSERIAL PRIMARY KEY,
    script_id BIGINT NOT NULL REFERENCES tavern_scripts(id) ON DELETE RESTRICT,
    owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(160) NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'draft',
    visibility VARCHAR(24) NOT NULL DEFAULT 'private',
    host_mode VARCHAR(24) NOT NULL DEFAULT 'ai_host',
    billing_mode VARCHAR(24) NOT NULL DEFAULT 'free',
    entry_credit_cost INT NOT NULL DEFAULT 0,
    entry_balance_cost NUMERIC(12, 4) NOT NULL DEFAULT 0,
    max_players INT NOT NULL DEFAULT 6,
    current_players INT NOT NULL DEFAULT 0,
    current_phase VARCHAR(32) NOT NULL DEFAULT 'lobby',
    room_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT tavern_rooms_player_count_check CHECK (max_players >= 1 AND max_players <= 12 AND current_players >= 0 AND current_players <= max_players),
    CONSTRAINT tavern_rooms_entry_credit_cost_check CHECK (entry_credit_cost >= 0),
    CONSTRAINT tavern_rooms_entry_balance_cost_check CHECK (entry_balance_cost >= 0)
);

CREATE INDEX IF NOT EXISTS idx_tavern_rooms_owner_status
    ON tavern_rooms(owner_id, status, updated_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_tavern_rooms_script_status
    ON tavern_rooms(script_id, status, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_tavern_rooms_public_list
    ON tavern_rooms(status, visibility, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS tavern_room_players (
    id BIGSERIAL PRIMARY KEY,
    room_id BIGINT NOT NULL REFERENCES tavern_rooms(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_name VARCHAR(80) NOT NULL DEFAULT '',
    status VARCHAR(24) NOT NULL DEFAULT 'joined',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    left_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(room_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_tavern_room_players_user
    ON tavern_room_players(user_id, joined_at DESC);

COMMENT ON TABLE tavern_scripts IS 'Zero City Tavern user-submitted scripts. Main repository stores review, billing and governance control data only.';
COMMENT ON COLUMN tavern_scripts.status IS 'draft / pending / listed / rejected / archived / delisted';
COMMENT ON COLUMN tavern_scripts.visibility IS 'private / public';
COMMENT ON COLUMN tavern_scripts.genre IS 'mystery / sci_fi / fantasy / horror / workplace / historical / open_world / other';
COMMENT ON COLUMN tavern_scripts.pricing_mode IS 'free / credit / balance / hybrid';
COMMENT ON TABLE tavern_rooms IS 'Runnable tavern rooms created from listed scripts. First phase supports draft/lobby control before realtime runtime integration.';
COMMENT ON COLUMN tavern_rooms.status IS 'draft / lobby / running / paused / completed / cancelled';
COMMENT ON COLUMN tavern_rooms.host_mode IS 'ai_host / human_host / mixed';
COMMENT ON COLUMN tavern_rooms.billing_mode IS 'free / credit / balance / hybrid';
