import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: { get }
}))

import { getAllCheckinCards } from '@/api/checkinCards'

describe('checkin collectible card pagination', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('loads every page and preserves the server asset total', async () => {
    get
      .mockResolvedValueOnce({
        data: {
          items: [{ id: 3, card_key: 'three' }, { id: 2, card_key: 'two' }],
          total: 3,
          page: 1,
          page_size: 2,
          has_more: true
        }
      })
      .mockResolvedValueOnce({
        data: {
          items: [{ id: 1, card_key: 'one' }],
          total: 3,
          page: 2,
          page_size: 2,
          has_more: false
        }
      })

    const result = await getAllCheckinCards()

    expect(get).toHaveBeenNthCalledWith(1, '/user/checkin/cards/page', {
      params: { page: 1, page_size: 100 }
    })
    expect(get).toHaveBeenNthCalledWith(2, '/user/checkin/cards/page', {
      params: { page: 2, page_size: 100 }
    })
    expect(result.total).toBe(3)
    expect(result.items.map(item => item.id)).toEqual([3, 2, 1])
  })

  it('deduplicates an item repeated across unstable page boundaries', async () => {
    get
      .mockResolvedValueOnce({
        data: {
          items: [{ id: 2, card_key: 'two' }],
          total: 2,
          page: 1,
          page_size: 1,
          has_more: true
        }
      })
      .mockResolvedValueOnce({
        data: {
          items: [{ id: 2, card_key: 'two' }, { id: 1, card_key: 'one' }],
          total: 2,
          page: 2,
          page_size: 2,
          has_more: false
        }
      })

    const result = await getAllCheckinCards()

    expect(result.items.map(item => item.id)).toEqual([2, 1])
    expect(result.total).toBe(2)
  })
})
