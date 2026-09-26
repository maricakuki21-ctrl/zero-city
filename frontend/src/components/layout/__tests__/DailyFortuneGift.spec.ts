import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import DailyFortuneGift from '../DailyFortuneGift.vue'
import type { CheckinClaimResponse, CheckinCollectibleCard, CheckinStatus, CreditLotterySession } from '@/types'
import { checkinOperationStorageKey } from '@/utils/lotteryOperationRecovery'

const apiMocks = vi.hoisted(() => ({
  getCheckinStatus: vi.fn(),
  getActiveCreditLotterySession: vi.fn(),
  getCheckinRecords: vi.fn(),
  getCheckinCards: vi.fn(),
  claimCheckin: vi.fn(),
  getCheckinOperation: vi.fn(),
  claimCheckinMilestone: vi.fn(),
  buyCheckinCard: vi.fn()
}))

const storeMocks = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
  patchUserBalance: vi.fn(),
  refreshUser: vi.fn(),
  user: { id: 7, balance: 0, credit_balance: 0 }
}))

vi.mock('@/api/user', () => ({
  default: apiMocks
}))

vi.mock('@/features/bizdecipher/components/layout/CreditLotteryThreeRounds.vue', () => ({
  default: {
    name: 'CreditLotteryThreeRounds',
    template: '<div data-testid="credit-lottery-three-rounds">credit lottery</div>'
  }
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError: storeMocks.showError,
    showSuccess: storeMocks.showSuccess
  }),
  useAuthStore: () => ({
    user: storeMocks.user,
    patchUserBalance: storeMocks.patchUserBalance,
    refreshUser: storeMocks.refreshUser
  })
}))

vi.mock('@/utils/apiError', () => ({
  extractApiErrorMessage: (_error: unknown, fallback: string) => fallback
}))

vi.mock('@/utils/localPreview', () => ({
  isLocalPreviewAuth: () => false
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      locale: { value: 'zh' }
    })
  }
})

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn()
  })
}))

function checkinStatus(overrides: Partial<CheckinStatus> = {}): CheckinStatus {
  return {
    date: '2026-07-12',
    free_claimed: false,
    paid_claimed: false,
    paid_cost: 18,
    free_reward_min: 5,
    free_reward_max: 15,
    paid_reward_min: 5,
    paid_reward_max: 100,
    credit_reward_min: 5,
    credit_reward_max: 100,
    balance_reward_min: 1,
    balance_reward_max: 20,
    balance: 0,
    credit_balance: 0,
    credit_count: 50,
    balance_count: 0,
    credit_limit: 50,
    balance_limit: 10,
    credit_cost: 20,
    balance_cost: 5,
    credit_jackpot: 500,
    balance_jackpot: 50,
    milestones: [],
    ...overrides
  }
}

function creditLotterySession(overrides: Partial<CreditLotterySession> = {}): CreditLotterySession {
  return {
    id: 10,
    user_id: 7,
    session_date: '2026-07-12',
    status: 'active',
    cost_credit: 20,
    max_rounds: 3,
    current_round: 2,
    current_result: {
      round_no: 2,
      reward_asset: 'credit',
      reward_amount: 50
    },
    ...overrides
  }
}

function claimResponse(overrides: Partial<CheckinClaimResponse> = {}): CheckinClaimResponse {
  return {
    type: 'free',
    date: '2026-07-19',
    cost: 0,
    reward: 10,
    reward_asset: 'credit',
    balance_after: 12,
    credit_balance_after: 88,
    already_claimed: false,
    ...overrides
  }
}

function collectibleCard(overrides: Partial<CheckinCollectibleCard> = {}): CheckinCollectibleCard {
  return {
    id: 101,
    card_key: 'low_battery_sprite',
    rarity: 'common',
    source_type: 'checkin',
    source_label: '这段来源说明不应挤进缩略卡',
    serial_no: 7,
    edition_no: 7,
    edition_supply: 9999,
    created_at: '2026-07-19T08:00:00Z',
    ...overrides
  }
}

function findSignCard(wrapper: ReturnType<typeof mount>, label: string) {
  const button = wrapper.findAll('.sign-card').find(item => item.text().includes(label))
  expect(button, `sign card ${label}`).toBeTruthy()
  return button!
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

describe('DailyFortuneGift', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    sessionStorage.clear()
    storeMocks.user.id = 7
    storeMocks.user.balance = 0
    storeMocks.user.credit_balance = 0
    apiMocks.getCheckinStatus.mockResolvedValue(checkinStatus())
    apiMocks.getActiveCreditLotterySession.mockResolvedValue(creditLotterySession())
    apiMocks.getCheckinRecords.mockResolvedValue([])
    apiMocks.getCheckinCards.mockResolvedValue([])
    apiMocks.claimCheckin.mockImplementation((_type: string, operationId: string) => Promise.resolve(claimResponse({ operation_id: operationId })))
    apiMocks.getCheckinOperation.mockRejectedValue({ status: 404, code: 'CHECKIN_OPERATION_NOT_FOUND' })
    storeMocks.refreshUser.mockResolvedValue(undefined)
  })

  afterEach(() => {
    vi.useRealTimers()
    sessionStorage.clear()
    document.body.innerHTML = ''
  })

  it('allows restoring an active credit lottery session when credit quota is already exhausted', async () => {
    const wrapper = mount(DailyFortuneGift, {
      attachTo: document.body,
      props: { autoOpen: true }
    })
    await flushPromises()
    await flushPromises()

    const creditButton = findSignCard(wrapper, '消耗：20 积分')
    expect(creditButton.attributes('disabled')).toBeUndefined()

    await creditButton.trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="credit-lottery-three-rounds"]').exists()).toBe(true)
  })

  it('claims a seven-day points gift without advertising or adding a cash reward', async () => {
    const milestone = {
      days: 7, credit_reward: 100, balance_reward: 0,
      eligible: true, claimed: false, claimable: true,
      requires_verified: false, requires_api_usage: false, activation_required: false
    }
    apiMocks.getCheckinStatus.mockResolvedValue(checkinStatus({
      balance: 12, credit_balance: 100, milestones: [milestone]
    }))
    apiMocks.claimCheckinMilestone.mockResolvedValue({
      milestone_days: 7, credit_reward: 100, balance_reward: 0,
      balance_after: 12, credit_balance_after: 200
    })
    const wrapper = mount(DailyFortuneGift, {
      attachTo: document.body,
      props: { autoOpen: true }
    })
    await flushPromises()

    const button = wrapper.find('.milestone-chip')
    expect(button.text()).toContain('7天')
    expect(button.text()).toContain('+100 积分')
    expect(button.attributes('aria-label')).not.toContain('余额 +')
    expect(wrapper.find('.milestone-policy').text()).toContain('不再赠送余额')

    apiMocks.getCheckinStatus.mockResolvedValue(checkinStatus({
      balance: 12, credit_balance: 200, milestones: [{ ...milestone, claimed: true, claimable: false }]
    }))
    await button.trigger('click')
    await flushPromises()

    expect(apiMocks.claimCheckinMilestone).toHaveBeenCalledTimes(1)
    expect(apiMocks.claimCheckinMilestone).toHaveBeenCalledWith(7)
    expect(storeMocks.patchUserBalance).toHaveBeenCalledWith(12, 200)
    expect(storeMocks.showSuccess).toHaveBeenCalledWith('积分 +100')
    expect(wrapper.find('.milestone-chip').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('keeps a previously claimed seven-day gift disabled and preserves the displayed cash balance', async () => {
    apiMocks.getCheckinStatus.mockResolvedValue(checkinStatus({
      balance: 12, credit_balance: 200,
      milestones: [{
        days: 7, credit_reward: 100, balance_reward: 0,
        eligible: true, claimed: true, claimable: false,
        requires_verified: false, requires_api_usage: false, activation_required: false
      }]
    }))
    const wrapper = mount(DailyFortuneGift, {
      attachTo: document.body,
      props: { autoOpen: true }
    })
    await flushPromises()

    expect(wrapper.find('.milestone-chip').attributes('disabled')).toBeDefined()
    expect(wrapper.find('.milestone-chip').text()).toContain('已领')
    expect(storeMocks.patchUserBalance).toHaveBeenCalledWith(12, 200)
    expect(apiMocks.claimCheckinMilestone).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('shows a visible retry state and blocks new draws until today status is confirmed', async () => {
    apiMocks.getCheckinStatus.mockRejectedValue(new Error('status unavailable'))
    const wrapper = mount(DailyFortuneGift, {
      attachTo: document.body,
      props: { autoOpen: true }
    })
    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain('今日状态读取失败')
    expect(wrapper.text()).toContain('重新加载')
    expect(wrapper.findAll('.sign-card')).toHaveLength(0)
    expect(apiMocks.claimCheckin).not.toHaveBeenCalled()

    apiMocks.getCheckinStatus.mockResolvedValue(checkinStatus({ balance: 20 }))
    await wrapper.find('.status-read-state button').trigger('click')
    await flushPromises()

    expect(wrapper.text()).not.toContain('今日状态读取失败')
    expect(findSignCard(wrapper, '消耗：免费').attributes('disabled')).toBeUndefined()
  })

  it('replaces panel ellipses with an explicit error and working retry button', async () => {
    apiMocks.getCheckinCards.mockRejectedValueOnce(new Error('cards unavailable'))
    const wrapper = mount(DailyFortuneGift, {
      attachTo: document.body,
      props: { autoOpen: true }
    })
    await flushPromises()
    await wrapper.find('.cards-corner').trigger('click')
    await flushPromises()

    expect(wrapper.find('.panel-load-error').exists()).toBe(true)
    expect(wrapper.text()).toContain('重新加载')

    apiMocks.getCheckinCards.mockResolvedValueOnce([collectibleCard()])
    await wrapper.find('.panel-load-error button').trigger('click')
    await flushPromises()

    expect(wrapper.find('.panel-load-error').exists()).toBe(false)
    expect(wrapper.find('.collectible-card').text()).toContain('低电量小人')
  })

  it('reveals a server result without waiting for background balance refreshes', async () => {
    const wrapper = mount(DailyFortuneGift, {
      attachTo: document.body,
      props: { autoOpen: true }
    })
    await flushPromises()
    await flushPromises()

    const never = new Promise<never>(() => undefined)
    apiMocks.getCheckinStatus.mockReturnValue(never)
    storeMocks.refreshUser.mockReturnValue(never)
    vi.useFakeTimers()

    await findSignCard(wrapper, '消耗：免费').trigger('click')
    await wrapper.find('.mini-start').trigger('click')
    await vi.advanceTimersByTimeAsync(900)
    await Promise.resolve()

    expect(wrapper.find('.mini-draw.ready').exists()).toBe(true)
    expect(wrapper.text()).toContain('点击翻面')
    expect(storeMocks.refreshUser).not.toHaveBeenCalled()
    vi.useRealTimers()
  })

  it('ignores a status response from the previous account', async () => {
    const pendingStatus = deferred<CheckinStatus>()
    apiMocks.getCheckinStatus.mockReturnValue(pendingStatus.promise)
    const wrapper = mount(DailyFortuneGift, {
      attachTo: document.body,
      props: { autoOpen: true }
    })
    await Promise.resolve()
    storeMocks.patchUserBalance.mockClear()

    storeMocks.user.id = 8
    storeMocks.user.balance = 8
    storeMocks.user.credit_balance = 80
    pendingStatus.resolve(checkinStatus({ balance: 777, credit_balance: 777 }))
    await flushPromises()

    expect(storeMocks.patchUserBalance).not.toHaveBeenCalledWith(777, 777)
    wrapper.unmount()
  })

  it('ignores a status response after the component is unmounted', async () => {
    const pendingStatus = deferred<CheckinStatus>()
    apiMocks.getCheckinStatus.mockReturnValue(pendingStatus.promise)
    const wrapper = mount(DailyFortuneGift, {
      attachTo: document.body,
      props: { autoOpen: true }
    })
    await Promise.resolve()
    storeMocks.patchUserBalance.mockClear()

    wrapper.unmount()
    pendingStatus.resolve(checkinStatus({ balance: 888, credit_balance: 888 }))
    await flushPromises()

    expect(storeMocks.patchUserBalance).not.toHaveBeenCalledWith(888, 888)
  })

  it('maps the same server response to the same displayed card', async () => {
    async function drawCardTitle() {
      const wrapper = mount(DailyFortuneGift, {
        attachTo: document.body,
        props: { autoOpen: true }
      })
      await flushPromises()
      await flushPromises()
      vi.useFakeTimers()
      await findSignCard(wrapper, '消耗：免费').trigger('click')
      await wrapper.find('.mini-start').trigger('click')
      await vi.advanceTimersByTimeAsync(900)
      await Promise.resolve()
      const title = wrapper.find('.wheel-card-title').text()
      wrapper.unmount()
      vi.useRealTimers()
      return title
    }

    const first = await drawCardTitle()
    const second = await drawCardTitle()

    expect(second).toBe(first)
  })

  it('groups duplicate collectibles and keeps long story/source copy out of thumbnails', async () => {
    apiMocks.getCheckinCards.mockResolvedValue([
      collectibleCard(),
      collectibleCard({ id: 102, serial_no: 8, edition_no: 8 })
    ])
    const wrapper = mount(DailyFortuneGift, {
      attachTo: document.body,
      props: { autoOpen: true }
    })
    await flushPromises()
    await wrapper.find('.cards-corner').trigger('click')
    await flushPromises()

    expect(wrapper.findAll('.collectible-card')).toHaveLength(1)
    expect(wrapper.find('.collectible-card').text()).toContain('低电量小人')
    expect(wrapper.find('.collectible-card').text()).toContain('×2')
    expect(wrapper.text()).not.toContain('这段来源说明不应挤进缩略卡')
    expect(wrapper.text()).not.toContain('负责举着一粒小灯泡')
  })

  it('enters a non-repeatable confirmation state after an uncertain request failure', async () => {
    apiMocks.claimCheckin.mockRejectedValue(new Error('network disconnected'))
    const wrapper = mount(DailyFortuneGift, {
      attachTo: document.body,
      props: { autoOpen: true }
    })
    await flushPromises()
    vi.useFakeTimers()

    await findSignCard(wrapper, '消耗：免费').trigger('click')
    await wrapper.find('.mini-start').trigger('click')
    await vi.advanceTimersByTimeAsync(900)
    await Promise.resolve()

    expect(wrapper.find('.mini-draw.uncertain').exists()).toBe(true)
    expect(wrapper.find('.mini-start').text()).toBe('查询原结果')
    expect(wrapper.text()).toContain('不会生成第二笔抽奖或重复扣款')
    expect(apiMocks.claimCheckin).toHaveBeenCalledTimes(1)
    vi.useRealTimers()
  })

  it('retries an explicitly missing draw with the same operation ID after the safety window', async () => {
    apiMocks.claimCheckin.mockRejectedValueOnce(new Error('network disconnected'))
    const wrapper = mount(DailyFortuneGift, {
      attachTo: document.body,
      props: { autoOpen: true }
    })
    await flushPromises()
    vi.useFakeTimers()

    await findSignCard(wrapper, '消耗：免费').trigger('click')
    await wrapper.find('.mini-start').trigger('click')
    await flushPromises()
    const operationId = apiMocks.claimCheckin.mock.calls[0][1]

    await wrapper.find('.mini-start').trigger('click')
    await flushPromises()

    expect(apiMocks.getCheckinOperation).toHaveBeenCalledWith(operationId)
    expect(apiMocks.claimCheckin).toHaveBeenCalledTimes(1)
    expect(wrapper.find('.mini-start').text()).toBe('查询原结果')
    expect(wrapper.text()).toContain('正在等待安全窗口')

    await vi.advanceTimersByTimeAsync(15_000)
    expect(wrapper.find('.mini-start').text()).toBe('重试原操作')

    apiMocks.claimCheckin.mockResolvedValueOnce(claimResponse({ operation_id: operationId }))
    await wrapper.find('.mini-start').trigger('click')
    await flushPromises()

    expect(apiMocks.claimCheckin).toHaveBeenCalledTimes(2)
    expect(apiMocks.claimCheckin.mock.calls[1]).toEqual(['free', operationId])
    expect(sessionStorage.getItem(checkinOperationStorageKey(7))).toBeNull()
    expect(wrapper.find('.mini-draw.ready').exists()).toBe(true)
  })

  it('stops the visual spinner after six seconds but keeps POST retry locked while the original draw is pending', async () => {
    vi.useFakeTimers()
    const originalRequest = deferred<CheckinClaimResponse>()
    apiMocks.claimCheckin.mockReturnValue(originalRequest.promise)
    const wrapper = mount(DailyFortuneGift, {
      attachTo: document.body,
      props: { autoOpen: true }
    })
    await flushPromises()

    await findSignCard(wrapper, '消耗：免费').trigger('click')
    await wrapper.find('.mini-start').trigger('click')
    await vi.advanceTimersByTimeAsync(6000)

    expect(wrapper.find('.mini-draw.uncertain').exists()).toBe(true)
    await wrapper.find('.mini-start').trigger('click')
    await flushPromises()
    await vi.advanceTimersByTimeAsync(15_000)

    expect(apiMocks.claimCheckin).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('原请求仍在等待返回')
    expect(wrapper.find('.mini-start').text()).toBe('查询原结果')

    originalRequest.resolve(claimResponse({ operation_id: apiMocks.claimCheckin.mock.calls[0][1] }))
    await flushPromises()

    expect(wrapper.find('.mini-draw.ready').exists()).toBe(true)
    expect(apiMocks.claimCheckin).toHaveBeenCalledTimes(1)
  })

  it.each([
    { label: 'a 500 response carrying a forged not-found code', error: { response: { status: 500 }, code: 'CHECKIN_OPERATION_NOT_FOUND' } },
    { label: 'a bare route 404', error: { status: 404 } },
    { label: 'an unrelated route 404', error: { status: 404, code: 'ROUTE_NOT_FOUND' } }
  ])('does not enable a retry from an aged pending operation after $label', async ({ error }) => {
    const operationId = 'checkin-free-server-error'
    sessionStorage.setItem(checkinOperationStorageKey(7), JSON.stringify({
      operationId,
      type: 'free',
      createdAt: Date.now() - 60_000
    }))
    apiMocks.getCheckinOperation.mockRejectedValue(error)
    vi.useFakeTimers()

    const wrapper = mount(DailyFortuneGift, {
      attachTo: document.body,
      props: { autoOpen: true }
    })
    await flushPromises()
    await vi.advanceTimersByTimeAsync(60_000)

    expect(wrapper.find('.mini-start').text()).toBe('查询原结果')
    expect(apiMocks.claimCheckin).not.toHaveBeenCalled()
    expect(sessionStorage.getItem(checkinOperationStorageKey(7))).toContain(operationId)
  })

  it.each([
    { label: 'null', payload: null },
    { label: 'an empty object', payload: {} },
    { label: 'an array', payload: [] },
    { label: 'an object missing required fields', payload: { operation_id: 'checkin-free-empty-payload', type: 'free' } }
  ])('keeps a 200 $label operation lookup in query-only mode', async ({ payload }) => {
    const operationId = 'checkin-free-empty-payload'
    sessionStorage.setItem(checkinOperationStorageKey(7), JSON.stringify({
      operationId,
      type: 'free',
      createdAt: Date.now() - 60_000
    }))
    apiMocks.getCheckinOperation.mockResolvedValue(payload)
    vi.useFakeTimers()

    const wrapper = mount(DailyFortuneGift, {
      attachTo: document.body,
      props: { autoOpen: true }
    })
    await flushPromises()
    await vi.advanceTimersByTimeAsync(60_000)

    expect(wrapper.find('.mini-start').text()).toBe('查询原结果')
    expect(wrapper.text()).not.toContain('重试原操作')
    expect(apiMocks.claimCheckin).not.toHaveBeenCalled()
    expect(sessionStorage.getItem(checkinOperationStorageKey(7))).toContain(operationId)
    expect(storeMocks.patchUserBalance.mock.calls.every(([balance, credit]) => Number.isFinite(balance) && Number.isFinite(credit))).toBe(true)
  })

  it.each([
    { label: 'null', payload: () => null },
    { label: 'an empty object', payload: () => ({}) },
    { label: 'an array', payload: () => [] },
    {
      label: 'an object missing a required balance field',
      payload: (operationId: string) => {
        const { balance_after: _balanceAfter, ...incomplete } = claimResponse({ operation_id: operationId })
        return incomplete
      }
    }
  ])('keeps the original marker when a draw POST returns 200 with $label', async ({ payload }) => {
    vi.useFakeTimers()
    apiMocks.claimCheckin.mockImplementation((_type: string, operationId: string) => Promise.resolve(payload(operationId)))

    const wrapper = mount(DailyFortuneGift, {
      attachTo: document.body,
      props: { autoOpen: true }
    })
    await flushPromises()
    storeMocks.patchUserBalance.mockClear()

    await findSignCard(wrapper, '消耗：免费').trigger('click')
    await wrapper.find('.mini-start').trigger('click')
    await vi.advanceTimersByTimeAsync(900)
    await flushPromises()

    const operationId = apiMocks.claimCheckin.mock.calls[0][1]
    expect(wrapper.find('.mini-draw.uncertain').exists()).toBe(true)
    expect(sessionStorage.getItem(checkinOperationStorageKey(7))).toContain(operationId)
    expect(apiMocks.claimCheckin).toHaveBeenCalledTimes(1)
    expect(storeMocks.patchUserBalance).not.toHaveBeenCalled()
  })

  it('ignores a late POST after an operation lookup has already restored the result', async () => {
    vi.useFakeTimers()
    const originalRequest = deferred<CheckinClaimResponse>()
    apiMocks.claimCheckin.mockReturnValue(originalRequest.promise)
    apiMocks.getCheckinOperation.mockImplementation((operationId: string) => Promise.resolve(claimResponse({
      operation_id: operationId,
      balance_after: 40,
      credit_balance_after: 90
    })))

    const wrapper = mount(DailyFortuneGift, {
      attachTo: document.body,
      props: { autoOpen: true }
    })
    await flushPromises()
    await findSignCard(wrapper, '消耗：免费').trigger('click')
    await wrapper.find('.mini-start').trigger('click')
    await vi.advanceTimersByTimeAsync(6000)

    await wrapper.find('.mini-start').trigger('click')
    await flushPromises()
    expect(wrapper.find('.mini-draw.ready').exists()).toBe(true)

    originalRequest.resolve(claimResponse({
      operation_id: apiMocks.claimCheckin.mock.calls[0][1],
      balance_after: 999,
      credit_balance_after: 999
    }))
    await flushPromises()

    expect(storeMocks.patchUserBalance).not.toHaveBeenCalledWith(999, 999)
    expect(wrapper.find('.mini-draw.ready').exists()).toBe(true)
    expect(sessionStorage.getItem(checkinOperationStorageKey(7))).toBeNull()
  })

  it('keeps the original user recovery marker and ignores a deferred POST after unmount and account switch', async () => {
    vi.useFakeTimers()
    const originalRequest = deferred<CheckinClaimResponse>()
    apiMocks.claimCheckin.mockReturnValue(originalRequest.promise)

    const wrapper = mount(DailyFortuneGift, {
      attachTo: document.body,
      props: { autoOpen: true }
    })
    await flushPromises()
    await findSignCard(wrapper, '消耗：免费').trigger('click')
    await wrapper.find('.mini-start').trigger('click')

    const operationId = apiMocks.claimCheckin.mock.calls[0][1]
    expect(sessionStorage.getItem(checkinOperationStorageKey(7))).toContain(operationId)
    storeMocks.patchUserBalance.mockClear()

    wrapper.unmount()
    storeMocks.user.id = 8
    storeMocks.user.balance = 8
    storeMocks.user.credit_balance = 80
    originalRequest.resolve(claimResponse({
      operation_id: operationId,
      balance_after: 777,
      credit_balance_after: 777
    }))
    await vi.advanceTimersByTimeAsync(900)
    await flushPromises()

    expect(storeMocks.patchUserBalance).not.toHaveBeenCalled()
    expect(sessionStorage.getItem(checkinOperationStorageKey(7))).toContain(operationId)
    expect(sessionStorage.getItem(checkinOperationStorageKey(8))).toBeNull()
  })

  it.each([
    { label: '消耗：免费', type: 'free' as const, status: checkinStatus() },
    { label: '消耗：5 余额', type: 'balance' as const, status: checkinStatus({ balance: 20 }) }
  ])('recovers the original $type result after a network failure without a second POST', async ({ label, type, status }) => {
    apiMocks.getCheckinStatus.mockResolvedValue(status)
    apiMocks.claimCheckin.mockRejectedValue(new Error('network disconnected'))
    apiMocks.getCheckinOperation.mockImplementation((operationId: string) => Promise.resolve(claimResponse({
      operation_id: operationId,
      type,
      reward_asset: type === 'balance' ? 'balance' : 'credit',
      reward: type === 'balance' ? 2 : 10
    })))

    const wrapper = mount(DailyFortuneGift, {
      attachTo: document.body,
      props: { autoOpen: true }
    })
    await flushPromises()
    vi.useFakeTimers()

    await findSignCard(wrapper, label).trigger('click')
    await wrapper.find('.mini-start').trigger('click')
    await vi.advanceTimersByTimeAsync(900)
    await Promise.resolve()

    const operationId = apiMocks.claimCheckin.mock.calls[0][1]
    expect(operationId).toMatch(new RegExp(`^checkin-${type}-`))
    expect(sessionStorage.getItem(checkinOperationStorageKey(7))).toContain(operationId)

    await wrapper.find('.mini-start').trigger('click')
    await flushPromises()

    expect(apiMocks.getCheckinOperation).toHaveBeenCalledWith(operationId)
    expect(apiMocks.claimCheckin).toHaveBeenCalledTimes(1)
    expect(wrapper.find('.mini-draw.ready').exists()).toBe(true)
    expect(sessionStorage.getItem(checkinOperationStorageKey(7))).toBeNull()
  })

  it('restores a pending balance draw after refresh by GET and never repeats the claim request', async () => {
    const operationId = 'checkin-balance-refresh-1'
    sessionStorage.setItem(checkinOperationStorageKey(7), JSON.stringify({
      operationId,
      type: 'balance',
      createdAt: Date.now()
    }))
    apiMocks.getCheckinStatus.mockResolvedValue(checkinStatus({ balance: 20 }))
    apiMocks.getCheckinOperation.mockResolvedValue(claimResponse({
      operation_id: operationId,
      type: 'balance',
      reward_asset: 'balance',
      reward: 5
    }))

    const wrapper = mount(DailyFortuneGift, {
      attachTo: document.body,
      props: { autoOpen: true }
    })
    await flushPromises()
    await flushPromises()

    expect(apiMocks.getCheckinOperation).toHaveBeenCalledWith(operationId)
    expect(apiMocks.claimCheckin).not.toHaveBeenCalled()
    expect(wrapper.find('.mini-draw.ready').exists()).toBe(true)
    expect(wrapper.text()).toContain('点击翻面')
    expect(sessionStorage.getItem(checkinOperationStorageKey(7))).toBeNull()
  })
})
