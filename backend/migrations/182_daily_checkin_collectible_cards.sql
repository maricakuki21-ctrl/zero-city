-- Daily fortune draw records and collectible rare cards.
CREATE SEQUENCE IF NOT EXISTS checkin_collectible_cards_serial_seq START 1001;

CREATE TABLE IF NOT EXISTS checkin_collectible_cards (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  checkin_id BIGINT NOT NULL REFERENCES daily_checkins(id) ON DELETE CASCADE,
  card_key VARCHAR(80) NOT NULL,
  rarity VARCHAR(24) NOT NULL,
  source_type VARCHAR(40) NOT NULL DEFAULT 'daily_fortune',
  source_label VARCHAR(40) NOT NULL DEFAULT 'free',
  serial_no BIGINT NOT NULL DEFAULT nextval('checkin_collectible_cards_serial_seq'),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (checkin_id),
  UNIQUE (serial_no),
  CONSTRAINT checkin_collectible_cards_rarity_check CHECK (rarity IN ('rare', 'epic', 'legendary', 'mythic'))
);

CREATE INDEX IF NOT EXISTS idx_checkin_collectible_cards_user_created
  ON checkin_collectible_cards(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_checkin_collectible_cards_key_rarity
  ON checkin_collectible_cards(card_key, rarity, created_at DESC);
