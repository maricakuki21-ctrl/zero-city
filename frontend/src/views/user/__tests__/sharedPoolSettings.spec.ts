import { describe, expect, it, vi } from 'vitest'
import type { SharedPool } from '@/features/bizdecipher/api/bizdecipher'
import {
  SHARED_POOL_REVERIFY_MESSAGE,
  buildSharedPoolSettingsPayload,
  persistSharedPoolSettings,
  type SharedPoolSettingsDraft,
} from '@/features/bizdecipher/views/user/sharedPoolSettings'

const pool: SharedPool = {
  id: 2002,
  config_version: 17,
  name: 'Original pool',
  description: 'Original description',
  avatar_url: 'https://assets.invalid/pool.png',
  status_note: 'Available',
  disabled_reason: '',
  owner_label: 'Owner',
  tier: 'standard',
  status: 'healthy',
  listed: true,
  models: ['custom-model'],
  model_configs: [{ provider: 'custom', model_name: 'custom-model', rate_multiplier: 1, model_open: true }],
  rate_multiplier: 1,
  max_users: 20,
  current_users: 3,
  min_balance_admission: 1,
  hourly_seat_fee: 0.1,
  hourly_min_usage_waiver: 1,
  today_availability: 99,
  seven_day_availability: 98,
  avg_latency_ms: 120,
  upstream_base_url: 'https://upstream.invalid/v1',
  proxy_id: 77,
  proxy_url: 'http://proxy.invalid:8080',
  proxy_region: 'test',
  proxy_status: 'active',
  account_concurrency: 2,
  user_concurrency: 1,
  account_mode_enabled: false,
  oauth_provider: 'openai',
  verification_mode: 'full_check',
  verification_exemption_reason: '',
}

const draft: SharedPoolSettingsDraft = {
  name: 'Renamed pool',
  description: 'Updated description',
  upstream_base_url: 'https://upstream.invalid/v1',
  upstream_api_key: '',
  account_mode_enabled: false,
  rate_multiplier: 1.25,
  max_users: 30,
  min_balance_admission: 2,
  hourly_seat_fee: 0.2,
  hourly_min_usage_waiver: 1.5,
  account_concurrency: 3,
  user_concurrency: 2,
  verification_mode: 'full_check',
  verification_exemption_reason: '',
}

describe('shared pool settings save', () => {
  it('builds a minimal PATCH without resending stale runtime fields or a hidden API key', () => {
    const payload = buildSharedPoolSettingsPayload(pool, draft)

    expect(payload).toEqual({
      expected_config_version: 17,
      name: 'Renamed pool',
      description: 'Updated description',
      rate_multiplier: 1.25,
      sync_model_rates: true,
      max_users: 30,
      min_balance_admission: 2,
      hourly_seat_fee: 0.2,
      hourly_min_usage_waiver: 1.5,
      account_concurrency: 3,
      user_concurrency: 2,
    })
    expect(payload).not.toHaveProperty('upstream_api_key')
    for (const key of [
      'avatar_url', 'status_note', 'disabled_reason', 'models', 'model_configs',
      'proxy_id', 'proxy_url', 'proxy_region', 'proxy_status', 'oauth_provider',
      'probe_model', 'listed', 'status',
      'upstream_base_url', 'account_mode_enabled', 'verification_mode',
      'verification_exemption_reason',
    ]) {
      expect(payload).not.toHaveProperty(key)
    }
  })

  it('sends only the loaded version when no setting changed', () => {
    const payload = buildSharedPoolSettingsPayload(pool, {
      name: pool.name,
      description: pool.description,
      upstream_base_url: pool.upstream_base_url || '',
      upstream_api_key: '',
      account_mode_enabled: Boolean(pool.account_mode_enabled),
      rate_multiplier: pool.rate_multiplier,
      max_users: pool.max_users,
      min_balance_admission: pool.min_balance_admission,
      hourly_seat_fee: pool.hourly_seat_fee,
      hourly_min_usage_waiver: pool.hourly_min_usage_waiver || 0,
      account_concurrency: pool.account_concurrency || 1,
      user_concurrency: pool.user_concurrency || 1,
      verification_mode: 'full_check',
      verification_exemption_reason: '',
    })

    expect(payload).toEqual({ expected_config_version: 17 })
  })

  it('keeps a 0.0001 multiplier and tells the backend to sync model rates', () => {
    const payload = buildSharedPoolSettingsPayload(pool, {
      ...draft,
      rate_multiplier: 0.0001,
    })

    expect(payload.rate_multiplier).toBe(0.0001)
    expect(payload.sync_model_rates).toBe(true)
  })

  it.each([0, 0.00009, Number.POSITIVE_INFINITY, Number.NaN])('rejects an invalid multiplier: %s', (rateMultiplier) => {
    expect(() => buildSharedPoolSettingsPayload(pool, {
      ...draft,
      rate_multiplier: rateMultiplier,
    })).toThrow('不能小于 0.0001')
  })

  it('sends a replacement API key only when the owner entered one', () => {
    const payload = buildSharedPoolSettingsPayload(pool, {
      ...draft,
      upstream_api_key: '  new-secret  ',
    })

    expect(payload.upstream_api_key).toBe('new-secret')
  })

  it('reports automatic unlisting after a gate configuration change', async () => {
    const updatePool = vi.fn().mockResolvedValue({ ...pool, listed: false })
    const showError = vi.fn()

    const result = await persistSharedPoolSettings({
      pool,
      draft,
      updatePool,
      formatError: vi.fn(),
      showError,
      formatEffectiveAt: (value) => value,
    })

    expect(result.message).toBe(SHARED_POOL_REVERIFY_MESSAGE)
    expect(updatePool).toHaveBeenCalledWith(pool.id, expect.not.objectContaining({ listed: expect.anything() }))
    expect(showError).not.toHaveBeenCalled()
  })

  it('keeps the normal settlement-effective message when the pool stays listed', async () => {
    const effectiveAt = '2026-07-20T12:00:00Z'
    const result = await persistSharedPoolSettings({
      pool,
      draft,
      updatePool: vi.fn().mockResolvedValue({
        ...pool,
        listed: true,
        pending_settlement_rule_effective_from: effectiveAt,
      }),
      formatError: vi.fn(),
      showError: vi.fn(),
      formatEffectiveAt: () => '2026/07/20 20:00',
    })

    expect(result.message).toBe('设置已保存，将于 2026/07/20 20:00 生效')
  })

  it('sends a global error and preserves the user-facing failure message', async () => {
    const originalError = new Error('request failed')
    const showError = vi.fn()
    const formatError = vi.fn().mockReturnValue('保存失败：上游请求超时')

    const promise = persistSharedPoolSettings({
      pool,
      draft,
      updatePool: vi.fn().mockRejectedValue(originalError),
      formatError,
      showError,
      formatEffectiveAt: (value) => value,
    })

    await expect(promise).rejects.toMatchObject({
      name: 'SharedPoolSettingsSaveError',
      message: '保存失败：上游请求超时',
      originalError,
    })
    expect(formatError).toHaveBeenCalledWith(originalError, '保存失败，请重试')
    expect(showError).toHaveBeenCalledWith('保存失败：上游请求超时')
  })
})
