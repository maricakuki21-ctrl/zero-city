import { createPinia, setActivePinia } from 'pinia'
import type { Driver } from 'driver.js'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useOnboardingStore } from '@/stores/onboarding'

function asDriver(value: { destroy: () => void }): Driver {
  return value as unknown as Driver
}

describe('useOnboardingStore driver ownership', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('destroys the previous driver before constructing its replacement', () => {
    const store = useOnboardingStore()
    const replacement = asDriver({ destroy: vi.fn() })
    const events: string[] = []
    const cleanup = vi.fn(() => {
      events.push('cleanup previous')
    })
    const previous = asDriver({
      destroy: vi.fn(() => {
        events.push('destroy previous')
      }),
    })

    store.replaceDriverInstance(() => previous, cleanup)
    events.length = 0
    vi.mocked(previous.destroy).mockClear()
    const result = store.replaceDriverInstance(() => {
      events.push('construct replacement')
      return replacement
    })

    expect(previous.destroy).toHaveBeenCalledOnce()
    expect(cleanup).toHaveBeenCalledOnce()
    expect(events).toEqual([
      'cleanup previous',
      'destroy previous',
      'construct replacement',
    ])
    expect(result).toBe(replacement)
    expect(store.getDriverInstance()).toBe(replacement)
  })

  it('only lets the owning driver clear the global reference', () => {
    const store = useOnboardingStore()
    const previous = asDriver({ destroy: vi.fn() })
    const current = asDriver({ destroy: vi.fn() })
    const cleanup = vi.fn()

    store.replaceDriverInstance(() => current, cleanup)
    store.clearDriverInstance(previous)

    expect(store.getDriverInstance()).toBe(current)
    expect(cleanup).not.toHaveBeenCalled()

    store.clearDriverInstance(current)
    expect(store.getDriverInstance()).toBeNull()
    expect(cleanup).toHaveBeenCalledOnce()
  })

  it('constructs and stores a driver when no previous owner exists', () => {
    const store = useOnboardingStore()
    const current = asDriver({ destroy: vi.fn() })
    const createDriver = vi.fn(() => current)

    const result = store.replaceDriverInstance(createDriver)

    expect(createDriver).toHaveBeenCalledOnce()
    expect(current.destroy).not.toHaveBeenCalled()
    expect(result).toBe(current)
    expect(store.getDriverInstance()).toBe(current)
  })

  it('only keeps the newest driver start request current', () => {
    const store = useOnboardingStore()
    const first = store.beginDriverStartRequest()
    const second = store.beginDriverStartRequest()

    expect(store.isDriverStartRequestCurrent(first)).toBe(false)
    expect(store.isDriverStartRequestCurrent(second)).toBe(true)

    store.cancelDriverStartRequest(first)
    expect(store.isDriverStartRequestCurrent(second)).toBe(true)

    store.cancelDriverStartRequest(second)
    expect(store.isDriverStartRequestCurrent(second)).toBe(false)
  })
})
