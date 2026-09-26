import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import ZeroCityActivityBoard from '../ZeroCityActivityBoard.vue'

const post = {
  id: 1, title: '真实议题记录', card: '自选昵称', time: '刚刚',
  excerpt: '记录摘要', body: '真实正文', replies: 2, views: 9, tags: ['产品'],
}

afterEach(() => { document.body.innerHTML = '' })

describe('ZeroCityActivityBoard', () => {
  it('uses vote language and only exposes the vote action for the vote hall', async () => {
    const wrapper = mount(ZeroCityActivityBoard, { props: { kind: 'votes', title: '投票大厅', posts: [], loading: false, error: false } })
    expect(wrapper.text()).toContain('城市共识')
    expect(wrapper.text()).toContain('暂无开放议题')
    await wrapper.get('.city-activity-primary').trigger('click')
    expect(wrapper.emitted('create')).toHaveLength(1)
    expect(wrapper.find('.city-activity-records').exists()).toBe(false)
    wrapper.unmount()
  })

  it('does not invent badges and routes to the real card album', async () => {
    const wrapper = mount(ZeroCityActivityBoard, { props: { kind: 'badges', title: '勋章墙', posts: [], loading: false, error: false } })
    expect(wrapper.text()).toContain('还没有可验证的授予记录')
    await wrapper.get('.city-activity-secondary').trigger('click')
    expect(wrapper.emitted('cards')).toHaveLength(1)
    expect(wrapper.text()).not.toContain('铜牌')
    wrapper.unmount()
  })

  it('renders rules as expandable records without a publish action', async () => {
    const wrapper = mount(ZeroCityActivityBoard, { props: { kind: 'rules', title: '规则公示', posts: [post], loading: false, error: false } })
    expect(wrapper.text()).toContain('已发布记录')
    expect(wrapper.text()).not.toContain('发帖')
    await wrapper.get('summary').trigger('click')
    expect(wrapper.text()).toContain('真实正文')
    wrapper.unmount()
  })

  it('keeps loading and failed states actionable', async () => {
    const wrapper = mount(ZeroCityActivityBoard, { props: { kind: 'rules', title: '规则公示', posts: [], loading: true, error: false } })
    expect(wrapper.text()).toContain('正在读取真实记录')
    await wrapper.setProps({ loading: false, error: true })
    expect(wrapper.text()).toContain('重新读取')
    await wrapper.get('.city-activity-secondary').trigger('click')
    expect(wrapper.emitted('retry')).toHaveLength(1)
    wrapper.unmount()
  })
  it('finds a published record by author and recovers from no matches', async () => {
    const wrapper = mount(ZeroCityActivityBoard, { props: { kind: 'rules', title: '规则公示', posts: [post, { ...post, id: 2, title: '新版规则', card: '另一发布者' }], loading: false, error: false } })
    await wrapper.get('input[type="search"]').setValue('另一发布者')
    expect(wrapper.findAll('details')).toHaveLength(1)
    expect(wrapper.get('details').text()).toContain('新版规则')
    expect(wrapper.get('[role="status"]').text()).toContain('当前已加载 2 条，匹配 1 条')
    await wrapper.get('input[type="search"]').setValue('不存在')
    expect(wrapper.findAll('details')).toHaveLength(0)
    await wrapper.get('.city-activity-state button').trigger('click')
    expect(wrapper.findAll('details')).toHaveLength(2)
    wrapper.unmount()
  })
  it('shows record provenance without presenting a contribution as an awarded badge', () => {
    const wrapper = mount(ZeroCityActivityBoard, { props: { kind: 'badges', title: '勋章墙', posts: [{ ...post, created_at: '2026-09-10T01:00:00Z' }], loading: false, error: false } })
    expect(wrapper.text()).toContain('不代表已获勋章或已开通权限')
    expect(wrapper.get('.activity-record-source').text()).toContain('社区记录 #1')
    expect(wrapper.get('time').text()).toContain('2026')
    expect(wrapper.get('.activity-record-source').text()).toContain('自选昵称')
    wrapper.unmount()
  })
  it('does not invent a publication time when supplied date is invalid', () => {
    const wrapper = mount(ZeroCityActivityBoard, { props: { kind: 'rules', title: '规则公示', posts: [{ ...post, created_at: 'invalid' }], loading: false, error: false } })
    expect(wrapper.get('time').text()).toBe('时间未记录')
    wrapper.unmount()
  })
})
