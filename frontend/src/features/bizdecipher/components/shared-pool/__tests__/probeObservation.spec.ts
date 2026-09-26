import { describe, expect, it } from 'vitest'
import type { SharedPoolProbeHistory } from '@/features/bizdecipher/api/bizdecipher'
import { PROBE_FRESHNESS_MS, summarizeProbeObservations } from '../probeObservation'

const now = Date.parse('2026-09-24T12:00:00Z')
function sample(ago: number, success = true): SharedPoolProbeHistory {
  return { id: ago, success, checked_at: new Date(now - ago).toISOString() } as SharedPoolProbeHistory
}

describe('probe observation evidence', () => {
  it('distinguishes no evidence from a failed request for evidence', () => {
    expect(summarizeProbeObservations([], now)).toMatchObject({ count: 0, latest: null, state: 'empty' })
    expect(summarizeProbeObservations([sample(1000)], now, true)).toMatchObject({ count: 0, passed: 0, latest: null, state: 'error' })
  })
  it('keeps one successful sample small rather than claiming stable availability', () => {
    expect(summarizeProbeObservations([sample(1000)], now)).toMatchObject({
      count: 1, passed: 1, latest: true, state: 'fresh', smallSample: true,
    })
  })
  it('does not present an old success as current health', () => {
    expect(summarizeProbeObservations([sample(PROBE_FRESHNESS_MS + 1)], now)).toMatchObject({
      count: 1, passed: 1, latest: null, state: 'stale',
    })
  })
  it('uses the newest valid timestamp and preserves the denominator', () => {
    expect(summarizeProbeObservations([sample(1000, false), sample(2000)], now)).toMatchObject({
      count: 2, passed: 1, latest: false, state: 'fresh',
    })
  })
  it('ignores future dates and malformed samples rather than painting them green', () => {
    expect(summarizeProbeObservations([
      sample(-1000), { ...sample(1000), checked_at: 'invalid' },
      { ...sample(1000), success: null } as unknown as SharedPoolProbeHistory,
    ], now)).toMatchObject({ count: 0, passed: 0, latest: null, state: 'empty' })
  })
})
