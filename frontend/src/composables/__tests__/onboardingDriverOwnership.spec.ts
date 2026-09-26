import { createPinia, setActivePinia } from 'pinia'
import { defineComponent, nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useOnboardingStore } from '@/stores/onboarding'
import { useOnboardingTour } from '../useOnboardingTour'
import { useSharedPoolOnboarding } from '../useSharedPoolOnboarding'

type MockDriver = {
  config: Record<string, any>
  active: boolean
  drive: ReturnType<typeof vi.fn>
  destroy: ReturnType<typeof vi.fn>
  isActive: ReturnType<typeof vi.fn>
  moveNext: ReturnType<typeof vi.fn>
  movePrevious: ReturnType<typeof vi.fn>
  getActiveIndex: ReturnType<typeof vi.fn>
  getActiveElement: ReturnType<typeof vi.fn>
}

const mockState = vi.hoisted(() => ({
  instances: [] as MockDriver[],
  currentConfig: null as Record<string, any> | null,
  active: false,
  events: [] as string[],
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { id: 42, role: 'admin' },
    isSimpleMode: false,
  }),
}))

vi.mock('@/components/Guide/steps', () => ({
  getAdminSteps: () => [
    { popover: { title: 'site guide' } },
    { element: '#slow-site-guide-target', popover: { title: 'slow site guide' } },
  ],
  getUserSteps: () => [
    { popover: { title: 'site guide' } },
    { element: '#slow-site-guide-target', popover: { title: 'slow site guide' } },
  ],
}))

vi.mock('@/components/shared-pool/sharedPoolGuide', () => ({
  getSharedPoolGuideSteps: () => [{ popover: { title: 'shared pool guide' } }],
  hasSeenSharedPoolGuide: () => false,
  markSharedPoolGuideSeen: vi.fn(),
}))

vi.mock('driver.js', () => ({
  driver: (config: Record<string, any>): MockDriver => {
    const id = mockState.instances.length
    mockState.currentConfig = config
    mockState.events.push(`construct ${id}`)
    const instance = {
      config,
      active: false,
      drive: vi.fn(() => {
        mockState.active = true
        mockState.instances.forEach(driverInstance => {
          driverInstance.active = false
        })
        instance.active = true
      }),
      destroy: vi.fn(() => {
        if (!mockState.active) return
        mockState.events.push(`destroy ${id}`)
        mockState.active = false
        mockState.instances.forEach(driverInstance => {
          driverInstance.active = false
        })
        const activeConfig = mockState.currentConfig
        activeConfig?.onDestroyed?.(undefined, undefined, {
          config: activeConfig,
          state: {},
          driver: instance,
        })
      }),
      isActive: vi.fn(() => mockState.active),
      moveNext: vi.fn(),
      movePrevious: vi.fn(),
      getActiveIndex: vi.fn(() => 0),
      getActiveElement: vi.fn(() => null),
    } as MockDriver
    mockState.instances.push(instance)
    return instance
  },
}))

const GuideHarness = defineComponent({
  props: {
    siteAutoStart: { type: Boolean, default: false },
  },
  setup(props) {
    const siteGuide = useOnboardingTour({ autoStart: props.siteAutoStart })
    const sharedPoolGuide = useSharedPoolOnboarding({ scope: 'market', autoStart: false })
    return {
      startSiteGuide: siteGuide.startTour,
      startSharedPoolGuide: sharedPoolGuide.startGuide,
      nextSiteStep: siteGuide.nextStep,
    }
  },
  template: '<div />',
})

describe('onboarding driver ownership across guides', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockState.instances.length = 0
    mockState.currentConfig = null
    mockState.active = false
    mockState.events.length = 0
    localStorage.clear()
  })

  afterEach(() => {
    mockState.instances.forEach((instance) => {
      if (instance.active) instance.destroy()
    })
    vi.useRealTimers()
    vi.clearAllMocks()
  })

  it('keeps exactly one driver when site and shared-pool guides replace each other', async () => {
    const wrapper = mount(GuideHarness)
    await nextTick()
    const store = useOnboardingStore()
    const api = wrapper.vm as unknown as {
      startSiteGuide: (startIndex?: number) => Promise<void>
      startSharedPoolGuide: (replaceActive?: boolean) => Promise<boolean>
      nextSiteStep: (delay?: number) => Promise<void>
    }

    await api.startSiteGuide()
    const firstSiteDriver = mockState.instances[0]
    expect(store.getDriverInstance()).toBe(firstSiteDriver)
    expect(firstSiteDriver.active).toBe(true)

    await expect(api.startSharedPoolGuide(true)).resolves.toBe(true)
    const poolDriver = mockState.instances[1]
    expect(firstSiteDriver.destroy).toHaveBeenCalledOnce()
    expect(store.getDriverInstance()).toBe(poolDriver)
    expect(mockState.instances.filter(instance => instance.active)).toEqual([poolDriver])
    expect(mockState.events).toEqual(['construct 0', 'destroy 0', 'construct 1'])

    firstSiteDriver.config.onDestroyed?.()
    expect(store.getDriverInstance()).toBe(poolDriver)

    await api.startSiteGuide()
    const secondSiteDriver = mockState.instances[2]
    expect(poolDriver.destroy).toHaveBeenCalledOnce()
    expect(store.getDriverInstance()).toBe(secondSiteDriver)
    expect(mockState.instances.filter(instance => instance.active)).toEqual([secondSiteDriver])
    expect(mockState.events).toEqual([
      'construct 0',
      'destroy 0',
      'construct 1',
      'destroy 1',
      'construct 2',
    ])

    firstSiteDriver.config.onDestroyed?.()
    poolDriver.config.onDestroyed?.()
    expect(store.getDriverInstance()).toBe(secondSiteDriver)

    await api.nextSiteStep(0)
    expect(secondSiteDriver.moveNext).toHaveBeenCalledOnce()

    secondSiteDriver.destroy()
    wrapper.unmount()
  })

  it('does not let an older waiting site request replace a newer site request', async () => {
    vi.useFakeTimers()
    const wrapper = mount(GuideHarness)
    await nextTick()
    const store = useOnboardingStore()
    const api = wrapper.vm as unknown as {
      startSiteGuide: (startIndex?: number) => Promise<void>
    }

    const staleStart = api.startSiteGuide(1)
    await nextTick()
    await Promise.resolve()
    expect(mockState.instances).toHaveLength(0)

    await api.startSiteGuide(0)
    const newestDriver = mockState.instances[0]

    await vi.advanceTimersByTimeAsync(150)
    await staleStart

    expect(mockState.instances).toHaveLength(1)
    expect(store.getDriverInstance()).toBe(newestDriver)
    expect(newestDriver.destroy).not.toHaveBeenCalled()

    newestDriver.destroy()
    wrapper.unmount()
  })

  it('does not let an older waiting site request replace a shared-pool guide', async () => {
    vi.useFakeTimers()
    const wrapper = mount(GuideHarness)
    await nextTick()
    const store = useOnboardingStore()
    const api = wrapper.vm as unknown as {
      startSiteGuide: (startIndex?: number) => Promise<void>
      startSharedPoolGuide: (replaceActive?: boolean) => Promise<boolean>
    }

    const staleStart = api.startSiteGuide(1)
    await nextTick()
    await Promise.resolve()
    expect(mockState.instances).toHaveLength(0)

    await expect(api.startSharedPoolGuide(true)).resolves.toBe(true)
    const poolDriver = mockState.instances[0]

    await vi.advanceTimersByTimeAsync(150)
    await staleStart

    expect(mockState.instances).toHaveLength(1)
    expect(store.getDriverInstance()).toBe(poolDriver)
    expect(poolDriver.destroy).not.toHaveBeenCalled()

    poolDriver.destroy()
    wrapper.unmount()
  })

  it('does not let site auto-start replace a manually opened shared-pool guide', async () => {
    vi.useFakeTimers()
    const wrapper = mount(GuideHarness, { props: { siteAutoStart: true } })
    await nextTick()
    const store = useOnboardingStore()
    const api = wrapper.vm as unknown as {
      startSharedPoolGuide: (replaceActive?: boolean) => Promise<boolean>
    }

    await expect(api.startSharedPoolGuide(true)).resolves.toBe(true)
    const poolDriver = mockState.instances[0]

    await vi.advanceTimersByTimeAsync(1100)

    expect(mockState.instances).toHaveLength(1)
    expect(store.getDriverInstance()).toBe(poolDriver)
    expect(poolDriver.destroy).not.toHaveBeenCalled()

    poolDriver.destroy()
    wrapper.unmount()
  })
})
