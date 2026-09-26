import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ChatMessage, ChatMessagePage } from '@/features/bizdecipher/api/chat'

const mocks = vi.hoisted(() => ({
  listChannels: vi.fn(),
  listMessages: vi.fn(),
  sendMessage: vi.fn(),
}))

vi.mock('@/features/bizdecipher/api/chat', () => ({
  chatAPI: {
    listChannels: mocks.listChannels,
    listMessages: mocks.listMessages,
    sendMessage: mocks.sendMessage,
  },
}))

import {
  globalChat,
  initGlobalChat,
  loadChannels,
  loadHistory,
  refreshUnread,
  resetGlobalChat,
  retryChatMessage,
  sendChatMessage,
} from '@/features/bizdecipher/chat/globalChat'

function message(overrides: Partial<ChatMessage> = {}): ChatMessage {
  return {
    id: 1,
    channel_id: 1,
    channel_slug: 'lobby',
    sender_user_id: 2,
    sender_name: 'Neighbor',
    sender_avatar_url: '',
    client_message_id: `server-${overrides.id || 1}`,
    body: 'hello',
    created_at: '2026-09-12T10:00:00Z',
    ...overrides,
  }
}

function page(items: ChatMessage[], nextCursor = items.at(-1)?.id || 0): ChatMessagePage {
  return { items, next_cursor: nextCursor, has_more: false }
}

describe('globalChat', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    sessionStorage.clear()
    resetGlobalChat()
    mocks.listChannels.mockResolvedValue({
      items: [
        { id: 1, slug: 'lobby', title: '零号城大厅', description: '', kind: 'public', sort_order: 10, created_at: '' },
        { id: 2, slug: 'tavern', title: '酒馆', description: '', kind: 'public', sort_order: 20, created_at: '' },
      ],
    })
  })

  it('loads history once and ignores duplicate deliveries on later polls', async () => {
    initGlobalChat(7)
    await loadChannels()
    mocks.listMessages.mockResolvedValue(page([message({ id: 5, body: 'first' })]))
    await loadHistory('lobby')
    expect(globalChat.state.messages.lobby).toHaveLength(1)

    mocks.listMessages.mockResolvedValue(page([message({ id: 5, body: 'first' })]))
    await loadHistory('lobby', { silent: true })
    expect(globalChat.state.messages.lobby).toHaveLength(1)
    expect(mocks.listMessages).toHaveBeenLastCalledWith('lobby', 5)
  })

  it('keeps a failed message, restores the draft, and does not clear it on error', async () => {
    initGlobalChat(7)
    await loadChannels()
    mocks.listMessages.mockResolvedValue(page([]))
    await loadHistory('lobby')
    mocks.sendMessage.mockRejectedValue(new Error('offline'))

    await sendChatMessage('lobby', '重试这条')

    const [failed] = globalChat.state.messages.lobby
    expect(failed.failed).toBe(true)
    expect(failed.body).toBe('重试这条')
    expect(globalChat.draftFor('lobby')).toBe('重试这条')
    expect(globalChat.state.error).toBe('offline')
  })

  it('keeps drafts and cursors isolated per account', async () => {
    initGlobalChat(7)
    await loadChannels()
    mocks.listMessages.mockResolvedValue(page([message({ id: 9 })]))
    await loadHistory('lobby')
    globalChat.setDraft('lobby', 'account seven draft')

    initGlobalChat(8)
    expect(globalChat.state.messages.lobby).toBeUndefined()
    expect(globalChat.draftFor('lobby')).toBe('')

    initGlobalChat(7)
    expect(globalChat.draftFor('lobby')).toBe('account seven draft')
    expect(globalChat.state.cursors.lobby).toBe(9)
  })

  it('does not count existing history as unread on the first poll of a channel', async () => {
    initGlobalChat(7)
    await loadChannels()
    globalChat.setActiveChannel('lobby')
    mocks.listMessages.mockResolvedValue(page([message({ id: 30, channel_slug: 'tavern' }), message({ id: 31, channel_slug: 'tavern' })]))

    await refreshUnread()
    expect(globalChat.state.unread.tavern || 0).toBe(0)
    expect(globalChat.state.cursors.tavern).toBe(31)

    mocks.listMessages.mockResolvedValue(page([message({ id: 40, channel_slug: 'tavern' })]))
    await refreshUnread()
    expect(globalChat.state.unread.tavern).toBe(1)
    expect(mocks.listMessages).toHaveBeenLastCalledWith('tavern', 31)
  })

  it('retries with the original client_message_id so a lost response cannot duplicate', async () => {
    initGlobalChat(7)
    await loadChannels()
    mocks.listMessages.mockResolvedValue(page([]))
    await loadHistory('lobby')
    mocks.sendMessage.mockRejectedValueOnce(new Error('timeout'))

    await sendChatMessage('lobby', '只发一次')
    const failed = globalChat.state.messages.lobby.find((message) => message.failed)
    expect(failed, 'failed optimistic message').toBeDefined()
    const originalID = failed!.client_message_id

    mocks.sendMessage.mockResolvedValueOnce({
      ...message({ id: 77, body: '只发一次' }),
      client_message_id: originalID,
    })
    await retryChatMessage('lobby', originalID)

    expect(mocks.sendMessage).toHaveBeenCalledTimes(2)
    expect(mocks.sendMessage.mock.calls[0][1].client_message_id).toBe(originalID)
    expect(mocks.sendMessage.mock.calls[1][1].client_message_id).toBe(originalID)
    const remaining = globalChat.state.messages.lobby.filter((message) => message.client_message_id === originalID)
    expect(remaining).toHaveLength(1)
    expect(remaining[0].failed).toBeFalsy()
    expect(remaining[0].id).toBe(77)
  })

  it('counts new messages in the last selected channel while closed, but not own messages', async () => {
    initGlobalChat(7)
    await loadChannels()
    mocks.listMessages.mockResolvedValue(page([]))
    await refreshUnread()
    mocks.listMessages.mockImplementation(async (slug: string) => slug === 'lobby'
      ? page([message({ id: 2 }), message({ id: 3, sender_user_id: 7 })])
      : page([]))
    await refreshUnread()
    expect(globalChat.totalUnread.value).toBe(1)
    await refreshUnread()
    expect(globalChat.totalUnread.value).toBe(1)
    globalChat.setVisible(true)
    expect(globalChat.totalUnread.value).toBe(0)
    expect(globalChat.activeMessages.value).toHaveLength(2)
  })

  it('does not consume unread history by fetching it in the background', async () => {
    initGlobalChat(7)
    await loadChannels()
    mocks.listMessages.mockResolvedValue(page([message({ id: 10 })]))
    await refreshUnread()
    mocks.listMessages.mockResolvedValue(page([message({ id: 11 })]))
    await refreshUnread()
    expect(globalChat.state.cursors.lobby).toBe(10)
    expect(globalChat.state.unread.lobby).toBe(1)
    globalChat.setVisible(true)
    await loadHistory('lobby')
    expect(globalChat.activeMessages.value.map((item) => item.id)).toEqual([10, 11])
    expect(globalChat.state.cursors.lobby).toBe(11)
    expect(globalChat.state.unread.lobby).toBe(0)
  })

  it('reloads visible history after refreshing even with a saved read position', async () => {
    sessionStorage.setItem('bizchat:cursor:7', JSON.stringify({ lobby: 50 }))
    initGlobalChat(7)
    mocks.listMessages.mockResolvedValue(page([message({ id: 49 }), message({ id: 50 })]))
    await loadHistory('lobby')
    expect(mocks.listMessages).toHaveBeenCalledWith('lobby', 0)
    expect(globalChat.activeMessages.value).toHaveLength(2)
    expect(globalChat.totalUnread.value).toBe(0)
  })

  it('does not mark messages read if the panel was closed during the request', async () => {
    initGlobalChat(7)
    mocks.listMessages.mockResolvedValueOnce(page([]))
    await loadHistory('lobby')
    globalChat.setVisible(true)
    let resolve!: (value: ChatMessagePage) => void
    mocks.listMessages.mockReturnValueOnce(new Promise<ChatMessagePage>((done) => { resolve = done }))
    const request = loadHistory('lobby')
    globalChat.setVisible(false)
    resolve(page([message({ id: 3 })]))
    await request
    expect(globalChat.state.unread.lobby).toBe(1)
  })

  it('ignores a history response belonging to the previous account', async () => {
    initGlobalChat(7)
    let resolve!: (value: ChatMessagePage) => void
    mocks.listMessages.mockReturnValueOnce(new Promise<ChatMessagePage>((done) => { resolve = done }))
    const request = loadHistory('lobby')
    initGlobalChat(8)
    resolve(page([message({ id: 3 })]))
    await request
    expect(globalChat.state.messages).toEqual({})
    expect(globalChat.state.cursors).toEqual({})
  })

  it('does not skip other users messages when sending advances the server sequence', async () => {
    initGlobalChat(7)
    mocks.listMessages.mockResolvedValueOnce(page([message({ id: 10 })]))
    await loadHistory('lobby')
    mocks.sendMessage.mockImplementation(async (_slug, payload) =>
      message({ id: 12, sender_user_id: 7, client_message_id: payload.client_message_id }))
    await sendChatMessage('lobby', 'mine')
    mocks.listMessages.mockResolvedValueOnce(page([message({ id: 11 })]))
    await loadHistory('lobby')
    expect(mocks.listMessages).toHaveBeenLastCalledWith('lobby', 10)
    expect(globalChat.activeMessages.value.map((item) => item.id)).toEqual([10, 11, 12])
  })

  it('ignores a send response and draft restoration after switching accounts', async () => {
    initGlobalChat(7)
    let reject!: (error: Error) => void
    mocks.sendMessage.mockReturnValueOnce(new Promise((_resolve, fail) => { reject = fail }))
    const request = sendChatMessage('lobby', 'private draft')
    initGlobalChat(8)
    reject(new Error('offline'))
    await request
    expect(globalChat.state.messages).toEqual({})
    expect(globalChat.draftFor('lobby')).toBe('')
    expect(globalChat.state.error).toBe('')
  })
})
