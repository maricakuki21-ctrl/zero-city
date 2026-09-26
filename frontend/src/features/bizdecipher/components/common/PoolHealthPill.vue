<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  health?: string
}>(), {
  health: 'healthy'
})

const tone = computed(() => {
  switch (props.health?.toLowerCase()) {
    case 'healthy':
    case 'ok':
      return { dot: '#16a34a', bg: '#dcfce7', text: '#166534' }
    case 'degraded':
    case 'warning':
      return { dot: '#d97706', bg: '#fef3c7', text: '#92400e' }
    case 'down':
    case 'critical':
      return { dot: '#dc2626', bg: '#fee2e2', text: '#991b1b' }
    default:
      return { dot: '#16a34a', bg: '#dcfce7', text: '#166534' }
  }
})

const label = computed(() => {
  const h = props.health?.toLowerCase()
  if (h === 'healthy' || h === 'ok') return 'Healthy'
  if (h === 'degraded' || h === 'warning') return 'Degraded'
  if (h === 'down' || h === 'critical') return 'Down'
  return props.health || 'Healthy'
})
</script>

<template>
  <span
    class="inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-semibold"
    :style="{ background: tone.bg, color: tone.text }"
  >
    <span class="h-1.5 w-1.5 rounded-full" :style="{ background: tone.dot }"></span>
    {{ label }}
  </span>
</template>
