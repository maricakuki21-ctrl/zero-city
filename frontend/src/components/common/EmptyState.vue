<template>
  <div class="empty-state">
    <!-- Zero City mascot / icon -->
    <div class="zero-empty-shell mb-5 flex h-24 w-24 items-center justify-center rounded-[1.7rem]">
      <slot name="icon">
        <component v-if="icon" :is="icon" class="empty-state-icon h-10 w-10" aria-hidden="true" />
        <img
          v-else
          :src="zeroCityMascots.emptyStateSleeper"
          alt=""
          class="zero-empty-mascot"
          aria-hidden="true"
        />
      </slot>
    </div>

    <h3 class="empty-state-title">
      {{ displayTitle }}
    </h3>

    <p class="empty-state-description">
      {{ displayDescription }}
    </p>

    <div v-if="actionText || $slots.action" class="mt-6">
      <slot name="action">
        <component
          :is="actionTo ? 'RouterLink' : 'button'"
          v-if="actionText"
          :to="actionTo"
          @click="!actionTo && $emit('action')"
          class="btn btn-primary"
        >
          <Icon v-if="actionIcon" name="plus" size="md" class="mr-2" />
          {{ actionText }}
        </component>
      </slot>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Component } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { zeroCityMascots } from '@/constants/zeroCityMascots'

const { t } = useI18n()

interface Props {
  icon?: Component | string
  title?: string
  description?: string
  actionText?: string
  actionTo?: string | object
  actionIcon?: boolean
  message?: string
}

const props = withDefaults(defineProps<Props>(), {
  description: '',
  actionIcon: true
})

const displayTitle = computed(() => props.title || t('common.noData'))
const displayDescription = computed(() => props.description || props.message || '')

defineEmits(['action'])
</script>

<style scoped>
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 2rem 1rem;
  text-align: center;
}

.zero-empty-shell {
  position: relative;
  overflow: hidden;
  background: var(--zc-bg);
  box-shadow: 14px 14px 30px var(--zc-shadow-dark), -14px -14px 30px var(--zc-shadow-light);
}

.zero-empty-shell::before {
  position: absolute;
  inset: 0.7rem;
  border-radius: 1.25rem;
  box-shadow: inset 5px 5px 12px var(--zc-shadow-dark), inset -5px -5px 12px var(--zc-shadow-light);
  content: '';
}

.zero-empty-mascot {
  position: relative;
  z-index: 1;
  width: 5.8rem;
  height: 5.8rem;
  object-fit: contain;
  filter: drop-shadow(0 14px 16px color-mix(in srgb, var(--zc-shadow-dark) 62%, transparent));
}

.empty-state-icon {
  color: var(--zc-accent);
}

.empty-state-title {
  color: var(--zc-text-strong);
  font-size: 1rem;
  font-weight: 950;
  letter-spacing: -0.025em;
}

.empty-state-description {
  margin-top: 0.5rem;
  max-width: 26rem;
  color: var(--zc-muted);
  font-size: 0.875rem;
  font-weight: 650;
  line-height: 1.7;
}
</style>
