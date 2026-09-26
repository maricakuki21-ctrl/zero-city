import { ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { useAffiliateDiscovery } from './useAffiliateDiscovery'

const wrappers: VueWrapper[] = []
function render(id = ref(7)) {
  let guide!: ReturnType<typeof useAffiliateDiscovery>
  wrappers.push(mount({
    setup() {
      guide = useAffiliateDiscovery(id)
      return {}
    },
    template: '<div />',
  }))
  return guide
}

describe('invitation discovery hint', () => {
  beforeEach(() => { localStorage.clear() })
  afterEach(() => {
    wrappers.splice(0).forEach(wrapper => wrapper.unmount())
    vi.restoreAllMocks()
  })

  it('shows once per account and remembers a successful visit after remount', () => {
    const guide = render()
    expect(guide.pending.value).toBe(true)
    guide.markSeen()
    expect(guide.pending.value).toBe(false)
    expect(localStorage.getItem('bizdecipher:affiliate-discovery:v1:7')).toBe('seen')
    wrappers.splice(0).forEach(wrapper => wrapper.unmount())
    expect(render().pending.value).toBe(false)
  })

  it('synchronizes the sidebar and invitation page in the same window', () => {
    const sidebar = render()
    const page = render()
    page.markSeen()
    expect(sidebar.pending.value).toBe(false)
  })

  it('isolates accounts and ignores acknowledgements from a previous account', () => {
    const id = ref(7)
    const guide = render(id)
    guide.markSeen()
    id.value = 8
    expect(guide.pending.value).toBe(true)
    guide.markSeen(7)
    expect(guide.pending.value).toBe(true)
    expect(localStorage.getItem('bizdecipher:affiliate-discovery:v1:8')).toBeNull()
    id.value = 7
    expect(guide.pending.value).toBe(false)
  })

  it('updates from another tab without polling or calling any reward endpoint', () => {
    const guide = render()
    const key = 'bizdecipher:affiliate-discovery:v1:7'
    localStorage.setItem(key, 'seen')
    window.dispatchEvent(new StorageEvent('storage', { key, newValue: 'seen' }))
    expect(guide.pending.value).toBe(false)
  })

  it('does not show or save a guest hint', () => {
    const guide = render(ref(0))
    expect(guide.pending.value).toBe(false)
    guide.markSeen()
    expect(localStorage.length).toBe(0)
  })

  it('dismisses within the page when browser storage is unavailable', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('blocked') })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('blocked') })
    const sidebar = render()
    const page = render()
    expect(() => page.markSeen()).not.toThrow()
    expect(sidebar.pending.value).toBe(false)
    expect(page.pending.value).toBe(false)
  })
})
