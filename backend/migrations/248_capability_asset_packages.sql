-- 248_capability_asset_packages.sql
-- Immutable capability-asset package versions and audited downloads.
-- Uploaded bytes are stored for delivery only; this migration does not
-- execute packages or create a second billing authority.

CREATE TABLE IF NOT EXISTS capability_asset_versions (
    id BIGSERIAL PRIMARY KEY,
    asset_id BIGINT NOT NULL REFERENCES capability_assets(id) ON DELETE CASCADE,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    version VARCHAR(64) NOT NULL CHECK (BTRIM(version) <> ''),
    runtime_kind VARCHAR(32) NOT NULL CHECK (BTRIM(runtime_kind) <> ''),
    manifest JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(manifest) = 'object'),
    package_digest CHAR(64),
    status VARCHAR(20) NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'ready', 'published', 'revoked')),
    file_count INTEGER NOT NULL DEFAULT 0 CHECK (file_count >= 0),
    total_bytes BIGINT NOT NULL DEFAULT 0 CHECK (total_bytes >= 0),
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (asset_id, version)
);

CREATE INDEX IF NOT EXISTS idx_capability_asset_versions_asset
    ON capability_asset_versions (asset_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_capability_asset_versions_public
    ON capability_asset_versions (asset_id, id DESC) WHERE status = 'published';

CREATE TABLE IF NOT EXISTS capability_asset_files (
    id BIGSERIAL PRIMARY KEY,
    version_id BIGINT NOT NULL REFERENCES capability_asset_versions(id) ON DELETE CASCADE,
    relative_path VARCHAR(512) NOT NULL CHECK (BTRIM(relative_path) <> ''),
    content_type VARCHAR(160) NOT NULL DEFAULT 'application/octet-stream',
    byte_size BIGINT NOT NULL CHECK (byte_size >= 0 AND byte_size <= 8388608),
    sha256 CHAR(64) NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    content BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (version_id, relative_path)
);

CREATE INDEX IF NOT EXISTS idx_capability_asset_files_version
    ON capability_asset_files (version_id, relative_path);

CREATE TABLE IF NOT EXISTS capability_asset_download_events (
    id BIGSERIAL PRIMARY KEY,
    version_id BIGINT NOT NULL REFERENCES capability_asset_versions(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_capability_asset_downloads_version
    ON capability_asset_download_events (version_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_capability_asset_downloads_user
    ON capability_asset_download_events (user_id, id DESC);
