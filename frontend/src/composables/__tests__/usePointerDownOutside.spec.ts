import { defineComponent, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { usePointerDownOutside } from '../usePointerDownOutside'

const Harness = defineComponent({
  setup() {
    const root = ref<HTMLElement | null>(null)
    const portal = ref<HTMLElement | null>(null)
    const open = ref(false)
    usePointerDownOutside([root, portal], () => {
      open.value = false
    }, open)
    return { root, portal, open }
  },
  template: `
    <div>
      <div ref="root">
        <button type="button" data-testid="trigger" @click="open = true">打开</button>
        <div v-if="open" data-testid="panel">
          <input data-testid="search" />
        </div>
      </div>
      <Teleport to="body">
        <div v-if="open" ref="portal" data-testid="portal-panel">
          <input data-testid="portal-search" />
        </div>
      </Teleport>
      <input data-testid="protected" @pointerdown.stop />
      <button type="button" data-testid="outside">外部</button>
    </div>
  `
})

describe('usePointerDownOutside', () => {
  it('不会把打开控件同一次鼠标手势误判成外部点击', async () => {
    const wrapper = mount(Harness, { attachTo: document.body })
    const trigger = wrapper.get('[data-testid="trigger"]')

    await trigger.trigger('pointerdown')
    await trigger.trigger('mouseup')
    await trigger.trigger('click')

    expect(wrapper.find('[data-testid="panel"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('点击面板内输入框后保持焦点和打开状态', async () => {
    const wrapper = mount(Harness, { attachTo: document.body })
    await wrapper.get('[data-testid="trigger"]').trigger('click')
    const search = wrapper.get<HTMLInputElement>('[data-testid="search"]')

    await search.trigger('pointerdown')
    await search.trigger('mousedown')
    search.element.focus()
    await search.trigger('mouseup')
    await search.trigger('click')

    expect(wrapper.find('[data-testid="panel"]').exists()).toBe(true)
    expect(document.activeElement).toBe(search.element)
    wrapper.unmount()
  })

  it('允许交互控件阻止上层外部关闭处理', async () => {
    const wrapper = mount(Harness, { attachTo: document.body })
    await wrapper.get('[data-testid="trigger"]').trigger('click')

    await wrapper.get('[data-testid="protected"]').trigger('pointerdown')

    expect(wrapper.find('[data-testid="panel"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('把 Teleport 面板和触发器视为同一个交互边界', async () => {
    const wrapper = mount(Harness, { attachTo: document.body })
    await wrapper.get('[data-testid="trigger"]').trigger('click')
    const search = document.querySelector<HTMLInputElement>('[data-testid="portal-search"]')
    expect(search).not.toBeNull()

    search!.dispatchEvent(new Event('pointerdown', { bubbles: true }))
    search!.focus()
    search!.dispatchEvent(new MouseEvent('click', { bubbles: true }))

    expect(wrapper.find('[data-testid="panel"]').exists()).toBe(true)
    expect(document.activeElement).toBe(search)
    wrapper.unmount()
  })

  it('仅在按下发生在外部时关闭', async () => {
    const wrapper = mount(Harness, { attachTo: document.body })
    await wrapper.get('[data-testid="trigger"]').trigger('click')
    expect(wrapper.find('[data-testid="panel"]').exists()).toBe(true)

    await wrapper.get('[data-testid="outside"]').trigger('pointerdown')
    expect(wrapper.find('[data-testid="panel"]').exists()).toBe(false)
    wrapper.unmount()
  })
})
