-- 100_capability_assets.sql
-- Capability assets are durable showcase entries for completed user work.
-- They are intentionally separate from operator/workbench task execution.

CREATE TABLE IF NOT EXISTS capability_assets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(160) NOT NULL,
    slug VARCHAR(190) NOT NULL,
    summary VARCHAR(360) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    asset_type VARCHAR(40) NOT NULL DEFAULT 'other',
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    scenario_tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    integration_tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    cover_url TEXT NOT NULL DEFAULT '',
    screenshot_urls JSONB NOT NULL DEFAULT '[]'::jsonb,
    video_url TEXT NOT NULL DEFAULT '',
    demo_url TEXT NOT NULL DEFAULT '',
    doc_url TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL DEFAULT '',
    template_url TEXT NOT NULL DEFAULT '',
    primary_action_type VARCHAR(40) NOT NULL DEFAULT 'view_detail',
    pricing_type VARCHAR(20) NOT NULL DEFAULT 'free',
    contact_enabled BOOLEAN NOT NULL DEFAULT false,
    is_featured BOOLEAN NOT NULL DEFAULT false,
    featured_weight INT NOT NULL DEFAULT 0,
    view_count BIGINT NOT NULL DEFAULT 0,
    like_count BIGINT NOT NULL DEFAULT 0,
    favorite_count BIGINT NOT NULL DEFAULT 0,
    comment_count BIGINT NOT NULL DEFAULT 0,
    rating_avg NUMERIC(4,2) NOT NULL DEFAULT 0,
    rating_count BIGINT NOT NULL DEFAULT 0,
    review_note TEXT NOT NULL DEFAULT '',
    reviewed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_capability_assets_slug_alive
    ON capability_assets(slug)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_capability_assets_public_list
    ON capability_assets(status, is_featured DESC, featured_weight DESC, published_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_capability_assets_user_status
    ON capability_assets(user_id, status, updated_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_capability_assets_type_status
    ON capability_assets(asset_type, status, published_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_capability_assets_tags_gin
    ON capability_assets USING GIN(tags);

CREATE INDEX IF NOT EXISTS idx_capability_assets_scenarios_gin
    ON capability_assets USING GIN(scenario_tags);

COMMENT ON TABLE capability_assets IS 'User-created capability assets: products, games, workflows, agents, tools, plugins, prompts and reports shown as durable showcase entries.';
COMMENT ON COLUMN capability_assets.status IS 'draft / pending / listed / rejected / archived / delisted';
COMMENT ON COLUMN capability_assets.asset_type IS 'product_app / game / workflow / agent / api_tool / plugin_template / prompt_solution / dataset_report / other';
COMMENT ON COLUMN capability_assets.primary_action_type IS 'visit_product / open_demo / view_workflow / open_agent / view_docs / copy_prompt / view_report / download_template / contact_author / view_detail';
