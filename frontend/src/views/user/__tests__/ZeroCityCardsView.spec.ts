import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ZeroCityCardsView from '@/features/bizdecipher/views/user/ZeroCityCardsView.vue'
import type { CheckinCollectibleCard } from '@/types'

const apiMocks = vi.hoisted(() => ({
  getAllCheckinCards: vi.fn(),
  getZeroCityPublicProfile: vi.fn(),
  updateZeroCityProfileBackground: vi.fn()
}))

const storeMocks = vi.hoisted(() => ({
  showSuccess: vi.fn(),
  showError: vi.fn(),
  user: { id: 7 }
}))

const routerMocks = vi.hoisted(() => ({
  replace: vi.fn(),
  push: vi.fn(),
  route: { query: {} as Record<string, string> }
}))

vi.mock('@/api/checkinCards', () => ({
  getAllCheckinCards: apiMocks.getAllCheckinCards
}))

vi.mock('@/features/bizdecipher/api/bizdecipher', () => ({
  getZeroCityPublicProfile: apiMocks.getZeroCityPublicProfile,
  updateZeroCityProfileBackground: apiMocks.updateZeroCityProfileBackground
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess: storeMocks.showSuccess,
    showError: storeMocks.showError
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ user: storeMocks.user })
}))

vi.mock('vue-router', () => ({
  useRoute: () => routerMocks.route,
  useRouter: () => ({
    replace: routerMocks.replace,
    push: routerMocks.push
  })
}))

function card(id: number, overrides: Partial<CheckinCollectibleCard> = {}): CheckinCollectibleCard {
  return {
    id,
    card_key: 'low_battery_sprite',
    rarity: 'common',
    source_type: 'free',
    source_label: 'free',
    serial_no: id,
    edition_no: id,
    edition_supply: 9999,
    created_at: '2026-07-19T00:00:00Z',
    ...overrides
  }
}

function mountAlbum() {
  return mount(ZeroCityCardsView, {
    attachTo: document.body,
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' }
      }
    }
  })
}

function findButton(wrapper: ReturnType<typeof mountAlbum>, label: string) {
  const button = wrapper.findAll('button').find(item => item.text().includes(label))
  expect(button, `button ${label}`).toBeTruthy()
  return button!
}

describe('ZeroCityCardsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    document.body.innerHTML = ''
    document.body.style.overflow = ''
    routerMocks.route.query = {}
    apiMocks.getAllCheckinCards.mockResolvedValue({
      items: [card(1), card(2)],
      total: 2,
      page: 1,
      page_size: 2,
      has_more: false
    })
    apiMocks.getZeroCityPublicProfile.mockResolvedValue({ profile: {} })
    apiMocks.updateZeroCityProfileBackground.mockResolvedValue({})
  })

  it('separates loaded items, unlocked faces and current catalog results', async () => {
    const wrapper = mountAlbum()
    await flushPromises()

    const summaries = wrapper.findAll('.album-summary article').map(item => item.text())
    expect(summaries).toEqual([
      '2资产实例总数',
      '1 / 82已解锁卡面 / 图鉴',
      '82当前筛选结果'
    ])
    expect(wrapper.text()).toContain('已完整读取 2 张收藏资产')
  })

  it('groups duplicate cards, uses formal names and keeps the catalog paginated', async () => {
    const wrapper = mountAlbum()
    await flushPromises()

    expect(wrapper.findAll('.album-card')).toHaveLength(12)
    expect(wrapper.find('.album-card').text()).toContain('低电量小人')
    expect(wrapper.find('.album-card').text()).toContain('×2')
    expect(wrapper.text()).toContain('第 1 / 7 页')

    await findButton(wrapper, '下一页').trigger('click')
    expect(wrapper.text()).toContain('第 2 / 7 页')
  })

  it('keeps story copy on the detail back and supports Escape to close', async () => {
    const wrapper = mountAlbum()
    await flushPromises()

    await wrapper.find('.album-card').trigger('click')
    await flushPromises()
    expect(wrapper.find('.card-detail-dialog').exists()).toBe(true)
    expect(document.activeElement).toBe(wrapper.find('.card-detail-close').element)
    expect(wrapper.find('.detail-card-shell').classes()).not.toContain('is-flipped')

    await findButton(wrapper, '查看背面故事').trigger('click')
    expect(wrapper.find('.detail-card-shell').classes()).toContain('is-flipped')
    expect(wrapper.find('.detail-card-back').text()).toContain('今天电量不满，也允许缓慢发光。')
    expect(wrapper.find('.detail-card-back').text()).toContain('负责举着一粒小灯泡')

    await wrapper.find('.card-detail-backdrop').trigger('keydown', { key: 'Escape', code: 'Escape', keyCode: 27 })
    await flushPromises()
    expect(routerMocks.replace).toHaveBeenLastCalledWith({ path: '/zero-city/cards' })
    expect(document.body.style.overflow).toBe('')
  })

  it('keeps unowned artwork inspectable without enabling equip', async () => {
    apiMocks.getAllCheckinCards.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 0,
      has_more: false
    })
    const wrapper = mountAlbum()
    await flushPromises()

    expect(wrapper.findAll('.album-card img')).toHaveLength(12)
    expect(wrapper.find('.album-card').attributes('disabled')).toBeUndefined()
    await wrapper.find('.album-card').trigger('click')
    expect(wrapper.find('.card-detail-dialog').exists()).toBe(true)
    expect(wrapper.find('.card-detail-dialog').text()).toContain('尚未拥有')
    expect(findButton(wrapper, '拥有后可展示').attributes('disabled')).toBeDefined()
  })

  it('filters the gallery by owned and missing definitions', async () => {
    const wrapper = mountAlbum()
    await flushPromises()

    await findButton(wrapper, '已拥有').trigger('click')
    expect(wrapper.findAll('.album-card')).toHaveLength(1)
    expect(wrapper.find('.album-card').text()).toContain('低电量小人')

    await findButton(wrapper, '未拥有').trigger('click')
    expect(wrapper.findAll('.album-card')).toHaveLength(12)
    expect(wrapper.find('.album-card').text()).toContain('尚未获得')
  })
})
