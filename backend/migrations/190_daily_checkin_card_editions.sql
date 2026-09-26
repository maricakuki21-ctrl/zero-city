-- Add per-card edition numbering and supply caps for Zero City collectible cards.
ALTER TABLE checkin_collectible_cards
  ADD COLUMN IF NOT EXISTS edition_no BIGINT,
  ADD COLUMN IF NOT EXISTS edition_supply BIGINT;

CREATE TABLE IF NOT EXISTS checkin_collectible_card_editions (
  card_key VARCHAR(80) PRIMARY KEY,
  rarity VARCHAR(24) NOT NULL,
  max_supply BIGINT NOT NULL CHECK (max_supply > 0),
  issued_count BIGINT NOT NULL DEFAULT 0 CHECK (issued_count >= 0 AND issued_count <= max_supply),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

WITH edition_seed(card_key, rarity, max_supply) AS (
  VALUES
    ('low_battery_sprite', 'common', 9999),
    ('early_failer', 'common', 9999),
    ('sofa_observer', 'common', 9999),
    ('micro_disconnect_worker', 'common', 9999),
    ('monday_escapee', 'common', 9999),
    ('warm_water_guard', 'common', 9999),
    ('slow_boot_machine', 'common', 9999),
    ('tiny_step_champion', 'common', 9999),
    ('self_rescue_store', 'common', 9999),
    ('retry_button_keeper', 'common', 9999),
    ('calendar_slacker', 'common', 9999),
    ('unread_message_monk', 'common', 9999),
    ('blanket_fort_guardian', 'common', 9999),
    ('snack_budget_scholar', 'common', 9999),
    ('fog_window_wiper', 'common', 9999),
    ('almost_ok_actor', 'common', 9999),
    ('battery_anxiety_meter', 'common', 9999),
    ('side_quest_picker', 'common', 9999),
    ('tiny_rain_shelter', 'common', 9999),
    ('lost_focus_fisher', 'common', 9999),
    ('browser_tab_shepherd', 'common', 9999),
    ('midnight_cookie_auditor', 'common', 9999),
    ('instant_noodle_prophet', 'common', 9999),
    ('pocket_sun_carrier', 'common', 9999),
    ('humble_loading_bar', 'common', 9999),
    ('small_cloud_tenant', 'common', 9999),
    ('chair_rooted_guard', 'common', 9999),
    ('pending_reply_sprite', 'common', 9999),
    ('tiny_courage_bean', 'common', 9999),
    ('desktop_dust_archivist', 'common', 9999),
    ('cloud_patch_apprentice', 'good', 3333),
    ('mood_buffer_agent', 'good', 3333),
    ('hope_coupon_keeper', 'good', 3333),
    ('rationality_dog_walker', 'good', 3333),
    ('tiny_luck_runner', 'good', 3333),
    ('anxiety_gardener', 'good', 3333),
    ('failure_recycler', 'good', 3333),
    ('luck_taster', 'good', 3333),
    ('reverse_koi_assistant', 'good', 3333),
    ('expectation_admin', 'good', 3333),
    ('metaphysics_compliance', 'good', 3333),
    ('digital_scavenger', 'good', 3333),
    ('little_profit_philosopher', 'good', 3333),
    ('wallet_listener', 'good', 3333),
    ('surprise_ticket_checker', 'good', 3333),
    ('risk_kitten_keeper', 'good', 3333),
    ('noise_reducer', 'good', 3333),
    ('hope_night_shift', 'good', 3333),
    ('almost_miracle_clerk', 'good', 3333),
    ('soft_deadline_negotiator', 'good', 3333),
    ('low_battery_saint', 'rare', 999),
    ('cloud_patch_worker', 'rare', 999),
    ('late_but_arrived', 'rare', 999),
    ('tiny_luck_clerk', 'rare', 999),
    ('mood_janitor', 'rare', 999),
    ('hope_inventory_keeper', 'rare', 999),
    ('doomed_plan_rescuer', 'rare', 999),
    ('tiny_storm_captain', 'rare', 999),
    ('lucky_bug_keeper', 'rare', 999),
    ('deadline_exorcist', 'rare', 999),
    ('half_awake_oracle', 'rare', 999),
    ('almost_win_archivist', 'rare', 999),
    ('probability_rebel', 'epic', 300),
    ('mirror_lake_admin', 'epic', 300),
    ('blue_hour_operator', 'epic', 300),
    ('black_sun_intern', 'epic', 300),
    ('fate_customer_service', 'epic', 300),
    ('hit_rate_tamer', 'epic', 300),
    ('miracle_tester', 'epic', 300),
    ('prize_pool_diver', 'epic', 300),
    ('numbered_crown_holder', 'legendary', 100),
    ('afterglow_cartographer', 'legendary', 100),
    ('zero_hour_lighthouse_keeper', 'legendary', 100),
    ('hidden_plot_curator', 'legendary', 100),
    ('zero_point_pilot', 'mythic', 30),
    ('terminal_hope_backup', 'mythic', 30)
), numbered_cards AS (
  SELECT
    c.id,
    c.card_key,
    ROW_NUMBER() OVER (PARTITION BY c.card_key ORDER BY c.created_at ASC, c.id ASC) AS edition_no,
    s.max_supply
  FROM checkin_collectible_cards c
  JOIN edition_seed s ON s.card_key = c.card_key
), issued_counts AS (
  SELECT card_key, COUNT(*)::BIGINT AS issued_count
  FROM numbered_cards
  GROUP BY card_key
)
INSERT INTO checkin_collectible_card_editions (card_key, rarity, max_supply, issued_count)
SELECT s.card_key, s.rarity, s.max_supply, COALESCE(i.issued_count, 0)
FROM edition_seed s
LEFT JOIN issued_counts i ON i.card_key = s.card_key
ON CONFLICT (card_key) DO UPDATE
SET rarity = EXCLUDED.rarity,
    max_supply = EXCLUDED.max_supply,
    issued_count = GREATEST(checkin_collectible_card_editions.issued_count, EXCLUDED.issued_count),
    updated_at = NOW();

WITH edition_seed(card_key, rarity, max_supply) AS (
  VALUES
    ('low_battery_sprite', 'common', 9999),
    ('early_failer', 'common', 9999),
    ('sofa_observer', 'common', 9999),
    ('micro_disconnect_worker', 'common', 9999),
    ('monday_escapee', 'common', 9999),
    ('warm_water_guard', 'common', 9999),
    ('slow_boot_machine', 'common', 9999),
    ('tiny_step_champion', 'common', 9999),
    ('self_rescue_store', 'common', 9999),
    ('retry_button_keeper', 'common', 9999),
    ('calendar_slacker', 'common', 9999),
    ('unread_message_monk', 'common', 9999),
    ('blanket_fort_guardian', 'common', 9999),
    ('snack_budget_scholar', 'common', 9999),
    ('fog_window_wiper', 'common', 9999),
    ('almost_ok_actor', 'common', 9999),
    ('battery_anxiety_meter', 'common', 9999),
    ('side_quest_picker', 'common', 9999),
    ('tiny_rain_shelter', 'common', 9999),
    ('lost_focus_fisher', 'common', 9999),
    ('browser_tab_shepherd', 'common', 9999),
    ('midnight_cookie_auditor', 'common', 9999),
    ('instant_noodle_prophet', 'common', 9999),
    ('pocket_sun_carrier', 'common', 9999),
    ('humble_loading_bar', 'common', 9999),
    ('small_cloud_tenant', 'common', 9999),
    ('chair_rooted_guard', 'common', 9999),
    ('pending_reply_sprite', 'common', 9999),
    ('tiny_courage_bean', 'common', 9999),
    ('desktop_dust_archivist', 'common', 9999),
    ('cloud_patch_apprentice', 'good', 3333),
    ('mood_buffer_agent', 'good', 3333),
    ('hope_coupon_keeper', 'good', 3333),
    ('rationality_dog_walker', 'good', 3333),
    ('tiny_luck_runner', 'good', 3333),
    ('anxiety_gardener', 'good', 3333),
    ('failure_recycler', 'good', 3333),
    ('luck_taster', 'good', 3333),
    ('reverse_koi_assistant', 'good', 3333),
    ('expectation_admin', 'good', 3333),
    ('metaphysics_compliance', 'good', 3333),
    ('digital_scavenger', 'good', 3333),
    ('little_profit_philosopher', 'good', 3333),
    ('wallet_listener', 'good', 3333),
    ('surprise_ticket_checker', 'good', 3333),
    ('risk_kitten_keeper', 'good', 3333),
    ('noise_reducer', 'good', 3333),
    ('hope_night_shift', 'good', 3333),
    ('almost_miracle_clerk', 'good', 3333),
    ('soft_deadline_negotiator', 'good', 3333),
    ('low_battery_saint', 'rare', 999),
    ('cloud_patch_worker', 'rare', 999),
    ('late_but_arrived', 'rare', 999),
    ('tiny_luck_clerk', 'rare', 999),
    ('mood_janitor', 'rare', 999),
    ('hope_inventory_keeper', 'rare', 999),
    ('doomed_plan_rescuer', 'rare', 999),
    ('tiny_storm_captain', 'rare', 999),
    ('lucky_bug_keeper', 'rare', 999),
    ('deadline_exorcist', 'rare', 999),
    ('half_awake_oracle', 'rare', 999),
    ('almost_win_archivist', 'rare', 999),
    ('probability_rebel', 'epic', 300),
    ('mirror_lake_admin', 'epic', 300),
    ('blue_hour_operator', 'epic', 300),
    ('black_sun_intern', 'epic', 300),
    ('fate_customer_service', 'epic', 300),
    ('hit_rate_tamer', 'epic', 300),
    ('miracle_tester', 'epic', 300),
    ('prize_pool_diver', 'epic', 300),
    ('numbered_crown_holder', 'legendary', 100),
    ('afterglow_cartographer', 'legendary', 100),
    ('zero_hour_lighthouse_keeper', 'legendary', 100),
    ('hidden_plot_curator', 'legendary', 100),
    ('zero_point_pilot', 'mythic', 30),
    ('terminal_hope_backup', 'mythic', 30)
), numbered_cards AS (
  SELECT
    c.id,
    ROW_NUMBER() OVER (PARTITION BY c.card_key ORDER BY c.created_at ASC, c.id ASC) AS edition_no,
    s.max_supply
  FROM checkin_collectible_cards c
  JOIN edition_seed s ON s.card_key = c.card_key
)
UPDATE checkin_collectible_cards c
SET edition_no = n.edition_no,
    edition_supply = n.max_supply
FROM numbered_cards n
WHERE c.id = n.id
  AND (c.edition_no IS NULL OR c.edition_supply IS NULL);

ALTER TABLE checkin_collectible_cards
  DROP CONSTRAINT IF EXISTS checkin_collectible_cards_edition_positive_check;

ALTER TABLE checkin_collectible_cards
  ADD CONSTRAINT checkin_collectible_cards_edition_positive_check
  CHECK (
    (edition_no IS NULL AND edition_supply IS NULL)
    OR (edition_no > 0 AND edition_supply > 0 AND edition_no <= edition_supply)
  );

CREATE UNIQUE INDEX IF NOT EXISTS idx_checkin_collectible_cards_key_edition
  ON checkin_collectible_cards(card_key, edition_no);

CREATE INDEX IF NOT EXISTS idx_checkin_collectible_card_editions_rarity
  ON checkin_collectible_card_editions(rarity, updated_at DESC);
