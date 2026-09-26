import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post }
}))

import {
  claimCheckin,
  continueCreditLotterySession,
  createCreditLotterySession,
  getCheckinOperation,
  getCreditLotteryOperation,
  settleCreditLotterySession
} from '@/api/user'

describe('user lottery operation recovery API', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    get.mockResolvedValue({ data: {} })
    post.mockResolvedValue({ data: {} })
  })

  it('sends the same check-in operation ID in the body and idempotency header', async () => {
    await claimCheckin('balance', 'checkin-balance-operation')

    expect(post).toHaveBeenCalledWith('/user/checkin/claim', {
      type: 'balance',
      operation_id: 'checkin-balance-operation'
    }, {
      headers: { 'Idempotency-Key': 'checkin-balance-operation' }
    })
  })

  it('queries check-in and credit results by an encoded operation ID', async () => {
    await getCheckinOperation('checkin/op 1')
    await getCreditLotteryOperation('credit/op 1')

    expect(get).toHaveBeenNthCalledWith(1, '/user/checkin/operations/checkin%2Fop%201')
    expect(get).toHaveBeenNthCalledWith(2, '/user/credit-lottery/operations/credit%2Fop%201')
  })

  it.each([
    {
      action: 'start',
      invoke: () => createCreditLotterySession('start-operation'),
      path: '/user/credit-lottery/sessions',
      body: { operation_id: 'start-operation', idempotency_key: 'start-operation' },
      operationId: 'start-operation'
    },
    {
      action: 'continue',
      invoke: () => continueCreditLotterySession(7, 1, 'continue-operation'),
      path: '/user/credit-lottery/sessions/7/continue',
      body: { expected_round: 1, operation_id: 'continue-operation', idempotency_key: 'continue-operation' },
      operationId: 'continue-operation'
    },
    {
      action: 'settle',
      invoke: () => settleCreditLotterySession(7, 2, 'settle-operation'),
      path: '/user/credit-lottery/sessions/7/settle',
      body: { expected_round: 2, operation_id: 'settle-operation', idempotency_key: 'settle-operation' },
      operationId: 'settle-operation'
    }
  ])('sends one matching operation ID for credit lottery $action', async ({ invoke, path, body, operationId }) => {
    await invoke()

    expect(post).toHaveBeenCalledWith(path, body, {
      headers: { 'Idempotency-Key': operationId }
    })
  })
})
