<template>
  <button class="shared-pool-guide-button" type="button" @click="emit('click')">
    <span class="shared-pool-guide-button-icon" aria-hidden="true">?</span>
    <span>{{ t('sharedPoolGuide.restart') }}</span>
  </button>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const emit = defineEmits<{ click: [] }>()
const { t } = useI18n()
</script>

<style scoped>
.shared-pool-guide-button {
  display: inline-flex;
  min-height: 36px;
  align-items: center;
  justify-content: center;
  gap: 7px;
  padding: 7px 11px;
  border: 1px solid color-mix(in srgb, var(--module-line, var(--zc-line)) 88%, transparent);
  border-radius: 8px;
  background: var(--module-panel, var(--zc-surface));
  color: var(--module-ink, var(--zc-text));
  box-shadow: var(--module-shadow, 0 8px 22px rgba(30, 45, 70, 0.1));
  font-size: 12px;
  font-weight: 800;
  cursor: pointer;
}

.shared-pool-guide-button:hover {
  border-color: color-mix(in srgb, var(--module-accent, var(--zc-accent)) 52%, transparent);
  color: var(--module-accent, var(--zc-accent));
}

.shared-pool-guide-button:focus-visible {
  outline: 2px solid var(--module-accent, var(--zc-accent));
  outline-offset: 2px;
}

.shared-pool-guide-button-icon {
  display: inline-grid;
  width: 18px;
  height: 18px;
  place-items: center;
  border: 1px solid currentColor;
  border-radius: 50%;
  font-size: 11px;
  line-height: 1;
}
</style>
