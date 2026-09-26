<template>
  <div class="space-y-4">
    <button
      type="button"
      :disabled="disabled"
      class="biz-oidc-btn w-full"
      @click="startLogin"
    >
      <span class="biz-oidc-badge">
        {{ providerInitial }}
      </span>
      <span>{{ t('auth.oidc.signIn', { providerName: normalizedProviderName }) }}</span>
    </button>

    <div v-if="showDivider" class="flex items-center gap-3">
      <div class="h-px flex-1 bg-gray-200 dark:bg-dark-700"></div>
      <span class="text-xs text-gray-500 dark:text-dark-400">
        {{ t('auth.oauthOrContinue') }}
      </span>
      <div class="h-px flex-1 bg-gray-200 dark:bg-dark-700"></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { resolveAffiliateReferralCode, storeOAuthAffiliateCode } from '@/utils/oauthAffiliate'

const props = withDefaults(defineProps<{
  disabled?: boolean
  affCode?: string
  providerName?: string
  showDivider?: boolean
}>(), {
  providerName: 'OIDC',
  showDivider: true
})

const route = useRoute()
const { t } = useI18n()

const normalizedProviderName = computed(() => {
  const name = props.providerName?.trim()
  return name || 'OIDC'
})

const providerInitial = computed(() => normalizedProviderName.value.charAt(0).toUpperCase() || 'O')

function startLogin(): void {
  const redirectTo = (route.query.redirect as string) || '/dashboard'
  const affiliateCode = resolveAffiliateReferralCode(props.affCode, route.query.aff, route.query.aff_code)
  storeOAuthAffiliateCode(affiliateCode)
  const apiBase = (import.meta.env.VITE_API_BASE_URL as string | undefined) || '/api/v1'
  const normalized = apiBase.replace(/\/$/, '')
  const params = new URLSearchParams({ redirect: redirectTo })
  if (affiliateCode) {
    params.set('aff_code', affiliateCode)
  }
  const startURL = `${normalized}/auth/oauth/oidc/start?${params.toString()}`
  window.location.href = startURL
}
</script>

<style scoped>
.biz-oidc-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.75rem 1.25rem;
  border-radius: 0.95rem;
  font-size: 0.9rem;
  font-weight: 650;
  line-height: 1.2;
  cursor: pointer;
  transition: background 0.2s ease, box-shadow 0.2s ease, transform 0.05s ease;
  background: linear-gradient(135deg, #111827 0%, #0f766e 100%);
  color: #ffffff;
  border: 1px solid rgba(15, 118, 110, 0.22);
  box-shadow: 0 14px 30px rgba(15, 23, 42, 0.18);
}
.biz-oidc-btn:hover:not(:disabled) {
  background: linear-gradient(135deg, #020617 0%, #115e59 100%);
  box-shadow: 0 18px 38px rgba(15, 23, 42, 0.24);
}
.biz-oidc-btn:active:not(:disabled) {
  transform: translateY(1px) scale(0.99);
}
.biz-oidc-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.biz-oidc-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.25rem;
  height: 1.25rem;
  margin-right: 0.6rem;
  border-radius: 9999px;
  font-size: 0.7rem;
  font-weight: 800;
  background: rgba(255, 255, 255, 0.18);
  color: #ffffff;
}
</style>
