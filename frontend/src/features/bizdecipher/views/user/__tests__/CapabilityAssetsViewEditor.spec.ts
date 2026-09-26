import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { CapabilityAsset } from '@/features/bizdecipher/api/bizdecipher'
import CapabilityAssetsView from '../CapabilityAssetsView.vue'

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
const store = vi.hoisted(() => ({ showError: vi.fn(), showSuccess: vi.fn(), showWarning: vi.fn() }))
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
vi.mock('vue-router', () => ({ useRoute: () => ({ query: { tab: 'mine' } }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => store }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))
vi.mock('@/components/icons/Icon.vue', () => ({ default: { template: '<span />' } }))
vi.mock('@/features/bizdecipher/components/assets/StarterTemplates.vue', () => ({ default: { template: '<div />' } }))

function draft(overrides: Partial<CapabilityAsset> = {}): CapabilityAsset {
  return {
    id: 73, user_id: 7, author: '资产主人', title: '原草稿', slug: 'draft-73', summary: '原摘要',
    description: '原正文', asset_type: 'workflow', status: 'draft', tags: ['AI'], scenario_tags: ['企业服务'],
    integration_tags: ['Slack'], cover_url: '', screenshot_urls: ['https://example.test/shot.png'], video_url: '',
    demo_url: 'https://example.test/demo', doc_url: '', source_url: '', template_url: '',
    primary_action_type: 'view_workflow', pricing_type: 'free', contact_enabled: true, is_featured: false,
    featured_weight: 0, view_count: 0, like_count: 0, favorite_count: 0, download_count: 0, use_count: 0,
    liked_by_me: false, favorited_by_me: false, downloaded_by_me: false, viewed_today: false,
    comment_count: 0, rating_avg: 0,
    rating_count: 0, review_note: '', created_at: '2026-09-08T00:00:00Z', updated_at: '2026-09-09T00:00:00Z',
    ...overrides,
  }
}

describe('owner draft editing', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.list.mockResolvedValue([])
    api.stats.mockResolvedValue([])
  })

  it('keeps the saved result and form when PATCH succeeds but list readback fails', async () => {
    const saved = draft({ title: '已保存标题', updated_at: '2026-09-10T03:00:00Z' })
    api.listMine.mockResolvedValueOnce([draft()]).mockRejectedValueOnce(new Error('readback unavailable'))
    api.update.mockResolvedValue(saved)
    const wrapper = mount(CapabilityAssetsView)
    await flushPromises()

    await wrapper.get('.my-asset-edit').trigger('click')
    const title = wrapper.get('.asset-editor input')
    expect((title.element as HTMLInputElement).value).toBe('原草稿')
    await title.setValue('已保存标题')
    const form = wrapper.get('.asset-editor form')
    await Promise.all([form.trigger('submit'), form.trigger('submit')])
    await flushPromises()

    expect(api.update).toHaveBeenCalledTimes(1)
    expect(api.update).toHaveBeenCalledWith(73, expect.objectContaining({
      title: '已保存标题', status: 'draft', integration_tags: ['Slack'], screenshot_urls: ['https://example.test/shot.png'],
    }))
    expect(api.create).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('草稿 #73 已保存，但列表重新读取失败')
    expect(wrapper.get('.my-asset-info h3').text()).toBe('已保存标题')
    expect((wrapper.get('.asset-editor input').element as HTMLInputElement).value).toBe('已保存标题')
    wrapper.unmount()
  })

  it('offers edit only for drafts and labels their timestamp as an update', async () => {
    api.listMine.mockResolvedValue([draft(), draft({ id: 74, title: '审核中', status: 'pending' })])
    const wrapper = mount(CapabilityAssetsView)
    await flushPromises()

    expect(wrapper.findAll('.my-asset-edit')).toHaveLength(1)
    expect(wrapper.findAll('.my-asset-date')[0].text()).toContain('更新')
    expect(wrapper.text()).not.toContain('发布 2026-09-08')
    wrapper.unmount()
  })
})
