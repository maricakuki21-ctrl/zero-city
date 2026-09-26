import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import CreditLotteryThreeRounds from '../CreditLotteryThreeRounds.vue'
import type { CreditLotterySession } from '@/types'
import { creditLotteryOperationStorageKey } from '@/utils/lotteryOperationRecovery'

const apiMocks = vi.hoisted(() => ({
  getActiveCreditLotterySession: vi.fn(),
  getCreditLotteryOperation: vi.fn(),
  createCreditLotterySession: vi.fn(),
  continueCreditLotterySession: vi.fn(),
  settleCreditLotterySession: vi.fn()
}))

const storeMocks = vi.hoisted(() => ({
  showError: vi.fn(),
  patchUserBalance: vi.fn(),
  user: { id: 3, balance: 0, credit_balance: 0 }
}))

vi.mock('@/api/user', () => ({
  default: apiMocks
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError: storeMocks.showError
  }),
  useAuthStore: () => ({
    user: storeMocks.user,
    patchUserBalance: storeMocks.patchUserBalance
  })
}))

vi.mock('@/utils/apiError', () => ({
  extractApiErrorMessage: (_error: unknown, fallback: string) => fallback
}))

function session(status: CreditLotterySession['status'], overrides: Partial<CreditLotterySession> = {}): CreditLotterySession {
  const result = {
    round_no: 1,
    reward_asset: 'credit',
    reward_amount: 10
  }

  return {
    id: 7,
    user_id: 3,
    mode: status === 'active' ? 'three_round' : 'single',
    session_date: '2026-07-11',
    status,
    cost_credit: 20,
    max_rounds: 3,
    current_round: 1,
    credit_balance_after: 80,
    current_result: status === 'active' ? result : undefined,
    final_result: status === 'settled' ? result : undefined,
    ...overrides
  }
}

function findButton(wrapper: ReturnType<typeof mount>, text: string) {
  const button = wrapper.findAll('button').find(item => item.text() === text)
  expect(button, `button ${text}`).toBeTruthy()
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

describe('CreditLotteryThreeRounds', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    sessionStorage.clear()
    storeMocks.user.id = 3
    storeMocks.user.balance = 0
    storeMocks.user.credit_balance = 0
    apiMocks.getCreditLotteryOperation.mockRejectedValue({ status: 404, code: 'CREDIT_LOTTERY_OPERATION_NOT_FOUND' })
  })

  afterEach(() => {
    vi.useRealTimers()
    sessionStorage.clear()
  })

  it('keeps the card deck rolling briefly before showing a new result', async () => {
    vi.useFakeTimers()
    apiMocks.getActiveCreditLotterySession.mockResolvedValue(null)
    apiMocks.createCreditLotterySession.mockResolvedValue(session('active'))

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()

    await findButton(wrapper, '点击开始').trigger('click')
    await flushPromises()

    expect(wrapper.classes()).toContain('spinning')
    expect(wrapper.text()).toContain('卡片旋转中')
    expect(wrapper.text()).not.toContain('当前签牌')

    await vi.advanceTimersByTimeAsync(1199)
    await flushPromises()
    expect(wrapper.classes()).toContain('spinning')

    await vi.advanceTimersByTimeAsync(1)
    await flushPromises()

    expect(wrapper.classes()).toContain('ready')
    expect(wrapper.text()).toContain('点击翻开')
    expect(wrapper.text()).not.toContain('当前签牌')

    await wrapper.find('.mini-slot').trigger('click')
    await flushPromises()

    expect(wrapper.classes()).toContain('revealed')
    expect(wrapper.text()).toContain('当前签牌')
    expect(wrapper.find('.wheel-card-heading b').text()).not.toBe('')
    expect(wrapper.find('.wheel-card-footer').text()).toContain('积分 +10')
    expect(wrapper.find('.rarity-badge').text()).toBe('进阶')
  })

  it('resets to the ready state after settling with stop', async () => {
    apiMocks.getActiveCreditLotterySession.mockResolvedValue(session('active'))
    apiMocks.settleCreditLotterySession.mockResolvedValue(session('settled'))

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()

    await wrapper.find('.mini-slot').trigger('click')
    await flushPromises()

    await findButton(wrapper, '收下本轮').trigger('click')
    await flushPromises()

    expect(apiMocks.settleCreditLotterySession).toHaveBeenCalledWith(7, 1, expect.stringMatching(/^settle-/))
    expect(wrapper.text()).toContain('点击开始')
    expect(wrapper.text()).not.toContain('完成')
    expect(wrapper.emitted('refresh')).toHaveLength(1)
  })

  it('accepts a newer active authority snapshot when settle uses a stale expected round', async () => {
    apiMocks.getActiveCreditLotterySession.mockResolvedValue(session('active'))
    apiMocks.settleCreditLotterySession.mockResolvedValue(session('active', {
      current_round: 2,
      current_result: { round_no: 2, reward_asset: 'credit', reward_amount: 20 }
    }))

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()
    await wrapper.find('.mini-slot').trigger('click')
    await findButton(wrapper, '收下本轮').trigger('click')
    await flushPromises()

    const operationId = apiMocks.settleCreditLotterySession.mock.calls[0][2]
    expect(apiMocks.settleCreditLotterySession).toHaveBeenCalledWith(7, 1, operationId)
    expect(apiMocks.settleCreditLotterySession).toHaveBeenCalledTimes(1)
    expect(apiMocks.getCreditLotteryOperation).not.toHaveBeenCalled()
    expect(sessionStorage.getItem(creditLotteryOperationStorageKey(3))).toBeNull()
    expect(wrapper.classes()).toContain('revealed')
    expect(wrapper.text()).toContain('第 2/3 轮')
    expect(wrapper.text()).toContain('积分 +20')
    expect(findButton(wrapper, '收下本轮').exists()).toBe(true)
  })

  it('keeps the marker when settle returns active without a newer authority round', async () => {
    apiMocks.getActiveCreditLotterySession.mockResolvedValue(session('active'))
    apiMocks.settleCreditLotterySession.mockResolvedValue(session('active'))

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()
    await wrapper.find('.mini-slot').trigger('click')
    await findButton(wrapper, '收下本轮').trigger('click')
    await flushPromises()

    const operationId = apiMocks.settleCreditLotterySession.mock.calls[0][2]
    expect(apiMocks.getCreditLotteryOperation).toHaveBeenCalledWith(operationId)
    expect(sessionStorage.getItem(creditLotteryOperationStorageKey(3))).toContain(operationId)
    expect(wrapper.classes()).toContain('confirming')
  })

  it('reveals a single draw as a settled result without continue controls', async () => {
    vi.useFakeTimers()
    apiMocks.getActiveCreditLotterySession.mockResolvedValue(null)
    apiMocks.createCreditLotterySession.mockResolvedValue(session('settled', {
      mode: 'single',
      max_rounds: 1,
      settlement_reason: 'single_draw'
    }))

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()

    await findButton(wrapper, '点击开始').trigger('click')
    await flushPromises()

    expect(apiMocks.createCreditLotterySession).toHaveBeenCalledWith(expect.stringMatching(/^start-/))

    await vi.advanceTimersByTimeAsync(1200)
    await flushPromises()

    expect(wrapper.text()).toContain('点击翻开')
    expect(wrapper.text()).toContain('单抽')

    await wrapper.find('.mini-slot').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('本次签牌')
    expect(wrapper.text()).toContain('完成')
    expect(wrapper.text()).not.toContain('下一轮')
    expect(wrapper.text()).not.toContain('收下本轮')
  })

  it('resets local state before closing from a settled result', async () => {
    apiMocks.getActiveCreditLotterySession.mockResolvedValue(session('settled'))

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()

    await findButton(wrapper, '完成').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('点击开始')
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('shows both credit and balance jackpot payouts', async () => {
    apiMocks.getActiveCreditLotterySession.mockResolvedValue(session('settled', {
      jackpot_hit: true,
      jackpot_payouts: [
        {
          pool_type: 'credit',
          reward_asset: 'credit',
          winner_amount: 300,
          celebration_amount: 50,
          celebration_actual_amount: 0,
          celebration_user_count: 0
        },
        {
          pool_type: 'balance',
          reward_asset: 'balance',
          winner_amount: 40,
          celebration_amount: 5,
          celebration_actual_amount: 0,
          celebration_user_count: 0
        }
      ]
    }))

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()

    expect(wrapper.text()).toContain('Jackpot 积分 +300.00 / 余额 +40.00')
  })

  it('maps the same session result to the same card and uses the formal collectible name', async () => {
    const activeSession = session('active', {
      current_round: 2,
      current_result: {
        round_no: 2,
        reward_asset: 'credit',
        reward_amount: 20,
        collectible_candidate: {
          id: 91,
          card_key: 'instant_noodle_prophet',
          rarity: 'common',
          source_type: 'credit',
          source_label: 'credit',
          serial_no: 91,
          created_at: '2026-07-19T00:00:00Z'
        }
      }
    })
    apiMocks.getActiveCreditLotterySession.mockResolvedValue(activeSession)

    const first = mount(CreditLotteryThreeRounds)
    const second = mount(CreditLotteryThreeRounds)
    await flushPromises()
    await first.find('.mini-slot').trigger('click')
    await second.find('.mini-slot').trigger('click')

    expect(first.find('.wheel-card-heading b').text()).toBe(second.find('.wheel-card-heading b').text())
    expect(first.text()).toContain('泡面预言家')
  })

  it('stops the endless spinner and restores a server-side session after six seconds', async () => {
    vi.useFakeTimers()
    const never = new Promise<CreditLotterySession>(() => undefined)
    apiMocks.getActiveCreditLotterySession
      .mockResolvedValueOnce(null)
    apiMocks.getCreditLotteryOperation.mockResolvedValue(session('active'))
    apiMocks.createCreditLotterySession.mockReturnValue(never)

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()
    await findButton(wrapper, '点击开始').trigger('click')
    await vi.advanceTimersByTimeAsync(6000)
    await Promise.resolve()

    expect(wrapper.classes()).toContain('confirming')
    expect(wrapper.classes()).not.toContain('spinning')
    expect(wrapper.text()).toContain('请勿重复抽取')

    await findButton(wrapper, '刷新结果').trigger('click')
    await flushPromises()

    const operationId = apiMocks.createCreditLotterySession.mock.calls[0][0]
    expect(wrapper.classes()).toContain('ready')
    expect(wrapper.text()).toContain('点击翻开')
    expect(apiMocks.createCreditLotterySession).toHaveBeenCalledTimes(1)
    expect(apiMocks.getCreditLotteryOperation).toHaveBeenCalledWith(operationId)
    expect(sessionStorage.getItem(creditLotteryOperationStorageKey(3))).toBeNull()
  })

  it('offers read-only recovery when the initial session load hangs', async () => {
    vi.useFakeTimers()
    apiMocks.getActiveCreditLotterySession.mockReturnValue(new Promise(() => undefined))

    const wrapper = mount(CreditLotteryThreeRounds)
    await vi.advanceTimersByTimeAsync(6000)
    await Promise.resolve()

    expect(wrapper.classes()).toContain('confirming')
    expect(wrapper.text()).toContain('刷新结果')
    expect(wrapper.text()).not.toContain('加载中')
  })

  it('shows reload after the initial session lookup fails beyond the confirmation timeout', async () => {
    vi.useFakeTimers()
    let rejectLookup!: (reason: unknown) => void
    apiMocks.getActiveCreditLotterySession.mockReturnValue(new Promise((_resolve, reject) => {
      rejectLookup = reject
    }))

    const wrapper = mount(CreditLotteryThreeRounds)
    await vi.advanceTimersByTimeAsync(6000)
    await Promise.resolve()

    expect(wrapper.classes()).toContain('confirming')

    rejectLookup({ response: { status: 503 } })
    await flushPromises()

    expect(wrapper.classes()).not.toContain('confirming')
    expect(wrapper.text()).toContain('抽奖状态读取失败')
    expect(wrapper.text()).toContain('重新加载')
    expect(wrapper.text()).not.toContain('请勿重复抽取')
  })

  it('leaves the confirmation screen after a read-only refresh confirms there is no active session', async () => {
    vi.useFakeTimers()
    apiMocks.getActiveCreditLotterySession
      .mockReturnValueOnce(new Promise(() => undefined))
      .mockResolvedValueOnce(null)

    const wrapper = mount(CreditLotteryThreeRounds)
    await vi.advanceTimersByTimeAsync(6000)
    await Promise.resolve()

    await findButton(wrapper, '刷新结果').trigger('click')
    await flushPromises()

    expect(wrapper.classes()).toContain('idle')
    expect(wrapper.text()).toContain('点击开始')
    expect(wrapper.text()).not.toContain('请勿重复抽取')
  })

  it('clears a rejected operation after a definite client error instead of locking the draw button', async () => {
    apiMocks.getActiveCreditLotterySession.mockResolvedValue(null)
    apiMocks.createCreditLotterySession.mockRejectedValue({ response: { status: 400 } })

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()

    await findButton(wrapper, '点击开始').trigger('click')
    await flushPromises()

    expect(apiMocks.getCreditLotteryOperation).not.toHaveBeenCalled()
    expect(sessionStorage.getItem(creditLotteryOperationStorageKey(3))).toBeNull()
    expect(findButton(wrapper, '点击开始').attributes('disabled')).toBeUndefined()
    expect(wrapper.text()).not.toContain('请勿重复抽取')
  })

  it.each([
    { action: 'start' as const, label: 'null', payload: null },
    { action: 'start' as const, label: 'an empty object', payload: {} },
    { action: 'continue' as const, label: 'null', payload: null },
    { action: 'continue' as const, label: 'an empty object', payload: {} },
    { action: 'settle' as const, label: 'null', payload: null },
    { action: 'settle' as const, label: 'an empty object', payload: {} }
  ])('keeps the original marker when $action POST returns 200 with $label', async ({ action, payload }) => {
    vi.useFakeTimers()
    apiMocks.getActiveCreditLotterySession.mockResolvedValue(action === 'start' ? null : session('active'))
    if (action === 'start') apiMocks.createCreditLotterySession.mockResolvedValue(payload)
    if (action === 'continue') apiMocks.continueCreditLotterySession.mockResolvedValue(payload)
    if (action === 'settle') apiMocks.settleCreditLotterySession.mockResolvedValue(payload)

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()
    storeMocks.patchUserBalance.mockClear()

    if (action === 'start') {
      await findButton(wrapper, '点击开始').trigger('click')
    } else {
      await wrapper.find('.mini-slot').trigger('click')
      if (action === 'continue') {
        const continueButton = wrapper.findAll('button').find(button => button.text().startsWith('继续抽'))
        expect(continueButton).toBeTruthy()
        await continueButton!.trigger('click')
      } else {
        await findButton(wrapper, '收下本轮').trigger('click')
      }
    }
    if (action !== 'settle') await vi.advanceTimersByTimeAsync(1200)
    await flushPromises()

    const operationId = action === 'start'
      ? apiMocks.createCreditLotterySession.mock.calls[0][0]
      : action === 'continue'
        ? apiMocks.continueCreditLotterySession.mock.calls[0][2]
        : apiMocks.settleCreditLotterySession.mock.calls[0][2]
    expect(wrapper.classes()).toContain('confirming')
    expect(sessionStorage.getItem(creditLotteryOperationStorageKey(3))).toContain(operationId)
    expect(apiMocks.getCreditLotteryOperation).toHaveBeenCalledWith(operationId)
    expect(storeMocks.patchUserBalance).not.toHaveBeenCalled()
  })

  it('retries an explicitly missing start with the same operation ID after the safety window', async () => {
    vi.useFakeTimers()
    apiMocks.getActiveCreditLotterySession.mockResolvedValue(null)
    apiMocks.createCreditLotterySession
      .mockRejectedValueOnce(new Error('network disconnected'))
      .mockResolvedValueOnce(session('active'))

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()
    await findButton(wrapper, '点击开始').trigger('click')
    await flushPromises()

    const operationId = apiMocks.createCreditLotterySession.mock.calls[0][0]
    expect(apiMocks.getCreditLotteryOperation).toHaveBeenCalledWith(operationId)
    expect(findButton(wrapper, '刷新结果').exists()).toBe(true)
    expect(wrapper.text()).toContain('正在等待安全窗口')

    await vi.advanceTimersByTimeAsync(15_000)
    await findButton(wrapper, '重试原操作').trigger('click')
    await flushPromises()

    expect(apiMocks.createCreditLotterySession).toHaveBeenCalledTimes(2)
    expect(apiMocks.createCreditLotterySession.mock.calls[1][0]).toBe(operationId)
    expect(sessionStorage.getItem(creditLotteryOperationStorageKey(3))).toBeNull()
    expect(wrapper.classes()).toContain('ready')
  })

  it('allows lookup after six seconds but never retries while the original POST is still in flight', async () => {
    vi.useFakeTimers()
    const originalRequest = deferred<CreditLotterySession>()
    apiMocks.getActiveCreditLotterySession.mockResolvedValue(null)
    apiMocks.createCreditLotterySession.mockReturnValue(originalRequest.promise)

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()
    await findButton(wrapper, '点击开始').trigger('click')
    await vi.advanceTimersByTimeAsync(6000)

    await findButton(wrapper, '刷新结果').trigger('click')
    await flushPromises()
    await vi.advanceTimersByTimeAsync(15_000)

    expect(apiMocks.createCreditLotterySession).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('原请求仍在等待返回')
    expect(wrapper.text()).not.toContain('重试原操作')

    originalRequest.resolve(session('active'))
    await flushPromises()

    expect(wrapper.classes()).toContain('ready')
    expect(apiMocks.createCreditLotterySession).toHaveBeenCalledTimes(1)
    expect(sessionStorage.getItem(creditLotteryOperationStorageKey(3))).toBeNull()
  })

  it('ignores a late POST response after operation lookup has already restored a newer snapshot', async () => {
    vi.useFakeTimers()
    const originalRequest = deferred<CreditLotterySession>()
    apiMocks.getActiveCreditLotterySession.mockResolvedValue(null)
    apiMocks.createCreditLotterySession.mockReturnValue(originalRequest.promise)
    apiMocks.getCreditLotteryOperation.mockResolvedValue(session('active', {
      current_round: 2,
      current_result: { round_no: 2, reward_asset: 'credit', reward_amount: 20 }
    }))

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()
    await findButton(wrapper, '点击开始').trigger('click')
    await vi.advanceTimersByTimeAsync(6000)
    await findButton(wrapper, '刷新结果').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('第 2/3 轮')

    originalRequest.resolve(session('active', {
      current_round: 1,
      current_result: { round_no: 1, reward_asset: 'credit', reward_amount: 5 }
    }))
    await flushPromises()

    expect(wrapper.text()).toContain('第 2/3 轮')
    expect(wrapper.text()).not.toContain('第 1/3 轮')
  })

  it.each([
    { label: 'a 500 response carrying a forged not-found code', error: { response: { status: 500 }, code: 'CREDIT_LOTTERY_OPERATION_NOT_FOUND' } },
    { label: 'a bare route 404', error: { status: 404 } },
    { label: 'an unrelated route 404', error: { status: 404, code: 'ROUTE_NOT_FOUND' } }
  ])('does not enable a retry for an aged operation after $label', async ({ error }) => {
    const operationId = 'start-server-error'
    sessionStorage.setItem(creditLotteryOperationStorageKey(3), JSON.stringify({
      operationId,
      action: 'start',
      createdAt: Date.now() - 60_000
    }))
    apiMocks.getCreditLotteryOperation.mockRejectedValue(error)
    vi.useFakeTimers()

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()
    await vi.advanceTimersByTimeAsync(60_000)

    expect(findButton(wrapper, '刷新结果').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('重试原操作')
    expect(apiMocks.createCreditLotterySession).not.toHaveBeenCalled()
    expect(sessionStorage.getItem(creditLotteryOperationStorageKey(3))).toContain(operationId)
  })

  it('keeps the original user recovery marker and ignores a deferred POST after unmount and account switch', async () => {
    vi.useFakeTimers()
    const originalRequest = deferred<CreditLotterySession>()
    apiMocks.getActiveCreditLotterySession.mockResolvedValue(null)
    apiMocks.createCreditLotterySession.mockReturnValue(originalRequest.promise)

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()
    await findButton(wrapper, '点击开始').trigger('click')

    const operationId = apiMocks.createCreditLotterySession.mock.calls[0][0]
    expect(sessionStorage.getItem(creditLotteryOperationStorageKey(3))).toContain(operationId)
    storeMocks.patchUserBalance.mockClear()

    wrapper.unmount()
    storeMocks.user.id = 4
    storeMocks.user.balance = 4
    storeMocks.user.credit_balance = 40
    originalRequest.resolve(session('active', {
      user_id: 3,
      balance_after: 777,
      credit_balance_after: 777
    }))
    await vi.advanceTimersByTimeAsync(1200)
    await flushPromises()

    expect(storeMocks.patchUserBalance).not.toHaveBeenCalled()
    expect(sessionStorage.getItem(creditLotteryOperationStorageKey(3))).toContain(operationId)
    expect(sessionStorage.getItem(creditLotteryOperationStorageKey(4))).toBeNull()
  })

  it('keeps query-only recovery when an operation endpoint returns 200 with an empty payload', async () => {
    const operationId = 'start-empty-payload'
    sessionStorage.setItem(creditLotteryOperationStorageKey(3), JSON.stringify({
      operationId,
      action: 'start',
      createdAt: Date.now() - 60_000
    }))
    apiMocks.getCreditLotteryOperation.mockResolvedValue(null)
    vi.useFakeTimers()

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()
    await vi.advanceTimersByTimeAsync(60_000)

    expect(findButton(wrapper, '刷新结果').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('重试原操作')
    expect(apiMocks.createCreditLotterySession).not.toHaveBeenCalled()
  })

  it.each([
    { action: 'continue' as const, api: 'continue' as const, status: 'active' as const },
    { action: 'settle' as const, api: 'settle' as const, status: 'settled' as const }
  ])('retries an explicitly missing $action with the same operation ID', async ({ action, api, status }) => {
    const operationId = `${action}-missing-operation`
    sessionStorage.setItem(creditLotteryOperationStorageKey(3), JSON.stringify({
      operationId,
      action,
      sessionId: 7,
      expectedRound: 1,
      createdAt: Date.now() - 60_000
    }))
    apiMocks.getCreditLotteryOperation.mockRejectedValue({
      status: 404,
      code: 'CREDIT_LOTTERY_OPERATION_NOT_FOUND'
    })
    apiMocks.continueCreditLotterySession.mockResolvedValue(session(status))
    apiMocks.settleCreditLotterySession.mockResolvedValue(session(status))

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()
    await findButton(wrapper, '重试原操作').trigger('click')
    await flushPromises()

    if (api === 'continue') {
      expect(apiMocks.continueCreditLotterySession).toHaveBeenCalledWith(7, 1, operationId)
      expect(apiMocks.settleCreditLotterySession).not.toHaveBeenCalled()
    } else {
      expect(apiMocks.settleCreditLotterySession).toHaveBeenCalledWith(7, 1, operationId)
      expect(apiMocks.continueCreditLotterySession).not.toHaveBeenCalled()
    }
    expect(sessionStorage.getItem(creditLotteryOperationStorageKey(3))).toBeNull()
  })

  it('accepts a newer active authority snapshot when retrying the original stale settle', async () => {
    const operationId = 'settle-stale-authority-retry'
    sessionStorage.setItem(creditLotteryOperationStorageKey(3), JSON.stringify({
      operationId,
      action: 'settle',
      sessionId: 7,
      expectedRound: 1,
      createdAt: Date.now() - 60_000
    }))
    apiMocks.getCreditLotteryOperation.mockRejectedValue({
      status: 404,
      code: 'CREDIT_LOTTERY_OPERATION_NOT_FOUND'
    })
    apiMocks.settleCreditLotterySession.mockResolvedValue(session('active', {
      current_round: 2,
      current_result: { round_no: 2, reward_asset: 'credit', reward_amount: 20 }
    }))

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()
    expect(apiMocks.getCreditLotteryOperation).toHaveBeenCalledTimes(1)

    await findButton(wrapper, '重试原操作').trigger('click')
    await flushPromises()

    expect(apiMocks.settleCreditLotterySession).toHaveBeenCalledWith(7, 1, operationId)
    expect(apiMocks.settleCreditLotterySession).toHaveBeenCalledTimes(1)
    expect(apiMocks.getCreditLotteryOperation).toHaveBeenCalledTimes(1)
    expect(sessionStorage.getItem(creditLotteryOperationStorageKey(3))).toBeNull()
    expect(wrapper.classes()).toContain('revealed')
    expect(wrapper.text()).toContain('第 2/3 轮')
    expect(findButton(wrapper, '收下本轮').exists()).toBe(true)
  })

  it('blocks a new draw when the initial session lookup fails until the user reloads it', async () => {
    apiMocks.getActiveCreditLotterySession
      .mockRejectedValueOnce({ response: { status: 503 } })
      .mockResolvedValueOnce(null)

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()

    expect(wrapper.text()).toContain('抽奖状态读取失败')
    expect(wrapper.findAll('button').some(button => button.text() === '点击开始')).toBe(false)

    await findButton(wrapper, '重新加载').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('点击开始')
  })

  it.each([
    { action: 'start', status: 'active' as const },
    { action: 'continue', status: 'active' as const },
    { action: 'settle', status: 'settled' as const }
  ])('restores a pending $action operation first after refresh', async ({ action, status }) => {
    const operationId = `${action}-refresh-operation`
    sessionStorage.setItem(creditLotteryOperationStorageKey(3), JSON.stringify({
      operationId,
      action,
      sessionId: action === 'start' ? undefined : 7,
      expectedRound: action === 'start' ? undefined : 1,
      createdAt: Date.now()
    }))
    apiMocks.getCreditLotteryOperation.mockResolvedValue(session(status))
    apiMocks.getActiveCreditLotterySession.mockResolvedValue(null)

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()

    expect(apiMocks.getCreditLotteryOperation).toHaveBeenCalledWith(operationId)
    expect(apiMocks.getActiveCreditLotterySession).not.toHaveBeenCalled()
    expect(apiMocks.createCreditLotterySession).not.toHaveBeenCalled()
    expect(apiMocks.continueCreditLotterySession).not.toHaveBeenCalled()
    expect(apiMocks.settleCreditLotterySession).not.toHaveBeenCalled()
    expect(sessionStorage.getItem(creditLotteryOperationStorageKey(3))).toBeNull()
    expect(wrapper.classes()).toContain(status === 'settled' ? 'revealed' : 'ready')
  })

  it('recovers a failed continue action by its exact operation ID without another continue request', async () => {
    apiMocks.getActiveCreditLotterySession.mockResolvedValue(session('active'))
    apiMocks.continueCreditLotterySession.mockRejectedValue(new Error('network disconnected'))
    apiMocks.getCreditLotteryOperation.mockResolvedValue(session('active', {
      current_round: 2,
      current_result: { round_no: 2, reward_asset: 'credit', reward_amount: 20 }
    }))

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()
    await wrapper.find('.mini-slot').trigger('click')
    const continueButton = wrapper.findAll('button').find(button => button.text().startsWith('继续抽'))
    expect(continueButton).toBeTruthy()
    await continueButton!.trigger('click')
    await flushPromises()

    const operationId = apiMocks.continueCreditLotterySession.mock.calls[0][2]
    expect(operationId).toMatch(/^continue-/)
    expect(apiMocks.getCreditLotteryOperation).toHaveBeenCalledWith(operationId)
    expect(apiMocks.continueCreditLotterySession).toHaveBeenCalledTimes(1)
    expect(sessionStorage.getItem(creditLotteryOperationStorageKey(3))).toBeNull()
    expect(wrapper.text()).toContain('点击翻开')
  })

  it('recovers a failed settle action by its exact operation ID without another settle request', async () => {
    apiMocks.getActiveCreditLotterySession.mockResolvedValue(session('active'))
    apiMocks.settleCreditLotterySession.mockRejectedValue(new Error('network disconnected'))
    apiMocks.getCreditLotteryOperation.mockResolvedValue(session('settled'))

    const wrapper = mount(CreditLotteryThreeRounds)
    await flushPromises()
    await wrapper.find('.mini-slot').trigger('click')
    await findButton(wrapper, '收下本轮').trigger('click')
    await flushPromises()

    const operationId = apiMocks.settleCreditLotterySession.mock.calls[0][2]
    expect(operationId).toMatch(/^settle-/)
    expect(apiMocks.getCreditLotteryOperation).toHaveBeenCalledWith(operationId)
    expect(apiMocks.settleCreditLotterySession).toHaveBeenCalledTimes(1)
    expect(sessionStorage.getItem(creditLotteryOperationStorageKey(3))).toBeNull()
    expect(wrapper.text()).toContain('完成')
  })
})
