import { describe, expect, it } from 'vitest'
import {
  resolveSharedPoolCardPresentation,
  sharedPoolDecision,
  sharedPoolServiceGrade,
} from '@/features/bizdecipher/components/shared-pool/sharedPoolPresentation'

describe('sharedPoolPresentation', () => {
  it('prefers the pool skin and resolves the collectible identity', () => {
    const card = resolveSharedPoolCardPresentation({
      cardKey: 'tiny_step_champion',
      cardRarity: 'common',
      fallbackCardKey: 'zero_hour_lighthouse_keeper',
      fallbackCardRarity: 'legendary',
    })

    expect(card).toMatchObject({
      key: 'tiny_step_champion',
      rarity: 'common',
      badgeLabel: '普通收藏卡',
      name: '小步冠军',
      role: '微小进度裁判',
    })
    expect(card?.imageUrl).toBe('/assets/zero-point-city/cards/collectible/common/tiny_step_champion.png')
  })

  it('falls back to the owner featured card when the pool has no skin', () => {
    const card = resolveSharedPoolCardPresentation({
      fallbackCardKey: 'tiny_step_champion',
      fallbackCardRarity: 'common',
    })

    expect(card?.name).toBe('小步冠军')
  })

  it('does not call an 85 percent pool highly available', () => {
    expect(sharedPoolServiceGrade({
      status: 'healthy',
      todayAvailability: 94.5,
      sevenDayAvailability: 85.23,
      hasEvidence: true,
    })).toMatchObject({ code: 'C', label: 'C级高波动', tone: 'risk' })

    expect(sharedPoolDecision({
      status: 'healthy',
      sevenDayAvailability: 85.23,
      hasEvidence: true,
    }).title).toContain('不建议作为唯一主池')
  })

  it('keeps pools without evidence in observation instead of fabricating a grade', () => {
    expect(sharedPoolServiceGrade({ status: 'healthy', hasEvidence: false })).toMatchObject({
      code: 'OBSERVE',
      label: '数据观察中',
    })
  })
})
