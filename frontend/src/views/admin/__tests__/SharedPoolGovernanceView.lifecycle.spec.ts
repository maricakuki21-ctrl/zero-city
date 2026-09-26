import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

import type { SharedPool } from '@/api/bizdecipher'
import {
  filterSharedPoolsByLifecycle,
  hasSharedPoolArchiveContract,
  isExplicitlyArchivedPool,
  needsSharedPoolAttention,
} from '../sharedPoolGovernanceView'

const viewSource = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../../../features/bizdecipher/views/admin/SharedPoolGovernanceView.vue'),
  'utf8',
)

function pool(overrides: Partial<SharedPool> = {}): SharedPool {
  return {
    id: 1,
    name: '测试池',
    description: '',
    owner_label: '池主',
    tier: 'standard',
    status: 'healthy',
    listed: true,
    models: ['gpt-test'],
    rate_multiplier: 1,
    max_users: 10,
    current_users: 1,
    min_balance_admission: 0,
    hourly_seat_fee: 0,
    today_availability: 100,
    seven_day_availability: 100,
    avg_latency_ms: 100,
    ...overrides,
  }
}

describe('shared-pool lifecycle classification', () => {
  it('recognizes archive state only from explicit backend evidence', () => {
    expect(isExplicitlyArchivedPool(pool({ status: 'offline' }))).toBe(false)
    expect(isExplicitlyArchivedPool(pool({ lifecycle_state: 'archived' }))).toBe(true)
    expect(isExplicitlyArchivedPool(pool({ archived_at: '2026-07-19T00:00:00Z' }))).toBe(true)
  })

  it('routes operational risks to attention without calling them archived', () => {
    expect(needsSharedPoolAttention(pool({ listed: false }))).toBe(true)
    expect(needsSharedPoolAttention(pool({ status: 'maintenance' }))).toBe(true)
    expect(needsSharedPoolAttention(pool({ governance_status: 'watch' }))).toBe(true)
    expect(needsSharedPoolAttention(pool({ consecutive_probe_failures: 3 }))).toBe(true)
    expect(needsSharedPoolAttention(pool({ consecutive_probe_failures: 2 }))).toBe(false)
    expect(needsSharedPoolAttention(pool({ listed: undefined }))).toBe(false)
  })

  it('keeps current, attention, archived and all views mutually understandable', () => {
    const values = [
      pool({ id: 1 }),
      pool({ id: 2, listed: false }),
      pool({ id: 3, lifecycle_state: 'archived', listed: false }),
    ]

    expect(filterSharedPoolsByLifecycle(values, 'current').map(({ id }) => id)).toEqual([1])
    expect(filterSharedPoolsByLifecycle(values, 'attention').map(({ id }) => id)).toEqual([2])
    expect(filterSharedPoolsByLifecycle(values, 'archived').map(({ id }) => id)).toEqual([3])
    expect(filterSharedPoolsByLifecycle(values, 'all')).toHaveLength(3)
  })

  it('detects an archive contract even when the backend returns null fields', () => {
    expect(hasSharedPoolArchiveContract([pool()])).toBe(false)
    expect(hasSharedPoolArchiveContract([pool({ archived_at: null })])).toBe(true)
  })
})

describe('SharedPoolGovernanceView lifecycle contract', () => {
  it('separates existing-pool bulk fees from the new-pool default in both admin views', () => {
    expect(viewSource).toContain('只修改当前筛选到的 {{ pools.length }} 个已有池，不影响新池默认。')
    expect(viewSource).toContain(`router.push('/admin/settings')`)
    expect(viewSource).toContain('前往系统设置')
  })

  it('defaults to current pools and explains that offline pools are never guessed as history', () => {
    expect(viewSource).toContain("ref<SharedPoolLifecycleView>('current')")
    expect(viewSource).toContain('后端归档契约尚未接入；系统不会把离线池猜成历史池。')
    expect(viewSource).toContain('离线、维护或未上架只进入“待处理”，不会被猜成历史归档。')
  })

  it('restores an archived pool only after confirmation and keeps it offline as a draft', () => {
    expect(viewSource).toContain('恢复为草稿')
    expect(viewSource).toContain('window.confirm')
    expect(viewSource).toContain('不会自动上架，也不会自动启用任何旧 Key')
    expect(viewSource).toContain('adminRestoreSharedPool(pool.id')
    expect(viewSource).toContain('await loadPools()')
  })
})
