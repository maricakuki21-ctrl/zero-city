import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import { nextTick } from 'vue'
import ZeroCityRankingDetailsDialog from '../ZeroCityRankingDetailsDialog.vue'
import { defaultZeroCityRankingConfig } from '@/features/bizdecipher/data/zeroCityRankings'

afterEach(() => {
  document.body.innerHTML = ''
})

describe('ZeroCityRankingDetailsDialog', () => {
  it('shows actual scope and rows, then emits profile selection and related navigation', async () => {
    const wrapper = mount(ZeroCityRankingDetailsDialog, {
      attachTo: document.body,
      props: {
        open: true,
        definition: defaultZeroCityRankingConfig.items[1],
        rows: [{ rank: 1, label: '林默', value: '86 贡献', secondary: '4 条动态', userId: 7 }],
      },
    })
    await nextTick()

    expect(document.body.textContent).toContain('基于当前可见动态')
    expect(document.body.textContent).toContain('林默')
    const row = document.body.querySelector<HTMLButtonElement>('.ranking-dialog-list button')
    row?.click()
    expect(wrapper.emitted('select')?.[0]?.[0]).toMatchObject({ userId: 7 })
    document.body.querySelector<HTMLButtonElement>('footer button')?.click()
    expect(wrapper.emitted('related')).toHaveLength(1)
    wrapper.unmount()
  })

  it('keeps an honest empty state and closes through Escape', async () => {
    const wrapper = mount(ZeroCityRankingDetailsDialog, {
      attachTo: document.body,
      props: { open: true, definition: defaultZeroCityRankingConfig.items[0], rows: [] },
    })
    await nextTick()

    expect(document.body.textContent).toContain('暂无可验证记录')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    expect(wrapper.emitted('update:open')?.some(([open]) => open === false)).toBe(true)
    wrapper.unmount()
  })
})
