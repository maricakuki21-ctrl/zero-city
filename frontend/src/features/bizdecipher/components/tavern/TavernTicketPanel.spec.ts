import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import TavernTicketPanel from './TavernTicketPanel.vue'
const api = vi.hoisted(() => ({ quote: vi.fn(), buy: vi.fn(), refund: vi.fn() }))
const auth = reactive({ user: { id: 7 } })
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('../../api/tavernCommerce', () => ({ tavernCommerceAPI: api }))
vi.mock('../../api/bizdecipher', () => ({ joinTavernRoom: vi.fn() }))
const q = { room_id: 1, owner_id: 9, author_user_id: 11, status: 'lobby', title: 'Story', price: '10.12345678', currency: 'USD', enabled: true }
describe('TavernTicketPanel', () => {
  beforeEach(() => {
    vi.resetAllMocks(); auth.user.id = 7
    api.quote.mockResolvedValue({ ...q })
    api.buy.mockResolvedValue({ id: 1, room_id: 1, amount: q.price, status: 'held' })
  })
  it('requires consent and reuses a ticket operation after uncertain response', async () => {
    api.buy.mockRejectedValueOnce(new Error('timeout'))
    const w = mount(TavernTicketPanel, { props: { roomId: 1 } })
    await flushPromises()
    expect(w.get('button').attributes('disabled')).toBeDefined()
    await w.get('input[type=checkbox]').setValue(true)
    await w.get('form').trigger('submit'); await flushPromises()
    const first = api.buy.mock.calls[0]
    expect(first.slice(0, 2)).toEqual([1, '10.12345678'])
    expect(first[2]).toMatch(/^tavern_[a-f0-9]{40}$/)
    await w.get('form').trigger('submit'); await flushPromises()
    expect(api.buy.mock.calls[1]).toEqual(first)
    expect(w.text()).toContain('已购票')
    w.unmount()
  })
  it('blocks new purchase while the gate is closed but leaves refunds available', async () => {
    api.quote.mockResolvedValue({ ...q, enabled: false })
    const w = mount(TavernTicketPanel, { props: { roomId: 1 } })
    await flushPromises(); expect(w.get('button').attributes('disabled')).toBeDefined()
    await w.get('form').trigger('submit'); expect(api.buy).not.toHaveBeenCalled()
    w.unmount()
    api.quote.mockResolvedValue({ ...q, enabled: false, ticket: { id: 1, status: 'held' } })
    api.refund.mockResolvedValue({ id: 1, status: 'refunded' })
    const refund = mount(TavernTicketPanel, { props: { roomId: 1 } })
    await flushPromises(); await refund.get('button').trigger('click'); await flushPromises()
    expect(api.refund).toHaveBeenCalledWith(1, 1)
    expect(refund.text()).toContain('已退款')
    refund.unmount()
  })
  it('hides refund after start and discards a prior account response', async () => {
    api.quote.mockResolvedValueOnce({ ...q, status: 'running', ticket: { status: 'released' } })
    const running = mount(TavernTicketPanel, { props: { roomId: 1 } })
    await flushPromises(); expect(running.find('button').exists()).toBe(false); running.unmount()
    let resolve!: (value: unknown) => void
    api.quote.mockReturnValueOnce(new Promise(r => { resolve = r })).mockResolvedValueOnce({ ...q, title: 'New account' })
    const w = mount(TavernTicketPanel, { props: { roomId: 1 } })
    auth.user.id = 20; await flushPromises()
    resolve({ ...q, title: 'Old private data', ticket: { status: 'held' } }); await flushPromises()
    expect(w.text()).not.toContain('Old private data')
    expect(w.text()).toContain('New account')
    w.unmount()
  })
})
