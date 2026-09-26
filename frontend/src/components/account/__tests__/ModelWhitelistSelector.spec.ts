import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import ModelWhitelistSelector from '../ModelWhitelistSelector.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => params ? `${key}:${JSON.stringify(params)}` : key
  })
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showInfo: vi.fn(),
    showSuccess: vi.fn(),
    showError: vi.fn()
  })
}))

vi.mock('@/api/admin/accounts', () => ({
  accountsAPI: {
    syncUpstreamModels: vi.fn(),
    syncUpstreamModelsPreview: vi.fn()
  }
}))

vi.mock('@/composables/useModelWhitelist', () => ({
  allModels: [
    { value: 'claude-sonnet-4', label: 'Claude Sonnet 4' },
    { value: 'gpt-4o', label: 'GPT-4o' }
  ],
  getModelsByPlatform: () => [
    { value: 'claude-sonnet-4', label: 'Claude Sonnet 4' }
  ]
}))

afterEach(() => {
  document.body.innerHTML = ''
})

describe('ModelWhitelistSelector', () => {
  it('renders the open dropdown in the document top layer', async () => {
    const host = document.createElement('div')
    document.body.appendChild(host)

    const wrapper = mount(ModelWhitelistSelector, {
      attachTo: host,
      props: {
        modelValue: [],
        platform: 'anthropic'
      },
      global: {
        stubs: {
          Icon: true,
          ModelIcon: true
        }
      }
    })

    await wrapper.find('.cursor-pointer').trigger('click')
    await wrapper.vm.$nextTick()

    const dropdown = document.body.querySelector('.model-dropdown')
    expect(dropdown).toBeTruthy()
    expect(host.contains(dropdown)).toBe(false)

    wrapper.unmount()
    host.remove()
  })
})
