<template>
  <div class="bd-public-surface auth-page">
    <PublicSiteHeader compact />
    <main class="auth-main">
      <img class="auth-mascot" src="/brand/editorial/key-keeper.png" alt="零号城守钥人" width="100" height="100" />
      <div class="auth-content"><slot /></div>
      <div v-if="$slots.footer" class="auth-links"><slot name="footer" /></div>
      <figure class="auth-signature">
        <blockquote>{{ signature.line }}</blockquote>
        <figcaption>{{ signature.name }} · 零号城</figcaption>
      </figure>
    </main>
    <footer class="auth-footer">&copy; {{ currentYear }} {{ siteName }}</footer>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { ZERO_CITY_CARD_MANIFEST } from '@/constants/zeroCityCardManifest'
import PublicSiteHeader from './PublicSiteHeader.vue'
const appStore = useAppStore()
const siteName = computed(() => appStore.siteName || 'BizDecipher')
const currentYear = new Date().getFullYear()
const signature = ZERO_CITY_CARD_MANIFEST.hope_night_shift
onMounted(() => { void appStore.fetchPublicSettings() })
</script>
<style scoped>
.auth-page { display: flex; flex-direction: column; min-height: 100svh; }
.auth-main { width: min(390px, calc(100% - 40px)); margin: auto; padding: 32px 0; }
.auth-mascot { display: block; width: 100px; height: 100px; object-fit: contain; margin: 0 auto 16px; }
.auth-content { min-width: 0; }
.auth-links { display: flex; align-items: center; justify-content: center; flex-wrap: wrap; gap: 8px; margin-top: 20px; font-size: 13px; }
.auth-signature { text-align: center; border-top: 1px solid var(--public-line); padding-top: 20px; margin: 28px 0 0; color: var(--public-muted); }
.auth-signature blockquote { margin: 0; font-size: 12px; }
.auth-signature figcaption { margin-top: 5px; font-size: 11px; }
.auth-footer { padding: 16px 20px; text-align: center; color: var(--public-muted); font-size: 11px; }
.auth-content :deep(.input) { background: var(--public-field); color: var(--public-text); border: 1px solid var(--public-line); border-radius: var(--public-radius); box-shadow: none; }
.auth-content :deep(.input:focus) { border-color: var(--public-action); }
.auth-content :deep(.btn-primary) { background: var(--public-action); color: #fff; border: 1px solid transparent; border-radius: var(--public-radius); box-shadow: none; }
.auth-content :deep(.btn-primary:hover:not(:disabled)) { background: var(--public-action-hover); }
.auth-links :deep(a) { color: var(--public-action); }
@media (max-width: 600px) {
  .auth-main { padding: 24px 0; }
  .auth-mascot { width: 84px; height: 84px; }
}
</style>
