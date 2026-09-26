import { beforeEach, describe, expect, it, vi } from 'vitest'
import { communityAPI, type CreateCommunityPollPayload } from '../community'

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: client }))

describe('community polls API', () => {
  beforeEach(() => {
    client.get.mockReset()
    client.post.mockReset()
    client.get.mockResolvedValue({ data: { items: [] } })
    client.post.mockResolvedValue({ data: { id: 9 } })
  })

  it('uses the public results endpoint and authenticated poll commands', async () => {
    const payload: CreateCommunityPollPayload = {
      title: '先做哪个功能？', body: '请选择一个方向。', options: ['搜索', '分享'],
    }

    await communityAPI.listPolls(20)
    await communityAPI.createPoll(payload)
    await communityAPI.votePoll(9, 77)
    await communityAPI.closePoll(9)

    expect(client.get).toHaveBeenCalledWith('/biz/community/polls', { params: { limit: 20 } })
    expect(client.post).toHaveBeenNthCalledWith(1, '/biz/community/polls', payload)
    expect(client.post).toHaveBeenNthCalledWith(2, '/biz/community/polls/9/vote', { option_id: 77 })
    expect(client.post).toHaveBeenNthCalledWith(3, '/biz/community/polls/9/close')
  })
})
