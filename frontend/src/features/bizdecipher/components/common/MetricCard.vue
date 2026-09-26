<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  label: string
  value: string | number
  hint?: string
  tone?: 'primary' | 'slate' | 'green' | 'red'
}>(), {
  tone: 'slate'
})

const valueColor = computed(() => {
  switch (props.tone) {
    case 'primary':
      return '#0f766e'
    case 'green':
      return '#16a34a'
    case 'red':
      return '#dc2626'
    default:
      return '#0f172a'
  }
})
</script>

<template>
  <div class="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
    <div class="text-2xl font-semibold tracking-tight" :style="{ color: valueColor }">{{ value }}</div>
    <div class="mt-1 text-sm text-slate-500">{{ label }}</div>
    <div v-if="hint" class="mt-1 text-xs text-slate-400">{{ hint }}</div>
  </div>
</template>
