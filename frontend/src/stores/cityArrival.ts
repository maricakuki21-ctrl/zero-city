import { defineStore } from 'pinia'
import { ref } from 'vue'

const completedKey = (id: number) => `zero-city:arrival:v1:${id}`
const visitKey = (id: number) => `zero-city:visit:v1:${id}`

function read(kind: 'localStorage' | 'sessionStorage', key: string): boolean {
  try { return window[kind].getItem(key) === '1' } catch { return false }
}
function write(kind: 'localStorage' | 'sessionStorage', key: string): void {
  try { window[kind].setItem(key, '1') } catch { /* In-memory state still prevents repeated prompts. */ }
}

export const useCityArrivalStore = defineStore('cityArrival', () => {
  const showStory = ref(false)
  const showGreeting = ref(false)
  const userId = ref<number | null>(null)
  const seen = new Set<number>()
  const completed = new Set<number>()

  function visit(id: number) {
    if (userId.value !== id) {
      showStory.value = false
      showGreeting.value = false
      userId.value = id
    }
    if (seen.has(id) || read('sessionStorage', visitKey(id))) return
    seen.add(id)
    if (completed.has(id) || read('localStorage', completedKey(id))) {
      showGreeting.value = true
      write('sessionStorage', visitKey(id))
    } else {
      showStory.value = true
    }
  }

  function finish() {
    const id = userId.value
    if (id !== null) {
      completed.add(id)
      seen.add(id)
      write('localStorage', completedKey(id))
      write('sessionStorage', visitKey(id))
    }
    showStory.value = false
    showGreeting.value = false
  }

  function replay(id: number) {
    userId.value = id
    seen.add(id)
    showGreeting.value = false
    showStory.value = true
  }

  function resetSession() {
    if (userId.value !== null) {
      seen.delete(userId.value)
      try { window.sessionStorage.removeItem(visitKey(userId.value)) } catch { /* Storage can be disabled. */ }
    }
    userId.value = null
    showStory.value = false
    showGreeting.value = false
  }

  return { showStory, showGreeting, userId, visit, finish, replay, resetSession }
})
