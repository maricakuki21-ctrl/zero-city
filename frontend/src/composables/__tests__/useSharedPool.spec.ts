import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  listSharedPools: vi.fn(),
  listSharedPoolSummaries: vi.fn(),
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard: vi.fn() }),
}))

vi.mock('@/api/bizdecipher', () => ({
  createSharedPoolAccessKey: vi.fn(),
  joinSharedPool: vi.fn(),
  leaveSharedPool: vi.fn(),
  likeSharedPool: vi.fn(),
  unlikeSharedPool: vi.fn(),
  listModelCatalog: vi.fn(),
  listMySeats: vi.fn(),
  listMySharedPoolAccessKeys: vi.fn(),
  listSharedPoolProbeHistories: vi.fn(),
  listSharedPools: mocks.listSharedPools,
  reportSharedPool: vi.fn(),
}))

vi.mock('@/api/community', () => ({
  communityAPI: { listSharedPoolSummaries: mocks.listSharedPoolSummaries },
}))

import { useSharedPool } from '@/composables/useSharedPool'
import type { SharedMarketPoolProjection, SharedMarketProductFacts } from '@/api/bizdecipher'

function productProjection(poolId: number, name: string): SharedMarketPoolProjection {
  const evidence = {
    source: 'bizdecipher.pool' as const, observed_at: '2026-09-14T00:00:00Z',
    freshness: 'recent' as const, confidence: 'high' as const,
  }
  const fact = <T>(value: T) => ({ value, evidence })
  const product: SharedMarketProductFacts = {
    name: fact(name), description: fact(''), owner_label: fact('Canonical owner'),
    membership: { current_users: fact(2), maximum_users: fact(10) },
    pricing: { rate_multiplier: fact('1'), minimum_balance: fact('0'), hourly_seat_fee: fact('0'), hourly_usage_waiver: fact('0') },
    community: { likes: fact(0), complaints: fact(0), discussions: fact(0) },
  }
  return { pool_id: poolId, product, official_service_status: fact('unknown'), stale: false, errors: [] }
}

const firstSuccessfulView = {
  pools: [{
    id: 7,
    name: 'Sub2 投影池',
    owner_label: 'Sub2',
    tier: 'standard',
    status: 'healthy',
    models: ['gpt-4.1-mini'],
    today_availability: 98,
    seven_day_availability: 97,
    rate_multiplier: 1,
    current_users: 2,
    max_users: 10,
    min_balance_admission: 0,
    hourly_seat_fee: 0,
    avg_latency_ms: 120,
    total_calls: 100,
    successful_calls: 100,
    account_summary: { success_rate: 42.5, model_coverage: [] },
  }],
  total: 1,
  online: 1,
  limited: 0,
  avg_availability: 98,
}

describe('useSharedPool', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.listSharedPoolSummaries.mockResolvedValue({ items: [] })
  })

  it('retains the last successful Sub2 projection and stats after a failed refresh', async () => {
    mocks.listSharedPools
      .mockResolvedValueOnce(firstSuccessfulView)
      .mockRejectedValueOnce({ message: 'Request failed with status code 503' })

    const sharedPool = useSharedPool()
    await sharedPool.loadPools()
    const retainedPools = [...sharedPool.pools.value]
    const retainedStats = { ...sharedPool.serverStats.value }

    await sharedPool.loadPools()

    expect(sharedPool.pools.value).toEqual(retainedPools)
    expect(sharedPool.serverStats.value).toEqual(retainedStats)
    expect(sharedPool.loadError.value).toBe('共享池市场加载失败，请重试。')
    expect(sharedPool.pools.value[0]?.successRate).toBe('42.5')
  })

  it('does not let a slow previous search replace newer results', async () => {
    let resolvePrevious: ((value: typeof firstSuccessfulView) => void) | undefined
    mocks.listSharedPools.mockImplementationOnce(() => new Promise(resolve => { resolvePrevious = resolve }))
      .mockResolvedValueOnce({ ...firstSuccessfulView, pools: [{ ...firstSuccessfulView.pools[0], name: 'New search' }] })
    const shared = useSharedPool()
    const previous = shared.loadPools()
    shared.keyword.value = 'New'
    await shared.loadPools()
    resolvePrevious?.(firstSuccessfulView)
    await previous
    expect(shared.pools.value[0]?.name).toBe('New search')
    expect(shared.loading.value).toBe(false)
  })

  it('joins optional canonical projections by pool id rather than response order', async () => {
    mocks.listSharedPools.mockResolvedValue({
      ...firstSuccessfulView,
      projections: [productProjection(99, 'Another pool'), productProjection(7, 'Canonical visible name')],
    })
    const shared = useSharedPool()
    await shared.loadPools()
    expect(shared.pools.value[0]).toMatchObject({
      id: 7, name: 'Canonical visible name', status: 'unknown', models: [], successRate: '-',
    })
  })

  it.each(['success', 'failure'])('ignores stale community summaries after a newer search (%s)', async outcome => {
    let finishPrevious: (() => void) | undefined
    mocks.listSharedPools.mockResolvedValue(firstSuccessfulView)
    mocks.listSharedPoolSummaries
      .mockImplementationOnce(() => new Promise((resolve, reject) => {
        finishPrevious = () => outcome === 'success'
          ? resolve({ items: [{ pool_id: 7, total_posts: 1 }] })
          : reject(new Error('Old request failed'))
      }))
      .mockResolvedValueOnce({ items: [{ pool_id: 7, total_posts: 9 }] })
    const shared = useSharedPool()
    const previous = shared.loadPools()
    await vi.waitFor(() => expect(mocks.listSharedPoolSummaries).toHaveBeenCalledTimes(1))
    await shared.loadPools()
    finishPrevious?.()
    await previous
    expect(shared.poolCommunitySummaries[7]?.total_posts).toBe(9)
  })
})
