-- Migration: 151_bizdecipher_asset_layer
-- BizDecipher Shared Gateway v1 用户资产、credits、贡献者、operator 和需求入口。
-- 这些表引用 Sub2API users，但保持 BizDecipher 业务资产层独立，便于未来替换网关底座。

CREATE TABLE IF NOT EXISTS biz_profiles (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    handle       VARCHAR(80),
    display_name VARCHAR(120) NOT NULL DEFAULT '',
    avatar_url   TEXT NOT NULL DEFAULT '',
    bio          TEXT NOT NULL DEFAULT '',
    role_flags   JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS biz_profiles_user_id_unique ON biz_profiles(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS biz_profiles_handle_unique ON biz_profiles(handle) WHERE handle IS NOT NULL AND handle <> '';

CREATE TABLE IF NOT EXISTS biz_contacts (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        VARCHAR(30) NOT NULL,
    value       VARCHAR(255) NOT NULL,
    verified_at TIMESTAMPTZ NULL,
    source      VARCHAR(50) NOT NULL DEFAULT 'manual',
    is_primary  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT biz_contacts_type_check CHECK (type IN ('email', 'telegram', 'discord', 'other'))
);

CREATE INDEX IF NOT EXISTS idx_biz_contacts_user_id ON biz_contacts(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS biz_contacts_user_type_value_unique ON biz_contacts(user_id, type, value);
CREATE UNIQUE INDEX IF NOT EXISTS biz_contacts_primary_unique ON biz_contacts(user_id, type) WHERE is_primary;

CREATE TABLE IF NOT EXISTS biz_identity_links (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider          VARCHAR(50) NOT NULL,
    provider_user_id  VARCHAR(255) NOT NULL,
    provider_username VARCHAR(255) NOT NULL DEFAULT '',
    provider_avatar   TEXT NOT NULL DEFAULT '',
    provider_email    VARCHAR(255) NOT NULL DEFAULT '',
    raw_profile_json  JSONB NOT NULL DEFAULT '{}'::jsonb,
    linked_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at      TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_biz_identity_links_user_id ON biz_identity_links(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS biz_identity_links_provider_subject_unique ON biz_identity_links(provider, provider_user_id);

CREATE TABLE IF NOT EXISTS credit_ledger (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_type   VARCHAR(50) NOT NULL,
    source_id     VARCHAR(120) NOT NULL DEFAULT '',
    amount        DECIMAL(20, 8) NOT NULL,
    balance_after DECIMAL(20, 8) NOT NULL,
    status        VARCHAR(20) NOT NULL DEFAULT 'posted',
    note          TEXT NOT NULL DEFAULT '',
    created_by    BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    posted_at     TIMESTAMPTZ NULL,
    CONSTRAINT credit_ledger_source_type_check CHECK (source_type IN ('starter', 'purchase', 'contribution', 'operator_reward', 'admin_adjustment')),
    CONSTRAINT credit_ledger_status_check CHECK (status IN ('pending', 'posted', 'reversed'))
);

CREATE INDEX IF NOT EXISTS idx_credit_ledger_user_created ON credit_ledger(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_credit_ledger_source ON credit_ledger(source_type, source_id);

CREATE TABLE IF NOT EXISTS contributor_profiles (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status            VARCHAR(20) NOT NULL DEFAULT 'pending',
    capacity_types    JSONB NOT NULL DEFAULT '[]'::jsonb,
    settlement_method VARCHAR(120) NOT NULL DEFAULT '',
    risk_notes        TEXT NOT NULL DEFAULT '',
    admin_notes       TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT contributor_profiles_status_check CHECK (status IN ('pending', 'approved', 'paused', 'rejected'))
);

CREATE UNIQUE INDEX IF NOT EXISTS contributor_profiles_user_id_unique ON contributor_profiles(user_id);
CREATE INDEX IF NOT EXISTS idx_contributor_profiles_status ON contributor_profiles(status);

CREATE TABLE IF NOT EXISTS capacity_contributions (
    id               BIGSERIAL PRIMARY KEY,
    contributor_id   BIGINT NOT NULL REFERENCES contributor_profiles(id) ON DELETE CASCADE,
    resource_type    VARCHAR(80) NOT NULL,
    label            VARCHAR(160) NOT NULL,
    status           VARCHAR(20) NOT NULL DEFAULT 'submitted',
    capacity_hint    TEXT NOT NULL DEFAULT '',
    health_score     INT NOT NULL DEFAULT 0,
    last_checked_at  TIMESTAMPTZ NULL,
    reward_policy_id VARCHAR(120) NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT capacity_contributions_status_check CHECK (status IN ('submitted', 'testing', 'active', 'degraded', 'paused', 'retired')),
    CONSTRAINT capacity_contributions_health_score_check CHECK (health_score >= 0 AND health_score <= 100)
);

CREATE INDEX IF NOT EXISTS idx_capacity_contributions_contributor_id ON capacity_contributions(contributor_id);
CREATE INDEX IF NOT EXISTS idx_capacity_contributions_status ON capacity_contributions(status);

CREATE TABLE IF NOT EXISTS operator_profiles (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    skills_json     JSONB NOT NULL DEFAULT '[]'::jsonb,
    rank            VARCHAR(40) NOT NULL DEFAULT 'candidate',
    completed_tasks INT NOT NULL DEFAULT 0,
    quality_score   DECIMAL(5, 2) NOT NULL DEFAULT 0,
    gateway_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT operator_profiles_status_check CHECK (status IN ('pending', 'approved', 'paused', 'rejected'))
);

CREATE UNIQUE INDEX IF NOT EXISTS operator_profiles_user_id_unique ON operator_profiles(user_id);
CREATE INDEX IF NOT EXISTS idx_operator_profiles_status ON operator_profiles(status);

CREATE TABLE IF NOT EXISTS custom_requests (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    goal            TEXT NOT NULL,
    context         TEXT NOT NULL DEFAULT '',
    budget_range    VARCHAR(120) NOT NULL DEFAULT '',
    deadline        TIMESTAMPTZ NULL,
    references_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    status          VARCHAR(20) NOT NULL DEFAULT 'submitted',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT custom_requests_status_check CHECK (status IN ('submitted', 'reviewing', 'quoted', 'accepted', 'assigned', 'delivered', 'closed'))
);

CREATE INDEX IF NOT EXISTS idx_custom_requests_user_created ON custom_requests(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_custom_requests_status ON custom_requests(status);

CREATE TABLE IF NOT EXISTS delivery_rooms (
    id               BIGSERIAL PRIMARY KEY,
    request_id       BIGINT NULL REFERENCES custom_requests(id) ON DELETE SET NULL,
    order_id         BIGINT NULL,
    buyer_user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    operator_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    status           VARCHAR(30) NOT NULL DEFAULT 'open',
    brief_json       JSONB NOT NULL DEFAULT '{}'::jsonb,
    final_output_url TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_delivery_rooms_request_id ON delivery_rooms(request_id);
CREATE INDEX IF NOT EXISTS idx_delivery_rooms_buyer_user_id ON delivery_rooms(buyer_user_id);
CREATE INDEX IF NOT EXISTS idx_delivery_rooms_operator_user_id ON delivery_rooms(operator_user_id);
CREATE INDEX IF NOT EXISTS idx_delivery_rooms_status ON delivery_rooms(status);
