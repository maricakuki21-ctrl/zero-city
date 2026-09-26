<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  status?: string
  /** size in px */
  size?: number
}>(), {
  status: 'active',
  size: 8
})

const color = computed(() => {
  switch (props.status?.toLowerCase()) {
    case 'active':
    case 'healthy':
    case 'approved':
    case 'posted':
    case 'ok':
      return '#16a34a'
    case 'pending':
    case 'submitted':
    case 'testing':
    case 'reviewing':
      return '#d97706'
    case 'degraded':
    case 'paused':
      return '#0891b2'
    case 'rejected':
    case 'reversed':
    case 'down':
    case 'retired':
      return '#dc2626'
    default:
      return '#64748b'
  }
})
</script>

<template>
  <span
    class="inline-block rounded-full"
    :style="{ width: `${size}px`, height: `${size}px`, background: color }"
  ></span>
</template>
