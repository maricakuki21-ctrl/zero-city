import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ZeroCityPersonalHub from '../ZeroCityPersonalHub.vue'

const api = vi.hoisted(() => ({
  getMyZeroCityProfile: vi.fn(),
  updateMyZeroCityProfile: vi.fn(),
  updateZeroCityProfileBackground: vi.fn(),
  getCheckinCards: vi.fn(),
}))

vi.mock('@/features/bizdecipher/api/bizdecipher', () => ({
  getMyZeroCityProfile: api.getMyZeroCityProfile,
  updateMyZeroCityProfile: api.updateMyZeroCityProfile,
  updateZeroCityProfileBackground: api.updateZeroCityProfileBackground,
}))
vi.mock('@/api/user', () => ({
  getCheckinCards: api.getCheckinCards,
}))

const user = { id: 7, username: '旧昵称', email: 'user@example.test', role: 'user' } as const
const profile = { user_id: 7, handle: 'resident-7', display_name: '旧昵称', avatar_url: '', bio: '', created_at: '', updated_at: '' }

function createWrapper(overrides: Record<string, unknown> = {}) {
  return mount(ZeroCityPersonalHub, {
    props: {
      user,
      history: { posts: 1, accepted: 0, confirmed: 0, assets: 0 },
      historyError: false,
      posts: [],
      loading: false,
      ...overrides,
    },
    global: { plugins: [createPinia()] },
  })
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: Error) => void
  const promise = new Promise<T>((next, fail) => { resolve = next; reject = fail })
  return { promise, resolve, reject }
}

describe('ZeroCityPersonalHub', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    api.getMyZeroCityProfile.mockResolvedValue(profile)
    api.getCheckinCards.mockResolvedValue([{ id: 1, card_key: 'archive-light', rarity: 'rare', source_type: 'checkin', source_label: '签到', serial_no: 12, created_at: '' }])
  })

  it('saves nickname and avatar through the existing user contract, then refreshes the city profile', async () => {
    api.updateMyZeroCityProfile.mockResolvedValue({ ...profile, display_name: '新昵称', avatar_url: 'https://example.test/avatar.png' })
    api.getMyZeroCityProfile.mockResolvedValueOnce(profile).mockResolvedValueOnce({ ...profile, display_name: '新昵称', avatar_url: 'https://example.test/avatar.png' })
    const wrapper = createWrapper()
    await flushPromises()
    await wrapper.get('.city-personal-primary').trigger('click')
    await flushPromises()
    await wrapper.get('input[name="nickname"]').setValue('新昵称')
    await wrapper.get('input[name="avatar"]').setValue('https://example.test/avatar.png')
    await wrapper.get('.city-personal-editor-form').trigger('submit')
    await flushPromises()

    expect(api.updateMyZeroCityProfile).toHaveBeenCalledWith({ display_name: '新昵称', avatar_url: 'https://example.test/avatar.png' })
    expect(api.getMyZeroCityProfile).toHaveBeenCalledTimes(2)
    expect(wrapper.get('#city-personal-title').text()).toBe('新昵称')
    expect(wrapper.get('.city-personal-avatar').attributes('src')).toBe('https://example.test/avatar.png')
  })

  it('sets a background only from cards returned by the real card collection contract', async () => {
    api.updateZeroCityProfileBackground.mockResolvedValue({ ...profile, background_card_key: 'archive-light', background_card_rarity: 'rare', background_card_serial_no: 12 })
    const wrapper = createWrapper()
    await flushPromises()
    await wrapper.get('.city-personal-primary').trigger('click')
    await flushPromises()
    await wrapper.get('.city-personal-card-options button').trigger('click')
    await flushPromises()

    expect(api.updateZeroCityProfileBackground).toHaveBeenCalledWith({ card_key: 'archive-light', serial_no: 12 })
    expect(wrapper.get('.city-personal-hero').attributes('style')).toContain('archive-light.png')
  })

  it('reports a successful save separately when profile readback fails', async () => {
    api.updateMyZeroCityProfile.mockResolvedValue({ ...profile, display_name: '已保存昵称', avatar_url: 'https://example.test/saved.png' })
    api.getMyZeroCityProfile.mockResolvedValueOnce(profile).mockRejectedValueOnce(new Error('readback unavailable'))
    const wrapper = createWrapper()
    await flushPromises()
    await wrapper.get('.city-personal-primary').trigger('click')
    await flushPromises()
    await wrapper.get('input[name="nickname"]').setValue('已保存昵称')
    await wrapper.get('input[name="avatar"]').setValue('https://example.test/saved.png')
    await wrapper.get('.city-personal-editor-form').trigger('submit')
    await flushPromises()

    expect(wrapper.get('.city-personal-editor-message').text()).toContain('已保存')
    expect(wrapper.get('.city-personal-editor-message').text()).toContain('重新读取失败')
    expect(wrapper.get('.city-personal-editor-message').text()).not.toContain('保存失败')
    expect(wrapper.get('#city-personal-title').text()).toBe('已保存昵称')
  })

  it('ignores a stale profile response after the account changes', async () => {
    const first = deferred<typeof profile>()
    const secondProfile = { ...profile, user_id: 8, handle: 'resident-8', display_name: '新账号' }
    api.getMyZeroCityProfile.mockReset()
    api.getMyZeroCityProfile.mockReturnValueOnce(first.promise).mockResolvedValueOnce(secondProfile)
    const wrapper = createWrapper()
    await wrapper.setProps({ user: { ...user, id: 8, username: '新账号' } })
    await flushPromises()
    first.resolve(profile)
    await flushPromises()

    expect(wrapper.get('#city-personal-title').text()).toBe('新账号')
    expect(api.getMyZeroCityProfile).toHaveBeenCalledTimes(2)
  })

  it('falls back to the city mascot when an avatar image is broken', async () => {
    api.getMyZeroCityProfile.mockResolvedValue({ ...profile, avatar_url: 'https://example.test/broken.png' })
    const wrapper = createWrapper()
    await flushPromises()
    await wrapper.get('.city-personal-avatar').trigger('error')

    expect(wrapper.get('.city-personal-avatar').attributes('src')).toContain('/assets/zero-point-city/mascots/')
  })

  it('shows unknown history with retry instead of confirmed zeroes after a load failure', async () => {
    const wrapper = createWrapper({
      history: { posts: 0, accepted: 0, confirmed: 0, assets: 0 },
      historyError: true,
    })
    await flushPromises()

    expect(wrapper.findAll('.city-personal-stats strong').map((item) => item.text())).toEqual(['—', '—', '—', '—'])
    expect(wrapper.get('[role="alert"]').text()).toContain('历史暂时没有加载成功')
    expect(wrapper.text()).not.toContain('还没有公开动态')
    await wrapper.get('[role="alert"] button').trigger('click')
    expect(wrapper.emitted('refresh')).toHaveLength(1)
  })

  it('uses the collection catalog name and explicitly labels an equipped background', async () => {
    api.getCheckinCards.mockResolvedValue([{ id: 1, card_key: 'low_battery_sprite', rarity: 'common', serial_no: 12 }])
    api.getMyZeroCityProfile.mockResolvedValue({ ...profile, background_card_key: 'low_battery_sprite', background_card_serial_no: 12 })
    const wrapper = createWrapper()
    await flushPromises()
    await wrapper.get('.city-personal-primary').trigger('click')
    await flushPromises()

    const card = wrapper.get('.city-personal-card-options button')
    expect(card.text()).toContain('低电量小人')
    expect(card.text()).toContain('普通 · #12')
    expect(card.text()).toContain('已佩戴')
    expect(card.attributes('aria-pressed')).toBe('true')
    expect(wrapper.get('.city-personal-card-signal').text()).toContain('低电量小人')
  })

  it('offers a card-specific retry without claiming an empty collection on error', async () => {
    api.getCheckinCards.mockRejectedValueOnce(new Error('unavailable')).mockResolvedValueOnce([])
    const wrapper = createWrapper()
    await flushPromises()
    await wrapper.get('.city-personal-primary').trigger('click')
    await flushPromises()

    expect(wrapper.get('.city-personal-card-error').text()).toContain('卡册暂时无法读取')
    expect(wrapper.text()).not.toContain('卡册中暂无可用收藏卡')
    await wrapper.get('.city-personal-card-error button').trigger('click')
    await flushPromises()
    expect(api.getCheckinCards).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('卡册中暂无可用收藏卡')
    expect(wrapper.find('.city-personal-card-error').exists()).toBe(false)
  })

  it('discards another account collection when its pending request finishes late', async () => {
    const first = deferred<unknown[]>()
    api.getCheckinCards.mockReturnValueOnce(first.promise).mockResolvedValueOnce([])
    const wrapper = createWrapper()
    await flushPromises()
    await wrapper.get('.city-personal-primary').trigger('click')
    await wrapper.setProps({ user: { ...user, id: 8, username: '新账号' } })
    await flushPromises()
    await wrapper.get('.city-personal-primary').trigger('click')
    await flushPromises()
    first.resolve([{ id: 1, card_key: 'low_battery_sprite', rarity: 'common', serial_no: 12 }])
    await flushPromises()

    expect(wrapper.find('.city-personal-card-options').exists()).toBe(false)
    expect(wrapper.text()).toContain('卡册中暂无可用收藏卡')
    expect(api.getCheckinCards).toHaveBeenCalledTimes(2)
  })

  it.each([
    ['identity', 'success'], ['identity', 'failure'],
    ['background', 'success'], ['background', 'failure'],
  ])('isolates late %s save %s from the next account and its pending save', async (kind, result) => {
    const oldSave = deferred<typeof profile>()
    const newSave = deferred<typeof profile>()
    const writer = kind === 'identity' ? api.updateMyZeroCityProfile : api.updateZeroCityProfileBackground
    writer.mockReturnValueOnce(oldSave.promise).mockReturnValueOnce(newSave.promise)
    const nextProfile = { ...profile, user_id: 8, display_name: '新账号' }
    const wrapper = createWrapper()
    await flushPromises()
    await wrapper.get('.city-personal-primary').trigger('click')
    await flushPromises()
    const submit = async () => {
      if (kind === 'identity') await wrapper.get('.city-personal-editor-form').trigger('submit')
      else await wrapper.get('.city-personal-card-options button').trigger('click')
    }
    await submit()
    api.getMyZeroCityProfile.mockResolvedValue(nextProfile)
    await wrapper.setProps({ user: { ...user, id: 8, username: '新账号' } })
    await flushPromises()
    await wrapper.get('.city-personal-primary').trigger('click')
    await flushPromises()
    await submit()
    if (result === 'success') oldSave.resolve(profile)
    else oldSave.reject(new Error('old account write failed'))
    await flushPromises()

    expect(wrapper.get('#city-personal-title').text()).toBe('新账号')
    expect(wrapper.find('.city-personal-editor-message').exists()).toBe(false)
    const pendingButton = kind === 'identity'
      ? wrapper.get('.city-personal-editor-form button[type="submit"]')
      : wrapper.get('.city-personal-card-options button')
    expect(pendingButton.attributes('disabled')).toBeDefined()
    expect(api.getMyZeroCityProfile).toHaveBeenCalledTimes(2)

    newSave.resolve(nextProfile)
    await flushPromises()
    expect(pendingButton.attributes('disabled')).toBeUndefined()
    expect(wrapper.get('#city-personal-title').text()).toBe('新账号')
    expect(wrapper.get('.city-personal-editor-message').text()).toContain(kind === 'identity' ? '公开身份已保存' : '卡片背景已更新')
  })
})
