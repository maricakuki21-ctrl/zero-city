<template>
  <div class="biz-public min-h-screen bg-[#f7f1e8] text-[#11110f]">
    <header class="sticky top-0 z-40 border-b border-[#11110f]/[0.08] bg-[#fffaf0]/85 shadow-[0_8px_30px_rgba(33,27,16,0.05)] backdrop-blur-xl">
      <nav class="mx-auto flex max-w-7xl items-center justify-between gap-6 px-6 py-3">

        <router-link to="/login" class="flex items-center gap-2.5" aria-label="BizDecipher">
          <img :src="siteLogo" alt="BizDecipher" class="h-8 w-8 rounded-xl object-contain shadow-sm" />
          <img :src="wordmarkLogo" alt="BizDecipher" class="h-5 w-28 object-contain object-left" />
        </router-link>


        <div class="hidden items-center gap-1 md:flex">
          <router-link
            v-for="item in navItems"
            :key="item.to"
            :to="item.to"
            class="rounded-full px-3.5 py-2 text-sm font-medium transition-colors"
            :class="navLinkClass(item.to)"
          >
            {{ t(item.label) }}
          </router-link>
        </div>

        <div class="flex items-center gap-3">
          <LocaleSwitcher />
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="rounded-full bg-[#11110f] px-4 py-2 text-sm font-black text-[#fffaf0] shadow-[inset_0_1px_0_rgba(255,255,255,0.14),0_10px_24px_rgba(33,27,16,0.18)] transition hover:bg-black"

          >
            {{ isAuthenticated ? t('biz.layout.dashboard') : t('biz.layout.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main>
      <slot />
    </main>

    <footer class="border-t border-[#11110f]/[0.08] bg-[#fffaf0] px-6 py-8 text-sm text-[#69645b]">
      <div class="mx-auto flex max-w-7xl flex-col justify-between gap-4 md:flex-row md:items-center">
        <span>{{ t('biz.layout.footer', { year: currentYear }) }}</span>
        <div class="flex items-center gap-5 text-xs">
          <router-link to="/status" class="hover:text-[#11110f]">{{ t('biz.pageTitles.status') }}</router-link>

          <router-link to="/contribute" class="hover:text-[#11110f]">{{ t('biz.pageTitles.contribute') }}</router-link>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores'
import { resolveBrandAsset } from '@/utils/brandResolver'

const authStore = useAuthStore()
const appStore = useAppStore()
const route = useRoute()
const { t } = useI18n()

const siteLogo = computed(() => resolveBrandAsset(appStore.siteLogo, 'mark').url)
const wordmarkLogo = computed(() => resolveBrandAsset(appStore.siteLogo, 'wordmark').url)
const isAuthenticated = computed(() => authStore.isAuthenticated)
const dashboardPath = computed(() => (authStore.isAdmin ? '/admin/dashboard' : '/dashboard'))
const currentYear = computed(() => new Date().getFullYear())
const navItems = computed(() => [
  { to: '/home', label: 'biz.layout.nav.home' },
  { to: '/home#story', label: 'biz.layout.nav.vision' },
  { to: '/home#market', label: 'biz.layout.nav.market' },
  { to: '/home#gateway', label: 'biz.layout.nav.gateway' }
])



function navLinkClass(to: string): string {
  const active = route.path === to || (to !== '/' && route.path.startsWith(to))
  return active
    ? 'bg-[#11110f] text-[#fffaf0] ring-1 ring-[#11110f]/10'
    : 'text-[#69645b] hover:bg-[#11110f]/[0.06] hover:text-[#11110f]'
}</script>
