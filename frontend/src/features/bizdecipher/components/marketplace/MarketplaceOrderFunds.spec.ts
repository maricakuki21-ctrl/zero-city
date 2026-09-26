import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import MarketplaceOrderFunds from './MarketplaceOrderFunds.vue'
import type { MarketplaceOrder } from '../../api/marketplace'

const api = vi.hoisted(() => ({ get: vi.fn(), policy: vi.fn(), price: vi.fn(), pay: vi.fn() }))
const identity = reactive({ user: { id: 7 } })
vi.mock('@/stores/auth', () => ({ useAuthStore: () => identity }))
vi.mock('../../api/marketplaceFunds', () => ({ marketplaceFundsAPI: api }))
const order = { id: 1, status: 'quoted', viewer_role: 'buyer' } as MarketplaceOrder
const funds = { order_id: 1, amount: '10.12345678', currency: 'USD', status: 'unpaid' }
describe('MarketplaceOrderFunds', () => {
  beforeEach(() => {
    vi.resetAllMocks(); identity.user.id = 7
    api.get.mockResolvedValue(funds); api.policy.mockResolvedValue({ enabled: true, currency: 'USD' })
    api.pay.mockResolvedValue({ ...funds, status: 'held' })
  })
  it('requires explicit USD confirmation and reuses the payment token after an uncertain failure', async () => {
    api.pay.mockRejectedValueOnce(new Error('timeout'))
    const wrapper = mount(MarketplaceOrderFunds, { props: { order } })
    await flushPromises()
    expect(wrapper.get('button').attributes('disabled')).toBeDefined()
    await wrapper.get('input[type=checkbox]').setValue(true)
    await wrapper.get('form').trigger('submit'); await flushPromises()
    const first = api.pay.mock.calls[0]
    expect(first[0]).toBe(1); expect(first[1]).toBe('10.12345678'); expect(first[2]).toMatch(/^market_[a-f0-9]{40}$/)
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.pay.mock.calls[1]).toEqual(first)
    expect(wrapper.text()).toContain('待交付与验收')
    wrapper.unmount()
  })
  it('does not submit payments while the platform gate is closed', async () => {
    api.policy.mockResolvedValue({ enabled: false, currency: 'USD' })
    const wrapper = mount(MarketplaceOrderFunds, { props: { order } })
    await flushPromises()
    expect(wrapper.get('input[type=checkbox]').attributes('disabled')).toBeDefined()
    await wrapper.get('form').trigger('submit')
    expect(api.pay).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('sets a separate numeric USD quote without parsing legacy text', async () => {
    api.get.mockResolvedValue(null); api.price.mockResolvedValue(funds)
    const wrapper = mount(MarketplaceOrderFunds, { props: { order: { ...order, viewer_role: 'seller', amount_text: '900 CNY' } } })
    await flushPromises()
    await wrapper.get('input').setValue('10.12345678')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.price).toHaveBeenCalledWith(1, '10.12345678')
    expect(api.pay).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('discards a prior account response and clears private financial state', async () => {
    let resolve!: (value: unknown) => void
    api.get.mockReturnValueOnce(new Promise(r => { resolve = r })).mockResolvedValueOnce(null)
    const wrapper = mount(MarketplaceOrderFunds, { props: { order } })
    identity.user.id = 42
    await flushPromises()
    resolve(funds); await flushPromises()
    expect(wrapper.text()).not.toContain('10.12345678')
    expect(wrapper.find('input[type=checkbox]').exists()).toBe(false)
    wrapper.unmount()
  })
})
