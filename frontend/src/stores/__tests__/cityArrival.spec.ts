import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useCityArrivalStore } from '../cityArrival'

describe('city arrival persistence', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    localStorage.clear()
    sessionStorage.clear()
    setActivePinia(createPinia())
  })

  it('welcomes first-time visitors without marking an unfinished story completed', () => {
    const arrival = useCityArrivalStore()
    arrival.visit(1)
    expect(arrival.showStory).toBe(true)
    expect(localStorage.getItem('zero-city:arrival:v1:1')).toBeNull()
    expect(sessionStorage.getItem('zero-city:visit:v1:1')).toBeNull()
  })

  it('completion and skip persist without repeating on page changes or reload', () => {
    const arrival = useCityArrivalStore()
    arrival.visit(1)
    arrival.finish()
    arrival.visit(1)
    expect(arrival.showStory).toBe(false)
    expect(arrival.showGreeting).toBe(false)
    setActivePinia(createPinia())
    const restored = useCityArrivalStore()
    restored.visit(1)
    expect(restored.showStory).toBe(false)
    expect(restored.showGreeting).toBe(false)
  })

  it('offers a non-modal greeting in a new session and allows deliberate replay', () => {
    const arrival = useCityArrivalStore()
    arrival.visit(1)
    arrival.finish()
    arrival.resetSession()
    arrival.visit(1)
    expect(arrival.showStory).toBe(false)
    expect(arrival.showGreeting).toBe(true)
    arrival.replay(1)
    expect(arrival.showGreeting).toBe(false)
    expect(arrival.showStory).toBe(true)
  })

  it('does not share completion across accounts', () => {
    const arrival = useCityArrivalStore()
    arrival.visit(1)
    arrival.finish()
    arrival.visit(2)
    expect(arrival.showStory).toBe(true)
    expect(arrival.userId).toBe(2)
    expect(localStorage.getItem('zero-city:arrival:v1:2')).toBeNull()
  })

  it('continues to work with disabled browser storage', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('denied') })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('denied') })
    vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => { throw new Error('denied') })
    const arrival = useCityArrivalStore()
    expect(() => arrival.visit(1)).not.toThrow()
    arrival.finish()
    arrival.visit(1)
    expect(arrival.showStory).toBe(false)
    arrival.resetSession()
    arrival.visit(1)
    expect(arrival.showGreeting).toBe(true)
  })
})
