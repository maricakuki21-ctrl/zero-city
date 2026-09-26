CREATE TABLE IF NOT EXISTS canonical_media_task_bindings (
    business_event_id TEXT PRIMARY KEY,
    idempotency_key TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    api_key_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    endpoint TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'pending',
    upstream_task_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_at TIMESTAMPTZ,
    CONSTRAINT canonical_media_task_identity_unique UNIQUE (api_key_id, idempotency_key),
    CONSTRAINT canonical_media_task_request_hash_length CHECK (length(request_hash) = 64),
    CONSTRAINT canonical_media_task_state_check CHECK (state IN ('pending', 'accepted')),
    CONSTRAINT canonical_media_task_accepted_shape_check CHECK (
        (state = 'pending' AND upstream_task_id IS NULL AND accepted_at IS NULL)
        OR (state = 'accepted' AND upstream_task_id IS NOT NULL AND accepted_at IS NOT NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_canonical_media_task_upstream_owner
    ON canonical_media_task_bindings (api_key_id, user_id, upstream_task_id)
    WHERE upstream_task_id IS NOT NULL;
