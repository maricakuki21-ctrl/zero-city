import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SharedPoolOwnerWalletPanel from '@/features/bizdecipher/components/shared-pool/SharedPoolOwnerWalletPanel.vue'

const apiMocks = vi.hoisted(() => ({
  listMySharedPoolLedger: vi.fn(),
  transferSharedPoolOwnerEarnings: vi.fn(),
}))

const earningsApiMocks = vi.hoisted(() => ({
  listSharedPoolOwnerEarningsPage: vi.fn(),
}))

vi.mock('@/features/bizdecipher/api/bizdecipher', () => apiMocks)
vi.mock('@/features/bizdecipher/api/sharedPoolOwnerLedger', () => earningsApiMocks)

function ledgerView() {
  return {
    wallet: {
      owner_id: 71,
      available_amount: 12.5,
      pending_amount: 1.25,
      frozen_amount: 0,
      transferred_amount: 8,
      total_earned: 20.5,
      version: 4,
      updated_at: '2026-07-19T08:00:00Z',
    },
    earnings: [
      {
        id: 1001,
        owner_id: 71,
        pool_id: 42,
        event_type: 'earning',
        operation_id: 'earning-1001',
        request_id: 'request-archived-pool-1001',
        pool_name_snapshot: '已经归档的满血池',
        owner_label_snapshot: '池主 71',
        model_snapshot: 'gpt-5.6',
        pricing_source_snapshot: 'official',
        gross_amount: 6,
        platform_fee_amount: 0.6,
        net_amount: 5.4,
        wallet_delta: 5.4,
        available_after: 12.5,
        status: 'posted',
        created_at: '2026-07-19T07:00:00Z',
        posted_at: '2026-07-19T07:00:01Z',
      },
      {
        id: 1002,
        owner_id: 71,
        pool_id: 77,
        event_type: 'earning',
        operation_id: 'earning-1002',
        request_id: 'request-other-pool',
        pool_name_snapshot: '另一个池',
        owner_label_snapshot: '池主 71',
        model_snapshot: 'gpt-4.1',
        pricing_source_snapshot: 'custom',
        gross_amount: 2,
        platform_fee_amount: 0.2,
        net_amount: 1.8,
        wallet_delta: 1.8,
        available_after: 7.1,
        status: 'posted',
        created_at: '2026-07-18T07:00:00Z',
      },
    ],
    activity: [],
    withdrawable: [],
    legacy_withdrawable: [
      {
        id: 1,
        user_id: 71,
        pool_id: 42,
        source_type: 'share_pool_payout',
        source_id: 'legacy-1',
        asset_type: 'balance_legacy',
        amount: 2,
        balance_after: 10,
        status: 'posted',
        note: '旧 API 分润',
        created_at: '2026-07-01T00:00:00Z',
      },
      {
        id: 2,
        user_id: 71,
        pool_id: 42,
        source_type: 'pool_owner_payout',
        source_id: 'legacy-2',
        asset_type: 'balance_legacy',
        amount: 3,
        balance_after: 13,
        status: 'posted',
        note: '旧席位分润',
        created_at: '2026-07-02T00:00:00Z',
      },
    ],
    incentives: [],
  }
}

describe('SharedPoolOwnerWalletPanel', () => {
  it('labels tavern ticket settlement and exposes only useful ticket identifiers', async () => {
    earningsApiMocks.listSharedPoolOwnerEarningsPage.mockResolvedValueOnce({
      items: [{
        ...ledgerView().earnings[0], pool_id: undefined, pool_name_snapshot: '', model_snapshot: '', pricing_source_snapshot: '',
        metadata: { source_type: 'tavern_ticket_settlement', title: '午夜剧场', script_id: 8, room_id: 91, ticket_id: 321, hidden: 'private-player-key' },
      }], has_more: false,
    })
    const wrapper = mount(SharedPoolOwnerWalletPanel)
    await flushPromises()
    expect(wrapper.get('[data-testid="wallet-pool-snapshot"]').text()).toBe('午夜剧场')
    expect(wrapper.text()).toContain('游戏入场收益 · 房间 #91 · 票据 #321')
    expect(wrapper.get('.spow-receipt').text()).toContain('8 / 91')
    expect(wrapper.get('.spow-receipt').text()).toContain('入场票据#321')
    expect(wrapper.text()).toContain('不包含玩家模型调用费用')
    expect(wrapper.text()).not.toContain('private-player-key')
    await wrapper.get('[aria-label="查找收益凭证"]').setValue('午夜剧场')
    expect(wrapper.findAll('.spow-ledger-row')).toHaveLength(1)
    wrapper.unmount()
  })
  it('labels and searches column receipts and distinguishes site-balance refunds', async () => {
    earningsApiMocks.listSharedPoolOwnerEarningsPage.mockResolvedValueOnce({
      items: [{
        ...ledgerView().earnings[0], pool_id: undefined, pool_name_snapshot: '',
        event_type: 'reversal', wallet_delta: -5.4, status: 'settled',
        metadata: { source_type: 'creator_column_refund', column_title: '创作手记', column_id: 9, purchase_id: 31, hidden: 'secret' },
      }], has_more: false,
    })
    const wrapper = mount(SharedPoolOwnerWalletPanel)
    await flushPromises()
    expect(wrapper.get('[data-testid="wallet-pool-snapshot"]').text()).toBe('创作手记')
    expect(wrapper.get('.spow-receipt').text()).toContain('9 / 31')
    expect(wrapper.text()).toContain('已退回买家的站内余额')
    expect(wrapper.text()).not.toContain('不代表已向付款人退款')
    expect(wrapper.text()).not.toContain('secret')
    await wrapper.get('[aria-label="查找收益凭证"]').setValue('创作手记')
    expect(wrapper.findAll('.spow-ledger-row')).toHaveLength(1)
  })
  it('shows full receipt identifiers and server amounts without exposing arbitrary metadata', async () => {
    earningsApiMocks.listSharedPoolOwnerEarningsPage.mockResolvedValueOnce({
      items: [{ ...ledgerView().earnings[0], account_id: 88, price_version_id: 19, metadata: { hidden: 'private-metadata' } }],
      has_more: false,
    })
    const wrapper = mount(SharedPoolOwnerWalletPanel)
    await flushPromises()
    const receipt = wrapper.get('.spow-receipt')
    expect(receipt.text()).toContain('已入账')
    expect(receipt.text()).toContain('earning-1001')
    expect(receipt.text()).toContain('request-archived-pool-1001')
    expect(receipt.text()).toContain('42 / 88 / 19')
    expect(receipt.text()).toContain('净收益5.40')
    expect(receipt.text()).not.toContain('private-metadata')
  })

  it('filters only loaded receipts and keeps pagination available for a zero-match filter', async () => {
    earningsApiMocks.listSharedPoolOwnerEarningsPage.mockResolvedValueOnce({
      items: ledgerView().earnings, has_more: true, next_before_id: 1002,
    }).mockResolvedValueOnce({
      items: [{ ...ledgerView().earnings[0], id: 900, event_type: 'reversal', wallet_delta: -5.4 }],
      has_more: false,
    })
    const wrapper = mount(SharedPoolOwnerWalletPanel)
    await flushPromises()
    await wrapper.get('[aria-label="筛选收益记录类型"]').setValue('reversal')
    expect(wrapper.text()).toContain('已加载记录中没有匹配项')
    expect(wrapper.find('[data-testid="wallet-earnings-load-more"]').exists()).toBe(true)
    await wrapper.get('[data-testid="wallet-earnings-load-more"]').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('.spow-ledger-row')).toHaveLength(1)
    expect(wrapper.text()).toContain('不代表已向付款人退款')
    await wrapper.get('[aria-label="查找收益凭证"]').setValue('request-archived-pool-1001')
    expect(wrapper.findAll('.spow-ledger-row')).toHaveLength(1)
    expect(wrapper.get('[data-testid="wallet-available"]').text()).toBe('12.50')
  })

  it('does not report zero historical earnings or empty records when initial load fails', async () => {
    apiMocks.listMySharedPoolLedger.mockRejectedValueOnce(new Error('ledger unavailable'))
    const wrapper = mount(SharedPoolOwnerWalletPanel)
    await flushPromises()
    expect(wrapper.get('[data-testid="wallet-legacy"]').text()).toBe('暂不可用')
    expect(wrapper.text()).toContain('当前不能确认是否有收益')
    expect(wrapper.text()).not.toContain('暂无新钱包收益记录')
  })

  beforeEach(() => {
    vi.clearAllMocks()
    sessionStorage.clear()
    apiMocks.listMySharedPoolLedger.mockResolvedValue(ledgerView())
    earningsApiMocks.listSharedPoolOwnerEarningsPage.mockResolvedValue({
      items: ledgerView().earnings,
      has_more: false,
    })
    apiMocks.transferSharedPoolOwnerEarnings.mockResolvedValue({
      operation_id: 'unused',
      amount: 4.25,
      wallet_after: 8.25,
      balance_after: 30,
      already_done: false,
    })
  })

  it('explains all four balances and reads an archived pool from its permanent snapshot', async () => {
    const wrapper = mount(SharedPoolOwnerWalletPanel, { props: { poolId: 42 } })
    await flushPromises()

    expect(wrapper.get('[data-testid="wallet-total-earned"]').text()).toBe('25.50')
    expect(wrapper.get('[data-testid="wallet-available"]').text()).toBe('12.50')
    expect(wrapper.get('[data-testid="wallet-transferred"]').text()).toBe('8.00')
    expect(wrapper.get('[data-testid="wallet-legacy"]').text()).toBe('5.00')
    expect(wrapper.text()).toContain('新收益先进入独立钱包，不会直接增加站点普通余额')
    expect(wrapper.text()).toContain('将收益转入站内余额')
    expect(wrapper.text()).toContain('不会转到银行卡或链上地址')
    expect(wrapper.text()).not.toContain('可提现')
    expect(wrapper.text()).toContain('已经归档的满血池')
    expect(wrapper.text()).not.toContain('另一个池')
    expect(wrapper.text()).toContain('名称按发生时快照保存')
  })

  it('requires a second confirmation and reuses the same operation id after a failed response', async () => {
    apiMocks.transferSharedPoolOwnerEarnings
      .mockRejectedValueOnce({ message: '网络中断，结果未知' })
      .mockResolvedValueOnce({
        operation_id: 'server-returns-the-request-operation-id',
        amount: 4.25,
        wallet_after: 8.25,
        balance_after: 30,
        already_done: true,
      })

    const wrapper = mount(SharedPoolOwnerWalletPanel)
    await flushPromises()
    await wrapper.get('[data-testid="wallet-transfer-input"]').setValue('4.25')
    await wrapper.get('[data-testid="wallet-transfer-review"]').trigger('click')

    expect(apiMocks.transferSharedPoolOwnerEarnings).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('请再确认一次')

    await wrapper.get('[data-testid="wallet-transfer-confirm"]').trigger('click')
    await flushPromises()

    expect(apiMocks.transferSharedPoolOwnerEarnings).toHaveBeenCalledTimes(1)
    const firstOperationId = apiMocks.transferSharedPoolOwnerEarnings.mock.calls[0][1]
    expect(firstOperationId).toMatch(/^shared-pool-owner-transfer-/)
    expect(wrapper.text()).toContain('网络中断，结果未知')

    await wrapper.get('[data-testid="wallet-transfer-confirm"]').trigger('click')
    await flushPromises()

    expect(apiMocks.transferSharedPoolOwnerEarnings).toHaveBeenCalledTimes(2)
    expect(apiMocks.transferSharedPoolOwnerEarnings.mock.calls[1][1]).toBe(firstOperationId)
    expect(wrapper.text()).toContain('此前已经完成')
    expect(apiMocks.listMySharedPoolLedger).toHaveBeenCalledTimes(3)
    expect(sessionStorage.getItem('shared-pool-owner-wallet-transfer-draft-v1')).toBeNull()
  })

  it('blocks duplicate confirmation clicks while the first request is still pending', async () => {
    let resolveTransfer!: (value: unknown) => void
    apiMocks.transferSharedPoolOwnerEarnings.mockReturnValue(new Promise((resolve) => {
      resolveTransfer = resolve
    }))

    const wrapper = mount(SharedPoolOwnerWalletPanel)
    await flushPromises()
    await wrapper.get('[data-testid="wallet-transfer-input"]').setValue('2')
    await wrapper.get('[data-testid="wallet-transfer-review"]').trigger('click')
    const confirmButton = wrapper.get('[data-testid="wallet-transfer-confirm"]')

    await confirmButton.trigger('click')
    await confirmButton.trigger('click')

    expect(apiMocks.transferSharedPoolOwnerEarnings).toHaveBeenCalledTimes(1)
    expect(confirmButton.attributes('disabled')).toBeDefined()

    resolveTransfer({
      operation_id: apiMocks.transferSharedPoolOwnerEarnings.mock.calls[0][1],
      amount: 2,
      wallet_after: 10.5,
      balance_after: 27.75,
      already_done: false,
    })
    await flushPromises()
    expect(wrapper.text()).toContain('已转入 2.00')
  })

  it('loads older permanent earnings with the same server-side pool filter', async () => {
    const initial = ledgerView().earnings.slice(0, 1)
    const older = {
      ...ledgerView().earnings[0],
      id: 900,
      request_id: 'request-archived-pool-older',
      pool_name_snapshot: '更早的归档池快照',
      created_at: '2026-06-01T00:00:00Z',
    }
    earningsApiMocks.listSharedPoolOwnerEarningsPage
      .mockResolvedValueOnce({
        items: initial,
        next_before_id: 1001,
        has_more: true,
      })
      .mockResolvedValueOnce({
        items: [older],
        has_more: false,
      })

    const wrapper = mount(SharedPoolOwnerWalletPanel, { props: { poolId: 42 } })
    await flushPromises()
    await wrapper.get('[data-testid="wallet-earnings-load-more"]').trigger('click')
    await flushPromises()

    expect(earningsApiMocks.listSharedPoolOwnerEarningsPage).toHaveBeenNthCalledWith(1, {
      poolId: 42,
      limit: 50,
    })
    expect(earningsApiMocks.listSharedPoolOwnerEarningsPage).toHaveBeenNthCalledWith(2, {
      beforeId: 1001,
      poolId: 42,
      limit: 50,
    })
    expect(wrapper.text()).toContain('更早的归档池快照')
    expect(wrapper.find('[data-testid="wallet-earnings-load-more"]').exists()).toBe(false)
  })

  it('does not turn a missing wallet into real zero balances', async () => {
    apiMocks.listMySharedPoolLedger.mockResolvedValue({
      wallet: undefined,
      earnings: [],
      activity: [],
      withdrawable: [],
      legacy_withdrawable: [],
      incentives: [],
    })
    const wrapper = mount(SharedPoolOwnerWalletPanel)
    await flushPromises()

    expect(wrapper.get('[data-testid="wallet-available"]').text()).toBe('暂不可用')
    expect(wrapper.get('[data-testid="wallet-total-earned"]').text()).toBe('暂不可用')
    expect(wrapper.get('[data-testid="wallet-transfer-input"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).not.toContain('0.00')
  })

  it('preserves genuine zero wallet amounts', async () => {
    apiMocks.listMySharedPoolLedger.mockResolvedValue({
      wallet: { owner_id: 71, available_amount: 0, pending_amount: 0, frozen_amount: 0, transferred_amount: 0, total_earned: 0, version: 1 },
      earnings: [], activity: [], withdrawable: [], legacy_withdrawable: [], incentives: [],
    })
    const wrapper = mount(SharedPoolOwnerWalletPanel)
    await flushPromises()
    expect(wrapper.get('[data-testid="wallet-available"]').text()).toBe('0.00')
    expect(wrapper.get('[data-testid="wallet-total-earned"]').text()).toBe('0.00')
  })
})
