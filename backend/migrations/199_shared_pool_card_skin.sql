-- 199_shared_pool_card_skin.sql
-- 为共享池添加收藏卡底色字段
-- 池主可以选择自己持有的收藏卡作为共享池卡片的视觉皮肤
-- 皮肤效果：在市场卡片上以低透明度/模糊方式渲染卡面作为背景光晕

-- 字段说明：
--   card_skin_key:    收藏卡的 card_key（对应 checkin_collectible_card_types.card_key）
--   card_skin_rarity: 收藏卡的稀有度（与 checkin_collectible_card_types.rarity 对应）
--
-- 约束：
--   card_skin_key 设置时，对应卡必须存在于该用户的 checkin_collectible_cards 中
--   此约束由 service 层校验，不做 FK（checkin_collectible_cards 允许有多张同 key）

ALTER TABLE shared_pools
  ADD COLUMN IF NOT EXISTS card_skin_key    TEXT    DEFAULT NULL,
  ADD COLUMN IF NOT EXISTS card_skin_rarity TEXT    DEFAULT NULL;

COMMENT ON COLUMN shared_pools.card_skin_key    IS '池主设置的收藏卡底色 key，来自 checkin_collectible_card_types.card_key';
COMMENT ON COLUMN shared_pools.card_skin_rarity IS '收藏卡稀有度，用于前端确定图片路径和光晕颜色';
