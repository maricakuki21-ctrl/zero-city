import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import MarketplaceView from '../MarketplaceView.vue'

describe('MarketplaceView', () => {
  it('is a compact route shell for the market experience', () => {
    const wrapper = mount(MarketplaceView, { global: { stubs: { AppLayout: { template: '<div data-layout><slot /></div>' }, MarketExperience: { template: '<section data-market />' } } } })
    expect(wrapper.find('[data-layout]').exists()).toBe(true); expect(wrapper.find('[data-market]').exists()).toBe(true)
  })
})
