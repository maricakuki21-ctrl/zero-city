import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { CapabilityAsset } from '@/features/bizdecipher/api/bizdecipher'
import CapabilityAssetsView from '../CapabilityAssetsView.vue'
vi.mock('@/features/bizdecipher/components/assets/AssetCommercePanel.vue', () => ({ default: { template: '<section />' } }))

const api = vi.hoisted(() => ({
  list: vi.fn(),
  listMine: vi.fn(),
  stats: vi.fn(),
  get: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  like: vi.fn(),
  favorite: vi.fn(),
}))
vi.mock('@/features/bizdecipher/api/bizdecipher', () => ({
  listCapabilityAssets: api.list,
  listMyCapabilityAssets: api.listMine,
  getMyCapabilityAssetStats: api.stats,
  getCapabilityAsset: api.get,
  createCapabilityAsset: api.create,
  updateCapabilityAsset: api.update,
  setCapabilityAssetLike: api.like,
  setCapabilityAssetFavorite: api.favorite,
}))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: { tab: 'mine', asset: '73' } }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn(), showWarning: vi.fn() }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))
vi.mock('@/components/icons/Icon.vue', () => ({ default: { template: '<span />' } }))
vi.mock('@/features/bizdecipher/components/assets/StarterTemplates.vue', () => ({ default: { template: '<div />' } }))

function asset(id: number): CapabilityAsset {
  return {
    id, user_id: 1, author: '测试用户', title: '已保存的 Harness 草稿', slug: 'saved-harness-draft',
    summary: '摘要', description: '结果正文', asset_type: 'prompt_solution', status: 'draft',
    tags: [], scenario_tags: [], integration_tags: [], cover_url: '', screenshot_urls: [], video_url: '',
    demo_url: '', doc_url: '', source_url: '', template_url: '', primary_action_type: 'view_detail',
    pricing_type: 'free', contact_enabled: false, is_featured: false, featured_weight: 0,
    view_count: 0, like_count: 0, favorite_count: 0, download_count: 0, use_count: 0,
    liked_by_me: false, favorited_by_me: false, downloaded_by_me: false, viewed_today: false,
    comment_count: 0, rating_avg: 0, rating_count: 0,
    review_note: '', created_at: '2026-09-09T00:00:00Z', updated_at: '2026-09-09T00:00:00Z',
  }
}

describe('Capability assets query selection', () => {
  it('opens the returned owner asset selected by the real assets route query', async () => {
    api.list.mockResolvedValue([])
    api.listMine.mockResolvedValue([asset(73)])
    api.stats.mockResolvedValue([])
    api.get.mockResolvedValue(asset(73))
    const wrapper = mount(CapabilityAssetsView)
    await flushPromises()

    expect(api.listMine).toHaveBeenCalledWith({ limit: 50 })
    expect(wrapper.get('.asset-detail-panel h2').text()).toBe('已保存的 Harness 草稿')
    wrapper.unmount()
  })
})
