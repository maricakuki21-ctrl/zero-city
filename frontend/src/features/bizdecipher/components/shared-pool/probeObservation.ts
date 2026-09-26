import type { SharedPoolProbeHistory } from '@/features/bizdecipher/api/bizdecipher'

// Display freshness, not an uptime SLA. Native checks normally run every 15
// minutes; a result over an hour old must not advertise current availability.
export const PROBE_FRESHNESS_MS = 60 * 60 * 1000
export const PROBE_SMALL_SAMPLE_COUNT = 5

export type ProbeObservation = {
  count: number
  passed: number
  latest: boolean | null
  latestAt: string | null
  state: 'empty' | 'fresh' | 'stale' | 'error'
  smallSample: boolean
}

export function summarizeProbeObservations(
  records: readonly SharedPoolProbeHistory[],
  now: number,
  failed = false,
): ProbeObservation {
  if (failed) {
    return { count: 0, passed: 0, latest: null, latestAt: null, state: 'error', smallSample: false }
  }
  const valid = records.filter(row => {
    const checkedAt = Date.parse(row.checked_at || row.created_at)
    return typeof row.success === 'boolean' && Number.isFinite(checkedAt) && checkedAt <= now
  })
  const latest = valid.reduce<SharedPoolProbeHistory | undefined>((result, row) =>
    !result || Date.parse(row.checked_at || row.created_at) > Date.parse(result.checked_at || result.created_at)
      ? row : result, undefined)
  const latestAt = latest ? latest.checked_at || latest.created_at : null
  const stale = latestAt !== null && now - Date.parse(latestAt) > PROBE_FRESHNESS_MS
  return {
    count: valid.length,
    passed: valid.filter(row => row.success).length,
    latest: latest && !stale ? latest.success : null,
    latestAt,
    state: !latest ? 'empty' : stale ? 'stale' : 'fresh',
    smallSample: valid.length > 0 && valid.length < PROBE_SMALL_SAMPLE_COUNT,
  }
}
