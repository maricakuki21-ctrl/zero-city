import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

describe('Turnstile script loading', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.useFakeTimers()
    document.head.innerHTML = ''
    delete window.turnstile
  })
  afterEach(() => {
    vi.useRealTimers()
    delete window.turnstile
  })
  it('shares one script without replacing global callbacks', async () => {
    const { loadTurnstileScript } = await import('../turnstileLoader')
    const callback = vi.fn()
    window.onTurnstileLoad = callback
    const first = loadTurnstileScript()
    const second = loadTurnstileScript()
    expect(first).toBe(second)
    expect(document.querySelectorAll('script')).toHaveLength(1)
    window.turnstile = { render: vi.fn(), reset: vi.fn(), remove: vi.fn() }
    document.querySelector('script')!.dispatchEvent(new Event('load'))
    await first
    expect(window.onTurnstileLoad).toBe(callback)
  })
  it('removes failed scripts so a later mount can retry', async () => {
    const { loadTurnstileScript } = await import('../turnstileLoader')
    const failure = expect(loadTurnstileScript()).rejects.toThrow('Failed to load')
    document.querySelector('script')!.dispatchEvent(new Event('error'))
    await failure
    expect(document.querySelector('script')).toBeNull()
    const retry = loadTurnstileScript()
    window.turnstile = { render: vi.fn(), reset: vi.fn(), remove: vi.fn() }
    document.querySelector('script')!.dispatchEvent(new Event('load'))
    await retry
  })
  it('times out instead of waiting forever', async () => {
    const { loadTurnstileScript } = await import('../turnstileLoader')
    const failure = expect(loadTurnstileScript()).rejects.toThrow('timed out')
    await vi.advanceTimersByTimeAsync(15000)
    await failure
    expect(document.querySelector('script')).toBeNull()
  })
})
