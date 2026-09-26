import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import ZeroCityChronicle from '../ZeroCityChronicle.vue'

const mounted: VueWrapper[] = []

async function render(query = '') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/zero-city/chronicle', component: { template: '<div />' } },
      { path: '/zero-city/cards', component: { template: '<div />' } },
    ],
  })
  await router.push(`/zero-city/chronicle${query}`)
  await router.isReady()
  const wrapper = mount(ZeroCityChronicle, { attachTo: document.body, global: { plugins: [router] } })
  mounted.push(wrapper)
  await flushPromises()
  return { wrapper, router }
}

afterEach(() => {
  mounted.splice(0).forEach(wrapper => wrapper.unmount())
  document.body.innerHTML = ''
})

describe('ZeroCityChronicle', () => {
  it('shows authored fiction and six usable stories without fake rewards or user activity', async () => {
    const { wrapper } = await render()
    expect(wrapper.get('h1').text()).toBe('城市编年史')
    expect(wrapper.findAll('.chronicle-timeline button')).toHaveLength(6)
    expect(wrapper.get('.chronicle-colophon').text()).toContain('零历为虚构纪年')
    expect(wrapper.get('.chronicle-colophon').text()).toContain('不代表真实用户行为')
    expect(wrapper.text()).not.toContain('领取奖励')
    expect(wrapper.find('a[href="/zero-city/cards"]').exists()).toBe(true)
  })

  it('filters by era and search, then clears a real empty state', async () => {
    const { wrapper } = await render()
    await wrapper.get('select').setValue('bridge')
    expect(wrapper.findAll('.chronicle-timeline button')).toHaveLength(2)
    await wrapper.get('input[type="search"]').setValue('没人知道的编号')
    expect(wrapper.find('.chronicle-story').exists()).toBe(false)
    expect(wrapper.get('.chronicle-empty').text()).toContain('没有找到这段故事')
    await wrapper.get('.chronicle-empty button').trigger('click')
    expect(wrapper.findAll('.chronicle-timeline button')).toHaveLength(6)
  })

  it('deep links to a story and navigates within the filtered chapter list', async () => {
    const { wrapper, router } = await render('?story=the-answer-across-water')
    expect(wrapper.get('#chronicle-story-title').text()).toBe('没有回音的另一岸')
    await wrapper.get('select').setValue('bridge')
    expect(wrapper.get('.story-pagination button').attributes('disabled')).toBeDefined()
    await wrapper.findAll('.story-pagination button')[1].trigger('click')
    await flushPromises()
    expect(wrapper.get('#chronicle-story-title').text()).toBe('一座允许坐下的桥')
    expect(router.currentRoute.value.query.story).toBe('a-bridge-with-benches')
    expect(wrapper.findAll('.chronicle-timeline button')).toHaveLength(2)
  })

  it('opens real character details, follows a relation, and links the exact card', async () => {
    const { wrapper, router } = await render('?story=the-last-three-percent')
    await wrapper.get('.story-cast button').trigger('click')
    await flushPromises()
    let dialog = document.querySelector('[role="dialog"]')!
    expect(dialog.textContent).toContain('低电量小人')
    expect(dialog.textContent).toContain('微光供电所楼下')
    expect(dialog.querySelector('a')?.getAttribute('href')).toBe('/zero-city/cards?card=low_battery_sprite')
    const relation = Array.from(dialog.querySelectorAll('button')).find(button => button.textContent?.trim() === '希望夜班员')!
    relation.click()
    await flushPromises()
    dialog = document.querySelector('[role="dialog"]')!
    expect(dialog.querySelector('h2')?.textContent).toBe('希望夜班员')
    expect(router.currentRoute.value.query.resident).toBe('hope_night_shift')
    ;(dialog.querySelector('[aria-label="关闭居民档案"]') as HTMLButtonElement).click()
    await flushPromises()
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    expect(router.currentRoute.value.query.resident).toBeUndefined()
  })

  it('opens a resident from a shared URL and reveals an associated story', async () => {
    const { wrapper, router } = await render('?resident=blue_hour_operator')
    const dialog = document.querySelector('[role="dialog"]')!
    expect(dialog.textContent).toContain('蓝时信号员')
    ;(dialog.querySelector('.resident-stories button') as HTMLButtonElement).click()
    await flushPromises()
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    expect(wrapper.get('#chronicle-story-title').text()).toBe('没有回音的另一岸')
    expect(router.currentRoute.value.query.story).toBe('the-answer-across-water')
  })

  it('follows a scene into its district and shows its actual residents', async () => {
    const { wrapper } = await render('?story=the-answer-across-water')
    await wrapper.get('.story-trace button').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('.chronicle-district')).toHaveLength(1)
    expect(wrapper.get('.chronicle-district h2').text()).toBe('蓝时海岸')
    expect(wrapper.get('.district-residents').text()).toContain('蓝时信号员')
    expect(wrapper.get('.district-residents').text()).toContain('零点灯塔守灯人')
  })

  it('switches views using keyboard activation and searches canonical residents', async () => {
    const { wrapper } = await render()
    const residentTab = wrapper.findAll('[role="tab"]').find(element => element.text() === '居民')!
    await residentTab.trigger('keydown', { key: 'Enter' })
    await flushPromises()
    expect(wrapper.findAll('.resident-entry')).toHaveLength(8)
    await wrapper.get('input[type="search"]').setValue('档案员')
    expect(wrapper.findAll('.resident-entry')).toHaveLength(1)
    expect(wrapper.get('.resident-entry').text()).toContain('桌面灰尘档案员')
  })

  it('ignores invalid story and character parameters without blanking the page', async () => {
    const { wrapper } = await render('?story=missing&resident=missing')
    expect(wrapper.get('#chronicle-story-title').text()).toBe('地图上没有的一条路')
    expect(document.querySelector('[role="dialog"]')).toBeNull()
  })

  it('enters from the community resident link and preserves the district destination when closing details', async () => {
    const { wrapper, router } = await render('?view=residents&resident=hope_night_shift')
    expect(wrapper.findAll('.resident-entry')).toHaveLength(8)
    const districtButton = document.querySelector('.resident-dialog-body .chronicle-link') as HTMLButtonElement
    districtButton.click()
    await flushPromises()
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    expect(wrapper.get('.chronicle-district h2').text()).toBe('未打烊巷')
    expect(router.currentRoute.value.query.view).toBe('districts')
    expect(router.currentRoute.value.query.resident).toBeUndefined()
  })

  it.each([
    { view: 'residents', label: '居民', selector: '.resident-entry', count: 8 },
    { view: 'districts', label: '城区', selector: '.chronicle-district', count: 5 },
  ])('honors explicit $view on a fresh mixed story URL', async ({ view, label, selector, count }) => {
    const { wrapper } = await render(`?story=the-answer-across-water&view=${view}`)
    expect(wrapper.get('[role="tab"][data-state="active"]').text()).toBe(label)
    expect(wrapper.findAll(selector)).toHaveLength(count)
    expect(wrapper.find('.chronicle-story').exists()).toBe(false)
    const storyTab = wrapper.findAll('[role="tab"]').find(element => element.text() === '编年史')!
    await storyTab.trigger('keydown', { key: 'Enter' })
    await flushPromises()
    expect(wrapper.get('#chronicle-story-title').text()).toBe('没有回音的另一岸')
  })

  it.each([
    { view: 'residents', label: '居民', selector: '.resident-entry', count: 8 },
    { view: 'districts', label: '城区', selector: '.chronicle-district', count: 5 },
  ])('honors explicit $view after external navigation and subsequent story-only changes', async ({ view, label, selector, count }) => {
    const { wrapper, router } = await render('?story=the-last-three-percent')
    await router.push(`/zero-city/chronicle?story=the-answer-across-water&view=${view}`)
    await flushPromises()
    expect(wrapper.get('[role="tab"][data-state="active"]').text()).toBe(label)
    expect(wrapper.findAll(selector)).toHaveLength(count)
    await router.push(`/zero-city/chronicle?story=a-bridge-with-benches&view=${view}`)
    await flushPromises()
    expect(wrapper.get('[role="tab"][data-state="active"]').text()).toBe(label)
    expect(wrapper.find('.chronicle-story').exists()).toBe(false)
    await router.push('/zero-city/chronicle?story=a-bridge-with-benches')
    await flushPromises()
    expect(wrapper.get('#chronicle-story-title').text()).toBe('一座允许坐下的桥')
  })

  it('still intentionally switches to a selected resident story from a mixed view URL', async () => {
    const { wrapper, router } = await render('?view=residents&story=the-answer-across-water&resident=hope_night_shift')
    expect(wrapper.get('[role="tab"][data-state="active"]').text()).toBe('居民')
    const storyButton = document.querySelector('.resident-stories button') as HTMLButtonElement
    storyButton.click()
    await flushPromises()
    expect(wrapper.get('[role="tab"][data-state="active"]').text()).toBe('编年史')
    expect(wrapper.get('#chronicle-story-title').text()).toBe('最后的 3% 留给谁')
    expect(router.currentRoute.value.query.view).toBeUndefined()
    expect(router.currentRoute.value.query.resident).toBeUndefined()
  })
})
