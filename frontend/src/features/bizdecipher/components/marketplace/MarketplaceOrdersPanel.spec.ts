import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import MarketplaceOrdersPanel from './MarketplaceOrdersPanel.vue'
import type { MarketplaceOrder } from '@/features/bizdecipher/api/marketplace'

const mocks = vi.hoisted(() => ({
  api: { listOrders: vi.fn(), getOrder: vi.fn(), applyOrderAction: vi.fn() },
}))
vi.mock('@/features/bizdecipher/api/marketplace', () => ({ marketplaceAPI: mocks.api }))
vi.mock('./MarketplaceOrderFunds.vue', () => ({ default: { template: '<div />' } }))

const order = (overrides: Partial<MarketplaceOrder> = {}): MarketplaceOrder => ({
  id: 51,
  inquiry_id: 31,
  listing_id: 11,
  listing_title: '自动化验收',
  buyer_user_id: 7,
  buyer_display_name: 'alice',
  seller_user_id: 9,
  seller_display_name: 'bob',
  status: 'quoted',
  scope_text: '交付脚本与验收记录',
  amount_text: '100 积分',
  delivery_text: '2 天',
  revision_limit: 1,
  delivery_note: '',
  accept_note: '',
  dispute_note: '',
  cancel_reason: '',
  quoted_at: '2026-09-12T01:00:00Z',
  created_at: '2026-09-12T01:00:00Z',
  updated_at: '2026-09-12T01:00:00Z',
  viewer_role: 'buyer',
  available_actions: ['confirm', 'cancel'],
  events: [],
  reviews: [],
  ...overrides,
})

describe('MarketplaceOrdersPanel', () => {
  it('shows the administrator resolution and reason to participants', async () => {
    const resolved = order({ status: 'confirmed', available_actions: ['dispute'], events: [{ id: 9, order_id: 51, actor_user_id: 42, actor_name: 'Admin', event: 'admin_resolve_dispute', from_status: 'disputed', to_status: 'confirmed', note: 'Revised delivery agreed', created_at: '2026-09-18T00:00:00Z' }] })
    mocks.api.listOrders.mockResolvedValue({ items: [resolved] })
    mocks.api.getOrder.mockResolvedValue(resolved)
    const wrapper = mount(MarketplaceOrdersPanel)
    await flushPromises()
    expect(wrapper.get('[aria-label="争议处理结果"]').text()).toContain('Revised delivery agreed')
    expect(wrapper.text()).toContain('继续履约，重新交付与验收')
  })
  beforeEach(() => {
    for (const fn of Object.values(mocks.api)) fn.mockReset()
    mocks.api.listOrders.mockResolvedValue({ items: [order()], next_cursor: null })
    mocks.api.getOrder.mockResolvedValue(order())
  })

  it('loads participant orders and server-authorized actions', async () => {
    const wrapper = mount(MarketplaceOrdersPanel)
    await flushPromises()

    expect(mocks.api.listOrders).toHaveBeenCalledWith(undefined)
    expect(mocks.api.getOrder).toHaveBeenCalledWith(51)
    expect(wrapper.text()).toContain('我的订单')
    expect(wrapper.text()).toContain('确认合作')
    expect(wrapper.text()).toContain('取消订单')
    expect(wrapper.text()).not.toContain('提交交付')
  })

  it('requires a note for delivery and sends the server action', async () => {
    const quoted = order({ viewer_role: 'seller', available_actions: ['deliver'] })
    const delivered = order({ status: 'delivered', viewer_role: 'seller', delivery_note: '验收包已上传', available_actions: ['dispute'] })
    mocks.api.listOrders.mockResolvedValue({ items: [quoted], next_cursor: null })
    mocks.api.getOrder.mockResolvedValue(quoted)
    mocks.api.applyOrderAction.mockResolvedValue(delivered)
    const wrapper = mount(MarketplaceOrdersPanel)
    await flushPromises()

    await wrapper.get('.order-actions .order-button').trigger('click')
    expect(wrapper.find('.order-action-note').exists()).toBe(true)
    await wrapper.get('.order-action-note textarea').setValue('验收包已上传')
    await wrapper.get('.order-action-note').trigger('submit')
    await flushPromises()

    expect(mocks.api.applyOrderAction).toHaveBeenCalledWith(51, { action: 'deliver', note: '验收包已上传' })
    expect(wrapper.text()).toContain('已记录')
    expect(wrapper.text()).toContain('验收包已上传')
  })

  it('submits rating and review text only when the server exposes review', async () => {
    const reviewable = order({ status: 'accepted', available_actions: ['review'], events: [] })
    const reviewed = order({ status: 'accepted', available_actions: [], reviews: [{ id: 71, order_id: 51, author_user_id: 7, author_name: 'alice', rating: 5, body: '交付清楚', created_at: '2026-09-12T03:00:00Z' }] })
    mocks.api.listOrders.mockResolvedValue({ items: [reviewable], next_cursor: null })
    mocks.api.getOrder.mockResolvedValue(reviewable)
    mocks.api.applyOrderAction.mockResolvedValue(reviewed)
    const wrapper = mount(MarketplaceOrdersPanel)
    await flushPromises()

    await wrapper.get('.order-review select').setValue('5')
    await wrapper.get('.order-review textarea').setValue('交付清楚')
    await wrapper.get('.order-review').trigger('submit')
    await flushPromises()

    expect(mocks.api.applyOrderAction).toHaveBeenCalledWith(51, { action: 'review', note: '交付清楚', rating: 5 })
    expect(wrapper.text()).toContain('交付清楚')
  })
})
