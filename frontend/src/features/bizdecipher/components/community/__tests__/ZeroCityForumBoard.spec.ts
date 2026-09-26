import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import { h, nextTick } from 'vue'
import ZeroCityForumBoard from '../ZeroCityForumBoard.vue'

const posts = [
  { id: 1, user_id: 11, title: '旧话题', card: '自选昵称', time: '昨天', excerpt: '一个关于游戏的问题', body: '完整正文', replies: 8, views: 30, created_at: '2026-09-08T00:00:00Z' },
  { id: 2, user_id: 12, title: '新话题', card: '第二位居民', time: '刚刚', excerpt: '今天的发现', replies: 0, views: 1, created_at: '2026-09-09T00:00:00Z' },
]
function render(overrides = {}) {
  return mount(ZeroCityForumBoard, {
    attachTo: document.body,
    props: { title: '闲聊广场', district: '闲聊广场', posts, loading: false, error: false, excerpts: true, ...overrides },
    slots: { detail: ({ post }: { post: typeof posts[number] }) => h('p', { class: 'detail-fixture' }, post.body || post.excerpt) },
  })
}
afterEach(() => { document.body.innerHTML = '' })

describe('ZeroCityForumBoard', () => {
  it('explains restricted participation before opening the composer', async () => {
    const wrapper = render({ posts: [], canCreate: false, participationNote: '需要 L1 参与资格' })
    expect(wrapper.get('.city-forum-participation').text()).toBe('需要 L1 参与资格')
    expect(wrapper.get('.city-forum-primary').text()).toContain('参与资格')
    expect(wrapper.text()).not.toContain('发布第一篇')
    await wrapper.get('.city-forum-primary').trigger('click')
    expect(wrapper.emitted('create')).toHaveLength(1)
    wrapper.unmount()
  })

  it('shows latest first and renders no full bodies or replies in the list', () => {
    const wrapper = render()
    expect(wrapper.findAll('.city-forum-topic-title').map(item => item.text())).toEqual(['新话题', '旧话题'])
    expect(wrapper.text()).not.toContain('完整正文')
    expect(wrapper.findAll('h1')).toHaveLength(1)
    expect(wrapper.find('form').exists()).toBe(false)
    wrapper.unmount()
  })

  it('sorts hot by replies and filters unanswered without mutating source order', async () => {
    const wrapper = render()
    await wrapper.findAll('.city-forum-modes button')[1]?.trigger('click')
    expect(wrapper.findAll('.city-forum-topic-title').map(item => item.text())).toEqual(['旧话题', '新话题'])
    await wrapper.findAll('.city-forum-modes button')[2]?.trigger('click')
    expect(wrapper.findAll('.city-forum-topic-title').map(item => item.text())).toEqual(['新话题'])
    expect(posts.map(item => item.id)).toEqual([1, 2])
    wrapper.unmount()
  })

  it('searches author names and preserves search and keyboard focus after returning from detail', async () => {
    const wrapper = render()
    await wrapper.get('input').setValue('自选昵称')
    await wrapper.get('.city-forum-topic-title').trigger('click')
    expect(wrapper.emitted('open')?.[0]?.[0]).toMatchObject({ id: 1 })
    expect(wrapper.get('.detail-fixture').text()).toBe('完整正文')
    expect(wrapper.find('.city-forum-topics').exists()).toBe(false)
    expect(document.activeElement?.className).toBe('city-forum-detail-title')
    await wrapper.get('.city-forum-back').trigger('click')
    await nextTick()
    expect(wrapper.get('input').element.value).toBe('自选昵称')
    expect(wrapper.findAll('.city-forum-row')).toHaveLength(1)
    expect(document.activeElement?.getAttribute('data-topic')).toBe('1')
    wrapper.unmount()
  })

  it('keeps loading, error, empty and no-match states distinct', async () => {
    const wrapper = render({ loading: true })
    expect(wrapper.text()).toContain('正在加载帖子')
    await wrapper.setProps({ loading: false, error: true })
    expect(wrapper.get('[role="alert"]').text()).toContain('加载成功')
    await wrapper.setProps({ error: false, posts: [] })
    expect(wrapper.text()).toContain('还没有帖子')
    await wrapper.setProps({ posts })
    await wrapper.get('input').setValue('不存在的关键词')
    expect(wrapper.text()).toContain('没有找到匹配的帖子')
    await wrapper.get('.city-forum-state button').trigger('click')
    expect(wrapper.findAll('.city-forum-row')).toHaveLength(2)
    wrapper.unmount()
  })

  it('emits create, refresh and profile actions without generating content', async () => {
    const wrapper = render()
    await wrapper.get('.city-forum-primary').trigger('click')
    await wrapper.get('[aria-label="刷新帖子"]').trigger('click')
    await wrapper.findAll('.city-forum-meta button')[0]?.trigger('click')
    expect(wrapper.emitted('create')).toHaveLength(1)
    expect(wrapper.emitted('retry')).toHaveLength(1)
    expect(wrapper.emitted('profile')?.[0]).toEqual([12])
    wrapper.unmount()
  })

  it('updates an open detail from current props instead of retaining stale post data', async () => {
    const wrapper = render()
    await wrapper.get('[data-topic="1"]').trigger('click')
    await wrapper.setProps({ posts: posts.map(post => ({ ...post, body: '服务返回的新内容' })) })
    expect(wrapper.get('.detail-fixture').text()).toBe('服务返回的新内容')
    wrapper.unmount()
  })

  it('uses compact title rows without excerpts outside the plaza', () => {
    const wrapper = render({ excerpts: false, title: '技术问答', district: '技术工坊' })
    expect(wrapper.find('.city-forum-excerpt').exists()).toBe(false)
    expect(wrapper.get('.city-forum-breadcrumb').text()).toContain('技术工坊')
    wrapper.unmount()
  })

  it('preserves plaza-only channel navigation through the compact selector', async () => {
    const wrapper = render({ channels: [{ key: 'chat-hall', label: '闲聊广场' }, { key: 'life-break', label: '生活摸鱼' }], activeChannel: 'chat-hall' })
    await wrapper.get('select').setValue('life-break')
    expect(wrapper.emitted('channel')?.[0]).toEqual(['life-break'])
    wrapper.unmount()
  })
})
