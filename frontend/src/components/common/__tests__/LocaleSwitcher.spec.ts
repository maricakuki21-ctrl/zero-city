import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'

import LocaleSwitcher from '../LocaleSwitcher.vue'

const waitForTransition = () => new Promise(resolve => setTimeout(resolve, 180))

const mocks = vi.hoisted(() => {
  const locale = { value: 'en' }
  const setLocale = vi.fn(async (code: string) => {
    locale.value = code
  })

  return { locale, setLocale }
})

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    locale: mocks.locale
  })
}))

vi.mock('@/i18n', () => ({
  availableLocales: [
    { code: 'en', name: 'English', flag: 'US' },
    { code: 'zh', name: '中文', flag: 'CN' }
  ],
  setLocale: mocks.setLocale
}))

afterEach(() => {
  document.body.innerHTML = ''
  mocks.locale.value = 'en'
  mocks.setLocale.mockClear()
  vi.restoreAllMocks()
})

describe('LocaleSwitcher', () => {
  function mountSwitcher() {
    const host = document.createElement('div')
    document.body.appendChild(host)

    const wrapper = mount(LocaleSwitcher, {
      attachTo: host,
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    return { host, wrapper }
  }

  it('renders the open dropdown in the document top layer', async () => {
    const { host, wrapper } = mountSwitcher()

    await wrapper.find('button').trigger('click')
    await nextTick()

    const dropdown = document.body.querySelector('.locale-switcher-dropdown')
    expect(dropdown).toBeTruthy()
    expect(host.contains(dropdown)).toBe(false)

    wrapper.unmount()
    host.remove()
  })

  it('positions the teleported dropdown from the trigger bounds', async () => {
    const { host, wrapper } = mountSwitcher()
    const container = wrapper.element as HTMLElement
    vi.spyOn(container, 'getBoundingClientRect').mockReturnValue({
      x: 220,
      y: 40,
      width: 96,
      height: 32,
      top: 40,
      right: 316,
      bottom: 72,
      left: 220,
      toJSON: () => ({})
    } as DOMRect)

    await wrapper.find('button').trigger('click')
    await nextTick()

    const dropdown = document.body.querySelector<HTMLElement>('.locale-switcher-dropdown')
    expect(dropdown?.style.top).toBe('76px')
    expect(dropdown?.style.left).toBe('188px')

    wrapper.unmount()
    host.remove()
  })

  it('keeps the menu open for inside clicks and closes it for outside pointer presses or Escape', async () => {
    const { host, wrapper } = mountSwitcher()

    await wrapper.find('button').trigger('click')
    await nextTick()

    const dropdown = document.body.querySelector<HTMLElement>('.locale-switcher-dropdown')
    expect(dropdown).toBeTruthy()

    dropdown?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()
    expect(document.body.querySelector('.locale-switcher-dropdown')).toBeTruthy()

    document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    await waitForTransition()
    expect(document.body.querySelector('.locale-switcher-dropdown')).toBeNull()

    await wrapper.find('button').trigger('click')
    await nextTick()
    document.body.dispatchEvent(new Event('pointerdown', { bubbles: true, composed: true }))
    await nextTick()
    await waitForTransition()
    expect(document.body.querySelector('.locale-switcher-dropdown')).toBeNull()

    wrapper.unmount()
    host.remove()
  })
})
