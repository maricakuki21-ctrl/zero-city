import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import WalletView from '../WalletView.vue'

const auth = vi.hoisted(() => ({
  store: null as null | { token: string; user: Record<string, unknown> },
}))
const api = vi.hoisted(() => ({ getCurrentUser: vi.fn() }))
const ledgerApi = vi.hoisted(() => ({ listMySharedPoolLedger: vi.fn() }))
const routeState = vi.hoisted(() => ({ query: {} as Record<string, unknown> }))
vi.mock('vue-router', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-router')>(),
  useRoute: () => routeState,
}))

vi.mock('@/stores/auth', async () => {
  const { reactive } = await import('vue')
  auth.store = reactive({ token: 'token-a', user: { id: 1, balance: 12.5, credit_balance: 80 } })
  return { useAuthStore: () => auth.store }
})
vi.mock('@/api', () => ({ authAPI: api }))
vi.mock('@/features/bizdecipher/api/bizdecipher', () => ({
  listMySharedPoolLedger: ledgerApi.listMySharedPoolLedger,
}))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))
vi.mock('@/features/bizdecipher/components/wallet/WithdrawalPanel.vue', () => ({
  default: { props: ['ownerId'], emits: ['changed'], template: '<button class="withdrawal-changed" :data-owner-id="ownerId" @click="$emit(\'changed\')">withdrawal</button>' },
}))
vi.mock('@/features/bizdecipher/components/shared-pool/SharedPoolOwnerWalletPanel.vue', () => ({
  default: { props: ['poolId'], emits: ['refreshed'], template: '<button class="ledger-refreshed" :data-pool-id="poolId" @click="$emit(\'refreshed\', {})">ledger</button>' },
}))

describe('WalletView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    routeState.query = {}
    auth.store!.token = 'token-a'
    auth.store!.user = { id: 1, balance: 12.5, credit_balance: 80 }
    ledgerApi.listMySharedPoolLedger.mockResolvedValue({
      wallet: { owner_id: 1, available_amount: 30, pending_amount: 4, frozen_amount: 0, transferred_amount: 6, total_earned: 40, version: 1 },
      earnings: [], activity: [], withdrawable: [], legacy_withdrawable: [], incentives: [],
    })
  })

  it.each([['42', '42'], ['-1', undefined], ['invalid', undefined], [['42'], undefined], ['9007199254740992', undefined]])('validates the pool filter %s', async (value, expected) => {
    routeState.query = { pool_id: value }
    api.getCurrentUser.mockResolvedValue({ data: auth.store!.user })
    const wrapper = mount(WalletView, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()
    expect(wrapper.get('.ledger-refreshed').attributes('data-pool-id')).toBe(expected)
    expect(wrapper.find('.wallet-earnings-context').exists()).toBe(!!expected)
  })

  it('refreshes auth and keeps site balance separate from credits', async () => {
    api.getCurrentUser.mockResolvedValue({ data: { id: 1, balance: 24.75, credit_balance: 135 } })
    const wrapper = mount(WalletView, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()

    expect(api.getCurrentUser).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-testid="site-balance"]').text()).toBe('24.75')
    expect(wrapper.get('[data-testid="credit-balance"]').text()).toBe('135.00')
    expect(wrapper.text()).not.toContain('160.00')
  })

  it('retains cached independent balances and exposes auth refresh errors', async () => {
    api.getCurrentUser.mockRejectedValue({ message: '账户接口不可用' })
    const wrapper = mount(WalletView, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()

    expect(wrapper.get('[role="alert"]').text()).toContain('账户接口不可用')
    expect(wrapper.get('[data-testid="site-balance"]').text()).toBe('12.50')
    expect(wrapper.get('[data-testid="credit-balance"]').text()).toBe('80.00')
  })

  it('refreshes the auth balance when the real earnings panel refreshes after transfer', async () => {
    api.getCurrentUser.mockResolvedValue({ data: auth.store!.user })
    const wrapper = mount(WalletView, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()
    await wrapper.get('.ledger-refreshed').trigger('click')
    await flushPromises()

    expect(api.getCurrentUser).toHaveBeenCalledTimes(2)
    expect(wrapper.get('.withdrawal-changed').attributes('data-owner-id')).toBe('1')
  })

  it('refreshes available earnings after a withdrawal change without mixing site balance', async () => {
    api.getCurrentUser.mockResolvedValue({ data: auth.store!.user })
    const wrapper = mount(WalletView, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()
    ledgerApi.listMySharedPoolLedger.mockResolvedValue({ wallet: { available_amount: 18, total_earned: 40 } })
    await wrapper.get('.withdrawal-changed').trigger('click')
    await flushPromises()
    expect(ledgerApi.listMySharedPoolLedger).toHaveBeenCalledTimes(2)
    expect(wrapper.get('[data-testid="earnings-available"]').text()).toBe('18.00')
    expect(wrapper.get('[data-testid="site-balance"]').text()).toBe('12.50')
  })

  it('ignores a late balance response after the authenticated user switches', async () => {
    let resolveRequest!: (value: unknown) => void
    api.getCurrentUser.mockReturnValue(new Promise(resolve => { resolveRequest = resolve }))
    const wrapper = mount(WalletView, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await nextTick()

    auth.store!.token = 'token-b'
    auth.store!.user = { id: 2, balance: 7.25, credit_balance: 9 }
    await nextTick()
    resolveRequest({ data: { id: 1, balance: 999, credit_balance: 999 } })
    await flushPromises()

    expect(wrapper.get('[data-testid="site-balance"]').text()).toBe('7.25')
    expect(wrapper.get('[data-testid="credit-balance"]').text()).toBe('9.00')
    expect(wrapper.get('.withdrawal-changed').attributes('data-owner-id')).toBe('2')
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })

  it('shows separate asset values without hardcoded development status', async () => {
    api.getCurrentUser.mockResolvedValue({ data: auth.store!.user })
    const wrapper = mount(WalletView, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()

    expect(wrapper.get('[data-testid="earnings-available"]').text()).toBe('30.00')
    expect(wrapper.text()).toContain('累计已结算 40.00')
    expect(wrapper.text()).toContain('待结算 4.00')
    expect(wrapper.text()).not.toContain('未接通')
    expect(wrapper.find('[aria-label="收益来源接通状态"]').exists()).toBe(false)
    // Three separate values must never be summed into one figure.
    expect(wrapper.text()).not.toContain('122.50')
    expect(wrapper.text()).not.toContain('150.00')
  })

  it('surfaces an earnings failure without hiding the cached balance', async () => {
    api.getCurrentUser.mockResolvedValue({ data: auth.store!.user })
    ledgerApi.listMySharedPoolLedger.mockRejectedValue({ message: '收益接口不可用' })
    const wrapper = mount(WalletView, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()

    expect(wrapper.text()).toContain('收益接口不可用')
    expect(wrapper.get('[data-testid="site-balance"]').text()).toBe('12.50')
    expect(wrapper.get('[data-testid="credit-balance"]').text()).toBe('80.00')
  })
})
