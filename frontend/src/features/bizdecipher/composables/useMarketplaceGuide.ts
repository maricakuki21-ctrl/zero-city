import { computed, reactive, watch, type Ref } from 'vue'
import { apiClient } from '@/api/client'

type Intent = 'hire' | 'sell' | 'browse'
interface GuideState { ready: boolean; completed: boolean; loading: boolean; saving: boolean; error: string }
interface GuideResponse { version: number; intent: string; completed_at: string | null }
const states = reactive(new Map<number, GuideState>())

/** Server-owned, account-isolated onboarding. Never a fake message unread count. */
export function useMarketplaceGuide(userId: Ref<number>) {
  function stateFor(id: number): GuideState {
    if (!states.has(id)) states.set(id, { ready: false, completed: false, loading: false, saving: false, error: '' })
    return states.get(id)!
  }
  const state = computed(() => stateFor(userId.value))
  async function load(force = false) {
    const id = userId.value
    if (id <= 0) return
    const entry = stateFor(id)
    if (entry.loading || entry.saving || (entry.ready && !force)) return
    entry.loading = true
    entry.error = ''
    try {
      const { data } = await apiClient.get<GuideResponse>('/biz/market/onboarding')
      entry.completed = Boolean(data.completed_at)
      entry.ready = true
    } catch {
      entry.error = '新手指南状态暂未同步，可以继续浏览或重试。'
    } finally { entry.loading = false }
  }
  async function complete(intent: Intent): Promise<boolean> {
    const id = userId.value
    if (id <= 0) return false
    const entry = stateFor(id)
    if (entry.loading || entry.saving) return false
    entry.saving = true
    entry.error = ''
    try {
      const { data } = await apiClient.put<GuideResponse>('/biz/market/onboarding', { intent })
      entry.completed = Boolean(data.completed_at)
      entry.ready = true
      return id === userId.value && entry.completed
    } catch {
      entry.error = '引导偏好未保存，请重试；不会发布内容或扣款。'
      return false
    } finally { entry.saving = false }
  }
  watch(userId, () => { void load() }, { immediate: true })
  return { state, pending: computed(() => userId.value > 0 && state.value.ready && !state.value.completed), load, complete }
}
