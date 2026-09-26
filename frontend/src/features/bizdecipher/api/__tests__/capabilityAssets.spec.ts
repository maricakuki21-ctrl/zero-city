import { describe, expect, it, vi } from 'vitest'
import type { CapabilityAssetPayload } from '../bizdecipher'
import {
  getMyCapabilityAssetStats,
  setCapabilityAssetFavorite,
  setCapabilityAssetLike,
  updateCapabilityAsset,
} from '../bizdecipher'

const { get, post, patch, del } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  patch: vi.fn(),
  del: vi.fn(),
}))
vi.mock('@/api/client', () => ({ apiClient: { get, post, patch, delete: del } }))

describe('capability asset API', () => {
  it('PATCHes the existing owner draft id with the full payload', async () => {
    const payload: CapabilityAssetPayload = {
      title: 'Edited', summary: 'Summary', description: 'Description', asset_type: 'workflow', status: 'draft',
      tags: ['AI'], scenario_tags: ['ops'], integration_tags: ['Slack'], cover_url: '', screenshot_urls: [],
      video_url: '', demo_url: '', doc_url: '', source_url: '', template_url: '', primary_action_type: 'view_workflow',
      pricing_type: 'free', contact_enabled: false,
    }
    patch.mockResolvedValue({ data: { id: 73, ...payload } })

    expect(await updateCapabilityAsset(73, payload)).toEqual({ id: 73, ...payload })
    expect(patch).toHaveBeenCalledWith('/biz/assets/73', payload)
  })

  it('loads owner-only activity stats from the dedicated endpoint', async () => {
    get.mockResolvedValue({ data: { items: [{ asset_id: 73, view_count: 4 }] } })

    await expect(getMyCapabilityAssetStats()).resolves.toEqual([{ asset_id: 73, view_count: 4 }])
    expect(get).toHaveBeenCalledWith('/biz/my/assets/stats')
  })

  it('uses idempotent POST and DELETE endpoints for like and favorite state', async () => {
    post.mockResolvedValue({ data: { id: 73, liked_by_me: true, favorited_by_me: true } })
    del.mockResolvedValue({ data: { id: 73, liked_by_me: false, favorited_by_me: false } })

    await setCapabilityAssetLike(73, true)
    await setCapabilityAssetLike(73, false)
    await setCapabilityAssetFavorite(73, true)
    await setCapabilityAssetFavorite(73, false)

    expect(post).toHaveBeenNthCalledWith(1, '/biz/assets/73/like')
    expect(del).toHaveBeenNthCalledWith(1, '/biz/assets/73/like')
    expect(post).toHaveBeenNthCalledWith(2, '/biz/assets/73/favorite')
    expect(del).toHaveBeenNthCalledWith(2, '/biz/assets/73/favorite')
  })
})
