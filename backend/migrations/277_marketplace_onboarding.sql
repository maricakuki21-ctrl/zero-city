CREATE TABLE IF NOT EXISTS marketplace_onboarding (
    user_id BIGINT NOT NULL REFERENCES users(id),
    version INTEGER NOT NULL CHECK (version > 0),
    intent VARCHAR(16) NOT NULL CHECK (intent IN ('hire', 'sell', 'browse')),
    completed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, version)
);
