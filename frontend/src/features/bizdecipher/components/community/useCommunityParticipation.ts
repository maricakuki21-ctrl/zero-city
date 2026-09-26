import { computed, onScopeDispose, ref, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { getCommunityParticipation, type CommunityParticipation } from '@/features/bizdecipher/api/community'
import { extractActionableApiErrorMessage } from '@/utils/apiError'

export function useCommunityParticipation() {
  const auth = useAuthStore()
  const access = ref<CommunityParticipation | null>(null)
  const loading = ref(false)
  const error = ref('')
  let request = 0
  async function refresh() {
    const current = ++request
    access.value = null
    error.value = ''
    if (!auth.user?.id) { loading.value = false; return }
    loading.value = true
    try {
      const result = await getCommunityParticipation()
      if (current === request) access.value = result
    } catch (cause) {
      if (current === request) error.value = extractActionableApiErrorMessage(cause, '参与资格加载失败')
    } finally {
      if (current === request) loading.value = false
    }
  }
  watch(() => auth.user?.id, () => void refresh(), { immediate: true })
  onScopeDispose(() => { request++ })
  const label = computed(() => access.value?.is_admin ? '管理员' : access.value?.level ? 'L1 居民' : 'L0 新来者')
  function canPost(district: string, channel: string) {
    if (!access.value) return false
    if (access.value.is_admin) return true
    if (district === 'governance' && channel === 'rules') return false
    return !district || district === 'tavern' || access.value.level >= 1
  }
  return { access, loading, error, label, refresh, canPost }
}
