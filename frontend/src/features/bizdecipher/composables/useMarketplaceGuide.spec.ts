import { ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { useMarketplaceGuide } from './useMarketplaceGuide'

const api = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: api }))

describe('marketplace guide state', () => {
  it('persists intent to server without creating a listing', async () => {
    api.get.mockResolvedValue({ data: { version: 1, intent: '', completed_at: null } })
    api.put.mockResolvedValue({ data: { version: 1, intent: 'hire', completed_at: '2026-09-23T12:00:00Z' } })
    let guide!: ReturnType<typeof useMarketplaceGuide>
    const wrapper = mount({ setup() { guide = useMarketplaceGuide(ref(9001)); return {} }, template: '<div />' })
    await flushPromises()
    expect(guide.pending.value).toBe(true)
    expect(await guide.complete('hire')).toBe(true)
    expect(api.put).toHaveBeenCalledWith('/biz/market/onboarding', { intent: 'hire' })
    expect(guide.pending.value).toBe(false)
    wrapper.unmount()
  })
  it('isolates late requests and does not acknowledge failed writes', async () => {
    let finish!: (value: unknown) => void
    api.get.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
      .mockResolvedValueOnce({ data: { version: 1, completed_at: null } })
    const id = ref(9002)
    let guide!: ReturnType<typeof useMarketplaceGuide>
    const wrapper = mount({ setup() { guide = useMarketplaceGuide(id); return {} }, template: '<div />' })
    id.value = 9003
    await flushPromises()
    finish({ data: { version: 1, completed_at: '2026-09-23T12:00:00Z' } })
    await flushPromises()
    expect(guide.pending.value).toBe(true)
    api.put.mockRejectedValueOnce(new Error('offline'))
    expect(await guide.complete('browse')).toBe(false)
    expect(guide.pending.value).toBe(true)
    expect(guide.state.value.error).toContain('未保存')
    wrapper.unmount()
  })
})
