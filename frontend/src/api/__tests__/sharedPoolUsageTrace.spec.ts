import { afterEach, describe, expect, it, vi } from 'vitest'
import { apiClient } from '@/api/client'
import { listMySharedPoolUsageTraces } from '@/api/sharedPoolUsageTrace'

const originalAdapter = apiClient.defaults.adapter

afterEach(() => {
  apiClient.defaults.adapter = originalAdapter
  vi.restoreAllMocks()
})

describe('shared pool usage traces', () => {
  it('requests the authenticated user page with a stable cursor', async () => {
    const adapter = vi.fn().mockImplementation(async (config) => ({
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
      data: {
        code: 0,
        data: {
          items: [{
            id: 9,
            pool_id: 3,
            request_id: 'req-safe',
            pool_name_snapshot: '小白友好池',
            model: 'gpt-test',
            endpoint: '/v1/responses',
            account_alias: '共享线路 ABC123',
            status: 'succeeded',
            settlement_outcome: 'settled',
            upstream_started: true,
            usage_observed: true,
            input_tokens: 12,
            output_tokens: 4,
            cache_read_tokens: 8,
            cache_creation_tokens: 0,
            retry_count: 0,
            created_at: '2026-07-19T00:00:00Z',
          }],
          next_before_id: 9,
          has_more: true,
        },
      },
    }))
    apiClient.defaults.adapter = adapter

    const page = await listMySharedPoolUsageTraces({ beforeId: 15, limit: 20 })

    expect(page.items[0].account_alias).toBe('共享线路 ABC123')
    expect(adapter).toHaveBeenCalledTimes(1)
    expect(adapter.mock.calls[0][0].url).toBe('/biz/shared-pool/usage-traces')
    expect(adapter.mock.calls[0][0].params).toEqual(expect.objectContaining({ before_id: 15, limit: 20 }))
  })
})
