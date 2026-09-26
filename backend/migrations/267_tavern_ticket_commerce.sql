-- Existing floating display prices are deliberately not converted into charges.
ALTER TABLE tavern_scripts ADD COLUMN entry_price_usd NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK(entry_price_usd>=0);
ALTER TABLE tavern_rooms ADD COLUMN ticket_price_usd NUMERIC(20,8) CHECK(ticket_price_usd>0);
ALTER TABLE tavern_rooms ADD COLUMN ticket_author_id BIGINT REFERENCES users(id);
ALTER TABLE tavern_rooms ADD CONSTRAINT tavern_room_ticket_snapshot CHECK ((ticket_price_usd IS NULL AND ticket_author_id IS NULL) OR (ticket_price_usd IS NOT NULL AND ticket_author_id IS NOT NULL));
CREATE TABLE tavern_commerce_policy (
 id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK(id),
 enabled BOOLEAN NOT NULL DEFAULT FALSE,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO tavern_commerce_policy(id) VALUES(TRUE);
CREATE TABLE tavern_commerce_audit (
 id BIGSERIAL PRIMARY KEY,
 actor_user_id BIGINT NOT NULL REFERENCES users(id),
 action VARCHAR(32) NOT NULL,
 detail JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE tavern_tickets (
 id BIGSERIAL PRIMARY KEY,
 room_id BIGINT NOT NULL REFERENCES tavern_rooms(id),
 script_id BIGINT NOT NULL REFERENCES tavern_scripts(id),
 buyer_user_id BIGINT NOT NULL REFERENCES users(id),
 author_user_id BIGINT NOT NULL REFERENCES users(id),
 operation_id VARCHAR(100) NOT NULL,
 title TEXT NOT NULL,
 amount NUMERIC(20,8) NOT NULL CHECK(amount>0),
 status VARCHAR(12) NOT NULL DEFAULT 'held' CHECK(status IN ('held','released','refunded')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 released_at TIMESTAMPTZ,
 refunded_at TIMESTAMPTZ,
 refund_reason TEXT NOT NULL DEFAULT '',
 UNIQUE(buyer_user_id,operation_id)
);
CREATE INDEX tavern_tickets_room_status ON tavern_tickets(room_id,status);
CREATE UNIQUE INDEX tavern_tickets_active_player ON tavern_tickets(room_id,buyer_user_id) WHERE status IN ('held','released');
