import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const packageApi = vi.hoisted(() => ({
  listCapabilityAssetVersions: vi.fn(),
  createCapabilityAssetVersion: vi.fn(),
  putCapabilityAssetFile: vi.fn(),
  finalizeCapabilityAssetVersion: vi.fn(),
  publishCapabilityAssetVersion: vi.fn(),
  revokeCapabilityAssetVersion: vi.fn(),
  downloadCapabilityAssetPackage: vi.fn(),
}))

const appStore = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/features/bizdecipher/api/bizdecipher', () => packageApi)
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('../AssetCommercePanel.vue', () => ({ default: { template: '<section />' } }))

import AssetPackageDialog from '../AssetPackageDialog.vue'

const asset = {
  id: 41,
  user_id: 7,
  author: 'owner',
  title: 'Story Runtime',
  slug: 'story-runtime',
  summary: 'A bounded game package',
  description: '',
  asset_type: 'game',
  status: 'draft',
  tags: [],
  scenario_tags: [],
  integration_tags: [],
  cover_url: '',
  screenshot_urls: [],
  video_url: '',
  demo_url: '',
  doc_url: '',
  source_url: '',
  template_url: '',
  primary_action_type: 'view_detail',
  pricing_type: 'free',
  contact_enabled: true,
  is_featured: false,
  featured_weight: 0,
  view_count: 0,
  like_count: 0,
  favorite_count: 0,
  download_count: 0,
  use_count: 0,
  liked_by_me: false,
  favorited_by_me: false,
  downloaded_by_me: false,
  viewed_today: false,
  comment_count: 0,
  rating_avg: 0,
  rating_count: 0,
  review_note: '',
  created_at: '2026-09-11T00:00:00Z',
  updated_at: '2026-09-11T00:00:00Z',
}

describe('AssetPackageDialog', () => {
  beforeEach(() => {
    Object.values(packageApi).forEach(mock => mock.mockReset())
    appStore.showError.mockReset()
    appStore.showSuccess.mockReset()
  })

  it('loads versions and exposes only lifecycle actions that match the server state', async () => {
    packageApi.listCapabilityAssetVersions.mockResolvedValue([
      {
        id: 2,
        asset_id: 41,
        version: 'v1.0.0',
        runtime_kind: 'game_script',
        manifest: {},
        status: 'draft',
        file_count: 1,
        total_bytes: 128,
        created_at: '2026-09-11T00:00:00Z',
        updated_at: '2026-09-11T00:00:00Z',
      },
    ])

    const wrapper = mount(AssetPackageDialog, {
      props: { open: true, asset },
      global: { stubs: { Icon: { template: '<span />' } } },
    })
    await flushPromises()

    expect(packageApi.listCapabilityAssetVersions).toHaveBeenCalledWith(41)
    expect(wrapper.text()).toContain('v1.0.0')
    expect(wrapper.text()).toContain('游戏 / 剧本')
    expect(wrapper.findAll('button').some(button => button.text().includes('封包'))).toBe(true)
    expect(wrapper.findAll('button').some(button => button.text().includes('发布版本'))).toBe(false)
  })

  it('finalizes a file-backed draft and reloads authoritative state', async () => {
    packageApi.listCapabilityAssetVersions
      .mockResolvedValueOnce([
        {
          id: 2,
          asset_id: 41,
          version: 'v1.0.0',
          runtime_kind: 'workflow',
          manifest: {},
          status: 'draft',
          file_count: 1,
          total_bytes: 128,
          created_at: '2026-09-11T00:00:00Z',
          updated_at: '2026-09-11T00:00:00Z',
        },
      ])
      .mockResolvedValueOnce([
        {
          id: 2,
          asset_id: 41,
          version: 'v1.0.0',
          runtime_kind: 'workflow',
          manifest: {},
          status: 'ready',
          package_digest: 'a'.repeat(64),
          file_count: 1,
          total_bytes: 128,
          created_at: '2026-09-11T00:00:00Z',
          updated_at: '2026-09-11T00:00:00Z',
        },
      ])
    packageApi.finalizeCapabilityAssetVersion.mockResolvedValue({ id: 2, version: 'v1.0.0', status: 'ready' })

    const wrapper = mount(AssetPackageDialog, {
      props: { open: true, asset },
      global: { stubs: { Icon: { template: '<span />' } } },
    })
    await flushPromises()

    const finalizeButton = wrapper.findAll('button').find(button => button.text().includes('封包'))
    expect(finalizeButton).toBeTruthy()
    await finalizeButton!.trigger('click')
    await flushPromises()

    expect(packageApi.finalizeCapabilityAssetVersion).toHaveBeenCalledWith(41, 'v1.0.0')
    expect(packageApi.listCapabilityAssetVersions).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('已封包')
    expect(wrapper.findAll('button').some(button => button.text().includes('发布版本'))).toBe(true)
  })
})
