<template>
  <header class="public-header">
    <RouterLink to="/home" class="public-brand" :aria-label="`${siteName} 官网`">
      <img :src="siteLogo" alt="" width="32" height="36" /><span>{{ siteName }}</span>
    </RouterLink>
    <template v-if="!compact">
      <nav class="public-header-nav" aria-label="官网导航">
        <RouterLink to="/assets">发现作品</RouterLink>
        <RouterLink to="/marketplace">找服务</RouterLink>
        <RouterLink to="/community">零号城</RouterLink>
      </nav>
      <div class="public-header-actions">
        <RouterLink v-if="!authStore.isAuthenticated" to="/login" class="public-text-link">登录</RouterLink>
        <RouterLink to="/operator" class="public-action"><span>进入工作台</span><ArrowUpRight :size="16" aria-hidden="true" /></RouterLink>
      </div>
    </template>
    <RouterLink v-else to="/home" class="public-text-link"><ArrowLeft :size="16" aria-hidden="true" />回到官网</RouterLink>
  </header>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { ArrowLeft, ArrowUpRight } from '@lucide/vue'
import { useAppStore, useAuthStore } from '@/stores'
import { resolveBrandAsset } from '@/utils/brandResolver'
import '@/styles/public-entry.css'
defineProps<{ compact?: boolean }>()
const appStore = useAppStore()
const authStore = useAuthStore()
const siteName = computed(() => appStore.siteName || 'BizDecipher')
const siteLogo = computed(() => resolveBrandAsset(appStore.siteLogo, 'mark').url)
</script>
<style scoped>
.public-header { min-height: 80px; padding: 16px 5%; display: flex; align-items: center; justify-content: space-between; gap: 24px; border-bottom: 1px solid var(--public-line); }
.public-brand { min-width: 0; display: inline-flex; align-items: center; gap: 10px; color: var(--public-text); font-size: 18px; font-weight: 650; }
.public-brand img { flex-shrink: 0; object-fit: contain; }
.public-brand span { overflow-wrap: anywhere; line-height: 1.3; }
.public-header-nav, .public-header-actions { display: flex; align-items: center; gap: 28px; }
.public-header-nav a { color: var(--public-muted); font-size: 13px; }
.public-header-nav a:hover { color: var(--public-action); }
.public-header-actions { gap: 18px; flex-shrink: 0; }
.public-header-actions .public-action { background: var(--public-text); }
.public-header-actions .public-action:hover { background: var(--public-action); }
@media (max-width: 800px) {
  .public-header { min-height: 64px; padding: 12px 20px; gap: 12px; }
  .public-header-nav { display: none; }
}
@media (max-width: 520px) {
  .public-brand { font-size: 15px; gap: 6px; }
  .public-brand img { width: 24px; height: 28px; }
  .public-header-actions { gap: 10px; }
  .public-header-actions .public-action { min-height: 34px; padding: 6px 10px; font-size: 11px; }
  .public-header-actions .public-action svg { display: none; }
}
</style>
