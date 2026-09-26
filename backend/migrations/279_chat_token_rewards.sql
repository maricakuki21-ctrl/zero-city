-- Additive only: promotional token entitlements never change a recipient's cash.
CREATE TABLE biz_token_packets (
    id BIGSERIAL PRIMARY KEY,
    channel_id BIGINT NOT NULL REFERENCES biz_chat_channels(id),
    sender_id BIGINT NOT NULL REFERENCES users(id),
    client_id VARCHAR(100) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    message_id BIGINT UNIQUE REFERENCES biz_chat_messages(id),
    resource_id VARCHAR(64) NOT NULL,
    model VARCHAR(200) NOT NULL,
    sponsor_key_id BIGINT NOT NULL REFERENCES api_keys(id),
    total_tokens BIGINT NOT NULL CHECK (total_tokens BETWEEN 1 AND 1000000000),
    portions INTEGER NOT NULL CHECK (portions BETWEEN 1 AND 500),
    allocations BIGINT[] NOT NULL,
    claimed INTEGER NOT NULL DEFAULT 0 CHECK (claimed >= 0 AND claimed <= portions),
    mode VARCHAR(12) NOT NULL CHECK (mode IN ('equal', 'random')),
    blessing VARCHAR(160) NOT NULL,
    opens_at TIMESTAMPTZ NOT NULL,
    closes_at TIMESTAMPTZ NOT NULL CHECK (closes_at > opens_at),
    use_hours INTEGER NOT NULL CHECK (use_hours BETWEEN 1 AND 720),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(sender_id, client_id),
    CHECK (array_length(allocations, 1) = portions)
);
CREATE TABLE biz_token_grants (
    id BIGSERIAL PRIMARY KEY,
    packet_id BIGINT NOT NULL REFERENCES biz_token_packets(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    tokens BIGINT NOT NULL CHECK (tokens > 0),
    used BIGINT NOT NULL DEFAULT 0 CHECK (used >= 0),
    reserved BIGINT NOT NULL DEFAULT 0 CHECK (reserved >= 0),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(packet_id, user_id),
    CHECK (used + reserved <= tokens)
);
CREATE INDEX biz_token_grants_user_expiry ON biz_token_grants(user_id, expires_at, id);
CREATE TABLE biz_token_runs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    client_id VARCHAR(100) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    resource_id VARCHAR(64) NOT NULL,
    sponsor_key_id BIGINT NOT NULL REFERENCES api_keys(id),
    model VARCHAR(200) NOT NULL,
    reserved BIGINT NOT NULL CHECK (reserved > 0),
    actual BIGINT CHECK (actual >= 0),
    covered BIGINT CHECK (covered >= 0),
    status VARCHAR(16) NOT NULL CHECK (status IN ('pending', 'succeeded', 'failed', 'review')),
    result TEXT NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT '',
    gateway_request_id VARCHAR(200) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, client_id)
);
CREATE TABLE biz_token_run_grants (
    run_id BIGINT NOT NULL REFERENCES biz_token_runs(id),
    grant_id BIGINT NOT NULL REFERENCES biz_token_grants(id),
    reserved BIGINT NOT NULL CHECK (reserved > 0),
    PRIMARY KEY(run_id, grant_id)
);
CREATE INDEX biz_token_runs_review ON biz_token_runs(status, created_at);
CREATE TABLE biz_token_run_resolutions (
    run_id BIGINT PRIMARY KEY REFERENCES biz_token_runs(id),
    operator_id BIGINT NOT NULL REFERENCES users(id),
    actual BIGINT NOT NULL CHECK (actual BETWEEN 0 AND 2000000000),
    evidence TEXT NOT NULL CHECK (char_length(evidence) BETWEEN 10 AND 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
