<template>
  <AppLayout>
    <div class="uc-page flex min-h-[70vh] items-center justify-center">
      <div class="uc-card mx-auto max-w-lg rounded-[2rem] p-10 text-center">
        <!-- Icon -->
        <div class="uc-icon mx-auto mb-6 flex h-20 w-20 items-center justify-center rounded-[1.5rem]">
          <svg class="h-10 w-10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83" />
          </svg>
        </div>

        <!-- Title -->
        <h1 class="uc-title text-2xl font-bold tracking-[-0.02em]">{{ pageTitle }}</h1>

        <!-- Description -->
        <p class="uc-desc mt-4 text-base leading-7">
          {{ pageDesc }}
        </p>

        <!-- Action -->
        <div class="mt-8">
          <router-link to="/assets" class="inline-flex items-center gap-2 rounded-full px-6 py-3 text-sm font-semibold transition hover:-translate-y-0.5">
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="19" y1="12" x2="5" y2="12" />
              <polyline points="12 19 5 12 12 5" />
            </svg>
            {{ t('nav.capabilityAssets') }}
          </router-link>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'

const route = useRoute()
const { t } = useI18n()

const pageLabels: Record<string, { titleKey: string; descKey: string }> = {
  '/operator': {
    titleKey: 'nav.operatorWorkbench',
    descKey: 'underConstruction.workbench'
  },
  '/marketplace': {
    titleKey: 'nav.marketplace',
    descKey: 'underConstruction.marketplace'
  },
  '/incentives': {
    titleKey: 'nav.incentives',
    descKey: 'underConstruction.incentives'
  }
}

const pageTitle = computed(() => {
  const cfg = pageLabels[route.path]
  return cfg ? t(cfg.titleKey) : t('underConstruction.defaultTitle')
})

const pageDesc = computed(() => {
  const cfg = pageLabels[route.path]
  return cfg ? t(cfg.descKey) : t('underConstruction.defaultDesc')
})
</script>

<style scoped>
.uc-page {
  --uc-surface: var(--zc-surface);
  --uc-card-bg: var(--zc-card-bg, var(--zc-surface));
  --uc-text: var(--zc-text);
  --uc-text-muted: var(--zc-text-muted);
  --uc-accent: var(--zc-accent);
  --uc-accent-soft: var(--zc-accent-soft, color-mix(in srgb, var(--zc-accent) 12%, transparent));
}

.uc-card {
  background: var(--uc-card-bg);
  color: var(--uc-text);
  box-shadow: var(--zc-shadow-raised, 0 4px 24px rgba(0, 0, 0, 0.06));
}

.uc-icon {
  background: var(--uc-accent-soft);
  color: var(--uc-accent);
}

.uc-title {
  color: var(--uc-text);
}

.uc-desc {
  color: var(--uc-text-muted);
}

.uc-card a {
  background: var(--uc-accent);
  color: var(--zc-accent-ink, #fff);
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
}
</style>
