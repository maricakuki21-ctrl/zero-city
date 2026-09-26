import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AccountSquareMyView from '@/features/bizdecipher/views/user/AccountSquareMyView.vue'

const mocks = vi.hoisted(() => ({
  seats: vi.fn(), keys: vi.fn(), ledger: vi.fn(), create: vi.fn(), copy: vi.fn(),
  route: { query: {} as Record<string, string> },
}))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))
vi.mock('@/features/bizdecipher/components/shared-pool/SharedPoolUsageTracePanel.vue', () => ({ default: { template: '<div />' } }))
vi.mock('vue-router', () => ({ useRoute: () => mocks.route }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: mocks.copy }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('@/features/bizdecipher/api/bizdecipher', () => ({
  listMySeats: mocks.seats, listMySharedPoolAccessKeys: mocks.keys, listMySharedPoolLedger: mocks.ledger,
  createSharedPoolAccessKey: mocks.create, deleteSharedPoolAccessKey: vi.fn(), leaveSharedPool: vi.fn(),
}))

const seat = { id: 1, pool_id: 7, pool_name: 'Example pool', status: 'active', hourly_seat_fee: 1, hourly_min_usage_waiver: 2, current_hour_usage_amount: .5, total_charged: 4, joined_at: '2026-09-08T00:00:00Z' }
const key = { id: 1, api_key_id: 2, pool_id: 7, pool_name: 'Example pool', status: 'active', key: 'fixture-not-real-key', key_preview: 'sk-share-••••', allowed_models: ['fixture-model'] }
function render() { return mount(AccountSquareMyView, { global: { stubs: { Teleport: true, RouterLink: { template: '<a><slot /></a>' } } } }) }
beforeEach(() => {
  vi.resetAllMocks()
  mocks.route.query = {}
  mocks.seats.mockResolvedValue([seat])
  mocks.keys.mockResolvedValue([key])
  mocks.ledger.mockResolvedValue({ activity: [], incentives: [] })
})
describe('Shared member center', () => {
  it('has three keyboard-operable main tabs instead of five competing entries', async () => {
    const wrapper = render()
    await flushPromises()
    expect(wrapper.findAll('[role=tab]').map(tab => tab.text())).toEqual(['已加入', '资源绑定', '消费账单'])
    expect(wrapper.find('.asmy-operations').exists()).toBe(false)
    await wrapper.get('#member-tab-pools').trigger('keydown', { key: 'ArrowRight' })
    expect(wrapper.get('#member-tab-unified').attributes('aria-selected')).toBe('true')
    expect(wrapper.get('[role=tabpanel]').attributes('aria-labelledby')).toBe('member-tab-unified')
  })
  it('opens an existing key without recreating the binding, masking its secret', async () => {
    const wrapper = render()
    await flushPromises()
    const button = wrapper.findAll('button').find(item => item.text() === '查看 Key')
    await button?.trigger('click')
    expect(mocks.create).not.toHaveBeenCalled()
    expect(wrapper.text()).not.toContain(key.key)
    await wrapper.get('[aria-label="复制完整 Key"]').trigger('click')
    expect(mocks.copy).toHaveBeenCalledWith(key.key, '密钥已复制')
    expect(wrapper.get('#member-tab-pools').attributes('aria-selected')).toBe('true')
  })
  it('searches joined resources and preserves the fee detail', async () => {
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('.member-resource-row').text()).toContain('席位费')
    await wrapper.get('[aria-label="搜索已加入资源"]').setValue('absent')
    expect(wrapper.text()).toContain('没有匹配的共享池')
  })
  it('keeps legacy key links but cannot copy a masked preview', async () => {
    mocks.route.query = { tab: 'unified' }
    mocks.keys.mockResolvedValue([{ ...key, key: undefined }])
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('#member-tab-unified').attributes('aria-selected')).toBe('true')
    const copy = wrapper.findAll('button').find(item => item.text() === '复制完整 Key')
    expect(copy?.attributes('disabled')).toBeDefined()
    expect(mocks.copy).not.toHaveBeenCalled()
  })
  it('keeps unavailable seats discoverable without allowing key creation', async () => {
    mocks.seats.mockResolvedValue([seat, { ...seat, id: 2, pool_id: 8, pool_name: 'Paused pool', status: 'suspended', release_reason: 'pool_unavailable' }])
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('h1').text()).toBe('我的资源')
    await wrapper.get('[aria-label="筛选资源状态"]').setValue('unavailable')
    const row = wrapper.get('.member-resource-row')
    expect(row.text()).toContain('Paused pool')
    expect(row.text()).toContain('池已下架或暂不可用')
    const keyButton = row.findAll('button').find(button => button.text() === '生成 Key')
    expect(keyButton?.attributes('disabled')).toBeDefined()
    expect(wrapper.findAll('.member-resource-row')).toHaveLength(1)
  })
  it('opens a pool-scoped ledger and can return to all loaded entries', async () => {
    mocks.ledger.mockResolvedValue({
      activity: [
        { id: 1, pool_id: 7, source_type: 'api_usage', amount: -1, created_at: '2026-09-12T00:00:00Z' },
        { id: 2, pool_id: 9, source_type: 'api_usage', amount: -2, created_at: '2026-09-12T00:00:00Z' },
      ], incentives: [
        { id: 3, pool_id: 7, source_type: 'adjustment', amount: 1, created_at: '2026-09-12T00:00:00Z' },
        { id: 4, pool_id: 9, source_type: 'adjustment', amount: 2, created_at: '2026-09-12T00:00:00Z' },
      ],
    })
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[aria-label="查看 Example pool 的消费记录"]').trigger('click')
    expect(wrapper.get('#member-tab-ledger').attributes('aria-selected')).toBe('true')
    expect(wrapper.findAll('.asmy-ledger-row')).toHaveLength(2)
    expect(wrapper.get('.asmy-ledger-row').text()).toContain('最终归属池 #7')
    await wrapper.get('.member-ledger-filter button').trigger('click')
    expect(wrapper.findAll('.asmy-ledger-row')).toHaveLength(4)
  })
  it('separates inactive resources from released seat history', async () => {
    mocks.seats.mockResolvedValue([
      { ...seat, status: 'inactive' },
      { ...seat, id: 2, pool_id: 8, status: 'left' },
      { ...seat, id: 3, pool_id: 9, status: 'released' },
    ])
    const wrapper = render()
    await flushPromises()
    expect(wrapper.findAll('.member-resource-row')).toHaveLength(1)
    expect(wrapper.get('.member-resource-row').text()).toContain('席位失效')
    await wrapper.get('#member-tab-ledger').trigger('click')
    const historyTab = wrapper.findAll('button').find(button => button.text() === '席位费用与退出记录')
    await historyTab?.trigger('click')
    expect(wrapper.findAll('.asmy-history-list article')).toHaveLength(2)
  })
})
