import { describe, expect, it } from 'vitest'
import type { SharedPool } from '@/features/bizdecipher/api/bizdecipher'
import {
  sharedPoolCompatibilitySummary,
  sharedPoolOpsBucket,
} from '@/features/bizdecipher/views/admin/sharedPoolGovernanceView'

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
    last_probe_at: '2026-09-13T00:00:00Z',
    last_probe_success: true,
    ...overrides,
  }
}

describe('shared pool governance presentation', () => {
  it('prioritizes missing supply and pricing before runtime health', () => {
    expect(sharedPoolOpsBucket(pool({ models: [] }))).toBe('supply')
    expect(sharedPoolOpsBucket(pool({
      model_configs: [{
        model_name: 'gpt-test',
        provider: 'openai',
        rate_multiplier: 1,
        five_hour_protection_percent: 0,
        seven_day_protection_percent: 0,
        max_concurrency: 1,
        pricing: null,
      }],
    }))).toBe('pricing')
  })

  it('classifies probe and community signals without assuming readiness from missing prices', () => {
    expect(sharedPoolOpsBucket(pool({ last_probe_at: undefined }))).toBe('probe')
    expect(sharedPoolOpsBucket(pool({ complaint_count: 1 }))).toBe('community')
    expect(sharedPoolOpsBucket(pool({ complaint_count: 0 }))).toBe('pricing')
    expect(sharedPoolOpsBucket(pool({ lifecycle_state: 'archived' }))).toBe('archived')
    expect(sharedPoolOpsBucket(pool({ governance_status: 'banned' }))).toBe('attention')
    expect(sharedPoolOpsBucket(pool({ status: 'offline' }))).toBe('attention')
  })

  it('summarizes discovered, priced and probe states honestly', () => {
    expect(sharedPoolCompatibilitySummary(pool({ model_configs: [] }))).toMatchObject({
      discovered: 1,
      priced: 0,
      probe: 'passed',
      label: '模型 1 · 报价待核',
    })
    expect(sharedPoolCompatibilitySummary(pool({ models: [], model_configs: [] }))).toMatchObject({
      discovered: 0,
      label: '待发现模型',
    })
    expect(sharedPoolCompatibilitySummary(pool({ last_probe_success: false }))).toMatchObject({
      probe: 'failed',
      label: '最近池级检测失败',
    })
  })

  it('does not expand one pool probe into proof for every model or ignore required gates', () => {
    expect(sharedPoolCompatibilitySummary(pool({
      models: ['a', 'a', 'b'], last_probe_gate_required: true, last_probe_gate_passed: true,
    }))).toMatchObject({ discovered: 2, probe: 'passed', label: '模型 2 · 报价待核' })
    expect(sharedPoolCompatibilitySummary(pool({
      last_probe_gate_required: true, last_probe_gate_passed: false,
    })).probe).toBe('failed')
    expect(sharedPoolCompatibilitySummary(pool({ last_probe_at: undefined })).probe).toBe('pending')
    expect(sharedPoolCompatibilitySummary(pool({ last_probe_at: 'invalid' })).probe).toBe('pending')
  })

  it('excludes explicitly closed models and counts the union of model names', () => {
    const config = { provider: 'openai', rate_multiplier: 1, five_hour_protection_percent: 0, seven_day_protection_percent: 0, max_concurrency: 1 }
    expect(sharedPoolCompatibilitySummary(pool({
      models: ['a', 'b'], model_configs: [
        { ...config, model_name: 'a', enabled: false },
        { ...config, model_name: 'c', enabled: true },
      ],
    })).discovered).toBe(2)
    expect(sharedPoolOpsBucket(pool({
      models: ['a'], model_configs: [{ ...config, model_name: 'a', model_open: false }],
    }))).toBe('supply')
  })
})
