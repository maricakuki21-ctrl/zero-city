export interface ForumTopic {
  readonly id?: number
  readonly user_id?: number
  readonly title: string
  readonly card: string
  readonly time: string
  readonly excerpt: string
  readonly body?: string
  readonly tags?: readonly string[]
  readonly created_at?: string
  readonly pinned?: boolean
  readonly replies: number
  readonly views: number
}
