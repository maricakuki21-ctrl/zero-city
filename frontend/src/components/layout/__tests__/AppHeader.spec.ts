import { shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AppHeader from '../AppHeader.vue'
import { createPinia, setActivePinia } from 'pinia'
import { useCityArrivalStore } from '@/stores/cityArrival'

const state = vi.hoisted(() => ({
  app: {
    publicSettingsLoaded: true,
    cachedPublicSettings: { payment_enabled: true as boolean | undefined },
    zeroCityTheme: 'day',
    docUrl: '',
  },
  auth: { user: { username: 'Tester', balance: 5, credit_balance: 10 } as object | null },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => state.app,
  useAuthStore: () => state.auth,
}))
vi.mock('@/stores/adminSettings', () => ({ useAdminSettingsStore: () => ({}) }))
vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/keys', query: {}, meta: {}, params: {} }),
  useRouter: () => ({ push: vi.fn() }),
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))
vi.mock('@/composables/usePointerDownOutside', () => ({ usePointerDownOutside: vi.fn() }))

function render() {
  return shallowMount(AppHeader, {
    global: {
      stubs: {
        transition: { template: '<div><slot /></div>' },
        RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
      },
    },
  })
}

describe('header account destinations', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    state.app.publicSettingsLoaded = true
    state.app.cachedPublicSettings.payment_enabled = true
    state.auth.user = { id: 7, username: 'Tester', balance: 5, credit_balance: 10 }
  })

  it.each([
    [true, true, true],
    [true, false, false],
    [false, false, true],
    [true, undefined, true],
  ])('matches payment route availability: loaded=%s enabled=%s', (loaded, enabled, visible) => {
    state.app.publicSettingsLoaded = loaded
    state.app.cachedPublicSettings.payment_enabled = enabled
    const wrapper = render()
    expect(wrapper.find('a[href="/purchase"]').exists()).toBe(visible)
    expect(wrapper.find('a[href="/wallet"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('does not expose account actions when signed out', () => {
    state.auth.user = null
    const wrapper = render()
    expect(wrapper.find('a[href="/purchase"]').exists()).toBe(false)
    expect(wrapper.find('a[href="/wallet"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('allows replay from the account menu', async () => {
    const wrapper = render()
    await wrapper.get('[aria-label="common.userMenu"]').trigger('click')
    const replay = wrapper.findAll('button').find(button => button.text().includes('重看入城故事'))!
    await replay.trigger('click')
    expect(useCityArrivalStore().userId).toBe(7)
    expect(useCityArrivalStore().showStory).toBe(true)
    expect(wrapper.find('.bd-account-menu').exists()).toBe(false)
    wrapper.unmount()
  })
})
