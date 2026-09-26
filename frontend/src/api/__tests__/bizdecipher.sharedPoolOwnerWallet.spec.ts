import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post },
}))

vi.mock('@/i18n', () => ({
  getLocale: () => 'zh',
}))

import { listMySharedPoolLedger, transferSharedPoolOwnerEarnings } from '@/api/bizdecipher'

describe('shared-pool owner wallet API', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
  })

  it('keeps old ledger responses readable before the wallet migration is enabled', async () => {
    get.mockResolvedValue({
      data: {
        activity: [],
        withdrawable: [{ id: 1 }],
        incentives: [],
      },
    })

    const result = await listMySharedPoolLedger()

    expect(result.wallet.available_amount).toBe(0)
    expect(result.earnings).toEqual([])
    expect(result.legacy_withdrawable).toEqual([])
    expect(result.withdrawable).toEqual([{ id: 1 }])
  })

  it('sends the stable operation id in both body and idempotency header', async () => {
    post.mockResolvedValue({
      data: {
        operation_id: 'owner-transfer-71-1',
        amount: 3.5,
        wallet_after: 9,
        balance_after: 30,
        already_done: false,
      },
    })

    await transferSharedPoolOwnerEarnings(3.5, 'owner-transfer-71-1')

    expect(post).toHaveBeenCalledWith(
      '/biz/shared-pool/earnings/transfer',
      { amount: 3.5, operation_id: 'owner-transfer-71-1' },
      { headers: { 'Idempotency-Key': 'owner-transfer-71-1' } },
    )
  })
})
