import { describe, expect, it } from 'vitest'
import {
  buildZeroCityRankingRows,
  defaultZeroCityRankingConfig,
  normalizeRankingConfig,
  parseZeroCityRankingConfig,
  type ZeroCityRankingPost,
} from './zeroCityRankings'

const recent = new Date(Date.now() - 2 * 24 * 60 * 60 * 1000).toISOString()
const old = new Date(Date.now() - 45 * 24 * 60 * 60 * 1000).toISOString()

const posts: ZeroCityRankingPost[] = [
  {
    id: 1,
    userId: 7,
    author: '近期香港',
    card: '近期香港',
    createdAt: recent,
    kind: 'support',
    scenario: 'incident_support',
    catches: 2,
    replies: 3,
    views: 20,
    evidenceCount: 1,
    official: false,
    accepted: true,
  },
  {
    id: 2,
    userId: 8,
    author: '旧记录',
    card: '旧记录',
    createdAt: old,
    kind: 'support',
    scenario: 'incident_support',
    catches: 30,
    replies: 30,
    views: 300,
    evidenceCount: 5,
    official: true,
    accepted: true,
  },
]

describe('zero city rankings', () => {
  it('normalizes configurable order and limit', () => {
    const value = normalizeRankingConfig({
      version: 2,
      items: [{ ...defaultZeroCityRankingConfig.items[1], order: 20, limit: 99 }, { ...defaultZeroCityRankingConfig.items[2], order: 10 }],
    })
    expect(value.items.map((item) => item.key)).toEqual(['answers', 'contribution'])
    expect(value.items[1]?.limit).toBe(10)
  })

  it('filters rows by the configured time window before scoring', () => {
    const definition = { ...defaultZeroCityRankingConfig.items[1], limit: 5 }
    const rows = buildZeroCityRankingRows(definition, posts)
    expect(rows).toHaveLength(1)
    expect(rows[0]?.label).toBe('近期香港')
  })

  it('does not synthesize effective spending ranks from community posts', () => {
    const rows = buildZeroCityRankingRows(defaultZeroCityRankingConfig.items[0], posts)
    expect(rows).toEqual([])
  })

  it('migrates legacy reward fields without losing configured identity or enabling disabled boards', () => {
    const config = parseZeroCityRankingConfig({
      version: 7,
      items: [
        {
          ...defaultZeroCityRankingConfig.items[1],
          title: '自定义贡献榜',
          order: 3,
          limit: 9,
          enabled: false,
          weeklyRule: undefined,
          monthlyRule: undefined,
          weeklyReward: '第1名：9999积分',
          monthlyReward: '随机传说卡',
        },
        { ...defaultZeroCityRankingConfig.items[2], order: 1 },
      ],
    })

    expect(config.version).toBe(7)
    expect(config.items.map((item) => item.title)).toEqual(['答疑互助', '自定义贡献榜'])
    expect(config.items[1]).toMatchObject({ order: 3, limit: 9, enabled: false })
    expect(config.items[1]?.weeklyRule).toBe(defaultZeroCityRankingConfig.items[1]?.weeklyRule)
    expect(JSON.stringify(config)).not.toContain('9999积分')
    expect(JSON.stringify(config)).not.toContain('随机传说卡')
  })
})
