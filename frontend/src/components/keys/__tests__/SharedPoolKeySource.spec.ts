import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SharedPoolKeySource from '../SharedPoolKeySource.vue'
const { seats } = vi.hoisted(() => ({ seats: vi.fn() }))
vi.mock('@/features/bizdecipher/api/bizdecipher', () => ({ listMySeats: seats }))
const render = () => mount(SharedPoolKeySource, { props: { modelValue: null }, global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
beforeEach(() => seats.mockReset())
describe('shared pool key source', () => {
  it('only selects an active seat without joining or charging a pool', async () => {
    seats.mockResolvedValue([{ id: 1, pool_id: 17, pool_name: '可用池', status: 'active' }, { id: 2, pool_id: 18, pool_name: '已退出池', status: 'released' }])
    const view = render()
    await flushPromises()
    expect(view.text()).toContain('可用池')
    expect(view.text()).not.toContain('已退出池')
    await view.get('input[type=radio]').setValue()
    expect(view.emitted('update:modelValue')?.at(-1)).toEqual([17])
    await view.get('input[type=search]').setValue('不匹配')
    expect(view.text()).toContain('没有匹配')
    view.unmount()
  })
  it('keeps a failed load distinct from an empty membership list and retries', async () => {
    seats.mockRejectedValueOnce(new Error('network failure')).mockResolvedValueOnce([])
    const view = render()
    await flushPromises()
    expect(view.find('[role=alert]').exists()).toBe(true)
    expect(view.text()).not.toContain('还没有已加入')
    await view.get('button[aria-label="刷新共享池"]').trigger('click')
    await flushPromises()
    expect(view.text()).toContain('还没有已加入的共享池')
    expect(view.emitted('update:modelValue')?.at(-1)).toEqual([null])
    view.unmount()
  })
})
