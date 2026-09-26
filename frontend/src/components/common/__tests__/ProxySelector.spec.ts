import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import ProxySelector from '../ProxySelector.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    proxies: {
      testProxy: vi.fn()
    }
  }
}))

afterEach(() => {
  document.body.innerHTML = ''
})

describe('ProxySelector', () => {
  it('renders the open dropdown in the document top layer', async () => {
    const host = document.createElement('div')
    document.body.appendChild(host)

    const wrapper = mount(ProxySelector, {
      attachTo: host,
      props: {
        modelValue: null,
        proxies: [
          {
            id: 1,
            name: 'Proxy A',
            protocol: 'http',
            host: '127.0.0.1',
            port: 8080
          }
        ] as any
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    await wrapper.find('.select-trigger').trigger('click')
    await wrapper.vm.$nextTick()

    const dropdown = document.body.querySelector('.select-dropdown')
    expect(dropdown).toBeTruthy()
    expect(host.contains(dropdown)).toBe(false)

    wrapper.unmount()
    host.remove()
  })
})
