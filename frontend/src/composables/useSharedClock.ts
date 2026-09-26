import { onMounted, onUnmounted, readonly, ref } from 'vue'

const nowMs = ref(Date.now())
let consumers = 0
let timer: ReturnType<typeof setInterval> | null = null

export function useSharedClock() {
  onMounted(() => {
    consumers += 1
    nowMs.value = Date.now()
    if (!timer) {
      timer = setInterval(() => {
        nowMs.value = Date.now()
      }, 1000)
    }
  })

  onUnmounted(() => {
    consumers = Math.max(0, consumers - 1)
    if (consumers === 0 && timer) {
      clearInterval(timer)
      timer = null
    }
  })

  return readonly(nowMs)
}
