CREATE TABLE IF NOT EXISTS creator_columns (
    id BIGSERIAL PRIMARY KEY,
    owner_user_id BIGINT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(120) NOT NULL CHECK (char_length(btrim(title)) > 0),
    description VARCHAR(2000) NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
    moderation_reason VARCHAR(2000) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS creator_columns_public_idx ON creator_columns (id DESC) WHERE status = 'active';

CREATE TABLE IF NOT EXISTS creator_column_articles (
    id BIGSERIAL PRIMARY KEY,
    column_id BIGINT NOT NULL REFERENCES creator_columns(id) ON DELETE CASCADE,
    author_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(180) NOT NULL CHECK (char_length(btrim(title)) > 0),
    summary VARCHAR(2000) NOT NULL DEFAULT '',
    body TEXT NOT NULL CHECK (char_length(body) <= 100000),
    status VARCHAR(16) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS creator_column_articles_page_idx ON creator_column_articles (column_id, id DESC);
