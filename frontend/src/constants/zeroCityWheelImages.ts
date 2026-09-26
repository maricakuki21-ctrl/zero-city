export type ZeroCityWheelType = 'free' | 'credit' | 'balance'
export type ZeroCityWheelRarity = 'common' | 'good' | 'rare' | 'epic' | 'diamond' | 'rainbow'
export type ZeroCityWheelThumbnailWidth = 160 | 320 | 640

const WHEEL_CARD_BASE = '/assets/zero-point-city/cards'

export function zeroCityWheelImagePath(type: ZeroCityWheelType, rarity: ZeroCityWheelRarity, key: string) {
  return `${WHEEL_CARD_BASE}/wheel/${type}/${rarity}/${key}.png`
}

export function zeroCityWheelThumbnailPath(
  type: ZeroCityWheelType,
  rarity: ZeroCityWheelRarity,
  key: string,
  width: ZeroCityWheelThumbnailWidth
) {
  return `${WHEEL_CARD_BASE}/wheel-thumbs/${width}/${type}/${rarity}/${key}.jpg`
}

export function zeroCityWheelImageSrcSet(type: ZeroCityWheelType, rarity: ZeroCityWheelRarity, key: string) {
  return [
    `${zeroCityWheelThumbnailPath(type, rarity, key, 160)} 160w`,
    `${zeroCityWheelThumbnailPath(type, rarity, key, 320)} 320w`,
    `${zeroCityWheelThumbnailPath(type, rarity, key, 640)} 640w`,
    `${zeroCityWheelImagePath(type, rarity, key)} 1024w`
  ].join(', ')
}
