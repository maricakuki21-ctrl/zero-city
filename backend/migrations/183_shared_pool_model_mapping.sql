ALTER TABLE shared_pool_models
    ADD COLUMN IF NOT EXISTS upstream_model_name VARCHAR(128) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_shared_pool_models_upstream_model_name
    ON shared_pool_models(upstream_model_name)
    WHERE upstream_model_name <> '';
