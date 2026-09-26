<template>
  <header class="bd-topbar">
    <div class="bd-topbar-inner">
      <div class="bd-context">
        <button
          type="button"
          class="bd-icon-button lg:hidden"
          :aria-label="t('common.toggleMenu')"
          @click="toggleMobileSidebar"
        >
          <Icon name="menu" size="md" />
        </button>
        <div class="bd-context-copy">
          <strong class="bd-context-title">{{ pageTitle || workspaceLabel }}</strong>
        </div>
      </div>

      <div class="bd-toolbar">
        <button
          type="button"
          class="bd-toolbar-button"
          :aria-label="themeLabel"
          :title="themeLabel"
          @click="toggleZeroCityTheme"
        >
          <Icon :name="zeroCityTheme === 'noir' ? 'moon' : 'sun'" size="sm" />
        </button>

        <AnnouncementBell v-if="user" />
        <DailyFortuneGift v-if="user || fortunePreview" :auto-open="fortunePreview" />

        <router-link v-if="user" to="/wallet" class="bd-balance-summary" title="打开钱包">
          <span class="bd-balance-item">
            <small>余额</small>
            <strong>{{ formatHeaderMoney(availableBalance) }}</strong>
          </span>
          <span class="bd-balance-divider" aria-hidden="true"></span>
          <span class="bd-balance-item">
            <small>积分</small>
            <strong>{{ points }}</strong>
          </span>
        </router-link>
        <router-link v-if="user" :to="rechargeAvailable ? '/purchase' : '/wallet'" class="bd-toolbar-button bd-recharge-button" :title="rechargeAvailable ? '充值' : '前往钱包查看充值通道'">
          <Icon name="plus" size="sm" />
          <span>充值</span>
        </router-link>

        <div v-if="user" ref="dropdownRef" class="bd-account">
          <button
            type="button"
            class="bd-account-trigger"
            :aria-expanded="dropdownOpen"
            :aria-label="t('common.userMenu')"
            @click="toggleDropdown"
          >
            <span class="bd-avatar">
              <img v-if="avatarUrl" :src="avatarUrl" :alt="displayName" />
              <img v-else :src="fallbackAvatar" :alt="displayName" />
            </span>
            <span class="hidden xl:block bd-account-copy">
              <strong>{{ displayName }}</strong>
            </span>
            <Icon name="chevronDown" size="sm" class="hidden md:block" />
          </button>

          <transition name="bd-menu">
            <div v-if="dropdownOpen" class="bd-account-menu">
              <div class="bd-account-menu-head">
                <strong>{{ displayName }}</strong>
                <span>{{ user.email }}</span>
              </div>
              <router-link to="/community?workspace=mine" class="bd-menu-item" @click="closeDropdown">
                <Icon name="user" size="sm" /> {{ t('nav.myCity') }}
              </router-link>
              <router-link to="/settings/account" class="bd-menu-item" @click="closeDropdown">
                <Icon name="cog" size="sm" /> 账户设置
              </router-link>
              <router-link to="/keys" class="bd-menu-item" @click="closeDropdown">
                <Icon name="key" size="sm" /> 我的密钥
              </router-link>
              <router-link to="/usage" class="bd-menu-item" @click="closeDropdown">
                <Icon name="chart" size="sm" /> {{ t('nav.usage') }}
              </router-link>
              <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="bd-menu-item" @click="closeDropdown"><Icon name="book" size="sm" />文档与帮助</a>
              <button v-if="!route.path.startsWith('/admin')" type="button" class="bd-menu-item" @click="replayArrival"><Icon name="book" size="sm" />重看入城故事</button>
              <div class="bd-menu-language"><span>语言</span><LocaleSwitcher /></div>
              <button type="button" class="bd-menu-item bd-menu-danger" @click="handleLogout">
                <Icon name="logOut" size="sm" /> {{ t('nav.logout') }}
              </button>
            </div>
          </transition>
        </div>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore } from '@/stores'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import { usePointerDownOutside } from '@/composables/usePointerDownOutside'
import { zeroCityMascots } from '@/constants/zeroCityMascots'
import { sanitizeUrl } from '@/utils/url'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import AnnouncementBell from '@/components/common/AnnouncementBell.vue'
import DailyFortuneGift from '@/components/layout/DailyFortuneGift.vue'
import Icon from '@/components/icons/Icon.vue'
import { useCityArrivalStore } from '@/stores/cityArrival'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const adminSettingsStore = useAdminSettingsStore()
const cityArrivalStore = useCityArrivalStore()

const user = computed(() => authStore.user)
const rechargeAvailable = computed(() => !(
  appStore.publicSettingsLoaded && appStore.cachedPublicSettings?.payment_enabled === false
))
const fortunePreview = computed(() => import.meta.env.DEV && route.query.fortunePreview === '1')
const dropdownOpen = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)
const docUrl = computed(() => sanitizeUrl(appStore.docUrl))
const avatarUrl = computed(() => user.value?.avatar_url?.trim() || '')
const zeroCityTheme = computed(() => appStore.zeroCityTheme)
const themeLabel = computed(() => zeroCityTheme.value === 'noir' ? '切换到日间模式' : '切换到夜间模式')
const fallbackAvatar = computed(() => zeroCityMascots.gatewayOperator)
const availableBalance = computed(() => Number(user.value?.balance || 0))
const points = computed(() => Number(user.value?.credit_balance || 0).toFixed(2))
const displayName = computed(() => user.value?.username || user.value?.email?.split('@')[0] || 'Builder')

const workspaceLabel = computed(() => {
  const path = route.path
  if (path.startsWith('/account-square')) return '共享市场'
  if (path.startsWith('/marketplace')) return '双边市场'
  if (path.startsWith('/operator')) return '创作工作台'
  if (path.startsWith('/tavern') || route.query.district === 'tavern') return '零号城 · 酒馆'
  if (path.startsWith('/community')) return '零号城'
  if (path === '/assets' || path.startsWith('/capability-assets')) return '能力资产'
  if (path.startsWith('/admin')) return '运营控制台'
  return 'BizDecipher Workspace'
})

const pageTitle = computed(() => {
  if (route.name === 'CustomPage') {
    const id = String(route.params.id ?? '')
    const publicItems = appStore.cachedPublicSettings?.custom_menu_items ?? []
    const menuItem = publicItems.find((item) => item.id === id)
      ?? (authStore.isAdmin ? adminSettingsStore.customMenuItems.find((item) => item.id === id) : undefined)
    if (menuItem?.label) return menuItem.label
  }
  const titleKey = route.meta.titleKey as string | undefined
  return titleKey ? t(titleKey) : String(route.meta.title ?? '')
})

function toggleMobileSidebar(): void {
  appStore.toggleMobileSidebar()
}

function toggleZeroCityTheme(): void {
  appStore.toggleZeroCityTheme()
}

function toggleDropdown(): void {
  dropdownOpen.value = !dropdownOpen.value
}

function closeDropdown(): void {
  dropdownOpen.value = false
}

function replayArrival(): void {
  closeDropdown()
  if (user.value) cityArrivalStore.replay(user.value.id)
}

async function handleLogout(): Promise<void> {
  closeDropdown()
  try {
    await authStore.logout()
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : '退出登录失败')
  } finally {
    await router.push('/login')
  }
}

function formatHeaderMoney(value: number): string {
  return Number.isFinite(value) ? `$${value.toFixed(2)}` : '$0.00'
}

usePointerDownOutside([dropdownRef], closeDropdown, dropdownOpen)
</script>

<style scoped>
.bd-topbar {
  position: sticky;
  top: 0;
  z-index: 30;
  min-height: 58px;
  border-bottom: 1px solid color-mix(in srgb, var(--bd-text-secondary) 16%, transparent);
  background: color-mix(in srgb, var(--bd-canvas) 94%, transparent);
  backdrop-filter: blur(16px);
}

.bd-topbar-inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--bd-space-4);
  max-width: 1600px;
  min-height: 58px;
  margin: 0 auto;
  padding: 8px 28px;
}

.bd-context,
.bd-toolbar,
.bd-account-trigger,
.bd-toolbar-button,
.bd-balance-summary {
  display: flex;
  align-items: center;
}

.bd-context {
  min-width: 0;
  gap: var(--bd-space-3);
}

.bd-context-copy {
  min-width: 0;
}

.bd-context-kicker {
  display: block;
  overflow: hidden;
  color: var(--bd-text-muted);
  font-size: var(--bd-type-overline);
  font-weight: var(--bd-weight-overline);
  letter-spacing: 0.08em;
  text-overflow: ellipsis;
  text-transform: uppercase;
  white-space: nowrap;
}

.bd-context h1 {
  margin: 2px 0 0;
  overflow: hidden;
  color: var(--bd-text-primary);
  font-size: 1.05rem;
  font-weight: 800;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.bd-context p {
  max-width: 44rem;
  margin: 2px 0 0;
  overflow: hidden;
  color: var(--bd-text-muted);
  font-size: var(--bd-type-caption);
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.bd-toolbar {
  flex: 0 0 auto;
  gap: var(--bd-space-2);
}

.bd-toolbar-button,
.bd-icon-button {
  justify-content: center;
  gap: var(--bd-space-2);
  min-height: 2.5rem;
  border: 1px solid transparent;
  border-radius: var(--bd-radius-sm);
  background: transparent;
  color: var(--bd-text-secondary);
  transition: color var(--bd-motion-micro) ease-out, background-color var(--bd-motion-micro) ease-out, border-color var(--bd-motion-micro) ease-out, transform var(--bd-motion-micro) ease-out;
}

.bd-toolbar-button {
  padding: 0 var(--bd-space-3);
  font-size: var(--bd-type-caption);
  font-weight: 700;
  text-decoration: none;
}

.bd-icon-button {
  width: 2.5rem;
}

.bd-toolbar-button:hover,
.bd-icon-button:hover,
.bd-account-trigger:hover {
  border-color: color-mix(in srgb, var(--bd-accent-teal) 36%, transparent);
  background: color-mix(in srgb, var(--bd-accent-teal) 8%, transparent);
  color: var(--bd-text-primary);
  transform: translateY(-1px);
}

.bd-toolbar-button:focus-visible,
.bd-icon-button:focus-visible,
.bd-account-trigger:focus-visible,
.bd-menu-item:focus-visible {
  outline: var(--bd-focus-ring-width) solid var(--bd-accent-teal);
  outline-offset: 2px;
}

.bd-balance-summary {
  gap: var(--bd-space-2);
  min-height: 2.5rem;
  padding: 0 var(--bd-space-3);
  border-left: 1px solid color-mix(in srgb, var(--bd-text-secondary) 18%, transparent);
  color: var(--bd-text-primary);
  text-decoration: none;
}

.bd-balance-item {
  display: grid;
  gap: 1px;
}

.bd-balance-item small {
  color: var(--bd-text-muted);
  font-size: 0.65rem;
}

.bd-balance-item strong {
  color: var(--bd-text-primary);
  font: 700 0.78rem/1.2 var(--bd-font-mono, ui-monospace, monospace);
}

.bd-balance-item:last-child strong {
  color: var(--bd-accent-teal);
}

.bd-balance-divider {
  width: 1px;
  height: 1.35rem;
  background: color-mix(in srgb, var(--bd-text-secondary) 18%, transparent);
}

.bd-account {
  position: relative;
}

.bd-account-trigger {
  gap: var(--bd-space-2);
  min-height: 2.5rem;
  padding: 0 var(--bd-space-2);
  border: 1px solid transparent;
  border-radius: var(--bd-radius-sm);
  background: transparent;
  color: var(--bd-text-secondary);
  transition: color var(--bd-motion-micro) ease-out, background-color var(--bd-motion-micro) ease-out, border-color var(--bd-motion-micro) ease-out, transform var(--bd-motion-micro) ease-out;
}

.bd-avatar {
  display: grid;
  place-items: center;
  width: 2rem;
  height: 2rem;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--bd-accent-teal) 32%, transparent);
  border-radius: var(--bd-radius-sm);
  background: var(--bd-surface-raised);
}

.bd-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.bd-account-copy {
  display: grid;
  min-width: 0;
  text-align: left;
}

.bd-account-copy strong {
  max-width: 8rem;
  overflow: hidden;
  color: var(--bd-text-primary);
  font-size: 0.78rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.bd-account-copy small {
  color: var(--bd-text-muted);
  font-size: 0.68rem;
}

.bd-account-menu {
  position: absolute;
  top: calc(100% + var(--bd-space-2));
  right: 0;
  width: 14rem;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--bd-text-secondary) 18%, transparent);
  border-radius: var(--bd-radius-md);
  background: var(--bd-surface-raised);
  box-shadow: 0 18px 42px color-mix(in srgb, #050810 42%, transparent);
}

.bd-account-menu-head {
  display: grid;
  gap: 2px;
  padding: var(--bd-space-3);
  border-bottom: 1px solid color-mix(in srgb, var(--bd-text-secondary) 14%, transparent);
}

.bd-account-menu-head strong {
  color: var(--bd-text-primary);
  font-size: 0.82rem;
}

.bd-account-menu-head span {
  overflow: hidden;
  color: var(--bd-text-muted);
  font-size: 0.7rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.bd-menu-item {
  display: flex;
  align-items: center;
  gap: var(--bd-space-2);
  width: 100%;
  padding: var(--bd-space-3);
  border: 0;
  background: transparent;
  color: var(--bd-text-secondary);
  font-size: 0.78rem;
  text-decoration: none;
  transition: background-color var(--bd-motion-micro) ease-out, color var(--bd-motion-micro) ease-out;
}

.bd-menu-item:hover {
  background: color-mix(in srgb, var(--bd-accent-teal) 8%, transparent);
  color: var(--bd-text-primary);
}

.bd-menu-danger:hover {
  background: color-mix(in srgb, var(--bd-status-danger) 10%, transparent);
  color: var(--bd-status-danger);
}

.bd-menu-enter-active,
.bd-menu-leave-active {
  transition: opacity var(--bd-motion-standard) ease-in-out, transform var(--bd-motion-standard) ease-in-out;
}

.bd-menu-enter-from,
.bd-menu-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

@media (max-width: 767px) {
  .bd-context { flex: 1; min-width: 0; gap: 4px; }
  .bd-context-copy { flex: 1; min-width: 0; }
  .bd-context-title { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-size: 13px; }
  .bd-context-kicker { display: none; }
  .bd-account-copy { display: none; }
  .bd-toolbar > a.bd-toolbar-button:not(.bd-recharge-button) { display: none; }
  .bd-toolbar :deep(.gift-text) { display: none; }
  .bd-toolbar :deep(.gift-entry) { width: 36px; min-width: 36px; height: 36px; padding: 4px; box-shadow: none; }
  .bd-toolbar :deep(.gift-box) { font-size: 18px; }
  .bd-topbar-inner {
    padding-inline: var(--bd-space-3);
  }

  .bd-context p,
  .bd-toolbar > :deep(.subscription-progress-mini),
  .bd-balance-summary {
    display: none;
  }

  .bd-toolbar {
    gap: var(--bd-space-1);
  }

  .bd-toolbar-button {
    width: 2.5rem;
    padding: 0;
  }
}
@media (max-width: 420px) {
  .bd-toolbar { gap: 0; }
  .bd-toolbar-button { width: 28px; }
  .bd-icon-button { width: 28px; flex-shrink: 0; }
  .bd-account-trigger { padding: 0 4px; }
}

.bd-toolbar .bd-recharge-button {
  width: auto;
  flex-shrink: 0;
  padding-inline: var(--bd-space-2);
  white-space: nowrap;
  color: var(--bd-on-action);
  background: var(--bd-accent-teal);
  border-radius: 6px;
}
.bd-menu-language { display: flex; align-items: center; justify-content: space-between; padding: 8px 12px; gap: 12px; font-size: 13px; color: var(--bd-text-secondary); }
.bd-toolbar :deep(.gift-entry) { box-shadow: none; background: transparent; border: 0; min-height: 36px; border-radius: 6px; }
.bd-toolbar :deep(.gift-text) { display: none; }
.bd-toolbar :deep(.gift-box) { font-size: 18px; }
.bd-toolbar :deep(.gift-dot) { width: 5px; height: 5px; box-shadow: none; }
@media (max-width: 767px) { .bd-topbar { min-height: 52px; } }
</style>
