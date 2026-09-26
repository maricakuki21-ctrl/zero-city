CREATE TABLE IF NOT EXISTS asset_workflow_checkpoints (
    actor_id BIGINT NOT NULL REFERENCES users(id),
    request_id VARCHAR(160) NOT NULL,
    input_digest CHAR(64) NOT NULL,
    state JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (actor_id, request_id),
    CHECK (octet_length(state::text) <= 4194304)
);
CREATE INDEX IF NOT EXISTS asset_workflow_owner_history_idx
    ON asset_workflow_checkpoints(actor_id, (state->>'asset_id'), (state->>'version'), created_at DESC);
