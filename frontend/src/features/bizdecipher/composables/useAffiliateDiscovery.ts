import { computed, onBeforeUnmount, onMounted, ref, watch, type Ref } from 'vue'

const STORAGE_PREFIX = 'bizdecipher:affiliate-discovery:v1:'
const SEEN_EVENT = 'bizdecipher:affiliate-discovery-seen'

/** A one-time feature introduction, not an unread message or an earned reward. */
export function useAffiliateDiscovery(userId: Ref<number>) {
  const seen = ref(false)
  const validUser = computed(() => Number.isSafeInteger(userId.value) && userId.value > 0)
  const storageKey = () => `${STORAGE_PREFIX}${userId.value}`

  function readSeen(): void {
    seen.value = false
    if (!validUser.value) return
    try {
      seen.value = localStorage.getItem(storageKey()) === 'seen'
    } catch {
      // Privacy-restricted storage must not block navigation or invitations.
    }
  }

  function markSeen(accountId = userId.value): void {
    if (!validUser.value || accountId !== userId.value) return
    seen.value = true
    try {
      localStorage.setItem(storageKey(), 'seen')
    } catch {
      // The in-page event still dismisses the hint for this visit.
    }
    window.dispatchEvent(new CustomEvent<number>(SEEN_EVENT, { detail: accountId }))
  }

  function onSeen(event: Event): void {
    if ((event as CustomEvent<number>).detail === userId.value) seen.value = true
  }

  function onStorage(event: StorageEvent): void {
    if (event.key === storageKey() || event.key === null) readSeen()
  }

  watch(userId, readSeen, { immediate: true, flush: 'sync' })
  onMounted(() => {
    window.addEventListener(SEEN_EVENT, onSeen)
    window.addEventListener('storage', onStorage)
  })
  onBeforeUnmount(() => {
    window.removeEventListener(SEEN_EVENT, onSeen)
    window.removeEventListener('storage', onStorage)
  })

  return { pending: computed(() => validUser.value && !seen.value), markSeen }
}
