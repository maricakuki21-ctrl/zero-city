<template>
  <div v-if="visible" class="promo-banner panel overflow-hidden">
    <div class="relative flex items-center justify-between gap-4 p-4">
      <!-- Background gradient -->
      <div class="promo-banner-haze pointer-events-none absolute inset-0"></div>
      
      <!-- Content -->
      <div class="relative flex items-center gap-4">
        <!-- Icon -->
        <div class="promo-banner-icon flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl">
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M21 11.25v8.25a1.5 1.5 0 01-1.5 1.5H5.25a1.5 1.5 0 01-1.5-1.5v-8.25M12 4.875A2.625 2.625 0 109.375 7.5H12m0-2.625V7.5m0-2.625A2.625 2.625 0 1114.625 7.5H12m0 0V21m-8.625-9.75h18c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125h-18c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" />
          </svg>
        </div>
        
        <!-- Text -->
        <div>
          <p class="promo-banner-title text-sm font-semibold">{{ title }}</p>
          <p class="promo-banner-description text-xs">{{ description }}</p>
        </div>
      </div>

      <!-- Right side: Countdown + CTA -->
      <div class="relative flex items-center gap-4">
        <!-- Countdown -->
        <div v-if="!claimed && !expired" class="hidden items-center gap-2 sm:flex">
          <div class="flex gap-1">
            <div class="promo-countdown-tile rounded-lg px-2 py-1 text-center">
              <p class="promo-countdown-value font-mono text-lg font-bold">{{ countdown.days }}</p>
              <p class="promo-countdown-label text-[10px]">{{ t('promo.days') }}</p>
            </div>
            <div class="promo-countdown-tile rounded-lg px-2 py-1 text-center">
              <p class="promo-countdown-value font-mono text-lg font-bold">{{ countdown.hours }}</p>
              <p class="promo-countdown-label text-[10px]">{{ t('promo.hours') }}</p>
            </div>
            <div class="promo-countdown-tile rounded-lg px-2 py-1 text-center">
              <p class="promo-countdown-value font-mono text-lg font-bold">{{ countdown.minutes }}</p>
              <p class="promo-countdown-label text-[10px]">{{ t('promo.minutes') }}</p>
            </div>
            <div class="promo-countdown-tile rounded-lg px-2 py-1 text-center">
              <p class="promo-countdown-value font-mono text-lg font-bold">{{ countdown.seconds }}</p>
              <p class="promo-countdown-label text-[10px]">{{ t('promo.seconds') }}</p>
            </div>
          </div>
        </div>

        <!-- CTA Button -->
        <button
          v-if="!claimed && !expired && claimable"
          class="btn btn-primary btn-sm"
          :disabled="claiming"
          @click="claimReward"
        >
          {{ claiming ? t('promo.claiming') : t('promo.claim', { amount: amount }) }}
        </button>

        <!-- Claimed state -->
        <div v-if="claimed" class="promo-claimed flex items-center gap-2">
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <span class="text-sm font-semibold">{{ t('promo.claimed') }}</span>
        </div>

        <!-- Expired state -->
        <div v-if="expired && !claimed" class="promo-expired text-xs">
          {{ t('promo.expired') }}
        </div>

        <!-- Close button -->
        <button
          class="promo-close flex h-6 w-6 items-center justify-center rounded-full transition-colors"
          @click="dismiss"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { getActivePromo, claimPromo, type PromoCampaign } from '@/api/bizdecipher'

const { t } = useI18n()

// Real backend-driven promo state.
// Source: GET /api/v1/biz/promo/active  +  POST /api/v1/biz/promo/:id/claim
const campaign = ref<PromoCampaign | null>(null)
const visible = ref(false)
const claimed = ref(false)
const claimable = ref(false)
const claiming = ref(false)
const now = ref(new Date())
let timer: ReturnType<typeof setInterval> | null = null

const title = computed(() => campaign.value?.title || campaign.value?.name || '')
const description = computed(() => campaign.value?.description || '')
const amount = computed(() => campaign.value?.credit_amount ?? 0)
const endAt = computed(() => (campaign.value ? new Date(campaign.value.end_at) : null))
const autoHide = computed(() => campaign.value?.auto_hide ?? true)

const expired = computed(() => {
  if (!endAt.value) return false
  return now.value >= endAt.value
})

const countdown = computed(() => {
  if (!endAt.value) {
    return { days: '00', hours: '00', minutes: '00', seconds: '00' }
  }
  const diff = Math.max(0, endAt.value.getTime() - now.value.getTime())
  const days = Math.floor(diff / (1000 * 60 * 60 * 24))
  const hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60))
  const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60))
  const seconds = Math.floor((diff % (1000 * 60)) / 1000)
  return {
    days: String(days).padStart(2, '0'),
    hours: String(hours).padStart(2, '0'),
    minutes: String(minutes).padStart(2, '0'),
    seconds: String(seconds).padStart(2, '0')
  }
})

async function claimReward(): Promise<void> {
  if (!campaign.value) return
  claiming.value = true
  try {
    const result = await claimPromo(campaign.value.id)
    claimed.value = true
    claimable.value = false
    // Notify the rest of the app that the balance changed (Dashboard / TopBar can refresh).
    try {
      window.dispatchEvent(new CustomEvent('promo-claimed', { detail: result }))
    } catch {
      // ignore event failures
    }
    // Auto-hide after 3 seconds.
    if (autoHide.value) {
      setTimeout(() => {
        visible.value = false
      }, 3000)
    }
  } catch (error) {
    // Surface nothing intrusive; keep the banner so the user can retry.
    console.error('Failed to claim promo:', error)
  } finally {
    claiming.value = false
  }
}

function dismiss(): void {
  visible.value = false
}

function startTimer(): void {
  timer = setInterval(() => {
    now.value = new Date()
    if (expired.value && autoHide.value && !claimed.value) {
      visible.value = false
      if (timer) {
        clearInterval(timer)
        timer = null
      }
    }
  }, 1000)
}

onMounted(async () => {
  try {
    const view = await getActivePromo()
    if (!view || !view.campaign) {
      visible.value = false
      return
    }
    campaign.value = view.campaign
    claimed.value = view.claimed
    claimable.value = view.claimable

    // Hide when already expired and nothing to show.
    if (expired.value && !claimed.value) {
      visible.value = false
      return
    }

    visible.value = true
    startTimer()
  } catch (error) {
    // Backend unreachable or no active promo: fail silently, hide the banner.
    visible.value = false
  }
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.promo-banner {
  background: var(--zc-bg) !important;
  box-shadow: 14px 14px 30px var(--zc-shadow-dark), -14px -14px 30px var(--zc-shadow-light) !important;
}

.promo-banner-haze {
  background: radial-gradient(circle at 6% 22%, color-mix(in srgb, var(--zc-accent) 12%, transparent), transparent 34%), radial-gradient(circle at 86% 12%, color-mix(in srgb, var(--zc-accent-2) 10%, transparent), transparent 38%);
}

.promo-banner-icon,
.promo-countdown-tile,
.promo-close {
  background: var(--zc-bg);
  color: var(--zc-accent);
  box-shadow: inset 5px 5px 12px var(--zc-shadow-dark), inset -5px -5px 12px var(--zc-shadow-light);
}

.promo-banner-title,
.promo-countdown-value {
  color: var(--zc-text-strong);
}

.promo-banner-description,
.promo-countdown-label,
.promo-expired {
  color: var(--zc-muted);
}

.promo-claimed {
  color: var(--zc-success);
}

.promo-close:hover {
  color: var(--zc-accent-2);
  box-shadow: 6px 6px 14px var(--zc-shadow-dark), -6px -6px 14px var(--zc-shadow-light);
}
</style>
