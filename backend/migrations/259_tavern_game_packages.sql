-- 259_tavern_game_packages.sql
-- Versioned declarative game packages and room-to-package pinning.
--
-- First phase intentionally supports only declarative manifests. Uploaded
-- remote code is not executed by the main application process.

CREATE TABLE IF NOT EXISTS tavern_game_packages (
    id BIGSERIAL PRIMARY KEY,
    script_id BIGINT NOT NULL REFERENCES tavern_scripts(id) ON DELETE CASCADE,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    version VARCHAR(64) NOT NULL,
    schema_version VARCHAR(64) NOT NULL,
    runtime_kind VARCHAR(32) NOT NULL
        CHECK (runtime_kind IN ('declarative')),
    protocol_version VARCHAR(64) NOT NULL,
    manifest JSONB NOT NULL DEFAULT '{}'::jsonb
        CHECK (jsonb_typeof(manifest) = 'object'),
    package_digest CHAR(64)
        CHECK (package_digest IS NULL OR package_digest ~ '^[0-9a-f]{64}$'),
    status VARCHAR(20) NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'published', 'revoked')),
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (script_id, version)
);

CREATE INDEX IF NOT EXISTS idx_tavern_game_packages_script
    ON tavern_game_packages (script_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_tavern_game_packages_public
    ON tavern_game_packages (script_id, id DESC)
    WHERE status = 'published';
CREATE INDEX IF NOT EXISTS idx_tavern_game_packages_owner
    ON tavern_game_packages (owner_user_id, updated_at DESC);

ALTER TABLE tavern_rooms
    ADD COLUMN IF NOT EXISTS package_id BIGINT REFERENCES tavern_game_packages(id) ON DELETE RESTRICT;

-- Existing scripts become explicit v1 packages so every listed script remains
-- runnable without pretending that it has an unversioned external runtime.
INSERT INTO tavern_game_packages (
    script_id, owner_user_id, version, schema_version, runtime_kind,
    protocol_version, manifest, status, published_at
)
SELECT
    s.id,
    s.user_id,
    'v1',
    'tavern.package.v1',
    'declarative',
    '2026-09-13.package.v1',
    jsonb_build_object(
        'schema_version', 'tavern.package.v1',
        'runtime_kind', 'declarative',
        'protocol_version', '2026-09-13.package.v1',
        'entry', jsonb_build_object('kind', 'prompt_flow', 'ref', s.slug),
        'permissions', jsonb_build_object(
            'ai_gateway', true,
            'save', true,
            'score', false,
            'purchases', false,
            'presence', false
        ),
        'content', jsonb_build_object(
            'description', s.description,
            'host_brief', s.host_brief,
            'opening_prompt', s.opening_prompt,
            'safety_notes', s.safety_notes,
            'npc_cards', s.npc_cards
        ),
        'limits', jsonb_build_object('max_turns', 48, 'max_scenes', 12)
    ),
    'published',
    COALESCE(s.published_at, s.created_at, NOW())
FROM tavern_scripts s
WHERE s.deleted_at IS NULL
ON CONFLICT (script_id, version) DO NOTHING;

UPDATE tavern_rooms r
SET package_id = p.id
FROM (
    SELECT DISTINCT ON (script_id) id, script_id
    FROM tavern_game_packages
    WHERE status = 'published'
    ORDER BY script_id, id DESC
) p
WHERE r.script_id = p.script_id
  AND r.package_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_tavern_rooms_package
    ON tavern_rooms (package_id, status);

COMMENT ON TABLE tavern_game_packages IS
    'Immutable declarative game packages. Rooms pin one published package version; no arbitrary remote code is executed by the main process.';
COMMENT ON COLUMN tavern_game_packages.manifest IS
    'Strict tavern.package.v1 manifest containing entry, permissions, content and limits.';
