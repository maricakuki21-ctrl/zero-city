-- 245_marketplace_listings_and_inquiries.sql
-- Public marketplace listings and private participant-only inquiries.
-- price_text is informational only; this migration creates no payment state.

CREATE TABLE IF NOT EXISTS marketplace_listings (
    id BIGSERIAL PRIMARY KEY,
    kind VARCHAR(20) NOT NULL CHECK (kind IN ('service', 'demand', 'talent')),
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    canonical_asset_id BIGINT REFERENCES capability_assets(id) ON DELETE RESTRICT,
    title VARCHAR(160) NOT NULL,
    summary VARCHAR(500) NOT NULL,
    category VARCHAR(80) NOT NULL,
    price_text VARCHAR(120) NOT NULL DEFAULT '',
    delivery_text VARCHAR(240) NOT NULL DEFAULT '',
    tags JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(tags) = 'array'),
    status VARCHAR(20) NOT NULL DEFAULT 'published'
        CHECK (status IN ('published', 'archived', 'taken_down')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION enforce_marketplace_owned_asset()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.canonical_asset_id IS NOT NULL AND NOT EXISTS (
        SELECT 1
        FROM capability_assets a
        WHERE a.id = NEW.canonical_asset_id
          AND a.user_id = NEW.owner_user_id
          AND a.deleted_at IS NULL
    ) THEN
        RAISE EXCEPTION 'marketplace canonical asset must belong to listing owner'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_marketplace_owned_asset ON marketplace_listings;
CREATE TRIGGER trg_marketplace_owned_asset
BEFORE INSERT OR UPDATE OF canonical_asset_id, owner_user_id ON marketplace_listings
FOR EACH ROW EXECUTE FUNCTION enforce_marketplace_owned_asset();

CREATE INDEX IF NOT EXISTS idx_marketplace_listings_public
    ON marketplace_listings (id DESC) WHERE status = 'published';
CREATE INDEX IF NOT EXISTS idx_marketplace_listings_owner
    ON marketplace_listings (owner_user_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_marketplace_listings_kind_category
    ON marketplace_listings (kind, category, id DESC) WHERE status = 'published';
CREATE INDEX IF NOT EXISTS idx_marketplace_listings_tags
    ON marketplace_listings USING GIN (tags);

CREATE TABLE IF NOT EXISTS marketplace_inquiries (
    id BIGSERIAL PRIMARY KEY,
    listing_id BIGINT NOT NULL REFERENCES marketplace_listings(id) ON DELETE RESTRICT,
    initiator_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    listing_owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_message_at TIMESTAMPTZ,
    UNIQUE (listing_id, initiator_user_id),
    CHECK (initiator_user_id <> listing_owner_user_id)
);

CREATE INDEX IF NOT EXISTS idx_marketplace_inquiries_initiator
    ON marketplace_inquiries (initiator_user_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_marketplace_inquiries_owner
    ON marketplace_inquiries (listing_owner_user_id, id DESC);

CREATE TABLE IF NOT EXISTS marketplace_inquiry_messages (
    id BIGSERIAL PRIMARY KEY,
    inquiry_id BIGINT NOT NULL REFERENCES marketplace_inquiries(id) ON DELETE CASCADE,
    sender_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_message_id VARCHAR(100) NOT NULL,
    body TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 4000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (inquiry_id, sender_user_id, client_message_id)
);

CREATE INDEX IF NOT EXISTS idx_marketplace_messages_inquiry
    ON marketplace_inquiry_messages (inquiry_id, id DESC);

