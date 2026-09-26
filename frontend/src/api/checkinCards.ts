import { apiClient } from './client'
import type { CheckinCollectibleCard } from '@/types'

export interface CheckinCollectibleCardsPage {
  items: CheckinCollectibleCard[]
  total: number
  page: number
  page_size: number
  has_more: boolean
}

export async function getCheckinCardsPage(page = 1, pageSize = 100): Promise<CheckinCollectibleCardsPage> {
  const { data } = await apiClient.get<CheckinCollectibleCardsPage>('/user/checkin/cards/page', {
    params: {
      page,
      page_size: pageSize
    }
  })
  return data
}

export async function getAllCheckinCards(): Promise<CheckinCollectibleCardsPage> {
  const items: CheckinCollectibleCard[] = []
  const seenIDs = new Set<number>()
  let page = 1
  let total = 0
  let hasMore = true

  while (hasMore) {
    const result = await getCheckinCardsPage(page, 100)
    total = Math.max(total, Number(result.total || 0))
    for (const card of Array.isArray(result.items) ? result.items : []) {
      if (!seenIDs.has(card.id)) {
        seenIDs.add(card.id)
        items.push(card)
      }
    }
    hasMore = Boolean(result.has_more)
    page += 1
    if (hasMore && page > Math.ceil(Math.max(total, items.length) / 100) + 2) {
      throw new Error('collectible card pagination did not converge')
    }
  }

  return {
    items,
    total: Math.max(total, items.length),
    page: 1,
    page_size: items.length,
    has_more: false
  }
}
