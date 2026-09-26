import { apiClient } from '@/api/client'
import type { SharedPoolOwnerEarningsEntry } from './bizdecipher'

export interface SharedPoolOwnerEarningsPage {
  items: SharedPoolOwnerEarningsEntry[]
  next_before_id?: number
  has_more: boolean
}

export async function listSharedPoolOwnerEarningsPage(options: {
  beforeId?: number
  poolId?: number
  limit?: number
} = {}): Promise<SharedPoolOwnerEarningsPage> {
  const { data } = await apiClient.get<SharedPoolOwnerEarningsPage>('/biz/shared-pool/ledger/earnings', {
    params: {
      before_id: options.beforeId || undefined,
      pool_id: options.poolId || undefined,
      limit: options.limit || 50
    }
  })
  return {
    items: Array.isArray(data?.items) ? data.items : [],
    next_before_id: Number(data?.next_before_id || 0) || undefined,
    has_more: Boolean(data?.has_more)
  }
}
