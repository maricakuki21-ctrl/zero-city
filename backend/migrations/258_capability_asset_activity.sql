-- 258_capability_asset_activity.sql
-- Real, append-only activity for capability assets.
--
-- Counts are derived from these tables. The legacy counter columns on
-- capability_assets remain for compatibility, but are not the source of truth.

CREATE TABLE IF NOT EXISTS capability_asset_view_events (
    id BIGSERIAL PRIMARY KEY,
    asset_id BIGINT NOT NULL REFERENCES capability_assets(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    view_day DATE NOT NULL DEFAULT CURRENT_DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (asset_id, user_id, view_day)
);

CREATE INDEX IF NOT EXISTS idx_capability_asset_views_asset
    ON capability_asset_view_events (asset_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_capability_asset_views_user
    ON capability_asset_view_events (user_id, id DESC);

CREATE TABLE IF NOT EXISTS capability_asset_likes (
    asset_id BIGINT NOT NULL REFERENCES capability_assets(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (asset_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_capability_asset_likes_user
    ON capability_asset_likes (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS capability_asset_favorites (
    asset_id BIGINT NOT NULL REFERENCES capability_assets(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (asset_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_capability_asset_favorites_user
    ON capability_asset_favorites (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS capability_asset_use_events (
    id BIGSERIAL PRIMARY KEY,
    asset_id BIGINT NOT NULL REFERENCES capability_assets(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type VARCHAR(24) NOT NULL CHECK (event_type IN ('run', 'install', 'derive')),
    source_kind VARCHAR(40) NOT NULL DEFAULT '',
    source_id VARCHAR(160) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_capability_asset_use_events_asset
    ON capability_asset_use_events (asset_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_capability_asset_use_events_user
    ON capability_asset_use_events (user_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_capability_asset_use_events_source
    ON capability_asset_use_events (asset_id, user_id, event_type, source_kind, source_id)
    WHERE BTRIM(source_id) <> '';

COMMENT ON TABLE capability_asset_view_events IS
    'Authenticated public views, deduplicated by user and UTC date. Author views are not recorded.';
COMMENT ON TABLE capability_asset_use_events IS
    'Successful install/run/derive events only. Opening a detail page must never create a use event.';
