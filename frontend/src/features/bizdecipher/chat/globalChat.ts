import { computed, reactive, readonly } from 'vue'
import { chatAPI, type ChatChannel, type ChatMessage } from '@/features/bizdecipher/api/chat'

export interface ChatViewMessage extends ChatMessage {
  pending?: boolean
  failed?: boolean
}

interface GlobalChatState {
  userId: number
  channels: ChatChannel[]
  activeSlug: string
  messages: Record<string, ChatViewMessage[]>
  cursors: Record<string, number>
  drafts: Record<string, string>
  unread: Record<string, number>
  loaded: boolean
  loading: boolean
  sending: boolean
  error: string
  visible: boolean
}

const CURSOR_PREFIX = 'bizchat:cursor:'
const DRAFT_PREFIX = 'bizchat:draft:'
const CHANNEL_KEY = 'bizchat:channel'

const state = reactive<GlobalChatState>({
  userId: 0,
  channels: [],
  activeSlug: 'lobby',
  messages: {},
  cursors: {},
  drafts: {},
  unread: {},
  loaded: false,
  loading: false,
  sending: false,
  error: '',
  visible: false,
})

let generation = 0
let fetchedCursors: Record<string, number> = {}

function scoped(prefix: string, userId: number): string {
  return `${prefix}${userId}`
}

function readJSON<T>(key: string, fallback: T): T {
  try {
    const raw = sessionStorage.getItem(key)
    return raw ? (JSON.parse(raw) as T) : fallback
  } catch {
    return fallback
  }
}

function writeJSON(key: string, value: unknown): void {
  try {
    sessionStorage.setItem(key, JSON.stringify(value))
  } catch {
    // Draft persistence is best-effort; storage failures must not break sending.
  }
}

export function initGlobalChat(userId: number): void {
  if (!userId || state.userId === userId) return
  resetGlobalChat()
  state.userId = userId
  state.messages = {}
  state.cursors = readJSON<Record<string, number>>(scoped(CURSOR_PREFIX, userId), {})
  state.drafts = readJSON<Record<string, string>>(scoped(DRAFT_PREFIX, userId), {})
  state.unread = {}
  state.loaded = false
  const storedChannel = readJSON<string>(scoped(CHANNEL_KEY, userId), '')
  state.activeSlug = storedChannel || 'lobby'
}

export async function loadChannels(): Promise<void> {
  const currentGeneration = generation
  const response = await chatAPI.listChannels().catch((error) => {
    if (currentGeneration === generation) state.error = error instanceof Error ? error.message : '频道加载失败'
    throw error
  })
  if (currentGeneration !== generation) return
  state.channels = response.items
  const allowed = new Set(response.items.map((channel) => channel.slug))
  for (const slug of Object.keys(state.messages)) {
    if (!allowed.has(slug)) {
      delete state.messages[slug]
      delete state.unread[slug]
      delete fetchedCursors[slug]
    }
  }
  if (!state.channels.some((channel) => channel.slug === state.activeSlug)) {
    state.activeSlug = state.channels[0]?.slug || 'lobby'
  }
  state.loaded = true
}

export async function loadHistory(slug: string, options: { silent?: boolean } = {}): Promise<void> {
  const currentGeneration = generation
  const channelKey = state.userId ? scoped(CURSOR_PREFIX, state.userId) : ''
  // Read positions survive refresh; fetch positions do not, because history is in memory.
  const after = fetchedCursors[slug] || 0
  if (!options.silent) state.loading = true
  try {
    const page = await chatAPI.listMessages(slug, after)
    if (currentGeneration !== generation) return
    const existing = state.messages[slug] || []
    const known = new Set(existing.map((message) => message.id))
    const knownClientIds = new Set(existing.filter((message) => !message.pending && !message.failed)
      .map((message) => message.client_message_id).filter(Boolean))
    const incoming = page.items.filter(
      (message) => !known.has(message.id) && (!message.client_message_id || !knownClientIds.has(message.client_message_id)),
    )
    if (incoming.length) {
      // Replace optimistic bubbles whose server row has now arrived.
      const pendingIds = new Set(incoming.map((message) => message.client_message_id))
      state.messages[slug] = [
        ...existing.filter((message) => !((message.pending || message.failed) && pendingIds.has(message.client_message_id))),
        ...incoming,
      ].sort((a, b) => a.id > 0 && b.id > 0 ? a.id - b.id : 0)
    } else if (!existing.length) {
      state.messages[slug] = []
    }
    fetchedCursors[slug] = Math.max(fetchedCursors[slug] || 0, after, page.next_cursor || 0)
    if (state.cursors[slug] === undefined) {
      // The first snapshot is history, including an empty channel with cursor zero.
      state.cursors[slug] = fetchedCursors[slug]
    }
    state.unread[slug] = (state.messages[slug] || []).filter(
      (message) => message.id > state.cursors[slug] && message.sender_user_id !== state.userId,
    ).length
    if (state.visible && slug === state.activeSlug) markRead(slug)
    if (channelKey) writeJSON(channelKey, state.cursors)
    if (slug === state.activeSlug) state.error = ''
  } catch (error) {
    if (currentGeneration === generation && slug === state.activeSlug) {
      state.error = error instanceof Error ? error.message : '消息同步失败'
    }
    throw error
  } finally {
    if (!options.silent && currentGeneration === generation) state.loading = false
  }
}

export async function refreshActive(): Promise<void> {
  if (!state.userId) return
  await loadHistory(state.activeSlug, { silent: true })
}

export async function refreshUnread(): Promise<void> {
  if (!state.userId || !state.channels.length) return
  await Promise.all(
    state.channels
      .filter((channel) => !state.visible || channel.slug !== state.activeSlug)
      .map(async (channel) => {
        try {
          await loadHistory(channel.slug, { silent: true })
        } catch {
          // A single failing channel must not blank the whole unread badge.
        }
      }),
  )
}

function markRead(slug: string): void {
  state.cursors[slug] = Math.max(state.cursors[slug] || 0, fetchedCursors[slug] || 0)
  state.unread[slug] = 0
  if (state.userId) writeJSON(scoped(CURSOR_PREFIX, state.userId), state.cursors)
}

export function setVisible(visible: boolean): void {
  state.visible = visible
  if (visible && fetchedCursors[state.activeSlug] !== undefined) markRead(state.activeSlug)
}

export function setActiveChannel(slug: string): void {
  if (!slug || slug === state.activeSlug) return
  state.activeSlug = slug
  if (state.visible && fetchedCursors[slug] !== undefined) markRead(slug)
  if (state.userId) writeJSON(scoped(CHANNEL_KEY, state.userId), slug)
}

export function setDraft(slug: string, value: string): void {
  state.drafts[slug] = value
  if (state.userId) writeJSON(scoped(DRAFT_PREFIX, state.userId), state.drafts)
}

export function draftFor(slug: string): string {
  return state.drafts[slug] || ''
}

function newClientMessageID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  return `c-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

async function deliverChatMessage(slug: string, body: string, clientMessageID: string): Promise<boolean> {
  const currentGeneration = generation
  const optimistic: ChatViewMessage = {
    id: -Date.now(),
    channel_id: 0,
    channel_slug: slug,
    sender_user_id: state.userId,
    sender_name: '我',
    sender_avatar_url: '',
    client_message_id: clientMessageID,
    body,
    created_at: new Date().toISOString(),
    pending: true,
  }
  state.messages[slug] = [...(state.messages[slug] || []), optimistic]
  state.sending = true
  try {
    const saved = await chatAPI.sendMessage(slug, { client_message_id: clientMessageID, body })
    if (currentGeneration !== generation) return false
    state.messages[slug] = [
      ...(state.messages[slug] || []).filter((message) => message.client_message_id !== clientMessageID),
      saved,
    ]
    // Sending must not advance the fetch cursor past messages from other users.
    state.error = ''
    return true
  } catch (error) {
    if (currentGeneration !== generation) return false
    state.messages[slug] = (state.messages[slug] || []).map((message) =>
      message.client_message_id === clientMessageID ? { ...message, pending: false, failed: true } : message,
    )
    state.error = error instanceof Error ? error.message : '消息发送失败'
    return false
  } finally {
    if (currentGeneration === generation) state.sending = false
  }
}

export async function sendChatMessage(slug: string, rawBody: string): Promise<void> {
  const currentGeneration = generation
  const body = rawBody.trim()
  if (!body || state.sending) return
  setDraft(slug, '')
  const delivered = await deliverChatMessage(slug, body, newClientMessageID())
  if (currentGeneration === generation && !draftFor(slug)) setDraft(slug, delivered ? '' : body)
}

export async function retryChatMessage(slug: string, clientMessageID: string): Promise<void> {
  const target = (state.messages[slug] || []).find((message) => message.client_message_id === clientMessageID)
  if (!target || state.sending) return
  // Reuse the original client_message_id so a retry after a lost response cannot
  // create a second row for the same logical message.
  state.messages[slug] = (state.messages[slug] || []).filter((message) => message.client_message_id !== clientMessageID)
  await deliverChatMessage(slug, target.body, clientMessageID)
}

export function clearChatError(): void {
  state.error = ''
}

export function resetGlobalChat(): void {
  generation++
  fetchedCursors = {}
  state.userId = 0
  state.channels = []
  state.activeSlug = 'lobby'
  state.messages = {}
  state.cursors = {}
  state.drafts = {}
  state.unread = {}
  state.loaded = false
  state.loading = false
  state.sending = false
  state.error = ''
  state.visible = false
}

export const globalChat = {
  state: readonly(state),
  activeChannel: computed(() => state.channels.find((channel) => channel.slug === state.activeSlug)),
  activeMessages: computed(() => state.messages[state.activeSlug] || []),
  totalUnread: computed(() => Object.values(state.unread).reduce((sum, count) => sum + count, 0)),
  init: initGlobalChat,
  loadChannels,
  loadHistory,
  refreshActive,
  refreshUnread,
  setActiveChannel,
  setVisible,
  setDraft,
  draftFor,
  send: sendChatMessage,
  retry: retryChatMessage,
  clearError: clearChatError,
  reset: resetGlobalChat,
}
