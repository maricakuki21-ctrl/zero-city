-- Migration: 160_shared_pools
-- Account Square shared-pool marketplace (READ-ONLY phase).
-- Replaces the hardcoded frontend AccountSquareView mock with a real backend.
--
-- SCOPE (intentionally limited): this phase delivers a real, queryable catalog of
-- shared pools + their models. Joining a pool, binding to an API key, seat-hour
-- billing and owner settlement are deliberately NOT implemented here — that logic
-- is the most error-prone part of the project (real money) and must be a separate,
-- independently-tested workstream. Until then, the "join" action stays disabled.

-- 1. Shared pool catalog
CREATE TABLE IF NOT EXISTS shared_pools (
    id                       BIGSERIAL PRIMARY KEY,
    owner_id                 BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    owner_label              VARCHAR(120) NOT NULL DEFAULT '',
    name                     VARCHAR(160) NOT NULL,
    description              TEXT NOT NULL DEFAULT '',
    tier                     VARCHAR(40) NOT NULL DEFAULT 'Standard',
    status                   VARCHAR(20) NOT NULL DEFAULT 'healthy',
    listed                   BOOLEAN NOT NULL DEFAULT TRUE,
    rate_multiplier          DECIMAL(8, 4) NOT NULL DEFAULT 1,
    max_users                INT NOT NULL DEFAULT 1,
    current_users            INT NOT NULL DEFAULT 0,
    min_balance_admission    DECIMAL(20, 8) NOT NULL DEFAULT 0,
    hourly_seat_fee          DECIMAL(20, 8) NOT NULL DEFAULT 0,
    today_availability       DECIMAL(5, 2) NOT NULL DEFAULT 0,
    seven_day_availability   DECIMAL(5, 2) NOT NULL DEFAULT 0,
    avg_latency_ms           INT NOT NULL DEFAULT 0,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shared_pools_status_check CHECK (status IN ('healthy', 'limited', 'offline', 'maintenance')),
    CONSTRAINT shared_pools_availability_check CHECK (today_availability >= 0 AND today_availability <= 100),
    CONSTRAINT shared_pools_seven_day_check CHECK (seven_day_availability >= 0 AND seven_day_availability <= 100)
);

CREATE INDEX IF NOT EXISTS idx_shared_pools_listed_status ON shared_pools(listed, status);
CREATE INDEX IF NOT EXISTS idx_shared_pools_availability ON shared_pools(today_availability DESC);

-- 2. Per-pool supported models
CREATE TABLE IF NOT EXISTS shared_pool_models (
    id           BIGSERIAL PRIMARY KEY,
    pool_id      BIGINT NOT NULL REFERENCES shared_pools(id) ON DELETE CASCADE,
    model_name   VARCHAR(128) NOT NULL,
    display_name VARCHAR(128) NOT NULL DEFAULT '',
    sort_order   INT NOT NULL DEFAULT 0,
    enabled      BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE INDEX IF NOT EXISTS idx_shared_pool_models_pool ON shared_pool_models(pool_id, sort_order);
CREATE UNIQUE INDEX IF NOT EXISTS shared_pool_models_pool_model_unique ON shared_pool_models(pool_id, model_name);

-- 3. Seed a small, realistic catalog so the marketplace shows real DB rows
--    instead of frontend mock. Idempotent: only seed when the table is empty.
DO $$
DECLARE
    v_pool_id BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM shared_pools LIMIT 1) THEN
        -- Aurora Shared Pool
        INSERT INTO shared_pools (owner_label, name, description, tier, status, rate_multiplier, max_users, current_users, min_balance_admission, hourly_seat_fee, today_availability, seven_day_availability, avg_latency_ms)
        VALUES ('community', 'Aurora Shared Pool', 'High-availability GPT-4o lane for general workloads.', 'Premium', 'healthy', 1.2, 50, 24, 5, 0.10, 99.20, 98.80, 120)
        RETURNING id INTO v_pool_id;
        INSERT INTO shared_pool_models (pool_id, model_name, sort_order) VALUES
            (v_pool_id, 'gpt-4o', 0), (v_pool_id, 'gpt-4o-mini', 1), (v_pool_id, 'o3-mini', 2);

        -- Mimir Reasoning Lane
        INSERT INTO shared_pools (owner_label, name, description, tier, status, rate_multiplier, max_users, current_users, min_balance_admission, hourly_seat_fee, today_availability, seven_day_availability, avg_latency_ms)
        VALUES ('community', 'Mimir Reasoning Lane', 'Reasoning-optimized pool (o-series + Claude Sonnet).', 'Standard', 'healthy', 1.5, 30, 12, 10, 0.15, 98.70, 97.50, 180)
        RETURNING id INTO v_pool_id;
        INSERT INTO shared_pool_models (pool_id, model_name, sort_order) VALUES
            (v_pool_id, 'o1', 0), (v_pool_id, 'o3', 1), (v_pool_id, 'claude-sonnet', 2);

        -- Pixel Creative Vault
        INSERT INTO shared_pools (owner_label, name, description, tier, status, rate_multiplier, max_users, current_users, min_balance_admission, hourly_seat_fee, today_availability, seven_day_availability, avg_latency_ms)
        VALUES ('community', 'Pixel Creative Vault', 'Image + vision models for creative work.', 'Budget', 'limited', 0.9, 20, 8, 1, 0.05, 94.60, 93.20, 250)
        RETURNING id INTO v_pool_id;
        INSERT INTO shared_pool_models (pool_id, model_name, sort_order) VALUES
            (v_pool_id, 'gpt-image', 0), (v_pool_id, 'gemini-flash', 1), (v_pool_id, 'vision', 2);

        -- Atlas Business Core
        INSERT INTO shared_pools (owner_label, name, description, tier, status, rate_multiplier, max_users, current_users, min_balance_admission, hourly_seat_fee, today_availability, seven_day_availability, avg_latency_ms)
        VALUES ('community', 'Atlas Business Core', 'Enterprise-grade frontier models with tight SLAs.', 'Enterprise', 'healthy', 1.8, 25, 18, 50, 0.25, 99.80, 99.50, 95)
        RETURNING id INTO v_pool_id;
        INSERT INTO shared_pool_models (pool_id, model_name, sort_order) VALUES
            (v_pool_id, 'gpt-4.1', 0), (v_pool_id, 'claude-opus', 1), (v_pool_id, 'gemini-pro', 2);

        -- Nebula Fast Lane
        INSERT INTO shared_pools (owner_label, name, description, tier, status, rate_multiplier, max_users, current_users, min_balance_admission, hourly_seat_fee, today_availability, seven_day_availability, avg_latency_ms)
        VALUES ('community', 'Nebula Fast Lane', 'Low-cost, high-throughput pool for light models.', 'Budget', 'healthy', 0.8, 100, 32, 1, 0.02, 97.90, 96.80, 85)
        RETURNING id INTO v_pool_id;
        INSERT INTO shared_pool_models (pool_id, model_name, sort_order) VALUES
            (v_pool_id, 'gpt-4o-mini', 0), (v_pool_id, 'deepseek-chat', 1), (v_pool_id, 'qwen-plus', 2);

        -- Oracle QA Pool
        INSERT INTO shared_pools (owner_label, name, description, tier, status, rate_multiplier, max_users, current_users, min_balance_admission, hourly_seat_fee, today_availability, seven_day_availability, avg_latency_ms)
        VALUES ('community', 'Oracle QA Pool', 'Balanced pool for QA and evaluation tasks.', 'Standard', 'maintenance', 1.1, 40, 10, 5, 0.08, 95.10, 94.30, 150)
        RETURNING id INTO v_pool_id;
        INSERT INTO shared_pool_models (pool_id, model_name, sort_order) VALUES
            (v_pool_id, 'claude-haiku', 0), (v_pool_id, 'gemini-flash', 1), (v_pool_id, 'qwen-max', 2);
    END IF;
END
$$;
