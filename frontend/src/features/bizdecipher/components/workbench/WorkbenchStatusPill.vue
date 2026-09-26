<template>
  <span class="wb-pill" :class="`is-${tone}`">{{ label }}</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { WorkbenchRunStatus } from '../../workbench/contracts'

const props = defineProps<{
  status: WorkbenchRunStatus
  label: string
}>()

const tone = computed(() => {
  switch (props.status) {
    case 'running':
    case 'queued':
      return 'info'
    case 'succeeded':
      return 'success'
    case 'failed':
      return 'danger'
    case 'cancelled':
      return 'warning'
    default:
      return 'neutral'
  }
})
</script>

<style scoped>
.wb-pill {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  border-radius: 999px;
  padding: 0.35rem 0.75rem;
  font-size: 0.78rem;
  font-weight: 700;
}

.is-neutral {
  background: var(--wb-overlay, color-mix(in srgb, var(--module-panel-raised) 9%, transparent));
  color: var(--wb-ink);
}

.is-info {
  background: color-mix(in srgb, var(--wb-accent) 18%, transparent);
  color: var(--wb-ink-strong);
}

.is-success {
  background: color-mix(in srgb, var(--zc-success) 20%, transparent);
  color: var(--wb-ink-strong);
}

.is-warning {
  background: color-mix(in srgb, var(--zc-warning) 20%, transparent);
  color: var(--wb-ink-strong);
}

.is-danger {
  background: color-mix(in srgb, var(--zc-danger) 20%, transparent);
  color: var(--wb-ink-strong);
}
</style>
