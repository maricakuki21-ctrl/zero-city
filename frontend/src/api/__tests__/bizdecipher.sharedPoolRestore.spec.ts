import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { post },
}))

vi.mock('@/i18n', () => ({
  getLocale: () => 'zh',
}))

import { adminRestoreSharedPool } from '@/api/bizdecipher'

describe('shared-pool restore API', () => {
  beforeEach(() => {
    post.mockReset()
    post.mockResolvedValue({
      data: {
        id: 56,
        name: '恢复后的草稿池',
        lifecycle_state: 'draft',
        listed: false,
        status: 'maintenance',
      },
    })
  })

  it('sends the same operation id in the body and Idempotency-Key header', async () => {
    const payload = {
      reason: '重新配置并通过满血检测后上架',
      operation_id: 'shared-pool-restore-56-operation-1',
    }

    const restored = await adminRestoreSharedPool(56, payload)

    expect(post).toHaveBeenCalledWith('/admin/biz/shared-pools/56/restore', payload, {
      headers: {
        'Idempotency-Key': 'shared-pool-restore-56-operation-1',
      },
    })
    expect(restored).toMatchObject({
      id: 56,
      lifecycle_state: 'draft',
      listed: false,
      status: 'maintenance',
    })
  })
})
