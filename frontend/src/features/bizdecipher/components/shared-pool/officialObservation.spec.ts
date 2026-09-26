import { describe, expect, it } from 'vitest'
import type { UserMonitorView } from '@/api/channelMonitor'
import { officialDiscoveryState, observationSamples, observedModelNames, resolveOfficialObservation } from './officialObservation'

const group = { id: 2, name: 'Official', platform: 'openai' as const }
const monitor: UserMonitorView = {
  id: 7, name: 'probe', provider: 'openai', group_name: 'Official', primary_model: 'model-a',
  primary_status: 'operational', primary_latency_ms: 100, primary_ping_latency_ms: null,
  availability_7d: 100, extra_models: [], timeline: [],
}
describe('official resource observations', () => {
  const now = Date.parse('2026-09-26T12:00:00Z')
  const failures = [3, 2, 1].map(minutes => ({
    status: 'failed' as const, latency_ms: null, ping_latency_ms: null,
    checked_at: new Date(now - minutes * 60_000).toISOString(),
  }))
  it('hides sustained recent failures, not a single fluctuation or unknown evidence', () => {
    expect(officialDiscoveryState('active', { ...monitor, primary_status: 'failed', timeline: failures }, now).hidden).toBe(true)
    expect(officialDiscoveryState('active', { ...monitor, timeline: failures.slice(-1) }, now).hidden).toBe(false)
    expect(officialDiscoveryState('active', undefined, now).hidden).toBe(false)
    expect(officialDiscoveryState('active', { ...monitor, timeline: failures }, now + 3600_000).hidden).toBe(false)
    expect(officialDiscoveryState('active', { ...monitor, timeline: failures.map(p => ({ ...p, status: 'error' })) }, now).hidden).toBe(false)
  })
  it('keeps usable alternative models and restores recovered groups', () => {
    expect(officialDiscoveryState('active', { ...monitor, timeline: failures, extra_models: [{ model: 'other', status: 'operational', latency_ms: 20 }] }, now).hidden).toBe(false)
    const recovered = [...failures, { ...failures[0], status: 'operational' as const, checked_at: new Date(now).toISOString() }]
    expect(officialDiscoveryState('active', { ...monitor, timeline: recovered }, now)).toEqual({ hidden: false, label: '检测模型可用' })
    expect(officialDiscoveryState('inactive', monitor, now).hidden).toBe(true)
    expect(officialDiscoveryState('active', { ...monitor, observation_mode: 'unavailable' }, now).hidden).toBe(true)
  })
  it('handles the production fifteen-minute probe interval between refreshes', () => {
    const timeline = [38, 23, 8].map(minutes => ({
      ...failures[0], checked_at: new Date(now - minutes * 60_000).toISOString(),
    }))
    expect(officialDiscoveryState('active', { ...monitor, timeline }, now).hidden).toBe(true)
  })
  it('matches only a unique group and monitor with the same provider', () => {
    expect(resolveOfficialObservation(group, [group], [monitor])).toBe(monitor)
    expect(resolveOfficialObservation(group, [group, { ...group, id: 3 }], [monitor])).toBeUndefined()
    expect(resolveOfficialObservation(group, [group], [monitor, { ...monitor, id: 8 }])).toBeUndefined()
    expect(resolveOfficialObservation(group, [group], [{ ...monitor, provider: 'gemini' }])).toBeUndefined()
  })
  it('requires the exact synthetic group identity and never fabricates samples', () => {
    const synthetic = { ...monitor, id: -2, synthetic: true }
    expect(resolveOfficialObservation(group, [group], [synthetic])).toBe(synthetic)
    expect(resolveOfficialObservation(group, [group], [{ ...synthetic, id: -3 }])).toBeUndefined()
    expect(observationSamples(synthetic)).toEqual(Array(24).fill(null))
  })
  it('sorts history and leaves unknown slots gray', () => {
    const sample = { status: 'failed' as const, latency_ms: null, ping_latency_ms: null, checked_at: '2026-09-13T01:00:00Z' }
    const data = { ...monitor, timeline: [sample, { ...sample, checked_at: 'invalid' }, { ...sample, checked_at: '2026-09-12T01:00:00Z' }] }
    const values = observationSamples(data)
    expect(values).toHaveLength(24)
    expect(values.filter(Boolean)).toHaveLength(2)
    expect(values[23]).toEqual(sample)
  })
  it('shows real account-test samples for a native group without a duplicate monitor', () => {
    const point = { status: 'operational' as const, latency_ms: 150, ping_latency_ms: null, checked_at: '2026-09-20T12:00:00Z' }
    expect(observationSamples({ ...monitor, synthetic: true, observation_mode: 'active', timeline: [point] }).filter(Boolean)).toEqual([point])
    expect(observationSamples({ ...monitor, synthetic: true, observation_mode: 'pending', timeline: [point] }).filter(Boolean)).toEqual([])
  })
  it('deduplicates model names without claiming they were verified', () => {
    expect(observedModelNames({ ...monitor, extra_models: [{ model: 'model-a', status: '', latency_ms: null }, { model: 'model-b', status: '', latency_ms: null }] })).toEqual(['model-a', 'model-b'])
  })
})
