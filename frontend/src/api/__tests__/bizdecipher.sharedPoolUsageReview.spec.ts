import { beforeEach, describe, expect, expectTypeOf, it, vi } from 'vitest'

import type {
  BatchResolveSharedPoolUsageReviewsPayload,
  ResolveSharedPoolUsageReviewPayload,
  SharedPoolUsageReview,
} from '@/api/bizdecipher'

const { get, post, put } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post, put },
}))

vi.mock('@/i18n', () => ({
  getLocale: () => 'zh',
}))

import {
  adminBatchResolveSharedPoolUsageReviews,
  adminGetSharedPoolUsageReviewPolicy,
  adminListSharedPoolUsageReviews,
  adminResolveSharedPoolUsageReview,
  adminUpdateSharedPoolUsageReviewPolicy,
} from '@/api/bizdecipher'

describe('shared-pool usage review API', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    put.mockReset()
  })

  it('keeps writes release-only while historical records retain retired action labels', () => {
    expectTypeOf<ResolveSharedPoolUsageReviewPayload['action']>().toEqualTypeOf<'release'>()
    expectTypeOf<BatchResolveSharedPoolUsageReviewsPayload['action']>().toEqualTypeOf<'release'>()
    expectTypeOf<NonNullable<SharedPoolUsageReview['resolution_action']>>()
      .toEqualTypeOf<'release' | 'capture_hold' | 'settle_amount'>()
  })

  it('lists a state with a stable cursor contract', async () => {
    get.mockResolvedValue({
      data: {
        items: [{ reservation_id: 42, reservation_status: 'review_required' }],
        next_before_id: 42,
        has_more: true,
      },
    })

    const page = await adminListSharedPoolUsageReviews({
      state: 'processing',
      before_id: 99,
      limit: 20,
    })

    expect(get).toHaveBeenCalledWith('/admin/biz/shared-pool-usage-reviews', {
      params: { state: 'processing', before_id: 99, limit: 20 },
    })
    expect(page.has_more).toBe(true)
    expect(page.items[0]?.reservation_id).toBe(42)
  })

  it('defaults to pending and omits an empty cursor', async () => {
    get.mockResolvedValue({ data: { items: [], has_more: false } })

    await adminListSharedPoolUsageReviews()

    expect(get).toHaveBeenCalledWith('/admin/biz/shared-pool-usage-reviews', {
      params: { state: 'pending', limit: 20 },
    })
  })

  it('sends the exact operation id in both body and Idempotency-Key', async () => {
    const payload = {
      action: 'release' as const,
      note: '旧复核记录不能作为人工扣费依据，释放冻结',
      operation_id: 'shared-pool-usage-review-42-operation-1',
    }
    post.mockResolvedValue({
      data: {
        review: { reservation_id: 42, reservation_status: 'released' },
        replay: false,
      },
    })

    const result = await adminResolveSharedPoolUsageReview(42, payload)

    expect(post).toHaveBeenCalledWith(
      '/admin/biz/shared-pool-usage-reviews/42/resolve',
      payload,
      { headers: { 'Idempotency-Key': payload.operation_id } },
    )
    expect(result.review.reservation_status).toBe('released')
  })

  it('loads and updates the low-value auto-release policy', async () => {
    const policy = {
      auto_release_enabled: true,
      auto_release_minutes: 15,
      auto_release_max_hold: 0.01,
    }
    get.mockResolvedValueOnce({ data: policy })
    put.mockResolvedValueOnce({ data: { ...policy, auto_release_minutes: 120 } })

    await expect(adminGetSharedPoolUsageReviewPolicy()).resolves.toEqual(policy)
    await expect(adminUpdateSharedPoolUsageReviewPolicy({ ...policy, auto_release_minutes: 120 })).resolves.toMatchObject({ auto_release_minutes: 120 })
    expect(get).toHaveBeenCalledWith('/admin/biz/shared-pool-usage-reviews/policy')
    expect(put).toHaveBeenCalledWith('/admin/biz/shared-pool-usage-reviews/policy', { ...policy, auto_release_minutes: 120 })
  })

  it('batch resolves selected reviews with one idempotent operation', async () => {
    const payload = {
      reservation_ids: [42, 43],
      action: 'release' as const,
      note: '同批上游明确失败，释放冻结',
      operation_id: 'shared-pool-review-batch-1',
    }
    post.mockResolvedValueOnce({ data: { items: [], succeeded: 2, failed: 0 } })

    const result = await adminBatchResolveSharedPoolUsageReviews(payload)

    expect(post).toHaveBeenCalledWith(
      '/admin/biz/shared-pool-usage-reviews/batch-resolve',
      payload,
      { headers: { 'Idempotency-Key': payload.operation_id } },
    )
    expect(result.succeeded).toBe(2)
  })
})
