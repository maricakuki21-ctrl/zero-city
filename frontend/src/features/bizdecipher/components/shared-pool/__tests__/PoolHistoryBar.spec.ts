import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import PoolHistoryBar from '../PoolHistoryBar.vue'

type IntersectionCallback = (entries: Array<{ isIntersecting: boolean }>) => void

const mocks = vi.hoisted(() => ({
  listHistories: vi.fn(),
  intersectionCallback: undefined as IntersectionCallback | undefined,
}))

vi.mock('@vueuse/core', () => ({
  useIntersectionObserver: (_target: unknown, callback: IntersectionCallback) => {
    mocks.intersectionCallback = callback
    return { stop: vi.fn() }
  },
}))

vi.mock('@/features/bizdecipher/api/bizdecipher', () => ({
  listSharedPoolProbeHistories: mocks.listHistories,
}))

function setVisible(isIntersecting: boolean): void {
  mocks.intersectionCallback?.([{ isIntersecting }])
}

describe('PoolHistoryBar', () => {
  afterEach(() => vi.restoreAllMocks())
  it('shows mixed-model denominators without pretending the last model covers all tests', async () => {
    mocks.listHistories.mockResolvedValueOnce([
      { id: 1, model_name: 'text-a', success: true, latency_ms: 100, checked_at: '2026-09-23T12:00:00Z' },
      { id: 2, model_name: 'image-b', success: false, latency_ms: 200, checked_at: '2026-09-23T12:01:00Z' },
    ])
    const wrapper = mount(PoolHistoryBar, { props: { poolId: 8199 } })
    setVisible(true); await flushPromises()
    expect(wrapper.text()).toContain('2 个模型合计')
    expect(wrapper.text()).toContain('1/2 次通过 · 50.0%')
    expect(wrapper.text()).toContain('不是全天可用率')
    await wrapper.get('select[aria-label="筛选检测模型"]').setValue('image-b')
    expect(wrapper.findAll('li')).toHaveLength(1)
    expect(wrapper.get('li').text()).toContain('image-b')
    wrapper.unmount()
  })
  it('emits stale evidence without a currently healthy result', async () => {
    vi.spyOn(Date, 'now').mockReturnValue(Date.parse('2026-09-24T12:00:00Z'))
    mocks.listHistories.mockResolvedValueOnce([
      { id: 1, model_name: 'text-a', success: true, latency_ms: 100, checked_at: '2026-09-23T12:00:00Z' },
    ])
    const wrapper = mount(PoolHistoryBar, { props: { poolId: 8120 } })
    setVisible(true); await flushPromises()
    expect(wrapper.text()).toContain('历史样本')
    expect(wrapper.emitted('observed')?.at(-1)?.[0]).toMatchObject({ state: 'stale', latest: null })
    wrapper.unmount()
  })
  it('warns about a small sample and rejects future or invalid timestamps', async () => {
    vi.spyOn(Date, 'now').mockReturnValue(Date.parse('2026-09-24T12:00:00Z'))
    mocks.listHistories.mockResolvedValueOnce([
      { id: 1, model_name: 'text-a', success: true, latency_ms: 100, checked_at: '2026-09-24T11:59:00Z' },
      { id: 2, model_name: 'text-a', success: true, latency_ms: 100, checked_at: '2026-09-25T12:00:00Z' },
      { id: 3, model_name: 'text-a', success: true, latency_ms: 100, checked_at: 'invalid' },
    ])
    const wrapper = mount(PoolHistoryBar, { props: { poolId: 8121 } })
    setVisible(true); await flushPromises()
    expect(wrapper.text()).toContain('样本较少')
    expect(wrapper.text()).toContain('1/1 次通过')
    expect(wrapper.findAll('.pool-history-bars .healthy')).toHaveLength(1)
    wrapper.unmount()
  })
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.intersectionCallback = undefined
  })

  it('renders 24 unknown slots and waits for visibility before loading', async () => {
    const wrapper = mount(PoolHistoryBar, { props: { poolId: 8101 } })

    expect(wrapper.findAll('.pool-history-bars .unknown')).toHaveLength(24)
    expect(mocks.listHistories).not.toHaveBeenCalled()

    mocks.listHistories.mockResolvedValueOnce([])
    setVisible(true)
    await flushPromises()

    expect(mocks.listHistories).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('待检测')
  })

  it('caches a successful empty result across visibility changes and remounts', async () => {
    mocks.listHistories.mockResolvedValueOnce([])
    const first = mount(PoolHistoryBar, { props: { poolId: 8102 } })
    setVisible(true)
    await flushPromises()
    setVisible(false)
    setVisible(true)
    await flushPromises()
    first.unmount()

    mount(PoolHistoryBar, { props: { poolId: 8102 } })
    setVisible(true)
    await flushPromises()

    expect(mocks.listHistories).toHaveBeenCalledTimes(1)
  })

  it('deduplicates a pending request when visibility toggles', async () => {
    let finishRequest: ((value: []) => void) | undefined
    mocks.listHistories.mockImplementationOnce(() => new Promise<[]>(resolve => { finishRequest = resolve }))
    mount(PoolHistoryBar, { props: { poolId: 8103 } })

    setVisible(true)
    setVisible(false)
    setVisible(true)
    await Promise.resolve()

    expect(mocks.listHistories).toHaveBeenCalledTimes(1)
    finishRequest?.([])
    await flushPromises()
  })

  it('offers an explicit retry after a failed request', async () => {
    mocks.listHistories
      .mockRejectedValueOnce(new Error('probe history unavailable'))
      .mockResolvedValueOnce([])
    const wrapper = mount(PoolHistoryBar, { props: { poolId: 8104 } })

    setVisible(true)
    await flushPromises()
    expect(wrapper.get('[data-testid="pool-history-retry"]').text()).toBe('重试')
    expect(wrapper.text()).toContain('检测记录加载失败')
    expect(wrapper.attributes('aria-label')).toContain('灰色不代表检测通过')
    expect(wrapper.emitted('observed')?.at(-1)?.[0]).toMatchObject({ state: 'error', latest: null, passed: 0 })

    await wrapper.get('[data-testid="pool-history-retry"]').trigger('click')
    await flushPromises()

    expect(mocks.listHistories).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-testid="pool-history-retry"]').exists()).toBe(false)
  })

  it('refreshes an expired cache on visibility rather than keeping old samples forever', async () => {
    const now = vi.spyOn(Date, 'now').mockReturnValue(100_000)
    mocks.listHistories.mockResolvedValue([])
    const wrapper = mount(PoolHistoryBar, { props: { poolId: 8110 } })
    setVisible(true)
    await flushPromises()
    setVisible(false)
    await flushPromises()
    now.mockReturnValue(161_000)
    setVisible(true)
    await flushPromises()
    expect(mocks.listHistories).toHaveBeenCalledTimes(2)
    wrapper.unmount()
    now.mockRestore()
  })
})
