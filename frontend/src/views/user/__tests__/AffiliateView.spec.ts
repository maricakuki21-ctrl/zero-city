import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AffiliateView from '../AffiliateView.vue'

const mocks = vi.hoisted(() => ({
  getAffiliateDetail: vi.fn(),
  transferAffiliateQuota: vi.fn(),
  refreshUser: vi.fn(),
  copy: vi.fn(),
  qr: vi.fn(),
  user: { id: 1 },
}))
vi.mock('@/api/user', () => ({ default: { getAffiliateDetail: mocks.getAffiliateDetail, transferAffiliateQuota: mocks.transferAffiliateQuota } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: mocks.user, refreshUser: mocks.refreshUser }) }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: mocks.copy }) }))
vi.mock('qrcode', () => ({ default: { toDataURL: mocks.qr } }))

function fixture() {
  const tiers = [
    { name: '初识', qualified_invitees: 0, direct_percent: 5, indirect_percent: 1 },
    { name: '同行', qualified_invitees: 5, direct_percent: 8, indirect_percent: 2 },
    { name: '伙伴', qualified_invitees: 20, direct_percent: 12, indirect_percent: 3 },
  ]
  return {
    user_id: 1, aff_code: 'code&safe', aff_count: 9, aff_quota: 7.5, aff_history_quota: 20,
    aff_frozen_quota: 0, effective_rebate_rate_percent: 99,
    growth: { qualified_paid_invitees: 6, current_tier: tiers[1], tiers, direct_percent: 8, indirect_percent: 2, max_combined_percent: 15 },
    invitees: [{ user_id: 3, username: '好友', email: 'friend@example.test', total_rebate: 1.5, created_at: '2026-09-01T00:00:00Z' }],
  }
}
const wrappers: ReturnType<typeof mount>[] = []
function render() {
  const wrapper = mount(AffiliateView, { global: { stubs: {
    AppLayout: { template: '<div><slot /></div>' },
    RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
    BaseDialog: { props: ['show', 'title'], template: '<section v-if="show" role="dialog"><h2>{{ title }}</h2><slot /><slot name="footer" /></section>' },
  } } })
  wrappers.push(wrapper)
  return wrapper
}

describe('AffiliateView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    mocks.user.id = 1
    mocks.getAffiliateDetail.mockResolvedValue(fixture())
    mocks.refreshUser.mockResolvedValue(undefined)
    mocks.transferAffiliateQuota.mockResolvedValue({ transferred_quota: 7.5, balance: 10, credit_balance: 11 })
    mocks.qr.mockResolvedValue('data:image/png;base64,fixture')
  })

  afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()) })

  it('introduces invitations as conditional rebates, not automatic balance top-ups', async () => {
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('.aff-intro').text()).toContain('为解决大家余额越用越少的问题')
    expect(wrapper.get('.aff-intro').text()).toContain('完成符合条件的付费订单')
    expect(wrapper.get('.aff-intro').text()).toContain('注册本身不发放现金')
    expect(localStorage.getItem('bizdecipher:affiliate-discovery:v1:1')).toBe('seen')
    expect(mocks.transferAffiliateQuota).not.toHaveBeenCalled()
  })

  it('keeps the discovery hint when the invitation page fails to load', async () => {
    mocks.getAffiliateDetail.mockRejectedValueOnce(new Error('offline'))
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('offline')
    expect(localStorage.getItem('bizdecipher:affiliate-discovery:v1:1')).toBeNull()
  })

  it('keeps the discovery hint when no invitation code is available', async () => {
    mocks.getAffiliateDetail.mockResolvedValueOnce({ ...fixture(), aff_code: '' })
    render()
    await flushPromises()
    expect(localStorage.getItem('bizdecipher:affiliate-discovery:v1:1')).toBeNull()
  })

  it('does not acknowledge or expose a response after switching accounts', async () => {
    let finish!: (value: ReturnType<typeof fixture>) => void
    mocks.getAffiliateDetail.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    const wrapper = render()
    mocks.user.id = 2
    finish(fixture())
    await flushPromises()
    expect(wrapper.find('.aff-share').exists()).toBe(false)
    expect(localStorage.getItem('bizdecipher:affiliate-discovery:v1:1')).toBeNull()
    expect(localStorage.getItem('bizdecipher:affiliate-discovery:v1:2')).toBeNull()
  })

  it('uses server growth rates and qualified invitees instead of legacy defaults', async () => {
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('.aff-rates').text()).toContain('直接邀请 8%')
    expect(wrapper.get('.aff-rates').text()).toContain('间接邀请 2%')
    expect(wrapper.text()).not.toContain('99%')
    expect(wrapper.get('.aff-progress').text()).toContain('还差 14 位')
    expect(wrapper.text()).not.toContain('历史待结算')
    expect(wrapper.text()).toContain('注册本身不发放现金')
    expect(wrapper.get('a').attributes('href')).toBe('/wallet')
  })

  it('does not invent rates when growth data is unavailable', async () => {
    mocks.getAffiliateDetail.mockResolvedValue({ ...fixture(), growth: undefined, aff_frozen_quota: 3 })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.text()).toContain('等级与返利比例暂不可用')
    expect(wrapper.find('.aff-rates').exists()).toBe(false)
    expect(wrapper.text()).toContain('历史待结算')
  })

  it('copies an encoded invite link and generates the QR locally', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[aria-label="复制邀请链接"]').trigger('click')
    const link = `${window.location.origin}/register?aff=code%26safe`
    expect(mocks.copy).toHaveBeenCalledWith(link, '邀请链接已复制')
    await wrapper.findAll('button').find(button => button.text() === '分享二维码')!.trigger('click')
    await flushPromises()
    expect(mocks.qr).toHaveBeenCalledWith(link, expect.objectContaining({ width: 256 }))
    expect(wrapper.get('img').attributes('src')).toBe('data:image/png;base64,fixture')
  })

  it('requires confirmation and preserves successful transfer through failed readback', async () => {
    mocks.getAffiliateDetail.mockResolvedValueOnce(fixture()).mockRejectedValueOnce(new Error('readback failed'))
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-testid="transfer-review"]').trigger('click')
    expect(mocks.transferAffiliateQuota).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="transfer-confirm"]').trigger('click')
    await flushPromises()
    expect(mocks.transferAffiliateQuota).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-testid="transfer-result"]').text()).toContain('已成功转入')
    expect(wrapper.get('[data-testid="transfer-result"]').text()).toContain('无需重复转入')
    expect(wrapper.get('[data-testid="transfer-review"]').attributes('disabled')).toBeDefined()
  })

  it('requires a refresh before retrying an uncertain transfer', async () => {
    mocks.transferAffiliateQuota.mockRejectedValueOnce(new Error('network interrupted'))
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-testid="transfer-review"]').trigger('click')
    await wrapper.get('[data-testid="transfer-confirm"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="transfer-result"]').text()).toContain('请先刷新核对')
    expect(wrapper.get('[data-testid="transfer-review"]').attributes('disabled')).toBeDefined()
    expect(wrapper.find('[data-testid="transfer-confirm"]').exists()).toBe(false)
  })

  it('shows load errors separately from empty invitees and allows retry', async () => {
    mocks.getAffiliateDetail.mockRejectedValueOnce(new Error('API unavailable'))
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('API unavailable')
    expect(wrapper.text()).not.toContain('暂无邀请记录')
    await wrapper.get('[aria-label="刷新邀请数据"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('friend@example.test')
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })
})
