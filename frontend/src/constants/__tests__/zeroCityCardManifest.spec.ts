import { describe, expect, it } from 'vitest'
import {
  ZERO_CITY_CARD_MANIFEST_VERSION,
  stableCardIndex,
  zeroCityCardDisplayName,
  zeroCityCardImageSrcSet,
  zeroCityCardManifestEntries
} from '../zeroCityCardManifest'

describe('zeroCityCardManifest', () => {
  it('publishes a versioned manifest with unique card codes', () => {
    const entries = zeroCityCardManifestEntries()
    const codes = entries.map(entry => entry.definition.code)

    expect(ZERO_CITY_CARD_MANIFEST_VERSION).toMatch(/^\d{4}-\d{2}-\d{2}\.\d+$/)
    expect(entries).toHaveLength(82)
    expect(new Set(codes).size).toBe(codes.length)
  })

  it('uses the formal Chinese collectible names and an explicit unknown fallback', () => {
    expect(zeroCityCardDisplayName('low_battery_sprite')).toBe('低电量小人')
    expect(zeroCityCardDisplayName('instant_noodle_prophet')).toBe('泡面预言家')
    expect(zeroCityCardDisplayName('missing-card')).toBe('未知收藏卡（missing-card）')
  })

  it('maps the same server result seed to the same bounded card index', () => {
    const seed = '2026-07-19:balance:5:88:2680:123'
    const first = stableCardIndex(seed, 12)

    expect(stableCardIndex(seed, 12)).toBe(first)
    expect(first).toBeGreaterThanOrEqual(0)
    expect(first).toBeLessThan(12)
    expect(stableCardIndex(seed, 0)).toBe(0)
  })

  it('serves real responsive thumbnails before the full resolution PNG', () => {
    const srcset = zeroCityCardImageSrcSet({
      card_key: 'low_battery_sprite',
      rarity: 'common'
    })

    expect(srcset).toContain('/collectible-thumbs/160/common/low_battery_sprite.jpg 160w')
    expect(srcset).toContain('/collectible-thumbs/320/common/low_battery_sprite.jpg 320w')
    expect(srcset).toContain('/collectible-thumbs/640/common/low_battery_sprite.jpg 640w')
    expect(srcset).toContain('/collectible/common/low_battery_sprite.png 1024w')
  })
})
