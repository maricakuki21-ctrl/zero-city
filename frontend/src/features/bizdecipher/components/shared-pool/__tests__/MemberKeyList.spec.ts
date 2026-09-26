import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import MemberKeyList from '../MemberKeyList.vue'

const key = { id: 1, api_key_id: 7, pool_id: 1, pool_name: 'Pool A', name: 'My key', status: 'active', key: 'fixture-secret-only', key_preview: 'sk-share-••••' }

describe('MemberKeyList', () => {
  it('deduplicates bindings and masks the key until explicitly revealed', async () => {
    const wrapper = mount(MemberKeyList, { props: { keys: [key, { ...key, id: 2, pool_id: 2, pool_name: 'Pool B' }], deletingId: null } })
    expect(wrapper.findAll('article')).toHaveLength(1)
    expect(wrapper.text()).toContain('2 个池')
    expect(wrapper.text()).not.toContain(key.key)
    await wrapper.get('[aria-label="显示 Key"]').trigger('click')
    expect(wrapper.get('code').text()).toBe(key.key)
    await wrapper.get('[aria-label="复制完整 Key"]').trigger('click')
    expect(wrapper.emitted('copy')?.[0]).toEqual([key.key])
    await wrapper.setProps({ keys: [{ ...key }] })
    expect(wrapper.text()).not.toContain(key.key)
  })
  it('never copies a preview as a complete credential', async () => {
    const wrapper = mount(MemberKeyList, { props: { keys: [{ ...key, key: undefined, status: 'disabled' }], deletingId: null } })
    expect(wrapper.get('[aria-label="复制完整 Key"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[aria-label="显示 Key"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('已停用')
    expect(wrapper.emitted('copy')).toBeUndefined()
  })
})
