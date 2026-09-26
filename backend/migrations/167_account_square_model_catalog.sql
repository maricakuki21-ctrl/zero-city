-- Account Square model-first foundation.
-- Builds a platform model catalog and per-pool model controls for pricing,
-- ranking, protection, and future routing.

CREATE TABLE IF NOT EXISTS model_catalog (
    id                      BIGSERIAL PRIMARY KEY,
    provider                VARCHAR(64) NOT NULL,
    model_name              VARCHAR(128) NOT NULL,
    display_name            VARCHAR(128) NOT NULL DEFAULT '',
    family                  VARCHAR(64) NOT NULL DEFAULT '',
    tier_label              VARCHAR(32) NOT NULL DEFAULT 'Unknown',
    capability_tags         JSONB NOT NULL DEFAULT '[]'::jsonb,
    aliases                 JSONB NOT NULL DEFAULT '[]'::jsonb,
    default_rate_multiplier NUMERIC(12, 4) NOT NULL DEFAULT 1,
    default_rank_weight     NUMERIC(12, 4) NOT NULL DEFAULT 100,
    mainstream              BOOLEAN NOT NULL DEFAULT TRUE,
    enabled                 BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order              INT NOT NULL DEFAULT 0,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, model_name)
);

CREATE INDEX IF NOT EXISTS idx_model_catalog_enabled_sort
    ON model_catalog(enabled, mainstream DESC, sort_order, provider, model_name);
CREATE INDEX IF NOT EXISTS idx_model_catalog_model_name
    ON model_catalog(model_name);

ALTER TABLE shared_pool_models
    ADD COLUMN IF NOT EXISTS provider VARCHAR(64) NOT NULL DEFAULT 'openai',
    ADD COLUMN IF NOT EXISTS model_aliases JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS rate_multiplier NUMERIC(12, 4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS rank_weight NUMERIC(12, 4) NOT NULL DEFAULT 100,
    ADD COLUMN IF NOT EXISTS five_hour_protection_percent NUMERIC(5, 2) NOT NULL DEFAULT 100,
    ADD COLUMN IF NOT EXISTS seven_day_protection_percent NUMERIC(5, 2) NOT NULL DEFAULT 100,
    ADD COLUMN IF NOT EXISTS min_balance_admission NUMERIC(12, 4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS hourly_seat_fee NUMERIC(12, 4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS max_concurrency INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tags JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE INDEX IF NOT EXISTS idx_shared_pool_models_model_enabled
    ON shared_pool_models(model_name, enabled);
CREATE INDEX IF NOT EXISTS idx_shared_pool_models_provider_enabled
    ON shared_pool_models(provider, enabled);

INSERT INTO model_catalog (provider, model_name, display_name, family, tier_label, capability_tags, aliases, default_rate_multiplier, default_rank_weight, sort_order)
VALUES
    ('openai', 'gpt-5.5', 'GPT-5.5', 'gpt', 'Team', '["chat","reasoning","frontier"]', '["gpt5.5"]', 1.60, 120, 10),
    ('openai', 'gpt-5.4', 'GPT-5.4', 'gpt', 'Pro', '["chat","reasoning","frontier"]', '["gpt5.4"]', 1.40, 115, 20),
    ('openai', 'gpt-5.4-mini', 'GPT-5.4 Mini', 'gpt', 'Plus', '["chat","fast"]', '["gpt5.4-mini"]', 0.60, 105, 30),
    ('openai', 'gpt-5.2', 'GPT-5.2', 'gpt', 'Pro', '["chat","reasoning"]', '["gpt5.2"]', 1.20, 110, 40),
    ('openai', 'gpt-4.1', 'GPT-4.1', 'gpt', 'Plus', '["chat","vision"]', '["gpt-4.1-latest"]', 1.00, 100, 50),
    ('openai', 'gpt-4o', 'GPT-4o', 'gpt', 'Plus', '["chat","vision","realtime"]', '["gpt-4o-latest"]', 0.90, 98, 60),
    ('openai', 'o3', 'o3', 'o-series', 'Pro', '["reasoning"]', '["o3-latest"]', 1.30, 110, 70),
    ('openai', 'o4-mini', 'o4 mini', 'o-series', 'Plus', '["reasoning","fast"]', '["o4-mini-latest"]', 0.70, 95, 80),
    ('openai', 'codex-auto-review', 'Codex Auto Review', 'codex', 'Team', '["code","review"]', '["codex"]', 1.20, 105, 90),
    ('anthropic', 'claude-sonnet-4.6', 'Claude Sonnet 4.6', 'claude', 'Pro', '["chat","code","reasoning"]', '["claude-sonnet","sonnet-4.6"]', 1.20, 105, 110),
    ('anthropic', 'claude-opus-4.5', 'Claude Opus 4.5', 'claude', 'Pro', '["chat","reasoning","frontier"]', '["claude-opus","opus-4.5"]', 1.50, 110, 120),
    ('anthropic', 'claude-haiku-4.5', 'Claude Haiku 4.5', 'claude', 'Plus', '["chat","fast"]', '["claude-haiku","haiku-4.5"]', 0.55, 90, 130),
    ('google', 'gemini-2.5-pro', 'Gemini 2.5 Pro', 'gemini', 'Pro', '["chat","reasoning","vision"]', '["gemini-pro"]', 1.10, 100, 150),
    ('google', 'gemini-2.5-flash', 'Gemini 2.5 Flash', 'gemini', 'Plus', '["chat","fast","vision"]', '["gemini-flash"]', 0.45, 88, 160),
    ('xai', 'grok-4', 'Grok 4', 'grok', 'Pro', '["chat","reasoning"]', '["grok"]', 1.10, 95, 180),
    ('deepseek', 'deepseek-chat', 'DeepSeek Chat', 'deepseek', 'Plus', '["chat"]', '[]', 0.35, 85, 200),
    ('deepseek', 'deepseek-reasoner', 'DeepSeek Reasoner', 'deepseek', 'Pro', '["reasoning"]', '[]', 0.65, 90, 210),
    ('qwen', 'qwen-max', 'Qwen Max', 'qwen', 'Pro', '["chat","reasoning"]', '[]', 0.80, 88, 230),
    ('qwen', 'qwen-plus', 'Qwen Plus', 'qwen', 'Plus', '["chat"]', '[]', 0.45, 82, 240),
    ('moonshot', 'kimi-k2', 'Kimi K2', 'kimi', 'Plus', '["chat","long-context"]', '["kimi"]', 0.55, 82, 260),
    ('zhipu', 'glm-4.5', 'GLM-4.5', 'glm', 'Plus', '["chat","reasoning"]', '["glm"]', 0.55, 82, 280)
ON CONFLICT (provider, model_name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    family = EXCLUDED.family,
    tier_label = EXCLUDED.tier_label,
    capability_tags = EXCLUDED.capability_tags,
    aliases = EXCLUDED.aliases,
    default_rate_multiplier = EXCLUDED.default_rate_multiplier,
    default_rank_weight = EXCLUDED.default_rank_weight,
    mainstream = TRUE,
    enabled = TRUE,
    sort_order = EXCLUDED.sort_order,
    updated_at = NOW();
