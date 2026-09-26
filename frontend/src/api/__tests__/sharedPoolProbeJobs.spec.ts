import { afterEach, describe, expect, it, vi } from 'vitest'
import { apiClient } from '@/api/client'
import {
  getSharedPoolProbeJob,
  isSharedPoolProbeJobTerminal,
  probeSharedPoolUpstream,
  waitForSharedPoolProbeJob,
  type SharedPoolProbeJob,
} from '@/api/sharedPoolProbeJobs'

const originalAdapter = apiClient.defaults.adapter

function probeJob(overrides: Partial<SharedPoolProbeJob> = {}): SharedPoolProbeJob {
  return {
    id: 'probe-job-1',
    operation_id: 'probe-operation-1',
    pool_id: 168,
    owner_id: 9,
    model_name: 'gpt-4o-mini',
    upstream_model_name: 'gpt-4o-mini',
    probe_type: 'manual',
    check_level: 'full',
    config_version: 3,
    status: 'queued',
    attempt: 0,
    max_attempts: 2,
    created_at: '2026-07-19T00:00:00Z',
    updated_at: '2026-07-19T00:00:00Z',
    ...overrides,
  }
}

afterEach(() => {
  apiClient.defaults.adapter = originalAdapter
  vi.restoreAllMocks()
})

describe('shared pool probe jobs', () => {
  it('queues a durable job with a matching body and header operation id', async () => {
    const adapter = vi.fn().mockImplementation(async (config) => ({
      status: 202,
      statusText: 'Accepted',
      headers: {},
      config,
      data: { code: 0, data: probeJob() },
    }))
    apiClient.defaults.adapter = adapter

    const job = await probeSharedPoolUpstream({
      pool_id: 168,
      probe_type: 'manual',
      operation_id: 'probe-operation-1',
    })

    const config = adapter.mock.calls[0][0]
    expect(job.status).toBe('queued')
    expect(config.url).toBe('/biz/upstream/probe')
    expect(config.headers.get('Idempotency-Key')).toBe('probe-operation-1')
    expect(JSON.parse(config.data).operation_id).toBe('probe-operation-1')
  })

  it('loads a terminal job and returns it without another polling delay', async () => {
    const completed = probeJob({
      status: 'succeeded',
      result: {
        ok: true,
        model: 'gpt-4o-mini',
        models: ['gpt-4o-mini'],
        message: 'ok',
        checked_at: '2026-07-19T00:01:00Z',
        gate_passed: true,
        full_check_passed: 15,
        full_check_total: 15,
        full_check_score: 100,
      },
    })
    const adapter = vi.fn().mockImplementation(async (config) => ({
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
      data: { code: 0, data: completed },
    }))
    apiClient.defaults.adapter = adapter

    await expect(getSharedPoolProbeJob('probe-job-1')).resolves.toEqual(completed)
    await expect(waitForSharedPoolProbeJob('probe-job-1', {
      pollIntervalMs: 250,
      timeoutMs: 1000,
    })).resolves.toEqual(completed)
    expect(adapter).toHaveBeenCalledTimes(2)
    expect(adapter.mock.calls[0][0].url).toBe('/biz/upstream/probe-jobs/probe-job-1')
    expect(isSharedPoolProbeJobTerminal(completed)).toBe(true)
  })

  it('stops local polling without cancelling the durable server job', async () => {
    const adapter = vi.fn()
    apiClient.defaults.adapter = adapter
    const controller = new AbortController()
    controller.abort()

    await expect(waitForSharedPoolProbeJob('probe-job-1', {
      signal: controller.signal,
    })).rejects.toMatchObject({ name: 'AbortError' })
    expect(adapter).not.toHaveBeenCalled()
  })
})
