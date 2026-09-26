import { isDraftRecord, localDraftSerializer } from '@/utils/localDraftStorage'
import { sanitizeUrl } from '@/utils/url'

export interface HarnessDraft {
  id: string
  title: string
  intent: string
  answer: string
  submitted: string
  skillIds: string[]
  languageKeyId: number
  languageModel: string
  languageResourceLabel?: string
  imageKeyId: number
  imageModel: string
  imageResourceLabel?: string
  images?: string[]
  resultState?: 'verified'
  completedAt?: string
  updatedAt?: string
  savedAssetId?: number
}

function isDrafts(value: unknown): value is HarnessDraft[] {
  return Array.isArray(value) && value.every(item => isDraftRecord(item)
    && ['id', 'title', 'intent', 'answer', 'submitted', 'languageModel', 'imageModel'].every(key => typeof item[key] === 'string')
    && Number.isSafeInteger(item.languageKeyId) && Number.isSafeInteger(item.imageKeyId)
    && Array.isArray(item.skillIds) && item.skillIds.every(id => typeof id === 'string')
    && (item.resultState === undefined || item.resultState === 'verified')
    && (item.savedAssetId === undefined || Number.isSafeInteger(item.savedAssetId))
    && (item.updatedAt === undefined || typeof item.updatedAt === 'string')
    && (item.completedAt === undefined || typeof item.completedAt === 'string')
    && (item.images === undefined || (Array.isArray(item.images) && item.images.every(url => typeof url === 'string' && Boolean(sanitizeUrl(url))))))
}

export const harnessDraftSerializer = localDraftSerializer(isDrafts)
export const harnessDraftStorageKey = (userId: number | null) => `harness-drafts-v1-${userId ?? 'local'}`
export const hasConfirmedDraftResult = (draft: HarnessDraft) => draft.resultState === 'verified' && Boolean(draft.answer)

export function readHarnessDrafts(userId: number | null): HarnessDraft[] {
  const raw = localStorage.getItem(harnessDraftStorageKey(userId))
  return raw ? harnessDraftSerializer.read(raw) : []
}
