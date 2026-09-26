import { defineComponent, nextTick } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { sharedPoolGuideStorageKey } from '@/components/shared-pool/sharedPoolGuide'
import { useSharedPoolOnboarding } from '../useSharedPoolOnboarding'

type MockDriver = {
  config: Record<string, any>
  active: boolean
  drive: ReturnType<typeof vi.fn>
  destroy: ReturnType<typeof vi.fn>
  isActive: ReturnType<typeof vi.fn>
  moveNext: ReturnType<typeof vi.fn>
  movePrevious: ReturnType<typeof vi.fn>
}

const mockState = vi.hoisted(() => ({
  activeDriver: null as MockDriver | null,
  instances: [] as MockDriver[],
  startRequestId: 0,
  userId: 42,
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ user: { id: mockState.userId } }),
}))

vi.mock('@/stores/onboarding', () => ({
  useOnboardingStore: () => ({
    beginDriverStartRequest: () => {
      mockState.startRequestId += 1
      return mockState.startRequestId
    },
    isDriverStartRequestCurrent: (requestId: number) => requestId === mockState.startRequestId,
    cancelDriverStartRequest: (requestId: number) => {
      if (requestId === mockState.startRequestId) mockState.startRequestId += 1
    },
    getDriverInstance: () => mockState.activeDriver,
    replaceDriverInstance: (createDriver: () => MockDriver) => {
      const previous = mockState.activeDriver
      previous?.destroy()
      if (mockState.activeDriver === previous) mockState.activeDriver = null
      const value = createDriver()
      mockState.activeDriver = value
      return value
    },
    clearDriverInstance: (expected: MockDriver) => {
      if (mockState.activeDriver === expected) mockState.activeDriver = null
    },
    isDriverActive: () => mockState.activeDriver?.active ?? false,
  }),
}))

vi.mock('driver.js', () => ({
  driver: (config: Record<string, any>): MockDriver => {
    const instance = {
      config,
      active: false,
      drive: vi.fn(() => {
        instance.active = true
      }),
      destroy: vi.fn(() => {
        const wasActive = instance.active
        instance.active = false
        if (wasActive) {
          config.onDestroyed?.(undefined, undefined, {
            config,
            state: {},
            driver: instance,
          })
        }
      }),
      isActive: vi.fn(() => instance.active),
      moveNext: vi.fn(),
      movePrevious: vi.fn(),
    } as MockDriver
    mockState.instances.push(instance)
    return instance
  },
}))

const GuideHarness = defineComponent({
  props: {
    autoStart: { type: Boolean, default: true },
  },
  setup(props) {
    return useSharedPoolOnboarding({ scope: 'market', autoStart: props.autoStart })
  },
  template: '<div />',
})

async function mountGuide(autoStart = true): Promise<VueWrapper> {
  const wrapper = mount(GuideHarness, { props: { autoStart } })
  await nextTick()
  return wrapper
}

async function advanceAutoStart(): Promise<void> {
  await vi.advanceTimersByTimeAsync(1500)
  await flushPromises()
}

describe('useSharedPoolOnboarding', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    localStorage.clear()
    mockState.activeDriver = null
    mockState.instances.length = 0
    mockState.startRequestId = 0
  })

  afterEach(() => {
    mockState.activeDriver?.destroy()
    mockState.activeDriver = null
    vi.clearAllTimers()
    vi.useRealTimers()
  })

  it('automatically opens once for a first-time user', async () => {
    const wrapper = await mountGuide()

    await advanceAutoStart()

    expect(mockState.instances).toHaveLength(1)
    expect(mockState.instances[0].drive).toHaveBeenCalledOnce()
    expect(mockState.instances[0].active).toBe(true)
    wrapper.unmount()
  })

  it('marks a skipped guide as seen and does not automatically reopen it', async () => {
    const first = await mountGuide()
    await advanceAutoStart()
    const guide = mockState.instances[0]

    guide.config.onCloseClick()

    expect(localStorage.getItem(sharedPoolGuideStorageKey('market', 42))).toBe('seen')
    expect(guide.destroy).toHaveBeenCalledOnce()
    first.unmount()

    const second = await mountGuide()
    await vi.advanceTimersByTimeAsync(5000)
    await flushPromises()

    expect(mockState.instances).toHaveLength(1)
    second.unmount()
  })

  it('allows a seen guide to be reopened manually', async () => {
    localStorage.setItem(sharedPoolGuideStorageKey('market', 42), 'seen')
    const wrapper = await mountGuide()
    await advanceAutoStart()
    expect(mockState.instances).toHaveLength(0)

    ;(wrapper.vm as unknown as { replayGuide: () => void }).replayGuide()
    await flushPromises()

    expect(mockState.instances).toHaveLength(1)
    expect(mockState.instances[0].drive).toHaveBeenCalledOnce()
    wrapper.unmount()
  })

  it('destroys an active guide when its component unmounts', async () => {
    const wrapper = await mountGuide()
    await advanceAutoStart()
    const guide = mockState.instances[0]

    wrapper.unmount()

    expect(guide.destroy).toHaveBeenCalledOnce()
    expect(guide.active).toBe(false)
    expect(mockState.activeDriver).toBeNull()
  })

  it('cancels a pending automatic start when its component unmounts', async () => {
    const wrapper = await mountGuide()
    wrapper.unmount()

    await vi.advanceTimersByTimeAsync(5000)
    await flushPromises()

    expect(mockState.instances).toHaveLength(0)
  })
})
