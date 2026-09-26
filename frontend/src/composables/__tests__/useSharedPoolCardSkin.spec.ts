import { describe, expect, it } from 'vitest'
import { toVM } from '../useSharedPool'
import type { SharedMarketEvidence, SharedMarketPoolProjection, SharedPool } from '@/api/bizdecipher'

const observed: SharedMarketEvidence = {
  source: 'sub2.usage_log',
  observed_at: '2026-09-02T09:00:00Z',
  freshness: 'recent',
  confidence: 'high',
}

const productObserved: SharedMarketEvidence = { ...observed, source: 'bizdecipher.price_snapshot' }

function fact<T>(value: T, evidence: SharedMarketEvidence = observed) {
  return { value, evidence }
}

describe('shared-pool market view model', () => {
  it('keeps the pool-specific card background returned by the API', () => {
    const vm = toVM({
      id: 271,
      name: '虾蹬2',
      description: '',
      owner_label: '虾蹬',
      tier: 'Standard',
      status: 'healthy',
      models: [],
      rate_multiplier: 0.02,
      max_users: 20,
      current_users: 11,
      min_balance_admission: 0,
      hourly_seat_fee: 0,
      today_availability: 99,
      seven_day_availability: 99,
      avg_latency_ms: 100,
      card_skin_key: 'pool-card-background',
      card_skin_rarity: 'legendary',
    } satisfies SharedPool)

    expect(vm.cardSkinKey).toBe('pool-card-background')
    expect(vm.cardSkinRarity).toBe('legendary')
    expect(vm).toMatchObject({ status: 'unknown', models: [], successRate: '-', throughputRPM: '-' })
    expect(Number.isNaN(vm.todayAvailability)).toBe(true)
    expect(Number.isNaN(vm.latency)).toBe(true)
  })

  it('uses canonical projections without calculating runtime authority from legacy counters', () => {
    const pool = {
      id: 42, name: 'Legacy', description: '', owner_label: 'Legacy owner', tier: 'Standard',
      status: 'healthy', models: ['legacy-model'], rate_multiplier: 9, max_users: 9,
      current_users: 9, min_balance_admission: 9, hourly_seat_fee: 9,
      today_availability: 9, seven_day_availability: 9, avg_latency_ms: 999,
      total_calls: 100, successful_calls: 99,
    } satisfies SharedPool
    const projection = {
      pool_id: 42,
      product: {
        name: fact('Canonical pool', productObserved), description: fact('', productObserved),
        owner_label: fact('Canonical owner', productObserved),
        membership: { current_users: fact(2, productObserved), maximum_users: fact(4, productObserved) },
        pricing: {
          rate_multiplier: fact('0.0001', productObserved), minimum_balance: fact('0', productObserved),
          hourly_seat_fee: fact('0.0100', productObserved), hourly_usage_waiver: fact('0', productObserved),
        },
        community: { likes: fact(3, productObserved), complaints: fact(1, productObserved), discussions: fact(2, productObserved) },
      },
      runtime: {
        group_id: fact('group-91'), account_ids: fact(['account-7']),
        models: fact([{ name: 'gpt-5', executable: true }, { name: 'listed-only', executable: false }]),
        group_availability: fact({ state: 'available', available: 1, total: 1 }),
        account_availability: fact({ state: 'available', available: 1, total: 1 }),
        health: fact({ state: 'limited', confidence: 'high' as const }),
        today_availability_percent: fact('98.75'), seven_day_availability_percent: fact('97.50'),
        latency_ms: fact('11.50'), throughput_rpm: fact('4.25'), success_rate_percent: fact('12.3400'),
        canonical_usage: fact({ requests_succeeded: '10', requests_failed: '71', input_tokens: '100', output_tokens: '20' }),
      },
      official_service_status: fact('operational', { ...observed, source: 'official.service_status' }),
      stale: false,
      errors: [],
    } satisfies SharedMarketPoolProjection

    const vm = toVM(pool, projection)

    expect(vm).toMatchObject({
      name: 'Canonical pool', owner: 'Canonical owner', status: 'limited', models: ['gpt-5'],
      successRate: '12.3400', throughputRPM: '4.25', rateDecimal: '0.0001', latency: 11.5,
      sourceFreshness: 'recent', sourceConfidence: 'high',
    })

    const stale = toVM(pool, { ...projection, stale: true })
    expect(stale.status).toBe('unknown')
    expect(stale.sourceFreshness).toBe('last_verified')

    const unavailableRuntime = toVM(pool, { ...projection, runtime: undefined })
    expect(unavailableRuntime).toMatchObject({
      name: 'Canonical pool', status: 'unknown', models: [], successRate: '-', throughputRPM: '-',
    })
    expect(Number.isNaN(unavailableRuntime.latency)).toBe(true)

    const differentPool = toVM(pool, { ...projection, pool_id: 99 })
    expect(differentPool.name).toBe('Legacy')
    expect(differentPool.status).toBe('unknown')
  })
})
