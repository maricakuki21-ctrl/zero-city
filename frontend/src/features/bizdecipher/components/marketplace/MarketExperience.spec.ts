import { reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import MarketExperience from './MarketExperience.vue'
import type { MarketplaceInquiry, MarketplaceListing, MarketplaceMessage } from '@/features/bizdecipher/api/marketplace'

const mocks = vi.hoisted(() => ({
  guideAPI: { get: vi.fn(), put: vi.fn() },
  auth: { state: undefined as unknown as { user: { id: number; username: string }; isAdmin?: boolean } },
  api: { listListings: vi.fn(), listMyListings: vi.fn(), createListing: vi.fn(), updateListing: vi.fn(), archiveListing: vi.fn(), createInquiry: vi.fn(), listInquiries: vi.fn(), listMessages: vi.fn(), sendMessage: vi.fn(), quoteOrder: vi.fn(), listOrders: vi.fn(), getOrder: vi.fn(), applyOrderAction: vi.fn() },
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => mocks.auth.state }))
vi.mock('@/api/client', () => ({ apiClient: mocks.guideAPI }))
vi.mock('@/features/bizdecipher/api/marketplace', () => ({ marketplaceAPI: mocks.api }))

const listing = (overrides: Partial<MarketplaceListing> = {}): MarketplaceListing => ({ id: 11, kind: 'service', owner_user_id: 9, owner_display_name: '公开昵称', title: '自动化验收', summary: '交付脚本与记录', category: '代码与自动化', price_text: '按项目沟通', delivery_text: '5 天', tags: ['自动化'], status: 'published', created_at: '2026-09-10T01:00:00Z', updated_at: '2026-09-10T01:00:00Z', ...overrides })
const inquiry = (overrides: Partial<MarketplaceInquiry> = {}): MarketplaceInquiry => ({ id: 31, listing_id: 11, listing: listing(), initiator_user_id: 7, listing_owner_user_id: 9, created_at: '2026-09-10T01:00:00Z', updated_at: '2026-09-10T01:00:00Z', last_message_at: '', ...overrides })
const deferred = <T,>() => {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((done, fail) => { resolve = done; reject = fail })
  return { promise, resolve, reject }
}

describe('MarketExperience real flows', () => {
  it('mounts dispute management only through the existing administrator market tab', async () => {
    const wrapper = mount(MarketExperience, { global: { stubs: { MarketplaceDisputesPanel: { template: '<div data-testid="dispute-panel" />' } } } })
    await flushPromises()
    expect(wrapper.text()).not.toContain('争议管理')
    mocks.auth.state.isAdmin = true
    await flushPromises()
    const tab = wrapper.findAll('.market-tabs button').find(button => button.text() === '争议管理')
    expect(tab).toBeDefined()
    await tab!.trigger('click')
    expect(wrapper.find('[data-testid="dispute-panel"]').exists()).toBe(true)
    mocks.auth.state.isAdmin = false
    await flushPromises()
    expect(wrapper.find('[data-testid="dispute-panel"]').exists()).toBe(false)
  })
  beforeEach(() => {
    mocks.guideAPI.get.mockResolvedValue({ data: { version: 1, completed_at: null } })
    mocks.guideAPI.put.mockResolvedValue({ data: { version: 1, completed_at: '2026-09-23T12:00:00Z' } })
    localStorage.clear(); mocks.auth.state = reactive({ user: { id: 7, username: 'alice' } })
    for (const fn of Object.values(mocks.api)) fn.mockReset()
    mocks.api.listListings.mockResolvedValue({ items: [listing()], next_cursor: null })
    mocks.api.listMyListings.mockResolvedValue({ items: [], next_cursor: null })
    mocks.api.listInquiries.mockResolvedValue({ items: [], next_cursor: null })
    mocks.api.listMessages.mockResolvedValue({ items: [], next_cursor: null })
  })

  it('guides a new buyer to an unpublished demand and can be replayed', async () => {
    mocks.auth.state.user = { id: 9876, username: 'newbuyer' }
    const wrapper = mount(MarketExperience); await flushPromises()
    expect(wrapper.find('[data-testid="market-guide"]').exists()).toBe(true)
    await wrapper.get('[data-testid="market-guide-hire"]').trigger('click'); await flushPromises()
    expect(wrapper.find('[data-testid="market-guide"]').exists()).toBe(false)
    expect((wrapper.get('[data-testid="market-editor"] select').element as HTMLSelectElement).value).toBe('demand')
    expect(mocks.api.createListing).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="market-guide-toggle"]').trigger('click'); await flushPromises()
    expect(wrapper.find('[data-testid="market-guide"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('renders server listings and publishes through the backend', async () => {
    mocks.api.createListing.mockResolvedValue(listing({ id: 41, owner_user_id: 7, title: '真实发布' }))
    const wrapper = mount(MarketExperience); await flushPromises()
    expect(wrapper.text()).toContain('公开昵称'); expect(wrapper.text()).not.toContain('示例目录')
    await wrapper.get('[data-testid="market-open-create"]').trigger('click')
    await wrapper.get('[data-testid="market-title"]').setValue('真实发布'); await wrapper.get('[data-testid="market-summary"]').setValue('真实交付范围')
    await wrapper.get('[data-testid="market-editor"]').trigger('submit'); await flushPromises()
    expect(mocks.api.createListing).toHaveBeenCalledWith(expect.objectContaining({ title: '真实发布', summary: '真实交付范围' }))
    expect(localStorage.getItem('market-drafts-v1-7')).toBeNull(); expect(wrapper.text()).toContain('发布已公开')
  })

  it('exposes owner management but inquiry only for another user', async () => {
    const own = listing({ id: 22, owner_user_id: 7, owner_display_name: 'alice' })
    mocks.api.listMyListings.mockResolvedValue({ items: [own], next_cursor: null }); mocks.api.archiveListing.mockResolvedValue({ ...own, status: 'archived' })
    const wrapper = mount(MarketExperience); await flushPromises()
    await wrapper.findAll('.market-tabs button')[3].trigger('click'); await flushPromises(); await wrapper.get('.market-item-title').trigger('click')
    expect(wrapper.text()).toContain('编辑'); expect(wrapper.find('[data-testid="market-start-inquiry"]').exists()).toBe(false)
    await wrapper.get('.market-button.danger').trigger('click'); await flushPromises(); expect(mocks.api.archiveListing).toHaveBeenCalledWith(22)
    await wrapper.findAll('.market-tabs button')[0].trigger('click'); await flushPromises(); await wrapper.get('.market-item-title').trigger('click')
    mocks.api.createInquiry.mockResolvedValue(inquiry()); await wrapper.get('[data-testid="market-start-inquiry"]').trigger('click'); await flushPromises()
    expect(mocks.api.createInquiry).toHaveBeenCalledWith(11)
  })

  it('orders messages for reading and retries with one client message id', async () => {
    const newer: MarketplaceMessage = { id: 102, inquiry_id: 31, sender_user_id: 9, client_message_id: 'b', body: '第二条', created_at: '2026-09-10T02:00:00Z' }
    const older: MarketplaceMessage = { id: 101, inquiry_id: 31, sender_user_id: 7, client_message_id: 'a', body: '第一条', created_at: '2026-09-10T01:00:00Z' }
    mocks.api.listInquiries.mockResolvedValue({ items: [inquiry()], next_cursor: null }); mocks.api.listMessages.mockResolvedValue({ items: [newer, older], next_cursor: null })
    mocks.api.sendMessage.mockRejectedValueOnce(new Error('offline')).mockResolvedValue({ ...newer, id: 103, sender_user_id: 7, body: '请确认范围' })
    const wrapper = mount(MarketExperience); await flushPromises(); await wrapper.get('[data-testid="market-open-messages"]').trigger('click'); await flushPromises()
    expect(wrapper.findAll('.market-messages > p').map(node => node.text())).toEqual(['第一条我', '第二条公开昵称'])
    await wrapper.get('[data-testid="market-message-body"]').setValue('请确认范围'); await wrapper.get('.market-message-form').trigger('submit'); await flushPromises()
    const firstPayload = mocks.api.sendMessage.mock.calls[0][1]; await wrapper.get('.market-message-form').trigger('submit'); await flushPromises()
    expect(mocks.api.sendMessage.mock.calls[1][1]).toEqual(firstPayload); expect((wrapper.get('[data-testid="market-message-body"]').element as HTMLTextAreaElement).value).toBe('')
  })

  it('lets the listing owner quote an inquiry without executing payment', async () => {
    const ownInquiry = inquiry({ listing: listing({ owner_user_id: 7, owner_display_name: 'alice' }), listing_owner_user_id: 7, initiator_user_id: 8 })
    mocks.api.listInquiries.mockResolvedValue({ items: [ownInquiry], next_cursor: null })
    mocks.api.quoteOrder.mockResolvedValue({ id: 51, status: 'quoted' })
    const wrapper = mount(MarketExperience); await flushPromises()
    await wrapper.get('[data-testid="market-open-messages"]').trigger('click'); await flushPromises()
    await wrapper.get('[data-testid="market-order-quote-toggle"]').trigger('click')
    const quoteForm = wrapper.get('[data-testid="market-order-quote-form"]')
    await quoteForm.get('textarea').setValue('交付完整实现与验收记录')
    await quoteForm.get('input').setValue('100 积分')
    await quoteForm.trigger('submit'); await flushPromises()

    expect(mocks.api.quoteOrder).toHaveBeenCalledWith(31, expect.objectContaining({
      scope_text: '交付完整实现与验收记录',
      amount_text: '100 积分',
      revision_limit: 0,
    }))
    expect(wrapper.text()).toContain('报价已记录')
  })

  it('drops a stale personal response after the authenticated user changes', async () => {
    const oldPage = deferred<{ items: MarketplaceListing[]; next_cursor: null }>(); const newPage = deferred<{ items: MarketplaceListing[]; next_cursor: null }>()
    mocks.api.listMyListings.mockReturnValueOnce(oldPage.promise).mockReturnValueOnce(newPage.promise)
    const wrapper = mount(MarketExperience); await flushPromises(); await wrapper.findAll('.market-tabs button')[3].trigger('click'); await flushPromises()
    mocks.auth.state.user = { id: 8, username: 'bob' }; await flushPromises()
    newPage.resolve({ items: [listing({ id: 82, owner_user_id: 8, title: '新用户发布' })], next_cursor: null }); await flushPromises()
    oldPage.resolve({ items: [listing({ id: 71, owner_user_id: 7, title: '旧用户发布' })], next_cursor: null }); await flushPromises()
    expect(wrapper.text()).toContain('新用户发布'); expect(wrapper.text()).not.toContain('旧用户发布')
  })

  it('ignores a stale public-list failure without clearing a newer request', async () => {
    const oldPage = deferred<{ items: MarketplaceListing[]; next_cursor: null }>()
    const filteredPage = deferred<{ items: MarketplaceListing[]; next_cursor: null }>()
    const refreshPage = deferred<{ items: MarketplaceListing[]; next_cursor: null }>()
    mocks.api.listListings.mockReset().mockReturnValueOnce(oldPage.promise).mockReturnValueOnce(filteredPage.promise).mockReturnValueOnce(refreshPage.promise)
    const wrapper = mount(MarketExperience); await flushPromises()
    await wrapper.get('[aria-label="筛选领域"]').setValue('研究与咨询'); await flushPromises()
    filteredPage.resolve({ items: [listing({ id: 91, title: '当前筛选结果', category: '研究与咨询' })], next_cursor: null }); await flushPromises()
    await wrapper.get('.market-toolbar').trigger('submit'); await flushPromises()
    oldPage.reject(new Error('旧筛选失败')); await flushPromises()
    expect(wrapper.text()).toContain('当前筛选结果')
    expect(wrapper.get('[aria-label="刷新市场"]').attributes('disabled')).toBeDefined()
    refreshPage.resolve({ items: [listing({ id: 92, title: '最新刷新结果', category: '研究与咨询' })], next_cursor: null }); await flushPromises()
    expect(wrapper.text()).toContain('最新刷新结果')
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })

  it('drops stale message and send responses after switching private threads', async () => {
    const first = inquiry()
    const second = inquiry({ id: 32, listing_id: 12, listing: listing({ id: 12, title: '第二个线程' }) })
    const firstMessages = deferred<{ items: MarketplaceMessage[]; next_cursor: null }>()
    const secondMessages = deferred<{ items: MarketplaceMessage[]; next_cursor: null }>()
    const staleSend = deferred<MarketplaceMessage>()
    mocks.api.listInquiries.mockResolvedValue({ items: [first, second], next_cursor: null })
    mocks.api.listMessages.mockReturnValueOnce(firstMessages.promise).mockReturnValueOnce(secondMessages.promise)
    mocks.api.sendMessage.mockReturnValue(staleSend.promise)
    const wrapper = mount(MarketExperience); await flushPromises(); await wrapper.get('[data-testid="market-open-messages"]').trigger('click'); await flushPromises()
    await wrapper.get('[data-testid="market-message-body"]').setValue('线程一草稿'); await wrapper.get('.market-message-form').trigger('submit'); await flushPromises()
    await wrapper.findAll('.market-thread')[1].trigger('click'); await flushPromises()
    secondMessages.resolve({ items: [{ id: 202, inquiry_id: 32, sender_user_id: 9, client_message_id: 'second', body: '线程二消息', created_at: '2026-09-10T03:00:00Z' }], next_cursor: null }); await flushPromises()
    firstMessages.resolve({ items: [{ id: 201, inquiry_id: 31, sender_user_id: 9, client_message_id: 'first', body: '线程一旧消息', created_at: '2026-09-10T02:00:00Z' }], next_cursor: null })
    staleSend.resolve({ id: 203, inquiry_id: 31, sender_user_id: 7, client_message_id: 'stale', body: '线程一草稿', created_at: '2026-09-10T04:00:00Z' }); await flushPromises()
    expect(wrapper.text()).toContain('线程二消息'); expect(wrapper.text()).not.toContain('线程一旧消息'); expect(wrapper.text()).not.toContain('线程一草稿')
    expect((wrapper.get('[data-testid="market-message-body"]').element as HTMLTextAreaElement).value).toBe('')
  })

  it('preserves malformed legacy drafts without publishing them', async () => {
    localStorage.setItem('market-drafts-v1-7', '{}'); const wrapper = mount(MarketExperience); await flushPromises()
    expect(wrapper.text()).toContain('原始数据已保留'); expect(localStorage.getItem('market-drafts-v1-7')).toBe('{}'); expect(mocks.api.createListing).not.toHaveBeenCalled()
  })
})
