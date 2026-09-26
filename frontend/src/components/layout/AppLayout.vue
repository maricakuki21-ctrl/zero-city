<template>
  <div class="bd-app-shell min-h-screen" :data-visual-domain="visualDomain">
    <a class="bd-skip-link" href="#main-content">跳转到主要内容</a>
    <AppSidebar />

    <div
      class="bd-app-frame"
      :class="sidebarCollapsed ? 'lg:ml-[72px]' : 'lg:ml-[218px]'"
    >
      <AppHeader />
      <main id="main-content" class="bd-app-main">
        <slot />
      </main>
    </div>
    <GlobalChatHost />
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import '@/styles/product-surface.css'
import { computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'
import GlobalChatHost from '@/features/bizdecipher/chat/GlobalChatHost.vue'

const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')

const visualDomain = computed(() => {
  const path = route.path
  const query = route.query
  if (path.startsWith('/account-square') || path.startsWith('/marketplace')) return 'market'
  if (path.startsWith('/tavern')) return 'tavern'
  if (path === '/assets' || path.startsWith('/capability-assets') || path.startsWith('/operator')) return 'asset'
  if (path === '/profile' || query.workspace === 'mine') return 'profile'
  if (path === '/community') {
    if (query.district === 'tavern') return 'tavern'
    if (query.district === 'governance') return 'governance'
    if (query.district === 'market') return 'asset'
    return 'city'
  }
  if (path.startsWith('/admin')) return 'governance'
  return 'gateway'
})

const { replayTour } = useOnboardingTour({
  storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: false,
})
const onboardingStore = useOnboardingStore()

onMounted(() => {
  onboardingStore.setReplayCallback(replayTour)
})

defineExpose({ replayTour })
</script>

<style scoped>
.bd-app-shell {
  position: relative;
  min-height: 100dvh;
  overflow-x: clip;
  color: var(--bd-text-primary);
  background: var(--bd-canvas);
}

.bd-app-backdrop,
.bd-backdrop-grid,
.bd-backdrop-glow {
  position: fixed;
  inset: 0;
  pointer-events: none;
}

.bd-app-backdrop {
  z-index: 0;
  overflow: hidden;
  background:
    radial-gradient(circle at 12% 0%, color-mix(in srgb, var(--bd-accent-teal) 11%, transparent), transparent 30rem),
    linear-gradient(180deg, color-mix(in srgb, var(--bd-canvas) 96%, #000 4%), var(--bd-canvas));
}

.bd-backdrop-grid {
  opacity: 0.16;
  background-image:
    linear-gradient(to right, color-mix(in srgb, var(--bd-text-secondary) 11%, transparent) 1px, transparent 1px),
    linear-gradient(to bottom, color-mix(in srgb, var(--bd-text-secondary) 11%, transparent) 1px, transparent 1px);
  background-size: 48px 48px;
  mask-image: linear-gradient(to bottom, #000, transparent 72%);
}

.bd-backdrop-glow {
  inset: auto -16rem -24rem auto;
  width: 42rem;
  height: 42rem;
  border-radius: 50%;
  background: color-mix(in srgb, var(--bd-accent-blue) 7%, transparent);
  filter: blur(80px);
}

.bd-app-frame {
  position: relative;
  z-index: 1;
  min-height: 100dvh;
  transition: margin-left var(--bd-motion-standard) ease-in-out;
}

.bd-app-main {
  width: 100%;
  max-width: 1600px;
  min-height: calc(100dvh - 4.5rem);
  margin: 0 auto;
  padding: var(--bd-space-6) clamp(var(--bd-space-4), 3vw, var(--bd-space-10)) var(--bd-space-12);
}

.bd-skip-link {
  position: fixed;
  z-index: 200;
  top: var(--bd-space-3);
  left: var(--bd-space-3);
  padding: var(--bd-space-2) var(--bd-space-3);
  border: 1px solid var(--bd-accent-teal);
  border-radius: var(--bd-radius-sm);
  background: var(--bd-surface-raised);
  color: var(--bd-text-primary);
  transform: translateY(-180%);
  transition: transform var(--bd-motion-micro) ease-out;
}

.bd-skip-link:focus {
  transform: translateY(0);
}

@media (max-width: 767px) {
  .bd-app-main {
    padding: var(--bd-space-4) var(--bd-space-3) var(--bd-space-10);
  }
}
</style>
