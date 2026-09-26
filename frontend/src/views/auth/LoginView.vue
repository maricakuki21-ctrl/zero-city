<template>
  <AuthLayout>
    <div class="la-form">

      <!-- Header -->
      <div class="la-header">
        <h2 class="la-title">{{ t('biz.auth.loginTitle') }}</h2>
        <p class="la-subtitle">{{ t('biz.auth.loginSubtitle') }}</p>
      </div>

      <!-- Login Agreement Prompt -->
      <Transition name="la-fade">
        <LoginAgreementPrompt
          v-if="showAgreementModal"
          :documents="loginAgreementDocuments"
          :mode="loginAgreementMode"
          :updated-at="loginAgreementUpdatedAt"
          :accepted="agreementAccepted"
          :visible="showAgreementModal"
          @accept="acceptLoginAgreement"
          @reject="rejectLoginAgreement"
          @open="showAgreementModal = true"
        />
      </Transition>

      <!-- Checkbox agreement -->
      <div
        v-if="loginAgreementEnabled && loginAgreementMode === 'checkbox' && loginAgreementDocuments.length"
        class="la-agreement-check"
      >
        <LoginAgreementPrompt
          :documents="loginAgreementDocuments"
          :mode="'checkbox'"
          :updated-at="loginAgreementUpdatedAt"
          :accepted="agreementAccepted"
          :visible="false"
          @accept="acceptLoginAgreement"
          @reject="rejectLoginAgreement"
          @open="showAgreementModal = true"
        />
      </div>

      <!-- Email / Password Form -->
      <form @submit.prevent="handleLogin" class="la-fields" novalidate>
        <!-- Email -->
        <div class="la-field">
          <label for="email" class="la-label">{{ t('auth.emailLabel') }}</label>
          <div class="la-input-wrap">
            <Icon name="mail" size="md" class="la-input-icon" />
            <input
              id="email"
              v-model="formData.email"
              type="email"
              required
              autofocus
              autocomplete="email"
              :disabled="authActionDisabled"
              class="input la-input"
              :class="{ 'input-error': errors.email }"
              :aria-invalid="!!errors.email"
              :aria-describedby="errors.email ? 'email-error' : undefined"
              :placeholder="t('auth.emailPlaceholder')"
            />
          </div>
          <p v-if="errors.email" id="email-error" class="la-error">{{ errors.email }}</p>
        </div>

        <!-- Password -->
        <div class="la-field">
          <div class="la-label-row">
            <label for="password" class="la-label">{{ t('auth.passwordLabel') }}</label>
            <RouterLink
              v-if="passwordResetEnabled && !backendModeEnabled"
              to="/forgot-password"
              class="la-forgot"
            >
              {{ t('auth.forgotPassword') }}
            </RouterLink>
          </div>
          <div class="la-input-wrap">
            <Icon name="lock" size="md" class="la-input-icon" />
            <input
              id="password"
              v-model="formData.password"
              :type="showPassword ? 'text' : 'password'"
              required
              autocomplete="current-password"
              :disabled="authActionDisabled"
              class="input la-input"
              :class="{ 'input-error': errors.password }"
              :aria-invalid="!!errors.password"
              :aria-describedby="errors.password ? 'password-error' : undefined"
              :placeholder="t('auth.passwordPlaceholder')"
            />
            <button
              type="button"
              class="la-eye-btn"
              :aria-label="showPassword ? '隐藏密码' : '显示密码'"
              :title="showPassword ? '隐藏密码' : '显示密码'"
              :aria-pressed="showPassword"
              :disabled="authActionDisabled"
              @click="showPassword = !showPassword"
            >
              <Icon :name="showPassword ? 'eyeOff' : 'eye'" size="md" />
            </button>
          </div>
          <p v-if="errors.password" id="password-error" class="la-error">{{ errors.password }}</p>
        </div>

        <!-- Turnstile -->
        <TurnstileWidget
          v-if="turnstileEnabled && turnstileSiteKey"
          ref="turnstileRef"
          :site-key="turnstileSiteKey"
          @verify="onTurnstileVerify"
          @expire="onTurnstileExpire"
          @error="onTurnstileError"
        />

        <p v-if="errorMessage" class="la-error" role="alert">{{ errorMessage }}</p>

        <!-- Submit -->
        <button
          type="submit"
          class="btn btn-primary w-full la-submit"
          :disabled="authActionDisabled || (turnstileEnabled && !turnstileToken)"
        >
          <svg v-if="isLoading" class="la-spin" viewBox="0 0 24 24" fill="none">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
          </svg>
          <Icon v-else name="login" size="md" />
          {{ isLoading ? t('auth.signingIn') : t('auth.signIn') }}
        </button>
      </form>

      <!-- OAuth Section -->
      <template v-if="showOAuthLogin">
        <div class="la-divider">
          <span>{{ t('auth.orContinueWith') }}</span>
        </div>
        <div class="la-oauth">
          <EmailOAuthButtons
            v-if="githubOAuthEnabled || googleOAuthEnabled"
            :github-enabled="githubOAuthEnabled"
            :google-enabled="googleOAuthEnabled"
            :disabled="agreementGateActive"
            :agreement-gate-active="agreementGateActive"
            @require-agreement="showAgreementModal = true"
          />
          <LinuxDoOAuthSection
            v-if="linuxdoOAuthEnabled"
            :disabled="agreementGateActive"
            :agreement-gate-active="agreementGateActive"
            @require-agreement="showAgreementModal = true"
          />
          <DingTalkOAuthSection
            v-if="dingtalkOAuthEnabled"
            :disabled="agreementGateActive"
            :agreement-gate-active="agreementGateActive"
            @require-agreement="showAgreementModal = true"
          />
          <WechatOAuthSection
            v-if="wechatOAuthEnabled"
            :disabled="agreementGateActive"
            :agreement-gate-active="agreementGateActive"
            @require-agreement="showAgreementModal = true"
          />
          <OidcOAuthSection
            v-if="oidcOAuthEnabled"
            :provider-name="oidcOAuthProviderName"
            :disabled="agreementGateActive"
            :agreement-gate-active="agreementGateActive"
            @require-agreement="showAgreementModal = true"
          />
        </div>
      </template>
    </div>

    <!-- Footer slot -->
    <template #footer>
      <span class="la-footer-text">{{ t('auth.dontHaveAccount') }}</span>
      <RouterLink to="/register" class="la-footer-link">
        {{ t('auth.createAccount') }}
      </RouterLink>
    </template>

    <!-- TOTP 2FA Modal -->
    <TotpLoginModal
      v-if="show2FAModal"
      ref="totpModalRef"
      :temp-token="totpTempToken"
      :user-email-masked="totpUserEmailMasked"
      @verify="handle2FAVerify"
      @cancel="handle2FACancel"
    />
  </AuthLayout>
</template>

<script setup lang="ts">
import { computed, ref, reactive, onMounted, watch } from 'vue'
import { useRouter, RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { AuthLayout } from '@/components/layout'
import LinuxDoOAuthSection from '@/components/auth/LinuxDoOAuthSection.vue'
import DingTalkOAuthSection from '@/components/auth/DingTalkOAuthSection.vue'
import OidcOAuthSection from '@/components/auth/OidcOAuthSection.vue'
import WechatOAuthSection from '@/components/auth/WechatOAuthSection.vue'
import EmailOAuthButtons from '@/components/auth/EmailOAuthButtons.vue'
import LoginAgreementPrompt from '@/components/auth/LoginAgreementPrompt.vue'
import TotpLoginModal from '@/components/auth/TotpLoginModal.vue'
import Icon from '@/components/icons/Icon.vue'
import TurnstileWidget from '@/components/TurnstileWidget.vue'
import { useAuthStore, useAppStore } from '@/stores'
import { getPublicSettings, isTotp2FARequired, isWeChatWebOAuthEnabled } from '@/api/auth'
import type { LoginAgreementDocument, TotpLoginResponse } from '@/types'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { clearAllAffiliateReferralCodes } from '@/utils/oauthAffiliate'

const { t } = useI18n()
const LOGIN_AGREEMENT_STORAGE_KEY = 'sub2api_login_agreement_consent'

// ── Stores & Router ──
const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()

// ── Loading / UI state ──
const isLoading = ref(false)
const showPassword = ref(false)
const publicSettingsLoaded = ref(false)
const errorMessage = ref('')

// ── Public settings ──
const turnstileEnabled = ref(false)
const turnstileSiteKey = ref('')
const linuxdoOAuthEnabled = ref(false)
const dingtalkOAuthEnabled = ref(false)
const wechatOAuthEnabled = ref(false)
const backendModeEnabled = ref(false)
const oidcOAuthEnabled = ref(false)
const oidcOAuthProviderName = ref('OIDC')
const githubOAuthEnabled = ref(false)
const googleOAuthEnabled = ref(false)
const passwordResetEnabled = ref(false)
const loginAgreementEnabled = ref(false)
const loginAgreementMode = ref<'modal' | 'checkbox' | string>('modal')
const loginAgreementUpdatedAt = ref('')
const loginAgreementRevision = ref('')
const loginAgreementDocuments = ref<LoginAgreementDocument[]>([])
const agreementAccepted = ref(false)
const showAgreementModal = ref(false)

// ── Turnstile ──
const turnstileRef = ref<InstanceType<typeof TurnstileWidget> | null>(null)
const turnstileToken = ref('')

// ── 2FA ──
const show2FAModal = ref(false)
const totpTempToken = ref('')
const totpUserEmailMasked = ref('')
const totpModalRef = ref<InstanceType<typeof TotpLoginModal> | null>(null)

// ── Form ──
const formData = reactive({ email: '', password: '' })
const errors = reactive({ email: '', password: '', turnstile: '' })

// ── Computed ──
const validationToastMessage = computed(() => errors.email || errors.password || errors.turnstile || '')
const agreementGateActive = computed(() => loginAgreementEnabled.value && !agreementAccepted.value)
const authActionDisabled = computed(() => isLoading.value || !publicSettingsLoaded.value || agreementGateActive.value)
const showOAuthLogin = computed(() =>
  !backendModeEnabled.value &&
  (linuxdoOAuthEnabled.value || dingtalkOAuthEnabled.value || wechatOAuthEnabled.value ||
   oidcOAuthEnabled.value || githubOAuthEnabled.value || googleOAuthEnabled.value)
)

watch(validationToastMessage, (v, prev) => { if (v && v !== prev) appStore.showError(v) })

// ── Lifecycle ──
onMounted(async () => {
  const expiredFlag = sessionStorage.getItem('auth_expired')
  if (expiredFlag) {
    sessionStorage.removeItem('auth_expired')
    const message = t('auth.reloginRequired')
    errorMessage.value = message
    appStore.showWarning(message)
  }
  try {
    const settings = await getPublicSettings()
    turnstileEnabled.value = settings.turnstile_enabled
    turnstileSiteKey.value = settings.turnstile_site_key || ''
    linuxdoOAuthEnabled.value = settings.linuxdo_oauth_enabled
    dingtalkOAuthEnabled.value = settings.dingtalk_oauth_enabled ?? false
    wechatOAuthEnabled.value = isWeChatWebOAuthEnabled(settings)
    backendModeEnabled.value = settings.backend_mode_enabled
    oidcOAuthEnabled.value = settings.oidc_oauth_enabled
    oidcOAuthProviderName.value = settings.oidc_oauth_provider_name || 'OIDC'
    githubOAuthEnabled.value = settings.github_oauth_enabled
    googleOAuthEnabled.value = settings.google_oauth_enabled
    passwordResetEnabled.value = settings.password_reset_enabled
    applyLoginAgreementSettings(settings)
  } catch {
    loginAgreementEnabled.value = false
    agreementAccepted.value = true
  } finally {
    publicSettingsLoaded.value = true
  }
})

// ── Agreement ──
function applyLoginAgreementSettings(settings: {
  login_agreement_enabled?: boolean; login_agreement_mode?: string
  login_agreement_updated_at?: string; login_agreement_revision?: string
  login_agreement_documents?: LoginAgreementDocument[]
}) {
  const docs = Array.isArray(settings.login_agreement_documents)
    ? settings.login_agreement_documents.filter(d => d.title?.trim()) : []
  loginAgreementDocuments.value = docs
  loginAgreementEnabled.value = settings.login_agreement_enabled === true && docs.length > 0
  loginAgreementMode.value = settings.login_agreement_mode === 'checkbox' ? 'checkbox' : 'modal'
  loginAgreementUpdatedAt.value = settings.login_agreement_updated_at || ''
  loginAgreementRevision.value = settings.login_agreement_revision ||
    `${loginAgreementUpdatedAt.value}:${docs.map(d => `${d.id}:${d.title}`).join('|')}`
  agreementAccepted.value = !loginAgreementEnabled.value || hasAcceptedLoginAgreement(loginAgreementRevision.value)
  showAgreementModal.value = loginAgreementEnabled.value && !agreementAccepted.value && loginAgreementMode.value !== 'checkbox'
}

function hasAcceptedLoginAgreement(revision: string): boolean {
  if (!revision) return false
  try {
    const raw = localStorage.getItem(LOGIN_AGREEMENT_STORAGE_KEY)
    if (!raw) return false
    return (JSON.parse(raw) as { revision?: string }).revision === revision
  } catch { return false }
}

function acceptLoginAgreement() {
  if (loginAgreementRevision.value) {
    localStorage.setItem(LOGIN_AGREEMENT_STORAGE_KEY, JSON.stringify({
      revision: loginAgreementRevision.value, accepted_at: new Date().toISOString()
    }))
  }
  agreementAccepted.value = true
  showAgreementModal.value = false
}

function rejectLoginAgreement() {
  localStorage.removeItem(LOGIN_AGREEMENT_STORAGE_KEY)
  agreementAccepted.value = false
  showAgreementModal.value = false
  appStore.showWarning(t('auth.agreementRequired') || '请先同意最新条款。')
  appStore.showWarning(t('legal.loginAgreementPrompt.loginRejectedWarning'))
}

// ── Turnstile ──
function onTurnstileVerify(token: string) { turnstileToken.value = token; errors.turnstile = '' }
function onTurnstileExpire() { turnstileToken.value = ''; errors.turnstile = t('auth.turnstileExpired') }
function onTurnstileError() { turnstileToken.value = ''; errors.turnstile = t('auth.turnstileFailed') }

// ── Validation ──
function validateForm(): boolean {
  errors.email = ''; errors.password = ''; errors.turnstile = ''
  if (agreementGateActive.value) {
    if (loginAgreementMode.value !== 'checkbox') showAgreementModal.value = true
    appStore.showWarning(t('legal.loginAgreementPrompt.loginRequiredWarning'))
    if (loginAgreementMode.value !== 'checkbox') {
      showAgreementModal.value = true
    }
    return false
  }
  let ok = true
  if (!formData.email.trim()) { errors.email = t('auth.emailRequired'); ok = false }
  else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(formData.email)) { errors.email = t('auth.invalidEmail'); ok = false }
  if (!formData.password) { errors.password = t('auth.passwordRequired'); ok = false }
  else if (formData.password.length < 6) { errors.password = t('auth.passwordMinLength'); ok = false }
  if (turnstileEnabled.value && !turnstileToken.value) { errors.turnstile = t('auth.completeVerification'); ok = false }
  return ok
}

// ── Login ──
async function handleLogin() {
  errorMessage.value = ''
  if (!validateForm()) return
  isLoading.value = true
  try {
    const response = await authStore.login({
      email: formData.email,
      password: formData.password,
      turnstile_token: turnstileEnabled.value ? turnstileToken.value : undefined
    })
    if (isTotp2FARequired(response)) {
      const r = response as TotpLoginResponse
      totpTempToken.value = r.temp_token || ''
      totpUserEmailMasked.value = r.user_email_masked || ''
      show2FAModal.value = true
      isLoading.value = false
      return
    }
    clearAllAffiliateReferralCodes()
    appStore.showSuccess(t('auth.loginSuccess'))
    await router.push((router.currentRoute.value.query.redirect as string) || '/dashboard')
  } catch (error: unknown) {
    if (turnstileRef.value) { turnstileRef.value.reset(); turnstileToken.value = '' }
    errorMessage.value = extractI18nErrorMessage(error, t, 'auth.errors', t('auth.loginFailed'))
    appStore.showError(errorMessage.value)
  } finally {
    isLoading.value = false
  }
}

// ── 2FA ──
async function handle2FAVerify(code: string) {
  totpModalRef.value?.setVerifying(true)
  try {
    await authStore.login2FA(totpTempToken.value, code)
    show2FAModal.value = false
    clearAllAffiliateReferralCodes()
    appStore.showSuccess(t('auth.loginSuccess'))
    await router.push((router.currentRoute.value.query.redirect as string) || '/dashboard')
  } catch (error: unknown) {
    const err = error as { message?: string; response?: { data?: { message?: string } } }
    const msg = err.response?.data?.message || err.message || t('profile.totp.loginFailed')
    totpModalRef.value?.setError(msg)
    totpModalRef.value?.setVerifying(false)
  }
}

function handle2FACancel() {
  show2FAModal.value = false
  totpTempToken.value = ''
  totpUserEmailMasked.value = ''
}
</script>

<style scoped>
/* Auth form structure. Palette comes from AuthLayout's shared auth tokens;
   this file only owns layout specific to the login form. */
.la-form {
  --bd-marketing-text: var(--public-text);
  --bd-marketing-text-secondary: var(--public-muted);
  --bd-marketing-accent: var(--public-action);
  --bd-marketing-action-from: var(--public-action-hover);
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.la-header { text-align: center; }

.la-title {
  font-size: 1.5rem;
  font-weight: 600;
  letter-spacing: 0;
  color: var(--bd-marketing-text);
  margin: 0;
}

.la-subtitle {
  margin: 6px 0 0;
  font-size: 0.875rem;
  color: var(--bd-marketing-text-secondary);
  line-height: 1.5;
}

/* Fields */
.la-fields { display: flex; flex-direction: column; gap: 16px; }

.la-field { display: flex; flex-direction: column; gap: 6px; }

.la-label {
  font-size: 0.8125rem;
  font-weight: 700;
  color: var(--bd-marketing-text-secondary);
}

.la-label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.la-forgot {
  font-size: 0.8125rem;
  font-weight: 600;
  color: var(--bd-marketing-accent);
  text-decoration: none;
  transition: color 0.15s;
}
.la-forgot:hover { color: var(--bd-marketing-action-from); }

/* Input wrapper */
.la-input-wrap { position: relative; }

.la-input-icon {
  position: absolute;
  left: 12px;
  top: 50%;
  transform: translateY(-50%);
  color: var(--bd-marketing-text-secondary);
  pointer-events: none;
}

.la-input {
  width: 100%;
  padding: 0 42px 0 40px;
  min-height: 2.75rem;
  border-radius: var(--public-radius);
  background: var(--public-field);
  border: 1px solid var(--public-line);
  color: var(--public-text);
  box-shadow: none;
}
.la-input::placeholder { color: var(--public-muted); }
.la-input:focus {
  border-color: var(--public-action);
  box-shadow: none;
}

.la-input.input-error { border-color: var(--public-danger); }
.la-error { color: var(--public-danger); font-size: 12px; line-height: 1.5; }

.la-eye-btn {
  position: absolute;
  right: 12px;
  top: 50%;
  transform: translateY(-50%);
  background: none;
  border: none;
  padding: 0;
  color: var(--bd-marketing-text-secondary);
  cursor: pointer;
  display: flex;
  align-items: center;
  transition: color 0.15s;
}
.la-eye-btn:hover { color: var(--bd-marketing-text); }

/* Submit */
.la-submit {
  min-height: 2.75rem;
  border-radius: var(--public-radius);
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.la-spin {
  width: 16px;
  height: 16px;
  animation: la-spin 1s linear infinite;
}
@keyframes la-spin { to { transform: rotate(360deg); } }

/* Divider */
.la-divider {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--bd-marketing-text-secondary);
  text-transform: uppercase;
  letter-spacing: 0;
}
.la-divider::before,
.la-divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: color-mix(in srgb, var(--bd-marketing-text) 12%, transparent);
}

/* OAuth */
.la-oauth { display: flex; flex-direction: column; gap: 8px; }

/* Agreement */
.la-agreement-check {
  background: color-mix(in srgb, var(--bd-marketing-text) 5%, transparent);
  border-radius: var(--public-radius);
  padding: 12px;
}

/* Footer */
.la-footer-text { color: var(--bd-marketing-text-secondary); font-size: 0.875rem; }
.la-footer-link {
  font-size: 0.875rem;
  font-weight: 700;
  color: var(--bd-marketing-accent);
  text-decoration: none;
  transition: color 0.15s;
}
.la-footer-link:hover { color: var(--bd-marketing-action-from); }

/* Transition */
.la-fade-enter-active,
.la-fade-leave-active { transition: opacity 0.2s, transform 0.2s; }
.la-fade-enter-from,
.la-fade-leave-to { opacity: 0; transform: translateY(-6px); }
</style>
