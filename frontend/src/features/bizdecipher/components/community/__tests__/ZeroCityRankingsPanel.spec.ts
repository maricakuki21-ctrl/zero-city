import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ZeroCityRankingsPanel from '../ZeroCityRankingsPanel.vue'
import { defaultZeroCityRankingConfig } from '@/features/bizdecipher/data/zeroCityRankings'
import type { ZeroCityRankingRow } from '@/features/bizdecipher/data/zeroCityRankings'

const definitions = defaultZeroCityRankingConfig.items.slice(0, 3)

const rowsByKey: Readonly<Record<string, readonly ZeroCityRankingRow[]>> = {
  spending: [{ rank: 1, label: '消费先锋', value: '$120.00', secondary: '42 次有效付费调用' }],
  contribution: [{ rank: 1, label: '林默', value: '86 贡献', secondary: '4 条动态' }],
  answers: [{ rank: 1, label: 'Mira Studio', value: '64 回答', secondary: '3 条动态' }],
  assets: [{ rank: 1, label: '夜航船', value: '41 复用', secondary: '2 条动态' }],
}

describe('ZeroCityRankingsPanel', () => {
  it('moves and activates tabs with the ARIA keyboard model', async () => {
    const wrapper = mount(ZeroCityRankingsPanel, {
      attachTo: document.body,
      props: { definitions, rowsByKey },
    })
    const tabs = wrapper.findAll('[role="tab"]')

    expect(tabs.map((tab) => tab.attributes('tabindex'))).toEqual(['0', '-1', '-1'])

    await tabs[0]?.trigger('keydown', { key: 'ArrowRight' })
    expect(tabs[1]?.attributes('aria-selected')).toBe('true')
    expect(tabs[1]?.attributes('tabindex')).toBe('0')
    expect(document.activeElement).toBe(tabs[1]?.element)

    await tabs[1]?.trigger('keydown', { key: 'End' })
    expect(tabs[2]?.attributes('aria-selected')).toBe('true')
    expect(document.activeElement).toBe(tabs[2]?.element)

    wrapper.unmount()
  })

  it('keeps heading, tab and panel ids unique across instances', () => {
    const wrapper = mount({
      components: { ZeroCityRankingsPanel },
      setup() {
        return { definitions, rowsByKey }
      },
      template: `
        <div>
          <ZeroCityRankingsPanel :definitions="definitions" :rows-by-key="rowsByKey" />
          <ZeroCityRankingsPanel :definitions="definitions" :rows-by-key="rowsByKey" />
        </div>
      `,
    })

    const headingIds = wrapper.findAll('section[aria-labelledby]').map((section) => section.attributes('aria-labelledby'))
    const tabIds = wrapper.findAll('[role="tab"]').map((tab) => tab.attributes('id'))
    const panelIds = wrapper.findAll('[role="tabpanel"]').map((panel) => panel.attributes('id'))

    expect(new Set(headingIds).size).toBe(headingIds.length)
    expect(new Set(tabIds).size).toBe(tabIds.length)
    expect(new Set(panelIds).size).toBe(panelIds.length)

    wrapper.unmount()
  })

  it('shows a crown for first place and exposes ranking boundaries without reward promises', () => {
    const wrapper = mount(ZeroCityRankingsPanel, {
      props: { definitions, rowsByKey },
    })

    expect(wrapper.find('.rank-1 svg').exists()).toBe(true)
    expect(wrapper.text()).toContain('统计口径与资格待权威数据源确认')
    expect(wrapper.text()).not.toContain('随机稀有卡')

    wrapper.unmount()
  })

  it('keeps three vacant podium seats visible when no rows exist', () => {
    const wrapper = mount(ZeroCityRankingsPanel, {
      props: { definitions, rowsByKey: {}, variant: 'top' },
    })

    expect(wrapper.findAll('.podium-place')).toHaveLength(3)
    expect(wrapper.text()).toContain('席位待定')
  })

  it('opens details for a definition without a related route', async () => {
    const definitionWithoutRoute = { ...definitions[0]!, route: undefined }
    const wrapper = mount(ZeroCityRankingsPanel, {
      props: { definitions: [definitionWithoutRoute], rowsByKey, variant: 'top' },
    })

    await wrapper.get('.zero-city-ranking-more').trigger('click')
    expect(wrapper.emitted('open')?.[0]?.[0]).toMatchObject({ key: definitionWithoutRoute.key })
  })
})
