-- Reuse Sub2 account tests without copying upstream credentials.
ALTER TABLE scheduled_test_plans ADD COLUMN IF NOT EXISTS auto_managed boolean NOT NULL DEFAULT false;
CREATE UNIQUE INDEX IF NOT EXISTS idx_scheduled_tests_auto_identity
    ON scheduled_test_plans(account_id, model_id) WHERE auto_managed = true;
CREATE INDEX IF NOT EXISTS idx_scheduled_test_results_plan_started
    ON scheduled_test_results(plan_id, started_at DESC);
