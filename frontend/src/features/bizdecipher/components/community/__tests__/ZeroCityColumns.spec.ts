import { reactive } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import ZeroCityColumns from '../ZeroCityColumns.vue'
import type { ColumnArticle, CreatorColumn } from '@/features/bizdecipher/api/columns'

const mocks = vi.hoisted(() => ({
  auth: { state: undefined as unknown as { user: { id: number; role: string } } },
  api: {
    list: vi.fn(), listAdmin: vi.fn(), get: vi.fn(), create: vi.fn(), update: vi.fn(),
    moderate: vi.fn(), articles: vi.fn(), article: vi.fn(), createArticle: vi.fn(), updateArticle: vi.fn(),
    policy: vi.fn(), setPolicy: vi.fn(), setPricing: vi.fn(), purchase: vi.fn(), purchases: vi.fn(), refund: vi.fn(),
  },
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => mocks.auth.state }))
vi.mock('@/features/bizdecipher/api/columns', () => ({ columnsAPI: mocks.api }))
const column: CreatorColumn = { id: 10, owner_user_id: 7, author_name: '城中作者', title: '创作笔记', description: '每周一篇实践记录', status: 'active', moderation_reason: '', article_count: 1, created_at: '2026-09-19', updated_at: '2026-09-19' }
const article: ColumnArticle = { id: 20, column_id: 10, author_user_id: 7, title: '第一篇文章', summary: '实践摘要', body: '<img src=x onerror=alert(1)>正文', status: 'published', created_at: '2026-09-19', updated_at: '2026-09-19' }
const mounted: VueWrapper[] = []
async function render(query = '') {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/community', component: { template: '<div />' } }] })
  await router.push(`/community?view=columns${query}`)
  await router.isReady()
  const wrapper = mount(ZeroCityColumns, { global: { plugins: [router] } })
  mounted.push(wrapper)
  await flushPromises()
  return { wrapper, router }
}
async function click(wrapper: VueWrapper, text: string) {
  const button = wrapper.findAll('button').find(item => item.text() === text)
  expect(button, text).toBeDefined()
  await button!.trigger('click')
  await flushPromises()
}
beforeEach(() => {
  mocks.auth.state = reactive({ user: { id: 7, role: 'user' } })
  Object.values(mocks.api).forEach(fn => fn.mockReset())
  mocks.api.list.mockResolvedValue({ items: [column] })
  mocks.api.listAdmin.mockResolvedValue({ items: [column] })
  mocks.api.get.mockResolvedValue(column)
  mocks.api.articles.mockResolvedValue({ items: [{ ...article, body: undefined }] })
  mocks.api.article.mockResolvedValue(article)
  mocks.api.policy.mockResolvedValue({ enabled: false, currency: 'USD', platform_fee: '0.00000000' })
  mocks.api.purchases.mockResolvedValue({ items: [] })
})
afterEach(() => { mounted.splice(0).forEach(wrapper => wrapper.unmount()) })

describe('creator columns', () => {
  it('lists real columns and retains a navigable article URL', async () => {
    const { wrapper, router } = await render()
    expect(mocks.api.list).toHaveBeenCalledWith(false, undefined)
    await click(wrapper, '创作笔记')
    expect(router.currentRoute.value.query.column).toBe('10')
    await click(wrapper, '第一篇文章')
    expect(router.currentRoute.value.query.article).toBe('20')
    expect(wrapper.get('.article-body').text()).toBe(article.body)
    expect(wrapper.find('.article-body img').exists()).toBe(false)
  })
  it('creates a column then opens the actual server record', async () => {
    mocks.api.create.mockResolvedValue(column)
    const { wrapper, router } = await render()
    await click(wrapper, '开通专栏')
    await wrapper.get('.editor input').setValue('创作笔记')
    await wrapper.get('.editor textarea').setValue('每周一篇实践记录')
    await wrapper.get('.editor').trigger('submit')
    await flushPromises()
    expect(mocks.api.create).toHaveBeenCalledWith({ title: column.title, description: column.description })
    expect(router.currentRoute.value.query.column).toBe('10')
  })
  it('publishes through the API, never merely adds a local article', async () => {
    mocks.api.createArticle.mockResolvedValue(article)
    const { wrapper } = await render('&column=10')
    await click(wrapper, '写文章')
    await wrapper.get('.article-editor input').setValue('第一篇文章')
    await wrapper.findAll('.article-editor textarea')[0].setValue('实践摘要')
    await wrapper.findAll('.article-editor textarea')[1].setValue('文章正文')
    await click(wrapper, '发布文章')
    expect(mocks.api.createArticle).toHaveBeenCalledWith(10, { title: '第一篇文章', summary: '实践摘要', body: '文章正文', status: 'published' })
    expect(wrapper.find('.article-editor').exists()).toBe(false)
  })
  it('keeps failed drafts editable and does not claim successful publication', async () => {
    mocks.api.createArticle.mockRejectedValue(new Error('服务暂不可用'))
    const { wrapper } = await render('&column=10')
    await click(wrapper, '写文章')
    await wrapper.get('.article-editor input').setValue('未发布')
    await wrapper.findAll('.article-editor textarea')[1].setValue('保留正文')
    await click(wrapper, '发布文章')
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    expect((wrapper.findAll('.article-editor textarea')[1].element as HTMLTextAreaElement).value).toBe('保留正文')
    expect(wrapper.text()).not.toContain('文章已发布。')
  })
  it('does not show author editing to another reader', async () => {
    mocks.auth.state.user.id = 8
    const { wrapper } = await render('&column=10&article=20')
    expect(mocks.api.articles).toHaveBeenCalledWith(10, false, undefined)
    expect(wrapper.findAll('button').map(button => button.text())).not.toContain('编辑')
    expect(wrapper.text()).not.toContain('写文章')
  })
  it('requires an admin reason and sends a real moderation action', async () => {
    mocks.auth.state.user.role = 'admin'
    mocks.api.moderate.mockResolvedValue({ ...column, status: 'suspended' })
    const { wrapper } = await render('&column=10')
    expect(wrapper.get('.moderation button').attributes('disabled')).toBeDefined()
    await wrapper.get('.moderation input').setValue('违规内容，暂停公开')
    await wrapper.get('.moderation').trigger('submit')
    await flushPromises()
    expect(mocks.api.moderate).toHaveBeenCalledWith(10, 'suspended', '违规内容，暂停公开')
  })
  it('drops an old account response after switching account', async () => {
    let resolve!: (value: { items: CreatorColumn[] }) => void
    mocks.api.list.mockImplementationOnce(() => new Promise(done => { resolve = done }))
    const { wrapper } = await render()
    mocks.api.list.mockResolvedValue({ items: [] })
    mocks.auth.state.user = { id: 8, role: 'user' }
    await flushPromises()
    resolve({ items: [column] })
    await flushPromises()
    expect(wrapper.text()).not.toContain('创作笔记')
  })
  it('does not fetch paid bodies or charge while purchases are disabled', async () => {
    mocks.auth.state.user.id = 8
    mocks.api.get.mockResolvedValue({ ...column, mode: 'paid', price: '2.00000000', can_read: false })
    const { wrapper } = await render('&column=10&article=20')
    expect(mocks.api.article).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('付费购买尚未开放')
    expect(wrapper.find('.article-body').exists()).toBe(false)
    expect(mocks.api.purchase).not.toHaveBeenCalled()
  })
  it('confirms the price and reuses an operation id after an uncertain purchase', async () => {
    sessionStorage.clear()
    mocks.auth.state.user.id = 8
    mocks.api.policy.mockResolvedValue({ enabled: true, currency: 'USD', platform_fee: '0.00000000' })
    mocks.api.get.mockResolvedValue({ ...column, mode: 'paid', price: '2.00000000', can_read: false })
    mocks.api.purchase.mockRejectedValueOnce(new Error('network timeout')).mockResolvedValueOnce({ id: 1, status: 'active' })
    const { wrapper } = await render('&column=10&article=20')
    await click(wrapper, '购买专栏')
    expect(mocks.api.purchase).not.toHaveBeenCalled()
    await click(wrapper, '确认扣款 2 USD')
    const first = mocks.api.purchase.mock.calls[0]
    expect(first).toEqual([10, expect.any(String), '2.00000000'])
    mocks.api.get.mockResolvedValue({ ...column, mode: 'paid', price: '2.00000000', can_read: true, viewer_purchase_id: 1 })
    await click(wrapper, '确认扣款 2 USD')
    expect(mocks.api.purchase.mock.calls[1]).toEqual(first)
    expect(mocks.api.article).toHaveBeenCalledWith(10, 20)
    expect(wrapper.find('.article-body').exists()).toBe(true)
  })
  it('allows only administrators to enable purchases and requires a reason', async () => {
    mocks.auth.state.user.role = 'admin'
    mocks.api.setPolicy.mockResolvedValue({ enabled: true, currency: 'USD', platform_fee: '0.00000000' })
    const { wrapper } = await render()
    await click(wrapper, '专栏管理')
    expect(wrapper.get('.commerce-policy button').attributes('disabled')).toBeDefined()
    await wrapper.get('.commerce-policy input[type="checkbox"]').setValue(true)
    await wrapper.get('.commerce-policy input:not([type="checkbox"])').setValue('已完成收费验收')
    await wrapper.get('.commerce-policy').trigger('submit')
    await flushPromises()
    expect(mocks.api.setPolicy).toHaveBeenCalledWith(true, '已完成收费验收')
  })
  it('requires a reason for a real administrator refund and preserves failures', async () => {
    mocks.auth.state.user.role = 'admin'
    mocks.api.purchases.mockResolvedValue({ items: [{ id: 30, buyer_user_id: 8, amount: '2.00000000', creator_amount: '2.00000000', status: 'active', created_at: '2026-09-19' }] })
    mocks.api.refund.mockRejectedValue(new Error('insufficient creator funds'))
    const { wrapper } = await render('&column=10')
    await click(wrapper, '管理退款')
    expect(wrapper.get('.refund-editor button[type="submit"]').attributes('disabled')).toBeDefined()
    await wrapper.get('.refund-editor textarea').setValue('已核实争议')
    await wrapper.get('.refund-editor').trigger('submit')
    await flushPromises()
    expect(mocks.api.refund).toHaveBeenCalledWith(10, 30, expect.any(String), '已核实争议')
    expect(wrapper.find('.refund-editor').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('已退回买家站点余额')
  })
})
