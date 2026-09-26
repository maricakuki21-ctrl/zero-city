import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import AssetEditorDialog from '../AssetEditorDialog.vue'

describe('AssetEditorDialog save lock', () => {
  it('disables every editable field while a save is in flight', () => {
    const wrapper = mount(AssetEditorDialog, {
      props: { open: true, mode: 'create', saving: true },
      global: { stubs: { Icon: { template: '<span />' } } },
    })

    expect(wrapper.get('fieldset').attributes('disabled')).toBeDefined()
    expect(wrapper.findAll('input, select, textarea').every(control => control.element.matches(':disabled'))).toBe(true)
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()
  })
})
