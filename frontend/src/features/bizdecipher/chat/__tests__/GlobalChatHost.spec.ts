import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import type { ChatMessage } from '@/features/bizdecipher/api/chat'

const mocks = vi.hoisted(() => ({
  listChannels: vi.fn(),
  listMessages: vi.fn(),
  sendMessage: vi.fn(),
}))

vi.mock('@/features/bizdecipher/api/chat', () => ({ chatAPI: mocks }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 7 } }) }))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: {} }) }))

import GlobalChatHost from '../GlobalChatHost.vue'
import { globalChat } from '../globalChat'

let wrapper: VueWrapper | undefined
let messages: Record<string, ChatMessage[]>

function message(id: number, slug = 'lobby', sender = 2): ChatMessage {
  return {
    id, channel_id: slug === 'lobby' ? 1 : 2, channel_slug: slug,
    sender_user_id: sender, sender_name: '邻居', sender_avatar_url: '',
    client_message_id: `message-${id}`, body: '新的聊天消息',
    created_at: '2026-09-25T00:00:00Z',
  }
}

async function render() {
  wrapper = mount(GlobalChatHost, { global: { stubs: { teleport: true } } })
  await flushPromises()
  return wrapper
}

async function poll() {
  await vi.advanceTimersByTimeAsync(10000)
  await flushPromises()
}

describe('zero city chat unread notification', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
    sessionStorage.clear()
    globalChat.reset()
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
    messages = { lobby: [], tavern: [] }
    mocks.listChannels.mockResolvedValue({
      items: [
        { id: 1, slug: 'lobby', title: '零号城大厅', description: '', kind: 'public', sort_order: 10, created_at: '' },
        { id: 2, slug: 'tavern', title: '酒馆', description: '', kind: 'public', sort_order: 20, created_at: '' },
      ],
    })
    mocks.listMessages.mockImplementation(async (slug: string, after: number) => ({
      items: messages[slug].filter(item => item.id > after),
      next_cursor: messages[slug].at(-1)?.id || 0,
      has_more: false,
    }))
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
    globalChat.reset()
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('does not invent an unread dot for existing history', async () => {
    messages.lobby = [message(1)]
    const host = await render()
    expect(host.find('.global-chat__unread-dot').exists()).toBe(false)
    expect(host.get('.global-chat__launcher').attributes('aria-label')).toBe('打开零号城聊天')
  })

  it('shows a red dot and accessible unread count for a new message during polling', async () => {
    const host = await render()
    messages.lobby.push(message(1))
    await poll()
    expect(host.find('.global-chat__unread-dot').exists()).toBe(true)
    expect(host.get('.global-chat__launcher').attributes('aria-label')).toBe('打开零号城聊天，1 条未读消息')
    expect(host.get('.global-chat__launcher').attributes('title')).toBe('1 条未读消息')
  })

  it('does not notify for the current user’s own messages', async () => {
    const host = await render()
    messages.lobby.push(message(1, 'lobby', 7))
    await poll()
    expect(host.find('.global-chat__unread-dot').exists()).toBe(false)
  })

  it('clears the dot after reading the current channel, including after a refresh', async () => {
    const host = await render()
    messages.lobby.push(message(1))
    await poll()
    await host.get('.global-chat__launcher').trigger('click')
    await flushPromises()
    await host.get('[aria-label="关闭聊天"]').trigger('click')
    expect(host.find('.global-chat__unread-dot').exists()).toBe(false)
    host.unmount()
    wrapper = undefined
    globalChat.reset()
    const refreshed = await render()
    expect(refreshed.find('.global-chat__unread-dot').exists()).toBe(false)
  })

  it('keeps other channels unread until they are opened', async () => {
    const host = await render()
    messages.tavern.push(message(1, 'tavern'))
    await poll()
    await host.get('.global-chat__launcher').trigger('click')
    await flushPromises()
    expect(host.get('.global-chat-panel__channels b').attributes('aria-label')).toBe('1 条未读消息')
    await host.get('[aria-label="关闭聊天"]').trigger('click')
    expect(host.find('.global-chat__unread-dot').exists()).toBe(true)
    await host.get('.global-chat__launcher').trigger('click')
    await flushPromises()
    const tavern = host.findAll('.global-chat-panel__channels button').find(button => button.text().includes('酒馆'))!
    await tavern.trigger('click')
    await flushPromises()
    await host.get('[aria-label="关闭聊天"]').trigger('click')
    expect(host.find('.global-chat__unread-dot').exists()).toBe(false)
  })

  it('does not discard unread notifications on a failed poll', async () => {
    const host = await render()
    messages.lobby.push(message(1))
    await poll()
    mocks.listMessages.mockRejectedValue(new Error('offline'))
    await poll()
    expect(host.find('.global-chat__unread-dot').exists()).toBe(true)
    expect(host.get('.global-chat__launcher').attributes('title')).toBe('1 条未读消息')
  })
})
