import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import i18n, { initI18n } from './i18n'
import { useAppStore } from '@/stores/app'
import { LOCAL_PREVIEW_TOKEN, isLocalPreviewHost } from '@/utils/localPreview'
import { updateFavicon } from '@/utils/branding'
import { isIOSDevice } from '@/utils/device'
import './style.css'

function resolveStoredTheme(): 'noir' | 'daylight' {
  const stored = localStorage.getItem('zero-city-theme') || localStorage.getItem('theme') || ''
  if (stored === 'daylight' || stored === 'light') return 'daylight'
  return 'noir'
}

function initIOSViewportZoomFix() {
  // iOS Safari 在输入框字号小于 16px 时聚焦会自动放大页面，且失焦后不会恢复。
  // 限制 maximum-scale 可阻止该行为；iOS 10+ 用户仍可双指手动缩放，不影响可访问性。
  // 仅在 iOS 设备上注入，避免影响 Android Chrome 的手动缩放能力。
  if (!isIOSDevice()) return

  const viewport = document.querySelector('meta[name="viewport"]')
  if (!viewport) return

  const content = viewport.getAttribute('content') || ''
  if (/maximum-scale/i.test(content)) return
  viewport.setAttribute('content', `${content}, maximum-scale=1.0`)
}

function initThemeClass() {
  const theme = resolveStoredTheme()
  document.documentElement.dataset.theme = theme
  document.documentElement.classList.toggle('dark', theme === 'noir')
  document.documentElement.classList.toggle('theme-noir', theme === 'noir')
  document.documentElement.classList.toggle('theme-daylight', theme === 'daylight')
}

function applyLocalPreviewAuthBypass(): void {
  if (!isLocalPreviewHost()) return

  const params = new URLSearchParams(window.location.search)
  if (params.get('previewAuth') !== '1') return

  const previewUser = {
    id: 1,
    username: 'preview',
    display_name: 'Preview User',
    email: 'preview@localhost',
    role: 'user',
    status: 'active',
    credit_balance: 500,
    balance: 100
  }

  localStorage.setItem('auth_token', LOCAL_PREVIEW_TOKEN)
  localStorage.setItem('auth_user', JSON.stringify(previewUser))
  localStorage.setItem('token_expires_at', String(Date.now() + 24 * 60 * 60 * 1000))
  if (!localStorage.getItem('zero-city-theme') && !localStorage.getItem('theme')) {
    localStorage.setItem('zero-city-theme', 'noir')
    localStorage.setItem('theme', 'dark')
  }

  const requestedPath = params.get('redirect') || window.location.pathname
  const safePath = requestedPath.startsWith('/') && !requestedPath.startsWith('//') ? requestedPath : '/dashboard'
  const nextPath = safePath === '/' || safePath === '/login' || safePath === '/register' ? '/dashboard' : safePath
  const nextUrl = new URL(window.location.href)
  nextUrl.pathname = nextPath
  nextUrl.search = ''
  nextUrl.hash = ''
  window.history.replaceState(null, '', nextUrl.toString())
}

async function bootstrap() {
  applyLocalPreviewAuthBypass()

  // Apply theme class globally before app mount to keep all routes consistent.
  initThemeClass()
  initIOSViewportZoomFix()

  const app = createApp(App)
  const pinia = createPinia()
  app.use(pinia)

  // Initialize settings from injected config BEFORE mounting (prevents flash)
  // This must happen after pinia is installed but before router and i18n
  const appStore = useAppStore()
  appStore.initFromInjectedConfig()

  // Set document title immediately after config is loaded
  if (appStore.siteName) {
    document.title = `${appStore.siteName} · AI Workspace`
  }
  updateFavicon(appStore.siteLogo)

  await initI18n()

  app.use(router)
  app.use(i18n)

  // 等待路由器完成初始导航后再挂载，避免竞态条件导致的空白渲染
  await router.isReady()
  app.mount('#app')
}

bootstrap()
