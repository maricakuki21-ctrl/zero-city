import { apiClient } from '@/api/client'

export interface ChatChannel {
  id: number
  slug: string
  title: string
  description: string
  kind: 'public' | 'private'
  sort_order: number
  created_at: string
}

export interface ChatMessage {
  token_packet_id?: number
  id: number
  channel_id: number
  channel_slug: string
  sender_user_id: number
  sender_name: string
  sender_avatar_url: string
  client_message_id: string
  body: string
  created_at: string
}

export interface ChatMessagePage {
  items: ChatMessage[]
  next_cursor: number
  has_more: boolean
}

export const chatAPI = {
  listChannels(): Promise<{ items: ChatChannel[] }> {
    return apiClient
      .get<{ items: ChatChannel[] }>('/biz/chat/channels')
      .then((response) => response.data)
  },

  listMessages(slug: string, after = 0, limit = 30): Promise<ChatMessagePage> {
    return apiClient
      .get<ChatMessagePage>(`/biz/chat/channels/${encodeURIComponent(slug)}/messages`, { params: { after, limit } })
      .then((response) => response.data)
  },

  sendMessage(slug: string, payload: { client_message_id: string; body: string }): Promise<ChatMessage> {
    return apiClient
      .post<ChatMessage>(`/biz/chat/channels/${encodeURIComponent(slug)}/messages`, payload)
      .then((response) => response.data)
  },
}
