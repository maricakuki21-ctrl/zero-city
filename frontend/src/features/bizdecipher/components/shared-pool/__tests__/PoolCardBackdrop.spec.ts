import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import PoolCardBackdrop from '../PoolCardBackdrop.vue'

describe('PoolCardBackdrop', () => {
  it('renders the selected card and exposes an honest recoverable failure', async () => {
    const wrapper = mount(PoolCardBackdrop, { props: { cardKey: 'zero_point_reconnector', rarity: 'mythic' } })
    expect(wrapper.get('img').attributes('src')).toContain('zero_point_reconnector')
    await wrapper.get('img').trigger('error')
    expect(wrapper.text()).toContain('已保留你的选择')
    expect(wrapper.find('img').exists()).toBe(false)
    await wrapper.get('button').trigger('click')
    expect(wrapper.get('img').attributes('src')).toContain('zero_point_reconnector')
  })
  it('does not invent a card without a selection', () => {
    expect(mount(PoolCardBackdrop).find('img').exists()).toBe(false)
  })
})
