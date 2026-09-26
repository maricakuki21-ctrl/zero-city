-- Expand daily fortune collectible cards into the Zero City season-one rarity ladder.
ALTER TABLE checkin_collectible_cards
  DROP CONSTRAINT IF EXISTS checkin_collectible_cards_rarity_check;

ALTER TABLE checkin_collectible_cards
  ADD CONSTRAINT checkin_collectible_cards_rarity_check
  CHECK (rarity IN ('common', 'good', 'rare', 'epic', 'legendary', 'mythic'));
