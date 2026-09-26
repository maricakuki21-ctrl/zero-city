import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { reactive } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import CityArrival from '../CityArrival.vue'
import { useCityArrivalStore } from '@/stores/cityArrival'

const mocks = vi.hoisted(() => ({ auth: {} as any, route: {} as any, push: vi.fn() }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => mocks.auth }))
vi.mock('vue-router', () => ({ useRoute: () => mocks.route, useRouter: () => ({ push: mocks.push }) }))
let wrapper: ReturnType<typeof mount> | undefined
const click = async (text: string) => {
  const button = Array.from(document.querySelectorAll('button')).find(el => el.textContent?.includes(text))
  expect(button, text).toBeTruthy()
  button!.click()
  await flushPromises()
}
const render = async () => {
  wrapper = mount(CityArrival, { attachTo: document.body })
  await flushPromises()
}

describe('city arrival experience', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    setActivePinia(createPinia())
    mocks.auth = reactive({ isAuthenticated: true, user: { id: 5, username: '归来的旅人' } })
    mocks.route = reactive({ path: '/keys', fullPath: '/keys?create=1', meta: { requiresAuth: true } })
    mocks.push.mockReset().mockResolvedValue(undefined)
    HTMLDialogElement.prototype.showModal = function () { this.open = true }
    HTMLDialogElement.prototype.close = function () { this.open = false }
  })
  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
    document.body.innerHTML = ''
    document.body.style.overflow = ''
    vi.useRealTimers()
  })

  it('shows the story, supports pause and skips without losing a deep link', async () => {
    await render()
    expect(document.querySelector('dialog')?.open).toBe(true)
    expect(document.body.textContent).toContain('归来的旅人，你到了')
    document.querySelector<HTMLButtonElement>('[aria-label="暂停画面"]')!.click()
    await flushPromises()
    expect(document.querySelector('.is-paused')).not.toBeNull()
    await click('跳过序章')
    expect(document.querySelector('dialog')?.open).toBe(false)
    expect(mocks.push).not.toHaveBeenCalled()
    expect(mocks.route.fullPath).toBe('/keys?create=1')
    expect(document.body.style.overflow).toBe('')
  })

  it('guides to an existing destination after the story', async () => {
    await render()
    await click('听听这座城的故事')
    expect(document.body.textContent).toContain('这里有人可以接班')
    await click('翻开我的这一页')
    await click('开始创作')
    expect(mocks.push).toHaveBeenCalledWith('/operator')
    expect(useCityArrivalStore().showStory).toBe(false)
  })

  it('retains the story and allows retry on navigation failure', async () => {
    mocks.push.mockRejectedValueOnce(new Error('offline'))
    await render()
    await click('你的第一站')
    await click('寻找资源')
    expect(document.querySelector('[role="alert"]')?.textContent).toContain('暂时没能打开目的地')
    expect(useCityArrivalStore().showStory).toBe(true)
    await click('寻找资源')
    expect(mocks.push).toHaveBeenLastCalledWith('/account-square')
    expect(useCityArrivalStore().showStory).toBe(false)
  })

  it('supports escape dismissal and restores body scrolling', async () => {
    await render()
    document.querySelector('dialog')!.dispatchEvent(new Event('cancel', { cancelable: true }))
    await flushPromises()
    expect(useCityArrivalStore().showStory).toBe(false)
    expect(document.body.style.overflow).toBe('')
  })

  it('does not interrupt login callbacks or admin routes', async () => {
    mocks.route.meta.requiresAuth = false
    await render()
    expect(useCityArrivalStore().showStory).toBe(false)
    mocks.route.path = '/admin/dashboard'
    mocks.route.meta.requiresAuth = true
    await flushPromises()
    expect(useCityArrivalStore().showStory).toBe(false)
    mocks.route.path = '/community'
    await flushPromises()
    expect(document.querySelector('dialog')?.open).toBe(true)
  })

  it('clears the old account experience on logout and starts separately for the next account', async () => {
    await render()
    await click('跳过序章')
    mocks.auth.user = null
    mocks.auth.isAuthenticated = false
    await flushPromises()
    expect(useCityArrivalStore().userId).toBeNull()
    mocks.auth.user = { id: 6, username: '另一位城民' }
    mocks.auth.isAuthenticated = true
    await flushPromises()
    expect(document.querySelector('dialog')?.open).toBe(true)
    expect(document.body.textContent).toContain('另一位城民')
  })
})
